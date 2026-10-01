package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	"github.com/Cometline/cometline/cometmind/internal/logging"
	"github.com/Cometline/cometline/cometmind/internal/processctl"
	"github.com/Cometline/cometline/cometmind/internal/runstate"
	"github.com/Cometline/cometline/cometmind/internal/runtime"
	"github.com/Cometline/cometline/cometmind/internal/server"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	servePort        int
	serveWatchParent bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the local HTTP + SSE server",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 7700, "Port to bind on 127.0.0.1")
	serveCmd.Flags().BoolVar(&serveWatchParent, "watch-parent", false, "Shut down automatically when the launching parent process exits (for sidecar use)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(_ *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	hupCh := make(chan os.Signal, 1)
	signal.Notify(hupCh, syscall.SIGHUP)
	defer signal.Stop(hupCh)

	if serveWatchParent {
		watchCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		watchParent(watchCtx, cancel)
		ctx = watchCtx
	}

	rt, err := runtime.New(ctx)
	if err != nil {
		return err
	}
	defer rt.Close()
	if err := processctl.WriteMetadata(processctl.ModeServe); err != nil {
		return err
	}
	defer processctl.RemoveMetadata(processctl.ModeServe)
	go handleReloadSignal(ctx, hupCh, processctl.ModeServe, func(reloadCtx context.Context) error {
		return rt.Reload(reloadCtx)
	})

	cleanupStartupSessions(ctx, rt)

	runState := runstate.New(rt.DB)
	runs := server.NewRunManager(runState)
	sessionEvents := event.NewSessionHub()
	engine, err := newServeEngine(ctx, rt, runs, sessionEvents)
	if err != nil {
		return err
	}

	rt.SetSessionRunningChecker(runs.Running)
	startServeWorkers(ctx, rt, runs, sessionEvents)

	return listenAndServe(ctx, engine)
}

// cleanupStartupSessions prunes sessions and workspaces that should not show
// up in the initial session list.
func cleanupStartupSessions(ctx context.Context, rt *runtime.Runtime) {
	// Remove unused New Chat rows before serving requests so the initial session
	// list cannot include a conversation that was never started.
	if pruned, err := rt.Sessions.PruneUnusedUserSessions(ctx); err != nil {
		logging.L().Warn("session.unused_prune_failed", zap.Error(err))
	} else if pruned > 0 {
		logging.L().Info("session.unused_pruned", zap.Int("count", pruned))
	}
	if discarded, err := rt.Sessions.DiscardFinishedEphemeralSessions(ctx, nil); err != nil {
		logging.L().Warn("session.ephemeral_discard_failed", zap.Error(err))
	} else if discarded > 0 {
		logging.L().Info("session.ephemeral_discarded", zap.Int("count", discarded))
	}

	// Prune workspaces whose filesystem path no longer exists. This stats every
	// workspace path (slow on network mounts), so run it in the background to
	// keep it off the startup critical path.
	go func() {
		if pruned, err := rt.Sessions.PruneMissingWorkspaces(ctx); err != nil {
			logging.L().Warn("workspace.prune_failed", zap.Error(err))
		} else if pruned > 0 {
			logging.L().Info("workspace.pruned", zap.Int("count", pruned))
		}
	}()
}

func newServeEngine(ctx context.Context, rt *runtime.Runtime, runs *server.RunManager, sessionEvents *event.SessionHub) (http.Handler, error) {
	engine, err := server.New(server.Deps{
		Config:    rt.Config,
		Sessions:  rt.Sessions,
		Memory:    rt.Memory,
		Events:    rt.Events,
		Jobs:      rt.Jobs,
		Inbox:     rt.Inbox,
		Usage:     rt.Usage,
		Scheduler: rt.Scheduler,
		RunRetention: func(ctx context.Context) (server.RetentionResult, error) {
			return rt.RunRetention(ctx)
		},
		RunBackup: func(ctx context.Context) (server.BackupResult, error) {
			return rt.RunBackup(ctx)
		},
		SetJobSettings: func(s jobs.Settings) {
			rt.SetJobSettings(s)
		},
		Runs:          runs,
		SessionEvents: sessionEvents,
		RunContext:    ctx,
		ACPMgr:        rt.ACPManager(),
		MCPMgr:        rt.MCPManager(),
		SubagentOrch:  rt.SubagentOrchestrator(),
		NewRunner: func(sess session.Session, workspacePath string, mode session.AgentMode) (server.Runner, error) {
			return rt.RunnerForMode(sess, workspacePath, mode)
		},
	})
	if err != nil {
		return nil, err
	}
	return engine, nil
}

func startServeWorkers(ctx context.Context, rt *runtime.Runtime, runs *server.RunManager, sessionEvents *event.SessionHub) {
	rt.StartJobsMaintenance(ctx)
	rt.StartRetentionMaintenance(ctx)
	rt.StartBackupMaintenance(ctx)
	rt.StartScheduler(ctx)
	rt.StartAutonomousJobWorker(ctx, runs, func(sessionID string, running bool) {
		runID, _, ok := runs.Current(context.Background(), sessionID)
		if running && ok {
			sessionEvents.Start(sessionID, runID)
			rt.Events.Publish(event.RunStarted(sessionID))
			return
		}
	}, func(sessionID string, ev event.Event) {
		runID, _, ok := runs.Current(context.Background(), sessionID)
		if ok {
			sessionEvents.Publish(sessionID, runID, ev)
		}
	})
	rt.StartInboxWorker(ctx, runs)
}

func listenAndServe(ctx context.Context, handler http.Handler) error {
	httpServer := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", servePort),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
