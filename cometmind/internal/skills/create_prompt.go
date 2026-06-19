package skills

import "strings"

// ExpandCreateSkillCommand turns a /create-skill slash invocation into agent instructions.
func ExpandCreateSkillCommand(userText string) string {
	rest := strings.TrimSpace(userText)
	prompt := `Create a new Agent Skill for CometMind.

Target directory: ~/.cometmind/skills/{skill-name}/

Requirements:
1. Use the ` + "`write_skill`" + ` tool to create SKILL.md (YAML frontmatter with name and description, then markdown body).
2. Follow Agent Skills conventions: clear trigger scenarios, step-by-step workflow, examples, and constraints.
3. Skill names use lowercase letters, numbers, and hyphens only.
4. If the request is vague, ask up to two clarifying questions before writing.
5. After writing, summarize the skill name, what it does, and how to invoke it with /{skill-name} in Cometline.`
	if rest != "" {
		prompt += "\n\nUser request:\n" + rest
	}
	return prompt
}
