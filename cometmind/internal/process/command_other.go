//go:build !unix

package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
)

// CommandContext is the non-unix fallback. It cannot kill a process group.
// WaitDelay still bounds the pipe wait so a descendant cannot pin the caller.
func CommandContext(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, arg...) //nolint:gosec // callers pass the command
	cmd.WaitDelay = treeWaitDelay
	cmd.Cancel = func() error { return KillTree(cmd) }
	return cmd
}

// KillTree kills the direct child. A missing process is not an error.
func KillTree(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return os.ErrProcessDone
	}
	err := cmd.Process.Kill()
	if err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}
