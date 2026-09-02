// Package ghapi is koa's GitHub REST client. It covers exactly the endpoints
// the product needs: identity, org membership, topic search, releases, readme
// and asset download (PRD §7, §8, §9, §12).
package ghapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the public GitHub API root.
const DefaultBaseURL = "https://api.github.com"

// userAgent identifies koa to GitHub; the API requires one.
const userAgent = "koa-desktop"

// Client talks to the GitHub REST API on behalf of the signed-in user.
// It is safe for concurrent use.
type Client struct {
	httpClient *http.Client
	baseURL    string

	mu    sync.RWMutex
	token string
}

// New returns a client for the public GitHub API.
func New(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    DefaultBaseURL,
		token:      token,
	}
}

// SetBaseURL overrides the API root. Used by tests.
func (c *Client) SetBaseURL(u string) { c.baseURL = strings.TrimSuffix(u, "/") }

// SetHTTPClient overrides the transport. Used by tests.
func (c *Client) SetHTTPClient(h *http.Client) { c.httpClient = h }

// Clone returns a client that talks to the same API host over the same
// transport but with a different token. It is how koa validates a candidate
// credential without disturbing the one already in use.
func (c *Client) Clone(token string) *Client {
	return &Client{
		httpClient: c.httpClient,
		baseURL:    c.baseURL,
		token:      token,
	}
}

// SetToken swaps the credential in place, so signing in or out does not
// require rebuilding everything that holds a reference to the client.
func (c *Client) SetToken(token string) {
	c.mu.Lock()
	c.token = token
	c.mu.Unlock()
}

// Token returns the credential currently in use.
func (c *Client) Token() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

// HasToken reports whether the client is authenticated.
func (c *Client) HasToken() bool { return c.Token() != "" }

// do issues a request against the API root and decodes a JSON body into out.
// It returns the response so callers can read pagination headers.
func (c *Client) do(ctx context.Context, method, path string, accept string, out any) (*http.Response, error) {
	endpoint := path
	if !strings.HasPrefix(path, "http") {
		endpoint = c.baseURL + path
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if accept == "" {
		accept = "application/vnd.github+json"
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", userAgent)
	if tok := c.Token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contact github: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		defer resp.Body.Close()
		return resp, newError(resp, decodeErrorMessage(resp.Body))
	}

	if out == nil {
		resp.Body.Close()
		return resp, nil
	}

	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return resp, fmt.Errorf("decode github response: %w", err)
	}
	return resp, nil
}

// decodeErrorMessage pulls GitHub's human-readable `message` out of an error
// body, tolerating bodies that are not JSON at all.
func decodeErrorMessage(r io.Reader) string {
	var payload struct {
		Message string `json:"message"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	body, err := io.ReadAll(io.LimitReader(r, 64<<10))
	if err != nil || len(body) == 0 {
		return ""
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return strings.TrimSpace(string(body))
	}
	if payload.Message == "" && len(payload.Errors) > 0 {
		return payload.Errors[0].Message
	}
	return payload.Message
}

var nextLinkPattern = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

// nextPage extracts the `rel="next"` URL from a Link header, if present.
func nextPage(resp *http.Response) string {
	if resp == nil {
		return ""
	}
	if m := nextLinkPattern.FindStringSubmatch(resp.Header.Get("Link")); len(m) == 2 {
		return m[1]
	}
	return ""
}

// User is the signed-in GitHub account (PRD §5.2 account footer).
type User struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

// CurrentUser is the lightweight call koa makes to validate a token before
// persisting it (PRD §7).
func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var u User
	if _, err := c.do(ctx, http.MethodGet, "/user", "", &u); err != nil {
		return User{}, err
	}
	return u, nil
}

// Org is one organization the user belongs to.
type Org struct {
	Login string `json:"login"`
}

// Orgs lists the signed-in user's org memberships, including private ones.
// It returns an empty slice — not an error — for fine-grained tokens, which
// GitHub deliberately excludes from this endpoint (PRD §7).
func (c *Client) Orgs(ctx context.Context) ([]Org, error) {
	var all []Org
	page := "/user/orgs?per_page=100"
	for page != "" {
		var batch []Org
		resp, err := c.do(ctx, http.MethodGet, page, "", &batch)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		page = nextPage(resp)
		if len(all) > 500 {
			break
		}
	}
	return all, nil
}

// Repo is a repository surfaced by topic search (PRD §8).
type Repo struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name"`
	Description   string `json:"description"`
	Private       bool   `json:"private"`
	HTMLURL       string `json:"html_url"`
	DefaultBranch string `json:"default_branch"`
	Owner         struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// Repository fetches a single repo, which is how koa gets metadata for a repo
// it was not handed by a search — an installed app, or a deep link.
func (c *Client) Repository(ctx context.Context, owner, repo string) (Repo, error) {
	var out Repo
	path := fmt.Sprintf("/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	if _, err := c.do(ctx, http.MethodGet, path, "", &out); err != nil {
		return Repo{}, err
	}
	return out, nil
}

// Visibility renders the badge text the UI shows next to a repo name.
func (r Repo) Visibility() string {
	if r.Private {
		return "Private"
	}
	return "Public"
}

// SearchTopic runs one `topic:{topic} {qualifier}` search, e.g.
// `topic:koa user:m-halvorsen` or `topic:koa org:playdead` (PRD §8).
func (c *Client) SearchTopic(ctx context.Context, topic, qualifier string) ([]Repo, error) {
	query := fmt.Sprintf("topic:%s %s", topic, qualifier)
	page := "/search/repositories?per_page=100&sort=updated&q=" + url.QueryEscape(query)

	var all []Repo
	for page != "" {
		var payload struct {
			Items []Repo `json:"items"`
		}
		resp, err := c.do(ctx, http.MethodGet, page, "", &payload)
		if err != nil {
			return nil, err
		}
		all = append(all, payload.Items...)
		page = nextPage(resp)
		// GitHub search caps results at 1000; stop well before that.
		if len(all) >= 300 {
			break
		}
	}
	return all, nil
}

// Asset is one file attached to a release.
type Asset struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	URL         string    `json:"url"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Release is a published GitHub release.
type Release struct {
	ID          int64     `json:"id"`
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []Asset   `json:"assets"`
}

// AssetNames lists the release's asset filenames, for the matcher.
func (r Release) AssetNames() []string {
	out := make([]string, 0, len(r.Assets))
	for _, a := range r.Assets {
		out = append(out, a.Name)
	}
	return out
}

// AssetByName finds an asset by exact filename.
func (r Release) AssetByName(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// ErrNoReleases is returned when a repo has published nothing yet.
var ErrNoReleases = errors.New("repository has no published releases")

// LatestRelease returns the repo's current latest release. GitHub's own
// definition is used, so drafts and prereleases are excluded.
func (c *Client) LatestRelease(ctx context.Context, owner, repo string) (Release, error) {
	var rel Release
	path := fmt.Sprintf("/repos/%s/%s/releases/latest", url.PathEscape(owner), url.PathEscape(repo))
	if _, err := c.do(ctx, http.MethodGet, path, "", &rel); err != nil {
		if IsNotFound(err) {
			return Release{}, ErrNoReleases
		}
		return Release{}, err
	}
	return rel, nil
}

// ListReleases returns up to limit published releases, newest first. Drafts
// are excluded; prereleases are kept so they can be rolled back to (PRD §10).
func (c *Client) ListReleases(ctx context.Context, owner, repo string, limit int) ([]Release, error) {
	if limit <= 0 {
		limit = 30
	}
	path := fmt.Sprintf("/repos/%s/%s/releases?per_page=%d", url.PathEscape(owner), url.PathEscape(repo), min(limit, 100))

	var out []Release
	page := path
	for page != "" && len(out) < limit {
		var batch []Release
		resp, err := c.do(ctx, http.MethodGet, page, "", &batch)
		if err != nil {
			if IsNotFound(err) {
				return nil, ErrNoReleases
			}
			return nil, err
		}
		for _, r := range batch {
			if r.Draft {
				continue
			}
			out = append(out, r)
		}
		page = nextPage(resp)
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Readme fetches a repo's readme as raw Markdown via the dedicated endpoint,
// so it is available before anything is installed (PRD §8, §12).
func (c *Client) Readme(ctx context.Context, owner, repo string) (string, error) {
	path := fmt.Sprintf("/repos/%s/%s/readme", url.PathEscape(owner), url.PathEscape(repo))
	endpoint := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github.raw+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", userAgent)
	if tok := c.Token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("contact github: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", newError(resp, decodeErrorMessage(resp.Body))
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", fmt.Errorf("read readme: %w", err)
	}
	return string(body), nil
}

// Progress reports download advancement. Total is 0 when unknown.
type Progress func(done, total int64)

// DownloadAsset streams a release asset into w. It requests the API asset URL
// with an octet-stream Accept header, which is what makes private-repo assets
// downloadable with the same token as everything else (PRD §7).
func (c *Client) DownloadAsset(ctx context.Context, asset Asset, w io.Writer, onProgress Progress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	req.Header.Set("User-Agent", userAgent)
	if tok := c.Token(); tok != "" {
		// net/http strips this on a cross-host redirect, which is exactly what
		// we want when GitHub hands off to its blob storage.
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	// Downloads can be large; give them their own generous ceiling.
	client := &http.Client{Timeout: 30 * time.Minute}
	if c.httpClient != nil {
		client.Transport = c.httpClient.Transport
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return newError(resp, decodeErrorMessage(resp.Body))
	}

	total := resp.ContentLength
	if total <= 0 {
		total = asset.Size
	}

	var done int64
	buf := make([]byte, 64<<10)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				return fmt.Errorf("write %s: %w", asset.Name, err)
			}
			done += int64(n)
			if onProgress != nil {
				onProgress(done, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("download %s: %w", asset.Name, readErr)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}
