package config

// GeneralSubagentStepBudgetNumerator and GeneralSubagentStepBudgetDenominator
// reserve one fifth of the main budget for the parent when a child budget is unset.
const (
	GeneralSubagentStepBudgetNumerator   = 4
	GeneralSubagentStepBudgetDenominator = 5
)

// SubagentSettings controls parallel subagent orchestration.
type SubagentSettings struct {
	GeneralMaxSteps        int `json:"general_max_steps" mapstructure:"general_max_steps"`
	MaxConcurrentPerParent int `json:"max_concurrent_per_parent" mapstructure:"max_concurrent_per_parent"`
}

func defaultSubagentSettings() SubagentSettings {
	return SubagentSettings{
		GeneralMaxSteps:        0, // derived from main max_steps when unset
		MaxConcurrentPerParent: 5,
	}
}

// GeneralMaxStepsFromMain returns the default step budget for general subagents.
func GeneralMaxStepsFromMain(mainMaxSteps int) int {
	if mainMaxSteps <= 0 {
		mainMaxSteps = Defaults().MaxSteps
	}
	steps := mainMaxSteps * GeneralSubagentStepBudgetNumerator / GeneralSubagentStepBudgetDenominator
	if steps < 1 {
		return 1
	}
	return steps
}

// EffectiveSubagentSettings returns subagent settings with defaults applied.
func (c *Config) EffectiveSubagentSettings() SubagentSettings {
	s := c.Subagent
	mainSteps := c.MaxSteps
	if mainSteps <= 0 {
		mainSteps = Defaults().MaxSteps
	}
	if s.GeneralMaxSteps <= 0 {
		s.GeneralMaxSteps = GeneralMaxStepsFromMain(mainSteps)
	}
	if s.MaxConcurrentPerParent <= 0 {
		s.MaxConcurrentPerParent = defaultSubagentSettings().MaxConcurrentPerParent
	}
	return s
}
