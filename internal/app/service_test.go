package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/playdead/koa/internal/config"
	"github.com/playdead/koa/internal/store"
)

// fakeGitHub serves the endpoints koa touches, with per-repo release tables the
// tests can rewrite mid-run.
type fakeGitHub struct {
	mu sync.Mutex

	server *httptest.Server
	login  string
	orgs   []string
	// repos maps "owner/name" to its metadata.
	repos map[string]repoFixture
	// searchByScope maps a search qualifier to the repo keys it returns.
	searchByScope map[string][]string
	// scopeStatus lets a test make one scope fail.
	scopeStatus map[string]int
	scopeHeader map[string]string
}

type repoFixture struct {
	Owner       string
	Name        string
	Description string
	Private     bool
	Readme      string
	// releases are newest first.
	releases []releaseFixture
}

type releaseFixture struct {
	Tag       string
	Published string
	Assets    map[string][]byte
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	t.Helper()
	f := &fakeGitHub{
		login:         "m-halvorsen",
		repos:         map[string]repoFixture{},
		searchByScope: map[string][]string{},
		scopeStatus:   map[string]int{},
		scopeHeader:   map[string]string{},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeGitHub) addRepo(scope string, r repoFixture) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.ToLower(r.Owner + "/" + r.Name)
	f.repos[key] = r
	f.searchByScope[scope] = append(f.searchByScope[scope], key)
}

func (f *fakeGitHub) setReleases(owner, name string, releases []releaseFixture) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := strings.ToLower(owner + "/" + name)
	r := f.repos[key]
	r.releases = releases
	f.repos[key] = r
}

func (f *fakeGitHub) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := r.URL.Path
	switch {
	case path == "/user":
		json.NewEncoder(w).Encode(map[string]any{"login": f.login, "name": "M Halvorsen", "avatar_url": "https://avatars/x.png"})
		return
	case path == "/user/orgs":
		out := make([]map[string]string, 0, len(f.orgs))
		for _, o := range f.orgs {
			out = append(out, map[string]string{"login": o})
		}
		json.NewEncoder(w).Encode(out)
		return
	case path == "/search/repositories":
		scope := strings.TrimPrefix(r.URL.Query().Get("q"), "topic:koa ")
		if status, ok := f.scopeStatus[scope]; ok {
			if hdr := f.scopeHeader[scope]; hdr != "" {
				w.Header().Set("X-GitHub-SSO", hdr)
			}
			w.WriteHeader(status)
			fmt.Fprint(w, `{"message":"Resource protected by organization SAML enforcement."}`)
			return
		}
		items := []map[string]any{}
		for _, key := range f.searchByScope[scope] {
			items = append(items, f.repoJSON(f.repos[key]))
		}
		json.NewEncoder(w).Encode(map[string]any{"items": items})
		return
	case strings.HasPrefix(path, "/asset/"):
		f.serveAsset(w, strings.TrimPrefix(path, "/asset/"))
		return
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "repos" {
		http.NotFound(w, r)
		return
	}
	key := strings.ToLower(parts[1] + "/" + parts[2])
	repo, ok := f.repos[key]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"message":"Not Found"}`)
		return
	}
	rest := parts[3:]

	switch {
	case len(rest) == 0:
		json.NewEncoder(w).Encode(f.repoJSON(repo))
	case rest[0] == "readme":
		if repo.Readme == "" {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"Not Found"}`)
			return
		}
		fmt.Fprint(w, repo.Readme)
	case rest[0] == "releases" && len(rest) == 2 && rest[1] == "latest":
		if len(repo.releases) == 0 {
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprint(w, `{"message":"Not Found"}`)
			return
		}
		json.NewEncoder(w).Encode(f.releaseJSON(repo.releases[0]))
	case rest[0] == "releases":
		out := make([]map[string]any, 0, len(repo.releases))
		for _, rel := range repo.releases {
			out = append(out, f.releaseJSON(rel))
		}
		json.NewEncoder(w).Encode(out)
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeGitHub) repoJSON(r repoFixture) map[string]any {
	return map[string]any{
		"name":           r.Name,
		"full_name":      r.Owner + "/" + r.Name,
		"description":    r.Description,
		"private":        r.Private,
		"html_url":       "https://github.com/" + r.Owner + "/" + r.Name,
		"default_branch": "main",
		"owner":          map[string]string{"login": r.Owner},
	}
}

func (f *fakeGitHub) releaseJSON(rel releaseFixture) map[string]any {
	assets := []map[string]any{}
	for name, body := range rel.Assets {
		assets = append(assets, map[string]any{
			"id":   1,
			"name": name,
			"size": len(body),
			"url":  f.server.URL + "/asset/" + name,
		})
	}
	published := rel.Published
	if published == "" {
		published = "2026-08-30T10:00:00Z"
	}
	return map[string]any{"tag_name": rel.Tag, "published_at": published, "assets": assets}
}

func (f *fakeGitHub) serveAsset(w http.ResponseWriter, name string) {
	for _, repo := range f.repos {
		for _, rel := range repo.releases {
			if body, ok := rel.Assets[name]; ok {
				w.Header().Set("Content-Length", fmt.Sprint(len(body)))
				w.Write(body)
				return
			}
		}
	}
	http.NotFound(w, nil)
}

// recordingHost captures emitted events.
type recordingHost struct {
	mu     sync.Mutex
	events map[string]int
	urls   []string
}

func newRecordingHost() *recordingHost {
	return &recordingHost{events: map[string]int{}}
}

func (h *recordingHost) Emit(event string, _ any) {
	h.mu.Lock()
	h.events[event]++
	h.mu.Unlock()
}

func (h *recordingHost) OpenURL(u string) error {
	h.mu.Lock()
	h.urls = append(h.urls, u)
	h.mu.Unlock()
	return nil
}

func (h *recordingHost) ShowWindow() {}
func (h *recordingHost) Quit()       {}

func (h *recordingHost) count(event string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.events[event]
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// newService builds a Service pointed at the fake GitHub, already signed in.
func newService(t *testing.T, gh *fakeGitHub) (*Service, *recordingHost, config.Paths) {
	t.Helper()

	root := t.TempDir()
	paths := config.Paths{
		BinDir:    filepath.Join(root, "bin"),
		ConfigDir: filepath.Join(root, "cfg"),
		StateFile: filepath.Join(root, "cfg", "state.json"),
		CacheDir:  filepath.Join(root, "cache"),
	}

	svc, err := New(Options{Version: "test", ClientID: "cid", Paths: paths})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	svc.gh.SetBaseURL(gh.server.URL)

	host := newRecordingHost()
	svc.hostMu.Lock()
	svc.host = host
	svc.hostMu.Unlock()

	if _, err := svc.SignInWithToken("ghp_test"); err != nil {
		t.Fatalf("SignInWithToken: %v", err)
	}
	t.Cleanup(svc.stop)
	return svc, host, paths
}

func assetName(repo, version string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("%s-%s-amd64-windows.zip", repo, version)
	}
	return fmt.Sprintf("%s-%s-amd64-linux.tar.gz", repo, version)
}

func TestDiscoverClassifiesEveryStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive fixtures are tar.gz; run the suite on linux")
	}
	gh := newFakeGitHub(t)
	gh.orgs = []string{"playdead"}

	binary := tarGz(t, map[string]string{"assetlint": "#!/bin/sh\n"})

	gh.addRepo("user:m-halvorsen", repoFixture{
		Owner: "m-halvorsen", Name: "rigcheck", Description: "Rig checks", Private: true,
		releases: []releaseFixture{{Tag: "v0.6.1", Assets: map[string][]byte{"rigcheck-0.6.1.tar.gz": binary}}},
	})
	gh.addRepo("org:playdead", repoFixture{
		Owner: "playdead", Name: "assetlint", Description: "Validates assets",
		releases: []releaseFixture{{Tag: "v1.3.0", Assets: map[string][]byte{assetName("assetlint", "1.3.0"): binary}}},
	})
	gh.addRepo("org:playdead", repoFixture{
		Owner: "playdead", Name: "brandnew", Description: "No releases yet",
	})

	svc, _, _ := newService(t, gh)

	got, err := svc.Discover(true)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if !got.SignedIn {
		t.Fatal("expected SignedIn")
	}
	if len(got.Repos) != 3 {
		t.Fatalf("got %d repos, want 3: %+v", len(got.Repos), got.Repos)
	}
	if want := []string{"user:m-halvorsen", "org:playdead"}; fmt.Sprint(got.Scopes) != fmt.Sprint(want) {
		t.Errorf("scopes = %v, want %v", got.Scopes, want)
	}

	byName := map[string]Repo{}
	for _, r := range got.Repos {
		byName[r.Name] = r
	}

	if r := byName["assetlint"]; r.StatusKind != StatusNotInstalled || r.Action != "Install" || !r.CanInstall {
		t.Errorf("assetlint = %+v", r)
	}
	if r := byName["assetlint"]; r.Visibility != "Public" || r.OwnerScope != "playdead" {
		t.Errorf("assetlint badges = %q / %q", r.Visibility, r.OwnerScope)
	}
	if r := byName["rigcheck"]; r.StatusKind != StatusIncompatible || r.CanInstall || !r.Incompatible {
		t.Errorf("rigcheck = %+v", r)
	}
	if r := byName["rigcheck"]; !strings.Contains(r.IncompatibleReason, "amd64-linux.tar.gz") {
		t.Errorf("rigcheck reason = %q", r.IncompatibleReason)
	}
	if r := byName["rigcheck"]; r.Visibility != "Private" || r.OwnerScope != "you" {
		t.Errorf("rigcheck badges = %q / %q", r.Visibility, r.OwnerScope)
	}
	if r := byName["brandnew"]; r.StatusKind != StatusNoRelease {
		t.Errorf("brandnew = %+v", r)
	}

	// Alphabetical ordering keeps the list stable between refreshes.
	if got.Repos[0].Name != "assetlint" || got.Repos[2].Name != "rigcheck" {
		t.Errorf("unexpected order: %s, %s, %s", got.Repos[0].Name, got.Repos[1].Name, got.Repos[2].Name)
	}
}

func TestDiscoverSignedOutIsEmptyNotAnError(t *testing.T) {
	gh := newFakeGitHub(t)
	svc, _, _ := newService(t, gh)
	if err := svc.SignOut(); err != nil {
		t.Fatalf("SignOut: %v", err)
	}

	got, err := svc.Discover(true)
	if err != nil {
		t.Fatalf("Discover after sign-out: %v", err)
	}
	if got.SignedIn || len(got.Repos) != 0 {
		t.Fatalf("expected an empty signed-out result, got %+v", got)
	}
}

func TestDiscoverSurfacesSSOErrorPerScope(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.orgs = []string{"locked"}
	gh.addRepo("user:m-halvorsen", repoFixture{Owner: "m-halvorsen", Name: "mine"})
	gh.scopeStatus["org:locked"] = http.StatusForbidden
	gh.scopeHeader["org:locked"] = "required; url=https://github.com/orgs/locked/sso"

	svc, _, _ := newService(t, gh)

	got, err := svc.Discover(true)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got.Errors) != 1 {
		t.Fatalf("expected one scope error, got %+v", got.Errors)
	}
	e := got.Errors[0]
	if !e.SSO || e.SSOURL == "" || e.Scope != "org:locked" {
		t.Fatalf("scope error = %+v", e)
	}
	if !strings.Contains(e.Message, "SAML SSO") {
		t.Errorf("message = %q", e.Message)
	}
	// The healthy scope still produced results.
	if len(got.Repos) != 1 {
		t.Fatalf("expected the working scope to still return repos, got %d", len(got.Repos))
	}
}

func TestInstallUpdateRollbackUninstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive fixtures are tar.gz; run the suite on linux")
	}
	gh := newFakeGitHub(t)
	old := tarGz(t, map[string]string{"dumpscope": "old-binary"})
	gh.addRepo("user:m-halvorsen", repoFixture{
		Owner: "m-halvorsen", Name: "dumpscope", Description: "Dump viewer", Private: true,
		Readme:   "# dumpscope\n\nOpen and diff dumps.\n",
		releases: []releaseFixture{{Tag: "v0.9.2", Assets: map[string][]byte{assetName("dumpscope", "0.9.2"): old}}},
	})

	svc, host, paths := newService(t, gh)

	installed, err := svc.Install("m-halvorsen", "dumpscope", "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if installed.Version != "v0.9.2" {
		t.Fatalf("version = %q", installed.Version)
	}
	if installed.Description != "Dump viewer" || installed.Visibility != "Private" {
		t.Errorf("metadata not captured: %+v", installed)
	}
	if installed.Command != config.CommandName("dumpscope") {
		t.Errorf("command = %q", installed.Command)
	}
	bin := filepath.Join(paths.BinDir, config.CommandName("dumpscope"))
	if body, err := os.ReadFile(bin); err != nil || string(body) != "old-binary" {
		t.Fatalf("binary content = %q (err %v)", body, err)
	}
	if host.count(EventInstall) < 4 {
		t.Errorf("expected staged progress events, got %d", host.count(EventInstall))
	}
	if host.count(EventApps) == 0 {
		t.Error("expected an apps event after install")
	}

	// Installed list reflects it.
	list := svc.Installed()
	if len(list) != 1 || list[0].Name != "dumpscope" || list[0].HasUpdate {
		t.Fatalf("Installed = %+v", list)
	}

	// A new release appears; the update check should notice it.
	fresh := tarGz(t, map[string]string{"dumpscope": "new-binary"})
	gh.setReleases("m-halvorsen", "dumpscope", []releaseFixture{
		{Tag: "v1.0.0", Assets: map[string][]byte{assetName("dumpscope", "1.0.0"): fresh}},
		{Tag: "v0.9.2", Assets: map[string][]byte{assetName("dumpscope", "0.9.2"): old}},
	})

	checked, err := svc.CheckForUpdates("m-halvorsen", "dumpscope")
	if err != nil {
		t.Fatalf("CheckForUpdates: %v", err)
	}
	if !checked.HasUpdate || checked.LatestVersion != "v1.0.0" {
		t.Fatalf("update not detected: %+v", checked)
	}
	if checked.LastChecked.IsZero() {
		t.Error("LastChecked not stamped")
	}

	// Versions tab.
	versions, err := svc.Versions("m-halvorsen", "dumpscope")
	if err != nil {
		t.Fatalf("Versions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("versions = %+v", versions)
	}
	if versions[0].Tag != "v1.0.0" || !versions[0].IsLatest || versions[0].Action != "Update" {
		t.Errorf("latest row = %+v", versions[0])
	}
	if versions[1].Tag != "v0.9.2" || !versions[1].IsCurrent || versions[1].Action != "Reinstall" {
		t.Errorf("current row = %+v", versions[1])
	}
	if !versions[0].Compatible || versions[0].SizeBytes == 0 {
		t.Errorf("latest row should be compatible with a size: %+v", versions[0])
	}

	// Update to latest.
	updated, err := svc.Install("m-halvorsen", "dumpscope", "")
	if err != nil {
		t.Fatalf("update install: %v", err)
	}
	if updated.Version != "v1.0.0" || updated.HasUpdate {
		t.Fatalf("after update: %+v", updated)
	}
	if body, _ := os.ReadFile(bin); string(body) != "new-binary" {
		t.Fatalf("binary not replaced: %q", body)
	}

	// Roll back to the older tag; the newer release stays flagged.
	rolled, err := svc.Install("m-halvorsen", "dumpscope", "v0.9.2")
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rolled.Version != "v0.9.2" {
		t.Fatalf("rollback version = %q", rolled.Version)
	}
	if !rolled.HasUpdate || rolled.LatestVersion != "v1.0.0" {
		t.Errorf("rollback should still show an available update: %+v", rolled)
	}
	if body, _ := os.ReadFile(bin); string(body) != "old-binary" {
		t.Fatalf("rollback did not restore the old binary: %q", body)
	}

	// Uninstall.
	if err := svc.Uninstall("m-halvorsen", "dumpscope"); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(bin); !os.IsNotExist(err) {
		t.Error("binary survived uninstall")
	}
	if len(svc.Installed()) != 0 {
		t.Error("app still tracked after uninstall")
	}
}

func TestDiscoverReflectsInstallWithoutRefetch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive fixtures are tar.gz")
	}
	gh := newFakeGitHub(t)
	body := tarGz(t, map[string]string{"assetlint": "bin"})
	gh.addRepo("user:m-halvorsen", repoFixture{
		Owner: "m-halvorsen", Name: "assetlint",
		releases: []releaseFixture{{Tag: "v1.3.0", Assets: map[string][]byte{assetName("assetlint", "1.3.0"): body}}},
	})
	svc, _, _ := newService(t, gh)

	if _, err := svc.Discover(true); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Install("m-halvorsen", "assetlint", ""); err != nil {
		t.Fatalf("Install: %v", err)
	}

	// Cached discovery must still show the new install state.
	got, err := svc.Discover(false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Repos[0].StatusKind != StatusInstalled || got.Repos[0].Action != "Open" {
		t.Fatalf("cached row not updated: %+v", got.Repos[0])
	}

	if err := svc.Uninstall("m-halvorsen", "assetlint"); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Discover(false)
	if got.Repos[0].StatusKind != StatusNotInstalled || got.Repos[0].Action != "Install" {
		t.Fatalf("cached row not reverted: %+v", got.Repos[0])
	}
}

func TestInstallIncompatibleReportsThePattern(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.addRepo("user:m-halvorsen", repoFixture{
		Owner: "m-halvorsen", Name: "rigcheck",
		releases: []releaseFixture{{Tag: "v0.6.1", Assets: map[string][]byte{"rigcheck-0.6.1.tar.gz": []byte("x")}}},
	})
	svc, _, _ := newService(t, gh)

	_, err := svc.Install("m-halvorsen", "rigcheck", "")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "v0.6.1") || !strings.Contains(err.Error(), "amd64") {
		t.Fatalf("error should name the tag and pattern: %v", err)
	}
}

func TestAutoUpdateInstallsOnCheck(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive fixtures are tar.gz")
	}
	gh := newFakeGitHub(t)
	v1 := tarGz(t, map[string]string{"pace-cli": "v1"})
	gh.addRepo("user:m-halvorsen", repoFixture{
		Owner: "m-halvorsen", Name: "pace-cli",
		releases: []releaseFixture{{Tag: "v1.0.0", Assets: map[string][]byte{assetName("pace-cli", "1.0.0"): v1}}},
	})
	svc, _, paths := newService(t, gh)

	if _, err := svc.Install("m-halvorsen", "pace-cli", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetAutoUpdate("m-halvorsen", "pace-cli", true); err != nil {
		t.Fatalf("SetAutoUpdate: %v", err)
	}

	v2 := tarGz(t, map[string]string{"pace-cli": "v2"})
	gh.setReleases("m-halvorsen", "pace-cli", []releaseFixture{
		{Tag: "v2.0.0", Assets: map[string][]byte{assetName("pace-cli", "2.0.0"): v2}},
		{Tag: "v1.0.0", Assets: map[string][]byte{assetName("pace-cli", "1.0.0"): v1}},
	})

	got, err := svc.CheckForUpdates("m-halvorsen", "pace-cli")
	if err != nil {
		t.Fatalf("CheckForUpdates: %v", err)
	}
	if got.Version != "v2.0.0" {
		t.Fatalf("auto-update did not install: %+v", got)
	}
	body, _ := os.ReadFile(filepath.Join(paths.BinDir, config.CommandName("pace-cli")))
	if string(body) != "v2" {
		t.Fatalf("binary = %q", body)
	}
}

func TestCheckAllForUpdates(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("archive fixtures are tar.gz")
	}
	gh := newFakeGitHub(t)
	for _, name := range []string{"one", "two"} {
		body := tarGz(t, map[string]string{name: "bin"})
		gh.addRepo("user:m-halvorsen", repoFixture{
			Owner: "m-halvorsen", Name: name,
			releases: []releaseFixture{{Tag: "v1.0.0", Assets: map[string][]byte{assetName(name, "1.0.0"): body}}},
		})
	}
	svc, _, _ := newService(t, gh)
	for _, name := range []string{"one", "two"} {
		if _, err := svc.Install("m-halvorsen", name, ""); err != nil {
			t.Fatal(err)
		}
	}

	gh.setReleases("m-halvorsen", "two", []releaseFixture{
		{Tag: "v3.0.0", Assets: map[string][]byte{assetName("two", "3.0.0"): tarGz(t, map[string]string{"two": "bin3"})}},
	})

	apps, err := svc.CheckAllForUpdates()
	if err != nil {
		t.Fatalf("CheckAllForUpdates: %v", err)
	}
	byName := map[string]App{}
	for _, a := range apps {
		byName[a.Name] = a
	}
	if byName["one"].HasUpdate {
		t.Errorf("one should have no update: %+v", byName["one"])
	}
	if !byName["two"].HasUpdate || byName["two"].LatestVersion != "v3.0.0" {
		t.Errorf("two should show an update: %+v", byName["two"])
	}
}

func TestRepoDetailRendersReadme(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.addRepo("user:m-halvorsen", repoFixture{
		Owner: "m-halvorsen", Name: "dumpscope", Description: "Dump viewer",
		Readme:   "# dumpscope\n\nOpen and **diff** dumps.\n\n![shot](docs/shot.png)\n",
		releases: []releaseFixture{{Tag: "v1.0.0", Assets: map[string][]byte{assetName("dumpscope", "1.0.0"): []byte("x")}}},
	})
	svc, _, _ := newService(t, gh)

	detail, err := svc.RepoDetail("m-halvorsen", "dumpscope")
	if err != nil {
		t.Fatalf("RepoDetail: %v", err)
	}
	if detail.ReadmeError != "" {
		t.Fatalf("readme error: %s", detail.ReadmeError)
	}
	if !strings.Contains(detail.ReadmeHTML, "<strong>diff</strong>") {
		t.Errorf("readme not rendered: %s", detail.ReadmeHTML)
	}
	if !strings.Contains(detail.ReadmeHTML, "raw.githubusercontent.com/m-halvorsen/dumpscope/main/docs/shot.png") {
		t.Errorf("relative image not resolved: %s", detail.ReadmeHTML)
	}
	if detail.Repo.Description != "Dump viewer" {
		t.Errorf("description = %q", detail.Repo.Description)
	}
}

func TestRepoDetailMissingReadmeIsReported(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.addRepo("user:m-halvorsen", repoFixture{Owner: "m-halvorsen", Name: "bare"})
	svc, _, _ := newService(t, gh)

	detail, err := svc.RepoDetail("m-halvorsen", "bare")
	if err != nil {
		t.Fatalf("RepoDetail: %v", err)
	}
	if detail.ReadmeError != "This repository has no readme." {
		t.Fatalf("readme error = %q", detail.ReadmeError)
	}
}

func TestLaunchStreamsAndStops(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as the installed binary")
	}
	gh := newFakeGitHub(t)
	body := tarGz(t, map[string]string{"talker": "#!/bin/sh\necho hello from talker\n"})
	gh.addRepo("user:m-halvorsen", repoFixture{
		Owner: "m-halvorsen", Name: "talker",
		releases: []releaseFixture{{Tag: "v1.0.0", Assets: map[string][]byte{assetName("talker", "1.0.0"): body}}},
	})
	svc, _, _ := newService(t, gh)
	if _, err := svc.Install("m-halvorsen", "talker", ""); err != nil {
		t.Fatal(err)
	}

	proc, err := svc.Launch("m-halvorsen", "talker")
	if err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if proc.PID == 0 {
		t.Fatal("no pid recorded")
	}

	deadline := time.Now().Add(5 * time.Second)
	var lines []LogLine
	for time.Now().Before(deadline) {
		lines, _ = svc.ProcessLogs(proc.ID)
		found := false
		for _, l := range lines {
			if strings.Contains(l.Text, "hello from talker") {
				found = true
			}
		}
		if found {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	joined := ""
	for _, l := range lines {
		joined += l.Text + "\n"
	}
	if !strings.Contains(joined, "hello from talker") {
		t.Fatalf("stdout not captured:\n%s", joined)
	}

	if err := svc.StopProcess(proc.ID); err != nil {
		t.Fatalf("StopProcess: %v", err)
	}
	if err := svc.CloseProcess(proc.ID); err != nil {
		t.Fatalf("CloseProcess: %v", err)
	}
	if len(svc.ListProcesses()) != 0 {
		t.Fatal("process tab not closed")
	}
}

func TestLaunchMissingBinaryIsExplained(t *testing.T) {
	gh := newFakeGitHub(t)
	svc, _, _ := newService(t, gh)
	if err := svc.store.PutApp(store.App{Owner: "o", Repo: "ghost", Version: "v1"}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Launch("o", "ghost")
	if err == nil || !strings.Contains(err.Error(), "reinstall") {
		t.Fatalf("expected a reinstall hint, got %v", err)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	gh := newFakeGitHub(t)
	svc, _, _ := newService(t, gh)

	if got, err := svc.SetTheme("dark"); err != nil || got.Theme != "dark" {
		t.Fatalf("SetTheme = %+v, %v", got, err)
	}
	if _, err := svc.SetTheme("chartreuse"); err == nil {
		t.Fatal("expected an invalid theme to be rejected")
	}
	if got, err := svc.SetMinimizeToTray(true); err != nil || !got.MinimizeToTray {
		t.Fatalf("SetMinimizeToTray = %+v, %v", got, err)
	}
	if !NewShell(svc).MinimizeToTray() {
		t.Fatal("MinimizeToTray accessor disagrees with settings")
	}
	got, err := svc.SetManualOrgs([]string{" playdead ", "playdead", "", "Acme"})
	if err != nil {
		t.Fatalf("SetManualOrgs: %v", err)
	}
	if fmt.Sprint(got.ManualOrgs) != "[Acme playdead]" {
		t.Fatalf("manual orgs = %v, want deduplicated and sorted", got.ManualOrgs)
	}
	if got, err := svc.AcknowledgeTrust(); err != nil || !got.TrustAcknowledged {
		t.Fatalf("AcknowledgeTrust = %+v, %v", got, err)
	}

	// Settings survive a reload.
	reopened, err := store.Open(svc.store.Path())
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Settings().Theme != store.ThemeDark || !reopened.Settings().MinimizeToTray {
		t.Fatalf("settings not persisted: %+v", reopened.Settings())
	}
}

func TestManualOrgsAreSearched(t *testing.T) {
	gh := newFakeGitHub(t)
	// No orgs from the API, mirroring the fine-grained token case.
	gh.addRepo("org:typed-by-hand", repoFixture{Owner: "typed-by-hand", Name: "tool"})
	svc, _, _ := newService(t, gh)

	if _, err := svc.SetManualOrgs([]string{"typed-by-hand"}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Discover(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 1 || got.Repos[0].Name != "tool" {
		t.Fatalf("manual org was not searched: %+v", got.Repos)
	}
}

func TestBootstrapDescribesEnvironment(t *testing.T) {
	gh := newFakeGitHub(t)
	svc, _, paths := newService(t, gh)

	boot := svc.Bootstrap()
	if boot.Version != "test" || boot.Platform != runtime.GOOS {
		t.Fatalf("bootstrap = %+v", boot)
	}
	if boot.BinDirAbsolute != paths.BinDir {
		t.Errorf("binDir = %q, want %q", boot.BinDirAbsolute, paths.BinDir)
	}
	if !boot.DeviceFlowReady {
		t.Error("device flow should be ready when a client id is configured")
	}
	if !strings.Contains(boot.AssetPattern, "amd64") {
		t.Errorf("asset pattern = %q", boot.AssetPattern)
	}
	if !boot.Account.SignedIn || boot.Account.Login != "m-halvorsen" {
		t.Errorf("account = %+v", boot.Account)
	}
	if boot.Settings.Theme != "system" {
		t.Errorf("default theme = %q", boot.Settings.Theme)
	}
}

func TestSignInWithBadTokenIsRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"message":"Bad credentials"}`)
	}))
	defer srv.Close()

	root := t.TempDir()
	paths := config.Paths{
		BinDir:    filepath.Join(root, "bin"),
		ConfigDir: filepath.Join(root, "cfg"),
		StateFile: filepath.Join(root, "cfg", "state.json"),
		CacheDir:  filepath.Join(root, "cache"),
	}
	svc, err := New(Options{Version: "test", Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	svc.gh.SetBaseURL(srv.URL)

	if _, err := svc.SignInWithToken("nope"); err == nil {
		t.Fatal("expected the token to be rejected")
	}
	if svc.Account().SignedIn {
		t.Fatal("account should not be signed in")
	}
	if _, err := svc.SignInWithToken("  "); err == nil {
		t.Fatal("expected an empty token to be rejected")
	}
}

func TestSignInWithoutClientIDFailsClearly(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		BinDir:    filepath.Join(root, "bin"),
		ConfigDir: filepath.Join(root, "cfg"),
		StateFile: filepath.Join(root, "cfg", "state.json"),
		CacheDir:  filepath.Join(root, "cache"),
	}
	svc, err := New(Options{Version: "test", Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SignInWithGitHub(); err == nil || !strings.Contains(err.Error(), "KOA_GITHUB_CLIENT_ID") {
		t.Fatalf("expected guidance about the client id, got %v", err)
	}
	if svc.Bootstrap().DeviceFlowReady {
		t.Error("DeviceFlowReady should be false without a client id")
	}
}
