package skills

import (
	"context"
	"fmt"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/llm"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/usage"
)

const synthesisMaxTokens = 2500

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
	return ProposeSkillFromJobRecorded(ctx, p, model, job, outcomes, nil, "")
}

// ProposeSkillFromJobRecorded is ProposeSkillFromJob plus optional spend recording.
func ProposeSkillFromJobRecorded(ctx context.Context, p cometsdk.Provider, model string, job SynthesisJob, outcomes []memory.ScoredMemory, rec usage.Recorder, workspaceID string) error {
	if p == nil {
		return fmt.Errorf("provider is required")
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return fmt.Errorf("model is required")
	}

	prompt := buildSynthesisPrompt(job, outcomes)

	out, err := generateSynthesisResult(ctx, p, model, job, prompt, rec, workspaceID)
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
	description := SkillMarkdownDescription(content)
	related, err := FindRelatedManagedSkills(name, description)
	if err != nil {
		return err
	}
	// Same-name draft may be refreshed; any other blocking overlap skips (no force on auto path).
	related = FilterSelfDraftOverwrite(related, name, true)
	if ShouldBlockOverlap(related) {
		logging.L().Info("skills.synthesis.skipped", "job_id", job.ID, "reason", "overlap:"+FormatOverlapBlock(related))
		return nil
	}
	if err := WriteDraft(name, content, true); err != nil {
		return err
	}
	logging.L().Info("skills.synthesis.draft_written", "job_id", job.ID, "draft", name)
	return nil
}

func generateSynthesisResult(ctx context.Context, p cometsdk.Provider, model string, job SynthesisJob, prompt string, rec usage.Recorder, workspaceID string) (synthesisResult, error) {
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
	if tok, err := llm.GenerateJSON(ctx, p, req, &out); err != nil {
		recordSkillUsage(ctx, rec, p, model, workspaceID, tok)
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
		retryTok, retryErr := llm.GenerateJSON(ctx, p, retryReq, &out)
		recordSkillUsage(ctx, rec, p, model, workspaceID, retryTok)
		if retryErr != nil {
			return synthesisResult{}, retryErr
		}
		return out, nil
	} else {
		recordSkillUsage(ctx, rec, p, model, workspaceID, tok)
	}
	return out, nil
}

func recordSkillUsage(ctx context.Context, rec usage.Recorder, p cometsdk.Provider, model, workspaceID string, u cometsdk.TokenUsage) {
	if rec == nil {
		return
	}
	providerID := ""
	if p != nil {
		providerID = p.ID()
	}
	if err := rec.Record(ctx, usage.Event{
		WorkspaceID: workspaceID,
		ProviderID:  providerID,
		ModelID:     model,
		CallKind:    usage.KindSkillSynthesis,
		Usage:       u,
	}); err != nil {
		logging.L().Warn("usage.record_failed", "kind", usage.KindSkillSynthesis, "model", model, "error", err)
	}
}

func shouldRetrySynthesisJSON(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "GenerateJSON: parse response:") || strings.Contains(msg, "GenerateJSON: unmarshal:")
}
