package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/memory"
)

const (
	defaultMemoryListLimit   = 20
	maxMemoryListLimit       = 100
	defaultMemorySearchLimit = 10
	maxMemorySearchLimit     = 50
	memoryWriteTimeout       = 30 * time.Second
)

type memoryToolResource struct {
	ID              string  `json:"id"`
	Kind            string  `json:"kind"`
	Content         string  `json:"content"`
	BaseWeight      float64 `json:"base_weight"`
	EffectiveWeight float64 `json:"effective_weight"`
	Pinned          bool    `json:"pinned"`
	AccessCount     int64   `json:"access_count"`
	Similarity      float64 `json:"similarity,omitempty"`
}

type memoryWritePublisher struct {
	events    *event.Hub
	operation string
}

func (p memoryWritePublisher) publish(rec memory.Record) {
	if p.events == nil {
		return
	}
	p.events.Publish(event.MemoryUpdated([]event.MemoryChangeWire{{
		Action:  p.operation,
		Kind:    rec.Kind,
		Content: rec.Content,
		ID:      rec.ID,
	}}))
}

func publishDeleted(events *event.Hub, rec memory.Record) {
	if events == nil {
		return
	}
	events.Publish(event.MemoryUpdated([]event.MemoryChangeWire{{
		Action:  "delete",
		Kind:    rec.Kind,
		Content: rec.Content,
		ID:      rec.ID,
	}}))
}

func logBackgroundWriteFailure(operation, id string, err error) {
	logging.L().Warn("memory.agent_tool_write_failed", "operation", operation, "memory_id", id, "error", err)
}

func memoryResourceFromScored(item memory.ScoredMemory) memoryToolResource {
	return memoryToolResource{
		ID:              item.ID,
		Kind:            item.Kind,
		Content:         item.Content,
		BaseWeight:      item.BaseWeight,
		EffectiveWeight: item.EffectiveWeight,
		Pinned:          item.Pinned,
		AccessCount:     item.AccessCount,
		Similarity:      item.Similarity,
	}
}

func marshalMemoryOutput(value any) (Result, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: string(raw)}, nil
}

func memoryUnavailable(m *memory.Service) (Result, bool) {
	if m == nil || !m.Enabled() {
		return Result{OK: false, Output: "memory service unavailable"}, true
	}
	return Result{}, false
}

func clampMemoryLimit(limit, fallback, maximum int) int {
	if limit <= 0 {
		limit = fallback
	}
	if limit > maximum {
		limit = maximum
	}
	return limit
}

// ListMemories exposes active memories to the agent for audits and deduplication.
type ListMemories struct {
	Memory *memory.Service
}

func (ListMemories) Spec() ToolSpec {
	return ToolSpec{
		Name:        "list_memories",
		Description: "List active semantic memories. Use this to inspect, audit, deduplicate, or clean up memories.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"limit":{"type":"integer","minimum":1,"maximum":100,"description":"Maximum memories to return. Defaults to 20."},
				"kind":{"type":"string","description":"Optional memory kind filter, such as preference or fact."},
				"pinned":{"type":"boolean","description":"Optional filter for pinned or unpinned memories."}
			}
		}`),
	}
}

func (t ListMemories) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if unavailable, ok := memoryUnavailable(t.Memory); ok {
		return unavailable, nil
	}
	var in struct {
		Limit  int    `json:"limit"`
		Kind   string `json:"kind"`
		Pinned *bool  `json:"pinned"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{OK: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}
	limit := clampMemoryLimit(in.Limit, defaultMemoryListLimit, maxMemoryListLimit)
	kind := strings.TrimSpace(in.Kind)
	items, err := t.Memory.ListActive(ctx)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	result := make([]memoryToolResource, 0, min(limit, len(items)))
	for _, item := range items {
		if kind != "" && !strings.EqualFold(item.Kind, kind) {
			continue
		}
		if in.Pinned != nil && item.Pinned != *in.Pinned {
			continue
		}
		result = append(result, memoryResourceFromScored(item))
		if len(result) >= limit {
			break
		}
	}
	return marshalMemoryOutput(result)
}

// SearchMemories performs hybrid semantic/FTS memory search for the agent.
type SearchMemories struct {
	Memory *memory.Service
}

func (SearchMemories) Spec() ToolSpec {
	return ToolSpec{
		Name:        "search_memories",
		Description: "Search semantic memories by meaning or keyword. Use this before changing a preference or looking for a related memory.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"query":{"type":"string","description":"Natural-language memory search query"},
				"limit":{"type":"integer","minimum":1,"maximum":50,"description":"Maximum memories to return. Defaults to 10."}
			},
			"required":["query"]
		}`),
	}
}

func (t SearchMemories) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if unavailable, ok := memoryUnavailable(t.Memory); ok {
		return unavailable, nil
	}
	var in struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{OK: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}
	query := strings.TrimSpace(in.Query)
	if query == "" {
		return Result{OK: false, Output: "query is required"}, nil
	}
	limit := clampMemoryLimit(in.Limit, defaultMemorySearchLimit, maxMemorySearchLimit)
	items, err := t.Memory.Search(ctx, query, limit)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	result := make([]memoryToolResource, 0, len(items))
	for _, item := range items {
		result = append(result, memoryResourceFromScored(item))
	}
	return marshalMemoryOutput(result)
}

type createMemoryInput struct {
	Content    string  `json:"content"`
	Kind       string  `json:"kind"`
	Pinned     bool    `json:"pinned"`
	BaseWeight float64 `json:"base_weight"`
}

// CreateMemory accepts a new memory and persists it in the background so
// embedding latency does not block the agent loop.
type CreateMemory struct {
	Memory *memory.Service
	Events *event.Hub
}

func (CreateMemory) Spec() ToolSpec {
	return ToolSpec{
		Name:        "create_memory",
		Description: "Save a durable semantic memory in the background. Returns accepted immediately; the UI is notified when it finishes.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"content":{"type":"string","description":"A concise fact, preference, or project detail to remember"},
				"kind":{"type":"string","description":"Memory kind, such as preference, fact, project, or task_outcome"},
				"pinned":{"type":"boolean","description":"Keep this memory from decaying"},
				"base_weight":{"type":"number","minimum":0,"description":"Optional importance weight; defaults to 1"}
			},
			"required":["content"]
		}`),
	}
}

func (t CreateMemory) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if unavailable, ok := memoryUnavailable(t.Memory); ok {
		return unavailable, nil
	}
	var in createMemoryInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{OK: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}
	in.Content = strings.TrimSpace(in.Content)
	if in.Content == "" {
		return Result{OK: false, Output: "content is required"}, nil
	}
	id := memory.NewID()
	go func() {
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), memoryWriteTimeout)
		defer cancel()
		rec, err := t.Memory.CreateManualWithID(writeCtx, id, in.Content, in.Kind, in.Pinned, in.BaseWeight)
		if err != nil {
			logBackgroundWriteFailure("create", id, err)
			return
		}
		memoryWritePublisher{events: t.Events, operation: "create"}.publish(rec)
	}()
	return marshalMemoryOutput(struct {
		Status string `json:"status"`
		ID     string `json:"id"`
	}{Status: "accepted", ID: id})
}

type updateMemoryInput struct {
	ID         string   `json:"id"`
	Content    string   `json:"content"`
	Kind       string   `json:"kind"`
	Pinned     *bool    `json:"pinned"`
	BaseWeight *float64 `json:"base_weight"`
}

// UpdateMemory accepts a memory edit and applies it in the background. Content
// changes are re-embedded by memory.Service.UpdateManual.
type UpdateMemory struct {
	Memory *memory.Service
	Events *event.Hub
}

func (UpdateMemory) Spec() ToolSpec {
	return ToolSpec{
		Name:        "update_memory",
		Description: "Update a durable memory in the background. Content changes are re-embedded; returns accepted immediately.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"id":{"type":"string","description":"Memory id from list_memories or search_memories"},
				"content":{"type":"string"},
				"kind":{"type":"string"},
				"pinned":{"type":"boolean"},
				"base_weight":{"type":"number","minimum":0}
			},
			"required":["id"]
		}`),
	}
}

func (t UpdateMemory) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if unavailable, ok := memoryUnavailable(t.Memory); ok {
		return unavailable, nil
	}
	var in updateMemoryInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{OK: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}
	in.ID = strings.TrimSpace(in.ID)
	if in.ID == "" {
		return Result{OK: false, Output: "id is required"}, nil
	}
	go func() {
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), memoryWriteTimeout)
		defer cancel()
		rec, err := t.Memory.UpdateManual(writeCtx, in.ID, in.Content, in.Kind, in.Pinned, in.BaseWeight)
		if err != nil {
			logBackgroundWriteFailure("update", in.ID, err)
			return
		}
		memoryWritePublisher{events: t.Events, operation: "update"}.publish(rec)
	}()
	return marshalMemoryOutput(struct {
		Status string `json:"status"`
		ID     string `json:"id"`
	}{Status: "accepted", ID: in.ID})
}

// DeleteMemory accepts a memory deletion and applies it in the background.
type DeleteMemory struct {
	Memory *memory.Service
	Events *event.Hub
}

func (DeleteMemory) Spec() ToolSpec {
	return ToolSpec{
		Name:        "delete_memory",
		Description: "Delete a durable memory in the background. Returns accepted immediately; the UI is notified when it finishes.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{"id":{"type":"string","description":"Memory id from list_memories or search_memories"}},
			"required":["id"]
		}`),
	}
}

func (t DeleteMemory) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	if unavailable, ok := memoryUnavailable(t.Memory); ok {
		return unavailable, nil
	}
	var in struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{OK: false, Output: fmt.Sprintf("invalid input: %v", err)}, nil
	}
	in.ID = strings.TrimSpace(in.ID)
	if in.ID == "" {
		return Result{OK: false, Output: "id is required"}, nil
	}
	go func() {
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), memoryWriteTimeout)
		defer cancel()
		rec, err := t.Memory.DeleteManual(writeCtx, in.ID)
		if err != nil {
			logBackgroundWriteFailure("delete", in.ID, err)
			return
		}
		publishDeleted(t.Events, rec)
	}()
	return marshalMemoryOutput(struct {
		Status string `json:"status"`
		ID     string `json:"id"`
	}{Status: "accepted", ID: in.ID})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
