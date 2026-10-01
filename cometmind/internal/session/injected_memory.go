package session

import (
	"encoding/json"
	"strings"
)

// InjectedMemory is a memory surfaced to the UI for a turn. It is persisted as
// a JSON array in messages.injected_memories so the memory card survives a
// session reload (previously these were only emitted live over SSE).
type MemoryBucket string

const (
	MemoryBucketPreference  MemoryBucket = "preference"
	MemoryBucketTaskOutcome MemoryBucket = "task_outcome"
	MemoryBucketSemantic    MemoryBucket = "semantic"
)

type InjectedMemory struct {
	ID              string       `json:"id"`
	Content         string       `json:"content"`
	Kind            string       `json:"kind"`
	Bucket          MemoryBucket `json:"bucket"`
	Similarity      float64      `json:"similarity"`
	EffectiveWeight float64      `json:"effective_weight"`
}

// marshalInjectedMemories serializes injected memories to a JSON array string,
// always returning a valid array (never "null") for the NOT NULL column.
func marshalInjectedMemories(memories []InjectedMemory) (string, error) {
	if len(memories) == 0 {
		return "[]", nil
	}
	raw, err := json.Marshal(memories)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// unmarshalInjectedMemories parses the persisted JSON array, tolerating empty
// or malformed values by returning an empty slice.
func unmarshalInjectedMemories(raw string) []InjectedMemory {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return nil
	}
	var memories []InjectedMemory
	if err := json.Unmarshal([]byte(raw), &memories); err != nil {
		return nil
	}
	for i := range memories {
		if memories[i].Bucket != "" {
			continue
		}
		switch memories[i].Kind {
		case "preference":
			memories[i].Bucket = MemoryBucketPreference
		case "task_outcome", "task_summary":
			memories[i].Bucket = MemoryBucketTaskOutcome
		default:
			memories[i].Bucket = MemoryBucketSemantic
		}
	}
	return memories
}
