package memory

import (
	"fmt"
	"strings"
)

type MemoryBucket string

const (
	BucketPreference  MemoryBucket = "preference"
	BucketTaskOutcome MemoryBucket = "task_outcome"
	BucketSemantic    MemoryBucket = "semantic"
)

type PromptMemory struct {
	ScoredMemory
	Bucket MemoryBucket
}

type PromptMemories struct {
	Records []PromptMemory
}

func NewPromptMemories(preferences, outcomes, semantic []ScoredMemory) PromptMemories {
	records := make([]PromptMemory, 0, len(preferences)+len(outcomes)+len(semantic))
	seen := make(map[string]struct{}, cap(records))
	for _, group := range []struct {
		bucket MemoryBucket
		items  []ScoredMemory
	}{{BucketPreference, preferences}, {BucketTaskOutcome, outcomes}, {BucketSemantic, semantic}} {
		for _, item := range group.items {
			if _, ok := seen[item.ID]; ok {
				continue
			}
			seen[item.ID] = struct{}{}
			records = append(records, PromptMemory{ScoredMemory: item, Bucket: group.bucket})
		}
	}
	return PromptMemories{Records: records}
}

func (m PromptMemories) Count(bucket MemoryBucket) int {
	count := 0
	for _, item := range m.Records {
		if item.Bucket == bucket {
			count++
		}
	}
	return count
}

func (m PromptMemories) WithinTokenAllowance(allowance int) PromptMemories {
	if allowance <= 0 {
		return PromptMemories{}
	}
	selected := make([]PromptMemory, 0, len(m.Records))
	for _, item := range m.Records {
		candidate := PromptMemories{Records: append(selected, item)}
		if EstimatePromptMemoriesTokens(candidate) > allowance {
			break
		}
		selected = append(selected, item)
	}
	return PromptMemories{Records: selected}
}

// EstimatePromptMemoriesTokens applies the conservative ceil(chars/4) rule to
// the exact suffix sent to the model, including headings and list decoration.
func EstimatePromptMemoriesTokens(mems PromptMemories) int {
	runes := len([]rune(FormatPromptMemories(mems)))
	if runes == 0 {
		return 0
	}
	return (runes + 3) / 4
}

func FormatPromptMemories(mems PromptMemories) string {
	if len(mems.Records) == 0 {
		return ""
	}
	var b strings.Builder
	if mems.Count(BucketPreference) > 0 {
		b.WriteString("\n\n## User preferences\n")
		i := 0
		for _, m := range mems.Records {
			if m.Bucket == BucketPreference {
				i++
				fmt.Fprintf(&b, "%d. %s\n", i, m.Content)
			}
		}
	}
	if mems.Count(BucketTaskOutcome) > 0 {
		b.WriteString("\n\n## Relevant task outcomes\n")
		i := 0
		for _, m := range mems.Records {
			if m.Bucket == BucketTaskOutcome {
				i++
				fmt.Fprintf(&b, "%d. %s\n", i, m.Content)
			}
		}
	}
	if mems.Count(BucketSemantic) > 0 {
		b.WriteString("\n\n## Semantic memories\n")
	}
	i := 0
	for _, m := range mems.Records {
		if m.Bucket == BucketSemantic {
			i++
			fmt.Fprintf(&b, "%d. [%s] %s\n", i, m.Kind, m.Content)
		}
	}
	return b.String()
}

// FormatForPrompt renders injected memories for the system prompt.
func FormatForPrompt(mems []ScoredMemory) string {
	return FormatPromptMemories(NewPromptMemories(nil, nil, mems))
}
