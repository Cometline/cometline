package server

import (
	"context"
	"io"
	"net/http"
	"slices"
	"syscall"
	"time"

	"github.com/cometline/cometmind/internal/processctl"
	"github.com/gin-gonic/gin"
)

func (a *App) handleListProcesses(c *gin.Context) {
	statuses, err := adminProcessStatuses(processctl.KnownModes())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, listAdminProcessesResponse{Processes: statuses})
}

func (a *App) handleAdminReload(c *gin.Context) {
	modes, err := decodeAdminModes(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	reloaded := make([]string, 0, len(modes))
	if slices.Contains(modes, processctl.ModeServe) {
		if a.reloadRuntime == nil {
			writeError(c, http.StatusServiceUnavailable, "reload_unavailable", "runtime reload is not available")
			return
		}
		reloadCtx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		if err := a.reloadRuntime(reloadCtx); err != nil {
			writeError(c, http.StatusInternalServerError, "reload_failed", err.Error())
			return
		}
		reloaded = append(reloaded, processctl.ModeServe)
	}
	for _, mode := range withoutMode(modes, processctl.ModeServe) {
		state, err := processctl.ReadState(mode)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "reload_failed", err.Error())
			return
		}
		if !state.Running {
			continue
		}
		if _, err := processctl.Signal(syscall.SIGHUP, mode); err != nil {
			writeError(c, http.StatusInternalServerError, "reload_failed", err.Error())
			return
		}
		reloaded = append(reloaded, mode)
	}
	statuses, err := adminProcessStatuses(modes)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	c.JSON(http.StatusOK, adminProcessControlResponse{Status: "ok", ReloadedProcesses: reloaded, Processes: statuses})
}

func (a *App) handleAdminRestart(c *gin.Context) {
	modes, err := decodeAdminModes(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	stopped := make([]string, 0, len(modes))
	for _, mode := range withoutMode(modes, processctl.ModeServe) {
		state, err := processctl.ReadState(mode)
		if err != nil {
			writeError(c, http.StatusInternalServerError, "restart_failed", err.Error())
			return
		}
		if !state.Running {
			continue
		}
		if _, err := processctl.Signal(syscall.SIGTERM, mode); err != nil {
			writeError(c, http.StatusInternalServerError, "restart_failed", err.Error())
			return
		}
		stopped = append(stopped, mode)
	}
	if slices.Contains(modes, processctl.ModeServe) {
		if a.requestStop == nil {
			writeError(c, http.StatusServiceUnavailable, "restart_unavailable", "process restart is not available")
			return
		}
		stopped = append(stopped, processctl.ModeServe)
		a.requestStop()
	}
	c.JSON(http.StatusAccepted, adminProcessControlResponse{Status: "accepted", StoppedProcesses: stopped})
}

func decodeAdminModes(c *gin.Context) ([]string, error) {
	var req adminProcessControlRequest
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		return nil, err
	}
	return processctl.TargetModes(req.Modes)
}

func adminProcessStatuses(modes []string) ([]adminProcessStatus, error) {
	statuses := make([]adminProcessStatus, 0, len(modes))
	for _, mode := range modes {
		state, err := processctl.ReadState(mode)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, adminProcessStatus{
			Mode:         mode,
			Present:      state.Present,
			Running:      state.Running,
			Stale:        state.Stale,
			Pid:          state.Metadata.PID,
			StartedAt:    state.Metadata.StartedAt,
			DataDir:      state.Metadata.DataDir,
			SettingsPath: state.Metadata.SettingsPath,
		})
	}
	return statuses, nil
}

func withoutMode(modes []string, excluded string) []string {
	out := make([]string, 0, len(modes))
	for _, mode := range modes {
		if mode != excluded {
			out = append(out, mode)
		}
	}
	return out
}
