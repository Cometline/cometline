package config

import "strings"

// ResolveRoleLLM returns the provider/model pair for a helper role (titles,
// memory extraction/compaction, autonomy, skill synthesis).
//
// Pins are atomic: both provider and model must be set, otherwise the Default
// model pair is used. Never mixes a pinned model with a different provider.
func (c *Config) ResolveRoleLLM(pinProvider, pinModel string) (providerID, modelID string) {
	pinProvider = strings.TrimSpace(pinProvider)
	pinModel = strings.TrimSpace(pinModel)
	if pinProvider != "" && pinModel != "" {
		return pinProvider, pinModel
	}
	if c == nil {
		return "", ""
	}
	defProvider := strings.TrimSpace(c.DefaultProviderID)
	defModel := strings.TrimSpace(c.DefaultModelID)
	if defProvider == "" {
		defProvider = strings.TrimSpace(c.Provider)
	}
	if defModel == "" {
		defModel = strings.TrimSpace(c.Model)
	}
	return defProvider, defModel
}
