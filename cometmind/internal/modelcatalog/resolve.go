package modelcatalog

import (
	"strings"
)

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
