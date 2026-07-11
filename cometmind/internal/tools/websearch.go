package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	webSearchTimeout        = 25 * time.Second
	webSearchAttemptTimeout = 8 * time.Second
	webSearchMaxQuery       = 500
	webSearchDefaultLimit   = 5
	webSearchMaxLimit       = 10
	webSearchMaxBodyBytes   = 2 << 20
	webSearchMaxOutput      = 30000
	webSearchUserAgent      = "CometMind/1.0 (+https://github.com/cometline/cometmind)"
)

// SearchResult is one normalized public-web search result.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
	Source  string `json:"source,omitempty"`
}

type webSearchInput struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
	Recency string `json:"recency,omitempty"`
}

type webSearchResponse struct {
	Query   string         `json:"query"`
	Backend string         `json:"backend"`
	Results []SearchResult `json:"results"`
}

// WebSearch searches public web pages. DuckDuckGo's public HTML page is always
// the primary backend. Desktop Cometline can supply a Chromium-backed Google
// bridge as a fallback; a protected web_fetch of Google is the final fallback.
type WebSearch struct {
	Endpoint string
	Token    string
	Client   *http.Client

	// fetchFallback is injectable for tests. Production uses WebFetch.
	fetchFallback func(context.Context, string) (Result, error)
}

func (WebSearch) Spec() ToolSpec {
	return ToolSpec{
		Name:        "web_search",
		Description: "Search the public web through DuckDuckGo and return a small list of current results. If DuckDuckGo is unavailable, the app may fall back to Google search and then a protected web fetch. Use web_fetch on selected result URLs to read their content. This tool does not log into websites or bypass consent screens, CAPTCHA, or rate limits.",
		Parameters: json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{` +
			`"query":{"type":"string","description":"Natural-language web search query"},` +
			`"limit":{"type":"integer","description":"Number of results to return, from 1 to 10 (default 5)"},` +
			`"recency":{"type":"string","description":"Optional recency hint such as day, week, month, or year"}` +
			`},"required":["query"]}`),
	}
}

func (w WebSearch) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	in, err := parseWebSearchInput(input)
	if err != nil {
		return Result{}, err
	}
	in.Query = strings.TrimSpace(in.Query)
	if in.Query == "" {
		return Result{OK: false, Output: "query is required"}, nil
	}
	if len([]rune(in.Query)) > webSearchMaxQuery {
		return Result{OK: false, Output: fmt.Sprintf("query exceeds %d characters", webSearchMaxQuery)}, nil
	}
	if in.Limit <= 0 {
		in.Limit = webSearchDefaultLimit
	}
	if in.Limit > webSearchMaxLimit {
		in.Limit = webSearchMaxLimit
	}

	searchCtx, cancel := context.WithTimeout(ctx, webSearchTimeout)
	defer cancel()

	ddgCtx, ddgCancel := webSearchAttemptContext(searchCtx)
	response, ddgErr := searchDuckDuckGoHTML(w, ddgCtx, in)
	ddgCancel()
	if ddgErr == nil && len(response.Results) > 0 {
		return Result{OK: true, Output: formatWebSearchResponse(response)}, nil
	}

	var failures []string
	if ddgErr != nil {
		failures = append(failures, "DuckDuckGo: "+ddgErr.Error())
	} else {
		failures = append(failures, "DuckDuckGo returned no results")
	}
	if strings.TrimSpace(w.Endpoint) != "" {
		bridgeCtx, bridgeCancel := webSearchAttemptContext(searchCtx)
		response, err = w.searchBrowserBridge(bridgeCtx, in)
		bridgeCancel()
		if err == nil && len(response.Results) > 0 {
			return Result{OK: true, Output: formatWebSearchResponse(response)}, nil
		}
		if err != nil {
			failures = append(failures, "Google bridge: "+err.Error())
		} else {
			failures = append(failures, "Google bridge returned no results")
		}
	}
	fetchCtx, fetchCancel := webSearchAttemptContext(searchCtx)
	fallback, fallbackErr := w.searchViaWebFetch(fetchCtx, in)
	fetchCancel()
	if fallbackErr == nil {
		return Result{OK: true, Output: formatWebSearchResponse(fallback)}, nil
	}
	failures = append(failures, "web_fetch: "+fallbackErr.Error())
	return Result{OK: false, Output: "web search failed: " + strings.Join(failures, "; ")}, nil
}

func webSearchAttemptContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, webSearchAttemptTimeout)
}

func parseWebSearchInput(input json.RawMessage) (webSearchInput, error) {
	var in webSearchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return webSearchInput{}, fmt.Errorf("invalid web_search input: %w", err)
	}
	return in, nil
}

func (w WebSearch) searchBrowserBridge(ctx context.Context, in webSearchInput) (webSearchResponse, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return webSearchResponse{}, fmt.Errorf("encode browser search: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.Endpoint, strings.NewReader(string(body)))
	if err != nil {
		return webSearchResponse{}, fmt.Errorf("build browser search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(w.Token) != "" {
		req.Header.Set("X-Cometline-Browser-Token", w.Token)
	}

	resp, err := w.httpClient().Do(req)
	if err != nil {
		return webSearchResponse{}, fmt.Errorf("browser bridge request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return webSearchResponse{}, fmt.Errorf("browser bridge returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(message)))
	}
	var out webSearchResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, webSearchMaxBodyBytes)).Decode(&out); err != nil {
		return webSearchResponse{}, fmt.Errorf("decode browser bridge response: %w", err)
	}
	if out.Backend == "" {
		out.Backend = "electron-chromium"
	}
	return normalizeSearchResponse(out, in), nil
}

func searchDuckDuckGoHTML(w WebSearch, ctx context.Context, in webSearchInput) (webSearchResponse, error) {
	query := url.Values{"q": []string{in.Query}}
	if filter := searchRecencyFilter(in.Recency); filter != "" {
		query.Set("df", filter)
	}
	target := "https://html.duckduckgo.com/html/?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return webSearchResponse{}, fmt.Errorf("build search request: %w", err)
	}
	req.Header.Set("User-Agent", webSearchUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := w.httpClient().Do(req)
	if err != nil {
		return webSearchResponse{}, fmt.Errorf("search request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return webSearchResponse{}, fmt.Errorf("search engine returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, webSearchMaxBodyBytes))
	if err != nil {
		return webSearchResponse{}, fmt.Errorf("read search results: %w", err)
	}
	results := parseDuckDuckGoResults(string(body), in.Limit)
	return normalizeSearchResponse(webSearchResponse{
		Query:   in.Query,
		Backend: "public-html",
		Results: results,
	}, in), nil
}

func (w WebSearch) searchViaWebFetch(ctx context.Context, in webSearchInput) (webSearchResponse, error) {
	target := "https://www.google.com/search?" + url.Values{"q": []string{in.Query}}.Encode()
	fetch := w.fetchFallback
	if fetch == nil {
		fetch = func(ctx context.Context, target string) (Result, error) {
			return WebFetch{}.Execute(ctx, json.RawMessage(fmt.Sprintf(`{"url":%q,"max_chars":%d}`, target, webSearchMaxOutput)))
		}
	}
	result, err := fetch(ctx, target)
	if err != nil {
		return webSearchResponse{}, err
	}
	if !result.OK {
		return webSearchResponse{}, fmt.Errorf("%s", strings.TrimSpace(result.Output))
	}
	text := strings.TrimSpace(result.Output)
	if text == "" {
		return webSearchResponse{}, fmt.Errorf("empty Google search page")
	}
	return normalizeSearchResponse(webSearchResponse{
		Query:   in.Query,
		Backend: "web-fetch-google",
		Results: []SearchResult{{
			Title:   "Google search fallback",
			URL:     target,
			Snippet: text,
			Source:  "Google via web_fetch",
		}},
	}, in), nil
}

func (w WebSearch) httpClient() *http.Client {
	if w.Client != nil {
		return w.Client
	}
	return &http.Client{Timeout: webSearchTimeout}
}

func searchRecencyFilter(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "day", "d":
		return "d"
	case "week", "w":
		return "w"
	case "month", "m":
		return "m"
	case "year", "y":
		return "y"
	default:
		return ""
	}
}

func parseDuckDuckGoResults(raw string, limit int) []SearchResult {
	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return nil
	}
	results := make([]SearchResult, 0, limit)
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if len(results) >= limit {
			return
		}
		if node.Type == html.ElementNode && node.Data == "a" && hasClass(node, "result__a") {
			if href := attr(node, "href"); href != "" {
				resultURL := normalizeSearchURL(href)
				if resultURL != "" {
					result := SearchResult{
						Title:   strings.TrimSpace(nodeText(node)),
						URL:     resultURL,
						Snippet: strings.TrimSpace(findClassText(node.Parent, "result__snippet")),
						Source:  "DuckDuckGo",
					}
					if result.Title != "" {
						results = append(results, result)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	return results
}

func normalizeSearchResponse(response webSearchResponse, in webSearchInput) webSearchResponse {
	response.Query = in.Query
	if response.Backend == "" {
		response.Backend = "unknown"
	}
	if len(response.Results) > in.Limit {
		response.Results = response.Results[:in.Limit]
	}
	return response
}

func formatWebSearchResponse(response webSearchResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Query: %s\nBackend: %s\nResults: %d\n", response.Query, response.Backend, len(response.Results))
	for i, result := range response.Results {
		fmt.Fprintf(&b, "\n%d. %s\nURL: %s\n", i+1, result.Title, result.URL)
		if result.Source != "" {
			fmt.Fprintf(&b, "Source: %s\n", result.Source)
		}
		if result.Snippet != "" {
			fmt.Fprintf(&b, "Snippet: %s\n", result.Snippet)
		}
	}
	out := b.String()
	if len([]rune(out)) > webSearchMaxOutput {
		out = string([]rune(out)[:webSearchMaxOutput]) + "\n\n[truncated]"
	}
	return out
}

func normalizeSearchURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" && strings.HasPrefix(raw, "//") {
		parsed, err = url.Parse("https:" + raw)
		if err != nil {
			return ""
		}
	}
	if parsed.Host == "duckduckgo.com" || strings.HasSuffix(parsed.Host, ".duckduckgo.com") {
		if target := parsed.Query().Get("uddg"); target != "" {
			decoded, err := url.QueryUnescape(target)
			if err == nil {
				parsed, err = url.Parse(decoded)
				if err != nil {
					return ""
				}
			}
		}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}

func attr(node *html.Node, name string) string {
	for _, attribute := range node.Attr {
		if attribute.Key == name {
			return attribute.Val
		}
	}
	return ""
}

func hasClass(node *html.Node, class string) bool {
	for _, candidate := range strings.Fields(attr(node, "class")) {
		if candidate == class {
			return true
		}
	}
	return false
}

func nodeText(node *html.Node) string {
	if node == nil {
		return ""
	}
	if node.Type == html.TextNode {
		return node.Data
	}
	var parts []string
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		parts = append(parts, nodeText(child))
	}
	return strings.Join(parts, " ")
}

func findClassText(node *html.Node, class string) string {
	if node == nil {
		return ""
	}
	if node.Type == html.ElementNode && hasClass(node, class) {
		return nodeText(node)
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if found := findClassText(child, class); found != "" {
			return found
		}
	}
	return ""
}
