package session

import (
	"encoding/json"
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/db"
)

func TestTrimTranscriptToolOutput(t *testing.T) {
	t.Parallel()
	short := "ok"
	if trimTranscriptToolOutput(short) != short {
		t.Fatalf("short string trimmed unexpectedly")
	}
	long := strings.Repeat("x", 500)
	got := trimTranscriptToolOutput(long)
	want := strings.Repeat("x", 400) + "…"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMessageContentBlocksRoundTrip(t *testing.T) {
	blocks := []ContentBlock{
		{Type: "text", Text: "describe this"},
		{Type: "image", MediaType: "image/png", Data: "aGVsbG8="},
	}

	raw, err := marshalMessageContent(blocks, "")
	if err != nil {
		t.Fatalf("marshalMessageContent() error = %v", err)
	}
	if !strings.HasPrefix(raw, contentEnvelopePrefix) {
		t.Fatalf("multimodal content missing envelope prefix: %q", raw)
	}

	decoded, err := DecodeMessageContent(raw)
	if err != nil {
		t.Fatalf("DecodeMessageContent() error = %v", err)
	}
	if len(decoded) != 2 || decoded[1].MediaType != "image/png" || decoded[1].Data != "aGVsbG8=" {
		t.Fatalf("decoded = %#v", decoded)
	}

	sdkBlocks := sdkBlocksFromContent(decoded)
	if _, ok := sdkBlocks[0].(cometsdk.TextBlock); !ok {
		t.Fatalf("first SDK block = %T, want TextBlock", sdkBlocks[0])
	}
	img, ok := sdkBlocks[1].(cometsdk.ImageBlock)
	if !ok {
		t.Fatalf("second SDK block = %T, want ImageBlock", sdkBlocks[1])
	}
	if img.MediaType != "image/png" || img.Data != "aGVsbG8=" {
		t.Fatalf("image block = %#v", img)
	}
}

func TestDecodeMessageContentPlainText(t *testing.T) {
	decoded, err := DecodeMessageContent("hello")
	if err != nil {
		t.Fatalf("DecodeMessageContent() error = %v", err)
	}
	if len(decoded) != 1 || decoded[0].Type != "text" || decoded[0].Text != "hello" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestDisplayTextFromStoredContent(t *testing.T) {
	raw, err := marshalMessageContent([]ContentBlock{{Type: "text", Text: "agent prompt"}}, "/job Fix login")
	if err != nil {
		t.Fatalf("marshalMessageContent() error = %v", err)
	}
	if got := DisplayTextFromStoredContent(raw); got != "/job Fix login" {
		t.Fatalf("DisplayTextFromStoredContent() = %q", got)
	}
	if got := PlainTextFromContent([]ContentBlock{{Type: "text", Text: "agent prompt"}}); got != "agent prompt" {
		t.Fatalf("PlainTextFromContent() = %q", got)
	}
}

func TestErrorMessageContentRoundTrip(t *testing.T) {
	raw, err := marshalErrorMessageContent("provider failed")
	if err != nil {
		t.Fatalf("marshalErrorMessageContent() error = %v", err)
	}
	if !strings.HasPrefix(raw, errorMessagePrefix) {
		t.Fatalf("error content missing envelope prefix: %q", raw)
	}
	got, ok := DecodeErrorMessageContent(raw)
	if !ok {
		t.Fatalf("DecodeErrorMessageContent() did not recognize envelope")
	}
	if got != "provider failed" {
		t.Fatalf("DecodeErrorMessageContent() = %q", got)
	}
}

func TestMarshalReasoningContentEmptyIsArray(t *testing.T) {
	t.Parallel()
	got, err := marshalReasoningContent(nil)
	if err != nil {
		t.Fatalf("marshalReasoningContent(nil) error = %v", err)
	}
	if got != "[]" {
		t.Fatalf("marshalReasoningContent(nil) = %q, want []", got)
	}
	got, err = marshalReasoningContent([]cometsdk.Block{})
	if err != nil {
		t.Fatalf("marshalReasoningContent(empty) error = %v", err)
	}
	if got != "[]" {
		t.Fatalf("marshalReasoningContent(empty) = %q, want []", got)
	}
}

func TestUnmarshalReasoningContentToleratesNull(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "null", "[]"} {
		blocks, err := unmarshalReasoningContent(raw)
		if err != nil {
			t.Fatalf("unmarshalReasoningContent(%q) error = %v", raw, err)
		}
		if len(blocks) != 0 {
			t.Fatalf("unmarshalReasoningContent(%q) len = %d, want 0", raw, len(blocks))
		}
	}
}

func TestAssistantBlocksDropsToolCallsWithoutResults(t *testing.T) {
	completedPayload, err := json.Marshal(toolResultPayload{ToolCallID: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := completedToolCallIDs([]db.Message{{ID: "result", Role: "tool_result", Content: string(completedPayload)}})
	if err != nil {
		t.Fatal(err)
	}

	blocks := assistantBlocks(db.Message{}, []db.ToolCall{
		{ID: "orphaned", ToolName: "list_dir", Arguments: `{}`},
		{ID: "completed", ToolName: "read_file", Arguments: `{"path":"README.md"}`},
	}, completed)
	if len(blocks) != 1 {
		t.Fatalf("blocks = %#v, want only completed tool call", blocks)
	}
	call, ok := blocks[0].(cometsdk.ToolCallBlock)
	if !ok || call.ID != "completed" {
		t.Fatalf("block = %#v, want completed tool call", blocks[0])
	}
}
