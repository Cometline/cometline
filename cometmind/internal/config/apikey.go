package config

import (
	"fmt"
	"os"
)

// ProviderAPIKey resolves the API key for the configured provider from the
// provider entry if one exists, otherwise from environment variables for
// legacy single-provider configs.
func ProviderAPIKey(c *Config, entry *ProviderEntry, method string) (string, error) {
	// Multi-provider entries carry their own key (which may be empty for
	// unauthenticated endpoints). Do not fall back to env here because env
	// overrides are scoped to the active provider.
	if entry != nil {
		return entry.APIKey, nil
	}

	switch method {
	case ProviderAnthropic:
		k := firstNonEmpty(os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("COMETMIND_API_KEY"))
		if k == "" {
			return "", fmt.Errorf("ANTHROPIC_API_KEY or COMETMIND_API_KEY is not set")
		}
		return k, nil
	case ProviderOpenAI, ProviderOpenAICompat, ProviderOpencodeGo:
		k := firstNonEmpty(os.Getenv("OPENAI_API_KEY"), os.Getenv("COMETMIND_API_KEY"))
		if k == "" {
			return "", fmt.Errorf("OPENAI_API_KEY or COMETMIND_API_KEY is not set")
		}
		return k, nil
	case ProviderCodex:
		return "", nil
	default:
		return "", fmt.Errorf("unknown provider %q (use %q, %q, %q, %q, or %q)", method, ProviderAnthropic, ProviderOpenAI, ProviderOpenAICompat, ProviderOpencodeGo, ProviderCodex)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
