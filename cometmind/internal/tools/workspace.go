package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/tools/sandbox"
)

// Workspace is the execution sandbox for tools: a root directory plus the
// path-resolution policy that keeps file operations inside it.
type Workspace struct {
	Root string
}

const runtimePrefix = "@runtime"

const (
	runtimeToolOutputMount = "tool-output"
	runtimeTmpMount        = "tmp"
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
	if mount != runtimeToolOutputMount && mount != runtimeTmpMount {
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
	if mount != runtimeTmpMount {
		return "", fmt.Errorf("runtime mount is read-only: %s", mount)
	}
	root, err := runtimeMountRoot(mount)
	if err != nil {
		return "", err
	}
	return sandbox.ResolveWorkspacePath(root, subpath)
}

// IsRuntimePath reports whether rel uses the explicit runtime alias.
func IsRuntimePath(rel string) bool {
	_, _, ok := runtimeMount(rel)
	return ok
}

func runtimeMount(rel string) (mount, subpath string, ok bool) {
	clean := filepath.ToSlash(strings.TrimSpace(rel))
	if clean == runtimePrefix || clean == runtimePrefix+"/" || !strings.HasPrefix(clean, runtimePrefix+"/") {
		return "", "", false
	}
	parts := strings.SplitN(strings.TrimPrefix(clean, runtimePrefix+"/"), "/", 2)
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
