package memory

import (
	"context"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/usage"
)

func workspaceForSession(ctx context.Context, reader session.TranscriptReader, sessionID string) string {
	if reader == nil || sessionID == "" {
		return ""
	}
	lookup, ok := reader.(interface {
		GetSession(context.Context, string) (session.Session, error)
	})
	if !ok {
		return ""
	}
	sess, err := lookup.GetSession(ctx, sessionID)
	if err != nil {
		return ""
	}
	return sess.WorkspaceID
}

func recordUsage(ctx context.Context, rec usage.Recorder, provider cometsdk.Provider, model, kind, sessionID string, u cometsdk.TokenUsage) {
	if rec == nil {
		return
	}
	scope := usage.ScopeFrom(ctx)
	if sessionID == "" {
		sessionID = scope.SessionID
	}
	providerID := ""
	if provider != nil {
		providerID = provider.ID()
	}
	if err := rec.Record(ctx, usage.Event{
		WorkspaceID: scope.WorkspaceID,
		SessionID:   sessionID,
		ProviderID:  providerID,
		ModelID:     model,
		CallKind:    kind,
		Usage:       u,
	}); err != nil {
		logging.L().Warn("usage.record_failed", "kind", kind, "model", model, "error", err)
	}
}

type recordingEmbedder struct {
	inner    Embedder
	rec      usage.Recorder
	provider string
	model    string
}

func wrapEmbedder(inner Embedder, rec usage.Recorder, settings EmbeddingSettings) Embedder {
	if inner == nil {
		return nil
	}
	if existing, ok := inner.(*recordingEmbedder); ok {
		inner = existing.inner
	}
	if rec == nil {
		return inner
	}
	return &recordingEmbedder{
		inner:    inner,
		rec:      rec,
		provider: settings.Provider,
		model:    settings.Model,
	}
}

func (e *recordingEmbedder) Model() string { return e.inner.Model() }

func (e *recordingEmbedder) Embed(ctx context.Context, texts ...string) ([][]float32, error) {
	vecs, tok, err := embedWithUsage(e.inner, ctx, texts...)
	if err != nil {
		return nil, err
	}
	if e.rec != nil {
		scope := usage.ScopeFrom(ctx)
		if err := e.rec.Record(ctx, usage.Event{
			WorkspaceID: scope.WorkspaceID,
			SessionID:   scope.SessionID,
			ProviderID:  e.provider,
			ModelID:     e.model,
			CallKind:    usage.KindEmbedding,
			Usage:       tok,
		}); err != nil {
			logging.L().Warn("usage.record_failed", "kind", usage.KindEmbedding, "model", e.model, "error", err)
		}
	}
	return vecs, nil
}

func embedWithUsage(inner Embedder, ctx context.Context, texts ...string) ([][]float32, cometsdk.TokenUsage, error) {
	if u, ok := inner.(interface {
		embedUsage(context.Context, ...string) ([][]float32, cometsdk.TokenUsage, error)
	}); ok {
		return u.embedUsage(ctx, texts...)
	}
	vecs, err := inner.Embed(ctx, texts...)
	if err != nil {
		return nil, cometsdk.TokenUsage{}, err
	}
	return vecs, estimateEmbeddingUsage(texts), nil
}

func estimateEmbeddingUsage(texts []string) cometsdk.TokenUsage {
	chars := 0
	for _, text := range texts {
		chars += len(text)
	}
	tokens := (chars + 3) / 4
	if tokens < 1 && chars > 0 {
		tokens = 1
	}
	return cometsdk.TokenUsage{InputTokens: tokens}
}
