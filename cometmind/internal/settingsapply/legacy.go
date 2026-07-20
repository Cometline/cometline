package settingsapply

import "strings"

// RewriteLegacyActiveProviderPatch remaps deprecated activeProviderId in a
// patch onto defaultProviderId when the patch does not set Default explicitly.
func RewriteLegacyActiveProviderPatch(patch map[string]any) {
	if patch == nil {
		return
	}
	active, ok := patch["activeProviderId"]
	if !ok {
		return
	}
	if _, hasDefault := patch["defaultProviderId"]; !hasDefault {
		if s, ok := active.(string); ok && strings.TrimSpace(s) != "" {
			patch["defaultProviderId"] = strings.TrimSpace(s)
		}
	}
	delete(patch, "activeProviderId")
}

// NormalizeLegacyActiveProvider migrates deprecated activeProviderId into
// defaultProviderId when Default is empty, then strips activeProviderId so
// writes no longer persist the legacy field.
func NormalizeLegacyActiveProvider(doc map[string]any) map[string]any {
	if doc == nil {
		return map[string]any{}
	}
	active, _ := doc["activeProviderId"].(string)
	active = strings.TrimSpace(active)
	def, _ := doc["defaultProviderId"].(string)
	def = strings.TrimSpace(def)
	if def == "" && active != "" {
		doc["defaultProviderId"] = active
	}
	delete(doc, "activeProviderId")
	return doc
}
