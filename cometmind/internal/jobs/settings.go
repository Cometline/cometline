package jobs

import "github.com/Cometline/cometline/cometmind/internal/config"

// Settings is the name jobs callers use for config.JobSettings.
type Settings = config.JobSettings

// DefaultSettings returns the default job settings.
func DefaultSettings() Settings {
	return config.DefaultJobSettings()
}
