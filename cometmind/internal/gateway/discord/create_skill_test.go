package discord

import (
	"strings"
	"testing"

	skillpkg "github.com/cometline/cometmind/internal/skills"
)

func TestExpandCreateSkillCommandForDiscord(t *testing.T) {
	t.Parallel()
	out := skillpkg.ExpandCreateSkillCommand("commit helper")
	if !strings.Contains(out, "write_skill") || !strings.Contains(out, "commit helper") {
		t.Fatalf("unexpected expansion: %s", out)
	}
}
