package codex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	cometsdk "github.com/cometline/comet-sdk"
	"github.com/cometline/comet-sdk/internal/providerbase"
	"github.com/gorilla/websocket"
)

const responsesWebsocketBeta = "responses_websockets=2026-02-06"

func (p *provider) streamWebSocket(ctx context.Context, req *cometsdk.Request) (<-chan cometsdk.Event, error) {
	token, err := borrowCodexToken(ctx, p.httpClient())
	if err != nil {
		return nil, err
	}

	body, err := toCodexWebSocketRequest(req)
	if err != nil {
		return nil, fmt.Errorf("codex: marshal websocket request: %w", err)
	}
	wsURL, err := responsesWebSocketURL(p.cfg.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("codex: websocket URL: %w", err)
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+token.AccessToken)
	responsesLite := codexUsesResponsesLite(req.Model)
	addCodexResponseHeaders(header, token, responsesLite)
	header.Set("OpenAI-Beta", responsesWebsocketBeta)
	header.Set("Originator", "codex_cli_rs")

	dialer := websocket.DefaultDialer
	if p.cfg.Timeout > 0 {
		dialer.HandshakeTimeout = p.cfg.Timeout
	}
	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return nil, fmt.Errorf("codex: websocket connect: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, body); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("codex: websocket send: %w", err)
	}
	p.log.DebugContext(ctx, "stream.transport", "transport", "websocket", "model", req.Model)

	reader, writer := io.Pipe()
	ch := make(chan cometsdk.Event, 32)
	go p.readWebSocket(ctx, conn, writer)
	go parseLoop(ctx, providerID, reader, ch, p.log)
	return ch, nil
}

func (p *provider) readWebSocket(ctx context.Context, conn *websocket.Conn, writer *io.PipeWriter) {
	defer conn.Close()
	defer writer.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				_ = writer.CloseWithError(ctx.Err())
			} else {
				_ = writer.CloseWithError(fmt.Errorf("codex: websocket read: %w", err))
			}
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		if err := writeWebSocketSSEEvent(writer, payload); err != nil {
			return
		}
		if strings.Contains(string(payload), `"type":"response.completed"`) {
			return
		}
	}
}

func writeWebSocketSSEEvent(writer io.Writer, payload []byte) error {
	payload = bytes.ReplaceAll(payload, []byte("\r\n"), []byte("\n"))
	for _, line := range bytes.Split(payload, []byte("\n")) {
		if _, err := fmt.Fprintf(writer, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(writer, "\n")
	return err
}

func addCodexResponseHeaders(header http.Header, token borrowedToken, responsesLite bool) {
	if token.AccountID != "" {
		header.Set("ChatGPT-Account-ID", token.AccountID)
	}
	if token.InstallationID != "" {
		header.Set("x-codex-installation-id", token.InstallationID)
	}
	if responsesLite {
		header.Set(responsesLiteHeader, "true")
	}
}

func responsesWebSocketURL(baseURL string) (string, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/responses")
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	case "ws", "wss":
	default:
		return "", fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	return u.String(), nil
}

func toCodexWebSocketRequest(req *cometsdk.Request) ([]byte, error) {
	// Start from the established Responses conversion so tool calls, images,
	// reasoning summaries, and provider overrides stay identical across SSE and
	// WebSocket. WebSocket mode does not use max_output_tokens or temperature in
	// the Codex wire request.
	data, err := toCodexRequest(req, true)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	delete(payload, "temperature")
	payload["type"] = "response.create"
	if _, ok := payload["tools"]; !ok {
		payload["tools"] = []any{}
	}
	payload["tool_choice"] = "auto"
	// Standard Codex Responses supports parallel tool calls; Responses-Lite
	// overrides this below because it currently requires serial execution.
	payload["parallel_tool_calls"] = true
	if codexUsesResponsesLite(req.Model) {
		payload["parallel_tool_calls"] = false
		payload["include"] = []string{"reasoning.encrypted_content"}
		reasoning, ok := payload["reasoning"].(map[string]any)
		if !ok {
			reasoning = make(map[string]any)
		}
		reasoning["context"] = "all_turns"
		payload["reasoning"] = reasoning
	}
	payload["stream"] = true
	return providerbase.MarshalWithOptions(payload, nil, providerID)
}

func codexUsesResponsesLite(model string) bool {
	// Responses-Lite is currently a Luna-only Codex capability. Keep this
	// isolated so model catalog metadata can replace the temporary capability
	// table without changing transport selection.
	return model == "gpt-5.6-luna"
}

func addResponsesLiteReasoningContext(data []byte) ([]byte, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	reasoning, ok := payload["reasoning"].(map[string]any)
	if !ok {
		reasoning = make(map[string]any)
	}
	reasoning["context"] = "all_turns"
	payload["reasoning"] = reasoning
	payload["parallel_tool_calls"] = false
	return json.Marshal(payload)
}
