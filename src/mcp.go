package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
)

// MCPClient speaks the Model Context Protocol over stdio (JSON-RPC 2.0
// newline-delimited). Supports auto-reconnect on process death.
type MCPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stderr io.ReadCloser

	name    string
	version string
	cfg     MCPServerConfig

	mu      sync.Mutex
	nextID  int
	pending map[int]chan json.RawMessage

	aliveMu sync.RWMutex
	alive   bool
}

// MCPServerConfig is one entry from the agent config's "mcp" map.
type MCPServerConfig struct {
	Command []string          `json:"command"`
	Enabled bool              `json:"enabled"`
	Env     map[string]string `json:"env"`
}

// MCPTool is what we hand back to the ToolRegistry.
type MCPTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpInitializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

type mcpListToolsResult struct {
	Tools []MCPTool `json:"tools"`
}

type mcpCallResult struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	IsError bool `json:"isError"`
}

// NewMCPClient spawns the server process and runs the initialize handshake.
func NewMCPClient(name string, cfg MCPServerConfig) (*MCPClient, error) {
	if len(cfg.Command) == 0 {
		return nil, fmt.Errorf("mcp %q: empty command", name)
	}
	c := &MCPClient{
		name:    name,
		cfg:     cfg,
		pending: make(map[int]chan json.RawMessage),
	}
	if err := c.spawn(); err != nil {
		return nil, err
	}
	return c, nil
}

// spawn starts the MCP server process and runs the initialize handshake.
func (c *MCPClient) spawn() error {
	cmd := exec.Command(c.cfg.Command[0], c.cfg.Command[1:]...)
	env := cmd.Environ()
	for k, v := range c.cfg.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn %s: %w", c.name, err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stderr = stderr

	c.mu.Lock()
	c.pending = make(map[int]chan json.RawMessage)
	c.mu.Unlock()
	c.setAlive(true)

	go c.readLoop(bufio.NewReader(stdout))
	go c.drainStderr()

	// Initialize handshake
	res, err := c.sendRaw(context.Background(), "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "smago", "version": "0.1.0"},
	})
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		c.setAlive(false)
		return fmt.Errorf("initialize %s: %w", c.name, err)
	}
	var init mcpInitializeResult
	if err := json.Unmarshal(res, &init); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		c.setAlive(false)
		return fmt.Errorf("parse init: %w", err)
	}
	c.version = init.ServerInfo.Version
	log.Printf("mcp %s: connected (%s)", c.name, init.ServerInfo.Name)
	c.notify("notifications/initialized", map[string]any{})
	return nil
}

func (c *MCPClient) setAlive(v bool) {
	c.aliveMu.Lock()
	c.alive = v
	c.aliveMu.Unlock()
}

func (c *MCPClient) isAlive() bool {
	c.aliveMu.RLock()
	defer c.aliveMu.RUnlock()
	return c.alive
}

// reconnect tears down the dead process and respawns it.
func (c *MCPClient) reconnect() error {
	log.Printf("mcp[%s]: reconnecting...", c.name)
	c.cleanup()
	return c.spawn()
}

func (c *MCPClient) cleanup() {
	c.setAlive(false)
	c.mu.Lock()
	for id, ch := range c.pending {
		close(ch)
		delete(c.pending, id)
	}
	c.mu.Unlock()
	if c.stdin != nil {
		_ = c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	if c.cmd != nil {
		_ = c.cmd.Wait()
	}
	c.cmd = nil
	c.stdin = nil
	c.stderr = nil
}

// readLoop reads lines from stdout and dispatches responses by ID.
func (c *MCPClient) readLoop(reader *bufio.Reader) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("mcp[%s]: readLoop ended: %v", c.name, err)
			c.setAlive(false)
			c.mu.Lock()
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.mu.Unlock()
			return
		}
		var msg struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			log.Printf("mcp[%s] non-JSON: %s", c.name, line)
			continue
		}
		c.mu.Lock()
		ch, ok := c.pending[msg.ID]
		c.mu.Unlock()
		if ok {
			if msg.Error != nil {
				errJSON, _ := json.Marshal(msg.Error)
				ch <- errJSON
			} else {
				ch <- msg.Result
			}
		}
	}
}

func (c *MCPClient) drainStderr() {
	r := bufio.NewReader(c.stderr)
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			log.Printf("mcp[%s] %s", c.name, line)
		}
		if err != nil {
			return
		}
	}
}

func (c *MCPClient) notify(method string, params any) {
	data, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
	data = append(data, '\n')
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stdin != nil {
		_, _ = c.stdin.Write(data)
	}
}

// sendRaw does a single RPC call without auto-reconnect.
func (c *MCPClient) sendRaw(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	data, _ := json.Marshal(req)
	data = append(data, '\n')

	c.mu.Lock()
	if c.stdin == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("mcp %s: stdin closed", c.name)
	}
	_, err := c.stdin.Write(data)
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case raw, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("mcp %s: connection lost", c.name)
		}
		var errCheck struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(raw, &errCheck); err == nil && errCheck.Code != 0 {
			return nil, fmt.Errorf("mcp error %d: %s", errCheck.Code, errCheck.Message)
		}
		return raw, nil
	case <-time.After(120 * time.Second):
		return nil, fmt.Errorf("mcp %s: timeout waiting for response to %s", c.name, method)
	}
}

// send performs an RPC call, auto-reconnecting once if the server is dead.
func (c *MCPClient) send(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if !c.isAlive() {
		if err := c.reconnect(); err != nil {
			return nil, fmt.Errorf("mcp %s: reconnect failed: %w", c.name, err)
		}
	}

	res, err := c.sendRaw(ctx, method, params)
	if err != nil && !c.isAlive() {
		// Connection died mid-call — reconnect and retry once
		log.Printf("mcp[%s]: call failed (%v), reconnecting...", c.name, err)
		if rerr := c.reconnect(); rerr != nil {
			return nil, fmt.Errorf("mcp %s: reconnect failed: %w", c.name, rerr)
		}
		return c.sendRaw(ctx, method, params)
	}
	return res, err
}

func (c *MCPClient) ListTools() ([]MCPTool, error) {
	res, err := c.send(context.Background(), "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var lt mcpListToolsResult
	if err := json.Unmarshal(res, &lt); err != nil {
		return nil, err
	}
	return lt.Tools, nil
}

func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]any) (string, error) {
	res, err := c.send(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}
	var cr mcpCallResult
	if err := json.Unmarshal(res, &cr); err != nil {
		return "", err
	}
	var out string
	for i, ct := range cr.Content {
		if ct.Type == "text" {
			if i > 0 {
				out += "\n"
			}
			out += ct.Text
		}
	}
	if cr.IsError && out == "" {
		out = "(tool reported error, no message)"
	}
	return out, nil
}

func (c *MCPClient) Close() error {
	c.cleanup()
	return nil
}
