//go:build !windows

package pathenv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// markerStart and markerEnd delimit the block koa owns inside a shell profile,
// so the edit is recognisable and can be rewritten idempotently.
const (
	markerStart = "# >>> koa >>>"
	markerEnd   = "# <<< koa <<<"
)

// profileCandidates are the shell startup files koa will write into. `.profile`
// covers login shells; the others cover the interactive shells that skip it.
var profileCandidates = []string{".profile", ".bashrc", ".zshrc"}

// Ensure adds binDir to PATH persistently. It rewrites a marker-delimited block
// in the user's shell profiles, so running it repeatedly is a no-op.
func Ensure(binDir string) (Status, error) {
	st := Status{OnPath: InPath(binDir)}

	home, err := os.UserHomeDir()
	if err != nil {
		return st, fmt.Errorf("locate home directory: %w", err)
	}

	block := fmt.Sprintf("%s\n# koa keeps installed binaries here and on PATH.\nexport PATH=\"%s:$PATH\"\n%s\n", markerStart, binDir, markerEnd)

	var written []string
	for i, name := range profileCandidates {
		path := filepath.Join(home, name)
		// Always maintain .profile; only touch the others if they exist, so koa
		// does not create a .zshrc for someone who does not use zsh.
		if i > 0 {
			if _, err := os.Stat(path); err != nil {
				continue
			}
		}
		changed, err := writeBlock(path, block)
		if err != nil {
			return st, err
		}
		st.Persisted = true
		if changed {
			written = append(written, "~/"+name)
		}
	}

	switch {
	case st.OnPath:
		st.Detail = "on your PATH"
	case len(written) > 0:
		st.NeedsRestart = true
		st.Detail = "added to " + strings.Join(written, ", ") + " — open a new terminal to pick it up"
	case st.Persisted:
		st.NeedsRestart = true
		st.Detail = "configured in ~/.profile — open a new terminal to pick it up"
	default:
		st.Detail = "not on your PATH"
	}
	return st, nil
}

// writeBlock replaces koa's marker block in path, appending it if absent.
// It reports whether the file's contents changed.
func writeBlock(path, block string) (bool, error) {
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	updated := replaceBlock(string(existing), block)
	if updated == string(existing) {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

// replaceBlock swaps an existing koa block for a new one, or appends it.
func replaceBlock(content, block string) string {
	start := strings.Index(content, markerStart)
	end := strings.Index(content, markerEnd)
	if start >= 0 && end > start {
		tail := content[end+len(markerEnd):]
		tail = strings.TrimPrefix(tail, "\n")
		return content[:start] + block + tail
	}
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if content != "" {
		content += "\n"
	}
	return content + block
}

// platformFold leaves a cleaned path as-is; unix paths are case-sensitive.
func platformFold(p string) string {
	return p
}

// Check reports the current state without modifying anything.
func Check(binDir string) Status {
	st := Status{OnPath: InPath(binDir)}
	home, err := os.UserHomeDir()
	if err == nil {
		for _, name := range profileCandidates {
			raw, err := os.ReadFile(filepath.Join(home, name))
			if err == nil && strings.Contains(string(raw), markerStart) {
				st.Persisted = true
				break
			}
		}
	}
	st.NeedsRestart = st.Persisted && !st.OnPath
	switch {
	case st.OnPath:
		st.Detail = "on your PATH"
	case st.NeedsRestart:
		st.Detail = "configured — open a new terminal to pick it up"
	default:
		st.Detail = "not on your PATH"
	}
	return st
}
