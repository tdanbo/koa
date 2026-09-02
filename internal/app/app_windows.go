//go:build windows

package app

import "os/exec"

// platformRevealCommand opens dir in Windows Explorer.
func platformRevealCommand(dir string) *exec.Cmd {
	return exec.Command("explorer", dir)
}
