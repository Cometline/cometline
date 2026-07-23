package agent

import (
	"encoding/base64"
	"fmt"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
)

// DowngradeImagesForNonVision rewrites ImageBlocks to text meta when the catalog
// says the model cannot view images. Transcript persistence is unchanged; this
// only mutates the outgoing LLM request copy.
func DowngradeImagesForNonVision(msgs []cometsdk.Message, visionKnown, vision bool) []cometsdk.Message {
	if !visionKnown || vision {
		return msgs
	}
	out := make([]cometsdk.Message, len(msgs))
	copy(out, msgs)
	for i := range out {
		if !messageHasImage(out[i]) {
			continue
		}
		content := make([]cometsdk.Block, 0, len(out[i].Content))
		for _, block := range out[i].Content {
			img, ok := block.(cometsdk.ImageBlock)
			if !ok {
				content = append(content, block)
				continue
			}
			content = append(content, cometsdk.TextBlock{Text: imageMetaPlaceholder(img)})
		}
		out[i].Content = content
	}
	return out
}

func messageHasImage(msg cometsdk.Message) bool {
	for _, block := range msg.Content {
		if _, ok := block.(cometsdk.ImageBlock); ok {
			return true
		}
	}
	return false
}

func imageMetaPlaceholder(img cometsdk.ImageBlock) string {
	mediaType := strings.TrimSpace(img.MediaType)
	if mediaType == "" {
		mediaType = "image"
	}
	approxBytes := approxDecodedBase64Bytes(img.Data)
	if approxBytes <= 0 {
		return fmt.Sprintf("[Attached image omitted: %s — this model cannot view images]", mediaType)
	}
	return fmt.Sprintf(
		"[Attached image omitted: %s ≈ %s — this model cannot view images]",
		mediaType,
		formatByteSize(approxBytes),
	)
}

func approxDecodedBase64Bytes(data string) int {
	data = strings.TrimSpace(data)
	if data == "" {
		return 0
	}
	if decoded, err := base64.StdEncoding.DecodeString(data); err == nil {
		return len(decoded)
	}
	return base64.StdEncoding.DecodedLen(len(data))
}

func formatByteSize(n int) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%d KB", (n+512)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
