package memory

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/logging"
)

type retriever struct {
	store    *store
	embedder Embedder
	settings Settings
}

func (r *retriever) retrieve(ctx context.Context, query string, maxN int, threshold float64) ([]ScoredMemory, error) {
	started := time.Now()
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	phaseStarted := time.Now()
	memories, err := r.store.listActive(ctx)
	if err != nil {
		return nil, err
	}
	logging.L().Info("memory.retrieve.list_active.completed", "active_count", len(memories), "duration_ms", time.Since(phaseStarted).Milliseconds())
	if len(memories) == 0 {
		return nil, nil
	}

	recordByID := make(map[string]Record, len(memories))
	for _, m := range memories {
		recordByID[m.ID] = m
	}

	simByID := make(map[string]float64)
	phaseStarted = time.Now()
	vecs, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	logging.L().Info("memory.retrieve.embed.completed", "duration_ms", time.Since(phaseStarted).Milliseconds(), "query_bytes", len(query), "model", r.embedder.Model())
	if len(vecs) > 0 {
		phaseStarted = time.Now()
		qvec := vecs[0]
		for _, m := range memories {
			if len(m.Embedding) == 0 {
				continue
			}
			simByID[m.ID] = cosineSimilarity(qvec, m.Embedding)
		}
		logging.L().Info("memory.retrieve.vector_rank.completed", "candidates", len(simByID), "duration_ms", time.Since(phaseStarted).Milliseconds())
	}

	vectorRanked := topNBySimilarity(simByID, retrievalPoolSize)

	phaseStarted = time.Now()
	ftsRanked, err := r.store.searchFTS(ctx, query, retrievalPoolSize)
	if err != nil {
		return nil, err
	}
	logging.L().Info("memory.retrieve.fts.completed", "hits", len(ftsRanked), "duration_ms", time.Since(phaseStarted).Milliseconds())
	ftsHit := make(map[string]struct{}, len(ftsRanked))
	for _, id := range ftsRanked {
		ftsHit[id] = struct{}{}
	}

	rrfScores := reciprocalRankFusion(vectorRanked, ftsRanked)
	if len(rrfScores) == 0 {
		return nil, nil
	}

	now := time.Now()
	var scored []ScoredMemory
	for id, rrfScore := range rrfScores {
		m, ok := recordByID[id]
		if !ok {
			continue
		}
		sim := simByID[id]
		if sim < threshold {
			if _, fts := ftsHit[id]; !fts {
				continue
			}
		}
		ew := EffectiveWeight(m, now, r.settings.Lifecycle)
		scored = append(scored, ScoredMemory{
			Record:          m,
			Similarity:      sim,
			EffectiveWeight: ew,
			RetrievalScore:  rrfScore * ew,
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].RetrievalScore > scored[j].RetrievalScore
	})
	if maxN <= 0 {
		maxN = r.settings.MaxRetrieved
	}
	if maxN > 0 && len(scored) > maxN {
		scored = scored[:maxN]
	}

	phaseStarted = time.Now()
	for _, sm := range scored {
		_ = r.store.touchAccess(ctx, sm.ID)
		_ = r.store.logEvent(ctx, sm.ID, "inject", "")
	}
	logging.L().Info("memory.retrieve.touch_completed", "count", len(scored), "duration_ms", time.Since(phaseStarted).Milliseconds())
	logging.L().Info("memory.retrieve.completed", "count", len(scored), "max", maxN, "threshold", threshold, "duration_ms", time.Since(started).Milliseconds())
	return scored, nil
}

func (r *retriever) search(ctx context.Context, query string, maxN int) ([]ScoredMemory, error) {
	return r.retrieve(ctx, query, maxN, 0)
}

func (r *retriever) bestMatch(ctx context.Context, vec []float32) (Record, float64, error) {
	memories, err := r.store.listActive(ctx)
	if err != nil {
		return Record{}, 0, err
	}
	var best Record
	var bestSim float64
	for _, m := range memories {
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
