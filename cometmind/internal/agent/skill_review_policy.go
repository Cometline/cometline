package agent

import (
	"path/filepath"
	"strings"
	"time"
	"unicode"
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

func countMutating(calls []skillCall) int {
	n := 0
	for _, call := range calls {
		if call.OK && isMutatingTool(call.Name) {
			n++
		}
	}
	return n
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

func correctionCompleted(userText string, previous skillTargets, calls []skillCall) bool {
	paths, commands, referred := referredTargets(userText, previous)
	if !referred {
		return false
	}
	for _, call := range calls {
		if !call.OK {
			continue
		}
		if path := normalizePath(call.Path); path != "" && containsPath(paths, path) {
			return true
		}
		if cmd := strings.TrimSpace(call.Command); cmd != "" && containsCommand(commands, cmd) {
			return true
		}
	}
	return false
}

func referredTargets(userText string, previous skillTargets) (paths, commands []string, ok bool) {
	text := strings.TrimSpace(userText)
	if text == "" || (len(previous.Paths) == 0 && len(previous.Commands) == 0) {
		return nil, nil, false
	}
	lower := strings.ToLower(text)
	for _, path := range previous.Paths {
		path = normalizePath(path)
		if path == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(path)) {
			paths = append(paths, path)
			continue
		}
		base := strings.ToLower(filepath.Base(path))
		if len(base) >= 5 && strings.Contains(base, ".") && containsToken(lower, base) {
			paths = append(paths, path)
		}
	}
	for _, cmd := range previous.Commands {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		cmdLower := strings.ToLower(cmd)
		if len(cmd) >= 6 && strings.Contains(lower, cmdLower) {
			commands = append(commands, cmd)
			continue
		}
		if containsToken(lower, cmdLower) {
			commands = append(commands, cmd)
		}
	}
	return paths, commands, len(paths)+len(commands) > 0
}

func containsPath(paths []string, path string) bool {
	for _, candidate := range paths {
		if normalizePath(candidate) == path {
			return true
		}
	}
	return false
}

func containsCommand(commands []string, command string) bool {
	command = strings.TrimSpace(command)
	for _, candidate := range commands {
		if strings.TrimSpace(candidate) == command {
			return true
		}
	}
	return false
}

func containsToken(text, token string) bool {
	if token == "" {
		return false
	}
	start := 0
	for start <= len(text) {
		idx := strings.Index(text[start:], token)
		if idx < 0 {
			return false
		}
		idx += start
		beforeOK := idx == 0 || !isTokenRune(rune(text[idx-1]))
		end := idx + len(token)
		afterOK := end == len(text) || !isTokenRune(rune(text[end]))
		if beforeOK && afterOK {
			return true
		}
		start = idx + 1
	}
	return false
}

func isTokenRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' || r == '/'
}

func normalizePath(path string) string {
	path = strings.TrimSpace(filepath.ToSlash(path))
	path = strings.TrimPrefix(path, "./")
	return path
}
