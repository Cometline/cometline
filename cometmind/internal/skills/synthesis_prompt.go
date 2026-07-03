package skills

import (
	"fmt"
	"strings"

	"github.com/cometline/cometmind/internal/memory"
)

const synthesisSystemPrompt = "You identify reusable Agent Skill opportunities from completed work. Prefer skipping over proposing a weak skill. Output JSON only."

const synthesisDecisionPolicy = `A completed autonomous job may have produced reusable operational knowledge.
Your job is to decide whether it is strong enough to promote into an Agent Skill draft.

Propose a skill only when ALL of these are true:
1. The lesson is a reusable multi-step procedure, decision process, or operational workflow.
2. The job shows concrete verification or a clear definition of success, not just an unverified idea.
3. The draft can teach future agents how to avoid at least one failure mode, dead end, or non-obvious constraint.

Do NOT propose a skill when the result is mostly:
- a one-off change or isolated fix
- a simple fact, preference, or project note
- generic advice with no repo-specific workflow
- speculative guidance that was not clearly validated

Think like a strict editor. It is better to skip than to create a vague or noisy skill.`

const synthesisSkillFormatGuide = `If you do propose a skill, return a complete SKILL.md with YAML frontmatter:
---
name: lowercase-hyphen-name
description: short trigger-focused description
---

Write the markdown body as a compact, reusable operating guide.
Prefer this structure when relevant:
- What this skill is for
- Trigger signals / when to use it
- Preconditions or required context
- Step-by-step workflow
- Constraints, pitfalls, and dead ends to avoid
- Verification or definition of done
- Short examples`

const synthesisSafetyRules = `Authoring rules:
- Capture the reusable procedure, not a one-time summary.
- Include concrete commands, checks, and file paths when they are part of the repeatable workflow.
- Include at least one failure mode, dead end, or subtle constraint when available.
- Do not include secrets, tokens, passwords, or copied credentials.
- Keep the skill concise and practical; do not add filler.`

const synthesisJSONContract = `Return JSON only: {"should_propose":true|false,"name":"lowercase-hyphen-name","content":"full SKILL.md","reason":"short reason"}.
If the job is not strong enough, set should_propose to false and leave name/content empty.`

func buildSynthesisPrompt(job SynthesisJob, outcomes []memory.ScoredMemory) string {
	return fmt.Sprintf(`%s

%s

%s

%s

Completed job:
Description: %s
Definition of done: %s
Progress: %s
Workspace: %s

Recent task outcomes:
%s`,
		synthesisDecisionPolicy,
		synthesisSkillFormatGuide,
		synthesisSafetyRules,
		synthesisJSONContract,
		strings.TrimSpace(job.Description),
		strings.TrimSpace(job.DefinitionOfDone),
		strings.TrimSpace(job.Progress),
		strings.TrimSpace(job.WorkspacePath),
		formatSynthesisOutcomes(outcomes),
	)
}

func formatSynthesisOutcomes(outcomes []memory.ScoredMemory) string {
	var contextBlock strings.Builder
	for i, outcome := range outcomes {
		if strings.TrimSpace(outcome.Content) == "" {
			continue
		}
		fmt.Fprintf(&contextBlock, "%d. %s\n", i+1, strings.TrimSpace(outcome.Content))
	}
	return strings.TrimSpace(contextBlock.String())
}
