package session

import (
	"fmt"
	"strings"
)

// AgentMode selects the agent capability surface for a session or turn.
// It is a tool-capability policy, not an OS sandbox: Plan removes command,
// file-write, and mutation tools while keeping host-wide reads and network.
type AgentMode string

const (
	// AgentModeAuto preserves the full parent-agent tool surface.
	AgentModeAuto AgentMode = "auto"
	// AgentModePlan is a read-only planning surface: no command, no workspace
	// mutation, no external coding harness, and research-only subagents.
	AgentModePlan AgentMode = "plan"
)

// ValidAgentModes lists the accepted agent mode values.
var ValidAgentModes = []AgentMode{AgentModeAuto, AgentModePlan}

// ParseAgentMode validates a wire value. Empty defaults to auto; unknown
// values fail closed with an error so callers can reject the request.
func ParseAgentMode(raw string) (AgentMode, error) {
	switch AgentMode(strings.TrimSpace(strings.ToLower(raw))) {
	case "":
		return AgentModeAuto, nil
	case AgentModeAuto:
		return AgentModeAuto, nil
	case AgentModePlan:
		return AgentModePlan, nil
	default:
		return "", fmt.Errorf("invalid agent mode %q (expected auto or plan)", raw)
	}
}

// IsPlan reports whether the mode is Plan.
func (m AgentMode) IsPlan() bool {
	return m == AgentModePlan
}
