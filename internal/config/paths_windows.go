//go:build windows

package config

import (
	"os"
	"path/filepath"
	"strings"
)

// platformBinDir is %LOCALAPPDATA%\koa\bin (PRD §6, §5.5).
func platformBinDir(home string) string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		local = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(local, "koa", "bin")
}

// platformCommandName appends the .exe extension windows requires.
func platformCommandName(repo string) string {
	return repo + ".exe"
}

// platformDisplay rewrites a path under a known env var into its %VAR% form.
func platformDisplay(path string) string {
	for _, v := range []string{"LOCALAPPDATA", "APPDATA", "USERPROFILE"} {
		if base := os.Getenv(v); base != "" && strings.HasPrefix(path, base) {
			return "%" + v + "%" + path[len(base):]
		}
	}
	return path
}
