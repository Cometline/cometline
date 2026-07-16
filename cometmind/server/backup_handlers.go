package server

import (
	"context"
	"net/http"

	"github.com/cometline/cometmind/internal/backup"
	"github.com/gin-gonic/gin"
)

type BackupResult = backup.Result

type BackupRunner func(context.Context) (BackupResult, error)

type runBackupResponse struct {
	Status      string `json:"status"`
	Path        string `json:"path"`
	FilesZipped int    `json:"files_zipped"`
	RemovedOld  int    `json:"removed_old"`
}

func (a *App) handleRunBackup(c *gin.Context) {
	if a.runBackup == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "backup unavailable"})
		return
	}
	result, err := a.runBackup(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, runBackupResponse{
		Status:      "ok",
		Path:        result.Path,
		FilesZipped: result.FilesZipped,
		RemovedOld:  result.RemovedOld,
	})
}
