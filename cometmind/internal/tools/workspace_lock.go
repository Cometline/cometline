package tools

import "sync"

// workspaceLocks provides per-workspace-root mutual exclusion for filesystem
// mutations. It prevents concurrent agent sessions that share the same
// workspace from interleaving write_file or run_command operations, which
// would silently corrupt files or produce conflicting shell side-effects.
//
// Read-only tools (read_file, list_dir) do not need to acquire this lock;
// only mutating operations (write_file, run_command) do.
//
// The map grows monotonically (one entry per distinct workspace path ever
// used in this process lifetime). For a local single-user daemon this is
// acceptable: the number of distinct workspaces is small and bounded.
var workspaceLocks sync.Map // key: string (workspace root path) → *sync.Mutex

// acquireWorkspaceLock locks the mutex for root and returns a release function.
// Callers must defer the returned function to ensure the lock is always released.
//
//	release := acquireWorkspaceLock(root)
//	defer release()
func acquireWorkspaceLock(root string) func() {
	v, _ := workspaceLocks.LoadOrStore(root, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
