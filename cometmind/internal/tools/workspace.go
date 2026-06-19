package tools

import (
	"github.com/cometline/cometmind/internal/tools/sandbox"
)

// Workspace is the execution sandbox for tools: a root directory plus the
// path-resolution policy that keeps file operations inside it.
type Workspace struct {
	Root string
}

// Resolve returns the absolute path for rel, ensuring it stays inside Root.
func (w Workspace) Resolve(rel string) (string, error) {
	return sandbox.ResolveWorkspacePath(w.Root, rel)
}
