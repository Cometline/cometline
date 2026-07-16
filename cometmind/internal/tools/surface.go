package tools

// ToolSurface is the capability policy for a registry: which tool families
// are exposed. Parent, research, and coding registries share this module so
// mode is not duplicated as stringly agentName / registry lists.
type ToolSurface struct {
	Read     bool // read_file, list, glob, grep, web
	Edit     bool // edit_file, write_file
	Run      bool // run_command
	Skills   bool // load/read skill (+ drafts on parent)
	Spawn    bool // spawn_general_agent, wait_subagents
	Delegate bool // delegate_coding_task (external harness)
	Jobs     bool
	Memory   bool
	MCP      bool
	Settings bool // list/get/patch_settings (parent only)
	SkillMut bool // write/promote skill drafts (parent only)
	Inbox    bool // leave_inbox_message (parent / autonomy)
}

// ParentSurface is the full parent-agent tool surface.
func ParentSurface(delegateEnabled bool) ToolSurface {
	return ToolSurface{
		Read: true, Edit: true, Run: true, Skills: true, SkillMut: true,
		Spawn: true, Delegate: delegateEnabled,
		Jobs: true, Memory: true, MCP: true, Settings: true, Inbox: true,
	}
}

// ResearchSurface is read-only in-process subagent tools.
func ResearchSurface() ToolSurface {
	return ToolSurface{Read: true, Skills: true}
}

// CodingSurface is in-process coding subagent tools (no spawn/delegate).
func CodingSurface() ToolSurface {
	return ToolSurface{Read: true, Edit: true, Run: true, Skills: true}
}

// SurfaceForMode maps SubagentMode to a ToolSurface.
func SurfaceForMode(mode SubagentMode) ToolSurface {
	if mode == SubagentModeCoding {
		return CodingSurface()
	}
	return ResearchSurface()
}

// Session kind + display label constants (persist kind; labels are derived).
const (
	SessionKindResearch = "general"
	SessionKindCoding   = "coding"
	SessionKindACP      = "acp"

	AgentLabelResearch = "cometmind"
	AgentLabelCoding   = "cometmind-coding"
)

// SessionKindForMode is the persisted subagent_kind value.
func SessionKindForMode(mode SubagentMode) string {
	if mode == SubagentModeCoding {
		return SessionKindCoding
	}
	return SessionKindResearch
}

// AgentLabelForMode is the SSE/display agent name for in-process children.
func AgentLabelForMode(mode SubagentMode) string {
	if mode == SubagentModeCoding {
		return AgentLabelCoding
	}
	return AgentLabelResearch
}

// AgentLabelForSessionKind maps persisted kind → display label.
func AgentLabelForSessionKind(kind string) string {
	switch kind {
	case SessionKindCoding:
		return AgentLabelCoding
	case SessionKindResearch, "":
		return AgentLabelResearch
	default:
		// External harness kinds (acp) keep harness-specific names elsewhere.
		return ""
	}
}
