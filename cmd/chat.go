package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cometline/cometmind/internal/agent"
	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/store"
	"github.com/cometline/cometmind/internal/tools"
	"github.com/spf13/cobra"
)

var chatSessionID string

var chatCmd = &cobra.Command{
	Use:   "chat [message...]",
	Short: "Send one user turn through the persisted agent loop",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runChat,
}

func init() {
	chatCmd.Flags().StringVar(&chatSessionID, "session", "", "Resume an existing session id instead of creating a new one")
	rootCmd.AddCommand(chatCmd)
}

func runChat(_ *cobra.Command, args []string) error {
	ctx := context.Background()
	userText := strings.TrimSpace(strings.Join(args, " "))
	if userText == "" {
		return fmt.Errorf("message is empty")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	root, err := WorkspaceRoot()
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
	ws, err := svc.EnsureWorkspace(ctx, root)
	if err != nil {
		return err
	}

	var sess db.Session
	if chatSessionID != "" {
		sess, err = svc.GetSession(ctx, chatSessionID)
		if err != nil {
			return fmt.Errorf("load session: %w", err)
		}
		if sess.WorkspaceID != ws.ID {
			return fmt.Errorf("session %s belongs to a different workspace", chatSessionID)
		}
		// Use persisted model/provider identifiers for resumed sessions.
		cfg.Model = sess.ModelID
		cfg.Provider = sess.ProviderID
	} else {
		sess, err = svc.NewSession(ctx, ws.ID, cfg.Model, cfg.Provider)
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
	}

	wsPath, err := svc.WorkspacePath(ctx, sess.WorkspaceID)
	if err != nil {
		return err
	}

	if _, err := svc.AppendUserMessage(ctx, sess.ID, userText); err != nil {
		return err
	}
	title := userText
	if len(title) > 80 {
		title = title[:80] + "…"
	}
	if err := svc.SetTitleIfEmpty(ctx, sess.ID, title); err != nil {
		return err
	}

	p, err := provider.New(cfg)
	if err != nil {
		return err
	}

	reg := tools.NewRegistry(wsPath)
	runner := agent.Runner{
		Provider:      p,
		Sessions:      svc,
		Registry:      reg,
		WorkspaceRoot: wsPath,
		MaxSteps:      cfg.MaxSteps,
		MaxTokens:     cfg.MaxTokens,
	}

	evCh := make(chan event.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Run(ctx, session.AgentTurnFromSession(sess), evCh)
		close(evCh)
	}()

	for ev := range evCh {
		switch ev.Kind {
		case event.KindReasoningStart:
		case event.KindReasoningDelta:
			if ev.ReasoningDelta != nil {
				fmt.Fprint(os.Stderr, ev.ReasoningDelta.Text)
			}
		case event.KindTextDelta:
			if ev.TextDelta != nil {
				fmt.Fprint(os.Stdout, ev.TextDelta.Delta)
			}
		case event.KindToolCall:
			if ev.ToolCall != nil {
				fmt.Fprintf(os.Stderr, "\n▶ %s %s\n", ev.ToolCall.Tool, string(ev.ToolCall.Input))
			}
		case event.KindToolResult:
			if ev.ToolOut != nil {
				out := strings.TrimSpace(ev.ToolOut.Output)
				if len(out) > 400 {
					out = out[:400] + "…"
				}
				fmt.Fprintf(os.Stderr, "✓ %s\n%s\n", ev.ToolOut.Tool, out)
			}
		case event.KindStepFinish:
			if ev.Step != nil {
				u := ev.Step.Usage
				fmt.Fprintf(os.Stderr, "[tokens in=%d out=%d]\n", u.InputTokens, u.OutputTokens)
			}
		case event.KindError:
			if ev.Err != nil {
				fmt.Fprintf(os.Stderr, "error: %s (%s)\n", ev.Err.Message, ev.Err.Code)
			}
		case event.KindDone:
			fmt.Fprint(os.Stdout, "\n")
		}
	}

	if err := <-errCh; err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "session=%s workspace=%s\n", sess.ID, wsPath)
	return nil
}
