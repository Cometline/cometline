package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/apigen"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/logging"
	mcppkg "github.com/cometline/cometmind/internal/mcp"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/retention"
	"github.com/cometline/cometmind/internal/session"
	skillpkg "github.com/cometline/cometmind/internal/skills"
	"github.com/cometline/cometmind/internal/subagent"
	"github.com/gin-gonic/gin"
)

type Runner interface {
	Run(context.Context, session.AgentTurn, chan<- event.Event) error
}

type RunnerFactory func(sess session.Session, workspacePath string) (Runner, error)

type RetentionResult = retention.Result

type RetentionRunner func(context.Context) (RetentionResult, error)

type Deps struct {
	Config         *config.Config
	Sessions       *session.Service
	Memory         *memory.Service
	Jobs           *jobs.Service
	RunRetention   RetentionRunner
	SetJobSettings func(jobs.Settings)
	NewRunner      RunnerFactory
	Runs           *RunManager
	ACPMgr         *acp.SessionManager
	MCPMgr         *mcppkg.Manager
	SubagentOrch   *subagent.Orchestrator
	ReloadRuntime  func(context.Context) error
	RequestStop    func()
}

type App struct {
	config         *config.Config
	sessions       *session.Service
	memory         *memory.Service
	jobs           *jobs.Service
	runRetention   RetentionRunner
	setJobSettings func(jobs.Settings)
	newRunner      RunnerFactory
	runs           *RunManager
	acpMgr         *acp.SessionManager
	mcpMgr         *mcppkg.Manager
	subagentOrch   *subagent.Orchestrator
	reloadRuntime  func(context.Context) error
	requestStop    func()
}

func New(deps Deps) (*gin.Engine, error) {
	if deps.Config == nil {
		return nil, fmt.Errorf("server config is required")
	}
	if deps.Sessions == nil {
		return nil, fmt.Errorf("session service is required")
	}
	if deps.NewRunner == nil {
		return nil, fmt.Errorf("runner factory is required")
	}
	if deps.Runs == nil {
		deps.Runs = NewRunManager()
	}

	app := &App{
		config:         deps.Config,
		sessions:       deps.Sessions,
		memory:         deps.Memory,
		jobs:           deps.Jobs,
		runRetention:   deps.RunRetention,
		setJobSettings: deps.SetJobSettings,
		newRunner:      deps.NewRunner,
		runs:           deps.Runs,
		acpMgr:         deps.ACPMgr,
		mcpMgr:         deps.MCPMgr,
		subagentOrch:   deps.SubagentOrch,
		reloadRuntime:  deps.ReloadRuntime,
		requestStop:    deps.RequestStop,
	}

	r := gin.New()
	r.Use(logging.Gin())
	r.Use(localCORS())
	r.Use(gin.Recovery())

	api := r.Group("/api/v1")

	// Health
	api.GET("/health", app.handleHealth)
	api.GET("/admin/processes", app.handleListProcesses)
	api.POST("/admin/reload-runs", app.handleAdminReload)
	api.POST("/admin/restart-runs", app.handleAdminRestart)

	// Models
	api.GET("/models", app.handleListModels)

	// Workspaces
	api.GET("/workspaces", app.handleListWorkspaces)
	api.POST("/workspaces", app.handleCreateWorkspace)
	api.DELETE("/workspaces", app.handleDeleteWorkspace)
	api.POST("/workspaces/prune-runs", app.handlePruneWorkspaces)
	api.GET("/workspaces/files", app.handleListWorkspaceFiles)
	api.GET("/workspaces/files/content", app.handleReadWorkspaceFileContent)
	api.PUT("/workspaces/files/content", app.handleWriteWorkspaceFileContent)

	// Sessions
	api.POST("/sessions", app.handleCreateSession)
	api.GET("/sessions", app.handleListSessions)
	api.GET("/sessions/:id", app.handleGetSession)
	api.PATCH("/sessions/:id", app.handlePatchSession)
	api.PATCH("/sessions/:id/workspace", app.handleChangeSessionWorkspace)
	api.POST("/sessions/:id/forks", app.handleForkSession)
	api.DELETE("/sessions/:id", app.handleDeleteSession)
	api.GET("/sessions/:id/messages", app.handleGetMessages)
	api.POST("/sessions/:id/messages", app.handlePostMessage)
	api.DELETE("/sessions/:id/messages", app.handleClearSession)
	api.GET("/sessions/:id/children", app.handleListChildSessions)
	api.DELETE("/sessions/:id/runs/current", app.handleAbortSession)

	// Skills
	api.GET("/skills", app.handleListSkills)
	api.POST("/skills/sync-runs", app.handleSyncSkills)
	api.GET("/skills/:name/archive", app.handleExportSkill)
	api.DELETE("/skills/:name", app.handleDeleteSkill)
	api.GET("/skills/:name", app.handleGetSkill)

	// MCP
	api.GET("/mcp/servers", app.handleListMCPServers)
	api.GET("/mcp/tools", app.handleListMCPTools)
	api.POST("/mcp/servers/:id/connection-tests", app.handleTestMCPServer)
	api.POST("/mcp/servers/:id/reconnection-runs", app.handleReconnectMCPServer)
	api.POST("/mcp/servers/:id/oauth-flows", app.handleStartMCPOAuth)

	// Memories
	api.GET("/memories", app.handleListMemories)
	api.POST("/memories", app.handleCreateMemory)
	api.PATCH("/memories/:id", app.handlePatchMemory)
	api.DELETE("/memories/:id", app.handleDeleteMemory)
	api.POST("/memories/searches", app.handleSearchMemories)

	// Memory settings & maintenance
	api.GET("/memories/settings", app.handleGetMemorySettings)
	api.PUT("/memories/settings", app.handlePutMemorySettings)
	api.POST("/memories/purge-runs", app.handlePurgeMemory)
	api.POST("/memories/compaction-runs", app.handleCompactMemory)
	api.GET("/memories/compaction-preview", app.handleCompactPreview)

	// Storage retention
	api.POST("/storage/retention/runs", app.handleRunStorageRetention)

	// Jobs
	api.GET("/jobs", app.handleListJobs)
	api.POST("/jobs", app.handleCreateJob)
	api.GET("/jobs/settings", app.handleGetJobSettings)
	api.PUT("/jobs/settings", app.handlePutJobSettings)
	api.GET("/jobs/:id", app.handleGetJob)
	api.PATCH("/jobs/:id", app.handleUpdateJob)
	api.DELETE("/jobs/:id", app.handleDeleteJob)
	api.GET("/jobs/:id/events", app.handleListJobEvents)
	api.PUT("/jobs/:id/lease", app.handleClaimJob)
	api.DELETE("/jobs/:id/lease", app.handleReleaseJob)
	api.PUT("/jobs/:id/completion", app.handleCompleteJob)
	api.PATCH("/jobs/:id/lease", app.handleHeartbeatJob)

	return r, nil
}

func localCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if isAllowedLocalOrigin(origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func isAllowedLocalOrigin(origin string) bool {
	if origin == "" || origin == "null" || origin == "file://" {
		return true
	}
	return strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "http://localhost:") ||
		strings.HasPrefix(origin, "app://")
}

type healthResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type tokenUsageResource struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CacheRead    int `json:"cache_read"`
	CacheWrite   int `json:"cache_write"`
}

type gatewayResource struct {
	Platform  string `json:"platform"`
	ChannelID string `json:"channel_id"`
	ThreadID  string `json:"thread_id,omitempty"`
}

type sessionResource struct {
	ID               string             `json:"id"`
	WorkspaceID      string             `json:"workspace_id"`
	WorkspacePath    string             `json:"workspace_path"`
	Title            string             `json:"title"`
	ModelID          string             `json:"model_id"`
	ProviderID       string             `json:"provider_id"`
	Status           string             `json:"status"`
	TokenUsage       tokenUsageResource `json:"token_usage"`
	ParentSessionID  string             `json:"parent_session_id,omitempty"`
	Purpose          string             `json:"purpose,omitempty"`
	DelegationStatus string             `json:"delegation_status,omitempty"`
	OutputSummary    string             `json:"output_summary,omitempty"`
	ACPSessionID     string             `json:"acp_session_id,omitempty"`
	PendingQuestion  string             `json:"pending_question,omitempty"`
	SubagentKind     string             `json:"subagent_kind,omitempty"`
	Gateway          *gatewayResource   `json:"gateway,omitempty"`
	Pinned           bool               `json:"pinned"`
	CreatedAt        int64              `json:"created_at"`
	UpdatedAt        int64              `json:"updated_at"`
}

type listSessionsResponse struct {
	Sessions []sessionResource `json:"sessions"`
}

type transcriptItem struct {
	Type       string              `json:"type"`
	Text       string              `json:"text,omitempty"`
	Images     []messageImageInput `json:"images,omitempty"`
	ToolName   string              `json:"tool_name,omitempty"`
	ToolInput  any                 `json:"tool_input,omitempty"`
	ToolOutput string              `json:"tool_output,omitempty"`
	ToolError  bool                `json:"tool_error,omitempty"`
	Memories   []transcriptMemory  `json:"memories,omitempty"`
}

type transcriptMemory struct {
	ID              string  `json:"id"`
	Content         string  `json:"content"`
	Kind            string  `json:"kind"`
	Similarity      float64 `json:"similarity"`
	EffectiveWeight float64 `json:"effective_weight"`
}

type transcriptResponse struct {
	SessionID string           `json:"session_id"`
	Items     []transcriptItem `json:"items"`
}

type statusResponse struct {
	Status string `json:"status"`
}

type skillResource struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Source      string `json:"source"`
	Internal    bool   `json:"internal"`
	IsSymlink   bool   `json:"is_symlink"`
	CanDelete   bool   `json:"can_delete"`
	CanExport   bool   `json:"can_export"`
}

type listSkillsResponse struct {
	Skills []skillResource `json:"skills"`
	Errors []string        `json:"errors,omitempty"`
}

type skillDetailResponse struct {
	Skill   skillResource `json:"skill"`
	Content string        `json:"content"`
}

type syncSkillsResponse struct {
	Created []string `json:"created"`
	Skipped []string `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

type adminProcessStatus struct {
	Mode         string `json:"mode"`
	Present      bool   `json:"present"`
	Running      bool   `json:"running"`
	Stale        bool   `json:"stale"`
	Pid          int    `json:"pid,omitempty"`
	StartedAt    string `json:"started_at,omitempty"`
	DataDir      string `json:"data_dir,omitempty"`
	SettingsPath string `json:"settings_path,omitempty"`
}

type listAdminProcessesResponse struct {
	Processes []adminProcessStatus `json:"processes"`
}

type adminProcessControlRequest struct {
	Modes []string `json:"modes"`
}

type adminProcessControlResponse struct {
	Status            string               `json:"status"`
	ReloadedProcesses []string             `json:"reloaded_processes,omitempty"`
	StoppedProcesses  []string             `json:"stopped_processes,omitempty"`
	Processes         []adminProcessStatus `json:"processes,omitempty"`
}

func (a *App) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}

func (a *App) handleListModels(c *gin.Context) {
	models, err := config.ListConfiguredModels()
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	items := make([]apigen.ModelEntry, 0, len(models))
	for _, m := range models {
		items = append(items, apigen.ModelEntry{
			ProviderId: m.ProviderID,
			ModelId:    m.ModelID,
			Name:       m.Name,
		})
	}
	c.JSON(http.StatusOK, apigen.ModelListResponse{Models: items})
}

func (a *App) handleListSkills(c *gin.Context) {
	reg := a.skillsForRequest(c)
	items := make([]skillResource, 0, len(reg.Skills))
	for _, skill := range reg.Skills {
		items = append(items, skillResourceFromModel(skill))
	}
	c.JSON(http.StatusOK, listSkillsResponse{Skills: items, Errors: reg.Errors})
}

func (a *App) handleGetSkill(c *gin.Context) {
	reg := a.skillsForRequest(c)
	skill, content, err := reg.SkillMarkdown(c.Param("name"))
	if err != nil {
		writeError(c, http.StatusNotFound, "skill_not_found", err.Error())
		return
	}
	c.JSON(http.StatusOK, skillDetailResponse{Skill: skillResourceFromModel(skill), Content: content})
}

func (a *App) handleSyncSkills(c *gin.Context) {
	reg := a.skillsForRequest(c)
	created, skipped, err := reg.SyncMirror(filepath.Join("~", ".cometmind", "skills"))
	if err != nil {
		writeError(c, http.StatusInternalServerError, "sync_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, syncSkillsResponse{Created: created, Skipped: skipped, Errors: reg.Errors})
}

func (a *App) handleExportSkill(c *gin.Context) {
	reg := a.skillsForRequest(c)
	name := strings.TrimSpace(c.Param("name"))
	skill, ok := reg.Find(name)
	if !ok {
		writeError(c, http.StatusNotFound, "skill_not_found", "unknown skill: "+name)
		return
	}
	caps, err := skillpkg.SkillCapabilities(skill)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if !caps.CanExport {
		writeError(c, http.StatusForbidden, "export_forbidden", "skill cannot be exported")
		return
	}
	data, err := skillpkg.ExportSkill(skill)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name+".zip"))
	c.Data(http.StatusOK, "application/zip", data)
}

func (a *App) handleDeleteSkill(c *gin.Context) {
	reg := a.skillsForRequest(c)
	name := strings.TrimSpace(c.Param("name"))
	skill, ok := reg.Find(name)
	if !ok {
		writeError(c, http.StatusNotFound, "skill_not_found", "unknown skill: "+name)
		return
	}
	caps, err := skillpkg.SkillCapabilities(skill)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	if !caps.CanDelete {
		writeError(c, http.StatusForbidden, "delete_forbidden", "external or symlink skills cannot be deleted")
		return
	}
	if err := skillpkg.DeleteManagedSkill(skill); err != nil {
		writeError(c, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	c.JSON(http.StatusOK, statusResponse{Status: "deleted"})
}

func (a *App) skillsForRequest(c *gin.Context) skillpkg.Registry {
	workspacePath := strings.TrimSpace(c.Query("workspace_path"))
	if workspacePath == "" && strings.TrimSpace(c.Query("workspace_id")) != "" {
		if path, err := a.sessions.WorkspacePath(c.Request.Context(), strings.TrimSpace(c.Query("workspace_id"))); err == nil {
			workspacePath = path
		}
	}
	return skillpkg.Discover(workspacePath, a.config.SkillSettings())
}

func skillResourceFromModel(skill skillpkg.Skill) skillResource {
	caps, _ := skillpkg.SkillCapabilities(skill)
	return skillResource{
		Name:        skill.Name,
		Description: skill.Description,
		Path:        skill.Path,
		Source:      skill.Source,
		Internal:    skill.Internal,
		IsSymlink:   caps.IsSymlink,
		CanDelete:   caps.CanDelete,
		CanExport:   caps.CanExport,
	}
}

func (a *App) loadSessionWithWorkspace(c *gin.Context, sessionID string) (session.Session, string, bool) {
	sess, err := a.sessions.GetSession(c.Request.Context(), sessionID)
	if errors.Is(err, session.ErrSessionNotFound) {
		writeError(c, http.StatusNotFound, "session_not_found", "session was not found")
		return session.Session{}, "", false
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return session.Session{}, "", false
	}

	wsPath, err := a.sessions.WorkspacePath(c.Request.Context(), sess.WorkspaceID)
	if errors.Is(err, session.ErrWorkspaceNotFound) {
		writeError(c, http.StatusNotFound, "workspace_not_found", "workspace was not found")
		return session.Session{}, "", false
	}
	if err != nil {
		writeError(c, http.StatusInternalServerError, "internal_error", err.Error())
		return session.Session{}, "", false
	}

	return sess, wsPath, true
}

func sessionResourceFromModel(sess session.Session, workspacePath string) (sessionResource, error) {
	wire, err := session.APISession(sess, workspacePath)
	if err != nil {
		return sessionResource{}, err
	}
	return sessionResourceFromAPISession(wire), nil
}

func sessionResourceFromAPISession(w apigen.Session) sessionResource {
	res := sessionResource{
		ID:            w.Id,
		WorkspaceID:   w.WorkspaceId,
		WorkspacePath: w.WorkspacePath,
		Title:         w.Title,
		ModelID:       w.ModelId,
		ProviderID:    w.ProviderId,
		Status:        string(w.Status),
		TokenUsage: tokenUsageResource{
			InputTokens:  w.TokenUsage.InputTokens,
			OutputTokens: w.TokenUsage.OutputTokens,
			CacheRead:    w.TokenUsage.CacheRead,
			CacheWrite:   w.TokenUsage.CacheWrite,
		},
		Pinned:    w.Pinned,
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	}
	if w.ParentSessionId != nil {
		res.ParentSessionID = *w.ParentSessionId
	}
	if w.Purpose != nil {
		res.Purpose = *w.Purpose
	}
	if w.DelegationStatus != nil {
		res.DelegationStatus = string(*w.DelegationStatus)
	}
	if w.OutputSummary != nil {
		res.OutputSummary = *w.OutputSummary
	}
	if w.AcpSessionId != nil {
		res.ACPSessionID = *w.AcpSessionId
	}
	if w.PendingQuestion != nil {
		res.PendingQuestion = *w.PendingQuestion
	}
	if w.SubagentKind != nil {
		res.SubagentKind = string(*w.SubagentKind)
	}
	if w.Gateway != nil {
		gw := &gatewayResource{}
		if w.Gateway.Platform != nil {
			gw.Platform = string(*w.Gateway.Platform)
		}
		if w.Gateway.ChannelId != nil {
			gw.ChannelID = *w.Gateway.ChannelId
		}
		if w.Gateway.ThreadId != nil {
			gw.ThreadID = *w.Gateway.ThreadId
		}
		res.Gateway = gw
	}
	return res
}

func transcriptItemFromModel(item session.TranscriptEntry) transcriptItem {
	switch item.Kind {
	case session.TranscriptKindUser:
		out := transcriptItem{Type: "user", Text: item.Text}
		for _, block := range item.Images {
			out.Images = append(out.Images, messageImageInput{MediaType: block.MediaType, Data: block.Data})
		}
		return out
	case session.TranscriptKindReasoning:
		return transcriptItem{Type: "reasoning", Text: item.Text}
	case session.TranscriptKindAssistant:
		return transcriptItem{Type: "assistant", Text: item.Text}
	case session.TranscriptKindTool:
		return transcriptItem{
			Type:       "tool",
			ToolName:   item.ToolName,
			ToolInput:  parseOpaqueJSON(item.ToolInput),
			ToolOutput: item.ToolOutput,
			ToolError:  item.ToolIsError,
		}
	case session.TranscriptKindSystem:
		return transcriptItem{Type: "system", Text: item.Text}
	case session.TranscriptKindMemory:
		out := transcriptItem{Type: "memory"}
		for _, mem := range item.Memories {
			out.Memories = append(out.Memories, transcriptMemory{
				ID:              mem.ID,
				Content:         mem.Content,
				Kind:            mem.Kind,
				Similarity:      mem.Similarity,
				EffectiveWeight: mem.EffectiveWeight,
			})
		}
		return out
	default:
		return transcriptItem{Type: string(item.Kind), Text: item.Text}
	}
}

func parseOpaqueJSON(raw string) any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]any{}
	}
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err == nil {
		return v
	}
	return raw
}

func writeSSE(w http.ResponseWriter, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", raw)
	return err
}

func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, errorResponse{
		Error: apiError{
			Code:    code,
			Message: message,
		},
	})
}
