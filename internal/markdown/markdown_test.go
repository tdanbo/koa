package markdown

import (
	"strings"
	"testing"
)

func TestRenderBasics(t *testing.T) {
	r := NewRenderer()
	html, err := r.Render("# dumpscope\n\nOpen and **diff** dumps.\n\n- one\n- two\n", "playdead", "dumpscope", "main")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	for _, want := range []string{"<h1", "dumpscope", "<strong>diff</strong>", "<li>one</li>"} {
		if !strings.Contains(html, want) {
			t.Errorf("output missing %q:\n%s", want, html)
		}
	}
}

func TestRenderStripsScriptsAndHandlers(t *testing.T) {
	r := NewRenderer()
	html, err := r.Render("<script>alert(1)</script>\n\n<img src=x onerror=\"alert(1)\">\n\n[click](javascript:alert(1))\n", "o", "r", "main")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	lower := strings.ToLower(html)
	for _, banned := range []string{"<script", "onerror", "javascript:"} {
		if strings.Contains(lower, banned) {
			t.Errorf("sanitizer let %q through:\n%s", banned, html)
		}
	}
}

func TestRenderResolvesRelativeDestinations(t *testing.T) {
	r := NewRenderer()
	html, err := r.Render("![shot](docs/shot.png)\n\n[contributing](./CONTRIBUTING.md)\n\n[abs](https://example.com/x)\n\n[anchor](#usage)\n", "playdead", "dumpscope", "main")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(html, "https://raw.githubusercontent.com/playdead/dumpscope/main/docs/shot.png") {
		t.Errorf("relative image not resolved:\n%s", html)
	}
	if !strings.Contains(html, "https://github.com/playdead/dumpscope/blob/main/CONTRIBUTING.md") {
		t.Errorf("relative link not resolved:\n%s", html)
	}
	if !strings.Contains(html, "https://example.com/x") {
		t.Errorf("absolute link was rewritten:\n%s", html)
	}
	if !strings.Contains(html, `href="#usage"`) {
		t.Errorf("anchor link was rewritten:\n%s", html)
	}
}

func TestRenderTablesAndFences(t *testing.T) {
	r := NewRenderer()
	html, err := r.Render("| a | b |\n|---|---|\n| 1 | 2 |\n\n```sh\nkoa install x\n```\n", "o", "r", "main")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(html, "<table>") {
		t.Errorf("GFM table not rendered:\n%s", html)
	}
	if !strings.Contains(html, "<pre>") || !strings.Contains(html, "koa install x") {
		t.Errorf("fenced code not rendered:\n%s", html)
	}
}

func TestRenderEmpty(t *testing.T) {
	r := NewRenderer()
	got, err := r.Render("   \n\n", "o", "r", "main")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty output, got %q", got)
	}
}
