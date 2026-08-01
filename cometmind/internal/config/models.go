package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/cometline/cometmind/internal/modelcatalog"
	"github.com/cometline/cometmind/internal/paths"
)

// ModelEntry is one selectable model from Cometline provider settings.
type ModelEntry struct {
	ProviderID            string   `json:"provider_id"`
	ModelID               string   `json:"model_id"`
	Name                  string   `json:"name"`
	Context               int      `json:"context"`
	Output                int      `json:"output"`
	LimitSource           string   `json:"limit_source"`
	Vision                bool     `json:"vision"`
	VisionKnown           bool     `json:"vision_known"`
	InputModalities       []string `json:"input_modalities"`
	ReasoningEffortOptions []string `json:"reasoning_effort_options"`
}

// LabelForModel formats a model id for display (matches Cometline UI labeling).
func LabelForModel(modelID string) string {
	parts := strings.FieldsFunc(modelID, func(r rune) bool {
		return r == '_' || r == '/'
	})
	if len(parts) == 0 {
		return modelID
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		for i := 1; i < len(runes); i++ {
			runes[i] = unicode.ToUpper(runes[i])
		}
		out = append(out, string(runes))
	}
	return strings.Join(out, " ")
}

// ListConfiguredModels returns enabled models across all enabled providers in settings.
func ListConfiguredModels() ([]ModelEntry, error) {
	raw, err := readCometlineSettingsJSON()
	if err != nil {
		return nil, err
	}
	return modelEntriesFromSettings(raw), nil
}

func modelEntriesFromSettings(raw cometlineSettingsJSON) []ModelEntry {
	out := make([]ModelEntry, 0)
	seen := make(map[string]struct{})
	for _, provider := range raw.Providers {
		if !provider.Enabled {
			continue
		}
		for _, modelID := range provider.EnabledModels {
			modelID = strings.TrimSpace(modelID)
			if modelID == "" || isEmbeddingModelName(modelID) {
				continue
			}
			key := provider.ID + "\x00" + modelID
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			limits := modelcatalog.ResolveLimits(provider.Method, provider.ID, modelID)
			out = append(out, ModelEntry{
				ProviderID:            strings.TrimSpace(provider.ID),
				ModelID:               modelID,
				Name:                  LabelForModel(modelID),
				Context:               limits.Context,
				Output:                limits.Output,
				LimitSource:           limits.Source,
				Vision:                limits.Vision,
				VisionKnown:           limits.VisionKnown,
				InputModalities:       append([]string(nil), limits.InputModalities...),
				ReasoningEffortOptions: modelcatalog.ResolveReasoningOptions(provider.Method, provider.ID, modelID).Effort,
			})
		}
	}
	return out
}

// UpdateDefaultModel writes defaultProviderId and defaultModelId to settings JSON.
func UpdateDefaultModel(providerID, modelID string) error {
	providerID = strings.TrimSpace(providerID)
	modelID = strings.TrimSpace(modelID)
	if providerID == "" || modelID == "" {
		return fmt.Errorf("provider id and model id are required")
	}

	settingsPath, err := paths.SettingsPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("settings file does not exist at %s", settingsPath)
		}
		return err
	}

	var raw cometlineSettingsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse settings JSON: %w", err)
	}
	if !modelEnabledForProvider(raw.Providers, providerID, modelID) {
		return fmt.Errorf("model %q is not enabled for provider %q", modelID, providerID)
	}

	doc := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse settings JSON: %w", err)
	}
	providerJSON, err := json.Marshal(providerID)
	if err != nil {
		return err
	}
	modelJSON, err := json.Marshal(modelID)
	if err != nil {
		return err
	}
	doc["defaultProviderId"] = providerJSON
	doc["defaultModelId"] = modelJSON

	formatted, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	formatted = append(formatted, '\n')
	return os.WriteFile(settingsPath, formatted, 0o600)
}

func modelEnabledForProvider(providers []cometlineProviderJSON, providerID, modelID string) bool {
	for _, provider := range providers {
		if strings.TrimSpace(provider.ID) != providerID || !provider.Enabled {
			continue
		}
		for _, enabled := range provider.EnabledModels {
			if strings.TrimSpace(enabled) == modelID {
				return true
			}
		}
		return false
	}
	return false
}

func readCometlineSettingsJSON() (cometlineSettingsJSON, error) {
	settingsPath, err := paths.SettingsPath()
	if err != nil {
		return cometlineSettingsJSON{}, err
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cometlineSettingsJSON{}, fmt.Errorf("settings file does not exist at %s", settingsPath)
		}
		return cometlineSettingsJSON{}, err
	}
	var raw cometlineSettingsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return cometlineSettingsJSON{}, fmt.Errorf("parse settings JSON: %w", err)
	}
	return raw, nil
}

// LookupModelCatalog resolves catalog limits for arbitrary model ids under a method.
func LookupModelCatalog(method, providerID string, modelIDs []string) []ModelCatalogLookupEntry {
	out := make([]ModelCatalogLookupEntry, 0, len(modelIDs))
	seen := make(map[string]struct{}, len(modelIDs))
	for _, modelID := range modelIDs {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			continue
		}
		if _, ok := seen[modelID]; ok {
			continue
		}
		seen[modelID] = struct{}{}
		limits := modelcatalog.ResolveLimits(method, providerID, modelID)
		out = append(out, ModelCatalogLookupEntry{
			ModelID:               modelID,
			Context:               limits.Context,
			Output:                limits.Output,
			LimitSource:           limits.Source,
			Vision:                limits.Vision,
			VisionKnown:           limits.VisionKnown,
			InputModalities:       append([]string(nil), limits.InputModalities...),
			ReasoningEffortOptions: modelcatalog.ResolveReasoningOptions(method, providerID, modelID).Effort,
		})
	}
	return out
}

// ModelCatalogLookupEntry is one resolved catalog row for fetch-time enrichment.
type ModelCatalogLookupEntry struct {
	ModelID               string   `json:"model_id"`
	Context               int      `json:"context"`
	Output                int      `json:"output"`
	LimitSource           string   `json:"limit_source"`
	Vision                bool     `json:"vision"`
	VisionKnown           bool     `json:"vision_known"`
	InputModalities       []string `json:"input_modalities"`
	ReasoningEffortOptions []string `json:"reasoning_effort_options"`
}
