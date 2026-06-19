package server

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/tools/sandbox"
)

type workspaceFileTextContent struct {
	Kind      string `json:"kind"`
	Content   string `json:"content"`
	Extension string `json:"extension"`
}

type workspaceFileImageContent struct {
	Kind     string `json:"kind"`
	MimeType string `json:"mime_type"`
	DataURL  string `json:"data_url"`
}

var workspaceFileImageMIME = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

func readWorkspaceFilePreview(workspacePath, relativePath string) (any, error) {
	rel := strings.TrimSpace(relativePath)
	if rel == "" {
		return nil, fmt.Errorf("path is required")
	}

	abs, err := sandbox.ResolveWorkspacePath(workspacePath, rel)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("file not found")
	}
	if info.IsDir() {
		return nil, fmt.Errorf("not a file")
	}
	if info.Size() > maxMessageFileBytes {
		return nil, fmt.Errorf("file exceeds %d KB preview limit", maxMessageFileBytes/1024)
	}

	ext := strings.ToLower(filepath.Ext(abs))
	if mimeType, ok := workspaceFileImageMIME[ext]; ok {
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, fmt.Errorf("cannot read file")
		}
		return workspaceFileImageContent{
			Kind:     "image",
			MimeType: mimeType,
			DataURL:  fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)),
		}, nil
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("cannot read file as text")
	}
	if containsBinaryContent(data) {
		return nil, fmt.Errorf("binary file cannot be previewed")
	}

	return workspaceFileTextContent{
		Kind:      "text",
		Content:   string(data),
		Extension: ext,
	}, nil
}

func writeWorkspaceFileContent(workspacePath, relativePath, content string) error {
	rel := strings.TrimSpace(relativePath)
	if rel == "" {
		return fmt.Errorf("path is required")
	}

	abs, err := sandbox.ResolveWorkspacePath(workspacePath, rel)
	if err != nil {
		return err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return fmt.Errorf("file not found")
	}
	if info.IsDir() {
		return fmt.Errorf("not a file")
	}

	if _, ok := workspaceFileImageMIME[strings.ToLower(filepath.Ext(abs))]; ok {
		return fmt.Errorf("image files cannot be edited")
	}

	data := []byte(content)
	if len(data) > maxMessageFileBytes {
		return fmt.Errorf("file exceeds %d KB preview limit", maxMessageFileBytes/1024)
	}
	if containsBinaryContent(data) {
		return fmt.Errorf("binary file cannot be previewed")
	}

	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	if err := os.WriteFile(abs, data, mode); err != nil {
		return fmt.Errorf("cannot write file")
	}

	return nil
}

func containsBinaryContent(data []byte) bool {
	return strings.Contains(string(data), "\x00")
}
