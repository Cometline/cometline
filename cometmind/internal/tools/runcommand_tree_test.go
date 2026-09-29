//go:build unix

package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunCommandTimeoutKillsDescendants(t *testing.T) {
	root := t.TempDir()
	pidFile := filepath.Join(root, "child.pid")
	tool := RunCommand{Workspace: Workspace{Root: root}}
	payload, err := json.Marshal(map[string]any{
		"command":     "sleep 30 & echo $! > child.pid; wait",
		"timeout_sec": 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	res, err := tool.Execute(t.Context(), payload)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 8*time.Second {
		t.Fatalf("Execute returned after %s", time.Since(start))
	}
	if res.OK || !strings.Contains(res.Output, "timed out") {
		t.Fatalf("result = %+v", res)
	}
	pid := readPIDFile(t, pidFile)
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
	if descendantAlive(pid) {
		t.Fatalf("descendant %d still alive", pid)
	}
}

func readPIDFile(t *testing.T, path string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		t.Fatalf("pid file %q", b)
	}
	return pid
}

func descendantAlive(pid int) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return syscall.Kill(pid, 0) == nil
}
