package session

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"strings"
)

const (
	imageTokenFallback  = 2048
	imageTileSize       = 512
	imageBaseTokens     = 85
	imageTokensPerTile  = 170
	dataURLBase64Marker = "base64,"
)

// EstimateImageTokens estimates vision tokens for an inline image.
// When PNG/JPEG dimensions can be read, this uses an OpenAI-style tile
// formula (85 + 170 × ceil(w/512) × ceil(h/512)). Invalid or undecodable
// payloads fall back to a flat 2048 — never chars/4 of the base64.
func EstimateImageTokens(mediaType, data string) int {
	data = strings.TrimSpace(data)
	if data == "" {
		return 0
	}
	if w, h, ok := decodeImageDimensions(data); ok {
		return imageTileTokens(w, h)
	}
	return imageTokenFallback
}

func imageTileTokens(width, height int) int {
	if width <= 0 || height <= 0 {
		return imageTokenFallback
	}
	tilesX := (width + imageTileSize - 1) / imageTileSize
	tilesY := (height + imageTileSize - 1) / imageTileSize
	return imageBaseTokens + imageTokensPerTile*tilesX*tilesY
}

func decodeImageDimensions(data string) (width, height int, ok bool) {
	raw, err := decodeImageBytes(data)
	if err != nil || len(raw) == 0 {
		return 0, 0, false
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

func decodeImageBytes(data string) ([]byte, error) {
	payload := data
	if i := strings.Index(strings.ToLower(data), dataURLBase64Marker); i >= 0 {
		payload = data[i+len(dataURLBase64Marker):]
	}
	payload = strings.TrimSpace(payload)
	if decoded, err := base64.StdEncoding.DecodeString(payload); err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(payload)
}

// EstimateContentBlocksTokens estimates tokens for persisted content blocks.
// Text uses chars/4; images use EstimateImageTokens.
func EstimateContentBlocksTokens(blocks []ContentBlock) int {
	total := 0
	for _, block := range blocks {
		if block.Type == "image" {
			total += EstimateImageTokens(block.MediaType, block.Data)
			continue
		}
		if block.Text != "" {
			total += EstimateTokens(block.Text)
		}
	}
	return total
}

func countImageBlocks(blocks []ContentBlock) int {
	n := 0
	for _, block := range blocks {
		if block.Type == "image" {
			n++
		}
	}
	return n
}
