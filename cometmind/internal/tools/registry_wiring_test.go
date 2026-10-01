package tools

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Cometline/cometline/cometmind/internal/db"
	mcppkg "github.com/Cometline/cometline/cometmind/internal/mcp"
	"github.com/Cometline/cometline/cometmind/internal/memory"
	"github.com/Cometline/cometline/cometmind/internal/session"
	"github.com/Cometline/cometline/cometmind/internal/skills"
	_ "modernc.org/sqlite"
)

func registryMemoryService(t *testing.T) (context.Context, *sql.DB, *memory.Service) {
	t.Helper()
	ctx := context.Background()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureSchema(ctx, conn); err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	svc, err := memory.NewService(conn, memory.DefaultSettings(), nil, nil)
	if err != nil {
		_ = conn.Close()
		t.Fatal(err)
	}
	return ctx, conn, svc
}

func TestRegistryIncludesAgentMemoryTools(t *testing.T) {
	_, conn, svc := registryMemoryService(t)
	defer conn.Close()

	r := NewRegistry(t.TempDir(), RegistryOptions{Memory: svc})
	for _, name := range []string{
		"list_memories",
		"search_memories",
		"create_memory",
		"update_memory",
		"delete_memory",
	} {
		if !r.Has(name) {
			t.Fatalf("registry missing %q", name)
		}
	}
}

func TestRegistryIncludesRecallTaskOutcomeWhenMemoryConfigured(t *testing.T) {
	_, conn, svc := registryMemoryService(t)
	defer conn.Close()

	r := NewRegistry(t.TempDir(), RegistryOptions{Memory: svc})
	if !r.Has("recall_task_outcome") {
		t.Fatal("registry missing recall_task_outcome")
	}
}

func TestRegistryIncludesMCPManageToolsWhenMCPConfigured(t *testing.T) {
	mgr := mcppkg.NewManager(mcppkg.Config{Enabled: true})
	mgr.Start(context.Background())

	r := NewRegistry(t.TempDir(), RegistryOptions{MCP: mgr})
	for _, name := range []string{"list_mcp_servers", "reconnect_mcp_server"} {
		if !r.Has(name) {
			t.Fatalf("registry missing %q", name)
		}
	}
}

func TestRegistryExcludesMCPManageToolsWhenMCPNotConfigured(t *testing.T) {
	r := NewRegistry(t.TempDir())
	for _, name := range []string{"list_mcp_servers", "reconnect_mcp_server"} {
		if r.Has(name) {
			t.Fatalf("registry should not include %q without an MCP manager", name)
		}
	}
}

func TestNewInboxProcessRegistryUsesParentSurface(t *testing.T) {
	root := t.TempDir()
	skillReg := skills.Discover(root, skills.Config{Enabled: true})
	r := NewInboxProcessRegistry(root, RegistryOptions{Skills: &skillReg})
	for _, name := range []string{
		"read_file", "edit_file", "write_file", "run_command",
		"write_skill_draft", "list_skill_drafts", "read_skill_draft",
		"list_settings",
	} {
		if !r.Has(name) {
			t.Errorf("inbox process registry missing %q", name)
		}
	}
	for _, name := range []string{
		"write_skill", "promote_skill_draft",
		"spawn_general_agent", "wait_subagents", "delegate_coding_task",
	} {
		if r.Has(name) {
			t.Errorf("inbox process registry must not include %q", name)
		}
	}
}

func TestNewInboxProcessRegistryHonorsPlanMode(t *testing.T) {
	root := t.TempDir()
	skillReg := skills.Discover(root, skills.Config{Enabled: true})
	r := NewInboxProcessRegistry(root, RegistryOptions{
		Skills:    &skillReg,
		AgentMode: session.AgentModePlan,
	})
	if !r.Has("read_file") || !r.Has("load_skill") {
		t.Fatal("plan inbox registry missing read tools")
	}
	for _, name := range []string{"edit_file", "write_file", "run_command", "write_skill", "write_skill_draft"} {
		if r.Has(name) {
			t.Errorf("plan inbox registry must not include %q", name)
		}
	}
}
