//go:build !windows

package auth

// platformKeyringLabel names the credential store the reference and Settings
// copy refer to on Linux (PRD §5.3, §7).
func platformKeyringLabel() string {
	return "the system keyring"
}
