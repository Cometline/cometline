package memory

import (
	"context"
	"sort"
	"strings"
	"time"
)

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
	vecs, err := r.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, nil
	}
	qvec := vecs[0]

	memories, err := r.store.listActive(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var scored []ScoredMemory
	for _, m := range memories {
		if len(m.Embedding) == 0 {
			continue
		}
		sim := cosineSimilarity(qvec, m.Embedding)
		if sim < threshold {
			continue
		}
		ew := EffectiveWeight(m, now, r.settings.Lifecycle)
		scored = append(scored, ScoredMemory{
			Record:          m,
			Similarity:      sim,
			EffectiveWeight: ew,
			RetrievalScore:  RetrievalScore(sim, ew),
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

	for _, sm := range scored {
		_ = r.store.touchAccess(ctx, sm.ID)
		_ = r.store.logEvent(ctx, sm.ID, "inject", "")
	}
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
