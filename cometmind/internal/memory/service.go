package memory

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"time"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/usage"
)

// Service is the global memory facade.
type Service struct {
	settings                  Settings
	store                     *store
	retriever                 *retriever
	extractor                 *extractor
	updater                   *updater
	compactor                 *compactor
	provider                  cometsdk.Provider
	usage                     usage.Recorder
	onCompactionCompleted     func(CompactionResult)
	reembed                   reembedState
	outcomeMu                 sync.Mutex
	rollUpTaskLineageOverride func(context.Context, string, string) error
}

// CompactionResult describes a completed manual or automatic compaction pass.
type CompactionResult struct {
	Before  int64
	After   int64
	Trigger string
}

// NewService wires memory subsystems. provider is used for extraction/compaction LLM calls.
// sessions is narrowed to the transcript-reading seam so memory can be tested without a live SQLite store.
func NewService(dbConn *sql.DB, settings Settings, provider cometsdk.Provider, sessions session.TranscriptReader) (*Service, error) {
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

// UpdateSettings replaces runtime memory settings and rebuilds the embedder
// when embedding credentials/model change.
func (s *Service) UpdateSettings(settings Settings) error {
	if s == nil {
		return fmt.Errorf("memory service is nil")
	}
	if settings.MaxRetrieved <= 0 {
		settings.MaxRetrieved = 5
	}
	if settings.SimilarityThreshold <= 0 {
		settings.SimilarityThreshold = 0.5
	}
	needEmbedder := !embeddingSettingsEqual(s.settings.Embedding, settings.Embedding)
	s.settings = settings
	s.retriever.settings = settings
	s.extractor.settings = settings
	s.updater.settings = settings
	s.compactor.settings = settings
	if !needEmbedder {
		return nil
	}
	embedder, err := NewEmbedder(settings.Embedding)
	if err != nil {
		return err
	}
	s.applyEmbedder(embedder)
	return nil
}

// SetProvider replaces the LLM used for extraction/compaction/updates.
func (s *Service) SetProvider(p cometsdk.Provider) {
	if s == nil {
		return
	}
	s.provider = p
	s.extractor.provider = p
	s.updater.provider = p
	s.compactor.provider = p
}

// SetUsageRecorder records memory LLM and embedding spend on the usage ledger.
func (s *Service) SetUsageRecorder(r usage.Recorder) {
	if s == nil {
		return
	}
	s.usage = r
	s.extractor.usage = r
	s.updater.usage = r
	s.compactor.usage = r
	s.applyEmbedder(s.retriever.embedder)
}

func (s *Service) applyEmbedder(embedder Embedder) {
	wrapped := wrapEmbedder(embedder, s.usage, s.settings.Embedding)
	s.retriever.embedder = wrapped
	s.updater.embedder = wrapped
	s.compactor.embedder = wrapped
}

func embeddingSettingsEqual(a, b EmbeddingSettings) bool {
	return a.Provider == b.Provider && a.Model == b.Model && a.BaseURL == b.BaseURL && a.APIKey == b.APIKey
}

func (s *Service) Enabled() bool { return s.settings.Enabled }

// SetCompactionCompletedNotifier registers the runtime notification bridge.
func (s *Service) SetCompactionCompletedNotifier(notify func(CompactionResult)) {
	s.onCompactionCompleted = notify
}

func (s *Service) notifyCompactionCompleted(result CompactionResult) {
	if s.onCompactionCompleted != nil {
		s.onCompactionCompleted(result)
	}
}

// RetrieveForTurn returns the canonical, budgeted records for one prompt.
func (s *Service) RetrieveForTurn(ctx context.Context, sessionID, query string, tokenAllowance int) (PromptMemories, error) {
	ctx = usage.WithScope(ctx, workspaceForSession(ctx, s.extractor.sessions, sessionID), sessionID)
	if !s.settings.Enabled || !s.settings.AutoRetrieve {
		logging.L().Info("memory.retrieve.skipped", "enabled", s.settings.Enabled, "auto_retrieve", s.settings.AutoRetrieve)
		return PromptMemories{}, nil
	}
	mems, err := s.retriever.retrievePools(ctx, query, tokenAllowance)
	if err != nil {
		logging.L().Error("memory.retrieve.failed", "error", err)
		return PromptMemories{}, err
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

// RecentTaskOutcomes returns recent task outcomes for continuity across job runs.
func (s *Service) RecentTaskOutcomes(ctx context.Context, limit int) ([]ScoredMemory, error) {
	if !s.settings.Enabled {
		logging.L().Info("memory.task_outcomes.skipped", "enabled", false)
		return nil, nil
	}
	if limit <= 0 {
		limit = s.settings.TaskOutcomeLimit
		if limit <= 0 {
			limit = DefaultSettings().TaskOutcomeLimit
		}
	}
	started := time.Now()
	recs, err := s.store.listRecentByKind(ctx, "task_outcome", limit)
	if err != nil {
		logging.L().Error("memory.task_outcomes.failed", "limit", limit, "error", err)
		return nil, err
	}
	now := time.Now()
	out := make([]ScoredMemory, len(recs))
	for i, rec := range recs {
		out[i] = ScoredMemory{Record: rec, EffectiveWeight: EffectiveWeight(rec, now, s.settings.Lifecycle)}
		_ = s.store.touchAccess(ctx, rec.ID)
		_ = s.store.logEvent(ctx, rec.ID, "task_outcome_inject", "")
	}
	logging.L().Info("memory.task_outcomes.loaded", "count", len(out), "limit", limit, "duration_ms", time.Since(started).Milliseconds())
	return out, nil
}

// SearchTaskOutcomes searches active task outcome memories for the explicit recall tool.
func (s *Service) SearchTaskOutcomes(ctx context.Context, query string, limit int) ([]ScoredMemory, error) {
	if !s.settings.Enabled {
		logging.L().Info("memory.task_outcome_search.skipped", "enabled", false)
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	results, err := s.retriever.searchTaskMemories(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	logging.L().Info("memory.task_outcome_search.completed", "count", len(results), "limit", limit)
	return results, nil
}

func (s *Service) CompactPreferenceCategory(ctx context.Context, category string) error {
	category = normalizePreferenceCategory("preference", "", category)
	active, err := s.store.listActivePreferencesByCategory(ctx, category)
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
		if recs[i].ApplicationPolicy != recs[j].ApplicationPolicy {
			return recs[i].ApplicationPolicy == ApplicationAlways
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
		if rec.RetentionPolicy == RetentionProtected || rec.ApplicationPolicy == ApplicationAlways {
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
	extractCtx := usage.WithScope(ctx, workspaceForSession(ctx, s.extractor.sessions, sessionID), sessionID)
	changes, err := s.extractor.extractAfterTurn(extractCtx, sessionID, model, llmProvider)
	if err != nil {
		logging.L().Error("memory.extract.failed", "session", sessionID, "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return changes, err
	}
	logging.L().Info("memory.extract.completed", "session", sessionID, "changes", len(changes), "duration_ms", time.Since(started).Milliseconds())
	for _, change := range changes {
		if change.Kind == "preference" {
			_ = s.CompactPreferenceCategory(extractCtx, change.PreferenceCategory)
		}
	}
	if s.settings.Lifecycle.CompactionOnExtract {
		if err := s.RunLifecycle(extractCtx); err != nil {
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
	before := count
	lc := s.settings.Lifecycle
	if err := s.compactor.forgetDecayed(ctx); err != nil {
		logging.L().Error("memory.lifecycle.failed", "active_count", count, "error", err)
		return err
	}
	count, err = s.store.countActive(ctx)
	if err != nil {
		logging.L().Error("memory.lifecycle.recount_failed", "error", err)
		return err
	}
	if int(count) >= lc.MaxMemories {
		err := s.compactor.run(ctx)
		if err != nil {
			logging.L().Error("memory.compact.failed", "active_count", count, "max_memories", lc.MaxMemories, "duration_ms", time.Since(started).Milliseconds(), "error", err)
			return err
		}
		after, err := s.store.countActive(ctx)
		if err != nil {
			logging.L().Error("memory.compact.result_count_failed", "trigger", "automatic", "error", err)
			return err
		}
		s.notifyCompactionCompleted(CompactionResult{Before: before, After: after, Trigger: "automatic"})
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

// Compact runs compaction immediately and reports the active count change.
func (s *Service) Compact(ctx context.Context) (CompactionResult, error) {
	started := time.Now()
	before, err := s.store.countActive(ctx)
	if err != nil {
		return CompactionResult{}, err
	}
	if err := s.compactor.run(ctx); err != nil {
		logging.L().Error("memory.compact.failed", "manual", true, "duration_ms", time.Since(started).Milliseconds(), "error", err)
		return CompactionResult{}, err
	}
	after, err := s.store.countActive(ctx)
	if err != nil {
		return CompactionResult{}, err
	}
	result := CompactionResult{Before: before, After: after, Trigger: "manual"}
	s.notifyCompactionCompleted(result)
	logging.L().Info("memory.compact.completed", "manual", true, "duration_ms", time.Since(started).Milliseconds())
	return result, nil
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
