package server

import (
	"errors"
	"net/http"

	"github.com/cometline/cometmind/internal/scheduler"
	"github.com/gin-gonic/gin"
)

type scheduledJobResource struct {
	ID               string `json:"id"`
	Description      string `json:"description"`
	DefinitionOfDone string `json:"definition_of_done"`
	WorkspacePath    string `json:"workspace_path,omitempty"`
	CreatedBy        string `json:"created_by"`
	SourceSessionID  string `json:"source_session_id,omitempty"`
	SourcePlatform   string `json:"source_platform,omitempty"`
	SourceChannelID  string `json:"source_channel_id,omitempty"`
	CronExpr         string `json:"cron_expr,omitempty"`
	RunAt            *int64 `json:"run_at,omitempty"`
	NextRunAt        int64  `json:"next_run_at"`
	LastRunAt        *int64 `json:"last_run_at,omitempty"`
	Enabled          bool   `json:"enabled"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

type createScheduledJobRequest struct {
	Description      string `json:"description"`
	DefinitionOfDone string `json:"definition_of_done"`
	WorkspacePath    string `json:"workspace_path"`
	CreatedBy        string `json:"created_by"`
	SourceSessionID  string `json:"source_session_id"`
	SourcePlatform   string `json:"source_platform"`
	SourceChannelID  string `json:"source_channel_id"`
	CronExpr         string `json:"cron_expr"`
	RunAt            int64  `json:"run_at"`
}

type updateScheduledJobRequest struct {
	Description      string `json:"description"`
	DefinitionOfDone string `json:"definition_of_done"`
	WorkspacePath    string `json:"workspace_path"`
	RunAt            int64  `json:"run_at"`
	Enabled          *bool  `json:"enabled"`
}

func scheduledJobToResource(j scheduler.ScheduledJob) scheduledJobResource {
	return scheduledJobResource{
		ID:               j.ID,
		Description:      j.Description,
		DefinitionOfDone: j.DefinitionOfDone,
		WorkspacePath:    j.WorkspacePath,
		CreatedBy:        j.CreatedBy,
		SourceSessionID:  j.SourceSessionID,
		SourcePlatform:   j.SourcePlatform,
		SourceChannelID:  j.SourceChannelID,
		CronExpr:         j.CronExpr,
		RunAt:            j.RunAt,
		NextRunAt:        j.NextRunAt,
		LastRunAt:        j.LastRunAt,
		Enabled:          j.Enabled,
		CreatedAt:        j.CreatedAt,
		UpdatedAt:        j.UpdatedAt,
	}
}

func writeScheduledJobError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, scheduler.ErrNotFound):
		writeError(c, http.StatusNotFound, "scheduled_job_not_found", err.Error())
	case errors.Is(err, scheduler.ErrInvalidInput):
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
	case errors.Is(err, scheduler.ErrConflict):
		writeError(c, http.StatusConflict, "scheduled_job_conflict", err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
	}
	return true
}

func (a *App) handleListScheduledJobs(c *gin.Context) {
	if a.scheduler == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "scheduler service unavailable")
		return
	}
	items, err := a.scheduler.List(c.Request.Context())
	if err != nil {
		writeScheduledJobError(c, err)
		return
	}
	out := make([]scheduledJobResource, 0, len(items))
	for _, item := range items {
		out = append(out, scheduledJobToResource(item))
	}
	c.JSON(http.StatusOK, gin.H{"scheduled_jobs": out})
}

func (a *App) handleCreateScheduledJob(c *gin.Context) {
	if a.scheduler == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "scheduler service unavailable")
		return
	}
	var req createScheduledJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	item, err := a.scheduler.Create(c.Request.Context(), scheduler.CreateInput(req))
	if err != nil {
		writeScheduledJobError(c, err)
		return
	}
	c.JSON(http.StatusCreated, scheduledJobToResource(item))
}

func (a *App) handleGetScheduledJob(c *gin.Context) {
	if a.scheduler == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "scheduler service unavailable")
		return
	}
	item, err := a.scheduler.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeScheduledJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, scheduledJobToResource(item))
}

func (a *App) handlePatchScheduledJob(c *gin.Context) {
	if a.scheduler == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "scheduler service unavailable")
		return
	}
	current, err := a.scheduler.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeScheduledJobError(c, err)
		return
	}
	var req updateScheduledJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	enabled := current.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	item, err := a.scheduler.Update(c.Request.Context(), current.ID, scheduler.UpdateInput{
		Description:      firstNonEmpty(req.Description, current.Description),
		DefinitionOfDone: firstNonEmpty(req.DefinitionOfDone, current.DefinitionOfDone),
		WorkspacePath:    firstNonEmpty(req.WorkspacePath, current.WorkspacePath),
		RunAt:            firstNonZero(req.RunAt, current.NextRunAt),
		Enabled:          enabled,
	})
	if err != nil {
		writeScheduledJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, scheduledJobToResource(item))
}

func (a *App) handleDeleteScheduledJob(c *gin.Context) {
	if a.scheduler == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "scheduler service unavailable")
		return
	}
	if err := a.scheduler.Delete(c.Request.Context(), c.Param("id")); err != nil {
		writeScheduledJobError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func firstNonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func firstNonZero(value, fallback int64) int64 {
	if value != 0 {
		return value
	}
	return fallback
}
