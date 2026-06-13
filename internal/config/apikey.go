package config

import (
	"fmt"
	"os"
)

// ProviderAPIKey resolves the API key for the configured provider from environment variables.
func ProviderAPIKey(c *Config) (string, error) {
	switch c.Provider {
	case ProviderAnthropic:
		k := firstNonEmpty(os.Getenv("ANTHROPIC_API_KEY"), os.Getenv("COMETMIND_API_KEY"))
		if k == "" {
			return "", fmt.Errorf("ANTHROPIC_API_KEY or COMETMIND_API_KEY is not set")
		}
		return k, nil
	case ProviderOpenAI:
		k := firstNonEmpty(os.Getenv("OPENAI_API_KEY"), os.Getenv("COMETMIND_API_KEY"))
		if k == "" {
			return "", fmt.Errorf("OPENAI_API_KEY or COMETMIND_API_KEY is not set")
		}
		return k, nil
	default:
		return "", fmt.Errorf("unknown provider %q (use %q or %q)", c.Provider, ProviderAnthropic, ProviderOpenAI)
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
