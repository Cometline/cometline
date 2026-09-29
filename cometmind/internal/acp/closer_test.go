//go:build unix

package acp

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/process"
)

func TestCmdWaitCloserKillsDescendants(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	cmd := process.CommandContext(t.Context(), "sh", "-c", "sleep 30 & echo $! > child.pid; wait")
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = process.KillTree(cmd) })

	pid := readChildPID(t, pidFile)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })

	done := make(chan error, 1)
	go func() { done <- (&cmdWaitCloser{cmd: cmd}).Close() }()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("Close did not return")
	}
	if childStillAlive(pid) {
		t.Fatalf("descendant %d still alive", pid)
	}
}

func readChildPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(path)
		if err == nil {
			pid, convErr := strconv.Atoi(strings.TrimSpace(string(b)))
			if convErr == nil && pid > 0 {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("pid file was not written")
	return 0
}

func childStillAlive(pid int) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return syscall.Kill(pid, 0) == nil
}
