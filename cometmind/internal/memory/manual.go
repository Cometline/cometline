package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/logging"
	"go.uber.org/zap"
)

// ListActive returns active memories with effective weights.
func (s *Service) ListActive(ctx context.Context) ([]ScoredMemory, error) {
	memories, err := s.store.listActive(ctx)
	if err != nil {
		logging.L().Error("memory.list_active.failed", zap.Error(err))
		return nil, err
	}
	now := time.Now()
	out := make([]ScoredMemory, len(memories))
	for i, m := range memories {
		ew := EffectiveWeight(m, now, s.settings.Lifecycle)
		out[i] = ScoredMemory{Record: m, EffectiveWeight: ew}
	}
	logging.L().Info("memory.list_active.completed", zap.Int("count", len(out)))
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
		logging.L().Error("memory.manual_create.failed", zap.String("kind", rec.Kind), zap.String("application_policy", rec.ApplicationPolicy), zap.String("retention_policy", rec.RetentionPolicy), zap.Error(err))
		return Record{}, err
	}
	_ = s.store.logEvent(ctx, rec.ID, "create", "manual")
	if rec.Kind == "preference" {
		_ = s.CompactPreferenceCategory(ctx, rec.PreferenceCategory)
	}
	logging.L().Info("memory.manual_create.completed", zap.String("memory_id", rec.ID), zap.String("kind", rec.Kind), zap.String("application_policy", rec.ApplicationPolicy), zap.String("retention_policy", rec.RetentionPolicy), zap.Float64("base_weight", rec.BaseWeight))
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
		logging.L().Error("memory.manual_update.failed", zap.String("memory_id", rec.ID), zap.Error(err))
		return Record{}, err
	}
	_ = s.store.logEvent(ctx, rec.ID, "manual_update", "")
	if rec.Kind == "preference" {
		_ = s.CompactPreferenceCategory(ctx, rec.PreferenceCategory)
	}
	logging.L().Info("memory.manual_update.completed", zap.String("memory_id", rec.ID), zap.String("kind", rec.Kind), zap.String("application_policy", rec.ApplicationPolicy), zap.String("retention_policy", rec.RetentionPolicy), zap.Float64("base_weight", rec.BaseWeight))
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
		logging.L().Error("memory.manual_delete.failed", zap.String("memory_id", id), zap.Error(err))
		return Record{}, err
	}
	if err := s.store.logEvent(ctx, id, "manual_delete", ""); err != nil {
		logging.L().Error("memory.manual_delete_event.failed", zap.String("memory_id", id), zap.Error(err))
		return Record{}, err
	}
	logging.L().Info("memory.manual_delete.completed", zap.String("memory_id", id))
	return rec, nil
}
