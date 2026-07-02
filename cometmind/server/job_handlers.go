package server

import (
	"errors"
	"net/http"

	"github.com/cometline/cometmind/internal/jobs"
	"github.com/gin-gonic/gin"
)

type jobResource struct {
	ID                string `json:"id"`
	Description       string `json:"description"`
	DefinitionOfDone  string `json:"definition_of_done"`
	Progress          string `json:"progress"`
	Status            string `json:"status"`
	WorkspacePath     string `json:"workspace_path,omitempty"`
	AssignedSessionID string `json:"assigned_session_id,omitempty"`
	LeaseExpiresAt    *int64 `json:"lease_expires_at,omitempty"`
	CreatedBy         string `json:"created_by"`
	SourceSessionID   string `json:"source_session_id,omitempty"`
	SourcePlatform    string `json:"source_platform,omitempty"`
	SourceChannelID   string `json:"source_channel_id,omitempty"`
	ArchivedAt        *int64 `json:"archived_at,omitempty"`
	FailureCount      int64  `json:"failure_count"`
	NextRetryAt       *int64 `json:"next_retry_at,omitempty"`
	LastFailureReason string `json:"last_failure_reason,omitempty"`
	DeletedAt         *int64 `json:"deleted_at,omitempty"`
	CreatedAt         int64  `json:"created_at"`
	UpdatedAt         int64  `json:"updated_at"`
}

type jobEventResource struct {
	ID             string `json:"id"`
	JobID          string `json:"job_id"`
	Action         string `json:"action"`
	Detail         string `json:"detail"`
	ActorSessionID string `json:"actor_session_id,omitempty"`
	CreatedAt      int64  `json:"created_at"`
}

type createJobRequest struct {
	Description      string `json:"description"`
	DefinitionOfDone string `json:"definition_of_done"`
	WorkspacePath    string `json:"workspace_path"`
	CreatedBy        string `json:"created_by"`
	SourceSessionID  string `json:"source_session_id"`
	SourcePlatform   string `json:"source_platform"`
	SourceChannelID  string `json:"source_channel_id"`
}

type updateJobRequest struct {
	Description      string `json:"description"`
	DefinitionOfDone string `json:"definition_of_done"`
	WorkspacePath    string `json:"workspace_path"`
}

type jobSessionRequest struct {
	SessionID string `json:"session_id"`
}

type jobReleaseRequest struct {
	SessionID string `json:"session_id"`
	Reason    string `json:"reason"`
}

type jobCompleteRequest struct {
	SessionID string `json:"session_id"`
	Progress  string `json:"progress"`
}

type jobSettingsRequest struct {
	Notifications            *jobNotificationSettingsRequest `json:"notifications"`
	LeaseMinutes             *int                            `json:"lease_minutes"`
	DeletedPurgeDays         *int                            `json:"deleted_purge_days"`
	ArchivedPurgeDays        *int                            `json:"archived_purge_days"`
	StaleReviewMinutes       *int                            `json:"stale_review_minutes"`
	MaxConsecutiveFailures   *int                            `json:"max_consecutive_failures"`
	RetryCooldownMinutes     *int                            `json:"retry_cooldown_minutes"`
	MaxRetryCooldownMinutes  *int                            `json:"max_retry_cooldown_minutes"`
	ReconcileIntervalSeconds *int                            `json:"reconcile_interval_seconds"`
}

type jobNotificationSettingsRequest struct {
	Enabled     *bool `json:"enabled"`
	OnClaimed   *bool `json:"on_claimed"`
	OnCompleted *bool `json:"on_completed"`
	OnReleased  *bool `json:"on_released"`
	OnBlocked   *bool `json:"on_blocked"`
}

func jobToResource(j jobs.Job) jobResource {
	return jobResource{
		ID:                j.ID,
		Description:       j.Description,
		DefinitionOfDone:  j.DefinitionOfDone,
		Progress:          j.Progress,
		Status:            j.Status,
		WorkspacePath:     j.WorkspacePath,
		AssignedSessionID: j.AssignedSessionID,
		LeaseExpiresAt:    j.LeaseExpiresAt,
		CreatedBy:         j.CreatedBy,
		SourceSessionID:   j.SourceSessionID,
		SourcePlatform:    j.SourcePlatform,
		SourceChannelID:   j.SourceChannelID,
		ArchivedAt:        j.ArchivedAt,
		FailureCount:      j.FailureCount,
		NextRetryAt:       j.NextRetryAt,
		LastFailureReason: j.LastFailureReason,
		DeletedAt:         j.DeletedAt,
		CreatedAt:         j.CreatedAt,
		UpdatedAt:         j.UpdatedAt,
	}
}

func jobEventToResource(e jobs.JobEvent) jobEventResource {
	return jobEventResource{
		ID:             e.ID,
		JobID:          e.JobID,
		Action:         e.Action,
		Detail:         e.Detail,
		ActorSessionID: e.ActorSessionID,
		CreatedAt:      e.CreatedAt,
	}
}

func settingsToResponse(s jobs.Settings) gin.H {
	return gin.H{
		"notifications": gin.H{
			"enabled":      s.Notifications.Enabled,
			"on_claimed":   s.Notifications.OnClaimed,
			"on_completed": s.Notifications.OnCompleted,
			"on_released":  s.Notifications.OnReleased,
			"on_blocked":   s.Notifications.OnBlocked,
		},
		"lease_minutes":              s.LeaseMinutes,
		"deleted_purge_days":         s.DeletedPurgeDays,
		"archived_purge_days":        s.ArchivedPurgeDays,
		"stale_review_minutes":       s.StaleReviewMinutes,
		"max_consecutive_failures":   s.MaxConsecutiveFailures,
		"retry_cooldown_minutes":     s.RetryCooldownMinutes,
		"max_retry_cooldown_minutes": s.MaxRetryCooldownMinutes,
		"reconcile_interval_seconds": s.ReconcileIntervalS,
	}
}

func writeJobError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, jobs.ErrNotFound):
		writeError(c, http.StatusNotFound, "job_not_found", err.Error())
	case errors.Is(err, jobs.ErrConflict), errors.Is(err, jobs.ErrAlreadyClaimed), errors.Is(err, jobs.ErrNotAssigned):
		writeError(c, http.StatusConflict, "job_conflict", err.Error())
	case errors.Is(err, jobs.ErrNotEditable):
		writeError(c, http.StatusConflict, "job_not_editable", err.Error())
	default:
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
	}
	return true
}

func (a *App) handleListJobs(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	filter := jobs.ListFilter{
		Status:          c.Query("status"),
		ReadyOnly:       c.Query("ready_only") == "true",
		IncludeDeleted:  c.Query("include_deleted") == "true",
		IncludeArchived: c.Query("include_archived") == "true",
	}
	items, err := a.jobs.List(c.Request.Context(), filter)
	if err != nil {
		writeJobError(c, err)
		return
	}
	out := make([]jobResource, 0, len(items))
	for _, item := range items {
		out = append(out, jobToResource(item))
	}
	c.JSON(http.StatusOK, gin.H{"jobs": out})
}

func (a *App) handleArchiveJob(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	job, err := a.jobs.Archive(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, jobToResource(job))
}

func (a *App) handleUnarchiveJob(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	job, err := a.jobs.Unarchive(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, jobToResource(job))
}

func (a *App) handleUnblockJob(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	job, err := a.jobs.Unblock(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, jobToResource(job))
}

func (a *App) handleCreateJob(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	var req createJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	job, err := a.jobs.Create(c.Request.Context(), jobs.CreateInput{
		Description:      req.Description,
		DefinitionOfDone: req.DefinitionOfDone,
		WorkspacePath:    req.WorkspacePath,
		CreatedBy:        req.CreatedBy,
		SourceSessionID:  req.SourceSessionID,
		SourcePlatform:   req.SourcePlatform,
		SourceChannelID:  req.SourceChannelID,
	})
	if err != nil {
		if err.Error() == "description is required" {
			writeError(c, http.StatusBadRequest, "bad_request", err.Error())
			return
		}
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusCreated, jobToResource(job))
}

func (a *App) handleGetJob(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	job, err := a.jobs.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, jobToResource(job))
}

func (a *App) handleUpdateJob(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	var req updateJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	job, err := a.jobs.UpdateTodo(c.Request.Context(), c.Param("id"), jobs.UpdateTodoInput{
		Description:      req.Description,
		DefinitionOfDone: req.DefinitionOfDone,
		WorkspacePath:    req.WorkspacePath,
	}, "")
	if err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, jobToResource(job))
}

func (a *App) handleDeleteJob(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	if err := a.jobs.SoftDelete(c.Request.Context(), c.Param("id")); err != nil {
		writeJobError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *App) handleListJobEvents(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	if _, err := a.jobs.Get(c.Request.Context(), c.Param("id")); err != nil {
		writeJobError(c, err)
		return
	}
	events, err := a.jobs.ListEvents(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeJobError(c, err)
		return
	}
	out := make([]jobEventResource, 0, len(events))
	for _, e := range events {
		out = append(out, jobEventToResource(e))
	}
	c.JSON(http.StatusOK, gin.H{"events": out})
}

func (a *App) handleClaimJob(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	var req jobSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
		writeError(c, http.StatusBadRequest, "bad_request", "session_id is required")
		return
	}
	job, err := a.jobs.Claim(c.Request.Context(), c.Param("id"), req.SessionID)
	if err != nil {
		writeJobError(c, err)
		return
	}
	_ = a.jobs.Heartbeat(c.Request.Context(), job.ID, req.SessionID)
	c.JSON(http.StatusOK, jobToResource(job))
}

func (a *App) handleReleaseJob(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	var req jobReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
		writeError(c, http.StatusBadRequest, "bad_request", "session_id is required")
		return
	}
	job, err := a.jobs.Release(c.Request.Context(), c.Param("id"), req.SessionID, req.Reason)
	if err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, jobToResource(job))
}

func (a *App) handleCompleteJob(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	var req jobCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
		writeError(c, http.StatusBadRequest, "bad_request", "session_id is required")
		return
	}
	job, err := a.jobs.Complete(c.Request.Context(), c.Param("id"), req.SessionID, req.Progress)
	if err != nil {
		writeJobError(c, err)
		return
	}
	c.JSON(http.StatusOK, jobToResource(job))
}

func (a *App) handleHeartbeatJob(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	var req jobSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
		writeError(c, http.StatusBadRequest, "bad_request", "session_id is required")
		return
	}
	if err := a.jobs.Heartbeat(c.Request.Context(), c.Param("id"), req.SessionID); err != nil {
		writeJobError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *App) handleGetJobSettings(c *gin.Context) {
	if a.jobs == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	c.JSON(http.StatusOK, settingsToResponse(a.jobs.Settings()))
}

func (a *App) handlePutJobSettings(c *gin.Context) {
	if a.jobs == nil || a.setJobSettings == nil {
		writeError(c, http.StatusInternalServerError, "internal_error", "jobs service unavailable")
		return
	}
	var req jobSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	current := a.jobs.Settings()
	if req.Notifications != nil {
		if req.Notifications.Enabled != nil {
			current.Notifications.Enabled = *req.Notifications.Enabled
		}
		if req.Notifications.OnClaimed != nil {
			current.Notifications.OnClaimed = *req.Notifications.OnClaimed
		}
		if req.Notifications.OnCompleted != nil {
			current.Notifications.OnCompleted = *req.Notifications.OnCompleted
		}
		if req.Notifications.OnReleased != nil {
			current.Notifications.OnReleased = *req.Notifications.OnReleased
		}
		if req.Notifications.OnBlocked != nil {
			current.Notifications.OnBlocked = *req.Notifications.OnBlocked
		}
	}
	if req.LeaseMinutes != nil {
		current.LeaseMinutes = *req.LeaseMinutes
	}
	if req.DeletedPurgeDays != nil {
		current.DeletedPurgeDays = *req.DeletedPurgeDays
	}
	if req.ArchivedPurgeDays != nil {
		current.ArchivedPurgeDays = *req.ArchivedPurgeDays
	}
	if req.StaleReviewMinutes != nil {
		current.StaleReviewMinutes = *req.StaleReviewMinutes
	}
	if req.MaxConsecutiveFailures != nil {
		current.MaxConsecutiveFailures = *req.MaxConsecutiveFailures
	}
	if req.RetryCooldownMinutes != nil {
		current.RetryCooldownMinutes = *req.RetryCooldownMinutes
	}
	if req.MaxRetryCooldownMinutes != nil {
		current.MaxRetryCooldownMinutes = *req.MaxRetryCooldownMinutes
	}
	if req.ReconcileIntervalSeconds != nil {
		current.ReconcileIntervalS = *req.ReconcileIntervalSeconds
	}
	a.setJobSettings(current)
	c.JSON(http.StatusOK, settingsToResponse(current))
}
