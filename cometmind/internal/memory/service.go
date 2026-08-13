package memory

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/session"
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
	s.retriever.embedder = embedder
	s.updater.embedder = embedder
	s.compactor.embedder = embedder
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
func (s *Service) RetrieveForTurn(ctx context.Context, query string, tokenAllowance int) (PromptMemories, error) {
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
func (s *Service) CreateManual(ctx context.Context, content, kind, applicationPolicy, retentionPolicy string, baseWeight float64) (Record, error) {
	return s.CreateManualWithID(ctx, NewID(), content, kind, applicationPolicy, retentionPolicy, baseWeight)
}

// CreateManualWithID inserts a user-authored memory with a caller-provided id.
// It lets asynchronous callers return an accepted id before embedding finishes.
func (s *Service) CreateManualWithID(ctx context.Context, id, content, kind, applicationPolicy, retentionPolicy string, baseWeight float64) (Record, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return Record{}, fmt.Errorf("content is required")
	}
	if strings.TrimSpace(id) == "" {
		id = NewID()
	}
	normalizedKind := normalizeKind(kind)
	if normalizedKind == "task_outcome" || normalizedKind == "task_summary" {
		return Record{}, fmt.Errorf("task memories require a durable job origin")
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
		ID:                 id,
		Scope:              "global",
		Kind:               normalizedKind,
		PreferenceCategory: normalizePreferenceCategory(kind, content, ""),
		Content:            content,
		Embedding:          vecs[0],
		EmbeddingModel:     s.retriever.embedder.Model(),
		Source:             "manual",
		BaseWeight:         baseWeight,
		ApplicationPolicy:  normalizeApplicationPolicy(kind, applicationPolicy),
		RetentionPolicy:    normalizeRetentionPolicy(retentionPolicy),
		LastAccessedAt:     &now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	applyPolicyInvariants(&rec)
	if err := s.store.insert(ctx, rec); err != nil {
		logging.L().Error("memory.manual_create.failed", "kind", rec.Kind, "application_policy", rec.ApplicationPolicy, "retention_policy", rec.RetentionPolicy, "error", err)
		return Record{}, err
	}
	_ = s.store.logEvent(ctx, rec.ID, "create", "manual")
	if rec.Kind == "preference" {
		_ = s.CompactPreferenceCategory(ctx, rec.PreferenceCategory)
	}
	logging.L().Info("memory.manual_create.completed", "memory_id", rec.ID, "kind", rec.Kind, "application_policy", rec.ApplicationPolicy, "retention_policy", rec.RetentionPolicy, "base_weight", rec.BaseWeight)
	return rec, nil
}

// UpdateManual edits a memory.
func (s *Service) UpdateManual(ctx context.Context, id, content, kind string, applicationPolicy, retentionPolicy *string, baseWeight *float64) (Record, error) {
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
	if applicationPolicy != nil {
		rec.ApplicationPolicy = normalizeApplicationPolicy(rec.Kind, *applicationPolicy)
	}
	if retentionPolicy != nil {
		rec.RetentionPolicy = normalizeRetentionPolicy(*retentionPolicy)
	}
	applyPolicyInvariants(&rec)
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
	logging.L().Info("memory.manual_update.completed", "memory_id", rec.ID, "kind", rec.Kind, "application_policy", rec.ApplicationPolicy, "retention_policy", rec.RetentionPolicy, "base_weight", rec.BaseWeight)
	return rec, nil
}

// Delete removes a memory permanently.
func (s *Service) Delete(ctx context.Context, id string) error {
	_, err := s.DeleteManual(ctx, id)
	return err
}

// DeleteManual permanently removes a memory and returns its previous value so
// callers can describe the change to the user.
func (s *Service) DeleteManual(ctx context.Context, id string) (Record, error) {
	rec, err := s.store.get(ctx, id)
	if err != nil {
		return Record{}, err
	}
	if err := s.store.delete(ctx, id); err != nil {
		logging.L().Error("memory.manual_delete.failed", "memory_id", id, "error", err)
		return Record{}, err
	}
	if err := s.store.logEvent(ctx, id, "manual_delete", ""); err != nil {
		logging.L().Error("memory.manual_delete_event.failed", "memory_id", id, "error", err)
		return Record{}, err
	}
	logging.L().Info("memory.manual_delete.completed", "memory_id", id)
	return rec, nil
}

type MemoryBucket string

const (
	BucketPreference  MemoryBucket = "preference"
	BucketTaskOutcome MemoryBucket = "task_outcome"
	BucketSemantic    MemoryBucket = "semantic"
)

type PromptMemory struct {
	ScoredMemory
	Bucket MemoryBucket
}

type PromptMemories struct {
	Records []PromptMemory
}

func NewPromptMemories(preferences, outcomes, semantic []ScoredMemory) PromptMemories {
	records := make([]PromptMemory, 0, len(preferences)+len(outcomes)+len(semantic))
	seen := make(map[string]struct{}, cap(records))
	for _, group := range []struct {
		bucket MemoryBucket
		items  []ScoredMemory
	}{{BucketPreference, preferences}, {BucketTaskOutcome, outcomes}, {BucketSemantic, semantic}} {
		for _, item := range group.items {
			if _, ok := seen[item.ID]; ok {
				continue
			}
			seen[item.ID] = struct{}{}
			records = append(records, PromptMemory{ScoredMemory: item, Bucket: group.bucket})
		}
	}
	return PromptMemories{Records: records}
}

func (m PromptMemories) Count(bucket MemoryBucket) int {
	count := 0
	for _, item := range m.Records {
		if item.Bucket == bucket {
			count++
		}
	}
	return count
}

func (m PromptMemories) WithinTokenAllowance(allowance int) PromptMemories {
	if allowance <= 0 {
		return PromptMemories{}
	}
	selected := make([]PromptMemory, 0, len(m.Records))
	for _, item := range m.Records {
		candidate := PromptMemories{Records: append(selected, item)}
		if EstimatePromptMemoriesTokens(candidate) > allowance {
			break
		}
		selected = append(selected, item)
	}
	return PromptMemories{Records: selected}
}

// EstimatePromptMemoriesTokens applies the conservative ceil(chars/4) rule to
// the exact suffix sent to the model, including headings and list decoration.
func EstimatePromptMemoriesTokens(mems PromptMemories) int {
	runes := len([]rune(FormatPromptMemories(mems)))
	if runes == 0 {
		return 0
	}
	return (runes + 3) / 4
}

func FormatPromptMemories(mems PromptMemories) string {
	if len(mems.Records) == 0 {
		return ""
	}
	var b strings.Builder
	if mems.Count(BucketPreference) > 0 {
		b.WriteString("\n\n## User preferences\n")
		i := 0
		for _, m := range mems.Records {
			if m.Bucket == BucketPreference {
				i++
				fmt.Fprintf(&b, "%d. %s\n", i, m.Content)
			}
		}
	}
	if mems.Count(BucketTaskOutcome) > 0 {
		b.WriteString("\n\n## Relevant task outcomes\n")
		i := 0
		for _, m := range mems.Records {
			if m.Bucket == BucketTaskOutcome {
				i++
				fmt.Fprintf(&b, "%d. %s\n", i, m.Content)
			}
		}
	}
	if mems.Count(BucketSemantic) > 0 {
		b.WriteString("\n\n## Semantic memories\n")
	}
	i := 0
	for _, m := range mems.Records {
		if m.Bucket == BucketSemantic {
			i++
			fmt.Fprintf(&b, "%d. [%s] %s\n", i, m.Kind, m.Content)
		}
	}
	return b.String()
}

// FormatForPrompt renders injected memories for the system prompt.
func FormatForPrompt(mems []ScoredMemory) string {
	return FormatPromptMemories(NewPromptMemories(nil, nil, mems))
}
