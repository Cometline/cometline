package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ListCaptureTargets lists displays and open app windows the desktop bridge can capture.
type ListCaptureTargets struct {
	Endpoint string
	Token    string
	Client   *http.Client
}

func (ListCaptureTargets) Spec() ToolSpec {
	return ToolSpec{
		Name: "list_capture_targets",
		Description: "List screens and open application windows that capture_screenshot can target. " +
			"Call this first when the user wants a specific app window (Chrome, Cursor, Terminal, etc.). " +
			"Returns id/name/type; pass source_id or window title into capture_screenshot.",
		Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
	}
}

func (l ListCaptureTargets) Execute(ctx context.Context, _ json.RawMessage) (Result, error) {
	endpoint := strings.TrimSpace(l.Endpoint)
	if endpoint == "" {
		return Result{OK: false, Output: "list_capture_targets requires the Cometline desktop app (screen capture bridge unavailable)"}, nil
	}
	raw, err := screenCaptureBridgeGET(ctx, l.Client, screenCaptureSourcesURL(endpoint), l.Token, 1<<20)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	var out struct {
		Sources []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Type      string `json:"type"`
			DisplayID string `json:"display_id"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Result{OK: false, Output: "decode capture targets: " + err.Error()}, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "capture targets (%d):\n", len(out.Sources))
	for _, source := range out.Sources {
		fmt.Fprintf(&b, "- type=%s id=%s name=%q", source.Type, source.ID, source.Name)
		if source.DisplayID != "" {
			fmt.Fprintf(&b, " display_id=%s", source.DisplayID)
		}
		b.WriteByte('\n')
	}
	return Result{OK: true, Output: strings.TrimSpace(b.String())}, nil
}
