package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Cometline/cometline/comet-sdk/provider/xai"
)

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
