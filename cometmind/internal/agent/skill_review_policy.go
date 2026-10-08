package agent

import (
	"path/filepath"
	"strings"
	"time"
)

const (
	reviewTurnThreshold = 10
	reviewMaxSteps      = 30
	reviewKind          = "skill_review"
)

type skillCall struct {
	Name    string
	Path    string
	Command string
	OK      bool
}

type skillTargets struct {
	Paths    []string `json:"paths,omitempty"`
	Commands []string `json:"commands,omitempty"`
}

type skillReviewInput struct {
	UserChat    bool
	HasModel    bool
	Now         time.Time
	LastStarted time.Time
	UserText    string
	Previous    skillTargets
	Calls       []skillCall
	Turns       int
}

type skillReviewDecision struct {
	Start      bool
	Reason     string
	WideWindow bool
	LogSkip    string
	Targets    skillTargets
}

func decideSkillReview(in skillReviewInput) skillReviewDecision {
	decision := skillReviewDecision{Targets: targetsFromCalls(in.Calls)}
	if !in.UserChat {
		return decision
	}
	if in.Turns < reviewTurnThreshold {
		return decision
	}
	if !in.HasModel {
		decision.LogSkip = "skills.review.skipped_no_model"
		return decision
	}
	decision.Start = true
	decision.WideWindow = true
	decision.Reason = "turns"
	return decision
}

func isMutatingTool(name string) bool {
	switch name {
	case "edit_file", "write_file", "run_command", "write_skill", "write_skill_draft", "promote_skill_draft":
		return true
	default:
		return false
	}
}

func targetsFromCalls(calls []skillCall) skillTargets {
	var out skillTargets
	seenPath := map[string]bool{}
	seenCmd := map[string]bool{}
	for _, call := range calls {
		if !call.OK || !isMutatingTool(call.Name) {
			continue
		}
		if path := normalizePath(call.Path); path != "" && !seenPath[path] {
			seenPath[path] = true
			out.Paths = append(out.Paths, path)
		}
		if cmd := strings.TrimSpace(call.Command); cmd != "" && !seenCmd[cmd] {
			seenCmd[cmd] = true
			out.Commands = append(out.Commands, cmd)
		}
	}
	return out
}

func normalizePath(path string) string {
	path = strings.TrimSpace(filepath.ToSlash(path))
	path = strings.TrimPrefix(path, "./")
	return path
}
