package runtime

import (
	"context"
	"strings"
	"sync"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/cometmind/internal/jobs"
	"github.com/cometline/cometmind/internal/logging"
	"github.com/cometline/cometmind/internal/memory"
	"github.com/cometline/cometmind/internal/skills"
)

type skillSynthesisNotifier struct {
	provider cometsdk.Provider
	model    string
	memory   *memory.Service
	sem      chan struct{}
	once     sync.Once
}

func (n *skillSynthesisNotifier) init() {
	n.once.Do(func() {
		if n.sem == nil {
			n.sem = make(chan struct{}, 1)
		}
	})
}

func (n *skillSynthesisNotifier) OnJobEvent(ctx context.Context, job jobs.Job, action, detail string) {
	if n == nil || action != jobs.EventCompleted || n.provider == nil || strings.TrimSpace(n.model) == "" {
		return
	}
	n.init()
	select {
	case n.sem <- struct{}{}:
	default:
		logging.L().Info("skills.synthesis.skipped", "job_id", job.ID, "reason", "busy")
		return
	}
	go func() {
		defer func() { <-n.sem }()
		var outcomes []memory.ScoredMemory
		if n.memory != nil {
			var err error
			outcomes, err = n.memory.RecentTaskOutcomes(ctx, 3)
			if err != nil {
				logging.L().Warn("skills.synthesis.outcomes_failed", "job_id", job.ID, "error", err)
			}
		}
		input := skills.SynthesisJob{
			ID:               job.ID,
			Description:      job.Description,
			DefinitionOfDone: job.DefinitionOfDone,
			Progress:         job.Progress,
			WorkspacePath:    job.WorkspacePath,
		}
		if err := skills.ProposeSkillFromJob(ctx, n.provider, n.model, input, outcomes); err != nil {
			logging.L().Warn("skills.synthesis.failed", "job_id", job.ID, "error", err)
		}
	}()
}
