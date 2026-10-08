package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/acp"
	"github.com/Cometline/cometline/cometmind/internal/apigen"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/inbox"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
	"github.com/Cometline/cometline/cometmind/internal/memory"
	"github.com/Cometline/cometline/cometmind/internal/retention"
	"github.com/Cometline/cometline/cometmind/internal/scheduler"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/skillcurator"
	"github.com/Cometline/cometline/cometmind/internal/subagent"
	"github.com/Cometline/cometline/cometmind/internal/usage"
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
	Usage          *usage.Service
	Scheduler      *scheduler.Service
	RunRetention   RetentionRunner
	RunBackup      BackupRunner
	SetJobSettings func(jobs.Settings)
	NewRunner      RunnerFactory
	Runs           *RunManager
	SessionEvents  *event.SessionHub
	// RunContext owns agent lifetimes independently from individual SSE
	// requests. A renderer disconnect must not cancel the model/tool loop.
	RunContext   context.Context
	ACPMgr       *acp.SessionManager
	MCPMgr       *mcppkg.Manager
	SubagentOrch *subagent.Orchestrator
	Curator      *skillcurator.Service
}

type App struct {
	config         *config.Config
	sessions       *session.Service
	memory         *memory.Service
	events         *event.Hub
	jobs           *jobs.Service
	inbox          *inbox.Service
	usage          *usage.Service
	scheduler      *scheduler.Service
	runRetention   RetentionRunner
	runBackup      BackupRunner
	setJobSettings func(jobs.Settings)
	newRunner      RunnerFactory
	runs           *RunManager
	sessionEvents  *event.SessionHub
	runContext     context.Context
	acpMgr         *acp.SessionManager
	mcpMgr         *mcppkg.Manager
	subagentOrch   *subagent.Orchestrator
	curator        *skillcurator.Service
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
		return nil, fmt.Errorf("run manager is required")
	}
	if deps.SessionEvents == nil {
		deps.SessionEvents = event.NewSessionHub()
	}
	deps.Runs.SetOnFinished(func(sessionID, runID string) {
		deps.SessionEvents.Finish(sessionID, runID)
		if deps.Events != nil {
			deps.Events.Publish(event.RunFinished(sessionID))
		}
	})
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
		usage:          deps.Usage,
		scheduler:      deps.Scheduler,
		runRetention:   deps.RunRetention,
		runBackup:      deps.RunBackup,
		setJobSettings: deps.SetJobSettings,
		newRunner:      deps.NewRunner,
		runs:           deps.Runs,
		sessionEvents:  deps.SessionEvents,
		runContext:     runContext,
		acpMgr:         deps.ACPMgr,
		mcpMgr:         deps.MCPMgr,
		subagentOrch:   deps.SubagentOrch,
		curator:        deps.Curator,
	}
	if deps.Memory != nil && deps.Events != nil {
		deps.Memory.SetCompactionCompletedNotifier(func(result memory.CompactionResult) {
			deps.Events.Publish(event.MemoryCompactionCompleted(result.Before, result.After, result.Trigger))
		})
	}

	return newEngine(app), nil
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
	Origin           string             `json:"origin"`
	TokenUsage       tokenUsageResource `json:"token_usage"`
	AgentMode        string             `json:"agent_mode"`
	ParentSessionID  string             `json:"parent_session_id,omitempty"`
	Purpose          string             `json:"purpose,omitempty"`
	DelegationStatus string             `json:"delegation_status,omitempty"`
	OutputSummary    string             `json:"output_summary,omitempty"`
	SubagentKind     string             `json:"subagent_kind,omitempty"`
	Gateway          *gatewayResource   `json:"gateway,omitempty"`
	Running          bool               `json:"running"`
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
	Media      []messageImageInput        `json:"media,omitempty"`
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
	SessionID  string           `json:"session_id"`
	Items      []transcriptItem `json:"items"`
	HasMore    bool             `json:"has_more,omitempty"`
	NextBefore string           `json:"next_before,omitempty"`
}

type statusResponse struct {
	Status string `json:"status"`
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

func sessionResourceFromAPISession(w session.WireSession) sessionResource {
	res := sessionResource{
		ID:            w.ID,
		WorkspaceID:   w.WorkspaceID,
		WorkspacePath: w.WorkspacePath,
		Title:         w.Title,
		ModelID:       w.ModelID,
		ProviderID:    w.ProviderID,
		Status:        string(w.Status),
		Origin:        string(w.Origin),
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
	if w.ParentSessionID != nil {
		res.ParentSessionID = *w.ParentSessionID
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
			out.Media = append(out.Media, messageImageInput{MediaType: block.MediaType, Data: block.Data})
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
			out.Media = append(out.Media, img)
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
