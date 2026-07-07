package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cometline/cometmind/internal/processctl"
)

func withTempDataDir(t *testing.T) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "state")
	t.Setenv("COMETMIND_DATA_DIR", dir)
}

// TestHandleReloadSignalSeedsGenerationFromDisk is the regression test for the
// bug reported as: after any settings save, CometMind logs
//
//	"settings reload failed for 2 process(es): ... reload did not confirm within 35s"
//
// and falls back to a full restart every single time.
//
// Root cause: handleReloadSignal's generation counter started at 0 on every
// process incarnation, but `cometmind settings reload` reads the *on-disk*
// generation left by a previous incarnation as its baseline. If a prior serve
// process had already reached generation N and the process later restarted
// (packaged sidecar respawn, dev rebuild, etc.), the fresh process would start
// counting from 0 again and its first reload would persist generation 1 —
// which is not > the on-disk baseline N, so WaitForReloadGeneration could
// never observe it as "new" and every reload would hang until timeout.
func TestHandleReloadSignalSeedsGenerationFromDisk(t *testing.T) {
	withTempDataDir(t)

	// Simulate a previous process incarnation that already reached generation 5.
	if err := processctl.WriteReloadResult(processctl.ModeServe, processctl.ReloadResult{
		Generation: 5,
		Success:    true,
	}); err != nil {
		t.Fatalf("seed WriteReloadResult() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hupCh := make(chan os.Signal, 1)
	go handleReloadSignal(ctx, hupCh, processctl.ModeServe, func(context.Context) error {
		return nil
	})

	hupCh <- os.Interrupt // any os.Signal value; handleReloadSignal doesn't inspect it

	result, confirmed := processctl.WaitForReloadGeneration(processctl.ModeServe, 5, 2*time.Second)
	if !confirmed {
		t.Fatal("WaitForReloadGeneration() confirmed = false, want true (fresh incarnation must seed from disk and increment past 5)")
	}
	if result.Generation != 6 {
		t.Fatalf("result.Generation = %d, want 6 (seeded 5 + 1)", result.Generation)
	}
	if !result.Success {
		t.Fatalf("result.Success = false, want true")
	}
}

// TestHandleReloadSignalStartsFromOneWithNoPriorResult covers the ordinary
// first-ever-boot case (no reload.json on disk yet) to make sure the seeding
// change didn't break the ability to observe the very first reload.
func TestHandleReloadSignalStartsFromOneWithNoPriorResult(t *testing.T) {
	withTempDataDir(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hupCh := make(chan os.Signal, 1)
	go handleReloadSignal(ctx, hupCh, processctl.ModeServe, func(context.Context) error {
		return nil
	})

	hupCh <- os.Interrupt

	result, confirmed := processctl.WaitForReloadGeneration(processctl.ModeServe, 0, 2*time.Second)
	if !confirmed {
		t.Fatal("WaitForReloadGeneration() confirmed = false, want true")
	}
	if result.Generation != 1 {
		t.Fatalf("result.Generation = %d, want 1", result.Generation)
	}
}

// TestHandleReloadSignalRecordsFailure ensures a failing reload callback is
// still recorded (with Success=false and the error message) rather than
// silently dropped, so `settings reload` can surface the real cause instead
// of a generic timeout.
func TestHandleReloadSignalRecordsFailure(t *testing.T) {
	withTempDataDir(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hupCh := make(chan os.Signal, 1)
	go handleReloadSignal(ctx, hupCh, processctl.ModeServe, func(context.Context) error {
		return errTestReloadFailure
	})

	hupCh <- os.Interrupt

	result, confirmed := processctl.WaitForReloadGeneration(processctl.ModeServe, 0, 2*time.Second)
	if !confirmed {
		t.Fatal("WaitForReloadGeneration() confirmed = false, want true")
	}
	if result.Success {
		t.Fatal("result.Success = true, want false")
	}
	if result.Error != errTestReloadFailure.Error() {
		t.Fatalf("result.Error = %q, want %q", result.Error, errTestReloadFailure.Error())
	}
}

// TestHandleReloadSignalRecordsTimeoutWhenReloadHangs covers the second stuck
// save path: if Runtime.Reload or an MCP transport ignores context cancellation
// and never returns, `settings reload` still needs a new reload-result
// generation so Electron can stop showing an indefinite saving state and fall
// back to a restart with a concrete reason.
func TestHandleReloadSignalRecordsTimeoutWhenReloadHangs(t *testing.T) {
	withTempDataDir(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hupCh := make(chan os.Signal, 1)
	started := make(chan struct{})
	go handleReloadSignalWithTimeout(ctx, hupCh, processctl.ModeServe, func(context.Context) error {
		close(started)
		select {}
	}, 20*time.Millisecond)

	hupCh <- os.Interrupt
	<-started

	result, confirmed := processctl.WaitForReloadGeneration(processctl.ModeServe, 0, 2*time.Second)
	if !confirmed {
		t.Fatal("WaitForReloadGeneration() confirmed = false, want true")
	}
	if result.Success {
		t.Fatal("result.Success = true, want false")
	}
	if !strings.Contains(result.Error, "reload timed out") {
		t.Fatalf("result.Error = %q, want timeout message", result.Error)
	}
}

type testReloadError struct{ msg string }

func (e testReloadError) Error() string { return e.msg }

var errTestReloadFailure = testReloadError{msg: "boom: reload failed"}
