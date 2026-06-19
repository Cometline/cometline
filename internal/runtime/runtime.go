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

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/agent"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/cometline/cometmind/internal/retention"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/skills"
	"github.com/cometline/cometmind/internal/store"
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
	SystemPrompt string
	acpMgr       *acp.SessionManager
	memorySem    chan struct{} // bounds concurrent memory-extraction goroutines
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
	}
	if cfg.MemoryRuntimeEnabled() {
		p, err := provider.New(cfg)
		if err == nil {
			mem, err := memory.NewService(sqlDB, cfg.MemorySettings(), p, sessions)
			if err == nil {
				r.Memory = mem
			}
		}
	}
	runRetention(ctx, sqlDB, sessions, r.Memory, cfg.EffectiveStorageConfig())
	return r, nil
}

func runRetention(ctx context.Context, db *sql.DB, sessions *session.Service, mem *memory.Service, cfg config.StorageConfig) {
	if !cfg.RetentionEnabled() && !cfg.MemoryPurgeEnabled() {
		return
	}
	rr := &retention.Runner{
		DB:       db,
		Sessions: sessions,
		Memory:   mem,
		Config:   cfg,
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

// Close releases runtime resources.
func (r *Runtime) Close() error {
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

// ACPManager returns the shared interactive ACP session manager.
func (r *Runtime) ACPManager() *acp.SessionManager {
	if r.acpMgr == nil {
		r.acpMgr = acp.NewSessionManager(r.Config.ACPSettings())
	}
	return r.acpMgr
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
		Registry:     tools.NewRegistry(workspacePath, tools.RegistryOptions{Sessions: r.Sessions, ACP: r.Config.ACPSettings(), ACPMgr: r.ACPManager(), Skills: &skillRegistry}),
		MaxSteps:     r.Config.MaxSteps,
		MaxTokens:    r.Config.MaxTokens,
		SystemPrompt: r.SystemPrompt,
		SkillIndex:   skillRegistry.PromptIndex(),
		MemorySem:    r.memorySem,
	}, nil
}

// SkillsForWorkspace discovers Agent Skills visible to one workspace.
func (r *Runtime) SkillsForWorkspace(workspacePath string) skills.Registry {
	return skills.Discover(workspacePath, r.Config.SkillSettings())
}
