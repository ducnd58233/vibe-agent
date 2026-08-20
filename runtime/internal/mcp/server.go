// Package mcp exposes the control plane to coding agents that have no hook
// system, chiefly Codex and opencode.
//
// Claude Code and Cursor get the same capabilities through hooks, which always
// fire. An MCP tool call is model-decided, so it is best effort by nature. That
// is why this is the fallback surface and not the primary one.
//
// A short tool list, not thirty. A long one makes routing worse, and every
// verifier exposed separately would be an invitation to call them out of order.
//
// vibe_fetch is the one entry that is not control-plane bookkeeping. It earns
// its slot by returning tokens rather than spending them, which is the thing a
// short list exists to protect.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/ducnd58233/vibe-agent/runtime/internal/shared/observability"
)

// ProtocolVersion is the MCP revision this server speaks.
const ProtocolVersion = "2025-06-18"

// Tool is one callable exposed to the host.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`

	Handler func(json.RawMessage) (any, error) `json:"-"`
}

// Server speaks MCP over stdio.
type Server struct {
	Name    string
	Version string
	Tools   []Tool
	Log     observability.Logger

	mu sync.Mutex
	// initialized guards tool calls until the host has completed the handshake.
	initialized bool
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	codeParse          = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
	codeInternal       = -32603
)

// Serve reads requests until the input closes.
//
// A malformed line is answered with an error and the loop continues. A server
// that exits on bad input would take the host's session down with it.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	encoder := json.NewEncoder(out)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var req request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			observability.LogError(s.Log, "mcp parse request", err)
			if writeErr := encoder.Encode(response{
				JSONRPC: "2.0",
				Error:   &rpcError{codeParse, "invalid JSON: " + err.Error()},
			}); writeErr != nil {
				return writeErr
			}
			continue
		}

		resp := s.dispatch(req)
		// A notification has no id and expects no reply.
		if len(req.ID) == 0 {
			continue
		}
		if err := encoder.Encode(resp); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *Server) dispatch(req request) response {
	reply := response{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		s.mu.Lock()
		s.initialized = true
		s.mu.Unlock()
		reply.Result = map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
		}
	case "notifications/initialized":
		return reply
	case "tools/list":
		reply.Result = map[string]any{"tools": s.Tools}
	case "tools/call":
		if s.Log != nil {
			s.Log.Debug("mcp tools/call")
		}
		reply = s.callTool(req, reply)
	case "ping":
		reply.Result = map[string]any{}
	default:
		reply.Error = &rpcError{codeMethodNotFound, "unknown method " + req.Method}
	}
	return reply
}

func (s *Server) callTool(req request, reply response) response {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		reply.Error = &rpcError{codeInvalidRequest, "invalid params: " + err.Error()}
		return reply
	}

	for _, tool := range s.Tools {
		if tool.Name != params.Name {
			continue
		}
		result, err := tool.Handler(params.Arguments)
		if err != nil {
			observability.LogError(s.Log, "mcp tool failed", fmt.Errorf("%s: %w", params.Name, err))
			// A tool failure is reported inside the result rather than as a
			// protocol error, so the model can read and act on it.
			reply.Result = toolResult(fmt.Sprintf("error: %v", err), true)
			return reply
		}
		encoded, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			observability.LogError(s.Log, "mcp encode result", err)
			reply.Error = &rpcError{codeInternal, "encode result: " + err.Error()}
			return reply
		}
		reply.Result = toolResult(string(encoded), false)
		return reply
	}

	reply.Error = &rpcError{codeMethodNotFound, "unknown tool " + params.Name}
	return reply
}

func toolResult(text string, isError bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isError,
	}
}
