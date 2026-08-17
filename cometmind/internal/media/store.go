// Package media stores assistant-presented image and video blobs under ~/.cometmind/media.
package media

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/id"
	"github.com/cometline/cometmind/internal/paths"
)

const (
	// MaxPresentedImageBytes matches the user ImageAttachment limit.
	MaxPresentedImageBytes = 4 << 20
	// MaxGeneratedImageBytes is the on-disk cap for model-generated stills.
	MaxGeneratedImageBytes = 20 << 20
	// MaxVideoBytes is the on-disk cap for generated or imported video.
	MaxVideoBytes = 80 << 20

	KindImage = "image"
	KindVideo = "video"
)

// ErrNotFound is returned when a media id has no file in the session directory.
var ErrNotFound = errors.New("media not found")

// Ref is a persisted media reference (message metadata; bytes live on disk).
type Ref struct {
	ID        string
	Kind      string
	MediaType string
	Alt       string
	Path      string // absolute filesystem path
	ByteSize  int64
}

var mediaTypeExt = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
	"video/mp4":  ".mp4",
	"video/webm": ".webm",
}

var extMediaType = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".mp4":  "video/mp4",
	".webm": "video/webm",
}

// KindForMediaType maps a MIME type onto the session_media kind.
func KindForMediaType(mediaType string) (string, error) {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	switch {
	case strings.HasPrefix(mediaType, "image/"):
		return KindImage, nil
	case strings.HasPrefix(mediaType, "video/"):
		return KindVideo, nil
	default:
		return "", fmt.Errorf("unsupported media type %q", mediaType)
	}
}

// DetectMediaType maps a file extension or sniff of common media magic bytes.
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
		case len(data) >= 12 && string(data[4:8]) == "ftyp":
			return "video/mp4", nil
		case len(data) >= 4 && string(data[0:4]) == "\x1aE\xdf\xa3":
			return "video/webm", nil
		}
	}
	return "", fmt.Errorf("unsupported media type (use png, jpeg, gif, webp, mp4, or webm)")
}

// RegisterBytes writes presented image bytes into the session media directory.
func RegisterBytes(sessionID, mediaType, alt string, data []byte) (Ref, error) {
	return RegisterBytesLimited(sessionID, mediaType, alt, data, MaxPresentedImageBytes)
}

// RegisterBytesLimited writes media bytes with an explicit size cap.
func RegisterBytesLimited(sessionID, mediaType, alt string, data []byte, maxBytes int) (Ref, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Ref{}, fmt.Errorf("session id is required")
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	ext, ok := mediaTypeExt[mediaType]
	if !ok {
		return Ref{}, fmt.Errorf("unsupported media type %q", mediaType)
	}
	kind, err := KindForMediaType(mediaType)
	if err != nil {
		return Ref{}, err
	}
	if len(data) == 0 {
		return Ref{}, fmt.Errorf("%s is empty", kind)
	}
	if maxBytes <= 0 {
		maxBytes = MaxPresentedImageBytes
	}
	if len(data) > maxBytes {
		return Ref{}, fmt.Errorf("%s is larger than %d MB", kind, maxBytes/(1<<20))
	}
	dir, err := paths.SessionMediaDir(sessionID)
	if err != nil {
		return Ref{}, err
	}
	mediaID := strings.ToLower(id.New())
	path := filepath.Join(dir, mediaID+ext)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return Ref{}, err
	}
	return Ref{
		ID:        mediaID,
		Kind:      kind,
		MediaType: mediaType,
		Alt:       strings.TrimSpace(alt),
		Path:      path,
		ByteSize:  int64(len(data)),
	}, nil
}

// RegisterFile reads a local media file and registers it for the session.
func RegisterFile(sessionID, absPath, alt string) (Ref, error) {
	return RegisterFileLimited(sessionID, absPath, alt, MaxPresentedImageBytes)
}

// RegisterFileLimited reads a local media file with an explicit size cap.
func RegisterFileLimited(sessionID, absPath, alt string, maxBytes int) (Ref, error) {
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
	return RegisterBytesLimited(sessionID, mediaType, alt, data, maxBytes)
}

// CopyFile duplicates an existing file into a new session media id.
func CopyFile(sessionID, absPath, mediaType, alt string) (Ref, error) {
	absPath = filepath.Clean(strings.TrimSpace(absPath))
	if absPath == "" {
		return Ref{}, fmt.Errorf("path is required")
	}
	info, err := os.Stat(absPath)
	if err != nil {
		return Ref{}, err
	}
	if !info.Mode().IsRegular() {
		return Ref{}, fmt.Errorf("media source is not a file")
	}
	if info.Size() > MaxVideoBytes {
		return Ref{}, fmt.Errorf("media is larger than %d MB", MaxVideoBytes/(1<<20))
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return Ref{}, fmt.Errorf("session id is required")
	}
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	if mediaType == "" {
		data, readErr := os.ReadFile(absPath)
		if readErr != nil {
			return Ref{}, readErr
		}
		detected, detectErr := DetectMediaType(absPath, data)
		if detectErr != nil {
			return Ref{}, detectErr
		}
		mediaType = detected
	}
	ext, ok := mediaTypeExt[mediaType]
	if !ok {
		return Ref{}, fmt.Errorf("unsupported media type %q", mediaType)
	}
	kind, err := KindForMediaType(mediaType)
	if err != nil {
		return Ref{}, err
	}
	dir, err := paths.SessionMediaDir(sessionID)
	if err != nil {
		return Ref{}, err
	}
	src, err := os.Open(absPath)
	if err != nil {
		return Ref{}, err
	}
	defer src.Close()

	mediaID := strings.ToLower(id.New())
	path := filepath.Join(dir, mediaID+ext)
	dst, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return Ref{}, err
	}
	copied, copyErr := io.Copy(dst, src)
	closeErr := dst.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return Ref{}, copyErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return Ref{}, closeErr
	}
	return Ref{
		ID:        mediaID,
		Kind:      kind,
		MediaType: mediaType,
		Alt:       strings.TrimSpace(alt),
		Path:      path,
		ByteSize:  copied,
	}, nil
}

// Read loads a registered media file by session and media id.
func Read(sessionID, mediaID string) (mediaType string, data []byte, err error) {
	file, mediaType, err := Open(sessionID, mediaID)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()
	data, err = io.ReadAll(file)
	if err != nil {
		return "", nil, err
	}
	return mediaType, data, nil
}

// Open returns a readable file handle for a registered media id.
// The caller must close the file.
func Open(sessionID, mediaID string) (*os.File, string, error) {
	path, mediaType, err := locate(sessionID, mediaID)
	if err != nil {
		return nil, "", err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	if mediaType == "" {
		head := make([]byte, 16)
		n, readErr := file.Read(head)
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			_ = file.Close()
			return nil, "", readErr
		}
		if detected, detectErr := DetectMediaType(path, head[:n]); detectErr == nil {
			mediaType = detected
		}
		if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
			_ = file.Close()
			return nil, "", seekErr
		}
	}
	return file, mediaType, nil
}

// AbsolutePath returns the on-disk path for a registered media file, if present.
func AbsolutePath(sessionID, mediaID string) (string, error) {
	path, _, err := locate(sessionID, mediaID)
	return path, err
}

// DeleteFile removes one registered media file. Missing files are not an error.
func DeleteFile(sessionID, mediaID string) error {
	path, _, err := locate(sessionID, mediaID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// DataURL builds a data: URL for an in-memory image payload.
func DataURL(mediaType string, data []byte) string {
	return "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// FileInfo is one on-disk media file for a session.
type FileInfo struct {
	ID        string
	MediaType string
	Path      string
	ByteSize  int64
}

// ListSessionFiles lists registered media files for a session without creating
// the directory when it does not exist.
func ListSessionFiles(sessionID string) ([]FileInfo, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}
	root, err := paths.MediaDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, sessionID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		mediaType, ok := extMediaType[ext]
		if !ok {
			continue
		}
		id := strings.TrimSuffix(name, ext)
		if id == "" || strings.Contains(id, ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		out = append(out, FileInfo{
			ID:        id,
			MediaType: mediaType,
			Path:      filepath.Join(dir, name),
			ByteSize:  info.Size(),
		})
	}
	return out, nil
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

func locate(sessionID, mediaID string) (path string, mediaType string, err error) {
	sessionID = strings.TrimSpace(sessionID)
	mediaID = strings.TrimSpace(mediaID)
	if sessionID == "" || mediaID == "" {
		return "", "", fmt.Errorf("session id and media id are required")
	}
	if strings.Contains(mediaID, "/") || strings.Contains(mediaID, "\\") || strings.Contains(mediaID, "..") {
		return "", "", fmt.Errorf("invalid media id")
	}
	dir, err := paths.SessionMediaDir(sessionID)
	if err != nil {
		return "", "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", ErrNotFound
		}
		return "", "", err
	}
	prefix := mediaID + "."
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name != mediaID && !strings.HasPrefix(name, prefix) {
			continue
		}
		path := filepath.Join(dir, name)
		return path, extMediaType[strings.ToLower(filepath.Ext(name))], nil
	}
	return "", "", ErrNotFound
}
