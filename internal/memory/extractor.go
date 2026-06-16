package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/cometline/cometmind/internal/session"
	"github.com/oklog/ulid/v2"
)

const (
	simSkipThreshold   = 0.92
	simUpdateThreshold = 0.80
)

type proposedMemory struct {
	Content    string  `json:"content"`
	Kind       string  `json:"kind"`
	Confidence float64 `json:"confidence"`
	ShouldSave bool    `json:"should_save"`
}

type extractionResult struct {
	Memories []proposedMemory `json:"memories"`
}

type extractor struct {
	store     *store
	retriever *retriever
	updater   *updater
	sessions  *session.Service
	provider  cometsdk.Provider
	settings  Settings
}

func (e *extractor) extractAfterTurn(ctx context.Context, sessionID, model string, llmProvider cometsdk.Provider) ([]Change, error) {
	if llmProvider == nil {
		llmProvider = e.provider
	}
	msgs, err := e.sessions.BuildSDKMessages(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, nil
	}

	var transcript strings.Builder
	for _, m := range msgs {
		text := messageText(m)
		if text == "" {
			continue
		}
		transcript.WriteString(string(m.Role))
		transcript.WriteString(": ")
		transcript.WriteString(text)
		transcript.WriteString("\n")
	}
	if transcript.Len() == 0 {
		return nil, nil
	}

	prompt := fmt.Sprintf(`Review this conversation and extract durable facts, preferences, or project knowledge worth remembering across future sessions.
Skip transient instructions, tool output, greetings, and one-off tasks.
Return JSON: {"memories":[{"content":"...","kind":"fact|preference|project","confidence":0.0-1.0,"should_save":true|false}]}

Conversation:
%s`, transcript.String())

	useModel := strings.TrimSpace(e.settings.ExtractionModel)
	if useModel == "" {
		useModel = model
	}

	var result extractionResult
	req := &cometsdk.Request{
		Model: useModel,
		System: "You extract long-term memory candidates. Output JSON only.",
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: prompt}},
		}},
		MaxTokens: 2048,
	}
	if err := llm.GenerateJSON(ctx, llmProvider, req, &result); err != nil {
		return nil, err
	}

	var changes []Change
	for _, pm := range result.Memories {
		if !pm.ShouldSave || strings.TrimSpace(pm.Content) == "" || pm.Confidence < 0.3 {
			continue
		}
		change, err := e.ingestProposal(ctx, sessionID, pm, llmProvider, useModel)
		if err != nil {
			return changes, err
		}
		if change.Action != "" {
			changes = append(changes, change)
		}
	}
	return changes, nil
}

func (e *extractor) ingestProposal(ctx context.Context, sessionID string, pm proposedMemory, llmProvider cometsdk.Provider, useModel string) (Change, error) {
	vecs, err := e.retriever.embedder.Embed(ctx, pm.Content)
	if err != nil {
		return Change{}, err
	}
	if len(vecs) == 0 {
		return Change{}, nil
	}
	vec := vecs[0]

	existing, sim, err := e.retriever.bestMatch(ctx, vec)
	if err != nil {
		return Change{}, err
	}
	if sim > simSkipThreshold {
		_ = e.store.logEvent(ctx, existing.ID, "extract_skip", fmt.Sprintf("similarity=%.3f", sim))
		return Change{}, nil
	}
	if sim >= simUpdateThreshold {
		return e.updater.handleSimilar(ctx, existing, pm, vec, sessionID, llmProvider, useModel)
	}
	return e.create(ctx, sessionID, pm, vec)
}

func (e *extractor) create(ctx context.Context, sessionID string, pm proposedMemory, vec []float32) (Change, error) {
	now := time.Now()
	rec := Record{
		ID:              ulid.Make().String(),
		Scope:           "global",
		Kind:            normalizeKind(pm.Kind),
		Content:         strings.TrimSpace(pm.Content),
		Embedding:       vec,
		EmbeddingModel:  e.retriever.embedder.Model(),
		Source:          "auto",
		BaseWeight:      pm.Confidence,
		SourceSessionID: sessionID,
		LastAccessedAt:  &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if rec.BaseWeight <= 0 {
		rec.BaseWeight = 0.7
	}
	if err := e.store.insert(ctx, rec); err != nil {
		return Change{}, err
	}
	if err := e.store.logEvent(ctx, rec.ID, "create", ""); err != nil {
		return Change{}, err
	}
	return Change{
		Action:  "create",
		Kind:    rec.Kind,
		Content: rec.Content,
		ID:      rec.ID,
	}, nil
}

func messageText(m cometsdk.Message) string {
	var b strings.Builder
	for _, bl := range m.Content {
		if tb, ok := bl.(cometsdk.TextBlock); ok {
			b.WriteString(tb.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

func normalizeKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "preference", "project":
		return strings.ToLower(kind)
	default:
		return "fact"
	}
}

func encodeDetail(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
