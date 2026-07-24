package tools_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/tools"
)

func TestCaptureScreenshotRegistersAndEmits(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())

	jpeg := []byte{0xff, 0xd8, 0xff, 0xd9}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/capture" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Cometline-Screen-Capture-Token") != "tok" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"media_type": "image/jpeg",
			"data":       base64.StdEncoding.EncodeToString(jpeg),
			"width":      10,
			"height":     10,
		})
	}))
	t.Cleanup(srv.Close)

	stub := &mediaAppenderStub{}
	var emitted []event.Event
	ctx := tools.WithToolSession(context.Background(), "sess-capture")
	ctx = tools.WithProgress(ctx, func(ev event.Event) {
		emitted = append(emitted, ev)
	})

	tool := tools.CaptureScreenshot{
		Endpoint: srv.URL + "/capture",
		Token:    "tok",
		Media:    stub,
		Client:   srv.Client(),
	}
	res, err := tool.Execute(ctx, json.RawMessage(`{"alt":"desktop"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not ok: %s", res.Output)
	}
	if len(stub.last) != 1 || stub.last[0].MediaType != "image/jpeg" {
		t.Fatalf("AppendAssistantMedia blocks = %#v", stub.last)
	}
	if len(emitted) != 1 || emitted[0].Kind != event.KindAssistantImage {
		t.Fatalf("emitted = %#v", emitted)
	}
	if !strings.Contains(res.Output, "captured image") {
		t.Fatalf("output = %q", res.Output)
	}
}

func TestCaptureScreenshotRequiresDesktopBridge(t *testing.T) {
	tool := tools.CaptureScreenshot{Media: &mediaAppenderStub{}}
	ctx := tools.WithToolSession(context.Background(), "sess-capture")
	res, err := tool.Execute(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.OK {
		t.Fatal("expected failure without bridge")
	}
	if !strings.Contains(res.Output, "desktop app") {
		t.Fatalf("output = %q", res.Output)
	}
}

func TestListCaptureTargets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/sources" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-Cometline-Screen-Capture-Token") != "tok" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"sources": []map[string]string{
				{"id": "screen:0", "name": "Built-in Retina", "type": "screen", "display_id": "1"},
				{"id": "window:22", "name": "Cursor", "type": "window", "display_id": ""},
			},
		})
	}))
	t.Cleanup(srv.Close)

	tool := tools.ListCaptureTargets{
		Endpoint: srv.URL + "/capture",
		Token:    "tok",
		Client:   srv.Client(),
	}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not ok: %s", res.Output)
	}
	if !strings.Contains(res.Output, "Cursor") || !strings.Contains(res.Output, "window:22") {
		t.Fatalf("output = %q", res.Output)
	}
}
