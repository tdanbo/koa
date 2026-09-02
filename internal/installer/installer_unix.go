//go:build !windows

package installer

import "os"

// platformFinalizePermissions makes the staged binary executable, since
// extraction does not preserve the +x bit through every archive format.
func platformFinalizePermissions(path string) error {
	return os.Chmod(path, 0o755)
}
