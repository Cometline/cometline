package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/session"
	"github.com/oklog/ulid/v2"
)

const (
	extractionTranscriptMessages = 8
	extractionMaxTokens          = 1024
	simSkipThreshold             = 0.92
	simUpdateThreshold           = 0.80
)

type proposedMemory struct {
	Content            string  `json:"content"`
	Kind               string  `json:"kind"`
	PreferenceCategory string  `json:"preference_category"`
	Confidence         float64 `json:"confidence"`
	ShouldSave         bool    `json:"should_save"`
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
		logging.L().Info("memory.extract.noop", "session", sessionID, "reason", "empty_messages")
		return nil, nil
	}
	if shouldSkipExtraction(msgs) {
		logging.L().Info("memory.extract.noop", "session", sessionID, "reason", "low_value_turn", "messages", len(msgs))
		return nil, nil
	}

	useModel := strings.TrimSpace(e.settings.ExtractionModel)
	if useModel == "" {
		useModel = model
	}

	if pm, ok := tryExplicitRemember(msgs); ok {
		logging.L().Info("memory.extract.explicit", "session", sessionID, "content_bytes", len(pm.Content))
		change, err := e.ingestProposal(ctx, sessionID, pm, llmProvider, useModel)
		if err != nil {
			return nil, err
		}
		if change.Action != "" {
			return []Change{change}, nil
		}
		return nil, nil
	}

	originalMessages := len(msgs)
	msgs = recentMessages(msgs, extractionTranscriptMessages)
	logging.L().Info("memory.extract.start", "session", sessionID, "model", model, "messages", len(msgs), "total_messages", originalMessages)

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
		logging.L().Info("memory.extract.noop", "session", sessionID, "reason", "empty_transcript")
		return nil, nil
	}

	prompt := fmt.Sprintf(`Review this conversation and extract durable facts, preferences, or project knowledge worth remembering across future sessions.
Skip transient instructions, tool output, greetings, and one-off tasks.
Use kind="preference" for durable user preferences about language, tone, verbosity, tools, workflow, models, or coding style.
For preference memories, include preference_category as one of: language, tone, verbosity, workflow, model, tooling, coding_style, other.
Return JSON: {"memories":[{"content":"...","kind":"fact|preference|project","preference_category":"language|tone|verbosity|workflow|model|tooling|coding_style|other","confidence":0.0-1.0,"should_save":true|false}]}

Conversation:
%s`, transcript.String())

	var result extractionResult
	req := &cometsdk.Request{
		Model:  useModel,
		System: "You extract long-term memory candidates. Output JSON only.",
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: prompt}},
		}},
		MaxTokens: extractionMaxTokens,
	}
	if err := llm.GenerateJSON(ctx, llmProvider, req, &result); err != nil {
		logging.L().Warn("memory.extract.llm_failed", "session", sessionID, "model", useModel, "error", err)
		return nil, nil
	}
	logging.L().Info("memory.extract.proposed", "session", sessionID, "count", len(result.Memories), "model", useModel)

	var changes []Change
	for _, pm := range result.Memories {
		if !pm.ShouldSave || strings.TrimSpace(pm.Content) == "" || pm.Confidence < 0.3 {
			logging.L().Info("memory.extract.proposal_skipped", "session", sessionID, "kind", pm.Kind, "confidence", pm.Confidence, "should_save", pm.ShouldSave)
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
		logging.L().Info("memory.extract.change", "session", sessionID, "action", "skip", "reason", "similar", "memory_id", existing.ID, "similarity", sim)
		return Change{
			Action:  "update",
			Kind:    existing.Kind,
			Content: existing.Content,
			ID:      existing.ID,
		}, nil
	}
	if sim >= simUpdateThreshold {
		logging.L().Info("memory.extract.change", "session", sessionID, "action", "update", "memory_id", existing.ID, "similarity", sim)
		return e.updater.handleSimilar(ctx, existing, pm, vec, sessionID, llmProvider, useModel)
	}
	logging.L().Info("memory.extract.change", "session", sessionID, "action", "create", "kind", pm.Kind, "similarity", sim)
	return e.create(ctx, sessionID, pm, vec)
}

func (e *extractor) create(ctx context.Context, sessionID string, pm proposedMemory, vec []float32) (Change, error) {
	now := time.Now()
	rec := Record{
		ID:                 ulid.Make().String(),
		Scope:              "global",
		Kind:               normalizeKind(pm.Kind),
		PreferenceCategory: normalizePreferenceCategory(pm.Kind, pm.Content, pm.PreferenceCategory),
		Content:            strings.TrimSpace(pm.Content),
		Embedding:          vec,
		EmbeddingModel:     e.retriever.embedder.Model(),
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
	if err := e.store.insert(ctx, rec); err != nil {
		return Change{}, err
	}
	if err := e.store.logEvent(ctx, rec.ID, "create", ""); err != nil {
		return Change{}, err
	}
	return Change{
		Action:             "create",
		Kind:               rec.Kind,
		PreferenceCategory: rec.PreferenceCategory,
		Content:            rec.Content,
		ID:                 rec.ID,
	}, nil
}

func recentMessages(msgs []cometsdk.Message, max int) []cometsdk.Message {
	if max <= 0 || len(msgs) <= max {
		return msgs
	}
	return msgs[len(msgs)-max:]
}

func shouldSkipExtraction(msgs []cometsdk.Message) bool {
	lastUser := ""
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role != cometsdk.RoleUser {
			continue
		}
		lastUser = strings.ToLower(strings.TrimSpace(messageText(msgs[i])))
		break
	}
	if lastUser == "" || len([]rune(lastUser)) <= 2 {
		return true
	}
	ackOnly := map[string]struct{}{
		"ok": {}, "okay": {}, "k": {}, "yes": {}, "yep": {}, "yeah": {}, "no": {},
		"nope": {}, "thanks": {}, "thank you": {}, "thx": {}, "got it": {},
		"continue": {}, "go on": {}, "sure": {}, "nice": {}, "cool": {},
	}
	_, ok := ackOnly[strings.Trim(lastUser, ".! ")]
	return ok
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
