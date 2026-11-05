package server

import (
	"net/http"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/apigen"
	"github.com/Cometline/cometline/cometmind/internal/hooks"
	"github.com/gin-gonic/gin"
)

func (a *App) handleListHooksCatalog(c *gin.Context) {
	workspacePath := strings.TrimSpace(c.Query("workspace_path"))
	cfg := a.config.HooksSettings()
	eng := hooks.NewEngine(cfg, workspacePath)
	entries := eng.Catalog()
	out := make([]apigen.HookCatalogEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, apigen.HookCatalogEntry{
			Event:   e.Event,
			Matcher: e.Matcher,
			Command: e.Command,
			Source:  e.Source,
		})
	}
	c.JSON(http.StatusOK, apigen.ListHooksCatalogResponse{
		Enabled:             eng.Enabled(),
		ProjectHooksAllowed: hooks.WorkspaceTrusted(workspacePath, cfg.TrustedWorkspacePaths),
		Entries:             out,
	})
}
