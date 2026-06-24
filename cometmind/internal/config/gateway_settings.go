package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cometline/cometmind/internal/paths"
)

// DiscordGatewayPatch updates Discord gateway fields in cometline-settings.json.
type DiscordGatewayPatch struct {
	Enabled         *bool
	WorkspacePath   *string
	ProviderID      *string
	ModelID         *string
	BotTokenEnv     *string
	RequireMention  *bool
	AllowedUsers    []string
	AllowedChannels []string
}

// ValidateCurrentSettings validates the on-disk Cometline settings file.
func ValidateCurrentSettings() error {
	data, err := readSettingsBytes()
	if err != nil {
		return err
	}
	return ValidateCometlineSettingsJSON(data)
}

// LoadDiscordGateway returns the configured Discord gateway block.
func LoadDiscordGateway() (DiscordGatewayConfig, error) {
	raw, err := readCometlineSettingsJSON()
	if err != nil {
		return DiscordGatewayConfig{}, err
	}
	cfg, err := adaptCometlineSettings(raw)
	if err != nil {
		return DiscordGatewayConfig{}, err
	}
	return cfg.Gateway.Discord, nil
}

// UpdateDiscordGateway merges patch into cometmind.gateway.discord in settings JSON.
func UpdateDiscordGateway(patch DiscordGatewayPatch) error {
	data, err := readSettingsBytes()
	if err != nil {
		return err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse settings JSON: %w", err)
	}

	var cometmind map[string]json.RawMessage
	if raw, ok := doc["cometmind"]; ok {
		if err := json.Unmarshal(raw, &cometmind); err != nil {
			return fmt.Errorf("parse cometmind settings: %w", err)
		}
	} else {
		cometmind = map[string]json.RawMessage{}
	}

	var gateway map[string]json.RawMessage
	if raw, ok := cometmind["gateway"]; ok {
		if err := json.Unmarshal(raw, &gateway); err != nil {
			return fmt.Errorf("parse gateway settings: %w", err)
		}
	} else {
		gateway = map[string]json.RawMessage{}
	}

	var discord map[string]any
	if raw, ok := gateway["discord"]; ok {
		if err := json.Unmarshal(raw, &discord); err != nil {
			return fmt.Errorf("parse discord settings: %w", err)
		}
	} else {
		discord = map[string]any{}
	}

	applyDiscordPatch(discord, patch)

	discordJSON, err := json.Marshal(discord)
	if err != nil {
		return err
	}
	gateway["discord"] = discordJSON
	gatewayJSON, err := json.Marshal(gateway)
	if err != nil {
		return err
	}
	cometmind["gateway"] = gatewayJSON
	cometmindJSON, err := json.Marshal(cometmind)
	if err != nil {
		return err
	}
	doc["cometmind"] = cometmindJSON

	formatted, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	formatted = append(formatted, '\n')

	settingsPath, err := paths.SettingsPath()
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, formatted, 0o600)
}

func applyDiscordPatch(discord map[string]any, patch DiscordGatewayPatch) {
	if patch.Enabled != nil {
		discord["enabled"] = *patch.Enabled
	}
	if patch.WorkspacePath != nil {
		discord["workspacePath"] = strings.TrimSpace(*patch.WorkspacePath)
	}
	if patch.ProviderID != nil {
		discord["providerId"] = strings.TrimSpace(*patch.ProviderID)
	}
	if patch.ModelID != nil {
		discord["modelId"] = strings.TrimSpace(*patch.ModelID)
	}
	if patch.BotTokenEnv != nil {
		discord["botTokenEnv"] = strings.TrimSpace(*patch.BotTokenEnv)
	}
	if patch.RequireMention != nil {
		discord["requireMention"] = *patch.RequireMention
	}
	if patch.AllowedUsers != nil {
		discord["allowedUsers"] = patch.AllowedUsers
	}
	if patch.AllowedChannels != nil {
		discord["allowedChannels"] = patch.AllowedChannels
	}
}

func readSettingsBytes() ([]byte, error) {
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
	return data, nil
}
