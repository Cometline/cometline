package memory

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/logging"
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
		logging.L().Info("memory.retrieve.skipped", "enabled", s.settings.Enabled, "auto_retrieve", s.settings.AutoRetrieve)
		return nil, nil
	}
	mems, err := s.retriever.retrieve(ctx, query, s.settings.MaxRetrieved, s.settings.SimilarityThreshold)
	if err != nil {
		logging.L().Error("memory.retrieve.failed", "error", err)
		return nil, err
	}
	return mems, nil
}

// Search performs semantic search for the UI.
func (s *Service) Search(ctx context.Context, query string, maxN int) ([]ScoredMemory, error) {
	if !s.settings.Enabled {
		logging.L().Info("memory.search.skipped", "enabled", false)
		return nil, nil
	}
	started := time.Now()
	mems, err := s.retriever.search(ctx, query, maxN)
	if err != nil {
		logging.L().Error("memory.search.failed", "limit", maxN, "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return nil, err
	}
	logging.L().Info("memory.search.completed", "count", len(mems), "limit", maxN, "duration_ms", time.Since(started).Milliseconds())
	return mems, nil
}

// BaselinePreferences returns a small, cheap-to-load set of user preferences
// that should be injected for substantive turns regardless of semantic match.
func (s *Service) BaselinePreferences(ctx context.Context, limit int) ([]ScoredMemory, error) {
	if !s.settings.Enabled {
		logging.L().Info("memory.preferences.skipped", "enabled", false)
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultBaselinePreferenceLimit
	}
	started := time.Now()
	recs, err := s.store.listBaselinePreferences(ctx, limit)
	if err != nil {
		logging.L().Error("memory.preferences.failed", "limit", limit, "error", err)
		return nil, err
	}
	now := time.Now()
	out := make([]ScoredMemory, len(recs))
	for i, rec := range recs {
		out[i] = ScoredMemory{Record: rec, EffectiveWeight: EffectiveWeight(rec, now, s.settings.Lifecycle)}
		_ = s.store.touchAccess(ctx, rec.ID)
		_ = s.store.logEvent(ctx, rec.ID, "preference_inject", "")
	}
	logging.L().Info("memory.preferences.loaded", "count", len(out), "limit", limit, "duration_ms", time.Since(started).Milliseconds())
	return out, nil
}

func (s *Service) CompactPreferenceCategory(ctx context.Context, category string) error {
	category = normalizePreferenceCategory("preference", "", category)
	active, err := s.store.listActive(ctx)
	if err != nil {
		logging.L().Error("memory.preference_category.failed", "category", category, "error", err)
		return err
	}
	var recs []Record
	for _, rec := range active {
		if rec.Kind != "preference" {
			continue
		}
		normalized := normalizePreferenceCategory(rec.Kind, rec.Content, rec.PreferenceCategory)
		if normalized != category {
			continue
		}
		if rec.PreferenceCategory != normalized {
			rec.PreferenceCategory = normalized
			if err := s.store.update(ctx, rec); err != nil {
				logging.L().Error("memory.preference_category_backfill.failed", "category", category, "memory_id", rec.ID, "error", err)
				return err
			}
		}
		recs = append(recs, rec)
	}
	sort.Slice(recs, func(i, j int) bool {
		if recs[i].Pinned != recs[j].Pinned {
			return recs[i].Pinned
		}
		if !recs[i].UpdatedAt.Equal(recs[j].UpdatedAt) {
			return recs[i].UpdatedAt.After(recs[j].UpdatedAt)
		}
		if recs[i].BaseWeight != recs[j].BaseWeight {
			return recs[i].BaseWeight > recs[j].BaseWeight
		}
		return recs[i].AccessCount > recs[j].AccessCount
	})
	cap := preferenceCategoryCap(category)
	kept := 0
	archived := 0
	for _, rec := range recs {
		if rec.Pinned {
			continue
		}
		kept++
		if kept <= cap {
			continue
		}
		if err := s.store.archive(ctx, rec.ID, "preference_category_cap", ""); err != nil {
			logging.L().Error("memory.preference_category_archive.failed", "category", category, "memory_id", rec.ID, "error", err)
			return err
		}
		_ = s.store.logEvent(ctx, rec.ID, "preference_category_cap", category)
		archived++
	}
	logging.L().Info("memory.preference_category.completed", "category", category, "active", len(recs), "cap", cap, "archived", archived)
	return nil
}

// ExtractAfterTurn proposes and stores memories from a completed turn.
// llmProvider should match the session provider used for the turn; when nil,
// the service's default provider is used.
func (s *Service) ExtractAfterTurn(ctx context.Context, sessionID, model string, llmProvider cometsdk.Provider) ([]Change, error) {
	if !s.settings.Enabled || !s.settings.AutoExtract {
		logging.L().Info("memory.extract.skipped", "session", sessionID, "enabled", s.settings.Enabled, "auto_extract", s.settings.AutoExtract)
		return nil, nil
	}
	started := time.Now()
	changes, err := s.extractor.extractAfterTurn(ctx, sessionID, model, llmProvider)
	if err != nil {
		logging.L().Error("memory.extract.failed", "session", sessionID, "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return changes, err
	}
	logging.L().Info("memory.extract.completed", "session", sessionID, "changes", len(changes), "duration_ms", time.Since(started).Milliseconds())
	for _, change := range changes {
		if change.Kind == "preference" {
			_ = s.CompactPreferenceCategory(ctx, change.PreferenceCategory)
		}
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
		logging.L().Info("memory.lifecycle.skipped", "enabled", false)
		return nil
	}
	started := time.Now()
	count, err := s.store.countActive(ctx)
	if err != nil {
		logging.L().Error("memory.lifecycle.failed", "error", err)
		return err
	}
	lc := s.settings.Lifecycle
	if err := s.compactor.forgetDecayed(ctx); err != nil {
		logging.L().Error("memory.lifecycle.failed", "active_count", count, "error", err)
		return err
	}
	if int(count) >= lc.MaxMemories {
		err := s.compactor.run(ctx)
		if err != nil {
			logging.L().Error("memory.compact.failed", "active_count", count, "max_memories", lc.MaxMemories, "duration_ms", time.Since(started).Milliseconds(), "error", err)
			return err
		}
		logging.L().Info("memory.compact.completed", "active_count", count, "max_memories", lc.MaxMemories, "duration_ms", time.Since(started).Milliseconds())
		return nil
	}
	logging.L().Info("memory.lifecycle.completed", "active_count", count, "max_memories", lc.MaxMemories, "compacted", false, "duration_ms", time.Since(started).Milliseconds())
	return nil
}

// CompactPreview returns candidates for the next compaction pass.
func (s *Service) CompactPreview(ctx context.Context) (CompactPreview, error) {
	preview, err := s.compactor.preview(ctx)
	if err != nil {
		logging.L().Error("memory.compact_preview.failed", "error", err)
		return preview, err
	}
	logging.L().Info("memory.compact_preview.completed", "to_forget", len(preview.ToForget), "merge_groups", len(preview.ToMerge))
	return preview, nil
}

// Compact runs compaction immediately.
func (s *Service) Compact(ctx context.Context) error {
	started := time.Now()
	if err := s.compactor.run(ctx); err != nil {
		logging.L().Error("memory.compact.failed", "manual", true, "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return err
	}
	logging.L().Info("memory.compact.completed", "manual", true, "duration_ms", time.Since(started).Milliseconds())
	return nil
}

// PurgeArchived hard-deletes archived memories and old memory_events.
func (s *Service) PurgeArchived(ctx context.Context, olderThanDays int) (memories int, events int, err error) {
	if olderThanDays <= 0 {
		logging.L().Info("memory.purge_archived.skipped", "older_than_days", olderThanDays)
		return 0, 0, nil
	}
	cutoff := time.Now().Add(-time.Duration(olderThanDays) * 24 * time.Hour).UnixMilli()
	memories, events, err = s.store.purgeArchived(ctx, cutoff)
	if err != nil {
		logging.L().Error("memory.purge_archived.failed", "older_than_days", olderThanDays, "error", err)
		return memories, events, err
	}
	logging.L().Info("memory.purge_archived.completed", "older_than_days", olderThanDays, "memories", memories, "events", events)
	return memories, events, nil
}

// ListActive returns active memories with effective weights.
func (s *Service) ListActive(ctx context.Context) ([]ScoredMemory, error) {
	memories, err := s.store.listActive(ctx)
	if err != nil {
		logging.L().Error("memory.list_active.failed", "error", err)
		return nil, err
	}
	now := time.Now()
	out := make([]ScoredMemory, len(memories))
	for i, m := range memories {
		ew := EffectiveWeight(m, now, s.settings.Lifecycle)
		out[i] = ScoredMemory{Record: m, EffectiveWeight: ew}
	}
	logging.L().Info("memory.list_active.completed", "count", len(out))
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
		ID:                 ulid.Make().String(),
		Scope:              "global",
		Kind:               normalizeKind(kind),
		PreferenceCategory: normalizePreferenceCategory(kind, content, ""),
		Content:            content,
		Embedding:          vecs[0],
		EmbeddingModel:     s.retriever.embedder.Model(),
		Source:             "manual",
		BaseWeight:         baseWeight,
		Pinned:             pinned,
		LastAccessedAt:     &now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.store.insert(ctx, rec); err != nil {
		logging.L().Error("memory.manual_create.failed", "kind", rec.Kind, "pinned", rec.Pinned, "error", err)
		return Record{}, err
	}
	_ = s.store.logEvent(ctx, rec.ID, "create", "manual")
	if rec.Kind == "preference" {
		_ = s.CompactPreferenceCategory(ctx, rec.PreferenceCategory)
	}
	logging.L().Info("memory.manual_create.completed", "memory_id", rec.ID, "kind", rec.Kind, "pinned", rec.Pinned, "base_weight", rec.BaseWeight)
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
	rec.PreferenceCategory = normalizePreferenceCategory(rec.Kind, rec.Content, rec.PreferenceCategory)
	if pinned != nil {
		rec.Pinned = *pinned
	}
	if baseWeight != nil {
		rec.BaseWeight = *baseWeight
	}
	rec.UpdatedAt = time.Now()
	if err := s.store.update(ctx, rec); err != nil {
		logging.L().Error("memory.manual_update.failed", "memory_id", rec.ID, "error", err)
		return Record{}, err
	}
	_ = s.store.logEvent(ctx, rec.ID, "manual_update", "")
	if rec.Kind == "preference" {
		_ = s.CompactPreferenceCategory(ctx, rec.PreferenceCategory)
	}
	logging.L().Info("memory.manual_update.completed", "memory_id", rec.ID, "kind", rec.Kind, "pinned", rec.Pinned, "base_weight", rec.BaseWeight)
	return rec, nil
}

// Delete removes a memory permanently.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.store.delete(ctx, id); err != nil {
		logging.L().Error("memory.manual_delete.failed", "memory_id", id, "error", err)
		return err
	}
	if err := s.store.logEvent(ctx, id, "manual_delete", ""); err != nil {
		logging.L().Error("memory.manual_delete_event.failed", "memory_id", id, "error", err)
		return err
	}
	logging.L().Info("memory.manual_delete.completed", "memory_id", id)
	return nil
}

type PromptMemories struct {
	Preferences []ScoredMemory
	Relevant    []ScoredMemory
}

func FormatPromptMemories(mems PromptMemories) string {
	if len(mems.Preferences) == 0 && len(mems.Relevant) == 0 {
		return ""
	}
	var b strings.Builder
	if len(mems.Preferences) > 0 {
		b.WriteString("\n\n## User preferences\n")
		for i, m := range mems.Preferences {
			fmt.Fprintf(&b, "%d. %s\n", i+1, m.Content)
		}
	}
	relevant := dedupeRelevantMemories(mems.Preferences, mems.Relevant)
	if len(relevant) > 0 {
		b.WriteString("\n\n## Relevant memories\n")
	}
	for i, m := range relevant {
		fmt.Fprintf(&b, "%d. [%s] %s\n", i+1, m.Kind, m.Content)
	}
	return b.String()
}

// FormatForPrompt renders injected memories for the system prompt.
func FormatForPrompt(mems []ScoredMemory) string {
	return FormatPromptMemories(PromptMemories{Relevant: mems})
}

func dedupeRelevantMemories(preferences, relevant []ScoredMemory) []ScoredMemory {
	seen := make(map[string]struct{}, len(preferences))
	for _, m := range preferences {
		seen[m.ID] = struct{}{}
	}
	out := make([]ScoredMemory, 0, len(relevant))
	for _, m := range relevant {
		if _, ok := seen[m.ID]; ok {
			continue
		}
		out = append(out, m)
	}
	return out
}
