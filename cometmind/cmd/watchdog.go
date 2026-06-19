package cmd

import (
	"context"
	"os"
	"time"
)

// parentWatchInterval is how often the watchdog checks whether its parent
// process is still alive. Kept short so an orphaned sidecar shuts down
// promptly and releases the TCP port and SQLite WAL lock.
const parentWatchInterval = 1 * time.Second

// watchParent cancels the returned context's work by invoking stop when the
// process that launched this one disappears.
//
// When CometMind runs as a sidecar under Electron, a normal quit triggers a
// graceful SIGTERM. But if the Electron parent dies abnormally (crash,
// SIGKILL, force quit, or OS logout), no signal is delivered and the Go
// process is orphaned: it keeps holding 127.0.0.1 and the SQLite WAL lock.
//
// The watchdog detects orphaning by watching the parent PID. On Unix an
// orphaned child is reparented (to PID 1 or a subreaper), so a change away
// from the original parent PID means the launcher is gone. When that happens
// we call stop, which cancels the serve context and runs the same graceful
// shutdown path as SIGTERM.
func watchParent(ctx context.Context, stop context.CancelFunc) {
	originalPPID := os.Getppid()

	go func() {
		ticker := time.NewTicker(parentWatchInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if os.Getppid() != originalPPID {
					stop()
					return
				}
			}
		}
	}()
}
