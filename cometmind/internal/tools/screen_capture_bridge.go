package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func screenCaptureSourcesURL(captureEndpoint string) string {
	endpoint := strings.TrimSpace(captureEndpoint)
	if strings.HasSuffix(endpoint, "/capture") {
		return strings.TrimSuffix(endpoint, "/capture") + "/sources"
	}
	return strings.TrimRight(endpoint, "/") + "/sources"
}

func screenCaptureBridgeGET(ctx context.Context, client *http.Client, endpoint, token string, maxBytes int64) ([]byte, error) {
	reqCtx, cancel := context.WithTimeout(ctx, captureScreenshotTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build screen capture request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if t := strings.TrimSpace(token); t != "" {
		req.Header.Set("X-Cometline-Screen-Capture-Token", t)
	}
	if client == nil {
		client = &http.Client{Timeout: captureScreenshotTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("screen capture bridge request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, fmt.Errorf("read capture response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errBody struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &errBody)
		if msg := strings.TrimSpace(errBody.Error); msg != "" {
			return nil, fmt.Errorf("screen capture failed: %s", msg)
		}
		return nil, fmt.Errorf("screen capture bridge returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return raw, nil
}
