package modelcatalog

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cometline/cometmind/internal/paths"
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

type diskCacheEnvelope struct {
	Version   int                      `json:"version"`
	Providers map[string]providerEntry `json:"providers"`
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

func modelsDevCachePath() (string, error) {
	d, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "models-dev.json"), nil
}

// ResolveLimits looks up context/output/vision for a provider method + model.
//
// Native providers (anthropic, openai, …) resolve within their models.dev
// provider bucket. Ollama / openai-compatible / unknown methods first try the
// settings provider id as a catalog key, then scan the whole catalog by model
// id (including common `org/model` and `:tag` variants) so company gateways and
// local runtimes still get caps when the underlying model is in models.dev.
//
// On fallback, VisionKnown is false so callers must not proactive-strip images.
func ResolveLimits(method, providerID, modelID string) Limits {
	fallback := Limits{Context: DefaultContext, Output: 0, Source: SourceFallback, Vision: false, VisionKnown: false}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return fallback
	}
	cat, err := load()
	if err != nil || cat == nil {
		return fallback
	}

	candidates := modelIDCandidates(modelID)
	if key, scoped := catalogProviderKey(method, providerID); scoped {
		if entry, ok := findModelInProvider(cat.Providers[key], candidates); ok {
			return limitsFromEntry(entry)
		}
		// OpenCode Zen (`opencode`) and OpenCode Go (`opencode-go`) are sibling
		// models.dev providers; try the sibling before falling back.
		if alt := opencodeSiblingProvider(key); alt != "" {
			if entry, ok := findModelInProvider(cat.Providers[alt], candidates); ok {
				return limitsFromEntry(entry)
			}
		}
		return fallback
	}

	// Unscoped: prefer an explicit catalog provider matching settings provider id,
	// then the ollama bucket for ollama method, then a global id scan.
	if pid := strings.ToLower(strings.TrimSpace(providerID)); pid != "" {
		if provider, ok := cat.Providers[pid]; ok {
			if entry, ok := findModelInProvider(provider, candidates); ok {
				return limitsFromEntry(entry)
			}
		}
	}
	if strings.EqualFold(strings.TrimSpace(method), "ollama") {
		if provider, ok := cat.Providers["ollama"]; ok {
			if entry, ok := findModelInProvider(provider, candidates); ok {
				return limitsFromEntry(entry)
			}
		}
	}
	if entry, ok := findModelAcrossCatalog(cat, candidates); ok {
		return limitsFromEntry(entry)
	}
	return fallback
}

// ResolveCost looks up models.dev per-1M-token rates. providerID is the
// settings / usage-event provider id; the catalog is scanned by model id
// when that key is not a models.dev bucket.
func ResolveCost(providerID, modelID string) (Cost, bool) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return Cost{}, false
	}
	cat, err := load()
	if err != nil || cat == nil {
		return Cost{}, false
	}
	candidates := modelIDCandidates(modelID)
	if pid := strings.ToLower(strings.TrimSpace(providerID)); pid != "" {
		if provider, ok := cat.Providers[pid]; ok {
			if entry, ok := findCatalogEntry(provider, candidates); ok {
				return costFromEntry(entry)
			}
		}
	}
	if entry, ok := findCatalogEntryAcross(cat, candidates); ok {
		return costFromEntry(entry)
	}
	return Cost{}, false
}

func costFromEntry(entry modelEntry) (Cost, bool) {
	if entry.Cost == nil {
		return Cost{}, false
	}
	return Cost{
		Input:      entry.Cost.Input,
		Output:     entry.Cost.Output,
		CacheRead:  entry.Cost.CacheRead,
		CacheWrite: entry.Cost.CacheWrite,
		Found:      true,
	}, true
}

func findCatalogEntry(provider providerEntry, candidates []string) (modelEntry, bool) {
	if len(provider.Models) == 0 || len(candidates) == 0 {
		return modelEntry{}, false
	}
	for _, candidate := range candidates {
		if entry, ok := provider.Models[candidate]; ok {
			return entry, true
		}
	}
	for _, candidate := range candidates {
		for id, entry := range provider.Models {
			if modelIdentityMatch(id, entry, candidate) {
				return entry, true
			}
		}
	}
	return modelEntry{}, false
}

func findCatalogEntryAcross(cat *Catalog, candidates []string) (modelEntry, bool) {
	if cat == nil || len(candidates) == 0 {
		return modelEntry{}, false
	}
	for _, candidate := range candidates {
		for _, providerID := range sortedCatalogProviderIDs(cat) {
			if entry, ok := findCatalogEntry(cat.Providers[providerID], []string{candidate}); ok {
				return entry, true
			}
		}
	}
	return modelEntry{}, false
}

func limitsFromEntry(entry modelEntry) Limits {
	modalities := normalizeInputModalities(entry.Modalities.Input)
	return Limits{
		Context:         entry.Limit.Context,
		Output:          entry.Limit.Output,
		Source:          SourceCatalog,
		Vision:          hasModality(modalities, "image"),
		VisionKnown:     true,
		InputModalities: modalities,
	}
}

// Protocol is the resolved wire protocol for one model: the AI SDK provider
// package it speaks plus the API base URL the provider entry carries.
type Protocol struct {
	NPM    string
	API    string
	Source string
}

// ResolveProviderMetadata resolves the wire protocol (npm package + API URL)
// for a model, mirroring OpenCode's precedence: model-level provider overrides
// win over provider-level defaults, which win over the openai-compatible
// default. It only applies to scoped catalog providers (opencode-go, opencode);
// other methods always resolve to the default protocol so custom gateways are
// never switched to the Responses protocol by accident.
func ResolveProviderMetadata(method, providerID, modelID string) Protocol {
	fallback := Protocol{NPM: DefaultProtocolNPM, Source: SourceFallback}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return fallback
	}
	key, scoped := catalogProviderKey(method, providerID)
	if !scoped {
		return fallback
	}
	cat, err := load()
	if err != nil || cat == nil {
		return fallback
	}

	candidates := modelIDCandidates(modelID)
	if provider, ok := cat.Providers[key]; ok {
		if entry, ok := findModelInProvider(provider, candidates); ok {
			return protocolFromEntry(provider, entry)
		}
	}
	// OpenCode Zen (`opencode`) and OpenCode Go (`opencode-go`) are sibling
	// models.dev providers; try the sibling before falling back.
	if alt := opencodeSiblingProvider(key); alt != "" {
		if provider, ok := cat.Providers[alt]; ok {
			if entry, ok := findModelInProvider(provider, candidates); ok {
				return protocolFromEntry(provider, entry)
			}
		}
	}
	return fallback
}

// RequiresEmptyReasoningContentReplay identifies the DeepSeek model family on
// OpenCode's OpenAI-compatible endpoints. DeepSeek thinking tool-call chains
// require reasoning_content to be replayed even when its value is an empty
// string. This mirrors OpenCode's DeepSeek family fallback rather than naming
// only known V4 releases.
func RequiresEmptyReasoningContentReplay(method, providerID, modelID string) bool {
	key, scoped := catalogProviderKey(method, providerID)
	if !scoped || (key != "opencode-go" && key != "opencode") {
		return false
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(modelID)), "deepseek")
}

func protocolFromEntry(provider providerEntry, entry modelEntry) Protocol {
	protocol := Protocol{Source: SourceCatalog}
	if entry.Provider != nil {
		protocol.NPM = entry.Provider.NPM
		protocol.API = entry.Provider.API
	}
	if protocol.NPM == "" {
		protocol.NPM = provider.NPM
	}
	if protocol.NPM == "" {
		protocol.NPM = DefaultProtocolNPM
	}
	if protocol.API == "" {
		protocol.API = provider.API
	}
	return protocol
}

// ReasoningOptions are the reasoning controls a model advertises via models.dev
// reasoning_options. Source is SourceCatalog when resolved and SourceFallback
// when the model is unknown.
type ReasoningOptions struct {
	// Effort lists the allowed reasoning effort values (e.g. none, low,
	// medium, high, xhigh, max) for models with an "effort" option.
	Effort []string
	// Toggle is true for models that support reasoning on/off.
	Toggle bool
	// BudgetMin / BudgetMax bound thinking token budgets when advertised.
	BudgetMin *int
	BudgetMax *int
	Source    string
}

// ResolveReasoningOptions looks up the reasoning controls for a model. Scoped
// providers use their exact catalog entry. Custom gateways first try a catalog
// provider matching their settings id, then conservatively intersect effort
// values advertised by matching models across the catalog.
func ResolveReasoningOptions(method, providerID, modelID string) ReasoningOptions {
	fallback := ReasoningOptions{Source: SourceFallback}
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return fallback
	}
	key, scoped := catalogProviderKey(method, providerID)
	cat, err := load()
	if err != nil || cat == nil {
		return fallback
	}

	candidates := modelIDCandidates(modelID)
	if scoped {
		if provider, ok := cat.Providers[key]; ok {
			if entry, ok := findModelInProvider(provider, candidates); ok {
				return reasoningOptionsFromEntry(entry)
			}
		}
		if alt := opencodeSiblingProvider(key); alt != "" {
			if provider, ok := cat.Providers[alt]; ok {
				if entry, ok := findModelInProvider(provider, candidates); ok {
					return reasoningOptionsFromEntry(entry)
				}
			}
		}
		return fallback
	}

	if pid := strings.ToLower(strings.TrimSpace(providerID)); pid != "" {
		if provider, ok := cat.Providers[pid]; ok {
			if entry, ok := findModelInProvider(provider, candidates); ok {
				return reasoningOptionsFromEntry(entry)
			}
		}
	}
	if strings.EqualFold(strings.TrimSpace(method), "ollama") {
		if provider, ok := cat.Providers["ollama"]; ok {
			if entry, ok := findModelInProvider(provider, candidates); ok {
				return reasoningOptionsFromEntry(entry)
			}
		}
	}
	if entries := findModelsAcrossCatalog(cat, candidates); len(entries) > 0 {
		return commonReasoningEffort(entries)
	}
	return fallback
}

func reasoningOptionsFromEntry(entry modelEntry) ReasoningOptions {
	out := ReasoningOptions{Source: SourceCatalog}
	for _, option := range entry.ReasoningOptions {
		switch option.Type {
		case "effort":
			out.Effort = append([]string(nil), option.Values...)
		case "toggle":
			out.Toggle = true
		case "budget_tokens":
			out.BudgetMin = option.Min
			out.BudgetMax = option.Max
		}
	}
	return out
}

func commonReasoningEffort(entries []modelEntry) ReasoningOptions {
	out := ReasoningOptions{Source: SourceCatalog}
	for _, entry := range entries {
		effort := reasoningOptionsFromEntry(entry).Effort
		if len(effort) == 0 {
			continue
		}
		if out.Effort == nil {
			out.Effort = append([]string(nil), effort...)
			continue
		}
		allowed := make(map[string]struct{}, len(effort))
		for _, value := range effort {
			allowed[value] = struct{}{}
		}
		common := out.Effort[:0]
		for _, value := range out.Effort {
			if _, ok := allowed[value]; ok {
				common = append(common, value)
			}
		}
		out.Effort = common
	}
	return out
}

// modelIDCandidates expands a runtime model id into lookup variants.
// Order matters: prefer the exact id, then tag-stripped / org-stripped /
// Claude family-order aliases / deployment-suffix-stripped forms.
func modelIDCandidates(modelID string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 8)
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		key := strings.ToLower(id)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, id)
	}
	add(modelID)
	if i := strings.LastIndex(modelID, ":"); i > 0 {
		add(modelID[:i])
	}
	if i := strings.Index(modelID, "/"); i > 0 && i+1 < len(modelID) {
		rest := modelID[i+1:]
		add(rest)
		if j := strings.LastIndex(rest, ":"); j > 0 {
			add(rest[:j])
		}
	}
	for _, base := range append([]string(nil), out...) {
		if alias := claudeFamilyAlias(base); alias != "" {
			add(alias)
		}
		if stripped := stripDeploymentSuffix(base); stripped != base {
			add(stripped)
			if alias := claudeFamilyAlias(stripped); alias != "" {
				add(alias)
			}
		}
	}
	return out
}

// claudeFamilyAlias rewrites gateway-style ids like claude-4-opus → claude-opus-4
// (models.dev / Anthropic prefer family-before-version).
func claudeFamilyAlias(modelID string) string {
	lower := strings.ToLower(strings.TrimSpace(modelID))
	if !strings.HasPrefix(lower, "claude-") {
		return ""
	}
	parts := strings.Split(lower, "-")
	if len(parts) < 3 {
		return ""
	}
	if parts[1] == "opus" || parts[1] == "sonnet" || parts[1] == "haiku" {
		return ""
	}
	famIdx := -1
	for i := 2; i < len(parts); i++ {
		switch parts[i] {
		case "opus", "sonnet", "haiku":
			famIdx = i
		}
		if famIdx >= 0 {
			break
		}
	}
	if famIdx < 0 {
		return ""
	}
	ver := strings.Join(parts[1:famIdx], "-")
	if ver == "" {
		return ""
	}
	alias := "claude-" + parts[famIdx] + "-" + ver
	if rest := parts[famIdx+1:]; len(rest) > 0 {
		alias += "-" + strings.Join(rest, "-")
	}
	if alias == lower {
		return ""
	}
	return alias
}

func stripDeploymentSuffix(modelID string) string {
	lower := strings.ToLower(modelID)
	for _, suf := range []string{"-aws", "-azure", "-gcp", "-bedrock", "-vertex"} {
		if strings.HasSuffix(lower, suf) {
			return modelID[:len(modelID)-len(suf)]
		}
	}
	return modelID
}

func modelIdentityMatch(catalogKey string, entry modelEntry, candidate string) bool {
	if strings.EqualFold(catalogKey, candidate) {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(entry.ID), candidate) {
		return true
	}
	// openrouter-style anthropic/claude-3-haiku vs bare claude-3-haiku
	if i := strings.LastIndex(catalogKey, "/"); i >= 0 && i+1 < len(catalogKey) {
		if strings.EqualFold(catalogKey[i+1:], candidate) {
			return true
		}
	}
	// sap-ai-core style anthropic--claude-3-haiku
	if i := strings.LastIndex(catalogKey, "--"); i >= 0 && i+2 < len(catalogKey) {
		if strings.EqualFold(catalogKey[i+2:], candidate) {
			return true
		}
	}
	return false
}

func findModelInProvider(provider providerEntry, candidates []string) (modelEntry, bool) {
	if len(provider.Models) == 0 || len(candidates) == 0 {
		return modelEntry{}, false
	}
	for _, candidate := range candidates {
		if entry, ok := provider.Models[candidate]; ok && entry.Limit.Context > 0 {
			return entry, true
		}
	}
	for _, candidate := range candidates {
		for id, entry := range provider.Models {
			if entry.Limit.Context <= 0 {
				continue
			}
			if modelIdentityMatch(id, entry, candidate) {
				return entry, true
			}
		}
	}
	return modelEntry{}, false
}

func findModelAcrossCatalog(cat *Catalog, candidates []string) (modelEntry, bool) {
	if cat == nil || len(candidates) == 0 {
		return modelEntry{}, false
	}
	providerIDs := sortedCatalogProviderIDs(cat)
	for _, candidate := range candidates {
		for _, providerID := range providerIDs {
			if entry, ok := findModelInProvider(cat.Providers[providerID], []string{candidate}); ok {
				return entry, true
			}
		}
	}
	return modelEntry{}, false
}

func findModelsAcrossCatalog(cat *Catalog, candidates []string) []modelEntry {
	if cat == nil || len(candidates) == 0 {
		return nil
	}
	providerIDs := sortedCatalogProviderIDs(cat)
	seen := make(map[string]struct{})
	out := make([]modelEntry, 0)
	for _, candidate := range candidates {
		for _, providerID := range providerIDs {
			if entry, ok := findModelInProvider(cat.Providers[providerID], []string{candidate}); ok {
				identity := strings.ToLower(strings.TrimSpace(entry.ID))
				if identity == "" {
					identity = strings.ToLower(candidate)
				}
				key := providerID + "\x00" + identity
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				out = append(out, entry)
			}
		}
	}
	return out
}

func sortedCatalogProviderIDs(cat *Catalog) []string {
	providerIDs := make([]string, 0, len(cat.Providers))
	for id := range cat.Providers {
		providerIDs = append(providerIDs, id)
	}
	sort.SliceStable(providerIDs, func(i, j int) bool {
		return catalogProviderPreferRank(providerIDs[i]) < catalogProviderPreferRank(providerIDs[j]) ||
			(catalogProviderPreferRank(providerIDs[i]) == catalogProviderPreferRank(providerIDs[j]) &&
				providerIDs[i] < providerIDs[j])
	})
	return providerIDs
}

func catalogProviderPreferRank(providerID string) int {
	switch strings.ToLower(providerID) {
	case "anthropic":
		return 0
	case "openai":
		return 1
	case "google":
		return 2
	case "xai":
		return 3
	case "opencode-go":
		return 4
	case "opencode":
		return 5
	case "ollama":
		return 6
	default:
		return 100
	}
}

func opencodeSiblingProvider(key string) string {
	switch key {
	case "opencode-go":
		return "opencode"
	case "opencode":
		return "opencode-go"
	default:
		return ""
	}
}

func normalizeInputModalities(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		m := strings.ToLower(strings.TrimSpace(item))
		switch m {
		case "text", "image", "video", "audio", "pdf":
			// ok
		case "file", "document", "docs":
			m = "pdf"
		default:
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}

func hasModality(modalities []string, want string) bool {
	for _, m := range modalities {
		if m == want {
			return true
		}
	}
	return false
}

func catalogProviderKey(method, providerID string) (string, bool) {
	m := strings.ToLower(strings.TrimSpace(method))
	if m == "" {
		m = strings.ToLower(strings.TrimSpace(providerID))
	}
	switch m {
	case "anthropic":
		return "anthropic", true
	case "openai":
		return "openai", true
	case "xai":
		return "xai", true
	case "codex":
		return "openai", true
	case "opencode-go":
		return "opencode-go", true
	case "opencode":
		return "opencode", true
	case "ollama", "openai-compatible":
		return "", false
	default:
		return "", false
	}
}

func load() (*Catalog, error) {
	mu.Lock()
	defer mu.Unlock()
	if cached != nil && nowFn().Sub(cached.FetchedAt) < CacheTTL {
		return cached, nil
	}
	if cached != nil && !refreshFailedAt.IsZero() && nowFn().Sub(refreshFailedAt) < RefreshBackoff {
		return cached, nil
	}
	if cat, ok := readDiskCache(); ok {
		cached = cat
		refreshFailedAt = time.Time{}
		return cat, nil
	}
	cat, err := fetchRemote()
	if err != nil {
		refreshFailedAt = nowFn()
		if cached != nil {
			return cached, nil
		}
		return nil, err
	}
	refreshFailedAt = time.Time{}
	cached = cat
	_ = writeDiskCache(cat)
	return cat, nil
}

func readDiskCache() (*Catalog, bool) {
	path, err := cachePathFn()
	if err != nil {
		return nil, false
	}
	info, err := os.Stat(path)
	if err != nil || nowFn().Sub(info.ModTime()) >= CacheTTL {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	cat, err := parseDiskCache(data, info.ModTime())
	if err != nil {
		return nil, false
	}
	if !catalogHasInputModalities(cat) {
		// Pre-vision caches only stored limit fields; force a fresh fetch.
		return nil, false
	}
	return cat, true
}

func writeDiskCache(cat *Catalog) error {
	path, err := cachePathFn()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	payload := diskCacheEnvelope{
		Version:   diskCacheVersion,
		Providers: cat.Providers,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func fetchRemote() (*Catalog, error) {
	req, err := http.NewRequest(http.MethodGet, fetchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "cometmind/modelcatalog")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("models.dev: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	return parseCatalog(data, nowFn())
}

func parseDiskCache(data []byte, fetchedAt time.Time) (*Catalog, error) {
	var envelope diskCacheEnvelope
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Version > 0 {
		if envelope.Version != diskCacheVersion {
			return nil, fmt.Errorf("models.dev cache version %d", envelope.Version)
		}
		if envelope.Providers == nil {
			envelope.Providers = map[string]providerEntry{}
		}
		return &Catalog{Providers: envelope.Providers, FetchedAt: fetchedAt}, nil
	}
	// Legacy caches were a bare provider map (and often missing modalities).
	return parseCatalog(data, fetchedAt)
}

func parseCatalog(data []byte, fetchedAt time.Time) (*Catalog, error) {
	var providers map[string]providerEntry
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, fmt.Errorf("parse models.dev: %w", err)
	}
	if providers == nil {
		providers = map[string]providerEntry{}
	}
	return &Catalog{Providers: providers, FetchedAt: fetchedAt}, nil
}

func catalogHasInputModalities(cat *Catalog) bool {
	if cat == nil {
		return false
	}
	for _, provider := range cat.Providers {
		for _, model := range provider.Models {
			if len(model.Modalities.Input) > 0 {
				return true
			}
		}
	}
	return false
}

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
