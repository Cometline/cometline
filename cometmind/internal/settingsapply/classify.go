// Package settingsapply classifies and redacts Cometline settings documents
// for agent tools and shared apply semantics with the desktop UI.
package settingsapply

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ApplyAction describes how a settings change should be applied at runtime.
type ApplyAction string

const (
	ApplyNone        ApplyAction = "none"
	ApplyReload      ApplyAction = "reload"
	ApplyGateway     ApplyAction = "gateway"
	ApplyUnsupported ApplyAction = "unsupported"
)

// SecretSentinel is the placeholder written into redacted settings for secret fields.
// On patch, this value (or "***") means "keep the previous secret".
const SecretSentinel = "__REDACTED__"

// FieldMeta describes one agent-editable settings path.
type FieldMeta struct {
	Path        string      `json:"path"`
	Secret      bool        `json:"secret"`
	ApplyClass  ApplyAction `json:"apply_class"`
	Description string      `json:"description"`
}

// Catalog lists agent-facing editable paths and their apply semantics.
func Catalog() []FieldMeta {
	return []FieldMeta{
		{Path: "providers", Secret: false, ApplyClass: ApplyReload, Description: "LLM provider configs (API keys are secret leaf fields)"},
		{Path: "providers[].apiKey", Secret: true, ApplyClass: ApplyReload, Description: "Provider API key"},
		{Path: "activeProviderId", Secret: false, ApplyClass: ApplyReload, Description: "Active provider id"},
		{Path: "defaultModelId", Secret: false, ApplyClass: ApplyReload, Description: "Default model id for new sessions"},
		{Path: "defaultProviderId", Secret: false, ApplyClass: ApplyReload, Description: "Default provider id for new sessions"},
		{Path: "cometmind.acp", Secret: false, ApplyClass: ApplyReload, Description: "Coding-harness delegation"},
		{Path: "cometmind.skills", Secret: false, ApplyClass: ApplyReload, Description: "Agent Skills discovery"},
		{Path: "cometmind.memory", Secret: false, ApplyClass: ApplyReload, Description: "Memory retrieval/extraction/embedding"},
		{Path: "cometmind.memory.embedding.apiKey", Secret: true, ApplyClass: ApplyReload, Description: "Embedding API key override"},
		{Path: "cometmind.storage", Secret: false, ApplyClass: ApplyReload, Description: "Storage retention and cleanup"},
		{Path: "cometmind.mcp", Secret: false, ApplyClass: ApplyReload, Description: "MCP servers"},
		{Path: "cometmind.jobs", Secret: false, ApplyClass: ApplyReload, Description: "Jobs queue settings"},
		{Path: "cometmind.autonomy", Secret: false, ApplyClass: ApplyReload, Description: "Autonomous job worker"},
		{Path: "cometmind.scheduler", Secret: false, ApplyClass: ApplyReload, Description: "Scheduled jobs"},
		{Path: "cometmind.maxTokens", Secret: false, ApplyClass: ApplyReload, Description: "Max output tokens"},
		{Path: "cometmind.contextWindowLimit", Secret: false, ApplyClass: ApplyReload, Description: "Context window limit"},
		{Path: "cometmind.titleProviderId", Secret: false, ApplyClass: ApplyReload, Description: "Title generation provider"},
		{Path: "cometmind.titleModelId", Secret: false, ApplyClass: ApplyReload, Description: "Title generation model"},
		{Path: "cometmind.gateway", Secret: false, ApplyClass: ApplyGateway, Description: "Gateway platforms (restarts gateway process(es) only; serve stays up)"},
		{Path: "cometmind.gateway.discord", Secret: false, ApplyClass: ApplyGateway, Description: "Discord gateway platform settings"},
		{Path: "cometmind.gateway.discord.botToken", Secret: true, ApplyClass: ApplyGateway, Description: "Discord bot token"},
		{Path: "cometmind.systemPromptPath", Secret: false, ApplyClass: ApplyUnsupported, Description: "Packaged prompt path — lives in cometline-desktop.json; use Settings UI"},
		{Path: "cometmind.host", Secret: false, ApplyClass: ApplyUnsupported, Description: "Listen host — not in settings files (Electron/CLI bind)"},
		{Path: "cometmind.port", Secret: false, ApplyClass: ApplyUnsupported, Description: "Listen port — not in settings files (Electron/CLI bind)"},
		{Path: "app", Secret: false, ApplyClass: ApplyUnsupported, Description: "Desktop app/persona — lives in cometline-desktop.json"},
		{Path: "appearance", Secret: false, ApplyClass: ApplyUnsupported, Description: "Desktop appearance — lives in cometline-desktop.json"},
		{Path: "shortcuts", Secret: false, ApplyClass: ApplyUnsupported, Description: "Desktop shortcuts — lives in cometline-desktop.json"},
	}
}

// ClassifyResult is the apply plan for a settings transition.
type ClassifyResult struct {
	Action      ApplyAction `json:"action"`
	Gateway     bool        `json:"gateway"`
	Unsupported []string    `json:"unsupported,omitempty"`
}

// Classify compares before/after settings JSON objects and returns how to apply.
// Unsupported path changes win. Gateway may combine with Reload when
// cometmind.gateway and other runtime fields both change.
func Classify(before, after map[string]any) (ClassifyResult, error) {
	out := ClassifyResult{Action: ApplyNone}
	if before == nil {
		before = map[string]any{}
	}
	if after == nil {
		after = map[string]any{}
	}
	beforeCanon, err := canonicalize(before)
	if err != nil {
		return out, err
	}
	afterCanon, err := canonicalize(after)
	if err != nil {
		return out, err
	}
	if beforeCanon == afterCanon {
		return out, nil
	}

	var unsupported []string
	if changedAt(before, after, "cometmind", "host") || changedAt(before, after, "cometmind", "port") {
		unsupported = append(unsupported, "cometmind.host/port")
	}
	if changedAt(before, after, "cometmind", "systemPromptPath") {
		unsupported = append(unsupported, "cometmind.systemPromptPath")
	}
	if changedAt(before, after, "app") {
		unsupported = append(unsupported, "app")
	}
	if changedAt(before, after, "appearance") {
		unsupported = append(unsupported, "appearance")
	}
	if changedAt(before, after, "shortcuts") {
		unsupported = append(unsupported, "shortcuts")
	}
	if len(unsupported) > 0 {
		out.Action = ApplyUnsupported
		out.Unsupported = unsupported
		return out, nil
	}

	gatewayChanged := changedAt(before, after, "cometmind", "gateway")
	out.Gateway = gatewayChanged

	cometmindSansGatewayEqual := equalSans(before["cometmind"], after["cometmind"], "gateway")
	providersEqual := canonicalizeEqual(before["providers"], after["providers"])
	otherTopEqual := true
	for _, key := range []string{"activeProviderId", "defaultModelId", "defaultProviderId"} {
		if changedAt(before, after, key) {
			otherTopEqual = false
			break
		}
	}
	// Also treat other top-level keys (except appearance/shortcuts/app already rejected)
	// as reload-worthy when they change.
	for k := range after {
		switch k {
		case "providers", "activeProviderId", "defaultModelId", "defaultProviderId", "cometmind", "appearance", "shortcuts", "app":
			continue
		default:
			if changedAt(before, after, k) {
				otherTopEqual = false
			}
		}
	}

	serveNeedsReload := !(cometmindSansGatewayEqual && providersEqual && otherTopEqual)
	if serveNeedsReload {
		out.Action = ApplyReload
		return out, nil
	}
	if gatewayChanged {
		out.Action = ApplyGateway
		return out, nil
	}
	out.Action = ApplyReload
	return out, nil
}

func canonicalize(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func canonicalizeEqual(a, b any) bool {
	ca, err := canonicalize(a)
	if err != nil {
		return false
	}
	cb, err := canonicalize(b)
	if err != nil {
		return false
	}
	return ca == cb
}

func changedAt(before, after map[string]any, path ...string) bool {
	return !canonicalizeEqual(getPath(before, path...), getPath(after, path...))
}

func getPath(root any, path ...string) any {
	cur := root
	for _, key := range path {
		m, ok := asMap(cur)
		if !ok {
			return nil
		}
		cur = m[key]
	}
	return cur
}

func equalSans(before, after any, dropKey string) bool {
	bm, bok := asMap(before)
	am, aok := asMap(after)
	if !bok && !aok {
		return true
	}
	bc := cloneMap(bm)
	ac := cloneMap(am)
	delete(bc, dropKey)
	delete(ac, dropKey)
	return canonicalizeEqual(bc, ac)
}

func asMap(v any) (map[string]any, bool) {
	if v == nil {
		return nil, false
	}
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	// json.Unmarshal into map[string]any is the expected shape; tolerate typed maps via re-marshal.
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, false
	}
	return m, true
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// IsSecretSentinel reports whether value means "keep previous secret".
func IsSecretSentinel(v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	s = strings.TrimSpace(s)
	return s == SecretSentinel || s == "***"
}

// FormatUnsupported explains why a patch cannot be applied by agent tools.
func FormatUnsupported(paths []string) string {
	if len(paths) == 0 {
		return "unsupported settings change; use the Settings UI (desktop keys live in cometline-desktop.json)"
	}
	return fmt.Sprintf("unsupported settings paths (use Settings UI / cometline-desktop.json): %s", strings.Join(paths, ", "))
}
