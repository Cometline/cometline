package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cometline/cometmind/internal/db"
	"github.com/cometline/cometmind/internal/jobs"
	_ "modernc.org/sqlite"
)

func testJobsService(t *testing.T) *jobs.Service {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.EnsureSchema(context.Background(), conn); err != nil {
		t.Fatal(err)
	}
	return jobs.NewService(conn, nil, nil)
}

func TestJobPromptIndexIncludesSessionWorkspace(t *testing.T) {
	idx := JobPromptIndex("/Users/me/project", "")
	if !strings.Contains(idx, "/Users/me/project") {
		t.Fatalf("expected session workspace in prompt index, got %q", idx)
	}
	if !strings.Contains(idx, "propose_job") {
		t.Fatalf("expected propose_job guidance, got %q", idx)
	}
}

func TestJobPromptIndexDiscordMentionsComponents(t *testing.T) {
	idx := JobPromptIndex("/tmp/ws", jobs.PlatformDiscord)
	if !strings.Contains(idx, "dropdown") {
		t.Fatalf("expected discord component guidance, got %q", idx)
	}
	if !strings.Contains(idx, "/create-job") {
		t.Fatalf("expected /create-job guidance, got %q", idx)
	}
}

func TestProposeJobReturnsAwaitingWorkspaceJSON(t *testing.T) {
	tool := proposeJobTool{deps: JobsDeps{SessionWorkspacePath: "/tmp/ws"}}
	input := json.RawMessage(`{"description":"Fix auth","definition_of_done":"Tests pass"}`)
	res, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected ok result, got %q", res.Output)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(res.Output), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "awaiting_workspace" {
		t.Fatalf("status = %q", payload["status"])
	}
	if payload["description"] != "Fix auth" {
		t.Fatalf("description = %q", payload["description"])
	}
	if payload["definition_of_done"] != "Tests pass" {
		t.Fatalf("definition_of_done = %q", payload["definition_of_done"])
	}
	if payload["default_workspace"] != "/tmp/ws" {
		t.Fatalf("default_workspace = %q", payload["default_workspace"])
	}
}

func TestProposeJobRequiresDescription(t *testing.T) {
	tool := proposeJobTool{deps: JobsDeps{}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"description":"  "}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.OK {
		t.Fatal("expected failure for empty description")
	}
}

func TestCreateJobDefaultsWorkspacePath(t *testing.T) {
	svc := testJobsService(t)
	tool := createJobTool{deps: JobsDeps{
		Service:              svc,
		SessionID:            "sess-1",
		SessionWorkspacePath: "/default/ws",
		SourcePlatform:       jobs.PlatformDesktop,
	}}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"description":"Task"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("expected ok, got %q", res.Output)
	}
	created, err := svc.List(context.Background(), jobs.ListFilter{Status: jobs.StatusTodo})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Fatalf("jobs=%d want 1", len(created))
	}
	if created[0].WorkspacePath != "/default/ws" {
		t.Fatalf("workspace_path=%q want /default/ws", created[0].WorkspacePath)
	}
}
