package skills

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Capabilities describes what UI/API operations are allowed for one skill.
type Capabilities struct {
	IsSymlink bool
	CanDelete bool
	CanExport bool
}

// MirrorRoot returns ~/.cometmind/skills.
func MirrorRoot() (string, error) {
	return expandPath("~/.cometmind/skills")
}

// SkillCapabilities reports export/delete rules for a discovered skill.
func SkillCapabilities(skill Skill) (Capabilities, error) {
	caps := Capabilities{CanExport: true}
	mirror, err := MirrorRoot()
	if err != nil {
		return caps, err
	}
	mirrorEntry := filepath.Join(mirror, skill.Name)
	info, err := os.Lstat(mirrorEntry)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if pathInfo, statErr := os.Lstat(skill.Path); statErr == nil && pathInfo.Mode()&os.ModeSymlink != 0 {
				caps.IsSymlink = true
			}
			caps.CanDelete = false
			return caps, nil
		}
		return caps, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		caps.IsSymlink = true
		caps.CanDelete = false
		return caps, nil
	}
	if info.IsDir() {
		caps.CanDelete = true
	}
	return caps, nil
}

// ExportSkill zips a skill directory (resolved through symlinks).
func ExportSkill(skill Skill) ([]byte, error) {
	root, err := filepath.EvalSymlinks(skill.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("skill path is not a directory")
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		w, err := zw.Create(rel)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		f.Close()
		return err
	})
	if err != nil {
		zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DeleteManagedSkill removes a non-symlink skill directory under ~/.cometmind/skills.
func DeleteManagedSkill(skill Skill) error {
	caps, err := SkillCapabilities(skill)
	if err != nil {
		return err
	}
	if !caps.CanDelete {
		return fmt.Errorf("skill %q cannot be deleted (external or symlink)", skill.Name)
	}
	mirror, err := MirrorRoot()
	if err != nil {
		return err
	}
	target := filepath.Join(mirror, skill.Name)
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return nil
}

// ValidSkillName matches Composer slash-command skill names.
func ValidSkillName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

// WriteSkill creates or updates SKILL.md under ~/.cometmind/skills/{name}/.
func WriteSkill(name, content string, overwrite bool) error {
	name = strings.TrimSpace(name)
	if !ValidSkillName(name) {
		return fmt.Errorf("invalid skill name %q", name)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("skill content is required")
	}
	fm, _, err := parseFrontmatter(content)
	if err != nil {
		return fmt.Errorf("invalid SKILL.md: %w", err)
	}
	if strings.TrimSpace(fm.Name) == "" || strings.TrimSpace(fm.Description) == "" {
		return fmt.Errorf("SKILL.md frontmatter must include name and description")
	}
	if strings.TrimSpace(fm.Name) != name {
		return fmt.Errorf("frontmatter name %q must match skill name %q", fm.Name, name)
	}

	mirror, err := MirrorRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(mirror, name)
	skillPath := filepath.Join(dir, "SKILL.md")
	if !overwrite {
		if _, err := os.Stat(skillPath); err == nil {
			return fmt.Errorf("skill %q already exists; delete it first or set overwrite=true", name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(skillPath, []byte(content), 0o644)
}
