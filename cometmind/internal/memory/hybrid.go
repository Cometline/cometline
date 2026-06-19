package memory

import (
	"sort"
)

const (
	rrfK              = 60
	retrievalPoolSize = 20
)

// reciprocalRankFusion merges ranked ID lists using standard RRF scoring.
func reciprocalRankFusion(rankings ...[]string) map[string]float64 {
	scores := make(map[string]float64)
	for _, ranking := range rankings {
		for rank, id := range ranking {
			scores[id] += 1.0 / (float64(rrfK) + float64(rank+1))
		}
	}
	return scores
}

// topNBySimilarity returns the top poolSize memory IDs ordered by cosine similarity.
func topNBySimilarity(simByID map[string]float64, poolSize int) []string {
	pairs := make([]struct {
		id  string
		sim float64
	}, 0, len(simByID))
	for id, sim := range simByID {
		pairs = append(pairs, struct {
			id  string
			sim float64
		}{id: id, sim: sim})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].sim > pairs[j].sim
	})
	if poolSize > 0 && len(pairs) > poolSize {
		pairs = pairs[:poolSize]
	}
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.id
	}
	return out
}
