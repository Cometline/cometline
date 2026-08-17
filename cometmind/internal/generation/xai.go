package generation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cometline/comet-sdk/provider/xai"
)

const (
	xaiBaseURL           = "https://api.x.ai/v1"
	xaiPollInterval      = 2 * time.Second
	xaiPollTimeout       = 8 * time.Minute
	xaiHeaderTimeout     = 4 * time.Minute
	xaiRequestTimeout    = 6 * time.Minute
	xaiDownloadTimeout   = 2 * time.Minute
	xaiTransientAttempts = 2
	defaultVideoDur      = 10
	minVideoDur          = 1
	maxVideoDur          = 15
	defaultVideoRes      = "720p"
	defaultVideoAR       = "16:9"
)

// XAIClient calls xAI Imagine endpoints with the local Grok OAuth session.
type XAIClient struct {
	HTTP     *http.Client
	BaseURL  string
	Borrow   func(context.Context, *http.Client) (string, error)
	Now      func() time.Time
	Sleep    func(time.Duration)
	MaxBytes int64
}

// NewXAIClient returns a client that borrows the local Grok subscription token.
func NewXAIClient(httpClient *http.Client) *XAIClient {
	return &XAIClient{
		HTTP:     generationHTTPClient(httpClient),
		BaseURL:  xaiBaseURL,
		Borrow:   xai.BorrowToken,
		Now:      time.Now,
		Sleep:    time.Sleep,
		MaxBytes: 80 << 20,
	}
}

func generationHTTPClient(base *http.Client) *http.Client {
	if base != nil {
		cloned := *base
		if cloned.Timeout == 0 {
			cloned.Timeout = xaiRequestTimeout
		}
		if transport, ok := cloned.Transport.(*http.Transport); ok {
			clonedTransport := transport.Clone()
			if clonedTransport.ResponseHeaderTimeout == 0 {
				clonedTransport.ResponseHeaderTimeout = xaiHeaderTimeout
			}
			cloned.Transport = clonedTransport
		}
		return &cloned
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = xaiHeaderTimeout
	return &http.Client{
		Timeout:   xaiRequestTimeout,
		Transport: transport,
	}
}

func (c *XAIClient) GenerateImage(ctx context.Context, req ImageRequest) (Result, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = DefaultImageModel
	}
	body := map[string]any{
		"model":  model,
		"prompt": strings.TrimSpace(req.Prompt),
	}
	if ar := strings.TrimSpace(req.AspectRatio); ar != "" {
		body["aspect_ratio"] = ar
	}
	payload, status, err := c.postJSON(ctx, "/images/generations", body)
	if err != nil {
		return Result{}, wrapGenerationErr("image", err)
	}
	if status < 200 || status >= 300 {
		return Result{}, fmt.Errorf("xai image generation failed (%d): %s", status, truncateErr(payload))
	}
	url, b64, mediaType, err := parseImageResponse(payload)
	if err != nil {
		return Result{}, err
	}
	data, err := c.materialize(ctx, url, b64)
	if err != nil {
		return Result{}, err
	}
	if mediaType == "" {
		mediaType = "image/png"
	}
	return Result{MediaType: mediaType, Data: data, Model: model}, nil
}

func (c *XAIClient) GenerateVideo(ctx context.Context, req VideoRequest) (Result, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = DefaultVideoModel
	}
	duration := ClampVideoDuration(req.Duration)
	body := map[string]any{
		"model":        model,
		"prompt":       strings.TrimSpace(req.Prompt),
		"duration":     duration,
		"aspect_ratio": firstNonEmpty(req.AspectRatio, defaultVideoAR),
		"resolution":   firstNonEmpty(req.Resolution, defaultVideoRes),
	}
	if len(req.Image) > 0 {
		mediaType := strings.TrimSpace(req.ImageType)
		if mediaType == "" {
			mediaType = "image/png"
		}
		body["image"] = map[string]any{
			"url": "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(req.Image),
		}
	}
	payload, status, err := c.postJSON(ctx, "/videos/generations", body)
	if err != nil {
		return Result{}, wrapGenerationErr("video", err)
	}
	if status < 200 || status >= 300 {
		return Result{}, fmt.Errorf("xai video generation failed (%d): %s", status, truncateErr(payload))
	}
	final, err := c.awaitVideo(ctx, payload)
	if err != nil {
		return Result{}, err
	}
	url, err := videoURL(final)
	if err != nil {
		return Result{}, err
	}
	data, err := c.download(ctx, url)
	if err != nil {
		return Result{}, err
	}
	return Result{MediaType: "video/mp4", Data: data, Model: model}, nil
}

func (c *XAIClient) awaitVideo(ctx context.Context, payload []byte) ([]byte, error) {
	if url, err := videoURL(payload); err == nil && url != "" {
		return payload, nil
	}
	requestID, status := videoStatus(payload)
	if isVideoReady(status) {
		return payload, nil
	}
	if requestID == "" {
		return nil, fmt.Errorf("xai video response did not include a request id or url")
	}
	deadline := c.now().Add(xaiPollTimeout)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if c.now().After(deadline) {
			return nil, fmt.Errorf("xai video generation timed out")
		}
		c.sleep(xaiPollInterval)
		next, code, err := c.getJSON(ctx, "/videos/"+requestID)
		if err != nil {
			return nil, err
		}
		if code < 200 || code >= 300 {
			return nil, fmt.Errorf("xai video poll failed (%d): %s", code, truncateErr(next))
		}
		_, status = videoStatus(next)
		if isVideoFailed(status) {
			return nil, fmt.Errorf("xai video generation failed: %s", truncateErr(next))
		}
		if _, err := videoURL(next); err == nil || isVideoReady(status) {
			return next, nil
		}
	}
}

func (c *XAIClient) materialize(ctx context.Context, remoteURL, b64 string) ([]byte, error) {
	if strings.TrimSpace(b64) != "" {
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("decode generated image: %w", err)
		}
		return data, nil
	}
	if strings.TrimSpace(remoteURL) == "" {
		return nil, fmt.Errorf("xai image response had no url or base64 data")
	}
	if strings.HasPrefix(remoteURL, "data:") {
		_, encoded, ok := strings.Cut(remoteURL, ",")
		if !ok {
			return nil, fmt.Errorf("invalid data url in xai image response")
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode generated image: %w", err)
		}
		return data, nil
	}
	return c.download(ctx, remoteURL)
}

func (c *XAIClient) postJSON(ctx context.Context, path string, body any) ([]byte, int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	var lastBody []byte
	var lastStatus int
	var lastErr error
	for attempt := 1; attempt <= xaiTransientAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(path), bytes.NewReader(raw))
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "cometline")
		lastBody, lastStatus, lastErr = c.do(ctx, req)
		if lastErr == nil || !isTransientGenerationErr(lastErr) || attempt == xaiTransientAttempts {
			return lastBody, lastStatus, lastErr
		}
		c.sleep(time.Duration(attempt) * time.Second)
	}
	return lastBody, lastStatus, lastErr
}

func (c *XAIClient) getJSON(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint(path), nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cometline")
	return c.do(ctx, req)
}

func (c *XAIClient) download(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || !c.allowedDownloadURL(parsed) {
		return nil, fmt.Errorf("refusing to download generated media from untrusted url")
	}
	downloadCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		downloadCtx, cancel = context.WithTimeout(ctx, xaiDownloadTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "cometline")
	if downloadUsesAPIAuth(parsed) {
		if token, err := c.borrow(ctx); err == nil && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	client := c.downloadHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download generated media: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("download generated media failed (%d): %s", resp.StatusCode, truncateErr(body))
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBytes()+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > c.maxBytes() {
		return nil, fmt.Errorf("generated media is larger than %d MB", c.maxBytes()/(1<<20))
	}
	return data, nil
}

func (c *XAIClient) do(ctx context.Context, req *http.Request) ([]byte, int, error) {
	token, err := c.borrow(ctx)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (c *XAIClient) borrow(ctx context.Context) (string, error) {
	fn := c.Borrow
	if fn == nil {
		fn = xai.BorrowToken
	}
	return fn(ctx, c.http())
}

func (c *XAIClient) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *XAIClient) endpoint(path string) string {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		base = xaiBaseURL
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func (c *XAIClient) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *XAIClient) sleep(d time.Duration) {
	if c.Sleep != nil {
		c.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (c *XAIClient) maxBytes() int64 {
	if c.MaxBytes > 0 {
		return c.MaxBytes
	}
	return 80 << 20
}

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

// ClampVideoDuration keeps tool and API durations inside the xAI range.
func ClampVideoDuration(seconds int) int {
	if seconds < minVideoDur {
		return defaultVideoDur
	}
	if seconds > maxVideoDur {
		return maxVideoDur
	}
	return seconds
}

func (c *XAIClient) downloadHTTPClient() *http.Client {
	base := c.http()
	cloned := *base
	cloned.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		if !c.allowedDownloadURL(req.URL) {
			return fmt.Errorf("refusing redirect to untrusted host %s", req.URL.Host)
		}
		if !downloadUsesAPIAuth(req.URL) {
			req.Header.Del("Authorization")
		}
		return nil
	}
	return &cloned
}

func (c *XAIClient) allowedDownloadURL(u *url.URL) bool {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	if c.allowsConfiguredHost(u) {
		return true
	}
	if u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return host == "api.x.ai" || host == "download.x.ai" || strings.HasSuffix(host, ".x.ai")
}

func (c *XAIClient) allowsConfiguredHost(u *url.URL) bool {
	raw := strings.TrimSpace(c.BaseURL)
	if raw == "" {
		return false
	}
	base, err := url.Parse(raw)
	if err != nil || base.Hostname() == "" {
		return false
	}
	return strings.EqualFold(u.Hostname(), base.Hostname())
}

func downloadUsesAPIAuth(u *url.URL) bool {
	return u != nil && strings.EqualFold(u.Hostname(), "api.x.ai")
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
