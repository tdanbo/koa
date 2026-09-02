package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s, path
}

func TestOpenMissingFileUsesDefaults(t *testing.T) {
	s, _ := newStore(t)
	if got := s.Settings().Theme; got != ThemeSystem {
		t.Fatalf("theme = %q, want %q", got, ThemeSystem)
	}
	if s.Settings().MinimizeToTray {
		t.Fatal("minimize-to-tray should default to off")
	}
	if len(s.Apps()) != 0 {
		t.Fatal("expected no tracked apps")
	}
}

func TestRoundTrip(t *testing.T) {
	s, path := newStore(t)

	if err := s.UpdateSettings(func(cfg *Settings) {
		cfg.Theme = ThemeDark
		cfg.MinimizeToTray = true
		cfg.ManualOrgs = []string{"playdead"}
	}); err != nil {
		t.Fatalf("UpdateSettings: %v", err)
	}

	app := App{Owner: "playdead", Repo: "dumpscope", Version: "v0.9.2", InstalledAt: time.Now().UTC().Truncate(time.Second)}
	if err := s.PutApp(app); err != nil {
		t.Fatalf("PutApp: %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := reopened.Settings().Theme; got != ThemeDark {
		t.Fatalf("theme = %q after reload", got)
	}
	if got := reopened.Settings().ManualOrgs; len(got) != 1 || got[0] != "playdead" {
		t.Fatalf("manual orgs = %v", got)
	}
	got, ok := reopened.App("PlayDead", "DumpScope")
	if !ok {
		t.Fatal("lookup should be case-insensitive")
	}
	if got.Version != "v0.9.2" {
		t.Fatalf("version = %q", got.Version)
	}
}

func TestMutateAndDelete(t *testing.T) {
	s, _ := newStore(t)
	if err := s.PutApp(App{Owner: "o", Repo: "r", Version: "v1"}); err != nil {
		t.Fatalf("PutApp: %v", err)
	}

	ok, err := s.MutateApp("o", "r", func(a *App) { a.AutoUpdate = true; a.LatestVersion = "v2" })
	if err != nil || !ok {
		t.Fatalf("MutateApp ok=%v err=%v", ok, err)
	}
	a, _ := s.App("o", "r")
	if !a.AutoUpdate || !a.HasUpdate() {
		t.Fatalf("mutation not applied: %+v", a)
	}

	ok, err = s.MutateApp("o", "missing", func(*App) {})
	if err != nil || ok {
		t.Fatalf("MutateApp on missing app ok=%v err=%v", ok, err)
	}

	if err := s.DeleteApp("O", "R"); err != nil {
		t.Fatalf("DeleteApp: %v", err)
	}
	if _, ok := s.App("o", "r"); ok {
		t.Fatal("app should be gone")
	}
}

func TestOpenCorruptFileErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("expected a parse error rather than a silent reset")
	}
}

func TestAppsSortedByRepo(t *testing.T) {
	s, _ := newStore(t)
	for _, name := range []string{"shaderpack", "assetlint", "dumpscope"} {
		if err := s.PutApp(App{Owner: "playdead", Repo: name}); err != nil {
			t.Fatal(err)
		}
	}
	got := s.Apps()
	want := []string{"assetlint", "dumpscope", "shaderpack"}
	for i, w := range want {
		if got[i].Repo != w {
			t.Fatalf("apps[%d] = %q, want %q", i, got[i].Repo, w)
		}
	}
}
