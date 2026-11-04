package agent

import (
	"strings"

	toolmcp "github.com/Cometline/cometline/cometmind/internal/tools/mcp"
)

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

func (r *Runner) buildSystemPrompt(s *turnState, contextSummary string, maxTokens int) string {
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
	if block := r.mcpServerPrompt(s); block != "" {
		parts = append(parts, block)
	}
	if len(parts) == 0 {
		return base
	}
	return base + "\n\n" + strings.Join(parts, "\n\n")
}

// mcpServerPrompt tells the model which MCP servers are connected without
// putting their schemas on the tool list. Unchanged catalogs return nothing.
func (r *Runner) mcpServerPrompt(s *turnState) string {
	if r == nil || r.Registry == nil || s == nil {
		return ""
	}
	mgr := r.Registry.MCP()
	if mgr == nil {
		return ""
	}
	reminder := toolmcp.ServerReminder(mgr.Catalog(), s.mcpCatalogFingerprint)
	s.mcpCatalogFingerprint = reminder.Fingerprint
	if !reminder.Changed {
		return ""
	}
	return reminder.Text
}
