package modelcatalog

import (
	"net/http"
	"sync"
	"time"
)

const (
	// DefaultContext is used when models.dev has no entry for the model.
	DefaultContext = 128_000
	// CacheTTL controls how long a disk/memory catalog snapshot stays fresh.
	CacheTTL = 60 * time.Minute
	// RefreshBackoff is how long a failed remote refresh keeps serving the
	// stale in-memory catalog instead of retrying models.dev.
	RefreshBackoff = 5 * time.Minute
	// APIURL is the models.dev public catalog endpoint.
	APIURL = "https://models.dev/api.json"

	SourceCatalog  = "catalog"
	SourceFallback = "fallback"

	// DefaultProtocolNPM is the AI SDK provider package assumed when models.dev
	// carries no npm override. It mirrors OpenCode's own default and selects
	// the OpenAI Chat Completions protocol.
	DefaultProtocolNPM = "@ai-sdk/openai-compatible"
	// NPMOpenAI selects the OpenAI Responses protocol.
	NPMOpenAI = "@ai-sdk/openai"
	// NPMAnthropic selects the Anthropic Messages protocol.
	NPMAnthropic = "@ai-sdk/anthropic"

	// diskCacheVersion bumps when the on-disk shape must be invalidated
	// (e.g. older caches stored only limit fields and dropped modalities).
	diskCacheVersion = 5
)

// Limits are the resolved context/output caps for one model.
type Limits struct {
	Context         int
	Output          int // 0 means unset (do not cap user max tokens)
	Source          string
	Vision          bool     // true when modalities.input includes "image"
	VisionKnown     bool     // false on silent fallback (do not proactive-strip)
	InputModalities []string // normalized catalog input modalities when VisionKnown
}

type modelLimit struct {
	Context int `json:"context"`
	Input   int `json:"input"`
	Output  int `json:"output"`
}

type modelModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type modelProviderMeta struct {
	NPM string `json:"npm"`
	API string `json:"api"`
}

type reasoningOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values"`
	Min    *int     `json:"min"`
	Max    *int     `json:"max"`
}

type modelCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cache_read"`
	CacheWrite float64 `json:"cache_write"`
}

// Cost is models.dev USD per 1M tokens. Found is false when the catalog
// has no cost object for the model.
type Cost struct {
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
	Found      bool
}

type modelEntry struct {
	ID               string             `json:"id"`
	Attachment       bool               `json:"attachment"`
	Modalities       modelModalities    `json:"modalities"`
	Limit            modelLimit         `json:"limit"`
	Cost             *modelCost         `json:"cost"`
	Provider         *modelProviderMeta `json:"provider"`
	ReasoningOptions []reasoningOption  `json:"reasoning_options"`
}

type providerEntry struct {
	ID     string                `json:"id"`
	API    string                `json:"api"`
	NPM    string                `json:"npm"`
	Models map[string]modelEntry `json:"models"`
}

// Catalog is a parsed models.dev snapshot keyed by provider → model.
type Catalog struct {
	Providers map[string]providerEntry
	FetchedAt time.Time
}

var (
	mu              sync.Mutex
	cached          *Catalog
	refreshFailedAt time.Time
	httpClient      = &http.Client{Timeout: 20 * time.Second}
	fetchURL        = APIURL
	nowFn           = time.Now
	cachePathFn     = modelsDevCachePath
)

// ResetCacheForTest clears in-memory catalog state (tests only).
func ResetCacheForTest() {
	mu.Lock()
	defer mu.Unlock()
	cached = nil
	refreshFailedAt = time.Time{}
}

// SetNowForTest overrides the catalog clock (tests only). Pass nil to restore.
func SetNowForTest(fn func() time.Time) {
	mu.Lock()
	defer mu.Unlock()
	if fn == nil {
		nowFn = time.Now
		return
	}
	nowFn = fn
}

// SetFetchURLForTest overrides the remote URL (tests only).
func SetFetchURLForTest(url string) {
	mu.Lock()
	defer mu.Unlock()
	fetchURL = url
}

// SetCachePathForTest overrides the disk cache path (tests only).
func SetCachePathForTest(path string) {
	mu.Lock()
	defer mu.Unlock()
	cachePathFn = func() (string, error) { return path, nil }
}

// ResetCachePathForTest restores the default disk cache path (tests only).
func ResetCachePathForTest() {
	mu.Lock()
	defer mu.Unlock()
	cachePathFn = modelsDevCachePath
}

// LoadFromJSONForTest installs a catalog parsed from JSON bytes (tests only).
func LoadFromJSONForTest(data []byte) error {
	cat, err := parseCatalog(data, nowFn())
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	cached = cat
	return nil
}
