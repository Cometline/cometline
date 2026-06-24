package server

import (
	"context"
	"strings"
	"time"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/provider"
	"github.com/cometline/cometmind/internal/session"
)

const titleSystemPrompt = "You generate short, descriptive titles for chat conversations. " +
	"Reply with only the title: 3 to 6 words, no quotes, no trailing punctuation, no markdown."

const maxTitleLen = 80
const titleLLMTimeout = 20 * time.Second

// maybeGenerateTitle sets the session title from its first user message. It runs
// only when the session has no title yet (first turn). It first writes a fast
// plain-text fallback so the sidebar never shows "Untitled", then asks the LLM
// for a concise title asynchronously and overwrites the fallback on success.
//
// LLM generation uses a detached context so a client disconnect during streaming
// does not cancel title generation. The frontend refreshes session metadata
// after each turn, so the sidebar picks up the LLM title once it lands.
func (a *App) maybeGenerateTitle(ctx context.Context, sess session.Session, blocks []session.ContentBlock, displayText string) {
	if strings.TrimSpace(sess.Title) != "" {
		return
	}

	fallback := plainTextTitle(blocks, displayText)
	if err := a.sessions.SetTitleIfEmpty(ctx, sess.ID, fallback); err != nil {
		logging.L().Warn("title.fallback_failed", "session", sess.ID, "error", err)
		return
	}

	text := strings.TrimSpace(session.TitleTextFromContent(blocks, displayText))
	if text == "" {
		// Image-only first turn: nothing useful to summarize.
		return
	}

	sessionID := sess.ID
	sessionCopy := sess
	go a.generateTitleAsync(context.WithoutCancel(ctx), sessionCopy, text, sessionID)
}

func (a *App) generateTitleAsync(ctx context.Context, sess session.Session, message, sessionID string) {
	ctx, cancel := context.WithTimeout(ctx, titleLLMTimeout)
	defer cancel()

	title, err := a.generateTitleLLM(ctx, sess, message)
	if err != nil {
		logging.L().Warn("title.generate_failed", "session", sessionID, "error", err)
		return
	}
	title = sanitizeTitle(title)
	if title == "" {
		return
	}
	if err := a.sessions.UpdateTitle(ctx, sessionID, title); err != nil {
		logging.L().Warn("title.update_failed", "session", sessionID, "error", err)
		return
	}
	logging.L().Info("title.generated", "session", sessionID, "title", title)
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
func plainTextTitle(blocks []session.ContentBlock, displayText string) string {
	title := strings.TrimSpace(session.TitleTextFromContent(blocks, displayText))
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
