package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ToolRegistry struct {
	cfg        *Config
	tools      map[string]ToolDef
	mcpClients []*MCPClient
	fileHashes map[string]uint64  // path -> content hash (xxh64-ish)
	fileData   map[string]string  // path -> last read content
}

type ToolDef struct {
	Name        string
	Description string
	Parameters  map[string]any
	Execute     func(ctx context.Context, args map[string]any) (string, error)
}

func NewToolRegistry(cfg *Config) *ToolRegistry {
	return &ToolRegistry{
		cfg:        cfg,
		tools:      make(map[string]ToolDef),
		fileHashes: make(map[string]uint64),
		fileData:   make(map[string]string),
	}
}

// simpleHash returns a fast 64-bit hash of data
func simpleHash(data []byte) uint64 {
	var h uint64 = 0xcbf29ce484222325
	for _, b := range data {
		h ^= uint64(b)
		h *= 0x100000001b3
	}
	return h
}

// RecordRead saves file content hash and data after read_file
func (r *ToolRegistry) RecordRead(path string, data []byte) {
	r.fileHashes[path] = simpleHash(data)
	r.fileData[path] = string(data)
}

// CheckHash returns nil if file hasn't changed since last RecordRead, error otherwise.
// For new files (never recorded), always returns nil.
func (r *ToolRegistry) CheckHash(path string) error {
	prevHash, ok := r.fileHashes[path]
	if !ok {
		return nil // new file, no prior read
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // file was deleted, that's fine
		}
		return err
	}
	if simpleHash(data) != prevHash {
		return fmt.Errorf("file %s was modified externally since last read_file (hash mismatch)", path)
	}
	return nil
}

// UpdateHash updates hash after successful write/edit
func (r *ToolRegistry) UpdateHash(path string, data []byte) {
	r.fileHashes[path] = simpleHash(data)
	r.fileData[path] = string(data)
}

func (r *ToolRegistry) registerDefaults() {
	ws := &WebSearchTool{}
	r.tools["web_search"] = ws.Definition()
	r.tools["self_modify"] = (&SelfModifyTool{cfg: r.cfg}).Definition()

	if v := r.cfg.Providers["opencode-go"]; v.BaseURL != "" || os.Getenv("SMAGO_OPENCODE_KEY") != "" {
		apiKey := v.APIKey
		if apiKey == "" {
			apiKey = os.Getenv("SMAGO_OPENCODE_KEY")
		}
		base := v.BaseURL
		if base == "" {
			base = "https://opencode.ai/zen/go/v1"
		}
		vt := &VisionTool{APIKey: apiKey, BaseURL: base, Model: "mimo-v2.5", MagickExe: r.cfg.MagickExe}
		r.tools["vision"] = vt.Definition()
	}

	r.tools["terminal"] = ToolDef{
		Name: "terminal", Description: "Run a shell command. Working dir: " + r.cfg.DataDir,
		Parameters: map[string]any{"type": "object", "properties": map[string]any{"command": map[string]any{"type": "string"}}, "required": []string{"command"}},
		Execute: r.execTerminal,
	}
	r.tools["read_file"] = ToolDef{
		Name: "read_file", Description: "Read a file from disk.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []string{"path"}},
		Execute: r.readFile,
	}
	r.tools["write_file"] = ToolDef{
		Name: "write_file", Description: "Write a file. Requires read_file first. Fails if file was modified externally.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}}, "required": []string{"path", "content"}},
		Execute: r.writeFile,
	}
	r.tools["edit_file"] = ToolDef{
		Name: "edit_file", Description: "Edit file by line operations. Requires read_file first. Fails if file changed externally.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{
			"path": map[string]any{"type": "string"}, "action": map[string]any{"type": "string", "enum": []string{"replace", "delete", "insert"}},
			"start": map[string]any{"type": "integer"}, "end": map[string]any{"type": "integer"},
			"content": map[string]any{"type": "string"},
		}, "required": []string{"path", "action", "start"}},
		Execute: r.editFile,
	}
	r.tools["list_dir"] = ToolDef{
		Name: "list_dir", Description: "List files in a directory.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}},
		Execute: r.listDir,
	}
	r.tools["compress"] = ToolDef{Name: "compress", Description: "Compress conversation ranges.", Parameters: compressSchema, Execute: nil}
	r.registerGrep()
	r.connectMCPServers()
}

func (r *ToolRegistry) execTerminal(ctx context.Context, args map[string]any) (string, error) {
	cmd, _ := args["command"].(string)
	if cmd == "" {
		return "", fmt.Errorf("command required")
	}
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	shell := ShellFromContext(ctx)
	name, shellArgs := BuildShellCommand(shell, cmd)
	c := hiddenCmdContext(runCtx, name, shellArgs...)
	c.Dir = r.cfg.DataDir
	out, err := c.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return fmt.Sprintf("ERROR: %v\n%s", err, string(out)), nil
	}
	return strings.TrimSpace(string(out)), nil
}

func (r *ToolRegistry) readFile(ctx context.Context, args map[string]any) (string, error) {
	p, _ := args["path"].(string)
	if p == "" {
		return "", fmt.Errorf("path required")
	}
	full := r.resolvePath(p)
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	r.RecordRead(p, data)
	if len(data) > 50_000 {
		return string(data[:50_000]) + "\n...[truncated]...", nil
	}
	return string(data), nil
}

func (r *ToolRegistry) writeFile(ctx context.Context, args map[string]any) (string, error) {
	p, _ := args["path"].(string)
	if p == "" {
		return "", fmt.Errorf("path required")
	}
	content, _ := args["content"].(string)
	full := r.resolvePath(p)
	dir := filepath.Dir(full)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("directory does not exist: %s", dir)
	}
	if err := r.CheckHash(p); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		return "", err
	}
	r.UpdateHash(p, []byte(content))
	return fmt.Sprintf("ok: wrote %d bytes to %s", len(content), p), nil
}

func (r *ToolRegistry) editFile(ctx context.Context, args map[string]any) (string, error) {
	p, _ := args["path"].(string)
	action, _ := args["action"].(string)
	if p == "" || action == "" {
		return "", fmt.Errorf("path and action required")
	}
	full := r.resolvePath(p)
	if err := r.CheckHash(p); err != nil {
		return "", err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	nLines := len(lines)
	start := toInt(args["start"])
	oldLines := make([]string, len(lines))
	copy(oldLines, lines)

	switch action {
	case "replace":
		if start < 1 {
			return "", fmt.Errorf("start >= 1")
		}
		end := toInt(args["end"])
		if end == 0 {
			end = start
		}
		if end < start {
			return "", fmt.Errorf("end >= start")
		}
		if start > nLines {
			return "", fmt.Errorf("start > %d", nLines)
		}
		content, _ := args["content"].(string)
		lines = append(append(lines[:start-1], strings.Split(content, "\n")...), lines[end:]...)
	case "delete":
		if start < 1 {
			return "", fmt.Errorf("start >= 1")
		}
		end := toInt(args["end"])
		if end == 0 {
			end = start
		}
		if end > nLines {
			end = nLines
		}
		lines = append(lines[:start-1], lines[end:]...)
	case "insert":
		if start < 0 {
			return "", fmt.Errorf("start >= 0")
		}
		content, _ := args["content"].(string)
		lines = append(append(lines[:start], strings.Split(content, "\n")...), lines[start:]...)
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}

	newContent := strings.Join(lines, "\n")
	if err := os.WriteFile(full, []byte(newContent), 0644); err != nil {
		return "", err
	}
	r.UpdateHash(p, []byte(newContent))
	return editDiffSummary(p, action, toInt(args["start"]), oldLines, lines), nil
}

func editDiffSummary(path, action string, startLine int, oldLines, newLines []string) string {
	nOld := len(oldLines)
	nNew := len(newLines)
	delta := nNew - nOld
	sign := "+"
	if delta <= 0 {
		sign = ""
	}
	return fmt.Sprintf("ok: %s on %s line %d (%d->%d lines, %s%d)", action, path, startLine, nOld, nNew, sign, delta)
}

func (r *ToolRegistry) listDir(ctx context.Context, args map[string]any) (string, error) {
	p, _ := args["path"].(string)
	if p == "" {
		p = "."
	}
	full := r.resolvePath(p)
	entries, err := os.ReadDir(full)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			b.WriteString("d  ")
		} else {
			b.WriteString("f  ")
		}
		b.WriteString(e.Name())
		b.WriteString("\n")
	}
	return b.String(), nil
}

func (r *ToolRegistry) resolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(r.cfg.DataDir, p)
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	default:
		return 0
	}
}

func dumpArgs(args map[string]any) string {
	b, _ := json.Marshal(args)
	return string(b)
}

func (r *ToolRegistry) connectMCPServers() {
	if len(r.cfg.MCP) == 0 {
		return
	}
	for name, cfg := range r.cfg.MCP {
		if !cfg.Enabled {
			continue
		}
		log.Printf("mcp: connecting to %s ...", name)
		client, err := NewMCPClient(name, cfg)
		if err != nil {
			log.Printf("mcp: ✗ %s failed: %v", name, err)
			continue
		}
		r.mcpClients = append(r.mcpClients, client)
		tools, err := client.ListTools()
		if err != nil {
			log.Printf("mcp: ✗ %s listTools: %v", name, err)
			continue
		}
		log.Printf("mcp: ✓ %s — %d tool(s)", name, len(tools))
		maxMcpTools := 10
		if maxMcpTools > len(tools) {
			maxMcpTools = len(tools)
		}
		for _, mt := range tools[:maxMcpTools] {
			toolName := name + "__" + mt.Name
			mtCopy := mt
			clientCopy := client
			desc := mtCopy.Description
			if len(desc) > 150 {
				desc = desc[:150] + "…"
			}
			r.tools[toolName] = ToolDef{Name: toolName, Description: desc, Parameters: mtCopy.InputSchema,
				Execute: func(ctx context.Context, args map[string]any) (string, error) {
					return clientCopy.CallTool(ctx, mtCopy.Name, args)
				}}
		}
	}
}

func (r *ToolRegistry) Close() {
	for _, c := range r.mcpClients {
		_ = c.Close()
	}
	r.mcpClients = nil
}

func (r *ToolRegistry) Register(name string, def ToolDef) { r.tools[name] = def }
func (r *ToolRegistry) All() []ToolDef {
	out := make([]ToolDef, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}
func (r *ToolRegistry) Get(name string) (ToolDef, bool) { t, ok := r.tools[name]; return t, ok }
func (r *ToolRegistry) AsLLMTools() []Tool {
	var out []Tool
	for _, t := range r.All() {
		var ti Tool
		ti.Type = "function"
		ti.Function.Name = t.Name
		ti.Function.Description = t.Description
		ti.Function.Parameters = t.Parameters
		out = append(out, ti)
	}
	return out
}
