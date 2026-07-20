package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/config"
	"github.com/cometline/cometmind/internal/paths"
	"github.com/cometline/cometmind/internal/processctl"
	"github.com/cometline/cometmind/internal/settingsapply"
)

// SettingsRuntime applies settings changes without killing the current agent turn.
type SettingsRuntime interface {
	Reload(ctx context.Context) error
}

type listSettingsTool struct{}

func (listSettingsTool) Spec() ToolSpec {
	return ToolSpec{
		Name: "list_settings",
		Description: "List agent-editable paths in ~/.cometmind/cometline-settings.json (runtime). " +
			"Desktop UI state (appearance, shortcuts, app/persona) lives in cometline-desktop.json and is unsupported here. " +
			"Apply classes: reload, gateway, or unsupported.",
		Parameters: json.RawMessage(`{"type":"object","properties":{}}`),
	}
}

func (listSettingsTool) Execute(_ context.Context, _ json.RawMessage) (Result, error) {
	b, err := json.MarshalIndent(settingsapply.Catalog(), "", "  ")
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: string(b)}, nil
}

type getSettingsTool struct{}

func (getSettingsTool) Spec() ToolSpec {
	return ToolSpec{
		Name: "get_settings",
		Description: "Read ~/.cometmind/cometline-settings.json (runtime only; not desktop UI). " +
			"Optional dotted path subtree. Secret fields are redacted to " +
			settingsapply.SecretSentinel + " with a sibling <field>_has_value boolean. Never expect raw API keys.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"path":{"type":"string","description":"Optional dotted path such as cometmind.memory or providers. Empty returns the full document."}
			}
		}`),
	}
}

func (getSettingsTool) Execute(_ context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Path string `json:"path"`
	}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return Result{OK: false, Output: "invalid tool input: " + err.Error()}, nil
		}
	}
	doc, err := readSettingsDoc()
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	doc = settingsapply.StripDesktopKeys(doc)
	redacted := settingsapply.RedactSecrets(doc)
	view := any(redacted)
	if p := strings.TrimSpace(in.Path); p != "" {
		view, err = valueAtPath(redacted, p)
		if err != nil {
			return Result{OK: false, Output: err.Error()}, nil
		}
	}
	b, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	return Result{OK: true, Output: string(b)}, nil
}

type patchSettingsTool struct {
	Runtime SettingsRuntime
}

func (patchSettingsTool) Spec() ToolSpec {
	return ToolSpec{
		Name: "patch_settings",
		Description: "Deep-merge a JSON patch into ~/.cometmind/cometline-settings.json (mode 0600), then apply: " +
			"in-place Runtime.Reload for almost all fields, or gateway process restart when cometmind.gateway changes. " +
			"Secret fields set to " + settingsapply.SecretSentinel + " or *** keep the previous value. " +
			"Rejects appearance/shortcuts/app (cometline-desktop.json), systemPromptPath, and host/port. " +
			"Does not restart the main CometMind serve process.",
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"patch":{"type":"object","description":"Partial settings object to deep-merge"}
			},
			"required":["patch"]
		}`),
	}
}

func (t patchSettingsTool) Execute(ctx context.Context, input json.RawMessage) (Result, error) {
	var in struct {
		Patch map[string]any `json:"patch"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return Result{OK: false, Output: "invalid tool input: " + err.Error()}, nil
	}
	if in.Patch == nil {
		return Result{OK: false, Output: "patch is required"}, nil
	}
	settingsapply.RewriteLegacyActiveProviderPatch(in.Patch)
	if desktopKeys := settingsapply.DesktopKeysInPatch(in.Patch); len(desktopKeys) > 0 {
		return Result{OK: false, Output: settingsapply.FormatUnsupported(desktopKeys)}, nil
	}

	before, err := readSettingsDoc()
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	before = settingsapply.StripDesktopKeys(before)
	merged := settingsapply.MergePatch(before, in.Patch)
	settingsapply.RestoreSecretsInProviders(merged, before)
	merged = settingsapply.StripDesktopKeys(merged)
	beforeNorm := settingsapply.NormalizeLegacyActiveProvider(cloneSettingsDoc(before))
	merged = settingsapply.NormalizeLegacyActiveProvider(merged)

	plan, err := settingsapply.Classify(beforeNorm, merged)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if plan.Action == settingsapply.ApplyUnsupported {
		return Result{OK: false, Output: settingsapply.FormatUnsupported(plan.Unsupported)}, nil
	}

	formatted, err := encodeSettingsJSON(merged)
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if err := config.ValidateCometlineSettingsJSON(formatted); err != nil {
		return Result{OK: false, Output: "invalid settings after patch: " + err.Error()}, nil
	}
	settingsPath, err := paths.SettingsPath()
	if err != nil {
		return Result{OK: false, Output: err.Error()}, nil
	}
	if err := os.WriteFile(settingsPath, formatted, 0o600); err != nil {
		return Result{OK: false, Output: "write settings: " + err.Error()}, nil
	}

	applied := []string{}
	if plan.Action == settingsapply.ApplyNone && !plan.Gateway {
		applied = append(applied, "none")
	}
	if plan.Action == settingsapply.ApplyReload {
		if err := t.reload(ctx); err != nil {
			return Result{OK: false, Output: fmt.Sprintf("settings written but reload failed: %v", err)}, nil
		}
		applied = append(applied, "reload")
	}
	if plan.Gateway || plan.Action == settingsapply.ApplyGateway {
		if err := restartGateways(); err != nil {
			return Result{OK: false, Output: fmt.Sprintf("settings written but gateway restart failed: %v", err)}, nil
		}
		applied = append(applied, "gateway")
	}

	out := map[string]any{
		"ok":           true,
		"apply":        applied,
		"settingsPath": settingsPath,
		"note":         "Secrets were preserved when patch used " + settingsapply.SecretSentinel + "/****. Main serve process was not restarted.",
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return Result{OK: true, Output: string(b)}, nil
}

func (t patchSettingsTool) reload(ctx context.Context) error {
	if t.Runtime == nil {
		return fmt.Errorf("settings runtime reload is not wired")
	}
	return t.Runtime.Reload(ctx)
}

func restartGateways() error {
	var firstErr error
	for _, mode := range processctl.GatewayModes() {
		state, err := processctl.ReadState(mode)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if !state.Running && !state.Present {
			continue
		}
		if err := processctl.Restart(mode, 10*time.Second); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("restart %s: %w", mode, err)
		}
	}
	return firstErr
}

func readSettingsDoc() (map[string]any, error) {
	settingsPath, err := paths.SettingsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("settings file does not exist at %s", settingsPath)
		}
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}
	return doc, nil
}

func encodeSettingsJSON(doc map[string]any) ([]byte, error) {
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	b = append(b, '\n')
	return b, nil
}

func cloneSettingsDoc(doc map[string]any) map[string]any {
	if doc == nil {
		return map[string]any{}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		out := make(map[string]any, len(doc))
		for k, v := range doc {
			out[k] = v
		}
		return out
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]any{}
	}
	return out
}

func valueAtPath(doc map[string]any, path string) (any, error) {
	parts := strings.Split(path, ".")
	var cur any = doc
	for _, part := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("path %q not found", path)
		}
		next, ok := m[part]
		if !ok {
			return nil, fmt.Errorf("path %q not found", path)
		}
		cur = next
	}
	return cur, nil
}
