package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/runtime"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/spf13/cobra"
)

var chatSessionID string
var chatModelID string
var chatProviderID string

var chatCmd = &cobra.Command{
	Use:   "chat [message...]",
	Short: "Send one user turn through the persisted agent loop",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runChat,
}

func init() {
	chatCmd.Flags().StringVar(&chatSessionID, "session", "", "Resume an existing session id instead of creating a new one")
	chatCmd.Flags().StringVar(&chatModelID, "model", "", "Override model for this turn only")
	chatCmd.Flags().StringVar(&chatProviderID, "provider", "", "Override provider for this turn only")
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

	ws, err := rt.WorkspaceForCommand(ctx, WorkspaceFlag())
	if err != nil {
		return err
	}

	sess, err := chatSession(ctx, rt, ws)
	if err != nil {
		return err
	}

	if _, err := rt.Sessions.AppendUserMessageAndMaybeTitle(ctx, sess.ID, userText); err != nil {
		return err
	}

	runSess, err := chatRunSession(sess)
	if err != nil {
		return err
	}

	runner, err := rt.RunnerFor(runSess, ws.Path)
	if err != nil {
		return err
	}

	evCh := make(chan event.Event, 64)
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Run(ctx, session.AgentTurnFromSession(runSess), evCh)
		close(evCh)
	}()

	for ev := range evCh {
		printChatEvent(ev)
	}

	if err := <-errCh; err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "session=%s workspace=%s\n", sess.ID, ws.Path)
	return nil
}

// chatSession resumes --session (which must belong to ws) or creates a new
// session with the configured default model.
func chatSession(ctx context.Context, rt *runtime.Runtime, ws session.Workspace) (session.Session, error) {
	if chatSessionID == "" {
		sess, err := rt.Sessions.NewSession(ctx, ws.ID, rt.Config.DefaultModelID, rt.Config.DefaultProviderID)
		if err != nil {
			return session.Session{}, fmt.Errorf("create session: %w", err)
		}
		return sess, nil
	}
	sess, err := rt.Sessions.GetSession(ctx, chatSessionID)
	if err != nil {
		return session.Session{}, fmt.Errorf("load session: %w", err)
	}
	if sess.WorkspaceID != ws.ID {
		return session.Session{}, fmt.Errorf("session %s belongs to a different workspace", chatSessionID)
	}
	return sess, nil
}

// chatRunSession applies the per-turn --model/--provider override without
// persisting it on the session.
func chatRunSession(sess session.Session) (session.Session, error) {
	modelID := strings.TrimSpace(chatModelID)
	providerID := strings.TrimSpace(chatProviderID)
	if modelID == "" && providerID == "" {
		return sess, nil
	}
	if modelID == "" || providerID == "" {
		return session.Session{}, fmt.Errorf("--model and --provider must both be provided together")
	}
	sess.ModelID = modelID
	sess.ProviderID = providerID
	return sess, nil
}

func printChatEvent(ev event.Event) {
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
