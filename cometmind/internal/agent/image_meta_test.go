package agent

import (
	"encoding/base64"
	"strings"
	"testing"

	cometsdk "github.com/cometline/comet-sdk"
)

func TestDowngradeImagesForNonVision(t *testing.T) {
	png := base64.StdEncoding.EncodeToString([]byte("hello-image"))
	msgs := []cometsdk.Message{{
		Role: cometsdk.RoleUser,
		Content: []cometsdk.Block{
			cometsdk.TextBlock{Text: "describe"},
			cometsdk.ImageBlock{MediaType: "image/png", Data: png},
		},
	}}

	unchanged := DowngradeImagesForNonVision(msgs, false, false)
	if _, ok := unchanged[0].Content[1].(cometsdk.ImageBlock); !ok {
		t.Fatal("unknown vision must keep ImageBlock")
	}
	unchangedVision := DowngradeImagesForNonVision(msgs, true, true)
	if _, ok := unchangedVision[0].Content[1].(cometsdk.ImageBlock); !ok {
		t.Fatal("vision model must keep ImageBlock")
	}

	got := DowngradeImagesForNonVision(msgs, true, false)
	if _, ok := msgs[0].Content[1].(cometsdk.ImageBlock); !ok {
		t.Fatal("original messages must remain unmodified")
	}
	text, ok := got[0].Content[1].(cometsdk.TextBlock)
	if !ok {
		t.Fatalf("expected TextBlock, got %T", got[0].Content[1])
	}
	if !strings.Contains(text.Text, "image/png") || !strings.Contains(text.Text, "cannot view images") {
		t.Fatalf("meta text = %q", text.Text)
	}
	if strings.Contains(text.Text, png) {
		t.Fatal("meta text must not include raw base64")
	}
}
