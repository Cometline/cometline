package tools

import "sync"

// fileLocks provides per-absolute-path mutual exclusion for file mutations.
// Prefer this over acquireWorkspaceLock for edit/write of a single file so
// concurrent sessions can edit different files in the same workspace.
var fileLocks sync.Map // key: string (absolute path) → *sync.Mutex

// acquireFileLock locks the mutex for absPath and returns a release function.
// Callers must defer the returned function.
func acquireFileLock(absPath string) func() {
	v, _ := fileLocks.LoadOrStore(absPath, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
