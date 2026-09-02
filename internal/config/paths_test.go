package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestResolveProducesPlatformPaths(t *testing.T) {
	paths, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	for name, value := range map[string]string{
		"BinDir":    paths.BinDir,
		"ConfigDir": paths.ConfigDir,
		"StateFile": paths.StateFile,
		"CacheDir":  paths.CacheDir,
	} {
		if value == "" {
			t.Errorf("%s is empty", name)
		}
		if !filepath.IsAbs(value) {
			t.Errorf("%s = %q, want an absolute path", name, value)
		}
	}
	if filepath.Base(paths.StateFile) != "state.json" {
		t.Errorf("state file = %q", paths.StateFile)
	}
	if !strings.Contains(paths.ConfigDir, "koa") {
		t.Errorf("config dir = %q, want it scoped to koa", paths.ConfigDir)
	}
	if runtime.GOOS != "windows" && !strings.HasSuffix(paths.BinDir, filepath.Join(".koa", "bin")) {
		t.Errorf("bin dir = %q, want ~/.koa/bin on unix", paths.BinDir)
	}
}

func TestEnsureDirsAndBinaryPath(t *testing.T) {
	root := t.TempDir()
	paths := Paths{
		BinDir:    filepath.Join(root, "bin"),
		ConfigDir: filepath.Join(root, "cfg"),
		StateFile: filepath.Join(root, "cfg", "state.json"),
		CacheDir:  filepath.Join(root, "cache"),
	}
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("EnsureDirs: %v", err)
	}
	// Running it twice must be a no-op.
	if err := paths.EnsureDirs(); err != nil {
		t.Fatalf("second EnsureDirs: %v", err)
	}
	for _, dir := range []string{paths.BinDir, paths.ConfigDir, paths.CacheDir} {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Errorf("%s was not created: %v", dir, err)
		}
	}

	want := filepath.Join(paths.BinDir, CommandName("dumpscope"))
	if got := paths.BinaryPath("dumpscope"); got != want {
		t.Errorf("BinaryPath = %q, want %q", got, want)
	}
}

func TestCommandNameMatchesPlatform(t *testing.T) {
	got := CommandName("dumpscope")
	if runtime.GOOS == "windows" {
		if got != "dumpscope.exe" {
			t.Fatalf("CommandName = %q on windows", got)
		}
		return
	}
	if got != "dumpscope" {
		t.Fatalf("CommandName = %q on unix", got)
	}
}

func TestDisplayShortensHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the %VAR% form is exercised on windows")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory in this environment")
	}
	if got := Display(filepath.Join(home, ".koa", "bin")); got != filepath.Join("~", ".koa", "bin") {
		t.Errorf("Display = %q, want a ~-prefixed path", got)
	}
	if got := Display("/usr/local/bin"); got != "/usr/local/bin" {
		t.Errorf("Display rewrote an unrelated path: %q", got)
	}
	if got := Display(""); got != "" {
		t.Errorf("Display(\"\") = %q", got)
	}
}
