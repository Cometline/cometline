package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cometline/cometmind/internal/generation"
	"github.com/cometline/cometmind/internal/media"
	"github.com/cometline/cometmind/internal/session"
)

// GenerateImage creates a still with the configured image generation model.
type GenerateImage struct {
	Media     session.AssistantMediaAppender
	Resolver  func() generation.Binding
	Generator generation.ImageGenerator
}

func (GenerateImage) Spec() ToolSpec {
	return ToolSpec{
		Name: "generate_image",
		Description: "Generate an image from a text prompt using the configured image model " +
			"(xAI Grok Imagine by default) and show it in the chat transcript. " +
			"Use this instead of describing an image in text when the user wants a picture.",
		Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{` +
			`"prompt":{"type":"string","description":"What the image should depict"},` +
			`"aspect_ratio":{"type":"string","description":"Optional aspect ratio such as 1:1, 16:9, 9:16, 4:3, or 3:4"},` +
			`"alt":{"type":"string","description":"Short accessible caption"}` +
			`},"required":["prompt"]}`),
	}
}

func (g GenerateImage) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Prompt      *string `json:"prompt"`
		AspectRatio *string `json:"aspect_ratio"`
		Alt         *string `json:"alt"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	prompt, bad, ok := requiredTrimmedString(in.Prompt, "prompt")
	if !ok {
		return bad, nil
	}
	sessionID := ToolSessionFrom(ctx)
	if sessionID == "" {
		return Result{OK: false, Output: "generate_image requires an active session"}, nil
	}
	if g.Media == nil {
		return Result{OK: false, Output: "generate_image is not configured"}, nil
	}
	binding := generation.Binding{}
	if g.Resolver != nil {
		binding = g.Resolver()
	}
	gen := g.Generator
	if gen == nil {
		resolved, err := generation.Resolve(generation.KindImage, binding)
		if err != nil {
			return Result{OK: false, Output: err.Error()}, nil
		}
		var ok bool
		gen, ok = resolved.(generation.ImageGenerator)
		if !ok {
			return Result{OK: false, Output: "image generation adapter is unavailable"}, nil
		}
	}
	aspect := ""
	if in.AspectRatio != nil {
		aspect = strings.TrimSpace(*in.AspectRatio)
	}
	alt := prompt
	if in.Alt != nil && strings.TrimSpace(*in.Alt) != "" {
		alt = strings.TrimSpace(*in.Alt)
	}
	out, err := gen.GenerateImage(ctx, generation.ImageRequest{
		Prompt:      prompt,
		Model:       binding.Model,
		AspectRatio: aspect,
	})
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	ref, err := media.RegisterBytesLimited(sessionID, out.MediaType, alt, out.Data, media.MaxGeneratedImageBytes)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return persistGeneratedMedia(ctx, g.Media, sessionID, ref, session.MediaMeta{
		Source:     "generated",
		Prompt:     prompt,
		Model:      binding.Model,
		ProviderID: binding.ProviderID,
		ByteSize:   ref.ByteSize,
	}, "generated")
}
