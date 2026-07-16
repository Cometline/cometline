package settingsapply_test

import (
	"testing"

	"github.com/cometline/cometmind/internal/settingsapply"
)

func TestClassifyReloadAndGateway(t *testing.T) {
	before := map[string]any{
		"providers": []any{
			map[string]any{"id": "openai", "apiKey": "sk-old"},
		},
		"cometmind": map[string]any{
			"memory": map[string]any{"enabled": true},
			"gateway": map[string]any{
				"discord": map[string]any{"enabled": false, "botToken": ""},
			},
		},
	}
	afterMemory := deepCopy(before)
	afterMemory["cometmind"].(map[string]any)["memory"] = map[string]any{"enabled": false}
	res, err := settingsapply.Classify(before, afterMemory)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != settingsapply.ApplyReload || res.Gateway || len(res.Unsupported) != 0 {
		t.Fatalf("memory change: %+v", res)
	}

	afterDiscord := deepCopy(before)
	afterDiscord["cometmind"].(map[string]any)["gateway"] = map[string]any{
		"discord": map[string]any{"enabled": true, "botToken": "tok"},
	}
	res, err = settingsapply.Classify(before, afterDiscord)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != settingsapply.ApplyGateway || !res.Gateway {
		t.Fatalf("gateway-only: %+v", res)
	}

	afterBoth := deepCopy(before)
	afterBoth["cometmind"].(map[string]any)["memory"] = map[string]any{"enabled": false}
	afterBoth["cometmind"].(map[string]any)["gateway"] = map[string]any{
		"discord": map[string]any{"enabled": true, "botToken": "tok"},
	}
	res, err = settingsapply.Classify(before, afterBoth)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != settingsapply.ApplyReload || !res.Gateway {
		t.Fatalf("memory+gateway: %+v", res)
	}

	afterHost := deepCopy(before)
	afterHost["cometmind"].(map[string]any)["host"] = "0.0.0.0"
	res, err = settingsapply.Classify(before, afterHost)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != settingsapply.ApplyUnsupported || len(res.Unsupported) == 0 {
		t.Fatalf("host change: %+v", res)
	}

	afterAppearance := deepCopy(before)
	afterAppearance["appearance"] = map[string]any{"heroComposer": map[string]any{"presetId": "rose"}}
	res, err = settingsapply.Classify(before, afterAppearance)
	if err != nil {
		t.Fatal(err)
	}
	if res.Action != settingsapply.ApplyUnsupported {
		t.Fatalf("appearance change: %+v", res)
	}
}

func TestRedactAndMergePreserveSecrets(t *testing.T) {
	doc := map[string]any{
		"providers": []any{
			map[string]any{"id": "openai", "apiKey": "sk-secret", "enabled": true},
		},
		"cometmind": map[string]any{
			"memory": map[string]any{
				"embedding": map[string]any{"apiKey": "emb-secret", "model": "m"},
			},
			"gateway": map[string]any{
				"discord": map[string]any{"botToken": "discord-tok", "enabled": true},
			},
		},
	}
	redacted := settingsapply.RedactSecrets(doc)
	providers := redacted["providers"].([]any)
	p0 := providers[0].(map[string]any)
	if p0["apiKey"] != settingsapply.SecretSentinel {
		t.Fatalf("apiKey not redacted: %v", p0["apiKey"])
	}
	if p0["apiKey_has_value"] != true {
		t.Fatalf("apiKey_has_value=%v", p0["apiKey_has_value"])
	}

	patch := map[string]any{
		"providers": []any{
			map[string]any{"id": "openai", "apiKey": settingsapply.SecretSentinel, "enabled": false},
		},
		"cometmind": map[string]any{
			"memory": map[string]any{
				"embedding": map[string]any{"apiKey": "***", "model": "m2"},
			},
		},
	}
	merged := settingsapply.MergePatch(doc, patch)
	settingsapply.RestoreSecretsInProviders(merged, doc)
	mp := merged["providers"].([]any)[0].(map[string]any)
	if mp["apiKey"] != "sk-secret" {
		t.Fatalf("apiKey not preserved: %v", mp["apiKey"])
	}
	if mp["enabled"] != false {
		t.Fatalf("enabled not patched: %v", mp["enabled"])
	}
	emb := merged["cometmind"].(map[string]any)["memory"].(map[string]any)["embedding"].(map[string]any)
	if emb["apiKey"] != "emb-secret" {
		t.Fatalf("embedding apiKey not preserved: %v", emb["apiKey"])
	}
	if emb["model"] != "m2" {
		t.Fatalf("embedding model not patched: %v", emb["model"])
	}
}

func deepCopy(in map[string]any) map[string]any {
	return settingsapply.MergePatch(map[string]any{}, in)
}
