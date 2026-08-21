package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/cometline/cometmind/internal/process"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// defaultConnectTimeout is the fallback used when a caller does not supply
	// its own deadline (stdio handshake). Remote HTTP/SSE servers use the
	// longer handshakeTimeoutHTTP instead — a single 10s budget is enough to
	// spawn a local subprocess but not to finish initialize +
	// notifications/initialized against a Cloudflare-fronted MCP.
	defaultConnectTimeout = 15 * time.Second
	handshakeTimeoutHTTP  = 45 * time.Second
	listToolsTimeoutStdio = 10 * time.Second
	listToolsTimeoutHTTP  = 20 * time.Second
)

// defaultKeepAlive enables the go-sdk's built-in ping/keepalive loop
// (mcp.ClientOptions.KeepAlive) for every MCP session, regardless of
// transport (stdio, HTTP, or SSE). Without this, a silently-dead connection
// (network drop, idle load-balancer reap, subprocess exit) is never detected
// proactively — the SDK only surfaces the failure reactively, the next time a
// tool call happens to be attempted. With KeepAlive set, the SDK pings the
// peer on this interval and, if a ping fails, closes the session itself
// (see go-sdk mcp/shared.go startKeepalive), which Manager's monitorConnection
// observes via ClientSession.Wait() to correct the cached status and drive a
// bounded automatic reconnect.
const defaultKeepAlive = 30 * time.Second

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

func connectTimeoutFor(cfg ServerConfig) time.Duration {
	switch cfg.Transport {
	case TransportHTTP, TransportSSE:
		return handshakeTimeoutHTTP
	default:
		return defaultConnectTimeout
	}
}

func listToolsTimeoutFor(cfg ServerConfig) time.Duration {
	switch cfg.Transport {
	case TransportHTTP, TransportSSE:
		return listToolsTimeoutHTTP
	default:
		return listToolsTimeoutStdio
	}
}

func connectServer(ctx context.Context, cfg ServerConfig) (*connectedServer, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("server %q is disabled", cfg.ID)
	}
	if cfg.Transport == TransportHTTP || cfg.Transport == TransportSSE {
		cfg.URL = NormalizeServerURL(cfg.URL)
		if oauthTokenStaleForURL(cfg.ID, cfg.URL) {
			return nil, newConnectError(CodeAuthExpired,
				"The saved sign-in is for a different MCP URL. Click Connect with OAuth and sign in again.",
				fmt.Errorf("oauth token resource mismatch for server %q", cfg.ID))
		}
	}
	transport, err := buildTransport(cfg)
	if err != nil {
		return nil, classifyConnectErrorFor(cfg.ID, err)
	}

	// Handshake and list-tools get independent budgets so a slow initialize
	// cannot steal the deadline from notifications/initialized + tools/list.
	connectCtx, cancel := context.WithTimeout(ctx, connectTimeoutFor(cfg))
	client := mcp.NewClient(clientImpl, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
		KeepAlive:    defaultKeepAlive,
	})
	session, err := client.Connect(connectCtx, transport, nil)
	cancel()
	if err != nil {
		return nil, classifyConnectErrorFor(cfg.ID, fmt.Errorf("connect: %w", err))
	}

	listCtx, listCancel := context.WithTimeout(ctx, listToolsTimeoutFor(cfg))
	tools, err := listTools(listCtx, session, cfg)
	listCancel()
	if err != nil {
		_ = session.Close()
		return nil, classifyConnectErrorFor(cfg.ID, err)
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
		resolved, err := process.ResolveCommand(command)
		if err != nil {
			return nil, fmt.Errorf(
				"stdio server %q: %w",
				cfg.ID,
				process.CommandNotFoundError(command, err),
			)
		}
		cmd := exec.Command(resolved, cfg.Args...)
		// Packaged Cometline/Electron often inherits a minimal PATH (no /usr/local/bin).
		// Use the same augmented PATH as coding harnesses and built-in shell tools.
		cmd.Env = process.Env()
		if len(cfg.Env) > 0 {
			cmd.Env = append(cmd.Env, envPairs(cfg.Env)...)
		}
		return &mcp.CommandTransport{Command: cmd}, nil
	case TransportHTTP:
		if err := validateRemoteURL(cfg.ID, cfg.URL); err != nil {
			return nil, err
		}
		return streamableTransport(cfg), nil
	case TransportSSE:
		if err := validateRemoteURL(cfg.ID, cfg.URL); err != nil {
			return nil, err
		}
		return sseTransport(cfg), nil
	default:
		return nil, fmt.Errorf("server %q: unsupported transport %q", cfg.ID, cfg.Transport)
	}
}

func validateRemoteURL(serverID, rawURL string) error {
	normalized := NormalizeServerURL(rawURL)
	u, err := url.Parse(normalized)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		if err == nil {
			err = fmt.Errorf("expected an absolute http(s) URL")
		}
		return newConnectError(
			CodeBadURL,
			"Enter a complete MCP URL beginning with http:// or https://.",
			fmt.Errorf("remote server %q has invalid URL %q: %w", serverID, strings.TrimSpace(rawURL), err),
		)
	}
	return nil
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
	connectCtx, cancel := context.WithTimeout(ctx, connectTimeoutFor(cfg))
	client := mcp.NewClient(clientImpl, &mcp.ClientOptions{
		Capabilities: &mcp.ClientCapabilities{},
		KeepAlive:    defaultKeepAlive,
	})
	session, err := client.Connect(connectCtx, transport, nil)
	cancel()
	if err != nil {
		return nil, classifyConnectErrorFor(cfg.ID, fmt.Errorf("connect: %w", err))
	}
	listCtx, listCancel := context.WithTimeout(ctx, listToolsTimeoutFor(cfg))
	tools, err := listTools(listCtx, session, cfg)
	listCancel()
	if err != nil {
		_ = session.Close()
		return nil, classifyConnectErrorFor(cfg.ID, err)
	}
	return &connectedServer{cfg: cfg, session: session, tools: tools}, nil
}
