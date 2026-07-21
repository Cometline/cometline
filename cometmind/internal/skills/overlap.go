package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Overlap thresholds (strict first; raise to loosen false positives).
const (
	overlapNameJaccardThreshold = 0.5
	overlapDescJaccardThreshold = 0.4
	overlapBlockThreshold       = 0.4
	overlapMaxResults           = 5
)

const (
	OverlapLocationLive  = "live"
	OverlapLocationDraft = "draft"

	OverlapKindExactName          = "exact_name"
	OverlapKindSimilarName        = "similar_name"
	OverlapKindSimilarDescription = "similar_description"
)

// RelatedSkill is one managed live skill or pending draft that may overlap a proposal.
type RelatedSkill struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Location    string  `json:"location"` // live | draft
	Kind        string  `json:"kind"`
	Score       float64 `json:"score"`
}

type managedSkillEntry struct {
	Name        string
	Description string
	Location    string
}

// FindRelatedManagedSkills compares name+description against ~/.cometmind/skills
// and ~/.cometmind/skill-drafts only (not workspace or builtin discovery roots).
func FindRelatedManagedSkills(name, description string) ([]RelatedSkill, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" && description == "" {
		return nil, nil
	}

	entries, err := listManagedSkillEntries()
	if err != nil {
		return nil, err
	}

	var related []RelatedSkill
	for _, entry := range entries {
		if hit, ok := scoreRelated(name, description, entry); ok {
			related = append(related, hit)
		}
	}
	sort.SliceStable(related, func(i, j int) bool {
		if related[i].Score == related[j].Score {
			return related[i].Name < related[j].Name
		}
		return related[i].Score > related[j].Score
	})
	if len(related) > overlapMaxResults {
		related = related[:overlapMaxResults]
	}
	return related, nil
}

// ShouldBlockOverlap reports whether related entries are strong enough to gate a new draft.
func ShouldBlockOverlap(related []RelatedSkill) bool {
	for _, r := range related {
		if r.Score >= overlapBlockThreshold {
			return true
		}
	}
	return false
}

// FilterSelfDraftOverwrite drops the exact-name draft match for name when overwrite
// is updating that same pending draft.
func FilterSelfDraftOverwrite(related []RelatedSkill, name string, overwrite bool) []RelatedSkill {
	if !overwrite {
		return related
	}
	name = strings.TrimSpace(name)
	out := make([]RelatedSkill, 0, len(related))
	for _, r := range related {
		if r.Location == OverlapLocationDraft && r.Kind == OverlapKindExactName && r.Name == name {
			continue
		}
		out = append(out, r)
	}
	return out
}

func listManagedSkillEntries() ([]managedSkillEntry, error) {
	var entries []managedSkillEntry

	mirror, err := MirrorRoot()
	if err != nil {
		return nil, err
	}
	live, err := readSkillEntriesFromRoot(mirror, OverlapLocationLive)
	if err != nil {
		return nil, err
	}
	entries = append(entries, live...)

	drafts, err := ListDrafts()
	if err != nil {
		return nil, err
	}
	for _, d := range drafts {
		entries = append(entries, managedSkillEntry{
			Name:        d.Name,
			Description: d.Description,
			Location:    OverlapLocationDraft,
		})
	}
	return entries, nil
}

func readSkillEntriesFromRoot(root, location string) ([]managedSkillEntry, error) {
	dirEntries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]managedSkillEntry, 0, len(dirEntries))
	for _, entry := range dirEntries {
		name := entry.Name()
		if !ValidSkillName(name) {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(root, name, "SKILL.md"))
		if readErr != nil {
			continue
		}
		fm, _, parseErr := parseFrontmatter(string(raw))
		if parseErr != nil {
			continue
		}
		skillName := strings.TrimSpace(fm.Name)
		if skillName == "" {
			skillName = name
		}
		out = append(out, managedSkillEntry{
			Name:        skillName,
			Description: strings.TrimSpace(fm.Description),
			Location:    location,
		})
	}
	return out, nil
}

func scoreRelated(name, description string, entry managedSkillEntry) (RelatedSkill, bool) {
	best := RelatedSkill{}
	found := false

	if name != "" && strings.EqualFold(name, entry.Name) {
		best = RelatedSkill{
			Name:        entry.Name,
			Description: entry.Description,
			Location:    entry.Location,
			Kind:        OverlapKindExactName,
			Score:       1.0,
		}
		return best, true
	}

	if name != "" && entry.Name != "" {
		if score, ok := nameSimilarity(name, entry.Name); ok && score >= overlapNameJaccardThreshold {
			best = RelatedSkill{
				Name:        entry.Name,
				Description: entry.Description,
				Location:    entry.Location,
				Kind:        OverlapKindSimilarName,
				Score:       score,
			}
			found = true
		}
	}

	if description != "" && entry.Description != "" {
		descScore := jaccard(tokenizeWords(description), tokenizeWords(entry.Description))
		if descScore >= overlapDescJaccardThreshold && descScore > best.Score {
			best = RelatedSkill{
				Name:        entry.Name,
				Description: entry.Description,
				Location:    entry.Location,
				Kind:        OverlapKindSimilarDescription,
				Score:       descScore,
			}
			found = true
		}
	}

	return best, found
}

func nameSimilarity(a, b string) (float64, bool) {
	ta := tokenizeName(a)
	tb := tokenizeName(b)
	if len(ta) == 0 || len(tb) == 0 {
		return 0, false
	}
	jac := jaccard(ta, tb)
	cont := nameContainmentScore(ta, tb)
	score := jac
	if cont > score {
		score = cont
	}
	if score < overlapNameJaccardThreshold {
		return score, false
	}
	return score, true
}

// nameContainmentScore is 1.0 when the smaller token set is fully contained in the larger
// (specialization like pr-review ⊂ go-pr-review), otherwise 0.
func nameContainmentScore(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	smaller, larger := a, b
	if len(b) < len(a) {
		smaller, larger = b, a
	}
	for tok := range smaller {
		if _, ok := larger[tok]; !ok {
			return 0
		}
	}
	return 1.0
}

func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for tok := range a {
		if _, ok := b[tok]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

func tokenizeName(name string) map[string]struct{} {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "_", "-")
	parts := strings.Split(name, "-")
	out := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out[p] = struct{}{}
	}
	return out
}

func tokenizeWords(text string) map[string]struct{} {
	text = strings.ToLower(strings.TrimSpace(text))
	out := make(map[string]struct{})
	var b strings.Builder
	flush := func() {
		w := b.String()
		b.Reset()
		if len(w) < 2 {
			return
		}
		out[w] = struct{}{}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

// FormatOverlapBlock formats related skills for tool / log messages.
func FormatOverlapBlock(related []RelatedSkill) string {
	if len(related) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Possible overlapping managed skills/drafts:\n")
	for _, r := range related {
		fmt.Fprintf(&b, "- %s (%s, %s, score=%.2f): %s\n", r.Name, r.Location, r.Kind, r.Score, r.Description)
	}
	return strings.TrimRight(b.String(), "\n")
}
