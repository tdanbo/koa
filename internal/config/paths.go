// Package config resolves the on-disk locations koa uses and exposes them in
// both absolute and display form (PRD §6, §16).
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Paths holds every directory and file koa touches.
type Paths struct {
	// BinDir is the koa-managed directory placed on the user's PATH. Every
	// installed binary lives here under its clean command name.
	BinDir string
	// ConfigDir holds state.json and the plaintext-token fallback.
	ConfigDir string
	// StateFile is the single local JSON file described in PRD §16.
	StateFile string
	// CacheDir is scratch space for downloads and extraction.
	CacheDir string
}

// Resolve computes the platform-correct paths. It does not create anything.
func Resolve() (Paths, error) {
	var p Paths

	home, err := os.UserHomeDir()
	if err != nil {
		return p, fmt.Errorf("locate home directory: %w", err)
	}

	cfg, err := os.UserConfigDir()
	if err != nil {
		return p, fmt.Errorf("locate config directory: %w", err)
	}
	p.ConfigDir = filepath.Join(cfg, "koa")
	p.StateFile = filepath.Join(p.ConfigDir, "state.json")

	cache, err := os.UserCacheDir()
	if err != nil {
		return p, fmt.Errorf("locate cache directory: %w", err)
	}
	p.CacheDir = filepath.Join(cache, "koa")

	p.BinDir = platformBinDir(home)

	return p, nil
}

// EnsureDirs creates the directories koa needs on first run.
func (p Paths) EnsureDirs() error {
	for _, dir := range []string{p.BinDir, p.ConfigDir, p.CacheDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return nil
}

// CommandName is the clean name an installed binary takes inside BinDir, so
// that typing it in any terminal runs it (PRD §6, §11).
func CommandName(repo string) string {
	return platformCommandName(repo)
}

// BinaryPath is the full path an installed repo's binary occupies.
func (p Paths) BinaryPath(repo string) string {
	return filepath.Join(p.BinDir, CommandName(repo))
}

// Display rewrites an absolute path into the shorthand the UI shows —
// `%LOCALAPPDATA%\koa\bin` on Windows, `~/.koa/bin` on Linux (PRD §5.5).
func Display(path string) string {
	if path == "" {
		return ""
	}
	return platformDisplay(path)
}
