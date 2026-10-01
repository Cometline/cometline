package modelcatalog

import (
	"strings"
)

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
