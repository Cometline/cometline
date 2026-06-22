// Package runtime is the shared composition root for CometMind commands.
//
// It owns config loading, SQLite opening, and the wiring that turns a
// persisted session into a runnable agent. Commands become thin: they call
// runtime.New, ask it for whatever service they need, and defer Close.
package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/agent"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/memory"
	mcppkg "github.com/cometline/cometmind/internal/mcp"
	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/cometline/cometmind/internal/retention"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/skills"
	"github.com/cometline/cometmind/internal/store"
	"github.com/cometline/cometmind/internal/subagent"
	"github.com/cometline/cometmind/internal/tools"
)

// memoryExtractionConcurrency is the maximum number of extractMemoryBackground
// goroutines that may run simultaneously across all sessions. Each completed
// turn spawns one such goroutine; without a cap, N simultaneous completions
// would fire N concurrent LLM API calls and contend on the SQLite write lock.
const memoryExtractionConcurrency = 3

// Runtime is the composition root shared by the CLI and server.
type Runtime struct {
	Config       *config.Config
	DB           *sql.DB
	Sessions     *session.Service
	Memory       *memory.Service
	Jobs         *jobs.Service
	jobSettings  jobs.Settings
	jobSettingsMu sync.RWMutex
	SystemPrompt string
	acpMgr       *acp.SessionManager
	mcpMgr       *mcppkg.Manager
	subagentOrch *subagent.Orchestrator
	memorySem    chan struct{} // bounds concurrent memory-extraction goroutines
	isRunning    func(sessionID string) bool
}

// New builds a Runtime from the environment and filesystem.
func New(ctx context.Context) (*Runtime, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	systemPrompt, err := loadSystemPrompt(cfg.SystemPromptPath)
	if err != nil {
		return nil, err
	}

	dbpath, err := paths.DBPath()
	if err != nil {
		return nil, fmt.Errorf("db path: %w", err)
	}
	sqlDB, err := store.OpenSQLite(ctx, dbpath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sessions := session.New(sqlDB)
	r := &Runtime{
		Config:       cfg,
		DB:           sqlDB,
		Sessions:     sessions,
		SystemPrompt: systemPrompt,
		memorySem:    make(chan struct{}, memoryExtractionConcurrency),
		jobSettings:  cfg.JobsSettings(),
	}
	notifier := jobs.NewNotifier(r.jobSettingsSnapshot)
	r.Jobs = jobs.NewService(sqlDB, r.jobSettingsSnapshot, notifier)
	if cfg.MemoryRuntimeEnabled() {
		p, err := provider.New(cfg)
		if err == nil {
			mem, err := memory.NewService(sqlDB, cfg.MemorySettings(), p, sessions)
			if err == nil {
				r.Memory = mem
			}
		}
	}
	runRetention(ctx, sqlDB, sessions, r.Memory, r.Jobs, cfg.EffectiveStorageConfig(), nil)
	if _, err := r.Jobs.Reconcile(ctx, nil); err != nil {
		log.Printf("cometmind: jobs reconcile on startup failed: %v", err)
	}
	r.mcpMgr = mcppkg.NewManager(cfg.MCPSettings())
	r.mcpMgr.Start(ctx)
	return r, nil
}

func runRetention(ctx context.Context, db *sql.DB, sessions *session.Service, mem *memory.Service, jobSvc *jobs.Service, cfg config.StorageConfig, isRunning func(string) bool) {
	if !cfg.RetentionEnabled() && !cfg.MemoryPurgeEnabled() && !cfg.JobPurgeEnabled() {
		return
	}
	rr := &retention.Runner{
		DB:       db,
		Sessions: sessions,
		Memory:   mem,
		Jobs:     jobSvc,
		Config:   cfg,
		IsRunning: isRunning,
	}
	if _, err := rr.Run(ctx); err != nil {
		log.Printf("cometmind: retention failed: %v", err)
	}
}

func loadSystemPrompt(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read system prompt %q: %w", path, err)
	}
	return strings.TrimSpace(string(raw)), nil
}

// SetSessionRunningChecker sets the callback used to detect in-flight agent turns.
func (r *Runtime) SetSessionRunningChecker(fn func(sessionID string) bool) {
	if r == nil {
		return
	}
	r.isRunning = fn
}

// StartJobsMaintenance runs periodic orphan reconcile and optional purge.
func (r *Runtime) StartJobsMaintenance(ctx context.Context) {
	if r == nil || r.Jobs == nil {
		return
	}
	interval := time.Duration(r.jobSettingsSnapshot().ReconcileIntervalS) * time.Second
	if interval <= 0 {
		interval = jobs.DefaultReconcileInterval
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := r.Jobs.Reconcile(ctx, r.isRunning); err != nil {
					log.Printf("cometmind: jobs reconcile failed: %v", err)
				}
				cfg := r.Config.EffectiveStorageConfig()
				if cfg.JobPurgeEnabled() {
					if _, err := r.Jobs.PurgeDeleted(ctx, cfg.DeletedJobPurgeDays); err != nil {
						log.Printf("cometmind: jobs purge failed: %v", err)
					}
				}
			}
		}
	}()
}

func (r *Runtime) jobSettingsSnapshot() jobs.Settings {
	if r == nil {
		return jobs.DefaultSettings()
	}
	r.jobSettingsMu.RLock()
	defer r.jobSettingsMu.RUnlock()
	return r.jobSettings
}

// SetJobSettings updates runtime job settings.
func (r *Runtime) SetJobSettings(s jobs.Settings) {
	if r == nil {
		return
	}
	r.jobSettingsMu.Lock()
	r.jobSettings = s
	r.jobSettingsMu.Unlock()
}

func (r *Runtime) Close() error {
	if r.mcpMgr != nil {
		_ = r.mcpMgr.Close()
	}
	return r.DB.Close()
}

// WorkspaceForCommand resolves the current directory (or the explicit workspace
// flag when passed) to a persisted workspace.
func (r *Runtime) WorkspaceForCommand(ctx context.Context, explicitWorkspace string) (session.Workspace, error) {
	root, err := paths.ResolveWorkspace(explicitWorkspace)
	if err != nil {
		return session.Workspace{}, fmt.Errorf("workspace root: %w", err)
	}
	return r.Sessions.EnsureWorkspace(ctx, root)
}

// ProviderForSession builds a provider configured for the given session's
// model/provider identifiers. The runtime's base config is copied so per-session
// overrides do not leak back into the global config.
func (r *Runtime) ProviderForSession(sess session.Session) (cometsdk.Provider, error) {
	cfg := *r.Config
	cfg.Model = sess.ModelID
	return provider.NewFor(&cfg, sess.ProviderID)
}

// ACPManager returns the shared ACP session manager.
func (r *Runtime) ACPManager() *acp.SessionManager {
	if r.acpMgr == nil {
		r.acpMgr = acp.NewSessionManager(r.Config.ACPSettings())
	}
	return r.acpMgr
}

// MCPManager returns the shared MCP client manager.
func (r *Runtime) MCPManager() *mcppkg.Manager {
	return r.mcpMgr
}

// SubagentOrchestrator returns the shared subagent orchestrator.
func (r *Runtime) SubagentOrchestrator() *subagent.Orchestrator {
	if r.subagentOrch == nil {
		r.subagentOrch = subagent.NewOrchestrator(r.Config.EffectiveSubagentSettings().MaxConcurrentPerParent)
	}
	return r.subagentOrch
}

// RunnerFor returns an agent runner wired for a specific session and workspace.
func (r *Runtime) RunnerFor(sess session.Session, workspacePath string) (*agent.Runner, error) {
	p, err := r.ProviderForSession(sess)
	if err != nil {
		return nil, err
	}
	skillRegistry := r.SkillsForWorkspace(workspacePath)

	return &agent.Runner{
		Config:       r.Config,
		Provider:     p,
		Sessions:     r.Sessions,
		Memory:       r.Memory,
		Registry:     r.toolRegistry(workspacePath, skillRegistry, sess.ID),
		Jobs:         r.Jobs,
		MaxSteps:     r.Config.MaxSteps,
		MaxTokens:    r.Config.MaxTokens,
		SystemPrompt: r.SystemPrompt,
		SkillIndex:   skillRegistry.PromptIndex(),
		JobIndex:     tools.JobPromptIndex(workspacePath, jobs.PlatformDesktop),
		MemorySem:    r.memorySem,
		Compactor:    &agent.ContextCompactor{Sessions: r.Sessions, Config: r.Config},
	}, nil
}

// SubagentRunnerFor returns a restricted runner for a general subagent child session.
func (r *Runtime) SubagentRunnerFor(child session.Session, workspacePath string, maxSteps int) (*agent.Runner, error) {
	p, err := r.ProviderForSession(child)
	if err != nil {
		return nil, err
	}
	skillRegistry := r.SkillsForWorkspace(workspacePath)
	return &agent.Runner{
		Config:       r.Config,
		Provider:     p,
		Sessions:     r.Sessions,
		Registry:     tools.NewSubagentRegistry(workspacePath, &skillRegistry),
		MaxSteps:     maxSteps,
		MaxTokens:    r.Config.MaxTokens,
		SystemPrompt: r.SystemPrompt,
		SkillIndex:   skillRegistry.PromptIndex(),
	}, nil
}

// RunnerForGateway is like RunnerFor but tags job tool metadata for a gateway channel.
func (r *Runtime) RunnerForGateway(sess session.Session, workspacePath, platform, sourceChannelID string) (*agent.Runner, error) {
	p, err := r.ProviderForSession(sess)
	if err != nil {
		return nil, err
	}
	skillRegistry := r.SkillsForWorkspace(workspacePath)

	return &agent.Runner{
		Config:       r.Config,
		Provider:     p,
		Sessions:     r.Sessions,
		Memory:       r.Memory,
		Registry:     r.toolRegistryWithJobMeta(workspacePath, skillRegistry, sess.ID, platform, sourceChannelID),
		Jobs:         r.Jobs,
		MaxSteps:     r.Config.MaxSteps,
		MaxTokens:    r.Config.MaxTokens,
		SystemPrompt: r.SystemPrompt,
		SkillIndex:   skillRegistry.PromptIndex(),
		JobIndex:     tools.JobPromptIndex(workspacePath, platform),
		MemorySem:    r.memorySem,
		Compactor:    &agent.ContextCompactor{Sessions: r.Sessions, Config: r.Config},
	}, nil
}

func (r *Runtime) toolRegistry(workspacePath string, skillRegistry skills.Registry, sessionID string) *tools.Registry {
	return r.toolRegistryWithJobMeta(workspacePath, skillRegistry, sessionID, jobs.PlatformDesktop, "")
}

func (r *Runtime) toolRegistryWithJobMeta(workspacePath string, skillRegistry skills.Registry, sessionID, platform, sourceChannelID string) *tools.Registry {
	sub := r.Config.EffectiveSubagentSettings()
	return tools.NewRegistry(workspacePath, tools.RegistryOptions{
		Sessions:           r.Sessions,
		ACP:                r.Config.ACPSettings(),
		ACPMgr:             r.ACPManager(),
		Skills:             &skillRegistry,
		MCP:                r.mcpMgr,
		Orchestrator:       r.SubagentOrchestrator(),
		Jobs:               r.Jobs,
		SessionID:          sessionID,
		JobPlatform:        platform,
		JobSourceChannelID: sourceChannelID,
		RunnerFactory: func(child session.Session, workspaceRoot string, maxSteps int) (tools.AgentLoopRunner, error) {
			return r.SubagentRunnerFor(child, workspaceRoot, maxSteps)
		},
		SubagentConfig: tools.SubagentToolConfig{
			GeneralMaxSteps: sub.GeneralMaxSteps,
			WaitTimeoutSec:  1800,
		},
	})
}

// SkillsForWorkspace discovers Agent Skills visible to one workspace.
func (r *Runtime) SkillsForWorkspace(workspacePath string) skills.Registry {
	return skills.Discover(workspacePath, r.Config.SkillSettings())
}
