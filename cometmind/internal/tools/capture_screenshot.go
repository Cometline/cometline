package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/media"
	"github.com/cometline/cometmind/internal/session"
)

const (
	captureScreenshotTimeout      = 20 * time.Second
	captureScreenshotMaxBodyBytes = 6 << 20
)

// CaptureScreenshot asks the Cometline desktop bridge to capture a screen or
// window and presents the result in the chat transcript.
type CaptureScreenshot struct {
	Endpoint string
	Token    string
	Media    session.AssistantMediaAppender
	Client   *http.Client
}

func (CaptureScreenshot) Spec() ToolSpec {
	return ToolSpec{
		Name: "capture_screenshot",
		Description: "Capture the user's screen or an open app window via the Cometline desktop app and show it inline in chat. " +
			"For any live screen or app-window screenshot, use this tool; do not create an image with run_command or write_file. " +
			"Use list_capture_targets first when targeting a specific window. " +
			"Pass source_id from that list, or window as a title substring (e.g. \"Chrome\", \"Cursor\"). " +
			"Optional crop_x/crop_y/crop_width/crop_height crop the captured bitmap (pixels). " +
			"Requires the desktop app with Screen & System Audio Recording granted. " +
			"Prefer this over asking the user to save a file and calling present_image.",
		Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{` +
			`"display":{"type":"integer","description":"Zero-based display index when capturing a full screen (default 0)"},` +
			`"source_id":{"type":"string","description":"Exact source id from list_capture_targets"},` +
			`"window":{"type":"string","description":"Window title substring to capture (e.g. Chrome, Cursor)"},` +
			`"max_width":{"type":"integer","description":"Max capture width in pixels (default 1920)"},` +
			`"max_height":{"type":"integer","description":"Max capture height in pixels (default 1920)"},` +
			`"crop_x":{"type":"integer","description":"Optional crop origin x in captured image pixels"},` +
			`"crop_y":{"type":"integer","description":"Optional crop origin y in captured image pixels"},` +
			`"crop_width":{"type":"integer","description":"Optional crop width in pixels"},` +
			`"crop_height":{"type":"integer","description":"Optional crop height in pixels"},` +
			`"alt":{"type":"string","description":"Short accessible caption for the screenshot"}` +
			`}}`),
	}
}

func (c CaptureScreenshot) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Display    *int    `json:"display"`
		SourceID   *string `json:"source_id"`
		Window     *string `json:"window"`
		MaxWidth   *int    `json:"max_width"`
		MaxHeight  *int    `json:"max_height"`
		CropX      *int    `json:"crop_x"`
		CropY      *int    `json:"crop_y"`
		CropWidth  *int    `json:"crop_width"`
		CropHeight *int    `json:"crop_height"`
		Alt        *string `json:"alt"`
	}
	if len(input) > 0 && string(input) != "null" {
		if err := json.Unmarshal(input, &in); err != nil {
			return Result{}, err
		}
	}

	sessionID := ToolSessionFrom(ctx)
	if sessionID == "" {
		return Result{OK: false, Output: "capture_screenshot requires an active session"}, nil
	}
	if c.Media == nil {
		return Result{OK: false, Output: "capture_screenshot is not configured"}, nil
	}
	endpoint := strings.TrimSpace(c.Endpoint)
	if endpoint == "" {
		return Result{OK: false, Output: "capture_screenshot requires the Cometline desktop app (screen capture bridge unavailable)"}, nil
	}

	alt := ""
	if in.Alt != nil {
		alt = strings.TrimSpace(*in.Alt)
	}
	if alt == "" {
		alt = "screenshot"
	}

	payload := map[string]any{}
	if in.Display != nil {
		payload["display"] = *in.Display
	}
	if in.SourceID != nil {
		if id := strings.TrimSpace(*in.SourceID); id != "" {
			payload["sourceId"] = id
		}
	}
	if in.Window != nil {
		if name := strings.TrimSpace(*in.Window); name != "" {
			payload["window"] = name
		}
	}
	if in.MaxWidth != nil {
		payload["maxWidth"] = *in.MaxWidth
	}
	if in.MaxHeight != nil {
		payload["maxHeight"] = *in.MaxHeight
	}
	if in.CropWidth != nil || in.CropHeight != nil || in.CropX != nil || in.CropY != nil {
		crop := map[string]any{}
		if in.CropX != nil {
			crop["x"] = *in.CropX
		}
		if in.CropY != nil {
			crop["y"] = *in.CropY
		}
		if in.CropWidth != nil {
			crop["width"] = *in.CropWidth
		}
		if in.CropHeight != nil {
			crop["height"] = *in.CropHeight
		}
		payload["crop"] = crop
	}

	data, mediaType, meta, err := c.capture(ctx, payload)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	ref, err := media.RegisterBytes(sessionID, mediaType, alt, data)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	res, err := presentRegisteredMedia(ctx, c.Media, sessionID, ref, "captured")
	if err != nil {
		return Result{}, err
	}
	if !res.OK {
		return res, nil
	}
	res.Output = fmt.Sprintf("%s path=desktop-capture source=%s type=%s name=%q", res.Output, meta.SourceID, meta.SourceType, meta.SourceName)
	return res, nil
}

type captureMeta struct {
	SourceID   string
	SourceName string
	SourceType string
}

func (c CaptureScreenshot) capture(ctx context.Context, payload map[string]any) ([]byte, string, captureMeta, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, "", captureMeta{}, fmt.Errorf("encode capture request: %w", err)
	}
	reqCtx, cancel := context.WithTimeout(ctx, captureScreenshotTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, c.Endpoint, strings.NewReader(string(body)))
	if err != nil {
		return nil, "", captureMeta{}, fmt.Errorf("build capture request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(c.Token); token != "" {
		req.Header.Set("X-Cometline-Screen-Capture-Token", token)
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, "", captureMeta{}, fmt.Errorf("screen capture bridge request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, captureScreenshotMaxBodyBytes))
	if err != nil {
		return nil, "", captureMeta{}, fmt.Errorf("read capture response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &errBody)
		if msg := strings.TrimSpace(errBody.Error); msg != "" {
			return nil, "", captureMeta{}, fmt.Errorf("screen capture failed: %s", msg)
		}
		return nil, "", captureMeta{}, fmt.Errorf("screen capture bridge returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out struct {
		MediaType  string `json:"media_type"`
		Data       string `json:"data"`
		SourceID   string `json:"source_id"`
		SourceName string `json:"source_name"`
		SourceType string `json:"source_type"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, "", captureMeta{}, fmt.Errorf("decode capture response: %w", err)
	}
	mediaType := strings.ToLower(strings.TrimSpace(out.MediaType))
	if mediaType == "" {
		mediaType = "image/jpeg"
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(out.Data))
	if err != nil {
		return nil, "", captureMeta{}, fmt.Errorf("decode capture image: %w", err)
	}
	if len(data) == 0 {
		return nil, "", captureMeta{}, fmt.Errorf("screen capture returned empty image")
	}
	return data, mediaType, captureMeta{
		SourceID:   out.SourceID,
		SourceName: out.SourceName,
		SourceType: out.SourceType,
	}, nil
}

func (c CaptureScreenshot) httpClient() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return &http.Client{Timeout: captureScreenshotTimeout}
}
