package generation

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Cometline/cometline/comet-sdk/provider/xai"
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
