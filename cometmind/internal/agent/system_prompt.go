package agent

import "strings"

func (r *Runner) systemPrompt() string {
	base := strings.TrimSpace(r.SystemPrompt)
	if base == "" {
		base = DefaultSystemPrompt()
	}
	if strings.TrimSpace(r.SkillIndex) == "" && strings.TrimSpace(r.JobIndex) == "" {
		return base
	}
	return base + r.SkillIndex + r.JobIndex
}

func (r *Runner) buildSystemPrompt(contextSummary string, maxTokens int) string {
	base := r.systemPrompt()
	var parts []string
	if block := FormatSummaryPromptBlock(contextSummary); block != "" {
		parts = append(parts, block)
	}
	if block := FormatOutputBudgetPromptBlock(maxTokens); block != "" {
		parts = append(parts, block)
	}
	if block := FormatAgentModePrompt(r.AgentMode); block != "" {
		parts = append(parts, block)
	}
	if len(parts) == 0 {
		return base
	}
	return base + "\n\n" + strings.Join(parts, "\n\n")
}
