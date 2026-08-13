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
	"github.com/cometline/cometmind/internal/inbox"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/logging"
	mcppkg "github.com/cometline/cometmind/internal/mcp"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/retention"
	"github.com/cometline/cometmind/internal/scheduler"
	"github.com/cometline/cometmind/internal/session"
	skillpkg "github.com/cometline/cometmind/internal/skills"
	"github.com/cometline/cometmind/internal/subagent"
	"github.com/gin-gonic/gin"
)

type Runner interface {
	Run(context.Context, session.AgentTurn, chan<- event.Event) error
}

type RunnerFactory func(sess session.Session, workspacePath string, mode session.AgentMode) (Runner, error)

type RetentionResult = retention.Result

type RetentionRunner func(context.Context) (RetentionResult, error)

type Deps struct {
	Config         *config.Config
	Sessions       *session.Service
	Memory         *memory.Service
	Events         *event.Hub
	Jobs           *jobs.Service
	Inbox          *inbox.Service
	Scheduler      *scheduler.Service
	RunRetention   RetentionRunner
	RunBackup      BackupRunner
	SetJobSettings func(jobs.Settings)
	NewRunner      RunnerFactory
	Runs           *RunManager
	// RunContext owns agent lifetimes independently from individual SSE
	// requests. A renderer disconnect must not cancel the model/tool loop.
	RunContext   context.Context
	ACPMgr       *acp.SessionManager
	MCPMgr       *mcppkg.Manager
	SubagentOrch *subagent.Orchestrator
}

type App struct {
	config         *config.Config
	sessions       *session.Service
	memory         *memory.Service
	events         *event.Hub
	jobs           *jobs.Service
	inbox          *inbox.Service
	scheduler      *scheduler.Service
	runRetention   RetentionRunner
	runBackup      BackupRunner
	setJobSettings func(jobs.Settings)
	newRunner      RunnerFactory
	runs           *RunManager
	runContext     context.Context
	acpMgr         *acp.SessionManager
	mcpMgr         *mcppkg.Manager
	subagentOrch   *subagent.Orchestrator
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
	runContext := deps.RunContext
	if runContext == nil {
		runContext = context.Background()
	}

	app := &App{
		config:         deps.Config,
		sessions:       deps.Sessions,
		memory:         deps.Memory,
		events:         deps.Events,
		jobs:           deps.Jobs,
		inbox:          deps.Inbox,
		scheduler:      deps.Scheduler,
		runRetention:   deps.RunRetention,
		runBackup:      deps.RunBackup,
		setJobSettings: deps.SetJobSettings,
		newRunner:      deps.NewRunner,
		runs:           deps.Runs,
		runContext:     runContext,
		acpMgr:         deps.ACPMgr,
		mcpMgr:         deps.MCPMgr,
		subagentOrch:   deps.SubagentOrch,
	}
	if deps.Memory != nil && deps.Events != nil {
		deps.Memory.SetCompactionCompletedNotifier(func(result memory.CompactionResult) {
			deps.Events.Publish(event.MemoryCompactionCompleted(result.Before, result.After, result.Trigger))
		})
	}

	r := gin.New()
	r.Use(logging.Gin())
	r.Use(localCORS())
	r.Use(gin.Recovery())

	api := r.Group("/api/v1")

	// Health
	api.GET("/health", app.handleHealth)

	// Models
	api.POST("/models/catalog-lookups", app.handleLookupModelCatalog)

	// Workspaces
	api.GET("/workspaces", app.handleListWorkspaces)
	api.POST("/workspaces", app.handleCreateWorkspace)
	api.DELETE("/workspaces", app.handleDeleteWorkspace)
	api.POST("/workspaces/prune-runs", app.handlePruneWorkspaces)
	api.GET("/workspaces/files", app.handleListWorkspaceFiles)
	api.GET("/workspaces/files/children", app.handleListWorkspaceFileChildren)
	api.GET("/workspaces/files/content", app.handleReadWorkspaceFileContent)
	api.PUT("/workspaces/files/content", app.handleWriteWorkspaceFileContent)
	api.GET("/workspaces/git/status", app.handleWorkspaceGitStatus)
	api.GET("/workspaces/git/diff", app.handleWorkspaceGitDiff)
	api.POST("/workspaces/git/stage", app.handleWorkspaceGitStage)
	api.POST("/workspaces/git/unstage", app.handleWorkspaceGitUnstage)
	api.POST("/workspaces/git/discard", app.handleWorkspaceGitDiscard)
	api.POST("/workspaces/git/commit", app.handleWorkspaceGitCommit)

	// Wiki files (global LLM wiki at ~/.cometmind/wiki/)
	api.GET("/wiki/files", app.handleListWikiFiles)
	api.GET("/wiki/files/children", app.handleListWikiFileChildren)
	api.GET("/wiki/files/backlinks", app.handleListWikiFileBacklinks)
	api.GET("/wiki/files/content", app.handleReadWikiFileContent)
	api.PUT("/wiki/files/content", app.handleWriteWikiFileContent)

	// Sessions
	api.POST("/sessions", app.handleCreateSession)
	api.GET("/sessions", app.handleListSessions)
	api.GET("/sessions/:id", app.handleGetSession)
	api.PATCH("/sessions/:id", app.handlePatchSession)
	api.POST("/sessions/:id/forks", app.handleForkSession)
	api.DELETE("/sessions/:id", app.handleDeleteSession)
	api.GET("/sessions/:id/messages", app.handleGetMessages)
	api.GET("/sessions/:id/media/:imageId", app.handleGetSessionMedia)
	api.GET("/events", app.handleEvents)
	api.POST("/sessions/:id/messages", app.handlePostMessage)
	api.DELETE("/sessions/:id/messages", app.handleClearSession)
	api.GET("/sessions/:id/children", app.handleListChildSessions)
	api.DELETE("/sessions/:id/runs/current", app.handleAbortSession)

	// Skills
	api.GET("/skills", app.handleListSkills)
	api.POST("/skills/sync-runs", app.handleSyncSkills)
	api.GET("/skills/:name/archive", app.handleExportSkill)
	api.DELETE("/skills/:name", app.handleDeleteSkill)
	api.GET("/skill-drafts", app.handleListSkillDrafts)
	api.GET("/skill-drafts/:name", app.handleGetSkillDraft)
	api.PUT("/skill-drafts/:name", app.handleUpdateSkillDraft)
	api.POST("/skill-drafts/:name/promote", app.handlePromoteSkillDraft)
	api.DELETE("/skill-drafts/:name", app.handleRejectSkillDraft)

	// MCP
	api.GET("/mcp/servers", app.handleListMCPServers)
	api.GET("/mcp/tools", app.handleListMCPTools)
	api.POST("/mcp/servers/:id/connection-tests", app.handleTestMCPServer)
	api.POST("/mcp/servers/:id/reconnection-runs", app.handleReconnectMCPServer)
	api.POST("/mcp/servers/:id/oauth-flows", app.handleStartMCPOAuth)

	// Memories
	api.GET("/memories", app.handleListMemories)
	api.POST("/memories", app.handleCreateMemory)
	api.DELETE("/memories/:id", app.handleDeleteMemory)
	api.POST("/memories/searches", app.handleSearchMemories)

	// Memory settings & maintenance
	api.GET("/memories/settings", app.handleGetMemorySettings)
	api.PUT("/memories/settings", app.handlePutMemorySettings)
	api.POST("/memories/reembed-preview", app.handlePreviewMemoryReembed)
	api.GET("/memories/reembed-jobs", app.handleGetMemoryReembedJob)
	api.POST("/memories/reembed-jobs", app.handleStartMemoryReembed)
	api.POST("/memories/reembed-jobs/current/cancellation", app.handleCancelMemoryReembed)
	api.POST("/memories/purge-runs", app.handlePurgeMemory)
	api.POST("/memories/compaction-runs", app.handleCompactMemory)
	api.GET("/memories/compaction-preview", app.handleCompactPreview)

	// Storage retention
	api.POST("/storage/retention/runs", app.handleRunStorageRetention)
	api.POST("/storage/backup/runs", app.handleRunBackup)

	// Jobs
	api.GET("/jobs", app.handleListJobs)
	api.POST("/jobs", app.handleCreateJob)
	api.GET("/jobs/settings", app.handleGetJobSettings)
	api.PUT("/jobs/settings", app.handlePutJobSettings)
	api.GET("/jobs/:id", app.handleGetJob)
	api.PATCH("/jobs/:id", app.handleUpdateJob)
	api.DELETE("/jobs/:id", app.handleDeleteJob)
	api.PUT("/jobs/:id/archive", app.handleArchiveJob)
	api.DELETE("/jobs/:id/archive", app.handleUnarchiveJob)
	api.POST("/jobs/:id/retry-runs", app.handleUnblockJob)
	api.GET("/jobs/:id/events", app.handleListJobEvents)
	api.PUT("/jobs/:id/lease", app.handleClaimJob)
	api.DELETE("/jobs/:id/lease", app.handleReleaseJob)
	api.PUT("/jobs/:id/completion", app.handleCompleteJob)
	api.PATCH("/jobs/:id/lease", app.handleHeartbeatJob)

	// Scheduled jobs
	api.GET("/scheduled-jobs", app.handleListScheduledJobs)
	api.POST("/scheduled-jobs", app.handleCreateScheduledJob)
	api.GET("/scheduled-jobs/:id", app.handleGetScheduledJob)
	api.PATCH("/scheduled-jobs/:id", app.handlePatchScheduledJob)
	api.DELETE("/scheduled-jobs/:id", app.handleDeleteScheduledJob)

	// Inbox
	api.GET("/inbox/messages", app.handleListInboxMessages)
	api.GET("/inbox/summary", app.handleGetInboxSummary)
	api.POST("/inbox/messages/:id/replies", app.handleReplyInboxMessage)
	api.POST("/inbox/messages/:id/dismissals", app.handleDismissInboxMessage)

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
	AgentMode        string             `json:"agent_mode"`
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
	Type       string                     `json:"type"`
	Text       string                     `json:"text,omitempty"`
	Images     []messageImageInput        `json:"images,omitempty"`
	Contexts   []transcriptMessageContext `json:"contexts,omitempty"`
	ToolName   string                     `json:"tool_name,omitempty"`
	ToolInput  any                        `json:"tool_input,omitempty"`
	ToolOutput string                     `json:"tool_output,omitempty"`
	ToolError  bool                       `json:"tool_error,omitempty"`
	Memories   []transcriptMemory         `json:"memories,omitempty"`
}

type transcriptMessageContext struct {
	Kind   string `json:"kind"`
	Title  string `json:"title,omitempty"`
	Source string `json:"source"`
	Role   string `json:"role,omitempty"`
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

type syncSkillsResponse struct {
	Created []string `json:"created"`
	Skipped []string `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}

func (a *App) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}

func (a *App) handleLookupModelCatalog(c *gin.Context) {
	var req apigen.ModelCatalogLookupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		writeError(c, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.Method) == "" && (req.ProviderId == nil || strings.TrimSpace(*req.ProviderId) == "") {
		writeError(c, http.StatusBadRequest, "bad_request", "method or provider_id is required")
		return
	}
	if len(req.ModelIds) == 0 {
		c.JSON(http.StatusOK, apigen.ModelCatalogLookupResponse{Models: []apigen.ModelCatalogLookupEntry{}})
		return
	}
	if len(req.ModelIds) > 500 {
		writeError(c, http.StatusBadRequest, "bad_request", "at most 500 model_ids are allowed")
		return
	}
	providerID := ""
	if req.ProviderId != nil {
		providerID = *req.ProviderId
	}
	looked := config.LookupModelCatalog(req.Method, providerID, req.ModelIds)
	items := make([]apigen.ModelCatalogLookupEntry, 0, len(looked))
	for _, m := range looked {
		items = append(items, apigen.ModelCatalogLookupEntry{
			ModelId:                m.ModelID,
			Context:                m.Context,
			Output:                 m.Output,
			LimitSource:            apigen.ModelCatalogLookupEntryLimitSource(m.LimitSource),
			Vision:                 m.Vision,
			VisionKnown:            m.VisionKnown,
			InputModalities:        toModelCatalogLookupInputModalities(m.InputModalities),
			ReasoningEffortOptions: optionalStringSlice(m.ReasoningEffortOptions),
		})
	}
	c.JSON(http.StatusOK, apigen.ModelCatalogLookupResponse{Models: items})
}

func (a *App) handleListSkills(c *gin.Context) {
	reg := a.skillsForRequest(c)
	items := make([]skillResource, 0, len(reg.Skills))
	for _, skill := range reg.Skills {
		items = append(items, skillResourceFromModel(skill))
	}
	c.JSON(http.StatusOK, listSkillsResponse{Skills: items, Errors: reg.Errors})
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
		AgentMode: string(w.AgentMode),
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
		for _, ctxRef := range item.Contexts {
			out.Contexts = append(out.Contexts, transcriptMessageContext{
				Kind:   ctxRef.Kind,
				Title:  ctxRef.Title,
				Source: ctxRef.Source,
				Role:   ctxRef.Role,
			})
		}
		return out
	case session.TranscriptKindReasoning:
		return transcriptItem{Type: "reasoning", Text: item.Text}
	case session.TranscriptKindAssistant:
		out := transcriptItem{Type: "assistant", Text: item.Text}
		for _, block := range item.Images {
			img := messageImageInput{
				ID:        block.ID,
				MediaType: block.MediaType,
				Data:      block.Data,
				Alt:       block.Alt,
			}
			out.Images = append(out.Images, img)
		}
		return out
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
	case session.TranscriptKindError:
		return transcriptItem{Type: "error", Text: item.Text}
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

func toModelCatalogLookupInputModalities(in []string) []apigen.ModelCatalogLookupEntryInputModalities {
	out := make([]apigen.ModelCatalogLookupEntryInputModalities, 0, len(in))
	for _, m := range in {
		out = append(out, apigen.ModelCatalogLookupEntryInputModalities(m))
	}
	return out
}

// optionalStringSlice returns nil for an empty slice so optional response
// fields are omitted from the wire instead of serialized as empty arrays.
func optionalStringSlice(in []string) *[]string {
	if len(in) == 0 {
		return nil
	}
	return &in
}
