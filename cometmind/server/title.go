package server

import (
	"context"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/cometline/cometmind/internal/session"
)

const titleSystemPrompt = "You generate short, descriptive titles for chat conversations. " +
	"Reply with only the title: 3 to 6 words, no quotes, no trailing punctuation, no markdown."

const maxTitleLen = 80

// maybeGenerateTitle sets the session title from its first user message. It runs
// only when the session has no title yet (first turn). It first writes a fast
// plain-text fallback so the sidebar never shows "Untitled", then asks the LLM
// for a concise title and overwrites the fallback on success.
//
// The call is synchronous but bounded (MaxTokens: 32) so the generated title is
// ready before the turn's SSE stream completes and the client refreshes the
// session. LLM failures are non-fatal: the plain-text fallback remains.
func (a *App) maybeGenerateTitle(ctx context.Context, sess session.Session, blocks []session.ContentBlock) {
	if strings.TrimSpace(sess.Title) != "" {
		return
	}

	fallback := plainTextTitle(blocks)
	if err := a.sessions.SetTitleIfEmpty(ctx, sess.ID, fallback); err != nil {
		logging.L().Warn("title.fallback_failed", "session", sess.ID, "error", err)
		return
	}

	text := strings.TrimSpace(session.PlainTextFromContent(blocks))
	if text == "" {
		// Image-only first turn: nothing useful to summarize.
		return
	}

	title, err := a.generateTitleLLM(ctx, sess, text)
	if err != nil {
		logging.L().Warn("title.generate_failed", "session", sess.ID, "error", err)
		return
	}
	title = sanitizeTitle(title)
	if title == "" {
		return
	}
	if err := a.sessions.UpdateTitle(ctx, sess.ID, title); err != nil {
		logging.L().Warn("title.update_failed", "session", sess.ID, "error", err)
		return
	}
	logging.L().Info("title.generated", "session", sess.ID, "title", title)
}

// generateTitleLLM asks an LLM for a concise title for the message. It uses the
// configured title provider/model when set (typically a cheaper, faster model),
// falling back to the session's own provider/model otherwise.
func (a *App) generateTitleLLM(ctx context.Context, sess session.Session, message string) (string, error) {
	providerID := strings.TrimSpace(a.config.TitleProvider)
	if providerID == "" {
		providerID = sess.ProviderID
	}
	model := strings.TrimSpace(a.config.TitleModel)
	if model == "" {
		model = sess.ModelID
	}

	p, err := provider.NewFor(a.config, providerID)
	if err != nil {
		return "", err
	}

	if len(message) > 2000 {
		message = message[:2000]
	}

	req := &cometsdk.Request{
		Model:     model,
		MaxTokens: 32,
		System:    titleSystemPrompt,
		Messages: []cometsdk.Message{{
			Role: cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{
				Text: "Write a title for a conversation that starts with this message:\n\n" + message,
			}},
		}},
	}

	result, err := llm.GenerateText(ctx, p, req)
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// plainTextTitle is the provisional first-turn title derived from the message.
func plainTextTitle(blocks []session.ContentBlock) string {
	title := strings.TrimSpace(session.PlainTextFromContent(blocks))
	if title == "" {
		return "Image"
	}
	return truncateTitle(title)
}

// sanitizeTitle strips wrapping quotes/whitespace and enforces the length cap.
func sanitizeTitle(title string) string {
	title = strings.TrimSpace(title)
	// Models sometimes wrap titles in quotes despite instructions.
	title = strings.Trim(title, "\"'`")
	title = strings.TrimSpace(title)
	// Collapse to the first line in case the model adds explanation.
	if idx := strings.IndexAny(title, "\r\n"); idx >= 0 {
		title = strings.TrimSpace(title[:idx])
	}
	return truncateTitle(title)
}

func truncateTitle(title string) string {
	r := []rune(title)
	if len(r) > maxTitleLen {
		return string(r[:maxTitleLen]) + "…"
	}
	return title
}
