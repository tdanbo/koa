//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

// configureProcessGroup puts the child in its own process group so koa can
// signal the whole tree, not just the entry-point binary.
func configureProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminate asks the process group to shut down cleanly.
func terminate(cmd *exec.Cmd) error {
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
		// The group may already be gone, or setpgid may not have applied;
		// fall back to signalling the process itself.
		return cmd.Process.Signal(syscall.SIGTERM)
	}
	return nil
}

// kill forcibly ends the process group.
func kill(cmd *exec.Cmd) error {
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
