package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config controls local Agent Skills discovery.
type Config struct {
	Enabled         bool
	Roots           []string
	IncludeOpenCode bool
	IncludeClaude   bool
}

// Skill is one discovered Agent Skill directory.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Source      string `json:"source"`
	Internal    bool   `json:"internal"`
}

// Registry is an immutable index of discovered skills.
type Registry struct {
	Skills []Skill
	byName map[string]Skill
	Errors []string
}

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Metadata    struct {
		Internal bool `yaml:"internal"`
	} `yaml:"metadata"`
}

// DefaultRoots returns the CometMind/OpenCode/Claude roots plus workspace-local roots.
func DefaultRoots(workspaceRoot string, cfg Config) []string {
	roots := make([]string, 0, 5+len(cfg.Roots))
	roots = append(roots, cfg.Roots...)
	roots = append(roots, "~/.cometmind/skills")
	if workspaceRoot != "" {
		roots = append(roots, filepath.Join(workspaceRoot, ".agents", "skills"))
		roots = append(roots, filepath.Join(workspaceRoot, ".claude", "skills"))
	}
	if cfg.IncludeOpenCode {
		roots = append(roots, "~/.config/opencode/skills")
	}
	if cfg.IncludeClaude {
		roots = append(roots, "~/.claude/skills")
	}
	return roots
}

// Discover builds a registry from configured skill roots. Earlier roots win on duplicate names.
func Discover(workspaceRoot string, cfg Config) Registry {
	if !cfg.Enabled {
		return Registry{byName: map[string]Skill{}}
	}
	reg := Registry{byName: map[string]Skill{}}
	seenRoot := map[string]bool{}
	for _, root := range DefaultRoots(workspaceRoot, cfg) {
		expanded, err := expandPath(root)
		if err != nil {
			reg.Errors = append(reg.Errors, err.Error())
			continue
		}
		if expanded == "" || seenRoot[expanded] {
			continue
		}
		seenRoot[expanded] = true
		discoverRoot(expanded, &reg)
	}
	sort.SliceStable(reg.Skills, func(i, j int) bool { return reg.Skills[i].Name < reg.Skills[j].Name })
	return reg
}

func discoverRoot(root string, reg *Registry) {
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		reg.Errors = append(reg.Errors, fmt.Sprintf("read skills root %q: %v", root, err))
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		skill, err := ReadSkill(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			reg.Errors = append(reg.Errors, err.Error())
			continue
		}
		if _, exists := reg.byName[skill.Name]; exists {
			continue
		}
		reg.byName[skill.Name] = skill
		reg.Skills = append(reg.Skills, skill)
	}
}

// ReadSkill parses one skill directory.
func ReadSkill(dir string) (Skill, error) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return Skill{}, err
	}
	raw, err := os.ReadFile(filepath.Join(resolved, "SKILL.md"))
	if err != nil {
		return Skill{}, err
	}
	fm, _, err := parseFrontmatter(string(raw))
	if err != nil {
		return Skill{}, fmt.Errorf("parse %s: %w", filepath.Join(resolved, "SKILL.md"), err)
	}
	name := strings.TrimSpace(fm.Name)
	desc := strings.TrimSpace(fm.Description)
	if name == "" || desc == "" {
		return Skill{}, fmt.Errorf("skill %q missing required name or description", resolved)
	}
	return Skill{Name: name, Description: desc, Path: resolved, Source: resolved, Internal: fm.Metadata.Internal}, nil
}

// Find returns a skill by name.
func (r Registry) Find(name string) (Skill, bool) {
	if r.byName == nil {
		r.byName = map[string]Skill{}
		for _, skill := range r.Skills {
			r.byName[skill.Name] = skill
		}
	}
	s, ok := r.byName[strings.TrimSpace(name)]
	return s, ok
}

// SkillMarkdown returns the full SKILL.md contents for a discovered skill.
func (r Registry) SkillMarkdown(name string) (Skill, string, error) {
	skill, ok := r.Find(name)
	if !ok {
		return Skill{}, "", fmt.Errorf("unknown skill: %s", name)
	}
	raw, err := os.ReadFile(filepath.Join(skill.Path, "SKILL.md"))
	if err != nil {
		return Skill{}, "", err
	}
	return skill, string(raw), nil
}

// ReadSkillFile reads a file inside one skill directory.
func (r Registry) ReadSkillFile(name, rel string) (Skill, string, error) {
	skill, ok := r.Find(name)
	if !ok {
		return Skill{}, "", fmt.Errorf("unknown skill: %s", name)
	}
	clean := filepath.Clean(strings.TrimSpace(rel))
	if clean == "." || strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
		return Skill{}, "", fmt.Errorf("skill file path must stay inside the skill")
	}
	p := filepath.Join(skill.Path, clean)
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return Skill{}, "", err
	}
	if !strings.HasPrefix(resolved, skill.Path+string(os.PathSeparator)) && resolved != skill.Path {
		return Skill{}, "", fmt.Errorf("skill file path escapes the skill")
	}
	raw, err := os.ReadFile(resolved)
	if err != nil {
		return Skill{}, "", err
	}
	return skill, string(raw), nil
}

// PromptIndex returns compact system-prompt text for discovered skills.
func (r Registry) PromptIndex() string {
	if len(r.Skills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## Available Skills\nUse `load_skill` when the user's task matches one of these skills. Load the full skill before following it; do not assume details from the summary alone.\n")
	for _, skill := range r.Skills {
		if skill.Internal {
			continue
		}
		b.WriteString("- ")
		b.WriteString(skill.Name)
		b.WriteString(": ")
		b.WriteString(skill.Description)
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

// SyncMirror creates symlinks in mirrorRoot for discovered skills.
func (r Registry) SyncMirror(mirrorRoot string) ([]string, []string, error) {
	expanded, err := expandPath(mirrorRoot)
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(expanded, 0o700); err != nil {
		return nil, nil, err
	}
	created := []string{}
	skipped := []string{}
	for _, skill := range r.Skills {
		target := filepath.Join(expanded, skill.Name)
		info, err := os.Lstat(target)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				_ = os.Remove(target)
			} else {
				skipped = append(skipped, skill.Name)
				continue
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return created, skipped, err
		}
		if filepath.Clean(skill.Path) == filepath.Clean(target) {
			skipped = append(skipped, skill.Name)
			continue
		}
		if err := os.Symlink(skill.Path, target); err != nil {
			return created, skipped, err
		}
		created = append(created, skill.Name)
	}
	return created, skipped, nil
}

func parseFrontmatter(raw string) (frontmatter, string, error) {
	var fm frontmatter
	if !strings.HasPrefix(raw, "---\n") && !strings.HasPrefix(raw, "---\r\n") {
		return fm, raw, fmt.Errorf("missing YAML frontmatter")
	}
	lines := strings.Split(raw, "\n")
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return fm, raw, fmt.Errorf("unterminated YAML frontmatter")
	}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &fm); err != nil {
		return fm, raw, err
	}
	return fm, strings.Join(lines[end+1:], "\n"), nil
}

func expandPath(p string) (string, error) {
	p = strings.TrimSpace(os.ExpandEnv(p))
	if p == "" {
		return "", nil
	}
	if strings.HasPrefix(p, "~/") || p == "~" {
		home, err := userHome()
		if err != nil {
			return "", err
		}
		if p == "~" {
			p = home
		} else {
			p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	}
	return filepath.Abs(p)
}

func userHome() (string, error) {
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return home, nil
	}
	return os.UserHomeDir()
}
