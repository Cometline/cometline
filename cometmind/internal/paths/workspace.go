package paths

import (
	"os"
	"path/filepath"
)

// ResolveWorkspace returns the absolute, cleaned workspace root.
// If explicit is non-empty it is used; otherwise the current working directory.
func ResolveWorkspace(explicit string) (string, error) {
	if explicit != "" {
		return absDir(explicit)
	}
	return os.Getwd()
}

func absDir(path string) (string, error) {
	if path == "" {
		return os.Getwd()
	}
	return absPath(path)
}

func absPath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		path = filepath.Join(wd, path)
	}
	return filepath.Clean(path), nil
}
