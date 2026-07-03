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
	for _, step := range steps {
		out = append(out, sessionPlanStepToResource(step))
	}
	c.JSON(http.StatusOK, sessionPlanResponse{SessionID: sessionID, Steps: out})
}
