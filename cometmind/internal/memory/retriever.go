package memory

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/logging"
)

const absoluteMaxRetrieved = 20

type retriever struct {
	store    *store
	embedder Embedder
	settings Settings
}

func (r *retriever) retrieve(ctx context.Context, query string, maxN int, threshold float64) ([]ScoredMemory, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	memories, err := r.store.listActive(ctx)
	if err != nil {
		return nil, err
	}
	vecs, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	var qvec []float32
	if len(vecs) > 0 {
		qvec = vecs[0]
	}
	fts, err := r.store.searchFTS(ctx, query, retrievalPoolSize)
	if err != nil {
		return nil, err
	}
	all := r.rank(memories, qvec, fts, threshold, nil)
	if maxN <= 0 {
		maxN = r.settings.MaxRetrieved
	}
	if maxN > 0 && len(all) > maxN {
		all = all[:maxN]
	}
	for _, item := range all {
		r.touch(ctx, item, "search")
	}
	return all, nil
}

func (r *retriever) retrievePools(ctx context.Context, query string, tokenAllowance int) (PromptMemories, error) {
	started := time.Now()
	query = strings.TrimSpace(query)
	if query == "" {
		return PromptMemories{}, nil
	}
	memories, err := r.store.listActive(ctx)
	if err != nil {
		return PromptMemories{}, err
	}
	if len(memories) == 0 {
		return PromptMemories{}, nil
	}
	vecs, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return PromptMemories{}, err
	}
	var qvec []float32
	if len(vecs) > 0 {
		qvec = vecs[0]
	}
	fts, err := r.store.searchFTS(ctx, query, retrievalPoolSize)
	if err != nil {
		return PromptMemories{}, err
	}

	threshold := r.settings.SimilarityThreshold
	alwaysPreferences := make([]ScoredMemory, 0, defaultBaselinePreferenceLimit)
	for _, rec := range memories {
		if rec.Kind == "preference" && rec.ApplicationPolicy == ApplicationAlways {
			alwaysPreferences = append(alwaysPreferences, scoredWithoutMatch(rec, r.settings))
		}
	}
	sort.Slice(alwaysPreferences, func(i, j int) bool {
		if !alwaysPreferences[i].UpdatedAt.Equal(alwaysPreferences[j].UpdatedAt) {
			return alwaysPreferences[i].UpdatedAt.After(alwaysPreferences[j].UpdatedAt)
		}
		return alwaysPreferences[i].EffectiveWeight > alwaysPreferences[j].EffectiveWeight
	})
	if len(alwaysPreferences) > defaultBaselinePreferenceLimit {
		alwaysPreferences = alwaysPreferences[:defaultBaselinePreferenceLimit]
	}

	preferenceCandidates := r.rank(memories, qvec, fts, threshold, func(rec Record) bool {
		return rec.Kind == "preference" && rec.ApplicationPolicy == ApplicationRelevant
	})
	preferences := append([]ScoredMemory(nil), alwaysPreferences...)
	preferences = appendCategoryDiverse(preferences, preferenceCandidates, defaultBaselinePreferenceLimit)

	taskLimit := r.settings.TaskOutcomeLimit
	if taskLimit <= 0 {
		taskLimit = DefaultSettings().TaskOutcomeLimit
	}
	tasks := r.rank(memories, qvec, fts, threshold, func(rec Record) bool {
		return rec.Kind == "task_outcome" || rec.Kind == "task_summary"
	})
	if len(tasks) > taskLimit {
		tasks = tasks[:taskLimit]
	}

	semanticLimit := r.settings.MaxRetrieved
	if semanticLimit <= 0 {
		semanticLimit = DefaultSettings().MaxRetrieved
	}
	if semanticLimit > absoluteMaxRetrieved {
		semanticLimit = absoluteMaxRetrieved
	}
	semantic := r.rank(memories, qvec, fts, threshold, func(rec Record) bool {
		return rec.Kind != "preference" && rec.Kind != "task_outcome" && rec.Kind != "task_summary"
	})
	if len(semantic) > semanticLimit {
		semantic = semantic[:semanticLimit]
	}

	prompt := NewPromptMemories(preferences, tasks, semantic).WithinTokenAllowance(tokenAllowance)
	for _, item := range prompt.Records {
		r.touch(ctx, item.ScoredMemory, "inject:"+string(item.Bucket))
	}
	logging.L().Info("memory.retrieve.completed", "active_count", len(memories), "preferences", prompt.Count(BucketPreference), "task_outcomes", prompt.Count(BucketTaskOutcome), "semantic", prompt.Count(BucketSemantic), "duration_ms", time.Since(started).Milliseconds())
	return prompt, nil
}

func scoredWithoutMatch(rec Record, settings Settings) ScoredMemory {
	return ScoredMemory{Record: rec, EffectiveWeight: EffectiveWeight(rec, time.Now(), settings.Lifecycle)}
}

func (r *retriever) rank(memories []Record, qvec []float32, ftsRanked []string, threshold float64, include func(Record) bool) []ScoredMemory {
	recordByID := make(map[string]Record, len(memories))
	simByID := make(map[string]float64, len(memories))
	for _, rec := range memories {
		if include != nil && !include(rec) {
			continue
		}
		recordByID[rec.ID] = rec
		if len(qvec) > 0 && len(rec.Embedding) > 0 {
			simByID[rec.ID] = cosineSimilarity(qvec, rec.Embedding)
		}
	}
	vectorRanked := topNBySimilarity(simByID, retrievalPoolSize)
	filteredFTS := make([]string, 0, len(ftsRanked))
	ftsHit := make(map[string]struct{}, len(ftsRanked))
	for _, id := range ftsRanked {
		if _, ok := recordByID[id]; !ok {
			continue
		}
		filteredFTS = append(filteredFTS, id)
		ftsHit[id] = struct{}{}
	}
	rrf := reciprocalRankFusion(vectorRanked, filteredFTS)
	now := time.Now()
	result := make([]ScoredMemory, 0, len(rrf))
	for id, score := range rrf {
		rec := recordByID[id]
		sim := simByID[id]
		if sim < threshold {
			if _, ok := ftsHit[id]; !ok {
				continue
			}
		}
		weight := EffectiveWeight(rec, now, r.settings.Lifecycle)
		result = append(result, ScoredMemory{Record: rec, Similarity: sim, EffectiveWeight: weight, RetrievalScore: score * weight})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].RetrievalScore > result[j].RetrievalScore })
	return result
}

func appendCategoryDiverse(selected, candidates []ScoredMemory, limit int) []ScoredMemory {
	seenID := make(map[string]struct{}, len(selected))
	seenCategory := make(map[string]struct{}, len(selected))
	for _, item := range selected {
		seenID[item.ID] = struct{}{}
		seenCategory[item.PreferenceCategory] = struct{}{}
	}
	for _, diverseOnly := range []bool{true, false} {
		for _, item := range candidates {
			if len(selected) >= limit {
				return selected
			}
			if _, ok := seenID[item.ID]; ok {
				continue
			}
			_, categorySeen := seenCategory[item.PreferenceCategory]
			if diverseOnly && categorySeen {
				continue
			}
			selected = append(selected, item)
			seenID[item.ID] = struct{}{}
			seenCategory[item.PreferenceCategory] = struct{}{}
		}
	}
	return selected
}

func (r *retriever) touch(ctx context.Context, item ScoredMemory, action string) {
	_ = r.store.touchAccess(ctx, item.ID)
	_ = r.store.logEvent(ctx, item.ID, action, "")
}

func (r *retriever) search(ctx context.Context, query string, maxN int) ([]ScoredMemory, error) {
	return r.retrieve(ctx, query, maxN, 0)
}

func (r *retriever) searchTaskMemories(ctx context.Context, query string, maxN int) ([]ScoredMemory, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}
	memories, err := r.store.listActive(ctx)
	if err != nil {
		return nil, err
	}
	vecs, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	var qvec []float32
	if len(vecs) > 0 {
		qvec = vecs[0]
	}
	fts, err := r.store.searchFTS(ctx, query, retrievalPoolSize)
	if err != nil {
		return nil, err
	}
	results := r.rank(memories, qvec, fts, 0, func(rec Record) bool {
		return rec.Kind == "task_outcome" || rec.Kind == "task_summary"
	})
	if len(results) > maxN {
		results = results[:maxN]
	}
	for _, item := range results {
		r.touch(ctx, item, "task_outcome_recall")
	}
	return results, nil
}

func (r *retriever) bestMatch(ctx context.Context, vec []float32, kind, scope, preferenceCategory string) (Record, float64, error) {
	memories, err := r.store.listActive(ctx)
	if err != nil {
		return Record{}, 0, err
	}
	var best Record
	var bestSim float64
	for _, m := range memories {
		if m.Kind != kind || m.Scope != scope {
			continue
		}
		if kind == "preference" && m.PreferenceCategory != preferenceCategory {
			continue
		}
		if len(m.Embedding) == 0 {
			continue
		}
		sim := cosineSimilarity(vec, m.Embedding)
		if sim > bestSim {
			bestSim = sim
			best = m
		}
	}
	return best, bestSim, nil
}
