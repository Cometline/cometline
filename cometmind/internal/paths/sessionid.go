package paths

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrInvalidSessionID is returned when a session id cannot be used as a path segment.
var ErrInvalidSessionID = errors.New("invalid session id")

var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// ValidSessionID reports whether id is safe to use as a terminal-env path segment.
func ValidSessionID(id string) bool {
	return sessionIDPattern.MatchString(id)
}

// IsTerminalEnvPath reports whether abs is the terminal-env root or a file under it.
func IsTerminalEnvPath(abs string) bool {
	root, err := TerminalEnvRoot()
	if err != nil || root == "" || abs == "" {
		return false
	}
	return pathWithin(root, abs)
}

func pathWithin(root, abs string) bool {
	root = filepath.Clean(root)
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// CommandMentionsTerminalEnv reports whether a shell command string names the
// private terminal-env snapshot directory.
func CommandMentionsTerminalEnv(command string) bool {
	if strings.TrimSpace(command) == "" {
		return false
	}
	root, err := TerminalEnvRoot()
	if err == nil && root != "" && strings.Contains(command, root) {
		return true
	}
	if strings.Contains(command, filepath.Join("~", ".cometmind", "terminal-env")) ||
		strings.Contains(command, "~/.cometmind/terminal-env") ||
		strings.Contains(command, "$HOME/.cometmind/terminal-env") ||
		strings.Contains(command, "${HOME}/.cometmind/terminal-env") {
		return true
	}
	if home, err := Home(); err == nil && home != "" &&
		strings.Contains(command, filepath.Join(home, ".cometmind", "terminal-env")) {
		return true
	}
	return false
}
