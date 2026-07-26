package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
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
	mu       sync.Mutex
}

func (c *compactor) preview(ctx context.Context) (CompactPreview, error) {
	memories, err := c.store.listCompactionCandidates(ctx, 200)
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
		if sm.EffectiveWeight < lc.ForgetThreshold && compactable(sm.Record) {
			forget = append(forget, sm)
		}
	}

	clusters := c.clusterLowWeight(scored, lc)
	active, err := c.store.countActive(ctx)
	if err != nil {
		return CompactPreview{}, err
	}
	return CompactPreview{
		ToForget:    forget,
		ToMerge:     clusters,
		Active:      active,
		MaxMemories: lc.MaxMemories,
	}, nil
}

func (c *compactor) run(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
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
	memories, err := c.store.listCompactionCandidates(ctx, 200)
	if err != nil {
		return err
	}
	now := time.Now()
	lc := c.settings.Lifecycle
	for _, m := range memories {
		if !compactable(m) {
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
	memories, err := c.store.listCompactionCandidates(ctx, 200)
	if err != nil {
		return err
	}
	now := time.Now()
	var scored []ScoredMemory
	for _, m := range memories {
		if !compactable(m) {
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
	memories, err := c.store.listCompactionCandidates(ctx, 100)
	if err != nil {
		return err
	}
	now := time.Now()
	var scored []ScoredMemory
	for _, m := range memories {
		if !mergeable(m) {
			continue
		}
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
	if n > 100 {
		n = 100
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
			if compatibleForMerge(seed.Record, other.Record) && cosineSimilarity(seed.Embedding, other.Embedding) >= 0.80 {
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
	if len(cluster) < 2 || !mergeable(cluster[0]) {
		return nil
	}
	for _, item := range cluster[1:] {
		if !compatibleForMerge(cluster[0], item) {
			return nil
		}
	}
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
	model := extractionModel(c.settings)
	if model == "" {
		return fmt.Errorf("memory compaction requires a model: set Memory extraction model, or configure an active chat model")
	}
	prompt := fmt.Sprintf(`Merge these related memories into one concise %s memory. Preserve specific details and do not change its kind.
Return JSON: {"content":"..."}

Memories:
%s`, cluster[0].Kind, b.String())

	var out struct {
		Content string `json:"content"`
	}
	req := &cometsdk.Request{
		Model:  model,
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
		ID:                 newID,
		Scope:              cluster[0].Scope,
		Kind:               cluster[0].Kind,
		PreferenceCategory: cluster[0].PreferenceCategory,
		Content:            content,
		Embedding:          vecs[0],
		EmbeddingModel:     c.embedder.Model(),
		Source:             "compacted",
		BaseWeight:         maxWeight,
		ApplicationPolicy:  cluster[0].ApplicationPolicy,
		RetentionPolicy:    cluster[0].RetentionPolicy,
		OriginType:         cluster[0].OriginType,
		OriginID:           cluster[0].OriginID,
		SummaryJSON:        cluster[0].SummaryJSON,
		LastAccessedAt:     &now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	detail := encodeDetail(map[string]any{"merged_into": newID, "cluster": ids})
	return c.store.replaceWithMerged(ctx, rec, cluster, detail)
}

func compactable(m Record) bool {
	if m.RetentionPolicy == RetentionProtected || m.ApplicationPolicy == ApplicationAlways {
		return false
	}
	return m.Kind != "task_outcome" && m.Kind != "task_summary"
}

func mergeable(m Record) bool {
	return compactable(m)
}

func compatibleForMerge(a, b Record) bool {
	if !mergeable(a) || !mergeable(b) || a.Kind != b.Kind || a.Scope != b.Scope ||
		a.ApplicationPolicy != b.ApplicationPolicy || a.RetentionPolicy != b.RetentionPolicy ||
		a.OriginType != b.OriginType || a.OriginID != b.OriginID {
		return false
	}
	if a.Kind == "preference" {
		return a.PreferenceCategory == b.PreferenceCategory && a.ApplicationPolicy == ApplicationRelevant && b.ApplicationPolicy == ApplicationRelevant
	}
	return true
}

func extractionModel(s Settings) string {
	if strings.TrimSpace(s.ExtractionModel) != "" {
		return s.ExtractionModel
	}
	return strings.TrimSpace(s.DefaultModel)
}
