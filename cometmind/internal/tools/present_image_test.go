package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/tools"
)

type mediaAppenderStub struct {
	last []session.ContentBlock
}

func (m *mediaAppenderStub) AppendAssistantMedia(
	_ context.Context,
	_ string,
	images []session.ContentBlock,
) (session.Message, error) {
	m.last = images
	return session.Message{ID: "msg1"}, nil
}

func TestPresentImageRegistersAndEmits(t *testing.T) {
	t.Setenv("COMETMIND_DATA_DIR", t.TempDir())
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "shot.png")
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	if err := os.WriteFile(pngPath, png, 0o600); err != nil {
		t.Fatal(err)
	}

	stub := &mediaAppenderStub{}
	var emitted []event.Event
	ctx := tools.WithToolSession(context.Background(), "sess-present")
	ctx = tools.WithProgress(ctx, func(ev event.Event) {
		emitted = append(emitted, ev)
	})

	tool := tools.PresentImage{
		Workspace: tools.Workspace{Root: dir},
		Media:     stub,
	}
	res, err := tool.Execute(ctx, json.RawMessage(`{"path":"shot.png","alt":"demo"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.OK {
		t.Fatalf("result not ok: %s", res.Output)
	}
	if len(stub.last) != 1 || stub.last[0].ID == "" {
		t.Fatalf("AppendAssistantMedia blocks = %#v", stub.last)
	}
	if len(emitted) != 1 || emitted[0].Kind != event.KindAssistantImage {
		t.Fatalf("emitted = %#v", emitted)
	}
	if !strings.Contains(res.Output, "presented image") {
		t.Fatalf("output = %q", res.Output)
	}
}
