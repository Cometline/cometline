package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/runtime"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/server"
	"github.com/spf13/cobra"
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
	if err := writeProcessMetadata(processModeServe); err != nil {
		return err
	}
	defer removeProcessMetadata(processModeServe)
	go handleReloadSignal(ctx, hupCh, func(reloadCtx context.Context) error {
		return rt.Reload(reloadCtx)
	})

	// Prune workspaces whose filesystem path no longer exists. This stats every
	// workspace path (slow on network mounts), so run it in the background to
	// keep it off the startup critical path.
	go func() {
		if pruned, err := rt.Sessions.PruneMissingWorkspaces(ctx); err != nil {
			logging.L().Warn("workspace.prune_failed", "error", err)
		} else if pruned > 0 {
			logging.L().Info("workspace.pruned", "count", pruned)
		}
	}()

	runs := server.NewRunManager()
	engine, err := server.New(server.Deps{
		Config:   rt.Config,
		Sessions: rt.Sessions,
		Memory:   rt.Memory,
		Jobs:     rt.Jobs,
		RunRetention: func(ctx context.Context) (server.RetentionResult, error) {
			return rt.RunRetention(ctx)
		},
		SetJobSettings: func(s jobs.Settings) {
			rt.SetJobSettings(s)
		},
		Runs:         runs,
		ACPMgr:       rt.ACPManager(),
		MCPMgr:       rt.MCPManager(),
		SubagentOrch: rt.SubagentOrchestrator(),
		NewRunner: func(sess session.Session, workspacePath string) (server.Runner, error) {
			return rt.RunnerFor(sess, workspacePath)
		},
	})
	if err != nil {
		return err
	}

	rt.SetSessionRunningChecker(runs.Running)
	rt.StartJobsMaintenance(ctx)
	rt.StartRetentionMaintenance(ctx)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", servePort),
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
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
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}
