package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type runStorageRetentionResponse struct {
	Status             string `json:"status"`
	SessionsDeleted    int    `json:"sessions_deleted"`
	SubagentsDeleted   int    `json:"subagents_deleted"`
	MemoriesPurged     int    `json:"memories_purged"`
	MemoryEventsPurged int    `json:"memory_events_purged"`
	JobsPurged         int    `json:"jobs_purged"`
	Vacuumed           bool   `json:"vacuumed"`
}

func (a *App) handleRunStorageRetention(c *gin.Context) {
	if a.runRetention == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "storage retention unavailable"})
		return
	}
	result, err := a.runRetention(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, runStorageRetentionResponse{
		Status:             "ok",
		SessionsDeleted:    result.SessionsDeleted,
		SubagentsDeleted:   result.SubagentsDeleted,
		MemoriesPurged:     result.MemoriesPurged,
		MemoryEventsPurged: result.MemoryEventsPurged,
		JobsPurged:         result.JobsPurged,
		Vacuumed:           result.Vacuumed,
	})
}
