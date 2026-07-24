package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadWorkspaceFilePreviewImageSizeLimit(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	smallPath := filepath.Join(workspacePath, "small.png")
	largePath := filepath.Join(workspacePath, "large.png")

	// Minimal valid-enough PNG payload for preview encoding (content is opaque).
	small := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if err := os.WriteFile(smallPath, small, 0o644); err != nil {
		t.Fatalf("write small png: %v", err)
	}
	large := make([]byte, maxWorkspaceImagePreviewBytes+1)
	copy(large, small)
	if err := os.WriteFile(largePath, large, 0o644); err != nil {
		t.Fatalf("write large png: %v", err)
	}

	got, err := readWorkspaceFilePreview(workspacePath, "small.png")
	if err != nil {
		t.Fatalf("small image preview: %v", err)
	}
	img, ok := got.(workspaceFileImageContent)
	if !ok || img.Kind != "image" || !strings.HasPrefix(img.DataURL, "data:image/png;base64,") {
		t.Fatalf("unexpected small image result: %#v", got)
	}

	_, err = readWorkspaceFilePreview(workspacePath, "large.png")
	if err == nil {
		t.Fatal("expected large image preview to fail")
	}
	if !strings.Contains(err.Error(), "preview limit") {
		t.Fatalf("error = %v, want preview limit", err)
	}
}

func TestReadWorkspaceFilePreviewAllowsImageAboveTextLimit(t *testing.T) {
	t.Parallel()

	workspacePath := t.TempDir()
	path := filepath.Join(workspacePath, "mid.png")
	// Between text (256 KiB) and image (8 MiB) limits.
	data := make([]byte, maxMessageFileBytes+1024)
	data[0], data[1], data[2], data[3] = 0x89, 0x50, 0x4e, 0x47
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write mid png: %v", err)
	}

	got, err := readWorkspaceFilePreview(workspacePath, "mid.png")
	if err != nil {
		t.Fatalf("mid-size image should use image limit: %v", err)
	}
	if _, ok := got.(workspaceFileImageContent); !ok {
		t.Fatalf("got = %#v, want image content", got)
	}
}
