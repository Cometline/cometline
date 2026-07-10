package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestRegistryIncludesAgentMemoryTools(t *testing.T) {
	_, conn, svc := testMemoryService(t)
	defer conn.Close()

	r := NewRegistry(t.TempDir(), RegistryOptions{Memory: svc})
	for _, name := range []string{
		"list_memories",
		"search_memories",
		"create_memory",
		"update_memory",
		"delete_memory",
	} {
		if !r.Has(name) {
			t.Fatalf("registry missing %q", name)
		}
	}
}

func TestListMemoriesFiltersAndLimits(t *testing.T) {
	ctx, conn, svc := testMemoryService(t)
	defer conn.Close()
	insertMemoryForToolTest(t, ctx, conn, "pref", "preference", "Use Traditional Chinese")
	insertMemoryForToolTest(t, ctx, conn, "fact", "fact", "The project uses Go")
	insertMemoryForToolTest(t, ctx, conn, "other", "preference", "Keep answers concise")

	result, err := (ListMemories{Memory: svc}).Execute(ctx, json.RawMessage(`{"kind":"preference","limit":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatalf("list failed: %s", result.Output)
	}
	var memories []memoryToolResource
	if err := json.Unmarshal([]byte(result.Output), &memories); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if len(memories) != 1 || memories[0].Kind != "preference" {
		t.Fatalf("unexpected memories: %+v", memories)
	}
}

func TestMemoryWriteToolsValidateInputBeforeAccepting(t *testing.T) {
	ctx := context.Background()
	result, _ := (CreateMemory{Memory: nil}).Execute(ctx, json.RawMessage(`{"content":"x"}`))
	if result.OK || result.Output != "memory service unavailable" {
		t.Fatalf("unexpected unavailable result: %+v", result)
	}

	result, _ = (UpdateMemory{Memory: nil}).Execute(ctx, json.RawMessage(`{"id":"mem_1"}`))
	if result.OK || result.Output != "memory service unavailable" {
		t.Fatalf("unexpected unavailable update result: %+v", result)
	}
}
