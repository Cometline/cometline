package skills

import "strings"

// ExpandCreateSkillCommand turns a /create-skill slash invocation into agent instructions.
func ExpandCreateSkillCommand(userText string) string {
	rest := strings.TrimSpace(userText)
	prompt := `Draft a new Agent Skill for CometMind.

Target directory: ~/.cometmind/skill-drafts/{skill-name}/

Requirements:
1. Use the ` + "`write_skill_draft`" + ` tool to create SKILL.md (YAML frontmatter with name and description, then markdown body).
2. If the user did not provide a detailed request, infer the draft from the current session context or the current completed job being discussed.
3. Follow Agent Skills conventions: clear trigger scenarios, step-by-step workflow, examples, and constraints.
4. Skill names use lowercase letters, numbers, and hyphens only.
5. If there is not enough reusable signal yet, explain that instead of forcing a draft.
6. After writing, summarize the draft name, what it does, and that it can be edited or promoted from Skill Drafts.`
	if rest != "" {
		prompt += "\n\nUser request:\n" + rest
	}
	return prompt
}
