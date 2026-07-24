package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cometline/cometmind/internal/media"
	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/session"
)

// PresentImage registers a local image into the session media store and shows it
// in the chat UI (and Discord when the gateway is active).
type PresentImage struct {
	Workspace Workspace
	Media     session.AssistantMediaAppender
}

func (PresentImage) Spec() ToolSpec {
	return ToolSpec{
		Name: "present_image",
		Description: "Show an image or screenshot file to the user in the chat transcript. " +
			"Pass a workspace-relative path, @runtime/tmp/... / @runtime/tool-output/... / @runtime/wiki/... path, " +
			"or an absolute path under the workspace or ~/.cometmind. " +
			"The image is copied into the session media store and rendered inline; do not paste raw base64. " +
			"To capture the live screen, use capture_screenshot instead.",
		Parameters: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Path to a png/jpeg/gif/webp image"},"alt":{"type":"string","description":"Short accessible caption"}},"required":["path"]}`),
	}
}

func (p PresentImage) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Path *string `json:"path"`
		Alt  *string `json:"alt"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	path, bad, ok := requiredTrimmedString(in.Path, "path")
	if !ok {
		return bad, nil
	}
	alt := ""
	if in.Alt != nil {
		alt = strings.TrimSpace(*in.Alt)
	}

	sessionID := ToolSessionFrom(ctx)
	if sessionID == "" {
		return Result{OK: false, Output: "present_image requires an active session"}, nil
	}
	if p.Media == nil {
		return Result{OK: false, Output: "present_image is not configured"}, nil
	}

	abs, err := resolvePresentableImagePath(p.Workspace, path)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	ref, err := media.RegisterFile(sessionID, abs, alt)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	res, err := presentRegisteredMedia(ctx, p.Media, sessionID, ref, "presented")
	if err != nil {
		return Result{}, err
	}
	if !res.OK {
		return res, nil
	}
	res.Output = fmt.Sprintf("%s path=%s", res.Output, path)
	return res, nil
}

func resolvePresentableImagePath(ws Workspace, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	if abs, err := ws.ResolveReadable(path); err == nil {
		return abs, nil
	}

	if filepath.IsAbs(path) {
		clean := filepath.Clean(path)
		if allowedPresentableAbs(ws.Root, clean) {
			if _, err := os.Stat(clean); err != nil {
				return "", err
			}
			return clean, nil
		}
		return "", fmt.Errorf("absolute path is outside the workspace and ~/.cometmind")
	}

	abs, err := ws.ResolveReadable(path)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func allowedPresentableAbs(workspaceRoot, abs string) bool {
	abs = filepath.Clean(abs)
	if underDir(abs, workspaceRoot) {
		return true
	}
	dataDir, err := paths.DataDir()
	if err != nil {
		return false
	}
	return underDir(abs, dataDir)
}

func underDir(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
