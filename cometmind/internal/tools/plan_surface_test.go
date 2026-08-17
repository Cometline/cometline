package tools

import (
	"context"
	"testing"

	"github.com/cometline/cometmind/internal/acp"
	"github.com/cometline/cometmind/internal/event"
	"github.com/cometline/cometmind/internal/session"
	"github.com/cometline/cometmind/internal/skills"
	"github.com/cometline/cometmind/internal/subagent"
)

// fullPlanOptions wires every optional capability so surface differences are
// attributable to AgentMode, not missing dependencies.
func fullPlanOptions(mode session.AgentMode) RegistryOptions {
	skillReg := skills.Discover("", skills.Config{Enabled: true})
	cfg := acp.DefaultHarnessConfig(acp.HarnessOpenCode)
	cfg.Enabled = true
	runnerFactory := ChildRunnerFactory(func(child session.Session, workspaceRoot string, maxSteps int, mode SubagentMode) (AgentLoopRunner, error) {
		return nil, context.DeadlineExceeded
	})
	return RegistryOptions{
		Sessions:        &session.Service{},
		ACP:             cfg,
		Skills:          &skillReg,
		Orchestrator:    subagent.NewOrchestrator(5),
		RunnerFactory:   runnerFactory,
		AgentMode:       mode,
		MemoryEvents:    event.NewHub(),
		SettingsRuntime: nil,
	}
}

func planRegistry(t *testing.T) *Registry {
	t.Helper()
	return NewRegistry(t.TempDir(), fullPlanOptions(session.AgentModePlan))
}

func TestNewRegistryPlanSurfaceIsReadOnlyAllowlist(t *testing.T) {
	r := planRegistry(t)

	included := []string{
		"read_file", "list_dir", "glob", "grep",
		"web_fetch", "web_search",
		"present_image", "present_image_url",
		"load_skill", "read_skill_file",
		"spawn_general_agent", "wait_subagents",
	}
	for _, name := range included {
		if !r.Has(name) {
			t.Errorf("plan registry missing %q", name)
		}
	}

	excluded := []string{
		"edit_file", "write_file",
		"run_command",
		"delegate_coding_task",
		"write_skill", "write_skill_draft", "promote_skill_draft",
		"list_settings", "get_settings", "patch_settings",
		"list_jobs", "create_job", "complete_job", "claim_job",
		"recall_task_outcome", "list_memories", "create_memory", "update_memory", "delete_memory",
		"leave_inbox_message",
		"list_mcp_servers", "reconnect_mcp_server",
		"generate_image", "generate_video",
	}
	for _, name := range excluded {
		if r.Has(name) {
			t.Errorf("plan registry should not include %q", name)
		}
	}
}

func TestNewRegistryAutoKeepsFullSurface(t *testing.T) {
	r := NewRegistry(t.TempDir(), fullPlanOptions(session.AgentModeAuto))
	for _, name := range []string{
		"read_file", "edit_file", "write_file", "list_dir", "glob", "grep", "run_command",
		"spawn_general_agent", "wait_subagents",
	} {
		if !r.Has(name) {
			t.Errorf("auto registry missing %q", name)
		}
	}
}

func TestNewRegistryEmptyModeDefaultsToAutoSurface(t *testing.T) {
	r := NewRegistry(t.TempDir())
	for _, name := range []string{"run_command", "edit_file", "write_file", "read_file", "grep"} {
		if !r.Has(name) {
			t.Errorf("default registry missing %q (must preserve existing behavior)", name)
		}
	}
}
