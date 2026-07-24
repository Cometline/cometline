// Package media stores assistant-presented image blobs under ~/.cometmind/media.
package media

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/id"
	"github.com/cometline/cometmind/internal/paths"
)

const maxImageBytes = 4 << 20 // 4 MiB, matches user ImageAttachment limit

// Ref is a persisted media reference (message metadata; bytes live on disk).
type Ref struct {
	ID        string
	MediaType string
	Alt       string
	Path      string // absolute filesystem path
}

var mediaTypeExt = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

var extMediaType = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// DetectMediaType maps a file extension or sniff of common image magic bytes.
func DetectMediaType(path string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if mt, ok := extMediaType[ext]; ok {
		return mt, nil
	}
	if len(data) >= 8 {
		switch {
		case data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4e && data[3] == 0x47:
			return "image/png", nil
		case data[0] == 0xff && data[1] == 0xd8:
			return "image/jpeg", nil
		case data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46:
			return "image/gif", nil
		case len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
			return "image/webp", nil
		}
	}
	return "", fmt.Errorf("unsupported image type (use png, jpeg, gif, or webp)")
}

// RegisterBytes writes image bytes into the session media directory.
func RegisterBytes(sessionID, mediaType, alt string, data []byte) (Ref, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Ref{}, fmt.Errorf("session id is required")
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	ext, ok := mediaTypeExt[mediaType]
	if !ok {
		return Ref{}, fmt.Errorf("unsupported media type %q", mediaType)
	}
	if len(data) == 0 {
		return Ref{}, fmt.Errorf("image is empty")
	}
	if len(data) > maxImageBytes {
		return Ref{}, fmt.Errorf("image is larger than %d MB", maxImageBytes/(1<<20))
	}
	dir, err := paths.SessionMediaDir(sessionID)
	if err != nil {
		return Ref{}, err
	}
	imageID := strings.ToLower(id.New())
	path := filepath.Join(dir, imageID+ext)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return Ref{}, err
	}
	return Ref{
		ID:        imageID,
		MediaType: mediaType,
		Alt:       strings.TrimSpace(alt),
		Path:      path,
	}, nil
}

// RegisterFile reads a local image file and registers it for the session.
func RegisterFile(sessionID, absPath, alt string) (Ref, error) {
	absPath = filepath.Clean(strings.TrimSpace(absPath))
	if absPath == "" {
		return Ref{}, fmt.Errorf("path is required")
	}
	data, err := os.ReadFile(absPath)
	if err != nil {
		return Ref{}, err
	}
	mediaType, err := DetectMediaType(absPath, data)
	if err != nil {
		return Ref{}, err
	}
	return RegisterBytes(sessionID, mediaType, alt, data)
}

// Read loads a registered image by session and image id.
func Read(sessionID, imageID string) (mediaType string, data []byte, err error) {
	sessionID = strings.TrimSpace(sessionID)
	imageID = strings.TrimSpace(imageID)
	if sessionID == "" || imageID == "" {
		return "", nil, fmt.Errorf("session id and image id are required")
	}
	if strings.Contains(imageID, "/") || strings.Contains(imageID, "\\") || strings.Contains(imageID, "..") {
		return "", nil, fmt.Errorf("invalid image id")
	}
	dir, err := paths.SessionMediaDir(sessionID)
	if err != nil {
		return "", nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, err
	}
	prefix := imageID + "."
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name != imageID && !strings.HasPrefix(name, prefix) {
			continue
		}
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		mt, err := DetectMediaType(path, data)
		if err != nil {
			return "", nil, err
		}
		return mt, data, nil
	}
	return "", nil, fmt.Errorf("image not found")
}

// AbsolutePath returns the on-disk path for a registered image, if present.
func AbsolutePath(sessionID, imageID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	imageID = strings.TrimSpace(imageID)
	if sessionID == "" || imageID == "" {
		return "", fmt.Errorf("session id and image id are required")
	}
	dir, err := paths.SessionMediaDir(sessionID)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	prefix := imageID + "."
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == imageID || strings.HasPrefix(name, prefix) {
			return filepath.Join(dir, name), nil
		}
	}
	return "", fmt.Errorf("image not found")
}

// DataURL builds a data: URL for an in-memory image payload.
func DataURL(mediaType string, data []byte) string {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// DeleteSession removes all media files for a session.
func DeleteSession(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	root, err := paths.MediaDir()
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(root, sessionID))
}
