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
	"os"
	"strings"
	"sync"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/acp"
	"github.com/Cometline/cometline/cometmind/internal/autonomy"
	"github.com/Cometline/cometline/cometmind/internal/config"
	"github.com/Cometline/cometline/cometmind/internal/db"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/inbox"
	"github.com/Cometline/cometline/cometmind/internal/inboxworker"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
	"github.com/Cometline/cometline/cometmind/internal/memory"
	"github.com/Cometline/cometline/cometmind/internal/modelcompat"
	"github.com/Cometline/cometline/cometmind/internal/paths"
	"github.com/Cometline/cometline/cometmind/internal/provider"
	"github.com/Cometline/cometline/cometmind/internal/scheduler"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/skills"
	"github.com/Cometline/cometline/cometmind/internal/sqlite"
	"github.com/Cometline/cometline/cometmind/internal/subagent"
	"github.com/Cometline/cometline/cometmind/internal/usage"
	"github.com/Cometline/cometline/cometmind/internal/wakeup"
)

// memoryExtractionConcurrency is the maximum number of extractMemoryAfterTurn
// calls that may run simultaneously across all sessions. Each completed turn
// runs one such call before the SSE stream closes; without a cap, N simultaneous
// completions would fire N concurrent LLM API calls and contend on the SQLite write lock.
const memoryExtractionConcurrency = 3

// Runtime is the composition root shared by the CLI and server.
type Runtime struct {
	Config           *config.Config
	DB               *sql.DB
	Sessions         *session.Service
	Usage            *usage.Service
	Memory           *memory.Service
	Compatibility    *modelcompat.Resolver
	Events           *event.Hub
	Jobs             *jobs.Service
	Inbox            *inbox.Service
	Scheduler        *scheduler.Service
	jobSettings      jobs.Settings
	jobSettingsMu    sync.RWMutex
	SystemPrompt     string
	acpMgr           *acp.SessionManager
	mcpMgr           *mcppkg.Manager
	subagentOrch     *subagent.Orchestrator
	memorySem        chan struct{} // bounds concurrent memory-extraction goroutines
	isRunning        func(sessionID string) bool
	retentionMu      sync.Mutex
	reloadMu         sync.Mutex
	jobsChanged      chan struct{}
	retentionChanged chan struct{}
	backupChanged    chan struct{}
	autonomyWorker   *autonomy.Worker
	inboxWorker      *inboxworker.Worker
	workers          *supervisor
	closeOnce        sync.Once
	closeErr         error
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
	sqlDB, err := sqlite.Open(ctx, dbpath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	sessions := session.New(sqlDB)
	usageSvc := usage.NewService(sqlDB)
	sessions.SetUsageRecorder(usageSvc)
	r := &Runtime{
		Config:           cfg,
		DB:               sqlDB,
		Sessions:         sessions,
		Usage:            usageSvc,
		Events:           event.NewHub(),
		SystemPrompt:     systemPrompt,
		memorySem:        make(chan struct{}, memoryExtractionConcurrency),
		jobSettings:      cfg.JobsSettings(),
		jobsChanged:      make(chan struct{}, 1),
		retentionChanged: make(chan struct{}, 1),
		backupChanged:    make(chan struct{}, 1),
		Compatibility:    modelcompat.New(db.New(sqlDB)),
		workers:          newSupervisor(),
	}
	notifier := jobs.NewNotifier(r.jobSettingsSnapshot)
	r.Jobs = jobs.NewService(sqlDB, r.jobSettingsSnapshot, notifier)
	r.Inbox = inbox.NewService(sqlDB)
	r.Scheduler = scheduler.NewService(sqlDB)
	if cfg.MemoryRuntimeEnabled() {
		r.Memory = newMemoryService(cfg, sqlDB, sessions, usageSvc)
	}
	if cfg.Skills.SynthesisEnabled {
		r.registerSkillSynthesis(notifier)
	}
	if _, err := r.RunRetention(ctx); err != nil {
		logging.L().Warn("retention.startup_failed", "error", err)
	}
	if _, err := r.Jobs.Reconcile(ctx, nil); err != nil {
		logging.L().Warn("jobs.reconcile.startup_failed", "error", err)
	}
	r.mcpMgr = mcppkg.NewManager(cfg.MCPSettings())
	// Connect MCP servers in the background so a slow or unreachable server
	// cannot block startup. The HTTP server (and health endpoint) come up
	// immediately; MCP tools are gathered lazily per agent turn, so any
	// in-progress connections simply surface their tools once ready.
	// Start is not supervised: it ignores ctx and waits out each server's
	// connect budget, so Close would stall shutdown waiting on it.
	go r.mcpMgr.Start(ctx)
	return r, nil
}

// newMemoryService returns nil (memory disabled) when the memory LLM or
// service cannot be built; startup continues without retrieval/extraction.
func newMemoryService(cfg *config.Config, sqlDB *sql.DB, sessions *session.Service, usageSvc *usage.Service) *memory.Service {
	p, err := provider.NewMemoryLLM(cfg)
	if err != nil {
		logging.L().Warn("memory.provider.init_failed",
			"error", err,
			"effect", "memory subsystem disabled; agent will run without retrieval/extraction")
		return nil
	}
	mem, err := memory.NewService(sqlDB, cfg.MemorySettings(), p, sessions)
	if err != nil {
		logging.L().Warn("memory.service.init_failed",
			"error", err,
			"effect", "memory subsystem disabled; agent will run without retrieval/extraction")
		return nil
	}
	mem.SetUsageRecorder(usageSvc)
	return mem
}

func (r *Runtime) registerSkillSynthesis(notifier *jobs.Notifier) {
	providerID, model := r.Config.ResolveRoleLLM(r.Config.Skills.SynthesisProviderID, r.Config.Skills.SynthesisModel)
	p, err := provider.NewForModel(r.Config, providerID, model)
	if err != nil {
		logging.L().Warn("skills.synthesis.provider.init_failed", "error", err)
		return
	}
	notifier.Register(&skillSynthesisNotifier{provider: p, model: model, memory: r.Memory, usage: r.Usage, sessions: r.Sessions, workers: r.workers})
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
	wakeup.Signal(r.jobsChanged)
	wakeup.Signal(r.retentionChanged)
}

// Close stops every background worker, waits for them to return, then
// releases MCP connections and the database. It is safe to call more than once.
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	r.closeOnce.Do(func() {
		r.workers.Stop()
		if r.mcpMgr != nil {
			_ = r.mcpMgr.Close()
		}
		if r.DB != nil {
			r.closeErr = r.DB.Close()
		}
	})
	return r.closeErr
}

// Reload re-reads config and applies settings that can change without a full process restart.
func (r *Runtime) Reload(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("runtime is nil")
	}
	r.reloadMu.Lock()
	defer r.reloadMu.Unlock()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("reload config: %w", err)
	}
	systemPrompt, err := loadSystemPrompt(cfg.SystemPromptPath)
	if err != nil {
		return err
	}

	if r.mcpMgr != nil {
		if err := r.mcpMgr.Reload(ctx, cfg.MCPSettings()); err != nil {
			return fmt.Errorf("reload mcp: %w", err)
		}
	}
	if r.acpMgr != nil {
		r.acpMgr.UpdateConfig(cfg.ACPSettings())
	}
	if err := r.reloadMemory(cfg); err != nil {
		return err
	}
	*r.Config = *cfg
	r.SystemPrompt = systemPrompt
	r.SetJobSettings(cfg.JobsSettings())
	if r.autonomyWorker != nil {
		r.autonomyWorker.UpdateConfig(
			cfg.EffectiveAutonomousJobsSettings(),
			r.autonomyModelIDFrom(cfg),
			r.autonomyProviderIDFrom(cfg),
		)
	}
	if r.inboxWorker != nil {
		r.inboxWorker.UpdateConfig(
			cfg.EffectiveInboxSettings(),
			r.autonomyModelIDFrom(cfg),
			r.autonomyProviderIDFrom(cfg),
		)
	}
	wakeup.Signal(r.retentionChanged)
	wakeup.Signal(r.backupChanged)
	logging.L().Info("runtime.reloaded")
	return nil
}

func (r *Runtime) reloadMemory(cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	if !cfg.MemoryRuntimeEnabled() {
		if r.Memory != nil {
			_ = r.Memory.UpdateSettings(cfg.MemorySettings())
		}
		return nil
	}
	p, err := provider.NewMemoryLLM(cfg)
	if err != nil {
		return fmt.Errorf("reload memory provider: %w", err)
	}
	if r.Memory == nil {
		mem, err := memory.NewService(r.DB, cfg.MemorySettings(), p, r.Sessions)
		if err != nil {
			return fmt.Errorf("create memory on reload: %w", err)
		}
		mem.SetUsageRecorder(r.Usage)
		r.Memory = mem
		if r.autonomyWorker != nil {
			r.autonomyWorker.Memory = mem
		}
		return nil
	}
	if err := r.Memory.UpdateSettings(cfg.MemorySettings()); err != nil {
		return fmt.Errorf("reload memory settings: %w", err)
	}
	r.Memory.SetProvider(p)
	return nil
}

func (r *Runtime) autonomyProviderIDFrom(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	providerID, _ := cfg.ResolveRoleLLM(cfg.Autonomy.ProviderID, cfg.Autonomy.ModelID)
	return providerID
}

func (r *Runtime) autonomyModelIDFrom(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	_, modelID := cfg.ResolveRoleLLM(cfg.Autonomy.ProviderID, cfg.Autonomy.ModelID)
	return modelID
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
	return provider.NewForModel(&cfg, sess.ProviderID, sess.ModelID)
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

// SkillsForWorkspace discovers Agent Skills visible to one workspace.
func (r *Runtime) SkillsForWorkspace(workspacePath string) skills.Registry {
	return skills.Discover(workspacePath, r.Config.SkillSettings())
}
