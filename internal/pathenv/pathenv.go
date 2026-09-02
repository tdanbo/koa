// Package pathenv keeps the koa bin folder on the user's PATH, so an installed
// binary runs by typing its repo name in any terminal (PRD §6, §11).
package pathenv

import (
	"os"
	"path/filepath"
)

// Status describes how the koa bin folder currently relates to PATH.
type Status struct {
	// OnPath is true when the folder is on PATH for the running process.
	OnPath bool
	// Persisted is true when the folder is recorded in the user's durable
	// environment (shell profile on Linux, user registry on Windows).
	Persisted bool
	// NeedsRestart is true when koa has persisted the change but the user's
	// existing terminals will not see it until they are restarted.
	NeedsRestart bool
	// Detail is a short human sentence for the status footer and settings.
	Detail string
}

// InPath reports whether dir appears in the process PATH, comparing cleaned
// paths so trailing separators and `.` segments do not cause false negatives.
func InPath(dir string) bool {
	want := normalize(dir)
	if want == "" {
		return false
	}
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if normalize(entry) == want {
			return true
		}
	}
	return false
}

func normalize(p string) string {
	if p == "" {
		return ""
	}
	return platformFold(filepath.Clean(os.ExpandEnv(p)))
}
