package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultConnectTimeout = 10 * time.Second

var clientImpl = &mcp.Implementation{Name: "cometmind", Version: "0.1.0"}

// DiscoveredTool is one MCP tool exposed to CometMind.
type DiscoveredTool struct {
	ServerID    string
	Name        string
	Description string
	Parameters  json.RawMessage
}

type connectedServer struct {
	cfg     ServerConfig
	session *mcp.ClientSession
	tools   []DiscoveredTool
}

func connectServer(ctx context.Context, cfg ServerConfig) (*connectedServer, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("server %q is disabled", cfg.ID)
	}
	transport, err := buildTransport(cfg)
	if err != nil {
		return nil, err
	}

	connectCtx, cancel := context.WithTimeout(ctx, defaultConnectTimeout)
	defer cancel()

	client := mcp.NewClient(clientImpl, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
	})
	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	tools, err := listTools(connectCtx, session, cfg)
	if err != nil {
		_ = session.Close()
		return nil, err
	}

	return &connectedServer{cfg: cfg, session: session, tools: tools}, nil
}

func buildTransport(cfg ServerConfig) (mcp.Transport, error) {
	switch cfg.Transport {
	case TransportStdio:
		command := strings.TrimSpace(cfg.Command)
		if command == "" {
			return nil, fmt.Errorf("stdio server %q: command is required", cfg.ID)
		}
		cmd := exec.Command(command, cfg.Args...)
		if len(cfg.Env) > 0 {
			cmd.Env = append(cmd.Environ(), envPairs(cfg.Env)...)
		}
		return &mcp.CommandTransport{Command: cmd}, nil
	case TransportHTTP:
		url := strings.TrimSpace(cfg.URL)
		if url == "" {
			return nil, fmt.Errorf("http server %q: url is required", cfg.ID)
		}
		return streamableTransport(cfg), nil
	case TransportSSE:
		url := strings.TrimSpace(cfg.URL)
		if url == "" {
			return nil, fmt.Errorf("sse server %q: url is required", cfg.ID)
		}
		return sseTransport(cfg), nil
	default:
		return nil, fmt.Errorf("server %q: unsupported transport %q", cfg.ID, cfg.Transport)
	}
}

func envPairs(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, fmt.Sprintf("%s=%s", k, v))
	}
	return out
}

func listTools(ctx context.Context, session *mcp.ClientSession, cfg ServerConfig) ([]DiscoveredTool, error) {
	res, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list tools: %w", err)
	}
	allowed := allowedToolSet(cfg.AllowedTools)
	out := make([]DiscoveredTool, 0, len(res.Tools))
	for _, tool := range res.Tools {
		if tool == nil || strings.TrimSpace(tool.Name) == "" {
			continue
		}
		if len(allowed) > 0 {
			if _, ok := allowed[tool.Name]; !ok {
				continue
			}
		}
		params, err := marshalInputSchema(tool.InputSchema)
		if err != nil {
			return nil, err
		}
		out = append(out, DiscoveredTool{
			ServerID:    cfg.ID,
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  params,
		})
	}
	return out, nil
}

func allowedToolSet(names []string) map[string]struct{} {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

func marshalInputSchema(schema any) (json.RawMessage, error) {
	if schema == nil {
		return json.RawMessage(`{"type":"object","properties":{}}`), nil
	}
	switch v := schema.(type) {
	case json.RawMessage:
		if len(v) == 0 {
			return json.RawMessage(`{"type":"object","properties":{}}`), nil
		}
		return v, nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(data), nil
	}
}

// connectServerWithTransport connects using an explicit transport (for tests).
func connectServerWithTransport(ctx context.Context, cfg ServerConfig, transport mcp.Transport) (*connectedServer, error) {
	if strings.TrimSpace(cfg.ID) == "" {
		cfg.ID = "test"
	}
	connectCtx, cancel := context.WithTimeout(ctx, defaultConnectTimeout)
	defer cancel()

	client := mcp.NewClient(clientImpl, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
	})
	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	tools, err := listTools(connectCtx, session, cfg)
	if err != nil {
		_ = session.Close()
		return nil, err
	}
	return &connectedServer{cfg: cfg, session: session, tools: tools}, nil
}
