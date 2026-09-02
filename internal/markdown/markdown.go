// Package markdown renders a repository readme to sanitized HTML for display
// in the Discover and App detail views (PRD §8, §12).
package markdown

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Renderer converts readme Markdown into HTML that is safe to inject.
type Renderer struct {
	policy *bluemonday.Policy
}

// NewRenderer builds a renderer with koa's sanitizer policy.
func NewRenderer() *Renderer {
	p := bluemonday.UGCPolicy()
	// Readmes lean on images, tables and task lists; allow the markup goldmark
	// emits for them while the policy keeps scripts and event handlers out.
	p.AllowImages()
	p.AllowTables()
	p.AllowAttrs("class").Matching(bluemonday.SpaceSeparatedTokens).OnElements("code", "pre", "span", "div", "li", "input")
	p.AllowAttrs("type", "checked", "disabled").OnElements("input")
	p.AllowAttrs("align").OnElements("td", "th", "p", "div", "img")
	p.AllowAttrs("start").OnElements("ol")
	p.AllowAttrs("id").OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	p.RequireNoFollowOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)
	return &Renderer{policy: p}
}

// Render converts markdown to sanitized HTML. Relative links and images are
// resolved against the repo on github.com so a readme written for GitHub looks
// the same inside koa.
func (r *Renderer) Render(markdown, owner, repo, defaultBranch string) (string, error) {
	if strings.TrimSpace(markdown) == "" {
		return "", nil
	}
	if defaultBranch == "" {
		defaultBranch = "HEAD"
	}

	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Footnote,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithASTTransformers(
				util.Prioritized(&linkResolver{owner: owner, repo: repo, branch: defaultBranch}, 100),
			),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		return "", fmt.Errorf("render readme: %w", err)
	}
	return r.policy.Sanitize(buf.String()), nil
}

// linkResolver rewrites repo-relative destinations to absolute GitHub URLs.
type linkResolver struct {
	owner  string
	repo   string
	branch string
}

func (l *linkResolver) Transform(doc *ast.Document, reader text.Reader, pc parser.Context) {
	if l.owner == "" || l.repo == "" {
		return
	}
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *ast.Image:
			node.Destination = l.absolute(node.Destination, true)
		case *ast.Link:
			node.Destination = l.absolute(node.Destination, false)
		}
		return ast.WalkContinue, nil
	})
}

// absolute turns a relative destination into a github.com URL. Images point at
// raw.githubusercontent.com so they load; links point at the blob view.
func (l *linkResolver) absolute(dest []byte, isImage bool) []byte {
	raw := string(dest)
	if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, "mailto:") {
		return dest
	}
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" {
		return dest
	}
	if strings.HasPrefix(raw, "//") {
		return dest
	}

	clean := strings.TrimPrefix(raw, "./")
	clean = strings.TrimPrefix(clean, "/")
	if isImage {
		return []byte(fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", l.owner, l.repo, l.branch, clean))
	}
	return []byte(fmt.Sprintf("https://github.com/%s/%s/blob/%s/%s", l.owner, l.repo, l.branch, clean))
}
