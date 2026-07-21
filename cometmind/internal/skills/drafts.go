package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Draft is a proposed Agent Skill that is not discoverable until promoted.
type Draft struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// DraftRoot returns ~/.cometmind/skill-drafts.
func DraftRoot() (string, error) {
	return expandPath("~/.cometmind/skill-drafts")
}

// ListDrafts returns pending skill drafts. Drafts live outside discovery roots.
func ListDrafts() ([]Draft, error) {
	root, err := DraftRoot()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []Draft{}, nil
	}
	if err != nil {
		return nil, err
	}
	drafts := make([]Draft, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		draft, err := ReadDraft(entry.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, draft)
	}
	sort.SliceStable(drafts, func(i, j int) bool { return drafts[i].UpdatedAt > drafts[j].UpdatedAt })
	return drafts, nil
}

// ReadDraft reads one draft's SKILL.md metadata.
func ReadDraft(name string) (Draft, error) {
	name = strings.TrimSpace(name)
	if !ValidSkillName(name) {
		return Draft{}, fmt.Errorf("invalid draft name %q", name)
	}
	root, err := DraftRoot()
	if err != nil {
		return Draft{}, err
	}
	dir := filepath.Join(root, name)
	resolved, err := filepath.EvalSymlinks(dir)
	if errors.Is(err, os.ErrNotExist) {
		return Draft{}, err
	}
	if err != nil {
		return Draft{}, err
	}
	rootResolved, err := filepath.EvalSymlinks(root)
	if errors.Is(err, os.ErrNotExist) {
		rootResolved = root
	} else if err != nil {
		return Draft{}, err
	}
	if resolved != rootResolved && !strings.HasPrefix(resolved, rootResolved+string(os.PathSeparator)) {
		return Draft{}, fmt.Errorf("draft path escapes draft root")
	}
	raw, err := os.ReadFile(filepath.Join(resolved, "SKILL.md"))
	if err != nil {
		return Draft{}, err
	}
	fm, _, err := parseFrontmatter(string(raw))
	if err != nil {
		return Draft{}, fmt.Errorf("parse %s: %w", filepath.Join(resolved, "SKILL.md"), err)
	}
	info, err := os.Stat(filepath.Join(resolved, "SKILL.md"))
	if err != nil {
		return Draft{}, err
	}
	return Draft{Name: strings.TrimSpace(fm.Name), Description: strings.TrimSpace(fm.Description), Path: resolved, CreatedAt: info.ModTime().UnixMilli(), UpdatedAt: info.ModTime().UnixMilli()}, nil
}

// DraftMarkdown returns one draft's SKILL.md contents.
func DraftMarkdown(name string) (Draft, string, error) {
	draft, err := ReadDraft(name)
	if err != nil {
		return Draft{}, "", err
	}
	raw, err := os.ReadFile(filepath.Join(draft.Path, "SKILL.md"))
	if err != nil {
		return Draft{}, "", err
	}
	return draft, string(raw), nil
}

// WriteDraft writes a proposed skill to ~/.cometmind/skill-drafts.
func WriteDraft(name, content string, overwrite bool) error {
	name = strings.TrimSpace(name)
	if !ValidSkillName(name) {
		return fmt.Errorf("invalid draft name %q", name)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("draft content is required")
	}
	fm, _, err := parseFrontmatter(content)
	if err != nil {
		return fmt.Errorf("invalid SKILL.md: %w", err)
	}
	if strings.TrimSpace(fm.Name) == "" || strings.TrimSpace(fm.Description) == "" {
		return fmt.Errorf("SKILL.md frontmatter must include name and description")
	}
	if strings.TrimSpace(fm.Name) != name {
		return fmt.Errorf("frontmatter name %q must match draft name %q", fm.Name, name)
	}
	root, err := DraftRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, name)
	skillPath := filepath.Join(dir, "SKILL.md")
	if !overwrite {
		if _, err := os.Stat(skillPath); err == nil {
			return fmt.Errorf("draft %q already exists", name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(skillPath, []byte(content), 0o644)
}

// SkillMarkdownDescription returns the YAML description field from SKILL.md content.
func SkillMarkdownDescription(content string) string {
	fm, _, err := parseFrontmatter(content)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(fm.Description)
}

// PromoteDraft validates and copies a draft into ~/.cometmind/skills, then removes it.
func PromoteDraft(name string) error {
	_, content, err := DraftMarkdown(name)
	if err != nil {
		return err
	}
	if err := WriteSkill(name, content, false); err != nil {
		return err
	}
	return RejectDraft(name)
}

// RejectDraft deletes a pending draft.
func RejectDraft(name string) error {
	name = strings.TrimSpace(name)
	if !ValidSkillName(name) {
		return fmt.Errorf("invalid draft name %q", name)
	}
	root, err := DraftRoot()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, name)
	if _, err := os.Stat(dir); err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return nil
}
