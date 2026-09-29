package settingsapply

// Desktop top-level keys that belong in cometline-desktop.json, not the runtime settings file.
var desktopTopLevelKeys = []string{"appearance", "shortcuts", "app"}

// HasDesktopKeys reports whether doc still carries desktop-owned top-level keys.
func HasDesktopKeys(doc map[string]any) bool {
	if doc == nil {
		return false
	}
	for _, key := range desktopTopLevelKeys {
		if _, ok := doc[key]; ok {
			return true
		}
	}
	return false
}

// SplitDocument peels a merged ProviderSettings-shaped document into runtime settings
// and desktop documents. systemPromptPath is copied into both: desktop is the UI
// source of truth; settings keeps a stamp so CometMind Load can ignore the desktop file.
func SplitDocument(merged map[string]any) (settings, desktop map[string]any) {
	settings = deepCloneMap(merged)
	desktop = map[string]any{}

	for _, key := range desktopTopLevelKeys {
		if v, ok := settings[key]; ok {
			desktop[key] = deepCloneValue(v)
			delete(settings, key)
		}
	}

	if cm, ok := asMap(settings["cometmind"]); ok {
		if prompt, ok := cm["systemPromptPath"]; ok {
			desktop["systemPromptPath"] = deepCloneValue(prompt)
			// Keep stamp on runtime settings for CometMind Load.
			settings["cometmind"] = cm
		}
	}
	return settings, desktop
}

// MergeDocuments combines runtime settings + desktop into one UI-facing document.
// Desktop systemPromptPath wins over settings when both are set.
func MergeDocuments(settings, desktop map[string]any) map[string]any {
	out := deepCloneMap(settings)
	if desktop == nil {
		return out
	}
	for _, key := range desktopTopLevelKeys {
		if v, ok := desktop[key]; ok {
			out[key] = deepCloneValue(v)
		}
	}
	if prompt, ok := desktop["systemPromptPath"]; ok {
		cm, ok := asMap(out["cometmind"])
		if !ok || cm == nil {
			cm = map[string]any{}
		} else {
			cm = deepCloneMap(cm)
		}
		cm["systemPromptPath"] = deepCloneValue(prompt)
		out["cometmind"] = cm
	}
	return out
}

// StripDesktopKeys returns a copy of doc without appearance/shortcuts/app.
func StripDesktopKeys(doc map[string]any) map[string]any {
	out := deepCloneMap(doc)
	for _, key := range desktopTopLevelKeys {
		delete(out, key)
	}
	return out
}

// DesktopKeysInPatch lists desktop-owned top-level keys present in patch.
func DesktopKeysInPatch(patch map[string]any) []string {
	if patch == nil {
		return nil
	}
	var found []string
	for _, key := range desktopTopLevelKeys {
		if _, ok := patch[key]; ok {
			found = append(found, key)
		}
	}
	return found
}
