package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/oklog/ulid/v2"
)

type CompactPreview struct {
	ToForget    []ScoredMemory `json:"to_forget"`
	ToMerge     [][]Record     `json:"to_merge"`
	Active      int64          `json:"active"`
	MaxMemories int            `json:"max_memories"`
}

type compactor struct {
	store    *store
	embedder Embedder
	provider cometsdk.Provider
	settings Settings
}

func (c *compactor) preview(ctx context.Context) (CompactPreview, error) {
	memories, err := c.store.listActive(ctx)
	if err != nil {
		return CompactPreview{}, err
	}
	now := time.Now()
	lc := c.settings.Lifecycle
	var scored []ScoredMemory
	for _, m := range memories {
		ew := EffectiveWeight(m, now, lc)
		scored = append(scored, ScoredMemory{Record: m, EffectiveWeight: ew})
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].EffectiveWeight < scored[j].EffectiveWeight
	})

	var forget []ScoredMemory
	for _, sm := range scored {
		if sm.EffectiveWeight < lc.ForgetThreshold && !sm.Pinned {
			forget = append(forget, sm)
		}
	}

	clusters := c.clusterLowWeight(scored, lc)
	return CompactPreview{
		ToForget:    forget,
		ToMerge:     clusters,
		Active:      int64(len(memories)),
		MaxMemories: lc.MaxMemories,
	}, nil
}

func (c *compactor) run(ctx context.Context) error {
	lc := c.settings.Lifecycle
	target := int(float64(lc.MaxMemories) * lc.CompactionTargetRatio)
	if target <= 0 {
		target = lc.MaxMemories
	}

	if err := c.forgetDecayed(ctx); err != nil {
		return err
	}

	for {
		count, err := c.store.countActive(ctx)
		if err != nil {
			return err
		}
		if int(count) <= target {
			return nil
		}
		before := count

		if err := c.mergePass(ctx); err != nil {
			return err
		}
		count, err = c.store.countActive(ctx)
		if err != nil {
			return err
		}
		if int(count) <= target {
			return nil
		}
		if err := c.forceForget(ctx, int(count)-target); err != nil {
			return err
		}
		count, err = c.store.countActive(ctx)
		if err != nil {
			return err
		}
		if int(count) <= target {
			return nil
		}
		// Avoid infinite loop if nothing changes.
		if count >= before {
			break
		}
	}
	return nil
}

func (c *compactor) forgetDecayed(ctx context.Context) error {
	memories, err := c.store.listActive(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	lc := c.settings.Lifecycle
	for _, m := range memories {
		if m.Pinned {
			continue
		}
		ew := EffectiveWeight(m, now, lc)
		if ew < lc.ForgetThreshold {
			if err := c.store.archive(ctx, m.ID, "decayed", ""); err != nil {
				return err
			}
			_ = c.store.logEvent(ctx, m.ID, "forget", encodeDetail(map[string]float64{"effective_weight": ew}))
		}
	}
	return nil
}

func (c *compactor) forceForget(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}
	memories, err := c.store.listActive(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	var scored []ScoredMemory
	for _, m := range memories {
		if m.Pinned {
			continue
		}
		scored = append(scored, ScoredMemory{
			Record:          m,
			EffectiveWeight: EffectiveWeight(m, now, c.settings.Lifecycle),
		})
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].EffectiveWeight < scored[j].EffectiveWeight
	})
	if n > len(scored) {
		n = len(scored)
	}
	for i := 0; i < n; i++ {
		id := scored[i].ID
		if err := c.store.archive(ctx, id, "compaction", ""); err != nil {
			return err
		}
		_ = c.store.logEvent(ctx, id, "compact_forget", "")
	}
	return nil
}

func (c *compactor) mergePass(ctx context.Context) error {
	memories, err := c.store.listActive(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	var scored []ScoredMemory
	for _, m := range memories {
		scored = append(scored, ScoredMemory{
			Record:          m,
			EffectiveWeight: EffectiveWeight(m, now, c.settings.Lifecycle),
		})
	}
	clusters := c.clusterLowWeight(scored, c.settings.Lifecycle)
	for _, cluster := range clusters {
		if len(cluster) < 2 {
			continue
		}
		if err := c.mergeCluster(ctx, cluster); err != nil {
			return err
		}
	}
	return nil
}

func (c *compactor) clusterLowWeight(scored []ScoredMemory, lc LifecycleSettings) [][]Record {
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].EffectiveWeight < scored[j].EffectiveWeight
	})
	n := len(scored) / 5
	if n < 2 {
		n = 2
	}
	if n > len(scored) {
		n = len(scored)
	}
	low := scored[:n]

	used := make(map[string]bool)
	var clusters [][]Record
	for _, seed := range low {
		if used[seed.ID] || len(seed.Embedding) == 0 {
			continue
		}
		cluster := []Record{seed.Record}
		used[seed.ID] = true
		for _, other := range low {
			if used[other.ID] || len(other.Embedding) == 0 {
				continue
			}
			if cosineSimilarity(seed.Embedding, other.Embedding) >= 0.80 {
				cluster = append(cluster, other.Record)
				used[other.ID] = true
			}
		}
		if len(cluster) >= 2 {
			clusters = append(clusters, cluster)
		}
	}
	return clusters
}

func (c *compactor) mergeCluster(ctx context.Context, cluster []Record) error {
	var b strings.Builder
	maxWeight := 0.0
	ids := make([]string, len(cluster))
	for i, m := range cluster {
		ids[i] = m.ID
		b.WriteString("- ")
		b.WriteString(m.Content)
		b.WriteString("\n")
		if m.BaseWeight > maxWeight {
			maxWeight = m.BaseWeight
		}
	}
	prompt := fmt.Sprintf(`Merge these related memories into one concise factual memory. Preserve specific details.
Return JSON: {"content":"..."}

Memories:
%s`, b.String())

	var out struct {
		Content string `json:"content"`
	}
	req := &cometsdk.Request{
		Model:  extractionModel(c.settings),
		System: "You consolidate memories. Output JSON only.",
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: prompt}},
		}},
		MaxTokens: 1024,
	}
	if err := llm.GenerateJSON(ctx, c.provider, req, &out); err != nil {
		return err
	}
	content := strings.TrimSpace(out.Content)
	if content == "" {
		return nil
	}
	vecs, err := c.embedder.Embed(ctx, content)
	if err != nil {
		return err
	}
	if len(vecs) == 0 {
		return nil
	}
	now := time.Now()
	newID := ulid.Make().String()
	rec := Record{
		ID:             newID,
		Scope:          "global",
		Kind:           "fact",
		Content:        content,
		Embedding:      vecs[0],
		EmbeddingModel: c.embedder.Model(),
		Source:         "compacted",
		BaseWeight:     maxWeight,
		LastAccessedAt: &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := c.store.insert(ctx, rec); err != nil {
		return err
	}
	for _, id := range ids {
		if err := c.store.archive(ctx, id, "compaction", newID); err != nil {
			return err
		}
		_ = c.store.logEvent(ctx, id, "compact_merge", encodeDetail(map[string]any{"merged_into": newID, "cluster": ids}))
	}
	return nil
}

func extractionModel(s Settings) string {
	if strings.TrimSpace(s.ExtractionModel) != "" {
		return s.ExtractionModel
	}
	return "claude-sonnet-4-5"
}
