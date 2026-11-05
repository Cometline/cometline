package hooks

import (
	"path/filepath"
	"strings"
)

// WorkspaceTrusted reports whether project-scoped hook config may run for workspaceRoot.
// Paths in trustedWorkspacePaths are normalized absolute paths (same bar as future run_command trust).
func WorkspaceTrusted(workspaceRoot string, trustedWorkspacePaths []string) bool {
	root, err := filepath.Abs(strings.TrimSpace(workspaceRoot))
	if err != nil || root == "" {
		return false
	}
	root = filepath.Clean(root)
	for _, raw := range trustedWorkspacePaths {
		p, err := filepath.Abs(strings.TrimSpace(raw))
		if err != nil {
			continue
		}
		p = filepath.Clean(p)
		if p == root {
			return true
		}
	}
	return false
}
