package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
)

func TestManagerInMemoryToolCall(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "Echo input",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
			},
		},
	}, func(_ context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		msg, _ := args["message"].(string)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: msg}},
		}, nil, nil
	})

	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()

	cfg := ServerConfig{ID: "demo", Name: "Demo", Enabled: true, Transport: TransportStdio}
	conn, err := connectServerWithTransport(ctx, cfg, t2)
	if err != nil {
		t.Fatalf("connectServerWithTransport: %v", err)
	}
	defer conn.session.Close()

	if len(conn.tools) != 1 || conn.tools[0].Name != "echo" {
		t.Fatalf("tools = %#v, want echo", conn.tools)
	}

	if ToolName(cfg.ID, conn.tools[0].Name) != "mcp_demo_echo" {
		t.Fatalf("registry name mismatch")
	}

	res, err := conn.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"message": "hello"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if text != "hello" {
		t.Fatalf("output = %q", text)
	}
}

func TestManagerStartAndList(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(Config{
		Enabled: true,
		Servers: []ServerConfig{{ID: "other", Name: "Other", Enabled: true, Transport: TransportStdio, Command: "false"}},
	})
	mgr.Start(ctx)
	statuses := mgr.ListServers()
	if len(statuses) != 1 {
		t.Fatalf("statuses = %d, want 1", len(statuses))
	}
	if statuses[0].Status != StatusError {
		t.Fatalf("status = %q, want error", statuses[0].Status)
	}
	if statuses[0].ErrorCode == "" && statuses[0].ErrorHint == "" && statuses[0].LastError == "" {
		t.Fatal("expected classified error fields on a failed connect")
	}
}

func TestStartDetachesFromCancelledParent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	mgr := NewManager(Config{
		Enabled: true,
		Servers: []ServerConfig{{ID: "a", Name: "A", Enabled: true, Transport: TransportStdio, Command: "false"}},
	})
	mgr.Start(ctx)
	statuses := mgr.ListServers()
	if len(statuses) != 1 {
		t.Fatalf("statuses = %d, want 1", len(statuses))
	}
	if statuses[0].Status != StatusError {
		t.Fatalf("status = %q, want error", statuses[0].Status)
	}
	if strings.Contains(strings.ToLower(statuses[0].LastError), "canceled") {
		t.Fatalf("cancelled parent leaked into connect: %q", statuses[0].LastError)
	}
}

func TestListServersOAuthConnectedWithoutOAuthBlock(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const serverID = "atlassian"
	if err := SaveOAuthToken(serverID, &oauth2.Token{AccessToken: "tok"}); err != nil {
		t.Fatalf("SaveOAuthToken: %v", err)
	}
	mgr := NewManager(Config{
		Enabled: true,
		Servers: []ServerConfig{{ID: serverID, Name: "Atlassian", Enabled: false, Transport: TransportHTTP, URL: "https://mcp.atlassian.com/v1/mcp"}},
	})
	mgr.Start(context.Background())
	statuses := mgr.ListServers()
	if len(statuses) != 1 {
		t.Fatalf("statuses = %d, want 1", len(statuses))
	}
	if !statuses[0].OAuthConnected {
		t.Fatal("OAuthConnected = false, want true even without an oauth settings block")
	}
}

func TestCallToolUsesLiveSession(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "Echo input",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"message": map[string]any{"type": "string"}}},
	}, func(_ context.Context, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
		msg, _ := args["message"].(string)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: msg}}}, nil, nil
	})

	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()

	cfg := ServerConfig{ID: "demo", Name: "Demo", Enabled: true, Transport: TransportStdio}
	conn, err := connectServerWithTransport(ctx, cfg, t2)
	if err != nil {
		t.Fatalf("connectServerWithTransport: %v", err)
	}
	defer conn.session.Close()

	mgr := NewManager(Config{Enabled: true})
	mgr.mu.Lock()
	mgr.servers["demo"] = &managedServer{cfg: cfg, conn: conn, status: StatusConnected}
	mgr.mu.Unlock()

	res, err := mgr.CallTool(ctx, "demo", "echo", map[string]any{"message": "live"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if text != "live" {
		t.Fatalf("output = %q", text)
	}

	mgr.mu.Lock()
	mgr.servers["demo"].conn = nil
	mgr.mu.Unlock()
	if _, err := mgr.CallTool(ctx, "demo", "echo", map[string]any{"message": "dead"}); err == nil {
		t.Fatal("CallTool after session drop: want error")
	}
}

func TestConnectServerRejectsStaleOAuthResource(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const serverID = "jira"
	if err := SaveOAuthToken(serverID, &oauth2.Token{AccessToken: "tok"}); err != nil {
		t.Fatalf("SaveOAuthToken: %v", err)
	}
	if err := saveOAuthClientInfo(serverID, &oauthClientInfo{
		TokenEndpoint: "https://auth.example.com/token",
		ClientID:      "client",
		ServerURL:     "https://mcp.atlassian.com/v1/mcp/authv2",
	}); err != nil {
		t.Fatalf("saveOAuthClientInfo: %v", err)
	}
	_, err := connectServer(context.Background(), ServerConfig{
		ID:        serverID,
		Enabled:   true,
		Transport: TransportHTTP,
		URL:       "https://mcp.github.com/mcp",
	})
	if err == nil {
		t.Fatal("expected stale OAuth error")
	}
	classified := classifyConnectError(err)
	if classified.Code != CodeAuthExpired {
		t.Fatalf("code = %q, want %q (err=%v)", classified.Code, CodeAuthExpired, err)
	}
}

func TestToolName(t *testing.T) {
	tests := []struct {
		serverID string
		toolName string
		want     string
	}{
		{"github", "create_issue", "mcp_github_create_issue"},
		{"demo", "echo", "mcp_demo_echo"},
		{"my-server", "search", "mcp_my-server_search"},
		{"ctx7", "resolve-library-id", "mcp_ctx7_resolve-library-id"},
		{"plugin", "browser/navigate", "mcp_plugin_browser_navigate"},
	}
	for _, tt := range tests {
		if got := ToolName(tt.serverID, tt.toolName); got != tt.want {
			t.Fatalf("ToolName(%q, %q) = %q, want %q", tt.serverID, tt.toolName, got, tt.want)
		}
	}
}
