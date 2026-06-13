package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/cometline/cometmind/internal/agent"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/cometline/cometmind/internal/tools"
	"github.com/cometline/cometmind/server"
	"github.com/spf13/cobra"
)

var servePort int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the local HTTP + SSE server",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 7700, "Port to bind on 127.0.0.1")
	rootCmd.AddCommand(serveCmd)
}

func runServe(_ *cobra.Command, _ []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	dbpath, err := paths.DBPath()
	if err != nil {
		return err
	}
	sqlDB, err := store.OpenSQLite(ctx, dbpath)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	svc := session.New(sqlDB)
	engine, err := server.New(server.Deps{
		Config:   cfg,
		Sessions: svc,
		NewRunner: func(sess db.Session, workspacePath string) (server.Runner, error) {
			runCfg := *cfg
			runCfg.Model = sess.ModelID
			runCfg.Provider = sess.ProviderID

			p, err := provider.New(&runCfg)
			if err != nil {
				return nil, err
			}

			return &agent.Runner{
				Provider:      p,
				Sessions:      svc,
				Registry:      tools.NewRegistry(workspacePath),
				WorkspaceRoot: workspacePath,
				MaxSteps:      runCfg.MaxSteps,
				MaxTokens:     runCfg.MaxTokens,
			}, nil
		},
	})
	if err != nil {
		return err
	}

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
