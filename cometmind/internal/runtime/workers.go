package runtime

import (
	"context"

	"github.com/Cometline/cometline/cometmind/internal/agent"
	"github.com/Cometline/cometline/cometmind/internal/autonomy"
	"github.com/Cometline/cometline/cometmind/internal/event"
	"github.com/Cometline/cometline/cometmind/internal/inboxworker"
	"github.com/Cometline/cometline/cometmind/internal/jobs"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/tools"
)

// StartAutonomousJobWorker starts the background worker that claims and
// executes ready jobs without a human opening a chat session first.
func (r *Runtime) StartAutonomousJobWorker(
	ctx context.Context,
	guard autonomy.RunGuard,
	onRunState func(sessionID string, running bool),
	onEvent func(sessionID string, ev event.Event),
) {
	if r == nil || r.Jobs == nil || r.Sessions == nil {
		return
	}
	w := &autonomy.Worker{
		Jobs:     r.Jobs,
		Sessions: r.Sessions,
		Memory:   r.Memory,
		NewRunner: func(sess session.Session, workspacePath string, maxSteps int) (*agent.Runner, error) {
			mode, err := session.ParseAgentMode(sess.AgentMode)
			if err != nil {
				return nil, err
			}
			return r.runnerFor(sess, workspacePath, RunnerOptions{MaxSteps: maxSteps, AgentMode: mode})
		},
		Guard:             guard,
		OnRunState:        onRunState,
		OnEvent:           onEvent,
		Config:            r.Config.EffectiveAutonomousJobsSettings(),
		DefaultModelID:    r.autonomyModelID(),
		DefaultProviderID: r.autonomyProviderID(),
	}
	r.autonomyWorker = w
	go w.Run(ctx)
}

// StartInboxWorker starts the background loop that internalizes user inbox replies.
func (r *Runtime) StartInboxWorker(ctx context.Context, guard inboxworker.RunGuard) {
	if r == nil || r.Inbox == nil || r.Sessions == nil {
		return
	}
	w := &inboxworker.Worker{
		Inbox:             r.Inbox,
		Sessions:          r.Sessions,
		Jobs:              r.Jobs,
		Memory:            r.Memory,
		Events:            r.Events,
		Guard:             guard,
		Config:            r.Config.EffectiveInboxSettings(),
		DefaultModelID:    r.autonomyModelID(),
		DefaultProviderID: r.autonomyProviderID(),
		NewRunner: func(sess session.Session, workspacePath string, registry *tools.Registry, maxSteps int) (*agent.Runner, error) {
			return r.RunnerForInbox(sess, workspacePath, registry, maxSteps)
		},
		RegistryOptions: func(sess session.Session, workspacePath string) tools.RegistryOptions {
			mode, err := session.ParseAgentMode(sess.AgentMode)
			if err != nil {
				mode = session.AgentModeAuto
			}
			return r.toolRegistryOptions(r.SkillsForWorkspace(workspacePath), sess.ID, jobs.PlatformDesktop, "", mode)
		},
	}
	r.inboxWorker = w
	go w.Run(ctx)
}

func (r *Runtime) autonomyProviderID() string {
	if r == nil || r.Config == nil {
		return ""
	}
	providerID, _ := r.Config.ResolveRoleLLM(r.Config.Autonomy.ProviderID, r.Config.Autonomy.ModelID)
	return providerID
}

func (r *Runtime) autonomyModelID() string {
	if r == nil || r.Config == nil {
		return ""
	}
	_, modelID := r.Config.ResolveRoleLLM(r.Config.Autonomy.ProviderID, r.Config.Autonomy.ModelID)
	return modelID
}
