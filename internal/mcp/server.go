package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

const ProtocolVersion = "2024-11-05"

type ServerInfo struct {
	Name    string
	Version string
}

type Server struct {
	info  ServerInfo
	mu    sync.RWMutex
	tools map[string]Tool
}

type Tool struct {
	Name        string
	Description string
	InputSchema json.RawMessage
	Handler     func(ctx context.Context, args json.RawMessage) (any, error)
}

func NewServer(info ServerInfo) *Server {
	if info.Version == "" {
		info.Version = "0.0.0"
	}
	return &Server{info: info, tools: map[string]Tool{}}
}

func (s *Server) Register(t Tool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools[t.Name] = t
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
)

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	enc := json.NewEncoder(out)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: errParseError, Message: err.Error()},
			})
			continue
		}
		resp := s.dispatch(ctx, req)
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (s *Server) dispatch(ctx context.Context, req rpcRequest) rpcResponse {
	base := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		base.Result = map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]any{
				"name":    s.info.Name,
				"version": s.info.Version,
			},
		}
		return base
	case "tools/list":
		s.mu.RLock()
		defer s.mu.RUnlock()
		out := make([]map[string]any, 0, len(s.tools))
		for _, t := range s.tools {
			out = append(out, map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"inputSchema": t.InputSchema,
			})
		}
		base.Result = map[string]any{"tools": out}
		return base
	case "tools/call":
		return s.callTool(ctx, base, req.Params)
	default:
		base.Error = &rpcError{Code: errMethodNotFound, Message: "method not found: " + req.Method}
		return base
	}
}

func (s *Server) callTool(ctx context.Context, base rpcResponse, raw json.RawMessage) rpcResponse {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		base.Error = &rpcError{Code: errInvalidParams, Message: err.Error()}
		return base
	}
	s.mu.RLock()
	tool, ok := s.tools[p.Name]
	s.mu.RUnlock()
	if !ok {
		base.Error = &rpcError{Code: errMethodNotFound, Message: "tool not found: " + p.Name}
		return base
	}
	result, err := tool.Handler(ctx, p.Arguments)
	if err != nil {
		base.Error = &rpcError{Code: errInternal, Message: err.Error()}
		return base
	}
	base.Result = map[string]any{
		"content": []map[string]any{
			{"type": "text", "text": toText(result)},
		},
		"isError":           false,
		"structuredContent": result,
	}
	return base
}

func toText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
