package tools

import (
	"context"
	"fmt"

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
	kind := ref.Kind
	if kind == "" {
		kind = media.KindImage
	}
	if _, err := persistPresentedMedia(ctx, appender, sessionID, ref, kind); err != nil {
		_ = media.DeleteFile(sessionID, ref.ID)
		return Result{OK: false, Output: fmt.Sprintf("failed to persist %s: %v", kind, err)}, nil
	}

	emitGeneratedMedia(ctx, ref)

	out := fmt.Sprintf("%s %s id=%s media_type=%s", verb, kind, ref.ID, ref.MediaType)
	if ref.Alt != "" {
		out += " alt=" + ref.Alt
	}
	return Result{OK: true, Output: out}, nil
}

func persistPresentedMedia(
	ctx context.Context,
	appender session.AssistantMediaAppender,
	sessionID string,
	ref media.Ref,
	kind string,
) (session.Message, error) {
	block := session.ContentBlock{
		Type:      kind,
		ID:        ref.ID,
		MediaType: ref.MediaType,
		Alt:       ref.Alt,
	}
	if withMeta, ok := appender.(session.AssistantMediaMetaAppender); ok {
		return withMeta.AppendAssistantMediaWithMeta(ctx, sessionID, []session.ContentBlock{block}, session.MediaMeta{
			Source:   "presented",
			ByteSize: ref.ByteSize,
		})
	}
	return appender.AppendAssistantMedia(ctx, sessionID, []session.ContentBlock{block})
}
