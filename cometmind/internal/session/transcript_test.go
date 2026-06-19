package session

import (
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
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

	raw, err := marshalMessageContent(blocks)
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
