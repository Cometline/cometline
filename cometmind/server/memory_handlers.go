package server

import (
	"net/http"
	"strconv"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/gin-gonic/gin"
)

type memoryResource struct {
	ID              string   `json:"id"`
	Scope           string   `json:"scope"`
	Kind            string   `json:"kind"`
	Content         string   `json:"content"`
	Source          string   `json:"source"`
	BaseWeight      float64  `json:"base_weight"`
	EffectiveWeight float64  `json:"effective_weight"`
	AccessCount     int64    `json:"access_count"`
	Pinned          bool     `json:"pinned"`
	LastAccessedAt  *int64   `json:"last_accessed_at,omitempty"`
	CreatedAt       int64    `json:"created_at"`
	UpdatedAt       int64    `json:"updated_at"`
	Similarity      *float64 `json:"similarity,omitempty"`
}

type createMemoryRequest struct {
	Content    string  `json:"content"`
	Kind       string  `json:"kind"`
	Pinned     bool    `json:"pinned"`
	BaseWeight float64 `json:"base_weight"`
}

type updateMemoryRequest struct {
	Content    string   `json:"content"`
	Kind       string   `json:"kind"`
	Pinned     *bool    `json:"pinned"`
	BaseWeight *float64 `json:"base_weight"`
}

type searchMemoryRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

type purgeMemoryRequest struct {
	OlderThanDays int `json:"older_than_days"`
}

type purgeMemoryResponse struct {
	Status             string `json:"status"`
	MemoriesPurged     int    `json:"memories_purged"`
	MemoryEventsPurged int    `json:"memory_events_purged"`
}

func scoredToResource(sm memory.ScoredMemory) memoryResource {
	var last *int64
	if sm.LastAccessedAt != nil {
		ms := sm.LastAccessedAt.UnixMilli()
		last = &ms
	}
	res := memoryResource{
		ID:              sm.ID,
		Scope:           sm.Scope,
		Kind:            sm.Kind,
		Content:         sm.Content,
		Source:          sm.Source,
		BaseWeight:      sm.BaseWeight,
		EffectiveWeight: sm.EffectiveWeight,
		AccessCount:     sm.AccessCount,
		Pinned:          sm.Pinned,
		LastAccessedAt:  last,
		CreatedAt:       sm.CreatedAt.UnixMilli(),
		UpdatedAt:       sm.UpdatedAt.UnixMilli(),
	}
	if sm.Similarity > 0 {
		s := sm.Similarity
		res.Similarity = &s
	}
	return res
}

func recordToResource(rec memory.Record, ew float64) memoryResource {
	return scoredToResource(memory.ScoredMemory{Record: rec, EffectiveWeight: ew})
}

func (a *App) handleListMemories(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusOK, gin.H{"memories": []memoryResource{}})
		return
	}
	mems, err := a.memory.ListActive(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]memoryResource, len(mems))
	for i, m := range mems {
		out[i] = scoredToResource(m)
	}
	c.JSON(http.StatusOK, gin.H{"memories": out})
}

func (a *App) handleCreateMemory(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory disabled"})
		return
	}
	var req createMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rec, err := a.memory.CreateManual(c.Request.Context(), req.Content, req.Kind, req.Pinned, req.BaseWeight)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, recordToResource(rec, rec.BaseWeight))
}

func (a *App) handlePatchMemory(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory disabled"})
		return
	}
	var req updateMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rec, err := a.memory.UpdateManual(c.Request.Context(), c.Param("id"), req.Content, req.Kind, req.Pinned, req.BaseWeight)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, recordToResource(rec, rec.BaseWeight))
}

func (a *App) handleDeleteMemory(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory disabled"})
		return
	}
	if err := a.memory.Delete(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *App) handleSearchMemories(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusOK, gin.H{"memories": []memoryResource{}})
		return
	}
	var req searchMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	mems, err := a.memory.Search(c.Request.Context(), req.Query, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]memoryResource, len(mems))
	for i, m := range mems {
		out[i] = scoredToResource(m)
	}
	c.JSON(http.StatusOK, gin.H{"memories": out})
}

func (a *App) handleGetMemorySettings(c *gin.Context) {
	c.JSON(http.StatusOK, a.config.EffectiveMemoryConfig())
}

func (a *App) handlePutMemorySettings(c *gin.Context) {
	var req config.MemoryConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if a.memory != nil {
		prev := a.config.Memory
		a.config.Memory = req
		prospective := a.config.MemorySettings()
		a.config.Memory = prev
		preview, err := a.memory.PreviewReembed(c.Request.Context(), prospective.Embedding)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if preview.MigrationNeeded {
			c.JSON(http.StatusConflict, gin.H{
				"error":   "embedding model change requires re-embed migration",
				"preview": preview,
			})
			return
		}
		a.config.Memory = req
		if err := a.memory.UpdateSettings(a.config.MemorySettings()); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if p, err := provider.NewMemoryLLM(a.config); err != nil {
			logging.L().Warn("memory.settings.provider_refresh_failed", "error", err)
		} else {
			a.memory.SetProvider(p)
		}
		c.JSON(http.StatusOK, a.config.EffectiveMemoryConfig())
		return
	}
	a.config.Memory = req
	c.JSON(http.StatusOK, a.config.EffectiveMemoryConfig())
}

func embeddingSettingsFromRequest(cfg *config.Config, emb config.MemoryEmbeddingConfig) memory.EmbeddingSettings {
	prev := cfg.Memory
	cfg.Memory.Embedding = emb
	out := cfg.MemorySettings().Embedding
	cfg.Memory = prev
	return out
}

func (a *App) handlePreviewMemoryReembed(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory disabled"})
		return
	}
	var req config.MemoryEmbeddingConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	preview, err := a.memory.PreviewReembed(c.Request.Context(), embeddingSettingsFromRequest(a.config, req))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, preview)
}

func (a *App) handleGetMemoryReembedJob(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory disabled"})
		return
	}
	job, err := a.memory.CurrentReembedJob(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if job == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (a *App) handleStartMemoryReembed(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory disabled"})
		return
	}
	var req struct {
		Embedding config.MemoryEmbeddingConfig `json:"embedding"`
		Force     bool                         `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	target := embeddingSettingsFromRequest(a.config, req.Embedding)
	job, err := a.memory.StartReembed(c.Request.Context(), target, req.Force)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Persist the intended embedding in config; retrieval switches when the job completes.
	a.config.Memory.Embedding = req.Embedding
	c.JSON(http.StatusOK, job)
}

func (a *App) handleCancelMemoryReembed(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory disabled"})
		return
	}
	job, err := a.memory.CancelReembed(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if job == nil {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	c.JSON(http.StatusOK, job)
}

func (a *App) handlePurgeMemory(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory disabled"})
		return
	}
	var req purgeMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	memories, events, err := a.memory.PurgeArchived(c.Request.Context(), req.OlderThanDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, purgeMemoryResponse{
		Status:             "ok",
		MemoriesPurged:     memories,
		MemoryEventsPurged: events,
	})
}

func (a *App) handleCompactMemory(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "memory disabled"})
		return
	}
	result, err := a.memory.Compact(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"before":  result.Before,
		"after":   result.After,
		"trigger": result.Trigger,
	})
}

func (a *App) handleCompactPreview(c *gin.Context) {
	if a.memory == nil {
		c.JSON(http.StatusOK, gin.H{"to_forget": []memoryResource{}, "to_merge": [][]memoryResource{}})
		return
	}
	preview, err := a.memory.CompactPreview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	forget := make([]memoryResource, len(preview.ToForget))
	for i, m := range preview.ToForget {
		forget[i] = scoredToResource(m)
	}
	merge := make([][]memoryResource, len(preview.ToMerge))
	for i, cluster := range preview.ToMerge {
		merge[i] = make([]memoryResource, len(cluster))
		for j, rec := range cluster {
			merge[i][j] = recordToResource(rec, rec.BaseWeight)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"to_forget":    forget,
		"to_merge":     merge,
		"active":       preview.Active,
		"max_memories": preview.MaxMemories,
	})
}

func parseMemoryLimit(c *gin.Context, fallback int) int {
	if raw := c.Query("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}
