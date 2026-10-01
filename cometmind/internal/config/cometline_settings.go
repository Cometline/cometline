package config

import (
	"encoding/json"
	"fmt"
	"os"
)

func loadCometlineSettingsJSON(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw cometlineSettingsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse cometline settings: %w", err)
	}
	return adaptCometlineSettings(raw)
}

// ValidateCometlineSettingsJSON checks whether data can be used as Cometline's
// saved settings file without applying environment overrides.
func ValidateCometlineSettingsJSON(data []byte) error {
	var raw cometlineSettingsJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse cometline settings: %w", err)
	}
	_, err := adaptCometlineSettings(raw)
	return err
}

func writeMinimalCometlineSettingsJSON(path string, def *Config) error {
	raw := cometlineSettingsJSON{
		Providers: []cometlineProviderJSON{
			{
				ID:            def.DefaultProviderID,
				Name:          def.DefaultProviderID,
				Method:        def.DefaultProviderID,
				Enabled:       true,
				BaseURL:       def.BaseURL,
				EnabledModels: []string{def.DefaultModelID},
				Models:        []string{def.DefaultModelID},
				SelectedModel: def.DefaultModelID,
			},
		},
		DefaultProviderID: def.DefaultProviderID,
		DefaultModelID:    def.DefaultModelID,
		Cometmind: cometlineCometmindJSON{
			SystemPromptPath: def.SystemPromptPath,
			ACP: cometlineACPJSON{
				Enabled:        boolPtr(false),
				DefaultHarness: "opencode",
			},
			Skills: cometlineSkillsJSON{
				Enabled:         true,
				IncludeOpenCode: true,
				IncludeClaude:   true,
			},
		},
	}
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
