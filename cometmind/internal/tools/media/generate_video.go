package media

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/generation"
	"github.com/Cometline/cometline/cometmind/internal/media"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/tools/toolkit"
)

// GenerateVideo creates a clip from a prompt or a session-local first frame.
type GenerateVideo struct {
	Media     session.ReadyMediaReader
	Appender  session.AssistantMediaAppender
	Resolver  func() generation.Binding
	Generator generation.VideoGenerator
}

func (GenerateVideo) Spec() ToolSpec {
	return ToolSpec{
		Name: "generate_video",
		Description: "Generate a short video from a text prompt using the configured video model " +
			"(xAI Grok Imagine by default) and show it in the chat transcript. " +
			"Optionally pass image_id for a first frame that already exists in this session.",
		Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{` +
			`"prompt":{"type":"string","description":"What should happen in the video"},` +
			`"image_id":{"type":"string","description":"Optional session-local image id to use as the first frame"},` +
			`"aspect_ratio":{"type":"string","description":"Optional aspect ratio such as 16:9 or 9:16"},` +
			`"duration":{"type":"integer","description":"Optional duration in seconds"},` +
			`"alt":{"type":"string","description":"Short accessible caption"}` +
			`},"required":["prompt"]}`),
	}
}

func (g GenerateVideo) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in videoInput
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	prompt, bad, ok := toolkit.RequiredTrimmedString(in.Prompt, "prompt")
	if !ok {
		return bad, nil
	}
	sessionID := toolkit.ToolSessionFrom(ctx)
	if sessionID == "" {
		return Result{OK: false, Output: "generate_video requires an active session"}, nil
	}
	if g.Appender == nil {
		return Result{OK: false, Output: "generate_video is not configured"}, nil
	}
	gen, binding, bad, ok := g.resolveVideoGenerator()
	if !ok {
		return bad, nil
	}
	req, sourceMediaID, bad, ok := g.prepareVideoRequest(ctx, sessionID, prompt, binding, in)
	if !ok {
		return bad, nil
	}

	out, err := gen.GenerateVideo(ctx, req)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	alt := prompt
	if in.Alt != nil && strings.TrimSpace(*in.Alt) != "" {
		alt = strings.TrimSpace(*in.Alt)
	}
	ref, err := media.RegisterBytesLimited(sessionID, out.MediaType, alt, out.Data, media.MaxVideoBytes)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	durationMs := int64(generation.ClampVideoDuration(req.Duration)) * 1000
	return persistGeneratedMedia(ctx, g.Appender, sessionID, ref, session.MediaMeta{
		Source:        "generated",
		Prompt:        prompt,
		Model:         binding.Model,
		ProviderID:    binding.ProviderID,
		SourceMediaID: sourceMediaID,
		ByteSize:      ref.ByteSize,
		DurationMs:    &durationMs,
	}, "generated")
}

func (g GenerateVideo) resolveVideoGenerator() (generation.VideoGenerator, generation.Binding, Result, bool) {
	binding := generation.Binding{}
	if g.Resolver != nil {
		binding = g.Resolver()
	}
	if g.Generator != nil {
		return g.Generator, binding, Result{}, true
	}
	resolved, err := generation.Resolve(generation.KindVideo, binding)
	if err != nil {
		return nil, binding, Result{OK: false, Output: err.Error()}, false
	}
	gen, ok := resolved.(generation.VideoGenerator)
	if !ok {
		return nil, binding, Result{OK: false, Output: "video generation adapter is unavailable"}, false
	}
	return gen, binding, Result{}, true
}

type videoInput struct {
	Prompt      *string `json:"prompt"`
	ImageID     *string `json:"image_id"`
	AspectRatio *string `json:"aspect_ratio"`
	Duration    *int    `json:"duration"`
	Alt         *string `json:"alt"`
}

func (g GenerateVideo) prepareVideoRequest(ctx context.Context, sessionID, prompt string, binding generation.Binding, in videoInput) (generation.VideoRequest, string, Result, bool) {
	req := generation.VideoRequest{
		Prompt:      prompt,
		Model:       binding.Model,
		AspectRatio: optionalTrimmed(in.AspectRatio),
	}
	if in.Duration != nil {
		req.Duration = generation.ClampVideoDuration(*in.Duration)
	}
	sourceMediaID := optionalTrimmed(in.ImageID)
	if sourceMediaID == "" {
		return req, "", Result{}, true
	}
	if g.Media == nil {
		return req, "", Result{OK: false, Output: "image_id is not available in this session"}, false
	}
	frame, err := g.Media.ReadySessionImage(ctx, sessionID, sourceMediaID)
	if err != nil {
		return req, "", Result{OK: false, Output: err.Error()}, false
	}
	req.Image = frame.Data
	req.ImageType = frame.MediaType
	return req, sourceMediaID, Result{}, true
}

func optionalTrimmed(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func emitGeneratedMedia(ctx context.Context, ref media.Ref) {
	progress := toolkit.ProgressFrom(ctx)
	if progress == nil {
		return
	}
	if ref.Kind == media.KindVideo {
		progress(event.AssistantVideo(ref.ID, ref.MediaType, ref.Alt))
		return
	}
	progress(event.AssistantImage(ref.ID, ref.MediaType, ref.Alt, ""))
}

func persistGeneratedMedia(
	ctx context.Context,
	appender session.AssistantMediaAppender,
	sessionID string,
	ref media.Ref,
	meta session.MediaMeta,
	verb string,
) (Result, error) {
	block := session.ContentBlock{
		Type:      ref.Kind,
		ID:        ref.ID,
		MediaType: ref.MediaType,
		Alt:       ref.Alt,
	}
	var persistErr error
	if withMeta, ok := appender.(session.AssistantMediaMetaAppender); ok {
		_, persistErr = withMeta.AppendAssistantMediaWithMeta(ctx, sessionID, []session.ContentBlock{block}, meta)
	} else {
		_, persistErr = appender.AppendAssistantMedia(ctx, sessionID, []session.ContentBlock{block})
	}
	if persistErr != nil {
		_ = media.DeleteFile(sessionID, ref.ID)
		return Result{OK: false, Output: fmt.Sprintf("failed to persist %s: %v", ref.Kind, persistErr)}, nil
	}
	emitGeneratedMedia(ctx, ref)
	out := fmt.Sprintf("%s %s id=%s media_type=%s", verb, ref.Kind, ref.ID, ref.MediaType)
	if ref.Alt != "" {
		out += " alt=" + ref.Alt
	}
	return Result{OK: true, Output: out}, nil
}
