package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func parseImageResponse(payload []byte) (remoteURL, b64, mediaType string, err error) {
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return "", "", "", fmt.Errorf("parse xai image response: %w", err)
	}
	if data, ok := parsed["data"].([]any); ok && len(data) > 0 {
		if first, ok := data[0].(map[string]any); ok {
			remoteURL = stringField(first, "url")
			b64 = stringField(first, "b64_json")
			mediaType = stringField(first, "media_type")
		}
	}
	if remoteURL == "" {
		remoteURL = stringField(parsed, "url")
	}
	if remoteURL == "" {
		if nested, ok := parsed["image"].(map[string]any); ok {
			remoteURL = stringField(nested, "url")
			if b64 == "" {
				b64 = stringField(nested, "b64_json")
			}
		}
	}
	if remoteURL == "" && b64 == "" {
		return "", "", "", fmt.Errorf("xai image response had no url or base64 data")
	}
	return remoteURL, b64, mediaType, nil
}

func videoURL(payload []byte) (string, error) {
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return "", err
	}
	if url := stringField(parsed, "url"); url != "" {
		return url, nil
	}
	if video, ok := parsed["video"].(map[string]any); ok {
		if url := stringField(video, "url"); url != "" {
			return url, nil
		}
	}
	if data, ok := parsed["data"].([]any); ok && len(data) > 0 {
		if first, ok := data[0].(map[string]any); ok {
			if url := stringField(first, "url"); url != "" {
				return url, nil
			}
		}
	}
	return "", fmt.Errorf("xai video response had no url")
}

func videoStatus(payload []byte) (id, status string) {
	var parsed map[string]any
	if err := json.Unmarshal(payload, &parsed); err != nil {
		return "", ""
	}
	id = firstNonEmpty(stringField(parsed, "request_id"), stringField(parsed, "id"))
	status = strings.ToLower(firstNonEmpty(stringField(parsed, "status"), stringField(parsed, "state")))
	return id, status
}

func isVideoReady(status string) bool {
	switch strings.ToLower(status) {
	case "done", "ready", "succeeded", "success", "completed", "complete":
		return true
	default:
		return false
	}
}

func isVideoFailed(status string) bool {
	switch strings.ToLower(status) {
	case "failed", "error", "cancelled", "canceled":
		return true
	default:
		return false
	}
}

func stringField(obj map[string]any, key string) string {
	value, _ := obj[key].(string)
	return strings.TrimSpace(value)
}

func wrapGenerationErr(kind string, err error) error {
	if err == nil {
		return nil
	}
	if isTransientGenerationErr(err) {
		return fmt.Errorf("xAI %s generation timed out waiting for the first response. The model may still be working; try again in a moment", kind)
	}
	return err
}

func isTransientGenerationErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline exceeded")
}

func truncateErr(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) > 300 {
		return text[:300]
	}
	if text == "" {
		return "empty response"
	}
	return text
}
