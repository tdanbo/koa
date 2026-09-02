package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/playdead/koa/internal/config"
)

func TestParseSelfUpdateRepo(t *testing.T) {
	cases := []struct {
		in        string
		wantOwner string
		wantRepo  string
	}{
		{"playdead/koa", "playdead", "koa"},
		{"  playdead/koa  ", "playdead", "koa"},
		{"", "", ""},
		{"noslash", "", ""},
		{"/koa", "", ""},
		{"playdead/", "", ""},
	}
	for _, tc := range cases {
		owner, repo := parseSelfUpdateRepo(tc.in)
		if owner != tc.wantOwner || repo != tc.wantRepo {
			t.Errorf("parseSelfUpdateRepo(%q) = (%q, %q), want (%q, %q)", tc.in, owner, repo, tc.wantOwner, tc.wantRepo)
		}
	}
}

// newSelfUpdateService builds a Service configured to self-update against
// gh's "playdead/koa" fixture, at the given running version.
func newSelfUpdateService(t *testing.T, gh *fakeGitHub, runningVersion string) *Service {
	t.Helper()
	root := t.TempDir()
	paths := config.Paths{
		BinDir:    filepath.Join(root, "bin"),
		ConfigDir: filepath.Join(root, "cfg"),
		StateFile: filepath.Join(root, "cfg", "state.json"),
		CacheDir:  filepath.Join(root, "cache"),
	}
	svc, err := New(Options{Version: runningVersion, Paths: paths, SelfUpdateRepo: "playdead/koa"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	svc.gh.SetBaseURL(gh.server.URL)
	host := newRecordingHost()
	svc.hostMu.Lock()
	svc.host = host
	svc.hostMu.Unlock()
	t.Cleanup(svc.stop)
	return svc
}

func TestCheckSelfUpdateSkipsDevBuilds(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.addRepo("koa", repoFixture{
		Owner: "playdead", Name: "koa",
		releases: []releaseFixture{{Tag: "v9.9.9"}},
	})

	svc := newSelfUpdateService(t, gh, "dev")
	info, err := svc.CheckSelfUpdate()
	if err != nil {
		t.Fatalf("CheckSelfUpdate: %v", err)
	}
	if info.Available || info.Configured {
		t.Errorf("a dev build should never report a self-update: %+v", info)
	}
}

func TestCheckSelfUpdateSkipsWhenNoRepoConfigured(t *testing.T) {
	root := t.TempDir()
	paths := config.Paths{
		BinDir:    filepath.Join(root, "bin"),
		ConfigDir: filepath.Join(root, "cfg"),
		StateFile: filepath.Join(root, "cfg", "state.json"),
		CacheDir:  filepath.Join(root, "cache"),
	}
	svc, err := New(Options{Version: "v1.0.0", Paths: paths})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(svc.stop)

	info, err := svc.CheckSelfUpdate()
	if err != nil {
		t.Fatalf("CheckSelfUpdate: %v", err)
	}
	if info.Available || info.Configured {
		t.Errorf("no self-update repo configured, want Available=false and Configured=false, got %+v", info)
	}
}

func TestCheckSelfUpdateDetectsNewerRelease(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.addRepo("koa", repoFixture{
		Owner: "playdead", Name: "koa",
		releases: []releaseFixture{{Tag: "v1.2.3", Published: "2026-08-30T10:00:00Z"}},
	})

	svc := newSelfUpdateService(t, gh, "v1.0.0")
	info, err := svc.CheckSelfUpdate()
	if err != nil {
		t.Fatalf("CheckSelfUpdate: %v", err)
	}
	if !info.Available {
		t.Fatalf("want an update available, got %+v", info)
	}
	if !info.Configured {
		t.Errorf("a running check should always be Configured, got %+v", info)
	}
	if info.Latest != "v1.2.3" {
		t.Errorf("latest = %q", info.Latest)
	}
	if info.Current != "v1.0.0" {
		t.Errorf("current = %q", info.Current)
	}

	// SelfUpdateStatus reflects the same result without another network call.
	if status := svc.SelfUpdateStatus(); !status.Available || status.Latest != "v1.2.3" {
		t.Errorf("SelfUpdateStatus = %+v", status)
	}
}

func TestCheckSelfUpdateUpToDate(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.addRepo("koa", repoFixture{
		Owner: "playdead", Name: "koa",
		releases: []releaseFixture{{Tag: "v1.0.0"}},
	})

	svc := newSelfUpdateService(t, gh, "v1.0.0")
	info, err := svc.CheckSelfUpdate()
	if err != nil {
		t.Fatalf("CheckSelfUpdate: %v", err)
	}
	if info.Available {
		t.Errorf("running version matches latest tag, want Available=false, got %+v", info)
	}
	if !info.Configured {
		t.Errorf("a repo with a matching latest release is still a Configured check, got %+v", info)
	}
}

func TestCheckSelfUpdateNoReleasesYet(t *testing.T) {
	gh := newFakeGitHub(t)
	gh.addRepo("koa", repoFixture{Owner: "playdead", Name: "koa"})

	svc := newSelfUpdateService(t, gh, "v1.0.0")
	info, err := svc.CheckSelfUpdate()
	if err != nil {
		t.Fatalf("CheckSelfUpdate: %v", err)
	}
	if info.Available {
		t.Errorf("a repo with no releases should never report an update: %+v", info)
	}
	if !info.Configured {
		t.Errorf("a repo with no releases yet is still a Configured check, got %+v", info)
	}
}

func TestSelfUpdateReplacesExecutableAndSurvivesRelaunchFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("covered on unix")
	}
	body := "not actually an executable\n"
	archive := tarGz(t, map[string]string{"koa": body})
	gh := newFakeGitHub(t)
	gh.addRepo("koa", repoFixture{
		Owner: "playdead", Name: "koa",
		releases: []releaseFixture{{
			Tag:    "v2.0.0",
			Assets: map[string][]byte{assetName("koa", "2.0.0"): archive},
		}},
	})

	svc := newSelfUpdateService(t, gh, "v1.0.0")

	dest := filepath.Join(t.TempDir(), "koa")
	if err := os.WriteFile(dest, []byte("old koa binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	// The staged asset is plain text, so exec-ing it as the relaunch step must
	// fail — selfUpdateTo should treat that as non-fatal since the binary swap
	// itself already succeeded.
	if err := svc.selfUpdateTo(context.Background(), dest); err != nil {
		t.Fatalf("selfUpdateTo: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if string(got) != body {
		t.Errorf("dest content = %q, want %q", got, body)
	}

	if status := svc.SelfUpdateStatus(); status.Current != "v2.0.0" {
		t.Errorf("SelfUpdateStatus.Current = %q, want v2.0.0", status.Current)
	}
}

func TestSelfExecutablePathResolvesTheTestBinary(t *testing.T) {
	path, err := selfExecutablePath()
	if err != nil {
		t.Fatalf("selfExecutablePath: %v", err)
	}
	if path == "" {
		t.Error("selfExecutablePath returned an empty path")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("resolved path does not exist: %v", err)
	}
}
