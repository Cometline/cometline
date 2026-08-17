package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/generation"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/tools"
)

type imageGenStub struct {
	last generation.ImageRequest
}

func (s *imageGenStub) GenerateImage(_ context.Context, req generation.ImageRequest) (generation.Result, error) {
	s.last = req
	return generation.Result{
		MediaType: "image/png",
		Data:      []byte{0x89, 0x50, 0x4e, 0x47},
		Model:     req.Model,
	}, nil
}

type recordingAppender struct {
	last []session.ContentBlock
}

func (r *recordingAppender) AppendAssistantMedia(_ context.Context, _ string, items []session.ContentBlock) (session.Message, error) {
	r.last = items
	return session.Message{ID: "msg"}, nil
}

func TestGenerateImageRequiresPrompt(t *testing.T) {
	tool := tools.GenerateImage{Media: &recordingAppender{}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatalf("expected prompt error, got %#v", res)
	}
}

func TestGenerateImagePersistsAndEmits(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	appender := &recordingAppender{}
	gen := &imageGenStub{}
	var emitted []event.Event
	ctx := tools.WithToolSession(context.Background(), "sess-gen")
	ctx = tools.WithProgress(ctx, func(ev event.Event) { emitted = append(emitted, ev) })

	tool := tools.GenerateImage{
		Media:     appender,
		Generator: gen,
		Resolver: func() generation.Binding {
			return generation.Binding{ProviderID: "xai", Model: "grok-imagine-image-2.0", Method: "xai"}
		},
	}
	res, err := tool.Execute(ctx, json.RawMessage(`{"prompt":"a lighthouse","aspect_ratio":"16:9"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result = %#v", res)
	}
	if gen.last.Prompt != "a lighthouse" || gen.last.AspectRatio != "16:9" {
		t.Fatalf("request = %#v", gen.last)
	}
	if len(appender.last) != 1 || appender.last[0].Type != "image" || appender.last[0].ID == "" {
		t.Fatalf("persisted = %#v", appender.last)
	}
	if len(emitted) != 1 || emitted[0].Kind != event.KindAssistantImage || emitted[0].DataURL != "" {
		t.Fatalf("emitted = %#v", emitted)
	}
}

type videoGenStub struct {
	last generation.VideoRequest
}

func (s *videoGenStub) GenerateVideo(_ context.Context, req generation.VideoRequest) (generation.Result, error) {
	s.last = req
	return generation.Result{MediaType: "video/mp4", Data: []byte("ftyp"), Model: req.Model}, nil
}

func TestGenerateVideoClampsDurationAndEmitsIDOnly(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	appender := &recordingAppender{}
	gen := &videoGenStub{}
	var emitted []event.Event
	ctx := tools.WithToolSession(context.Background(), "sess-video")
	ctx = tools.WithProgress(ctx, func(ev event.Event) { emitted = append(emitted, ev) })

	tool := tools.GenerateVideo{
		Appender:  appender,
		Generator: gen,
		Resolver: func() generation.Binding {
			return generation.Binding{ProviderID: "xai", Model: "grok-imagine-video-1.5", Method: "xai"}
		},
	}
	res, err := tool.Execute(ctx, json.RawMessage(`{"prompt":"lift off","duration":99}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result = %#v", res)
	}
	if gen.last.Duration != 15 {
		t.Fatalf("duration = %d", gen.last.Duration)
	}
	if len(emitted) != 1 || emitted[0].Kind != event.KindAssistantVideo || emitted[0].DataURL != "" {
		t.Fatalf("emitted = %#v", emitted)
	}
}

func TestGenerateImageRejectsUnsupportedProvider(t *testing.T) {
	tool := tools.GenerateImage{
		Media: &recordingAppender{},
		Resolver: func() generation.Binding {
			return generation.Binding{ProviderID: "openai", Model: "gpt-image-2", Method: "openai"}
		},
	}
	ctx := tools.WithToolSession(context.Background(), "sess-gen")
	res, err := tool.Execute(ctx, json.RawMessage(`{"prompt":"nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatalf("expected unsupported provider, got %#v", res)
	}
}
