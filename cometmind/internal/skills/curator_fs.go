package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ArchiveRoot is ~/.cometmind/skills/.archive.
func ArchiveRoot() (string, error) {
	mirror, err := MirrorRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(mirror, ".archive"), nil
}

// ArchiveManagedSkill moves a live managed skill into the archive directory.
func ArchiveManagedSkill(name string) error {
	name = strings.TrimSpace(name)
	if !ValidSkillName(name) {
		return fmt.Errorf("invalid skill name %q", name)
	}
	mirror, err := MirrorRoot()
	if err != nil {
		return err
	}
	src := filepath.Join(mirror, name)
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("skill %q is a symlink", name)
	}
	archive, err := ArchiveRoot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(archive, 0o700); err != nil {
		return err
	}
	dst := filepath.Join(archive, name)
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return os.Rename(src, dst)
}

// RestoreArchivedSkill moves an archived skill back to the live root.
func RestoreArchivedSkill(name string) error {
	name = strings.TrimSpace(name)
	if !ValidSkillName(name) {
		return fmt.Errorf("invalid skill name %q", name)
	}
	archive, err := ArchiveRoot()
	if err != nil {
		return err
	}
	src := filepath.Join(archive, name)
	if _, err := os.Stat(src); err != nil {
		return err
	}
	mirror, err := MirrorRoot()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(mirror, 0o700); err != nil {
		return err
	}
	dst := filepath.Join(mirror, name)
	if _, err := os.Lstat(dst); err == nil {
		return fmt.Errorf("skill %q already exists", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(src, dst)
}

// DeleteArchivedSkill removes an archived skill directory.
func DeleteArchivedSkill(name string) error {
	name = strings.TrimSpace(name)
	if !ValidSkillName(name) {
		return fmt.Errorf("invalid skill name %q", name)
	}
	archive, err := ArchiveRoot()
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(archive, name))
}

// ReadArchivedSkill reads one archived SKILL.md.
func ReadArchivedSkill(name string) (Skill, string, error) {
	archive, err := ArchiveRoot()
	if err != nil {
		return Skill{}, "", err
	}
	dir := filepath.Join(archive, strings.TrimSpace(name))
	skill, err := ReadSkill(dir)
	if err != nil {
		return Skill{}, "", err
	}
	raw, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return Skill{}, "", err
	}
	return skill, string(raw), nil
}
