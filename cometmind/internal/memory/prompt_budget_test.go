package memory

import "testing"

func TestPromptMemoriesAllowanceIncludesExactFormattingBoundary(t *testing.T) {
	prompt := NewPromptMemories(
		[]ScoredMemory{{Record: Record{ID: "pref", Kind: "preference", Content: "Use concise answers."}}},
		[]ScoredMemory{{Record: Record{ID: "task", Kind: "task_outcome", Content: "Completed retry handling."}}},
		[]ScoredMemory{{Record: Record{ID: "fact", Kind: "project", Content: "The service uses SQLite."}}},
	)

	for n := 1; n <= len(prompt.Records); n++ {
		prefix := PromptMemories{Records: prompt.Records[:n]}
		allowance := EstimatePromptMemoriesTokens(prefix)
		got := prompt.WithinTokenAllowance(allowance)
		if len(got.Records) != n {
			t.Fatalf("exact allowance for %d records selected %d", n, len(got.Records))
		}
		if estimated := EstimatePromptMemoriesTokens(got); estimated > allowance {
			t.Fatalf("formatted estimate %d exceeds allowance %d", estimated, allowance)
		}
		if allowance > 0 {
			below := prompt.WithinTokenAllowance(allowance - 1)
			if len(below.Records) >= n {
				t.Fatalf("allowance below %d-record boundary selected %d", n, len(below.Records))
			}
		}
	}
}

func TestPromptMemoriesAllowanceCountsNumberingAndSemanticKind(t *testing.T) {
	semantic := make([]ScoredMemory, 10)
	for i := range semantic {
		semantic[i] = ScoredMemory{Record: Record{ID: string(rune('a' + i)), Kind: "project", Content: "x"}}
	}
	prompt := NewPromptMemories(nil, nil, semantic)
	fullAllowance := EstimatePromptMemoriesTokens(prompt)
	selected := prompt.WithinTokenAllowance(fullAllowance)
	if len(selected.Records) != 10 {
		t.Fatalf("selected %d records at exact full allowance", len(selected.Records))
	}
	if got := EstimatePromptMemoriesTokens(selected); got != fullAllowance {
		t.Fatalf("estimate = %d, want %d", got, fullAllowance)
	}
	below := prompt.WithinTokenAllowance(fullAllowance - 1)
	if len(below.Records) != 9 {
		t.Fatalf("selected %d records below two-digit numbering boundary, want 9", len(below.Records))
	}
}
