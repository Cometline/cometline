package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/oklog/ulid/v2"
)

type updateDecision struct {
	Action  string `json:"action"` // update | supersede | skip
	Content string `json:"content"`
}

type updater struct {
	store    *store
	embedder Embedder
	provider cometsdk.Provider
	settings Settings
}

func (u *updater) handleSimilar(ctx context.Context, existing Record, pm proposedMemory, vec []float32, sessionID string, llmProvider cometsdk.Provider, useModel string) (Change, error) {
	if llmProvider == nil {
		llmProvider = u.provider
	}
	decision, err := u.decide(ctx, existing, pm, llmProvider, useModel)
	if err != nil {
		return Change{}, err
	}
	switch strings.ToLower(decision.Action) {
	case "skip":
		return Change{}, u.store.logEvent(ctx, existing.ID, "extract_skip", decision.Action)
	case "supersede":
		return u.supersede(ctx, existing, decision.Content, pm, vec, sessionID)
	default:
		return u.merge(ctx, existing, decision.Content, pm, vec)
	}
}

func (u *updater) decide(ctx context.Context, existing Record, pm proposedMemory, llmProvider cometsdk.Provider, useModel string) (updateDecision, error) {
	prompt := fmt.Sprintf(`Existing memory:
%s

New candidate:
%s

Decide whether to skip, update (merge into existing), or supersede (replace existing).
Return JSON: {"action":"update|supersede|skip","content":"merged or replacement text if not skip"}`, existing.Content, pm.Content)

	var out updateDecision
	req := &cometsdk.Request{
		Model:  useModel,
		System: "You reconcile memory updates. Output JSON only.",
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: prompt}},
		}},
		MaxTokens: 1024,
	}
	if err := llm.GenerateJSON(ctx, llmProvider, req, &out); err != nil {
		return updateDecision{}, err
	}
	if out.Action == "" {
		out.Action = "update"
	}
	if strings.TrimSpace(out.Content) == "" && out.Action != "skip" {
		out.Content = pm.Content
	}
	return out, nil
}

func (u *updater) merge(ctx context.Context, existing Record, content string, pm proposedMemory, vec []float32) (Change, error) {
	now := time.Now()
	existing.Content = strings.TrimSpace(content)
	existing.PreferenceCategory = normalizePreferenceCategory(existing.Kind, existing.Content, existing.PreferenceCategory)
	existing.Embedding = vec
	existing.EmbeddingModel = u.embedder.Model()
	if pm.Confidence > existing.BaseWeight {
		existing.BaseWeight = pm.Confidence
	}
	existing.LastAccessedAt = &now
	existing.UpdatedAt = now
	if err := u.store.update(ctx, existing); err != nil {
		return Change{}, err
	}
	if err := u.store.logEvent(ctx, existing.ID, "update", encodeDetail(map[string]any{"confidence": pm.Confidence})); err != nil {
		return Change{}, err
	}
	return Change{
		Action:             "update",
		Kind:               existing.Kind,
		PreferenceCategory: existing.PreferenceCategory,
		Content:            existing.Content,
		ID:                 existing.ID,
	}, nil
}

func (u *updater) supersede(ctx context.Context, existing Record, content string, pm proposedMemory, vec []float32, sessionID string) (Change, error) {
	newID := ulid.Make().String()
	now := time.Now()
	if err := u.store.archive(ctx, existing.ID, "superseded", newID); err != nil {
		return Change{}, err
	}
	_ = u.store.logEvent(ctx, existing.ID, "supersede", encodeDetail(map[string]string{"superseded_by": newID}))

	rec := Record{
		ID:                 newID,
		Scope:              existing.Scope,
		Kind:               normalizeKind(pm.Kind),
		PreferenceCategory: normalizePreferenceCategory(pm.Kind, content, pm.PreferenceCategory),
		Content:            strings.TrimSpace(content),
		Embedding:          vec,
		EmbeddingModel:     u.embedder.Model(),
		Source:             "auto",
		BaseWeight:         pm.Confidence,
		SourceSessionID:    sessionID,
		LastAccessedAt:     &now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if rec.BaseWeight <= 0 {
		rec.BaseWeight = 0.7
	}
	if err := u.store.insert(ctx, rec); err != nil {
		return Change{}, err
	}
	if err := u.store.logEvent(ctx, rec.ID, "create", "supersede"); err != nil {
		return Change{}, err
	}
	return Change{
		Action:             "supersede",
		Kind:               rec.Kind,
		PreferenceCategory: rec.PreferenceCategory,
		Content:            rec.Content,
		ID:                 rec.ID,
	}, nil
}
