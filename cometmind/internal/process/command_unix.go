//go:build unix

package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// CommandContext is exec.CommandContext for a process tree, not a single PID.
// The child is a process-group leader. Cancelling ctx sends SIGKILL to that
// group. Callers must not replace SysProcAttr.
func CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...) //nolint:gosec // callers pass the command
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = treeWaitDelay
	cmd.Cancel = func() error { return KillTree(cmd) }
	return cmd
}

// KillTree signals the process group started by CommandContext. If the
// process is not a group leader, it falls back to killing that process.
// A missing process is not an error.
func KillTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return os.ErrProcessDone
	}
	pid := cmd.Process.Pid
	if pid > 0 {
		err := syscall.Kill(-pid, syscall.SIGKILL)
		if err == nil {
			return nil
		}
	}
	err := cmd.Process.Kill()
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return err
}
