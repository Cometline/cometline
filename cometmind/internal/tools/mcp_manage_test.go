package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcppkg "github.com/cometline/cometmind/internal/mcp"
)

// TestListMCPServersToolNilManager verifies the tool degrades gracefully
// when MCP isn't configured at all, instead of panicking on a nil manager.
func TestListMCPServersToolNilManager(t *testing.T) {
	tool := listMCPServersTool{}
	res, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.OK {
		t.Fatal("Execute() OK = true, want false (nil manager)")
	}
	if res.Output != "MCP is not configured" {
		t.Fatalf("Execute() output = %q, want %q", res.Output, "MCP is not configured")
	}
}

// TestListMCPServersToolNoServers verifies the "no servers configured"
// message when MCP is enabled but nothing has been added.
func TestListMCPServersToolNoServers(t *testing.T) {
	mgr := mcppkg.NewManager(mcppkg.Config{Enabled: true})
	mgr.Start(context.Background())

	tool := listMCPServersTool{mgr: mgr}
	res, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !res.OK {
		t.Fatalf("Execute() OK = false, want true")
	}
	if res.Output != "No MCP servers configured." {
		t.Fatalf("Execute() output = %q, want %q", res.Output, "No MCP servers configured.")
	}
}

// TestListMCPServersToolFormatsStatusAndError verifies the tool lists every
// configured server with its transport, status, tool count, and — when
// present — its last error, so the agent can answer "is Jira connected?"
// without needing to hit the management API itself.
func TestListMCPServersToolFormatsStatusAndError(t *testing.T) {
	mgr := mcppkg.NewManager(mcppkg.Config{
		Enabled: true,
		Servers: []mcppkg.ServerConfig{
			{ID: "broken", Name: "Broken", Enabled: true, Transport: mcppkg.TransportStdio, Command: "false"},
		},
	})
	mgr.Start(context.Background())

	tool := listMCPServersTool{mgr: mgr}
	res, err := tool.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !res.OK {
		t.Fatalf("Execute() OK = false, want true")
	}
	for _, want := range []string{"Broken", "broken", "stdio transport", "status=error", "tools=0", "last_error="} {
		if !strings.Contains(res.Output, want) {
			t.Errorf("Execute() output = %q, missing %q", res.Output, want)
		}
	}
}

// TestListMCPServersToolSpec verifies the exposed tool schema stays stable
// (name and an empty-object parameter shape, since this tool takes no input).
func TestListMCPServersToolSpec(t *testing.T) {
	spec := listMCPServersTool{}.Spec()
	if spec.Name != "list_mcp_servers" {
		t.Fatalf("Spec().Name = %q, want %q", spec.Name, "list_mcp_servers")
	}
	if spec.Description == "" {
		t.Fatal("Spec().Description is empty")
	}
	var schema map[string]any
	if err := json.Unmarshal(spec.Parameters, &schema); err != nil {
		t.Fatalf("Spec().Parameters is not valid JSON: %v", err)
	}
}

// TestReconnectMCPServerToolNilManager verifies the reconnect tool also
// degrades gracefully with no manager configured.
func TestReconnectMCPServerToolNilManager(t *testing.T) {
	tool := reconnectMCPServerTool{}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"server_id":"anything"}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.OK {
		t.Fatal("Execute() OK = true, want false (nil manager)")
	}
	if res.Output != "MCP is not configured" {
		t.Fatalf("Execute() output = %q, want %q", res.Output, "MCP is not configured")
	}
}

// TestReconnectMCPServerToolMissingServerID verifies the tool validates its
// required input instead of forwarding an empty ID to the manager.
func TestReconnectMCPServerToolMissingServerID(t *testing.T) {
	mgr := mcppkg.NewManager(mcppkg.Config{Enabled: true})
	mgr.Start(context.Background())

	tool := reconnectMCPServerTool{mgr: mgr}
	for _, input := range []json.RawMessage{
		json.RawMessage(`{}`),
		json.RawMessage(`{"server_id":"   "}`),
	} {
		res, err := tool.Execute(context.Background(), input)
		if err != nil {
			t.Fatalf("Execute(%s) error = %v", input, err)
		}
		if res.OK {
			t.Fatalf("Execute(%s) OK = true, want false", input)
		}
		if res.Output != "server_id is required" {
			t.Fatalf("Execute(%s) output = %q, want %q", input, res.Output, "server_id is required")
		}
	}
}

// TestReconnectMCPServerToolInvalidJSON verifies malformed input surfaces as
// a soft tool failure (OK=false) rather than an error/panic.
func TestReconnectMCPServerToolInvalidJSON(t *testing.T) {
	mgr := mcppkg.NewManager(mcppkg.Config{Enabled: true})
	mgr.Start(context.Background())

	tool := reconnectMCPServerTool{mgr: mgr}
	res, err := tool.Execute(context.Background(), json.RawMessage(`not json`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.OK {
		t.Fatal("Execute() OK = true, want false for invalid JSON input")
	}
	if !strings.Contains(res.Output, "invalid tool input") {
		t.Fatalf("Execute() output = %q, want it to mention invalid input", res.Output)
	}
}

// TestReconnectMCPServerToolUnknownServer verifies reconnecting an ID that
// was never configured reports a clear "unknown server" failure rather than
// a generic error, so the agent (or user) knows to re-check list_mcp_servers.
func TestReconnectMCPServerToolUnknownServer(t *testing.T) {
	mgr := mcppkg.NewManager(mcppkg.Config{Enabled: true})
	mgr.Start(context.Background())

	tool := reconnectMCPServerTool{mgr: mgr}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"server_id":"ghost"}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.OK {
		t.Fatal("Execute() OK = true, want false for an unknown server")
	}
	if !strings.Contains(res.Output, "unknown server: ghost") {
		t.Fatalf("Execute() output = %q, want it to mention unknown server", res.Output)
	}
}

// TestReconnectMCPServerToolFailedReconnect verifies a reconnect attempt
// against a server that fails to connect reports OK=false with the
// underlying connect error surfaced as the output, so the agent can relay
// the concrete failure reason to the user.
func TestReconnectMCPServerToolFailedReconnect(t *testing.T) {
	mgr := mcppkg.NewManager(mcppkg.Config{
		Enabled: true,
		Servers: []mcppkg.ServerConfig{
			{ID: "broken", Name: "Broken", Enabled: true, Transport: mcppkg.TransportStdio, Command: "false"},
		},
	})
	mgr.Start(context.Background())

	tool := reconnectMCPServerTool{mgr: mgr}
	res, err := tool.Execute(context.Background(), json.RawMessage(`{"server_id":"broken"}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.OK {
		t.Fatal("Execute() OK = true, want false (command always fails)")
	}
	if res.Output == "" {
		t.Fatal("Execute() output is empty, want the underlying connect error")
	}

	// The server's cached status should also reflect the failure, so a
	// follow-up list_mcp_servers call is consistent with this result.
	for _, s := range mgr.ListServers() {
		if s.ID == "broken" && s.Status != mcppkg.StatusError {
			t.Errorf("server status = %q, want %q", s.Status, mcppkg.StatusError)
		}
	}
}

// TestReconnectMCPServerToolSpec verifies the exposed schema requires
// server_id, since Execute depends on it being present.
func TestReconnectMCPServerToolSpec(t *testing.T) {
	spec := reconnectMCPServerTool{}.Spec()
	if spec.Name != "reconnect_mcp_server" {
		t.Fatalf("Spec().Name = %q, want %q", spec.Name, "reconnect_mcp_server")
	}
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(spec.Parameters, &schema); err != nil {
		t.Fatalf("Spec().Parameters is not valid JSON: %v", err)
	}
	found := false
	for _, r := range schema.Required {
		if r == "server_id" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Spec().Parameters required = %v, want it to include %q", schema.Required, "server_id")
	}
}

// TestRegistryIncludesMCPManageToolsWhenMCPConfigured verifies NewRegistry
// wires list_mcp_servers and reconnect_mcp_server whenever an MCP manager is
// supplied via RegistryOptions.
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

// TestRegistryExcludesMCPManageToolsWhenMCPNotConfigured verifies the tools
// are absent entirely (not just non-functional) when no MCP manager is
// passed to NewRegistry, keeping the tool list clean for workspaces without
// MCP configured.
func TestRegistryExcludesMCPManageToolsWhenMCPNotConfigured(t *testing.T) {
	r := NewRegistry(t.TempDir())
	for _, name := range []string{"list_mcp_servers", "reconnect_mcp_server"} {
		if r.Has(name) {
			t.Fatalf("registry should not include %q without an MCP manager", name)
		}
	}
}
