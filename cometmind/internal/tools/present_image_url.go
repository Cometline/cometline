package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/media"
	"github.com/cometline/cometmind/internal/session"
)

const (
	presentImageURLTimeout  = 30 * time.Second
	presentImageURLMaxBytes = 4 << 20
)

// PresentImageURL downloads a public raster image directly into session media.
type PresentImageURL struct {
	Media  session.AssistantMediaAppender
	Client *http.Client
}

func (PresentImageURL) Spec() ToolSpec {
	return ToolSpec{
		Name: "present_image_url",
		Description: "Download a public png, jpeg, gif, or webp image URL and show it inline in chat. " +
			"Use this for a web image instead of run_command, write_file, @runtime paths, or present_image. " +
			"The image is stored directly in session media and never written to the workspace.",
		Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{` +
			`"url":{"type":"string","description":"Absolute public http(s) URL for a png, jpeg, gif, or webp image"},` +
			`"alt":{"type":"string","description":"Short accessible caption"}` +
			`},"required":["url"]}`),
	}
}

func (p PresentImageURL) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		URL *string `json:"url"`
		Alt *string `json:"alt"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{}, err
	}
	imageURL, bad, ok := requiredTrimmedString(in.URL, "url")
	if !ok {
		return bad, nil
	}
	sessionID := ToolSessionFrom(ctx)
	if sessionID == "" {
		return Result{OK: false, Output: "present_image_url requires an active session"}, nil
	}
	if p.Media == nil {
		return Result{OK: false, Output: "present_image_url is not configured"}, nil
	}
	alt := ""
	if in.Alt != nil {
		alt = strings.TrimSpace(*in.Alt)
	}

	data, mediaType, err := p.download(ctx, imageURL)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	ref, err := media.RegisterBytes(sessionID, mediaType, alt, data)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	res, err := presentRegisteredMedia(ctx, p.Media, sessionID, ref, "downloaded")
	if err != nil {
		return Result{}, err
	}
	if res.OK {
		res.Output += " url=" + imageURL
	}
	return res, nil
}

func (p PresentImageURL) download(ctx context.Context, rawURL string) ([]byte, string, error) {
	target, err := validatePublicImageURL(rawURL)
	if err != nil {
		return nil, "", err
	}
	reqCtx, cancel := context.WithTimeout(ctx, presentImageURLTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, "", fmt.Errorf("build image request: %w", err)
	}
	req.Header.Set("Accept", "image/png,image/jpeg,image/gif,image/webp")

	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("download image: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("image URL returned HTTP %d", resp.StatusCode)
	}
	declared, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(strings.ToLower(declared), "image/") {
		return nil, "", fmt.Errorf("image URL did not return an image content type")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, presentImageURLMaxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("read image response: %w", err)
	}
	if len(data) > presentImageURLMaxBytes {
		return nil, "", fmt.Errorf("image is larger than %d MB", presentImageURLMaxBytes/(1<<20))
	}
	mediaType, err := media.DetectMediaType("", data)
	if err != nil {
		return nil, "", fmt.Errorf("image URL returned unsupported image data: %w", err)
	}
	return data, mediaType, nil
}

func (p PresentImageURL) httpClient() *http.Client {
	client := &http.Client{Timeout: presentImageURLTimeout}
	if p.Client != nil {
		clone := *p.Client
		client = &clone
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		_, err := validatePublicImageURL(req.URL.String())
		return err
	}
	return client
}

func validatePublicImageURL(rawURL string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("invalid image URL: %w", err)
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("only http(s) image URLs are supported")
	}
	if err := guardAgainstSSRF(target.Hostname()); err != nil {
		return nil, err
	}
	return target, nil
}
