//go:build windows

package installer

// platformFinalizePermissions is a no-op on Windows, which has no +x bit.
func platformFinalizePermissions(path string) error {
	return nil
}
