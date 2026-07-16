package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/tools/sandbox"
)

// Workspace is the FileWorkspace module: path policy, runtime mounts, and
// lock helpers for filesystem tools. Tools are thin adapters over this seam.
type Workspace struct {
	Root string
}

const RuntimePrefix = "@runtime"

const (
	runtimeToolOutputMount = "tool-output"
	runtimeTmpMount        = "tmp"
	runtimeWikiMount       = "wiki"
)

var (
	fileLocks      sync.Map // abs path → *sync.Mutex
	workspaceLocks sync.Map // workspace root → *sync.Mutex
)

// Resolve returns the absolute path for rel, ensuring it stays inside Root.
func (w Workspace) Resolve(rel string) (string, error) {
	return sandbox.ResolveWorkspacePath(w.Root, rel)
}

// ResolveReadable resolves workspace paths plus explicitly exposed runtime
// mounts. It does not expose ~/.cometmind as a general filesystem root.
func (w Workspace) ResolveReadable(rel string) (string, error) {
	mount, subpath, ok := runtimeMount(rel)
	if !ok {
		return w.Resolve(rel)
	}
	if mount != runtimeToolOutputMount && mount != runtimeTmpMount && mount != runtimeWikiMount {
		return "", fmt.Errorf("runtime mount is not readable: %s", mount)
	}
	root, err := runtimeMountRoot(mount)
	if err != nil {
		return "", err
	}
	return sandbox.ResolveWorkspacePath(root, subpath)
}

// ResolveWritable resolves normal workspace paths and the shared runtime tmp
// mount. Tool output is intentionally read-only after it is created.
func (w Workspace) ResolveWritable(rel string) (string, error) {
	mount, subpath, ok := runtimeMount(rel)
	if !ok {
		return w.Resolve(rel)
	}
	if mount != runtimeTmpMount && mount != runtimeWikiMount {
		return "", fmt.Errorf("runtime mount is read-only: %s", mount)
	}
	root, err := runtimeMountRoot(mount)
	if err != nil {
		return "", err
	}
	return sandbox.ResolveWorkspacePath(root, subpath)
}

// ResolveSearchRoot resolves a path that may be a readable runtime mount and
// returns the display prefix needed to make search results usable by other tools.
func (w Workspace) ResolveSearchRoot(rel string) (root, displayPrefix string, err error) {
	if !IsRuntimePath(rel) {
		root, err = w.Resolve(rel)
		return root, "", err
	}
	root, err = w.ResolveReadable(rel)
	return root, strings.TrimSuffix(filepath.ToSlash(strings.TrimSpace(rel)), "/"), err
}

// DisplayPath returns the path shown in tool results (workspace-relative or
// the original @runtime alias when applicable).
func (w Workspace) DisplayPath(abs, inputRel string) string {
	if IsRuntimePath(inputRel) {
		return filepath.ToSlash(inputRel)
	}
	rel := strings.TrimPrefix(strings.TrimPrefix(abs, w.Root), string(filepath.Separator))
	return filepath.ToSlash(rel)
}

// LockFile acquires a per-file mutex for absPath. Prefer this for edit/write
// of a single file so concurrent sessions can edit different files.
func (w Workspace) LockFile(absPath string) (unlock func()) {
	return acquireFileLock(absPath)
}

// LockWorkspace acquires a per-workspace-root mutex. Prefer this for
// run_command (and other workspace-wide side effects).
func (w Workspace) LockWorkspace() (unlock func()) {
	return acquireWorkspaceLock(w.Root)
}

// MountDocs returns the coding-prompt lines describing runtime mounts.
func MountDocs() string {
	return "The managed runtime mounts are available without exposing secrets: " +
		"@runtime/tool-output is read-only, @runtime/tmp is shared read/write across sessions, and " +
		"@runtime/wiki is a persistent shared read/write knowledge wiki. " +
		"Use these aliases instead of guessing ~/.cometmind paths."
}

// IsRuntimePath reports whether rel uses the explicit runtime alias.
func IsRuntimePath(rel string) bool {
	_, _, ok := runtimeMount(rel)
	return ok
}

func runtimeMount(rel string) (mount, subpath string, ok bool) {
	clean := filepath.ToSlash(strings.TrimSpace(rel))
	if clean == RuntimePrefix || clean == RuntimePrefix+"/" || !strings.HasPrefix(clean, RuntimePrefix+"/") {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(clean, RuntimePrefix+"/"), "/", 2)
	mount = parts[0]
	subpath = "."
	if len(parts) == 2 && parts[1] != "" {
		subpath = parts[1]
	}
	return mount, subpath, true
}

func runtimeMountRoot(mount string) (string, error) {
	dataDir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	name := mount
	if mount == runtimeTmpMount {
		name = "agent-tmp"
	}
	root := filepath.Join(dataDir, name)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	return root, nil
}

// acquireFileLock locks the mutex for absPath and returns a release function.
func acquireFileLock(absPath string) func() {
	v, _ := fileLocks.LoadOrStore(absPath, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// acquireWorkspaceLock locks the mutex for root and returns a release function.
func acquireWorkspaceLock(root string) func() {
	v, _ := workspaceLocks.LoadOrStore(root, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}
