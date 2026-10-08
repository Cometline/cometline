package tools

import (
	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
	"github.com/Cometline/cometline/cometmind/internal/tools/fsops"
	"github.com/Cometline/cometline/cometmind/internal/tools/inbox"
	"github.com/Cometline/cometline/cometmind/internal/tools/jobs"
	"github.com/Cometline/cometline/cometmind/internal/tools/mcp"
	"github.com/Cometline/cometline/cometmind/internal/tools/media"
	"github.com/Cometline/cometline/cometmind/internal/tools/memory"
	"github.com/Cometline/cometline/cometmind/internal/tools/settings"
	"github.com/Cometline/cometline/cometmind/internal/tools/skills"
	childagent "github.com/Cometline/cometline/cometmind/internal/tools/subagent"
	"github.com/Cometline/cometline/cometmind/internal/tools/web"
)

// Exported tool types stay addressable from this package so the registry and
// existing callers keep the same names after the family split.
type (
	ReadFile      = fsops.ReadFile
	EditFile      = fsops.EditFile
	WriteFile     = fsops.WriteFile
	WikiEditFile  = fsops.WikiEditFile
	WikiWriteFile = fsops.WikiWriteFile
	ListDir       = fsops.ListDir
	Glob          = fsops.Glob
	Grep          = fsops.Grep
	RunCommand    = fsops.RunCommand

	WebFetch  = web.WebFetch
	WebSearch = web.WebSearch

	PresentImage       = media.PresentImage
	PresentImageURL    = media.PresentImageURL
	CaptureScreenshot  = media.CaptureScreenshot
	ListCaptureTargets = media.ListCaptureTargets
	GenerateImage      = media.GenerateImage
	GenerateVideo      = media.GenerateVideo

	LoadSkill         = skills.LoadSkill
	ReadSkillFile     = skills.ReadSkillFile
	WriteSkillDraft   = skills.WriteSkillDraft
	ListSkillDrafts   = skills.ListSkillDrafts
	ReadSkillDraft    = skills.ReadSkillDraft
	WriteSkill        = skills.WriteSkill
	ReviewWriteSkill  = skills.ReviewWriteSkill
	PromoteSkillDraft = skills.PromoteSkillDraft

	DelegateCodingTask = childagent.DelegateCodingTask
	SpawnGeneralAgent  = childagent.SpawnGeneralAgent
	WaitSubagents      = childagent.WaitSubagents

	RecallTaskOutcome = memory.RecallTaskOutcome
	ListMemories      = memory.ListMemories
	SearchMemories    = memory.SearchMemories
	CreateMemory      = memory.CreateMemory
	UpdateMemory      = memory.UpdateMemory
	DeleteMemory      = memory.DeleteMemory

	JobsDeps        = jobs.JobsDeps
	InboxDeps       = inbox.InboxDeps
	SettingsRuntime = settings.SettingsRuntime
)

// RegisterJobTools adds job and scheduled-job tools when deps are configured.
func RegisterJobTools(r *Registry, deps JobsDeps) {
	if r == nil {
		return
	}
	jobs.AddJobTools(r.Add, deps)
}

// JobPromptIndex returns system prompt guidance for job tools.
func JobPromptIndex(sessionWorkspace, platform string) string {
	return jobs.JobPromptIndex(sessionWorkspace, platform)
}

// RegisterInboxLeaveTool adds leave_inbox_message to a parent or autonomy registry.
func RegisterInboxLeaveTool(r *Registry, deps InboxDeps) {
	if r == nil {
		return
	}
	inbox.AddLeaveTool(r.Add, deps)
}

// AddMCPTools registers MCP management tools and the live remote tool list.
func AddMCPTools(add func(Tool), mgr *mcppkg.Manager) {
	mcp.AddMCPTools(add, mgr)
}

// AddSettingsTools registers the parent-agent settings tools.
func AddSettingsTools(add func(Tool), runtime SettingsRuntime) {
	settings.AddSettingsTools(add, runtime)
}
