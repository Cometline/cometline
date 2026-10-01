package modelcatalog

import (
	"sort"
	"strings"
)

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
