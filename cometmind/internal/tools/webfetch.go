package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	webFetchTimeout      = 30 * time.Second
	webFetchMaxBodyBytes = 5 << 20 // 5 MiB cap on the downloaded response body
	webFetchDefaultChars = 20000   // default cap on returned text
	webFetchMaxChars     = 100000  // hard ceiling regardless of max_chars
	webFetchUserAgent    = "CometMind/1.0 (+https://github.com/cometline/cometmind)"
)

// WebFetch fetches a web page over http(s) and returns its readable text.
// Requests to loopback, private, and link-local addresses are blocked to avoid
// SSRF against the user's machine and cloud metadata endpoints.
type WebFetch struct{}

type webFetchInput struct {
	URL      *string `json:"url"`
	MaxChars int     `json:"max_chars"`
}

func (WebFetch) Spec() ToolSpec {
	return ToolSpec{
		Name:        "web_fetch",
		Description: "Fetch a public web page over http(s) and return its readable text content. Use this to read documentation, articles, or any URL the user provides. Private/localhost addresses are not reachable.",
		Parameters: json.RawMessage(`{"type":"object","properties":{` +
			`"url":{"type":"string","description":"Absolute http(s) URL to fetch"},` +
			`"max_chars":{"type":"integer","description":"Maximum characters of text to return (default 20000)"}` +
			`},"required":["url"]}`),
	}
}

func (WebFetch) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	in, err := parseWebFetchInput(input)
	if err != nil {
		return Result{}, err
	}

	target, bad, ok := requiredTrimmedString(in.URL, "url")
	if !ok {
		return bad, nil
	}
	parsed, err := url.Parse(target)
	if err != nil {
		return Result{OK: false, Output: "invalid url: " + err.Error()}, nil
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return Result{OK: false, Output: "only http(s) URLs are supported"}, nil
	}
	if err := guardAgainstSSRF(parsed.Hostname()); err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}

	maxChars := in.MaxChars
	if maxChars <= 0 {
		maxChars = webFetchDefaultChars
	}
	if maxChars > webFetchMaxChars {
		maxChars = webFetchMaxChars
	}

	reqCtx, cancel := context.WithTimeout(ctx, webFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return Result{OK: false, Output: "build request: " + err.Error()}, nil
	}
	req.Header.Set("User-Agent", webFetchUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,*/*")

	client := &http.Client{Timeout: webFetchTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return Result{OK: false, Output: "fetch failed: " + err.Error()}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return Result{OK: false, Output: fmt.Sprintf("HTTP %d from %s", resp.StatusCode, target)}, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, webFetchMaxBodyBytes))
	if err != nil {
		return Result{OK: false, Output: "read body: " + err.Error()}, nil
	}

	contentType := resp.Header.Get("Content-Type")
	var text string
	if isHTMLContentType(contentType) {
		text = htmlToText(string(body))
	} else {
		text = string(body)
	}
	text = strings.TrimSpace(text)

	truncated := false
	if len([]rune(text)) > maxChars {
		text = string([]rune(text)[:maxChars])
		truncated = true
	}

	out := fmt.Sprintf("URL: %s\n\n%s", target, text)
	if truncated {
		out += "\n\n[truncated]"
	}
	return Result{OK: true, Output: out}, nil
}

func parseWebFetchInput(input json.RawMessage) (webFetchInput, error) {
	var in webFetchInput
	if err := json.Unmarshal(input, &in); err == nil {
		return in, nil
	} else {
		var raw string
		if stringErr := json.Unmarshal(input, &raw); stringErr == nil {
			raw, _, ok := requiredTrimmedString(&raw, "url")
			if !ok {
				return webFetchInput{}, nil
			}
			if strings.HasPrefix(raw, "{") {
				var nested webFetchInput
				if nestedErr := json.Unmarshal([]byte(raw), &nested); nestedErr == nil {
					return nested, nil
				}
			}
			return webFetchInput{URL: &raw}, nil
		}
		return webFetchInput{}, err
	}
}

func isHTMLContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.Contains(ct, "text/html") || strings.Contains(ct, "application/xhtml")
}

// htmlToText extracts visible text from an HTML document, skipping script,
// style, and head noise, and collapsing runs of whitespace.
func htmlToText(htmlStr string) string {
	node, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "script", "style", "head", "noscript", "svg":
				return
			}
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteString(" ")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode {
			switch strings.ToLower(n.Data) {
			case "p", "div", "br", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6":
				b.WriteString("\n")
			}
		}
	}
	walk(node)
	return collapseWhitespace(b.String())
}

// collapseWhitespace trims each line and removes excessive blank lines.
func collapseWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		trimmed := strings.Join(strings.Fields(line), " ")
		if trimmed == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, trimmed)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// guardAgainstSSRF rejects hostnames that resolve to loopback, private, or
// link-local addresses (cloud metadata, the local cometmind server, etc.).
func guardAgainstSSRF(host string) error {
	if host == "" {
		return fmt.Errorf("missing host")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" {
		return fmt.Errorf("refusing to fetch a local address: %s", host)
	}

	// If the host is a literal IP, check it directly; otherwise resolve it.
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		resolved, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("could not resolve host: %s", host)
		}
		ips = resolved
	}
	for _, ip := range ips {
		if isBlockedIP(ip) {
			return fmt.Errorf("refusing to fetch a private or local address: %s", host)
		}
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}
	// Block the IPv4 cloud-metadata address explicitly (covered by link-local,
	// but make the intent clear).
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 169 && ip4[1] == 254 {
		return true
	}
	return false
}
