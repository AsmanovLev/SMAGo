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
	fileHashes map[string]uint64
	fileData   map[string]string
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

func simpleHash(data []byte) uint64 {
	var h uint64 = 0xcbf29ce484222325
	for _, b := range data {
		h ^= uint64(b)
		h *= 0x100000001b3
	}
	return h
}

func (r *ToolRegistry) RecordRead(path string, data []byte) {
	r.fileHashes[path] = simpleHash(data)
	r.fileData[path] = string(data)
}

func (r *ToolRegistry) WasRead(path string) bool {
	_, ok := r.fileHashes[path]
	return ok
}

func (r *ToolRegistry) MarkRead(path string) {
	r.fileHashes[path] = 0
	r.fileData[path] = ""
}

func (r *ToolRegistry) CheckHash(path string) error {
	full := r.resolvePath(path)
	_, statErr := os.Stat(full)
	wasRead := r.WasRead(path)
	if statErr == nil && !wasRead {
		return fmt.Errorf("read_file must be called before write_file/edit_file (file exists, not marked as read)")
	}
	if !wasRead {
		return nil
	}
	expectedHash := r.fileHashes[path]
	data, err := os.ReadFile(full)
	if err != nil {
		return fmt.Errorf("read_file must be called before write_file/edit_file: %w", err)
	}
	if simpleHash(data) != expectedHash {
		return fmt.Errorf("file %s was modified externally since last read_file (hash mismatch)", path)
	}
	return nil
}

func (r *ToolRegistry) UpdateHash(path string, data []byte) {
	r.fileHashes[path] = simpleHash(data)
	r.fileData[path] = string(data)
}

// ============================================================================
// registerDefaults — register all built-in tools
// ============================================================================

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
	r.tools["delete_file"] = ToolDef{
		Name: "delete_file", Description: "Delete a file permanently. Use with caution.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string", "description": "File path to delete"}}, "required": []string{"path"}},
		Execute: r.deleteFile,
	}
	r.tools["compress"] = ToolDef{Name: "compress", Description: "Compress conversation ranges.", Parameters: compressSchema, Execute: nil}
	r.tools["find_files"] = ToolDef{
		Name: "find_files", Description: "Recursive file search with glob patterns. Searches within working directory.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern":    map[string]any{"type": "string", "description": "Glob pattern (e.g. *.go, src/**/*.py)"},
				"max_results": map[string]any{"type": "integer", "description": "Max results (default 50)"},
			},
			"required": []string{"pattern"},
		},
		Execute: r.execFindFiles,
	}
	r.tools["file_info"] = ToolDef{
		Name: "file_info", Description: "Get file/dir metadata (size, permissions, mod time) without reading content.",
		Parameters: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []string{"path"}},
		Execute: r.execFileInfo,
	}
	r.tools["move_file"] = ToolDef{
		Name: "move_file", Description: "Rename or move a file or directory.",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"source": map[string]any{"type": "string", "description": "Source path"},
				"dest":   map[string]any{"type": "string", "description": "Destination path"},
			},
			"required": []string{"source", "dest"},
		},
		Execute: r.execMoveFile,
	}
	r.tools["diff"] = ToolDef{
		Name: "diff", Description: "Show unified diff between two files, or git diff for a path.",
		Parameters: map[string]any{
			"type": "object", "properties": map[string]any{
				"path1": map[string]any{"type": "string", "description": "First file path (or git path if path2 omitted)"},
				"path2": map[string]any{"type": "string", "description": "Second file path (optional — if omitted shows git diff for path1)"},
			},
			"required": []string{"path1"},
		},
		Execute: r.execDiff,
	}

	r.registerGrep()
	r.connectMCPServers()
}

// ============================================================================
// terminal
// ============================================================================

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

// ============================================================================
// read_file
// ============================================================================

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

// ============================================================================
// write_file
// ============================================================================

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

// ============================================================================
// edit_file
// ============================================================================

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
			return "", fmt.Errorf("replace: start must be >= 1 (1-based)")
		}
		end := toInt(args["end"])
		if end == 0 {
			end = start
		}
		if end < start {
			return "", fmt.Errorf("replace: end must be >= start")
		}
		if start > nLines {
			return "", fmt.Errorf("replace: start %d > file has %d lines", start, nLines)
		}
		content, _ := args["content"].(string)
		var result []string
		result = append(result, lines[:start-1]...)
		result = append(result, strings.Split(content, "\n")...)
		result = append(result, lines[end:]...)
		lines = result

	case "delete":
		if start < 1 {
			return "", fmt.Errorf("delete: start must be >= 1 (1-based)")
		}
		if start > nLines {
			return "", fmt.Errorf("delete: start %d > file has %d lines", start, nLines)
		}
		end := toInt(args["end"])
		if end == 0 {
			end = start
		}
		if end > nLines {
			end = nLines
		}
		var result []string
		result = append(result, lines[:start-1]...)
		result = append(result, lines[end:]...)
		lines = result

	case "insert":
		if start < 0 {
			return "", fmt.Errorf("insert: start must be >= 0 (0-based, insert after line N)")
		}
		if start > nLines {
			return "", fmt.Errorf("insert: start %d > file has %d lines", start, nLines)
		}
		content, _ := args["content"].(string)
		var result []string
		result = append(result, lines[:start]...)
		result = append(result, strings.Split(content, "\n")...)
		result = append(result, lines[start:]...)
		lines = result

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

// ============================================================================
// list_dir
// ============================================================================

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

// ============================================================================
// delete_file
// ============================================================================

func (r *ToolRegistry) deleteFile(ctx context.Context, args map[string]any) (string, error) {
	p, _ := args["path"].(string)
	if p == "" {
		return "", fmt.Errorf("path required")
	}
	full := r.resolvePath(p)
	info, err := os.Stat(full)
	if err != nil {
		return "", fmt.Errorf("delete: %w", err)
	}
	if info.IsDir() {
		if err := os.RemoveAll(full); err != nil {
			return "", fmt.Errorf("delete dir: %w", err)
		}
		return fmt.Sprintf("ok: deleted directory %s (with contents)", p), nil
	}
	if err := os.Remove(full); err != nil {
		return "", fmt.Errorf("delete: %w", err)
	}
	delete(r.fileHashes, p)
	delete(r.fileData, p)
	return fmt.Sprintf("ok: deleted file %s", p), nil
}

// ============================================================================
// find_files
// ============================================================================

func (r *ToolRegistry) execFindFiles(ctx context.Context, args map[string]any) (string, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		return "", fmt.Errorf("pattern required")
	}
	maxResults := toInt(args["max_results"])
	if maxResults == 0 {
		maxResults = 50
	}
	matches, err := filepath.Glob(filepath.Join(r.cfg.DataDir, pattern))
	if err != nil {
		return "", fmt.Errorf("find_files: %w", err)
	}
	if len(matches) == 0 {
		return fmt.Sprintf("no files matching %q", pattern), nil
	}
	var b strings.Builder
	count := 0
	for _, m := range matches {
		if count >= maxResults {
			fmt.Fprintf(&b, "\n... (%d more omitted)", len(matches)-count)
			break
		}
		rel, _ := filepath.Rel(r.cfg.DataDir, m)
		if rel == "" {
			rel = m
		}
		info, err := os.Stat(m)
		if err == nil {
			size := info.Size()
			mod := info.ModTime().Format("2006-01-02 15:04")
			if info.IsDir() {
				b.WriteString("  d  ")
			} else {
				b.WriteString("  f  ")
			}
			fmt.Fprintf(&b, "%s  %s  %d\n", mod, rel, size)
		} else {
			b.WriteString("     " + rel + "\n")
		}
		count++
	}
	return fmt.Sprintf("%d files matching %q:\n%s", len(matches), pattern, b.String()), nil
}

// ============================================================================
// file_info
// ============================================================================

func (r *ToolRegistry) execFileInfo(ctx context.Context, args map[string]any) (string, error) {
	p, _ := args["path"].(string)
	if p == "" {
		return "", fmt.Errorf("path required")
	}
	full := r.resolvePath(p)
	info, err := os.Stat(full)
	if err != nil {
		return "", fmt.Errorf("file_info: %w", err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "path:     %s\n", p)
	fmt.Fprintf(&b, "size:     %d bytes\n", info.Size())
	fmt.Fprintf(&b, "modified: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&b, "mode:     %s\n", info.Mode().Perm())
	if info.IsDir() {
		b.WriteString("type:     directory\n")
	} else {
		b.WriteString("type:     file\n")
	}
	return b.String(), nil
}

// ============================================================================
// move_file
// ============================================================================

func (r *ToolRegistry) execMoveFile(ctx context.Context, args map[string]any) (string, error) {
	src, _ := args["source"].(string)
	dst, _ := args["dest"].(string)
	if src == "" || dst == "" {
		return "", fmt.Errorf("source and dest required")
	}
	srcFull := r.resolvePath(src)
	dstFull := r.resolvePath(dst)
	dstDir := filepath.Dir(dstFull)
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		return "", fmt.Errorf("destination directory does not exist: %s", dstDir)
	}
	if err := os.Rename(srcFull, dstFull); err != nil {
		return "", fmt.Errorf("move: %w", err)
	}
	if h, ok := r.fileHashes[src]; ok {
		r.fileHashes[dst] = h
		delete(r.fileHashes, src)
	}
	if d, ok := r.fileData[src]; ok {
		r.fileData[dst] = d
		delete(r.fileData, src)
	}
	return fmt.Sprintf("ok: moved %s -> %s", src, dst), nil
}

// ============================================================================
// diff
// ============================================================================

func (r *ToolRegistry) execDiff(ctx context.Context, args map[string]any) (string, error) {
	path1, _ := args["path1"].(string)
	path2, _ := args["path2"].(string)
	if path1 == "" {
		return "", fmt.Errorf("path1 required")
	}
	if path2 == "" {
		full := r.resolvePath(path1)
		rel, _ := filepath.Rel(r.cfg.DataDir, full)
		shell := ShellFromContext(ctx)
		name, shellArgs := BuildShellCommand(shell, fmt.Sprintf("git diff -- '%s'", rel))
		c := hiddenCmdContext(ctx, name, shellArgs...)
		c.Dir = r.cfg.DataDir
		c.Env = append(os.Environ(), "GIT_PAGER=cat")
		out, err := c.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("git diff output:\n%s", string(out)), nil
		}
		result := strings.TrimSpace(string(out))
		if result == "" {
			return "no uncommitted changes", nil
		}
		return result, nil
	}
	full1 := r.resolvePath(path1)
	full2 := r.resolvePath(path2)
	data1, err := os.ReadFile(full1)
	if err != nil {
		return "", fmt.Errorf("diff: %w", err)
	}
	data2, err := os.ReadFile(full2)
	if err != nil {
		return "", fmt.Errorf("diff: %w", err)
	}
	lines1 := strings.Split(string(data1), "\n")
	lines2 := strings.Split(string(data2), "\n")
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n+++ %s\n", path1, path2)
	maxLen := len(lines1)
	if len(lines2) > maxLen {
		maxLen = len(lines2)
	}
	changed := false
	for i := 0; i < maxLen; i++ {
		l1 := ""
		l2 := ""
		if i < len(lines1) {
			l1 = lines1[i]
		}
		if i < len(lines2) {
			l2 = lines2[i]
		}
		if l1 != l2 {
			changed = true
			if i < len(lines1) {
				fmt.Fprintf(&b, "-%s\n", l1)
			}
			if i < len(lines2) {
				fmt.Fprintf(&b, "+%s\n", l2)
			}
		}
	}
	if !changed {
		b.WriteString("(identical)\n")
	}
	return b.String(), nil
}

// ============================================================================
// helpers
// ============================================================================

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

// ============================================================================
// MCP
// ============================================================================

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
