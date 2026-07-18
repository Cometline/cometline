package provider

import (
	"net/url"
	"strings"
)

const defaultOllamaNativeBase = "http://127.0.0.1:11434"

// NormalizeOllamaNativeBase returns the daemon base without a trailing /v1.
func NormalizeOllamaNativeBase(raw string) string {
	base := strings.TrimRight(strings.TrimSpace(raw), "/")
	if base == "" {
		return defaultOllamaNativeBase
	}
	if strings.HasSuffix(strings.ToLower(base), "/v1") {
		base = strings.TrimRight(base[:len(base)-3], "/")
	}
	if base == "" {
		return defaultOllamaNativeBase
	}
	return base
}

// OllamaChatBaseURL returns the OpenAI-compatible chat base (…/v1).
func OllamaChatBaseURL(raw string) string {
	return NormalizeOllamaNativeBase(raw) + "/v1"
}

// IsLoopbackOllamaURL reports whether the URL targets a loopback host.
func IsLoopbackOllamaURL(raw string) bool {
	parsed, err := url.Parse(NormalizeOllamaNativeBase(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "127.0.0.1" || host == "localhost" || host == "::1"
}
