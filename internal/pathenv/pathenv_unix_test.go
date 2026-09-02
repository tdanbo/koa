//go:build !windows

package pathenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// A pre-existing .bashrc should be updated; .zshrc should not be created.
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("export EDITOR=vim\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(home, ".koa", "bin")

	first, err := Ensure(binDir)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if !first.Persisted {
		t.Fatal("expected the change to be persisted")
	}

	profile := filepath.Join(home, ".profile")
	before, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read .profile: %v", err)
	}
	if !strings.Contains(string(before), binDir) {
		t.Fatalf(".profile does not mention the bin dir:\n%s", before)
	}
	if strings.Count(string(before), markerStart) != 1 {
		t.Fatalf("expected exactly one koa block:\n%s", before)
	}

	if _, err := Ensure(binDir); err != nil {
		t.Fatalf("second Ensure: %v", err)
	}
	after, _ := os.ReadFile(profile)
	if string(after) != string(before) {
		t.Fatalf("second Ensure changed the file:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	bashrc, _ := os.ReadFile(filepath.Join(home, ".bashrc"))
	if !strings.Contains(string(bashrc), "export EDITOR=vim") {
		t.Error("existing .bashrc content was lost")
	}
	if !strings.Contains(string(bashrc), binDir) {
		t.Error("existing .bashrc was not updated")
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); err == nil {
		t.Error("koa should not create a .zshrc that did not exist")
	}
}

func TestEnsureRewritesStaleBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	profile := filepath.Join(home, ".profile")

	stale := "export A=1\n\n" + markerStart + "\nexport PATH=\"/old/koa/bin:$PATH\"\n" + markerEnd + "\n\nexport B=2\n"
	if err := os.WriteFile(profile, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(home, ".koa", "bin")
	if _, err := Ensure(binDir); err != nil {
		t.Fatalf("Ensure: %v", err)
	}

	got, _ := os.ReadFile(profile)
	if strings.Contains(string(got), "/old/koa/bin") {
		t.Errorf("stale entry survived:\n%s", got)
	}
	if !strings.Contains(string(got), "export A=1") || !strings.Contains(string(got), "export B=2") {
		t.Errorf("surrounding lines were lost:\n%s", got)
	}
	if strings.Count(string(got), markerStart) != 1 {
		t.Errorf("expected one block:\n%s", got)
	}
}

func TestCheckReportsRestartNeeded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", "/usr/bin:/bin")

	binDir := filepath.Join(home, ".koa", "bin")
	if got := Check(binDir); got.Persisted || got.OnPath {
		t.Fatalf("clean home should report nothing configured: %+v", got)
	}

	if _, err := Ensure(binDir); err != nil {
		t.Fatal(err)
	}
	got := Check(binDir)
	if !got.Persisted || got.OnPath || !got.NeedsRestart {
		t.Fatalf("after Ensure: %+v", got)
	}

	t.Setenv("PATH", binDir+":/usr/bin")
	got = Check(binDir)
	if !got.OnPath || got.NeedsRestart {
		t.Fatalf("with the dir on PATH: %+v", got)
	}
}

func TestInPathIgnoresTrailingSlash(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/home/u/.koa/bin/")
	if !InPath("/home/u/.koa/bin") {
		t.Fatal("trailing separator should not defeat the check")
	}
}
