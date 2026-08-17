package session

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"strings"
	"testing"
)

func TestEstimateImageTokensUsesTilesNotBase64Length(t *testing.T) {
	t.Parallel()
	png := pngIHDR(64, 64)
	got := EstimateImageTokens("image/png", base64.StdEncoding.EncodeToString(png))
	if got != 255 {
		t.Fatalf("64x64 PNG tokens = %d, want 255 (1 tile)", got)
	}

	huge := strings.Repeat("A", 800_000)
	fallback := EstimateImageTokens("image/png", huge)
	if fallback != imageTokenFallback {
		t.Fatalf("undecodable payload tokens = %d, want fallback %d", fallback, imageTokenFallback)
	}
	if fallback >= len(huge)/4 {
		t.Fatalf("undecodable payload must not use chars/4: got %d chars/4=%d", fallback, len(huge)/4)
	}
}

func TestEstimateImageTokensScalesWithDimensions(t *testing.T) {
	t.Parallel()
	png := pngIHDR(1920, 1080)
	got := EstimateImageTokens("image/png", base64.StdEncoding.EncodeToString(png))
	// ceil(1920/512)=4, ceil(1080/512)=3 → 85 + 170*12 = 2125
	if got != 2125 {
		t.Fatalf("1920x1080 PNG tokens = %d, want 2125", got)
	}
}

func TestEstimateImageTokensAcceptsDataURL(t *testing.T) {
	t.Parallel()
	png := pngIHDR(512, 512)
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	got := EstimateImageTokens("image/png", dataURL)
	if got != 255 {
		t.Fatalf("data URL tokens = %d, want 255", got)
	}
}

func TestEstimateImageTokensEmpty(t *testing.T) {
	t.Parallel()
	if got := EstimateImageTokens("image/png", ""); got != 0 {
		t.Fatalf("empty data tokens = %d, want 0", got)
	}
}

func pngIHDR(width, height int) []byte {
	var buf bytes.Buffer
	buf.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], uint32(width))
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(height))
	ihdr[8] = 8
	ihdr[9] = 2
	writePNGChunk(&buf, "IHDR", ihdr)
	writePNGChunk(&buf, "IEND", nil)
	return buf.Bytes()
}

func writePNGChunk(buf *bytes.Buffer, name string, data []byte) {
	var chunk bytes.Buffer
	chunk.WriteString(name)
	chunk.Write(data)
	_ = binary.Write(buf, binary.BigEndian, uint32(len(data)))
	buf.Write(chunk.Bytes())
	_ = binary.Write(buf, binary.BigEndian, crc32.ChecksumIEEE(chunk.Bytes()))
}
