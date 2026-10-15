package server

import (
	"errors"
	"net/http"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/agent"
	"github.com/Cometline/cometline/cometmind/internal/provider"
	"github.com/Cometline/cometline/cometmind/internal/runstate"
	"github.com/gin-gonic/gin"
)

type contextCompactionResponse struct {
	Status        string `json:"status"`
	Reason        string `json:"reason,omitempty"`
	Estimated     int    `json:"estimated,omitempty"`
	Available     int    `json:"available,omitempty"`
	ContextWindow int    `json:"context_window,omitempty"`
	Compacted     bool   `json:"compacted,omitempty"`
}

func (a *App) handleCompactSessionContext(c *gin.Context) {
	sess, _, ok := a.loadSessionWithWorkspace(c, c.Param("id"))
	if !ok {
		return
	}
	runCtx, finish, err := a.runs.StartCompaction(a.runContext, sess.ID)
	if err != nil {
		if errors.Is(err, runstate.ErrAlreadyRunning) {
			writeError(c, http.StatusConflict, "session_running", "session is running")
			return
		}
		writeError(c, http.StatusInternalServerError, "run_state_failed", err.Error())
		return
	}
	defer finish()

	compactor := &agent.ContextCompactor{Sessions: a.sessions, Config: a.config}
	outcome, err := compactor.ForceCompact(runCtx, sess, a.openCompactionProvider)
	if err != nil {
		if errors.Is(err, runCtx.Err()) {
			c.JSON(http.StatusOK, contextCompactionResponse{Status: "skipped", Reason: "cancelled"})
			return
		}
		writeError(c, http.StatusInternalServerError, "context_compaction_failed", err.Error())
		return
	}
	res := contextCompactionResponse{
		Status: string(outcome.Status),
		Reason: outcome.Reason,
	}
	if budget, budgetErr := compactor.EstimatePromptBudget(runCtx, sess.ID, "", nil, outcome.ProviderID, outcome.ModelID); budgetErr == nil {
		res.Estimated = budget.Estimated
		res.Available = budget.Available
		res.ContextWindow = budget.ContextWindow
		res.Compacted = outcome.Status == agent.CompactStatusCompacted
	}
	c.JSON(http.StatusOK, res)
}

func (a *App) openCompactionProvider(providerID, modelID string) (cometsdk.Provider, error) {
	return provider.NewForModel(a.config, providerID, modelID)
}
