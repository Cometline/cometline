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
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/agent"
	"github.com/cometline/cometmind/internal/autonomy"
	"github.com/cometline/cometmind/internal/backup"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/inbox"
	"github.com/cometline/cometmind/internal/inboxworker"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/logging"
	mcppkg "github.com/cometline/cometmind/internal/mcp"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/modelcompat"
	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/cometline/cometmind/internal/retention"
	"github.com/cometline/cometmind/internal/scheduler"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/skills"
	"github.com/cometline/cometmind/internal/store"
	"github.com/cometline/cometmind/internal/subagent"
	"github.com/cometline/cometmind/internal/tools"
)

// memoryExtractionConcurrency is the maximum number of extractMemoryAfterTurn
// calls that may run simultaneously across all sessions. Each completed turn
// runs one such call before the SSE stream closes; without a cap, N simultaneous
// completions would fire N concurrent LLM API calls and contend on the SQLite write lock.
const memoryExtractionConcurrency = 3

// Runtime is the composition root shared by the CLI and server.
type Runtime struct {
	Config         *config.Config
	DB             *sql.DB
	Sessions       *session.Service
	Memory         *memory.Service
	Compatibility  *modelcompat.Resolver
	Events         *event.Hub
	Jobs           *jobs.Service
	Inbox          *inbox.Service
	Scheduler      *scheduler.Service
	jobSettings    jobs.Settings
	jobSettingsMu  sync.RWMutex
	SystemPrompt   string
	acpMgr         *acp.SessionManager
	mcpMgr         *mcppkg.Manager
	subagentOrch   *subagent.Orchestrator
	memorySem      chan struct{} // bounds concurrent memory-extraction goroutines
	isRunning      func(sessionID string) bool
	retentionMu    sync.Mutex
	reloadMu       sync.Mutex
	autonomyWorker *autonomy.Worker
	inboxWorker    *inboxworker.Worker
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
		Config:        cfg,
		DB:            sqlDB,
		Sessions:      sessions,
		Events:        event.NewHub(),
		SystemPrompt:  systemPrompt,
		memorySem:     make(chan struct{}, memoryExtractionConcurrency),
		jobSettings:   cfg.JobsSettings(),
		Compatibility: modelcompat.New(db.New(sqlDB)),
	}
	notifier := jobs.NewNotifier(r.jobSettingsSnapshot)
	r.Jobs = jobs.NewService(sqlDB, r.jobSettingsSnapshot, notifier)
	r.Inbox = inbox.NewService(sqlDB)
	r.Scheduler = scheduler.NewService(sqlDB)
	if cfg.MemoryRuntimeEnabled() {
		p, err := provider.NewMemoryLLM(cfg)
		if err != nil {
			logging.L().Warn("memory.provider.init_failed",
				"error", err,
				"effect", "memory subsystem disabled; agent will run without retrieval/extraction")
		} else {
			mem, err := memory.NewService(sqlDB, cfg.MemorySettings(), p, sessions)
			if err != nil {
				logging.L().Warn("memory.service.init_failed",
					"error", err,
					"effect", "memory subsystem disabled; agent will run without retrieval/extraction")
			} else {
				r.Memory = mem
			}
		}
	}
	if cfg.Skills.SynthesisEnabled {
		providerID, model := cfg.ResolveRoleLLM(cfg.Skills.SynthesisProviderID, cfg.Skills.SynthesisModel)
		p, err := provider.NewForModel(cfg, providerID, model)
		if err != nil {
			logging.L().Warn("skills.synthesis.provider.init_failed", "error", err)
		} else {
			notifier.Register(&skillSynthesisNotifier{provider: p, model: model, memory: r.Memory})
		}
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
	go r.mcpMgr.Start(ctx)
	return r, nil
}

// StartScheduler materializes due scheduled jobs into the normal jobs queue.
func (r *Runtime) StartScheduler(ctx context.Context) {
	if r == nil || r.Scheduler == nil || r.Jobs == nil {
		return
	}
	cfg := r.Config.EffectiveSchedulerSettings()
	if !cfg.Enabled {
		return
	}
	interval := time.Duration(cfg.PollIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				count, err := r.Scheduler.MaterializeDue(ctx, r.Jobs, 0)
				if err != nil {
					logging.L().Warn("scheduler.materialize.failed", "error", err)
				} else if count > 0 {
					logging.L().Info("scheduler.materialized", "count", count)
				}
			}
		}
	}()
}

func runRetention(ctx context.Context, db *sql.DB, sessions *session.Service, mem *memory.Service, jobSvc *jobs.Service, inboxSvc *inbox.Service, cfg config.StorageConfig, inboxCfg config.InboxConfig, isRunning func(string) bool) (retention.Result, error) {
	var out retention.Result
	needDB := cfg.RetentionEnabled() || cfg.MemoryPurgeEnabled() || cfg.JobPurgeEnabled() || inboxSvc != nil
	if needDB {
		rr := &retention.Runner{
			DB:          db,
			Sessions:    sessions,
			Memory:      mem,
			Jobs:        jobSvc,
			Inbox:       inboxSvc,
			Config:      cfg,
			InboxCfg:    inboxCfg,
			IsRunning:   isRunning,
			VacuumAsync: true,
		}
		var err error
		out, err = rr.Run(ctx)
		if err != nil {
			return out, err
		}
	}
	if cfg.RuntimeFilesEnabled() {
		to, at := retention.PurgeRuntimeFiles(cfg)
		out.ToolOutputDeleted = to
		out.AgentTmpDeleted = at
		if to > 0 || at > 0 {
			logging.L().Info("retention.runtime_files",
				"tool_output_deleted", to,
				"agent_tmp_deleted", at,
			)
		}
	}
	return out, nil
}

// RunRetention executes the configured storage retention rules once.
func (r *Runtime) RunRetention(ctx context.Context) (retention.Result, error) {
	if r == nil {
		return retention.Result{}, fmt.Errorf("runtime is nil")
	}
	r.retentionMu.Lock()
	defer r.retentionMu.Unlock()
	result, err := runRetention(ctx, r.DB, r.Sessions, r.Memory, r.Jobs, r.Inbox, r.Config.EffectiveStorageConfig(), r.Config.EffectiveInboxSettings(), r.isRunning)
	if err != nil {
		return result, err
	}
	return result, nil
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
// The reconcile interval is re-read each cycle so Reload can change it live.
func (r *Runtime) StartJobsMaintenance(ctx context.Context) {
	if r == nil || r.Jobs == nil {
		return
	}
	go func() {
		for {
			interval := time.Duration(r.jobSettingsSnapshot().ReconcileIntervalS) * time.Second
			if interval <= 0 {
				interval = jobs.DefaultReconcileInterval
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			if _, err := r.Jobs.Reconcile(ctx, r.isRunning); err != nil {
				logging.L().Warn("jobs.reconcile.failed", "error", err)
			}
			settings := r.jobSettingsSnapshot()
			if stale, err := r.Jobs.StaleOngoing(ctx, time.Duration(settings.StaleReviewMinutes)*time.Minute); err != nil {
				logging.L().Warn("jobs.stale_review.failed", "error", err)
			} else {
				for _, job := range stale {
					logging.L().Warn("jobs.stale_ongoing", "job_id", job.ID, "assigned_session_id", job.AssignedSessionID, "updated_at", job.UpdatedAt)
				}
			}
			cfg := r.Config.EffectiveStorageConfig()
			if cfg.JobPurgeEnabled() {
				if _, err := r.Jobs.PurgeDeleted(ctx, cfg.DeletedJobPurgeDays); err != nil {
					logging.L().Warn("jobs.purge.failed", "error", err)
				}
			}
			if _, err := r.Jobs.ArchiveDone(ctx, settings.DoneArchiveDays); err != nil {
				logging.L().Warn("jobs.done_archive.failed", "error", err)
			}
			if _, err := r.Jobs.PurgeArchived(ctx, settings.ArchivedPurgeDays); err != nil {
				logging.L().Warn("jobs.archive_purge.failed", "error", err)
			}
		}
	}()
}

// StartRetentionMaintenance runs full storage retention on the configured interval.
// Interval and enablement are re-read each cycle so Reload can change them live.
func (r *Runtime) StartRetentionMaintenance(ctx context.Context) {
	if r == nil {
		return
	}
	go func() {
		for {
			cfg := r.Config.EffectiveStorageConfig()
			enabled := cfg.RetentionEnabled() || cfg.MemoryPurgeEnabled() || cfg.JobPurgeEnabled() || cfg.RuntimeFilesEnabled()
			interval := time.Duration(cfg.CleanupIntervalMinutes) * time.Minute
			if interval <= 0 {
				interval = time.Hour
			}
			if !enabled {
				interval = 2 * time.Minute
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			if !enabled {
				continue
			}
			result, err := r.RunRetention(ctx)
			if err != nil {
				logging.L().Warn("retention.failed", "error", err)
				continue
			}
			if result.SessionsDeleted > 0 || result.SubagentsDeleted > 0 || result.MemoriesPurged > 0 || result.MemoryEventsPurged > 0 || result.JobsPurged > 0 || result.InboxPurged > 0 || result.ToolOutputDeleted > 0 || result.AgentTmpDeleted > 0 {
				logging.L().Info("retention.completed",
					"sessions_deleted", result.SessionsDeleted,
					"subagents_deleted", result.SubagentsDeleted,
					"memories_purged", result.MemoriesPurged,
					"memory_events_purged", result.MemoryEventsPurged,
					"jobs_purged", result.JobsPurged,
					"inbox_purged", result.InboxPurged,
					"tool_output_deleted", result.ToolOutputDeleted,
					"agent_tmp_deleted", result.AgentTmpDeleted,
					"vacuumed", result.Vacuumed,
				)
			}
		}
	}()
}

// RunBackup creates one zip archive of ~/.cometmind in the configured destination.
func (r *Runtime) RunBackup(ctx context.Context) (backup.Result, error) {
	if r == nil {
		return backup.Result{}, fmt.Errorf("runtime is nil")
	}
	cfg := r.Config.EffectiveStorageConfig().Backup
	if strings.TrimSpace(cfg.DestinationDir) == "" {
		return backup.Result{}, fmt.Errorf("backup destination directory is not configured")
	}
	return (&backup.Archiver{DB: r.DB}).Run(ctx, backup.Config{
		DestinationDir: cfg.DestinationDir,
		MaxBackups:     cfg.MaxBackups,
	})
}

// StartBackupMaintenance runs automatic zip backups on the configured interval.
func (r *Runtime) StartBackupMaintenance(ctx context.Context) {
	if r == nil {
		return
	}
	go func() {
		for {
			cfg := r.Config.EffectiveStorageConfig()
			enabled := cfg.BackupEnabled()
			interval := time.Duration(cfg.Backup.IntervalHours) * time.Hour
			if interval <= 0 {
				interval = 24 * time.Hour
			}
			if !enabled {
				interval = 2 * time.Minute
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(interval):
			}
			if !enabled {
				continue
			}
			result, err := r.RunBackup(ctx)
			if err != nil {
				logging.L().Warn("backup.failed", "error", err)
				continue
			}
			logging.L().Info("backup.completed",
				"path", result.Path,
				"files_zipped", result.FilesZipped,
				"removed_old", result.RemovedOld,
			)
		}
	}()
}

// StartAutonomousJobWorker starts the background worker that claims and
// executes ready jobs without a human opening a chat session first.
func (r *Runtime) StartAutonomousJobWorker(ctx context.Context, guard autonomy.RunGuard) {
	if r == nil || r.Jobs == nil || r.Sessions == nil {
		return
	}
	w := &autonomy.Worker{
		Jobs:              r.Jobs,
		Sessions:          r.Sessions,
		Memory:            r.Memory,
		NewRunner:         r.RunnerFor,
		Guard:             guard,
		Config:            r.Config.EffectiveAutonomousJobsSettings(),
		DefaultModelID:    r.autonomyModelID(),
		DefaultProviderID: r.autonomyProviderID(),
	}
	r.autonomyWorker = w
	go w.Run(ctx)
}

// StartInboxWorker starts the background loop that internalizes user inbox replies.
func (r *Runtime) StartInboxWorker(ctx context.Context, guard inboxworker.RunGuard) {
	if r == nil || r.Inbox == nil || r.Sessions == nil {
		return
	}
	w := &inboxworker.Worker{
		Inbox:             r.Inbox,
		Sessions:          r.Sessions,
		Jobs:              r.Jobs,
		Memory:            r.Memory,
		Events:            r.Events,
		Guard:             guard,
		Config:            r.Config.EffectiveInboxSettings(),
		DefaultModelID:    r.autonomyModelID(),
		DefaultProviderID: r.autonomyProviderID(),
		NewRunner: func(sess session.Session, workspacePath string, registry *tools.Registry, maxSteps int) (*agent.Runner, error) {
			return r.RunnerForInbox(sess, workspacePath, registry, maxSteps)
		},
	}
	r.inboxWorker = w
	go w.Run(ctx)
}

// RunnerForInbox builds a runner with a caller-supplied limited tool registry.
func (r *Runtime) RunnerForInbox(sess session.Session, workspacePath string, registry *tools.Registry, maxSteps int) (*agent.Runner, error) {
	p, err := r.ProviderForSession(sess)
	if err != nil {
		return nil, err
	}
	if maxSteps <= 0 {
		maxSteps = 8
	}
	return &agent.Runner{
		Config:       r.Config,
		Provider:     p,
		Sessions:     r.Sessions,
		Memory:       r.Memory,
		Registry:     registry,
		Jobs:         r.Jobs,
		MaxSteps:     maxSteps,
		MaxTokens:    r.Config.MaxTokens,
		SystemPrompt: "You internalize inbox replies into durable memory when warranted. Prefer no-op over noisy memories.",
		MemorySem:    r.memorySem,
	}, nil
}

func (r *Runtime) autonomyProviderID() string {
	if r == nil || r.Config == nil {
		return ""
	}
	providerID, _ := r.Config.ResolveRoleLLM(r.Config.Autonomy.ProviderID, r.Config.Autonomy.ModelID)
	return providerID
}

func (r *Runtime) autonomyModelID() string {
	if r == nil || r.Config == nil {
		return ""
	}
	_, modelID := r.Config.ResolveRoleLLM(r.Config.Autonomy.ProviderID, r.Config.Autonomy.ModelID)
	return modelID
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
	*r.Config = *cfg
	r.SystemPrompt = systemPrompt
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

// RunnerOptions controls how a runner is assembled. Most callers should use
// RunnerFor, RunnerForMode, SubagentRunnerFor, or RunnerForGateway instead.
type RunnerOptions struct {
	MaxSteps        int
	Platform        string
	SourceChannelID string
	Subagent        bool
	SubagentMode    tools.SubagentMode
	AgentMode       session.AgentMode
}

// RunnerFor returns an agent runner wired for a specific session and workspace
// using the session's persisted agent mode.
func (r *Runtime) RunnerFor(sess session.Session, workspacePath string) (*agent.Runner, error) {
	mode, err := session.ParseAgentMode(sess.AgentMode)
	if err != nil {
		return nil, err
	}
	return r.runnerFor(sess, workspacePath, RunnerOptions{AgentMode: mode})
}

// RunnerForMode returns an agent runner wired for a specific session, workspace,
// and explicit per-turn agent mode.
func (r *Runtime) RunnerForMode(sess session.Session, workspacePath string, mode session.AgentMode) (*agent.Runner, error) {
	return r.runnerFor(sess, workspacePath, RunnerOptions{AgentMode: mode})
}

// SubagentRunnerFor returns a runner for an in-process subagent child session.
// mode selects research (read-only) vs coding (edit/write/run) tool surface.
func (r *Runtime) SubagentRunnerFor(child session.Session, workspacePath string, maxSteps int, mode tools.SubagentMode) (*agent.Runner, error) {
	return r.runnerFor(child, workspacePath, RunnerOptions{
		MaxSteps:     maxSteps,
		Subagent:     true,
		SubagentMode: mode,
	})
}

// RunnerForGateway is like RunnerFor but tags job tool metadata for a gateway channel.
func (r *Runtime) RunnerForGateway(sess session.Session, workspacePath, platform, sourceChannelID string) (*agent.Runner, error) {
	return r.runnerFor(sess, workspacePath, RunnerOptions{Platform: platform, SourceChannelID: sourceChannelID})
}

func (r *Runtime) runnerFor(sess session.Session, workspacePath string, opts RunnerOptions) (*agent.Runner, error) {
	p, err := r.ProviderForSession(sess)
	if err != nil {
		return nil, err
	}
	skillRegistry := r.SkillsForWorkspace(workspacePath)

	maxSteps := opts.MaxSteps
	if maxSteps == 0 {
		maxSteps = r.Config.MaxSteps
	}
	platform := opts.Platform
	if platform == "" {
		platform = jobs.PlatformDesktop
	}

	var registry *tools.Registry
	if opts.Subagent {
		mode := opts.SubagentMode
		if mode == "" {
			mode = tools.SubagentModeResearch
		}
		registry = tools.NewSubagentRegistry(workspacePath, &skillRegistry, mode)
	} else {
		mode := opts.AgentMode
		if mode == "" {
			var err error
			mode, err = session.ParseAgentMode(sess.AgentMode)
			if err != nil {
				return nil, err
			}
		}
		registry = r.toolRegistryWithJobMeta(workspacePath, skillRegistry, sess.ID, platform, opts.SourceChannelID, mode)
	}

	runner := &agent.Runner{
		Config:               r.Config,
		Provider:             p,
		Sessions:             r.Sessions,
		Memory:               r.Memory,
		Registry:             registry,
		Jobs:                 r.Jobs,
		MaxSteps:             maxSteps,
		MaxTokens:            r.Config.MaxTokens,
		SystemPrompt:         r.SystemPrompt,
		SkillIndex:           skillRegistry.PromptIndex(),
		SubagentOrchestrator: r.subagentOrchestratorForRunner(opts.Subagent),
		MemorySem:            r.memorySem,
		Compatibility:        r.Compatibility,
		CompatibilityScope: cometsdk.CapabilityScope{
			ProviderID: sess.ProviderID,
			Endpoint:   provider.CompatibilityEndpoint(r.Config, sess.ProviderID),
			ModelID:    sess.ModelID,
		},
	}
	if !opts.Subagent {
		runner.JobIndex = tools.JobPromptIndex(workspacePath, platform)
		runner.Compactor = &agent.ContextCompactor{Sessions: r.Sessions, Config: r.Config}
	}
	return runner, nil
}

func (r *Runtime) subagentOrchestratorForRunner(isSubagent bool) *subagent.Orchestrator {
	if isSubagent {
		return nil
	}
	return r.SubagentOrchestrator()
}

func (r *Runtime) toolRegistryWithJobMeta(workspacePath string, skillRegistry skills.Registry, sessionID, platform, sourceChannelID string, mode session.AgentMode) *tools.Registry {
	sub := r.Config.EffectiveSubagentSettings()
	return tools.NewRegistry(workspacePath, tools.RegistryOptions{
		Sessions:           r.Sessions,
		AssistantMedia:     r.Sessions,
		ACP:                r.Config.ACPSettings(),
		ACPMgr:             r.ACPManager(),
		Skills:             &skillRegistry,
		MCP:                r.mcpMgr,
		Orchestrator:       r.SubagentOrchestrator(),
		Jobs:               r.Jobs,
		Scheduler:          r.Scheduler,
		Inbox:              r.Inbox,
		Memory:             r.Memory,
		MemoryEvents:       r.Events,
		SessionID:          sessionID,
		JobPlatform:        platform,
		JobSourceChannelID: sourceChannelID,
		AgentMode:          mode,
		BrowserSearchURL:     os.Getenv("COMETLINE_BROWSER_SEARCH_URL"),
		BrowserSearchToken:   os.Getenv("COMETLINE_BROWSER_SEARCH_TOKEN"),
		ScreenCaptureURL:     os.Getenv("COMETLINE_SCREEN_CAPTURE_URL"),
		ScreenCaptureToken:   os.Getenv("COMETLINE_SCREEN_CAPTURE_TOKEN"),
		SettingsRuntime:      r,
		RunnerFactory: func(child session.Session, workspaceRoot string, maxSteps int, mode tools.SubagentMode) (tools.AgentLoopRunner, error) {
			return r.SubagentRunnerFor(child, workspaceRoot, maxSteps, mode)
		},
		SubagentConfig: tools.SubagentToolConfig{
			GeneralMaxSteps: sub.GeneralMaxSteps,
		},
	})
}

// SkillsForWorkspace discovers Agent Skills visible to one workspace.
func (r *Runtime) SkillsForWorkspace(workspacePath string) skills.Registry {
	return skills.Discover(workspacePath, r.Config.SkillSettings())
}
