package sandbox

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveWorkspacePath ensures rel resolves under root (after Clean+eval symlinks skipped for v1).
func ResolveWorkspacePath(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("path is empty")
	}
	root = filepath.Clean(root)
	if !filepath.IsAbs(rel) {
		rel = filepath.Clean(filepath.Join(root, rel))
	} else {
		rel = filepath.Clean(rel)
	}
	rootPrefix := root
	if !strings.HasSuffix(rootPrefix, string(filepath.Separator)) {
		rootPrefix += string(filepath.Separator)
	}
	// Allow exact root match
	if rel == root {
		return rel, nil
	}
	if !strings.HasPrefix(rel, rootPrefix) && rel != root {
		return "", fmt.Errorf("path escapes workspace: %s", rel)
	}
	return rel, nil
}
