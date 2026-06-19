package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/runtime"
	"github.com/cometline/cometmind/internal/session"
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

	rt, err := runtime.New(ctx)
	if err != nil {
		return err
	}
	defer rt.Close()

	ws, err := rt.WorkspaceForCommand(ctx, "")
	if err != nil {
		return err
	}

	var sess session.Session
	if chatSessionID != "" {
		sess, err = rt.Sessions.GetSession(ctx, chatSessionID)
		if err != nil {
			return fmt.Errorf("load session: %w", err)
		}
		if sess.WorkspaceID != ws.ID {
			return fmt.Errorf("session %s belongs to a different workspace", chatSessionID)
		}
	} else {
		sess, err = rt.Sessions.NewSession(ctx, ws.ID, rt.Config.Model, rt.Config.Provider)
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
	}

	if _, err := rt.Sessions.AppendUserMessageAndMaybeTitle(ctx, sess.ID, userText); err != nil {
		return err
	}

	runner, err := rt.RunnerFor(sess, ws.Path)
	if err != nil {
		return err
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
			fmt.Fprint(os.Stderr, ev.Text)
		case event.KindTextDelta:
			fmt.Fprint(os.Stdout, ev.Delta)
		case event.KindToolCall:
			fmt.Fprintf(os.Stderr, "\n▶ %s %s\n", ev.Tool, string(ev.Input))
		case event.KindToolResult:
			out := strings.TrimSpace(ev.Output)
			if len(out) > 400 {
				out = out[:400] + "…"
			}
			fmt.Fprintf(os.Stderr, "✓ %s\n%s\n", ev.Tool, out)
		case event.KindStepFinish:
			fmt.Fprintf(os.Stderr, "[tokens in=%d out=%d]\n", ev.Usage.InputTokens, ev.Usage.OutputTokens)
		case event.KindError:
			fmt.Fprintf(os.Stderr, "error: %s (%s)\n", ev.Message, ev.Code)
		case event.KindDone:
			fmt.Fprint(os.Stdout, "\n")
		}
	}

	if err := <-errCh; err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "session=%s workspace=%s\n", sess.ID, ws.Path)
	return nil
}
