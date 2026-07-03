package skills

import (
	"context"
	"fmt"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/memory"
)

const synthesisMaxTokens = 2500

const synthesisSystemPrompt = "You identify reusable Agent Skill opportunities and output JSON only."

type synthesisResult struct {
	ShouldPropose bool   `json:"should_propose"`
	Name          string `json:"name"`
	Content       string `json:"content"`
	Reason        string `json:"reason"`
}

// SynthesisJob is the completed-job context needed to propose a skill draft.
type SynthesisJob struct {
	ID               string
	Description      string
	DefinitionOfDone string
	Progress         string
	WorkspacePath    string
}

// ProposeSkillFromJob asks an LLM whether a completed job should become a reusable skill draft.
func ProposeSkillFromJob(ctx context.Context, p cometsdk.Provider, model string, job SynthesisJob, outcomes []memory.ScoredMemory) error {
	if p == nil {
		return fmt.Errorf("provider is required")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("model is required")
	}

	var contextBlock strings.Builder
	for i, outcome := range outcomes {
		if strings.TrimSpace(outcome.Content) == "" {
			continue
		}
		fmt.Fprintf(&contextBlock, "%d. %s\n", i+1, strings.TrimSpace(outcome.Content))
	}

	prompt := fmt.Sprintf(`A completed autonomous job may have produced reusable operational knowledge.
Decide whether it should become an Agent Skill. Propose a draft only when the work contains reusable workflow steps, constraints, or examples that would help future agents. Skip one-off tasks.

If useful, return a complete SKILL.md with YAML frontmatter:
---
name: lowercase-hyphen-name
description: short trigger-focused description
---

The markdown body should include trigger scenarios, workflow steps, constraints, and examples where useful.
Return JSON only: {"should_propose":true|false,"name":"lowercase-hyphen-name","content":"full SKILL.md","reason":"short reason"}.

Completed job:
Description: %s
Definition of done: %s
Progress: %s
Workspace: %s

Recent task outcomes:
%s`, strings.TrimSpace(job.Description), strings.TrimSpace(job.DefinitionOfDone), strings.TrimSpace(job.Progress), strings.TrimSpace(job.WorkspacePath), strings.TrimSpace(contextBlock.String()))

	out, err := generateSynthesisResult(ctx, p, model, job, prompt)
	if err != nil {
		return err
	}
	if !out.ShouldPropose {
		logging.L().Info("skills.synthesis.skipped", "job_id", job.ID, "reason", strings.TrimSpace(out.Reason))
		return nil
	}
	name := strings.TrimSpace(out.Name)
	content := strings.TrimSpace(out.Content)
	if name == "" || content == "" {
		return fmt.Errorf("synthesis proposal missing name or content")
	}
	if err := WriteDraft(name, content, true); err != nil {
		return err
	}
	logging.L().Info("skills.synthesis.draft_written", "job_id", job.ID, "draft", name)
	return nil
}

func generateSynthesisResult(ctx context.Context, p cometsdk.Provider, model string, job SynthesisJob, prompt string) (synthesisResult, error) {
	var out synthesisResult
	zero := 0.0
	req := &cometsdk.Request{
		Model:  model,
		System: synthesisSystemPrompt,
		Messages: []cometsdk.Message{{
			Role:    cometsdk.RoleUser,
			Content: []cometsdk.Block{cometsdk.TextBlock{Text: prompt}},
		}},
		MaxTokens:   synthesisMaxTokens,
		Temperature: &zero,
	}
	if err := llm.GenerateJSON(ctx, p, req, &out); err != nil {
		if !shouldRetrySynthesisJSON(err) {
			return synthesisResult{}, err
		}
		logging.L().Warn("skills.synthesis.invalid_json_retry", "job_id", job.ID, "error", err)
		repairPrompt := prompt + `

Your previous response was invalid for the required JSON contract.
Respond again with exactly one valid JSON object and nothing else.
If this job is not reusable, set "should_propose" to false and leave "name" and "content" empty.`
		retryReq := &cometsdk.Request{
			Model:       model,
			System:      synthesisSystemPrompt,
			Messages:    []cometsdk.Message{{Role: cometsdk.RoleUser, Content: []cometsdk.Block{cometsdk.TextBlock{Text: repairPrompt}}}},
			MaxTokens:   synthesisMaxTokens,
			Temperature: &zero,
		}
		if retryErr := llm.GenerateJSON(ctx, p, retryReq, &out); retryErr != nil {
			return synthesisResult{}, retryErr
		}
	}
	return out, nil
}

func shouldRetrySynthesisJSON(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "GenerateJSON: parse response:") || strings.Contains(msg, "GenerateJSON: unmarshal:")
}
