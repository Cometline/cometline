package settingsapply

import "testing"

func TestRedactMCPEnvAndHeaders(t *testing.T) {
	doc := map[string]any{
		"cometmind": map[string]any{
			"mcp": map[string]any{
				"servers": []any{
					map[string]any{
						"id": "linear",
						"env": map[string]any{
							"TOKEN": "secret-env",
						},
						"headers": map[string]any{
							"Authorization": "Bearer tok",
						},
					},
				},
			},
		},
	}
	redacted := RedactSecrets(doc)
	cm := redacted["cometmind"].(map[string]any)
	mcp := cm["mcp"].(map[string]any)
	servers := mcp["servers"].([]any)
	srv := servers[0].(map[string]any)
	env := srv["env"].(map[string]any)
	if env["TOKEN"] != SecretSentinel || env["TOKEN_has_value"] != true {
		t.Fatalf("env redaction: %v", env)
	}
	headers := srv["headers"].(map[string]any)
	if headers["Authorization"] != SecretSentinel {
		t.Fatalf("headers redaction: %v", headers)
	}
}

func TestRestoreMCPSecretsStripsHasValueKeys(t *testing.T) {
	before := map[string]any{
		"cometmind": map[string]any{
			"mcp": map[string]any{
				"servers": []any{
					map[string]any{
						"id": "sentry",
						"env": map[string]any{
							"API_KEY": "real-key",
						},
					},
				},
			},
		},
	}
	merged := map[string]any{
		"cometmind": map[string]any{
			"mcp": map[string]any{
				"servers": []any{
					map[string]any{
						"id": "sentry",
						"env": map[string]any{
							"API_KEY":           SecretSentinel,
							"API_KEY_has_value": true,
						},
					},
				},
			},
		},
	}
	RestoreMCPSecretsInCometmind(merged, before)
	cm := merged["cometmind"].(map[string]any)
	mcp := cm["mcp"].(map[string]any)
	servers := mcp["servers"].([]any)
	srv := servers[0].(map[string]any)
	env := srv["env"].(map[string]any)
	if env["API_KEY"] != "real-key" {
		t.Fatalf("expected preserved env secret, got %v", env["API_KEY"])
	}
	if _, ok := env["API_KEY_has_value"]; ok {
		t.Fatal("API_KEY_has_value should not persist in env map")
	}
}

func TestRestoreMCPSecretsPreservesEnv(t *testing.T) {
	before := map[string]any{
		"cometmind": map[string]any{
			"mcp": map[string]any{
				"servers": []any{
					map[string]any{
						"id": "sentry",
						"env": map[string]any{
							"API_KEY": "real-key",
						},
					},
				},
			},
		},
	}
	merged := map[string]any{
		"cometmind": map[string]any{
			"mcp": map[string]any{
				"servers": []any{
					map[string]any{
						"id": "sentry",
						"env": map[string]any{
							"API_KEY": SecretSentinel,
						},
					},
				},
			},
		},
	}
	RestoreMCPSecretsInCometmind(merged, before)
	cm := merged["cometmind"].(map[string]any)
	mcp := cm["mcp"].(map[string]any)
	servers := mcp["servers"].([]any)
	srv := servers[0].(map[string]any)
	env := srv["env"].(map[string]any)
	if env["API_KEY"] != "real-key" {
		t.Fatalf("expected preserved env secret, got %v", env["API_KEY"])
	}
}
