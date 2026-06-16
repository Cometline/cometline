package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/session"
	"github.com/oklog/ulid/v2"
)

// Service is the global memory facade.
type Service struct {
	settings  Settings
	store     *store
	retriever *retriever
	extractor *extractor
	updater   *updater
	compactor *compactor
	provider  cometsdk.Provider
}

// NewService wires memory subsystems. provider is used for extraction/compaction LLM calls.
func NewService(dbConn *sql.DB, settings Settings, provider cometsdk.Provider, sessions *session.Service) (*Service, error) {
	if settings.MaxRetrieved <= 0 {
		settings.MaxRetrieved = 5
	}
	if settings.SimilarityThreshold <= 0 {
		settings.SimilarityThreshold = 0.5
	}
	embedder, err := NewEmbedder(settings.Embedding)
	if err != nil {
		return nil, err
	}
	st := newStore(dbConn)
	ret := &retriever{store: st, embedder: embedder, settings: settings}
	upd := &updater{store: st, embedder: embedder, provider: provider, settings: settings}
	ext := &extractor{
		store:     st,
		retriever: ret,
		updater:   upd,
		sessions:  sessions,
		provider:  provider,
		settings:  settings,
	}
	comp := &compactor{store: st, embedder: embedder, provider: provider, settings: settings}
	return &Service{
		settings:  settings,
		store:     st,
		retriever: ret,
		extractor: ext,
		updater:   upd,
		compactor: comp,
		provider:  provider,
	}, nil
}

// UpdateSettings replaces runtime memory settings.
func (s *Service) UpdateSettings(settings Settings) {
	s.settings = settings
	s.retriever.settings = settings
	s.extractor.settings = settings
	s.updater.settings = settings
	s.compactor.settings = settings
}

func (s *Service) Settings() Settings { return s.settings }


func (s *Service) Enabled() bool { return s.settings.Enabled }

// RetrieveForTurn returns scored memories for injection.
func (s *Service) RetrieveForTurn(ctx context.Context, query string) ([]ScoredMemory, error) {
	if !s.settings.Enabled || !s.settings.AutoRetrieve {
		return nil, nil
	}
	return s.retriever.retrieve(ctx, query, s.settings.MaxRetrieved, s.settings.SimilarityThreshold)
}

// Search performs semantic search for the UI.
func (s *Service) Search(ctx context.Context, query string, maxN int) ([]ScoredMemory, error) {
	if !s.settings.Enabled {
		return nil, nil
	}
	return s.retriever.search(ctx, query, maxN)
}

// ExtractAfterTurn proposes and stores memories from a completed turn.
// llmProvider should match the session provider used for the turn; when nil,
// the service's default provider is used.
func (s *Service) ExtractAfterTurn(ctx context.Context, sessionID, model string, llmProvider cometsdk.Provider) ([]Change, error) {
	if !s.settings.Enabled || !s.settings.AutoExtract {
		return nil, nil
	}
	changes, err := s.extractor.extractAfterTurn(ctx, sessionID, model, llmProvider)
	if err != nil {
		return changes, err
	}
	if s.settings.Lifecycle.CompactionOnExtract {
		if err := s.RunLifecycle(ctx); err != nil {
			return changes, err
		}
	}
	return changes, nil
}

// RunLifecycle applies decay forget and compaction if needed.
func (s *Service) RunLifecycle(ctx context.Context) error {
	if !s.settings.Enabled {
		return nil
	}
	count, err := s.store.countActive(ctx)
	if err != nil {
		return err
	}
	lc := s.settings.Lifecycle
	if err := s.compactor.forgetDecayed(ctx); err != nil {
		return err
	}
	if int(count) >= lc.MaxMemories {
		return s.compactor.run(ctx)
	}
	return nil
}

// CompactPreview returns candidates for the next compaction pass.
func (s *Service) CompactPreview(ctx context.Context) (CompactPreview, error) {
	return s.compactor.preview(ctx)
}

// Compact runs compaction immediately.
func (s *Service) Compact(ctx context.Context) error {
	return s.compactor.run(ctx)
}

// ListActive returns active memories with effective weights.
func (s *Service) ListActive(ctx context.Context) ([]ScoredMemory, error) {
	memories, err := s.store.listActive(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]ScoredMemory, len(memories))
	for i, m := range memories {
		ew := EffectiveWeight(m, now, s.settings.Lifecycle)
		out[i] = ScoredMemory{Record: m, EffectiveWeight: ew}
	}
	return out, nil
}

// CreateManual inserts a user-authored memory.
func (s *Service) CreateManual(ctx context.Context, content, kind string, pinned bool, baseWeight float64) (Record, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return Record{}, fmt.Errorf("content is required")
	}
	vecs, err := s.retriever.embedder.Embed(ctx, content)
	if err != nil {
		return Record{}, err
	}
	if len(vecs) == 0 {
		return Record{}, fmt.Errorf("embedding failed")
	}
	now := time.Now()
	if baseWeight <= 0 {
		baseWeight = 1.0
	}
	rec := Record{
		ID:             ulid.Make().String(),
		Scope:          "global",
		Kind:           normalizeKind(kind),
		Content:        content,
		Embedding:      vecs[0],
		EmbeddingModel: s.retriever.embedder.Model(),
		Source:         "manual",
		BaseWeight:     baseWeight,
		Pinned:         pinned,
		LastAccessedAt: &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.store.insert(ctx, rec); err != nil {
		return Record{}, err
	}
	_ = s.store.logEvent(ctx, rec.ID, "create", "manual")
	return rec, nil
}

// UpdateManual edits a memory.
func (s *Service) UpdateManual(ctx context.Context, id, content, kind string, pinned *bool, baseWeight *float64) (Record, error) {
	rec, err := s.store.get(ctx, id)
	if err != nil {
		return Record{}, err
	}
	if rec.Archived {
		return Record{}, fmt.Errorf("memory archived")
	}
	if strings.TrimSpace(content) != "" {
		rec.Content = strings.TrimSpace(content)
		vecs, err := s.retriever.embedder.Embed(ctx, rec.Content)
		if err != nil {
			return Record{}, err
		}
		if len(vecs) > 0 {
			rec.Embedding = vecs[0]
			rec.EmbeddingModel = s.retriever.embedder.Model()
		}
	}
	if kind != "" {
		rec.Kind = normalizeKind(kind)
	}
	if pinned != nil {
		rec.Pinned = *pinned
	}
	if baseWeight != nil {
		rec.BaseWeight = *baseWeight
	}
	rec.UpdatedAt = time.Now()
	if err := s.store.update(ctx, rec); err != nil {
		return Record{}, err
	}
	_ = s.store.logEvent(ctx, rec.ID, "manual_update", "")
	return rec, nil
}

// Delete removes a memory permanently.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.store.delete(ctx, id); err != nil {
		return err
	}
	return s.store.logEvent(ctx, id, "manual_delete", "")
}

// FormatForPrompt renders injected memories for the system prompt.
func FormatForPrompt(mems []ScoredMemory) string {
	if len(mems) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## Relevant memories\n")
	for i, m := range mems {
		fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, m.Kind, m.Content)
	}
	return b.String()
}
