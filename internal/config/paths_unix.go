//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"strings"
)

// platformBinDir is ~/.koa/bin (PRD §6, §5.5).
func platformBinDir(home string) string {
	return filepath.Join(home, ".koa", "bin")
}

// platformCommandName leaves repo as-is; unix has no executable extension.
func platformCommandName(repo string) string {
	return repo
}

// platformDisplay shortens a path under $HOME to a ~-prefixed form.
func platformDisplay(path string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
