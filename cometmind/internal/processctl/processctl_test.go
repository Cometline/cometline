package processctl

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withTempDataDir(t *testing.T) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "state")
	t.Setenv("COMETMIND_DATA_DIR", dir)
}

func TestReadReloadResultMissingFileReturnsZeroValue(t *testing.T) {
	withTempDataDir(t)

	result, err := ReadReloadResult(ModeServe)
	if err != nil {
		t.Fatalf("ReadReloadResult() error = %v", err)
	}
	if result.Generation != 0 || result.Success {
		t.Fatalf("result = %+v, want zero value", result)
	}
}

func TestWriteReadReloadResultRoundTrip(t *testing.T) {
	withTempDataDir(t)

	want := ReloadResult{
		Generation: 3,
		Success:    true,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := WriteReloadResult(ModeServe, want); err != nil {
		t.Fatalf("WriteReloadResult() error = %v", err)
	}

	got, err := ReadReloadResult(ModeServe)
	if err != nil {
		t.Fatalf("ReadReloadResult() error = %v", err)
	}
	if got != want {
		t.Fatalf("ReadReloadResult() = %+v, want %+v", got, want)
	}
}

func TestWriteReloadResultDoesNotLeaveTempFile(t *testing.T) {
	withTempDataDir(t)

	if err := WriteReloadResult(ModeServe, ReloadResult{Generation: 1, Success: true}); err != nil {
		t.Fatalf("WriteReloadResult() error = %v", err)
	}

	path, err := reloadResultPath(ModeServe)
	if err != nil {
		t.Fatalf("reloadResultPath() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected final file to exist: %v", err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("expected temp file to be renamed away, stat err = %v", err)
	}
}

func TestWriteReloadResultUnknownModeErrors(t *testing.T) {
	withTempDataDir(t)

	if err := WriteReloadResult("bogus", ReloadResult{}); err == nil {
		t.Fatal("WriteReloadResult() with unknown mode: want error, got nil")
	}
	if _, err := ReadReloadResult("bogus"); err == nil {
		t.Fatal("ReadReloadResult() with unknown mode: want error, got nil")
	}
}

func TestWaitForReloadGenerationObservesNewGeneration(t *testing.T) {
	withTempDataDir(t)

	if err := WriteReloadResult(ModeServe, ReloadResult{Generation: 1, Success: true}); err != nil {
		t.Fatalf("seed WriteReloadResult() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = WriteReloadResult(ModeServe, ReloadResult{Generation: 2, Success: true, FinishedAt: "later"})
		close(done)
	}()

	result, confirmed := WaitForReloadGeneration(ModeServe, 1, 2*time.Second)
	<-done
	if !confirmed {
		t.Fatal("WaitForReloadGeneration() confirmed = false, want true")
	}
	if result.Generation != 2 {
		t.Fatalf("result.Generation = %d, want 2", result.Generation)
	}
}

func TestWaitForReloadGenerationTimesOutWithoutNewGeneration(t *testing.T) {
	withTempDataDir(t)

	if err := WriteReloadResult(ModeServe, ReloadResult{Generation: 5, Success: true}); err != nil {
		t.Fatalf("seed WriteReloadResult() error = %v", err)
	}

	start := time.Now()
	result, confirmed := WaitForReloadGeneration(ModeServe, 5, 300*time.Millisecond)
	elapsed := time.Since(start)

	if confirmed {
		t.Fatal("WaitForReloadGeneration() confirmed = true, want false (no new generation written)")
	}
	if result.Generation != 5 {
		t.Fatalf("result.Generation = %d, want 5 (last known)", result.Generation)
	}
	if elapsed < 300*time.Millisecond {
		t.Fatalf("returned too early: elapsed = %v, want >= 300ms", elapsed)
	}
}

// TestWaitForReloadGenerationConcurrentCallsDoNotStackTimeouts documents the
// contract `cmd.settingsReloadCmd` relies on: waiting on N modes concurrently
// takes ~1x timeout in the worst case, not Nx. Before the fix, settingsReloadCmd
// awaited each mode's WaitForReloadGeneration sequentially, so with both
// `serve` and `gateway-discord` timing out (as happened when the generation
// counter regressed, see cmd/process_test.go), the CLI could take up to 2x
// reloadConfirmTimeout — long enough that Electron's own budget expired first
// and force-restarted the whole sidecar. WaitForReloadGeneration itself is
// mode-independent, so this test exercises the same concurrency pattern
// settingsReloadCmd now uses.
func TestWaitForReloadGenerationConcurrentCallsDoNotStackTimeouts(t *testing.T) {
	withTempDataDir(t)

	const timeout = 200 * time.Millisecond
	modes := []string{ModeServe, ModeGatewayDiscord}

	start := time.Now()
	results := make(chan bool, len(modes))
	for _, mode := range modes {
		go func(mode string) {
			_, confirmed := WaitForReloadGeneration(mode, 0, timeout)
			results <- confirmed
		}(mode)
	}
	for range modes {
		if confirmed := <-results; confirmed {
			t.Fatal("expected timeout (no generation written), got confirmed = true")
		}
	}
	elapsed := time.Since(start)

	// Sequential waiting would take ~2x timeout (400ms); concurrent waiting
	// should stay close to 1x timeout regardless of how many modes are waited on.
	if elapsed >= 2*timeout {
		t.Fatalf("elapsed = %v, want < %v (waits should run concurrently, not stack)", elapsed, 2*timeout)
	}
}

func TestWriteMetadataUsesAtomicWrite(t *testing.T) {
	withTempDataDir(t)

	if err := WriteMetadata(ModeServe); err != nil {
		t.Fatalf("WriteMetadata() error = %v", err)
	}
	defer RemoveMetadata(ModeServe)

	state, err := ReadState(ModeServe)
	if err != nil {
		t.Fatalf("ReadState() error = %v", err)
	}
	if !state.Present {
		t.Fatal("state.Present = false, want true")
	}
	if state.Metadata.PID != os.Getpid() {
		t.Fatalf("state.Metadata.PID = %d, want %d", state.Metadata.PID, os.Getpid())
	}
}
