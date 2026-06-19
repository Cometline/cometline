package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveWorkspacePath ensures rel resolves under root after following symlinks
// through existing path segments. This prevents workspace escape via symlinks
// such as workspace/link -> /etc.
func ResolveWorkspacePath(root, rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("path is empty")
	}
	resolvedRoot, err := filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(rel) {
		rel = filepath.Clean(filepath.Join(resolvedRoot, rel))
	} else {
		rel = filepath.Clean(rel)
	}
	if rel == resolvedRoot {
		return resolvedRoot, nil
	}
	if !isWithinRoot(resolvedRoot, rel) {
		return "", fmt.Errorf("path escapes workspace: %s", rel)
	}

	relFromRoot, err := filepath.Rel(resolvedRoot, rel)
	if err != nil {
		return "", err
	}
	parts := strings.Split(relFromRoot, string(filepath.Separator))
	current := resolvedRoot
	for i, part := range parts {
		next := filepath.Join(current, part)
		info, err := os.Lstat(next)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return filepath.Join(current, filepath.Join(parts[i:]...)), nil
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			next, err = filepath.EvalSymlinks(next)
			if err != nil {
				return "", err
			}
		}
		next = filepath.Clean(next)
		if !isWithinRoot(resolvedRoot, next) {
			return "", fmt.Errorf("path escapes workspace: %s", next)
		}
		current = next
	}
	return current, nil
}

func isWithinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
