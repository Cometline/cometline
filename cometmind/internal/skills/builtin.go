package skills

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/paths"
)

//go:embed builtin/*/SKILL.md
var builtinSkills embed.FS

// BuiltinRoot returns the materialized root for bundled skills.
func BuiltinRoot() (string, error) {
	return paths.BuiltinSkillsDir()
}

func ensureBuiltinSkills() (string, error) {
	root, err := BuiltinRoot()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	err = fs.WalkDir(builtinSkills, "builtin", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || filepath.Base(path) != "SKILL.md" {
			return nil
		}
		rel := strings.TrimPrefix(path, "builtin/")
		target := filepath.Join(root, filepath.FromSlash(rel))
		raw, err := builtinSkills.ReadFile(path)
		if err != nil {
			return err
		}
		if existing, err := os.ReadFile(target); err == nil && string(existing) == string(raw) {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o644)
	})
	if err != nil {
		return "", err
	}
	return root, nil
}
