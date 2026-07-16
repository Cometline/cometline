package settingsapply

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/cometline/cometmind/internal/paths"
)

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

// MigrateSplitFilesIfNeeded peels desktop keys from cometline-settings.json into
// cometline-desktop.json when needed. Idempotent.
func MigrateSplitFilesIfNeeded() error {
	settingsPath, err := paths.SettingsPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse settings for migrate: %w", err)
	}
	if !HasDesktopKeys(doc) {
		// Still ensure desktop file exists if systemPromptPath should be mirrored.
		return ensureDesktopPromptMirror(doc)
	}

	desktopPath, err := paths.DesktopSettingsPath()
	if err != nil {
		return err
	}
	existingDesktop := map[string]any{}
	if raw, err := os.ReadFile(desktopPath); err == nil {
		_ = json.Unmarshal(raw, &existingDesktop)
	}

	settingsDoc, peeled := SplitDocument(doc)
	// Prefer freshly peeled desktop keys; keep any other desktop-only fields already present.
	for k, v := range existingDesktop {
		if _, taken := peeled[k]; !taken {
			peeled[k] = v
		}
	}

	if err := writeJSONFile(desktopPath, peeled); err != nil {
		return err
	}
	return writeJSONFile(settingsPath, settingsDoc)
}

func ensureDesktopPromptMirror(settingsDoc map[string]any) error {
	cm, ok := asMap(settingsDoc["cometmind"])
	if !ok {
		return nil
	}
	prompt, ok := cm["systemPromptPath"]
	if !ok {
		return nil
	}
	desktopPath, err := paths.DesktopSettingsPath()
	if err != nil {
		return err
	}
	desktop := map[string]any{}
	if raw, err := os.ReadFile(desktopPath); err == nil {
		_ = json.Unmarshal(raw, &desktop)
	}
	if _, exists := desktop["systemPromptPath"]; exists {
		return nil
	}
	desktop["systemPromptPath"] = prompt
	return writeJSONFile(desktopPath, desktop)
}

func writeJSONFile(path string, doc map[string]any) error {
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o600)
}
