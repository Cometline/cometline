package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReviewWrite is one skill the review fork created or updated.
type ReviewWrite struct {
	Name        string
	Action      string // created | updated
	Description string
}

// WriteReviewSkill creates or overwrites a live skill for the review fork.
// Creates always receive origin self-improvement. Overwrites are rejected
// unless the existing skill already has that origin. Overlap with another
// self-improvement skill must be an overwrite; any other blocking match
// writes nothing.
func WriteReviewSkill(name, content string, overwrite bool) (ReviewWrite, error) {
	name = strings.TrimSpace(name)
	if !ValidSkillName(name) {
		return ReviewWrite{}, fmt.Errorf("invalid skill name %q", name)
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ReviewWrite{}, fmt.Errorf("skill content is required")
	}
	fm, err := parseFrontmatter(content)
	if err != nil {
		return ReviewWrite{}, fmt.Errorf("invalid SKILL.md: %w", err)
	}
	if strings.TrimSpace(fm.Name) == "" || strings.TrimSpace(fm.Description) == "" {
		return ReviewWrite{}, fmt.Errorf("SKILL.md frontmatter must include name and description")
	}
	if strings.TrimSpace(fm.Name) != name {
		return ReviewWrite{}, fmt.Errorf("frontmatter name %q must match skill name %q", fm.Name, name)
	}
	origin := frontmatterOrigin(fm)
	if origin != "" && origin != OriginSelfImprovement {
		return ReviewWrite{}, fmt.Errorf("review cannot set origin %q", origin)
	}
	content, err = withOrigin(content, OriginSelfImprovement)
	if err != nil {
		return ReviewWrite{}, err
	}

	mirror, err := MirrorRoot()
	if err != nil {
		return ReviewWrite{}, err
	}
	dir := filepath.Join(mirror, name)
	if info, lstatErr := os.Lstat(dir); lstatErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return ReviewWrite{}, fmt.Errorf("skill %q is a symlink; the review cannot edit it", name)
	} else if lstatErr != nil && !errors.Is(lstatErr, os.ErrNotExist) {
		return ReviewWrite{}, lstatErr
	}

	action := "created"
	if overwrite {
		action = "updated"
		existing, readErr := ReadSkill(dir)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				return ReviewWrite{}, fmt.Errorf("skill %q does not exist", name)
			}
			return ReviewWrite{}, readErr
		}
		if !IsSelfImprovement(existing) {
			return ReviewWrite{}, fmt.Errorf("skill %q origin is not self-improvement; the review cannot edit it", name)
		}
	} else {
		if _, statErr := os.Stat(filepath.Join(dir, "SKILL.md")); statErr == nil {
			return ReviewWrite{}, fmt.Errorf("skill %q already exists; overwrite the self-improvement skill instead of creating another", name)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return ReviewWrite{}, statErr
		}
		if err := rejectReviewOverlap(name, strings.TrimSpace(fm.Description)); err != nil {
			return ReviewWrite{}, err
		}
	}

	if err := WriteSkill(name, content, overwrite); err != nil {
		return ReviewWrite{}, err
	}
	return ReviewWrite{
		Name:        name,
		Action:      action,
		Description: strings.TrimSpace(fm.Description),
	}, nil
}

func rejectReviewOverlap(name, description string) error {
	related, err := FindRelatedManagedSkills(name, description)
	if err != nil {
		return err
	}
	var selfName, otherName string
	for _, match := range related {
		if match.Score < overlapBlockThreshold {
			continue
		}
		if match.Location == OverlapLocationLive && match.Origin == OriginSelfImprovement {
			if selfName == "" {
				selfName = match.Name
			}
			continue
		}
		if otherName == "" {
			otherName = match.Name
		}
	}
	if otherName != "" {
		return fmt.Errorf("skill %q overlaps %q, which is not a self-improvement skill; stop without writing", name, otherName)
	}
	if selfName != "" {
		return fmt.Errorf("skill %q overlaps self-improvement skill %q; overwrite that skill with overwrite=true instead of creating another", name, selfName)
	}
	return nil
}
