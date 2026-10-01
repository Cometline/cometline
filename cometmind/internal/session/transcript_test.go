package session

import (
	"encoding/json"
	"strings"
	"testing"

	cometsdk "github.com/Cometline/cometline/comet-sdk"
	"github.com/Cometline/cometline/cometmind/internal/db"
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

	raw, err := marshalMessageContent(blocks, "", nil)
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
	raw, err := marshalMessageContent([]ContentBlock{{Type: "text", Text: "agent prompt"}}, "/job Fix login", nil)
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

func TestContextsFromStoredContent(t *testing.T) {
	t.Parallel()
	contexts := []MessageContextRef{
		{Kind: "file", Title: "notes.md", Source: "workspace-file:notes.md", Role: "viewing"},
		{Kind: "file", Title: "notes.md:2-3", Source: "workspace-file:notes.md#L2-L3"},
		{Kind: "page", Title: "Example", Source: "https://example.com"},
	}
	raw, err := marshalMessageContent(
		[]ContentBlock{{Type: "text", Text: "agent prompt with contexts"}},
		"look here",
		contexts,
	)
	if err != nil {
		t.Fatalf("marshalMessageContent() error = %v", err)
	}
	if got := DisplayTextFromStoredContent(raw); got != "look here" {
		t.Fatalf("DisplayTextFromStoredContent() = %q", got)
	}
	got := ContextsFromStoredContent(raw)
	if len(got) != 3 {
		t.Fatalf("ContextsFromStoredContent() len = %d, want 3", len(got))
	}
	if got[0].Role != "viewing" || got[0].Source != "workspace-file:notes.md" {
		t.Fatalf("first context = %#v", got[0])
	}
	if got[1].Source != "workspace-file:notes.md#L2-L3" {
		t.Fatalf("second context = %#v", got[1])
	}
	if got[2].Kind != "page" || got[2].Source != "https://example.com" {
		t.Fatalf("third context = %#v", got[2])
	}
	if ContextsFromStoredContent("plain text") != nil {
		t.Fatalf("plain text should have no contexts")
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

func TestMediaBlocksKeepsImagesAndVideosInOrder(t *testing.T) {
	got := mediaBlocks([]ContentBlock{
		{Type: "text", Text: "hi"},
		{Type: "video", ID: "v1"},
		{Type: "image", ID: "i1"},
	})
	if len(got) != 2 || got[0].ID != "v1" || got[1].ID != "i1" {
		t.Fatalf("mediaBlocks = %#v, want [v1 i1]", got)
	}
	if got := mediaBlocks([]ContentBlock{{Type: "text", Text: "hi"}}); got != nil {
		t.Fatalf("mediaBlocks(text only) = %#v, want nil", got)
	}
}

func TestEnvelopeMediaBlocks(t *testing.T) {
	raw, err := marshalMessageContent([]ContentBlock{
		{Type: "text", Text: "look"},
		{Type: "image", ID: "img-1", MediaType: "image/png"},
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := envelopeMediaBlocks(raw)
	if len(got) != 1 || got[0].ID != "img-1" {
		t.Fatalf("envelopeMediaBlocks = %#v, want img-1", got)
	}
	for _, raw := range []string{"plain text", contentEnvelopePrefix + "{not json"} {
		if got := envelopeMediaBlocks(raw); got != nil {
			t.Fatalf("envelopeMediaBlocks(%q) = %#v, want nil", raw, got)
		}
	}
}

func TestToolResultErrorFlagsSkipsUndecodableRows(t *testing.T) {
	failed, err := json.Marshal(toolResultPayload{ToolCallID: "failed", IsError: true})
	if err != nil {
		t.Fatal(err)
	}
	flags := toolResultErrorFlags([]db.Message{
		{Role: "tool_result", Content: string(failed)},
		{Role: "tool_result", Content: "{bad"},
		{Role: "assistant", Content: string(failed)},
	})
	if len(flags) != 1 || !flags["failed"] {
		t.Fatalf("flags = %#v, want only failed=true", flags)
	}
}

func TestSystemTranscriptEntry(t *testing.T) {
	raw, err := marshalErrorMessageContent("boom")
	if err != nil {
		t.Fatal(err)
	}
	if got := systemTranscriptEntry(db.Message{Content: raw}); got.Kind != TranscriptKindError || got.Text != "boom" {
		t.Fatalf("error entry = %#v", got)
	}
	if got := systemTranscriptEntry(db.Message{Content: "  note \n"}); got.Kind != TranscriptKindSystem || got.Text != "note" {
		t.Fatalf("system entry = %#v", got)
	}
}

func TestAssistantTextEntry(t *testing.T) {
	if _, ok := assistantTextEntry(db.Message{Content: "  "}); ok {
		t.Fatal("blank assistant content should produce no entry")
	}
	entry, ok := assistantTextEntry(db.Message{Content: " plain "})
	if !ok || entry.Text != "plain" || entry.Images != nil {
		t.Fatalf("plain entry = %#v, ok=%v", entry, ok)
	}
	raw, err := marshalMessageContent([]ContentBlock{{Type: "image", ID: "img-1", MediaType: "image/png"}}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	entry, ok = assistantTextEntry(db.Message{Content: raw})
	if !ok || entry.Text != "" || len(entry.Images) != 1 {
		t.Fatalf("media entry = %#v, ok=%v", entry, ok)
	}
}
