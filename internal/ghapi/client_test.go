package ghapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New("test-token")
	c.SetBaseURL(srv.URL)
	return c
}

func TestCurrentUserSendsAuth(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q", got)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("User-Agent must be set")
		}
		fmt.Fprint(w, `{"login":"m-halvorsen","avatar_url":"https://x/y.png"}`)
	}))

	u, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser: %v", err)
	}
	if u.Login != "m-halvorsen" {
		t.Fatalf("login = %q", u.Login)
	}
}

func TestUnauthorizedIsClassified(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"Bad credentials"}`)
	}))

	_, err := c.CurrentUser(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !IsUnauthorized(err) {
		t.Fatalf("expected unauthorized classification, got %v", err)
	}
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Message != "Bad credentials" {
		t.Fatalf("message not surfaced: %v", err)
	}
}

func TestSSOErrorCapturesAuthorizationURL(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-GitHub-SSO", "required; url=https://github.com/orgs/playdead/sso?authorization_request=abc")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"Resource protected by organization SAML enforcement."}`)
	}))

	_, err := c.SearchTopic(context.Background(), "koa", "org:playdead")
	var apiErr *Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *Error, got %v", err)
	}
	if !apiErr.SSO {
		t.Fatal("SSO flag not set")
	}
	if apiErr.SSOURL != "https://github.com/orgs/playdead/sso?authorization_request=abc" {
		t.Fatalf("SSOURL = %q", apiErr.SSOURL)
	}
}

func TestRateLimitIsClassified(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1790000000")
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"API rate limit exceeded"}`)
	}))

	_, err := c.Orgs(context.Background())
	var apiErr *Error
	if !errors.As(err, &apiErr) || !apiErr.RateLimited {
		t.Fatalf("expected rate-limit classification, got %v", err)
	}
	if apiErr.ResetAt.IsZero() {
		t.Fatal("ResetAt not parsed")
	}
}

func TestSearchTopicBuildsQueryAndPaginates(t *testing.T) {
	var queries []string
	var srvURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/search/repositories", func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Get("q"))
		if r.URL.Query().Get("page") == "" {
			w.Header().Set("Link", fmt.Sprintf(`<%s/search/repositories?q=x&page=2>; rel="next"`, srvURL))
			fmt.Fprint(w, `{"items":[{"name":"dumpscope","full_name":"playdead/dumpscope","private":true,"owner":{"login":"playdead"}}]}`)
			return
		}
		fmt.Fprint(w, `{"items":[{"name":"assetlint","full_name":"playdead/assetlint","owner":{"login":"playdead"}}]}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	c := New("t")
	c.SetBaseURL(srv.URL)

	repos, err := c.SearchTopic(context.Background(), "koa", "org:playdead")
	if err != nil {
		t.Fatalf("SearchTopic: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("got %d repos, want 2 across both pages", len(repos))
	}
	if queries[0] != "topic:koa org:playdead" {
		t.Fatalf("query = %q", queries[0])
	}
	if repos[0].Visibility() != "Private" || repos[1].Visibility() != "Public" {
		t.Fatalf("visibility badges wrong: %q %q", repos[0].Visibility(), repos[1].Visibility())
	}
}

func TestLatestReleaseNoReleases(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
	}))

	_, err := c.LatestRelease(context.Background(), "playdead", "rigcheck")
	if !errors.Is(err, ErrNoReleases) {
		t.Fatalf("want ErrNoReleases, got %v", err)
	}
}

func TestListReleasesSkipsDrafts(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[
		  {"tag_name":"v1.1.0","draft":true},
		  {"tag_name":"v1.0.0","draft":false},
		  {"tag_name":"v0.9.2","draft":false,"prerelease":true}
		]`)
	}))

	rels, err := c.ListReleases(context.Background(), "o", "r", 10)
	if err != nil {
		t.Fatalf("ListReleases: %v", err)
	}
	if len(rels) != 2 {
		t.Fatalf("got %d releases, want drafts excluded and prereleases kept", len(rels))
	}
	if rels[0].TagName != "v1.0.0" {
		t.Fatalf("first tag = %q", rels[0].TagName)
	}
}

func TestReadmeRequestsRawMarkdown(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept"), "raw") {
			t.Errorf("Accept = %q, want a raw media type", r.Header.Get("Accept"))
		}
		fmt.Fprint(w, "# dumpscope\n\nOpen, diff and watch dumps.")
	}))

	md, err := c.Readme(context.Background(), "playdead", "dumpscope")
	if err != nil {
		t.Fatalf("Readme: %v", err)
	}
	if !strings.HasPrefix(md, "# dumpscope") {
		t.Fatalf("readme = %q", md)
	}
}

func TestDownloadAssetStreamsWithProgress(t *testing.T) {
	payload := bytes.Repeat([]byte("koa"), 5000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/octet-stream" {
			t.Errorf("Accept = %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		w.Write(payload)
	}))
	defer srv.Close()

	c := New("t")
	var buf bytes.Buffer
	var lastDone, lastTotal int64
	asset := Asset{Name: "a.tar.gz", Size: int64(len(payload)), URL: srv.URL + "/asset"}

	if err := c.DownloadAsset(context.Background(), asset, &buf, func(done, total int64) {
		lastDone, lastTotal = done, total
	}); err != nil {
		t.Fatalf("DownloadAsset: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), payload) {
		t.Fatal("downloaded bytes differ")
	}
	if lastDone != int64(len(payload)) || lastTotal != int64(len(payload)) {
		t.Fatalf("progress = %d/%d", lastDone, lastTotal)
	}
}

func TestAssetHelpers(t *testing.T) {
	rel := Release{Assets: []Asset{{Name: "a"}, {Name: "b"}}}
	if names := rel.AssetNames(); len(names) != 2 || names[1] != "b" {
		t.Fatalf("AssetNames = %v", names)
	}
	if _, ok := rel.AssetByName("b"); !ok {
		t.Fatal("AssetByName should find b")
	}
	if _, ok := rel.AssetByName("c"); ok {
		t.Fatal("AssetByName should not invent c")
	}
}

func TestCloneKeepsBaseURL(t *testing.T) {
	var seenAuth string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		fmt.Fprint(w, `{"login":"someone"}`)
	}))

	clone := c.Clone("other-token")
	if _, err := clone.CurrentUser(context.Background()); err != nil {
		t.Fatalf("clone should reach the same host: %v", err)
	}
	if seenAuth != "Bearer other-token" {
		t.Fatalf("clone Authorization = %q", seenAuth)
	}
	if c.Token() != "test-token" {
		t.Fatalf("original token changed to %q", c.Token())
	}
}
