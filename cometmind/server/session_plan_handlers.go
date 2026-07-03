package server

import (
	"net/http"

	"github.com/cometline/cometmind/internal/planning"
	"github.com/gin-gonic/gin"
)

type sessionPlanStepResource struct {
	ID            string `json:"id"`
	SessionID     string `json:"session_id"`
	StepIndex     int64  `json:"step_index"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	BlockerReason string `json:"blocker_reason"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type sessionPlanResponse struct {
	SessionID string                    `json:"session_id"`
	Steps     []sessionPlanStepResource `json:"steps"`
	// Dismissed is true once the user (or an auto-hide timer) has dismissed
	// this plan, e.g. after all steps completed. Callers should treat a
	// dismissed plan as having no steps to display, even though the
	// underlying rows are retained for history. All steps in a plan share
	// the same dismissed state (dismissal applies to the whole session plan).
	Dismissed bool `json:"dismissed"`
}

func sessionPlanStepToResource(step planning.Step) sessionPlanStepResource {
	return sessionPlanStepResource{
		ID:            step.ID,
		SessionID:     step.SessionID,
		StepIndex:     step.StepIndex,
		Description:   step.Description,
		Status:        step.Status,
		BlockerReason: step.BlockerReason,
		CreatedAt:     step.CreatedAt,
		UpdatedAt:     step.UpdatedAt,
	}
}

func (a *App) handleGetSessionPlan(c *gin.Context) {
	if a.planning == nil {
		writeError(c, http.StatusServiceUnavailable, "planning_unavailable", "planning service unavailable")
		return
	}
	sessionID := c.Param("id")
	steps, err := a.planning.GetPlan(c.Request.Context(), sessionID)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "planning_error", err.Error())
		return
	}
	out := make([]sessionPlanStepResource, 0, len(steps))
	dismissed := len(steps) > 0
	for _, step := range steps {
		out = append(out, sessionPlanStepToResource(step))
		if step.DismissedAt == nil {
			dismissed = false
		}
	}
	c.JSON(http.StatusOK, sessionPlanResponse{SessionID: sessionID, Steps: out, Dismissed: dismissed})
}

func (a *App) handleDismissSessionPlan(c *gin.Context) {
	if a.planning == nil {
		writeError(c, http.StatusServiceUnavailable, "planning_unavailable", "planning service unavailable")
		return
	}
	sessionID := c.Param("id")
	if err := a.planning.DismissPlan(c.Request.Context(), sessionID); err != nil {
		writeError(c, http.StatusInternalServerError, "planning_error", err.Error())
		return
	}
	c.Status(http.StatusNoContent)
}
