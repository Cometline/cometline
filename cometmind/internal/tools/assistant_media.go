package tools

import (
	"context"
	"fmt"
	"os"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/media"
	"github.com/cometline/cometmind/internal/session"
)

func presentRegisteredMedia(
	ctx context.Context,
	appender session.AssistantMediaAppender,
	sessionID string,
	ref media.Ref,
	verb string,
) (Result, error) {
	if appender == nil {
		return Result{OK: false, Output: verb + " is not configured"}, nil
	}
	if _, err := appender.AppendAssistantMedia(ctx, sessionID, []session.ContentBlock{{
		Type:      "image",
		ID:        ref.ID,
		MediaType: ref.MediaType,
		Alt:       ref.Alt,
	}}); err != nil {
		return Result{OK: false, Output: fmt.Sprintf("failed to persist image: %v", err)}, nil
	}

	data, err := os.ReadFile(ref.Path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if progress := ProgressFrom(ctx); progress != nil {
		progress(event.AssistantImage(ref.ID, ref.MediaType, ref.Alt, media.DataURL(ref.MediaType, data)))
	}

	out := fmt.Sprintf("%s image id=%s media_type=%s", verb, ref.ID, ref.MediaType)
	if ref.Alt != "" {
		out += " alt=" + ref.Alt
	}
	return Result{OK: true, Output: out}, nil
}
