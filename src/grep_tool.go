package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func (r *ToolRegistry) registerGrep() {
	r.tools["grep"] = ToolDef{
		Name:        "grep",
		Description: "Search for pattern in a file or directory. Returns matching lines with line numbers. Set is_regex=true for regexp. Supports directories (recursive) and glob patterns.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string", "description": "File, directory, or glob pattern to search"},
				"pattern":  map[string]any{"type": "string", "description": "Search pattern"},
				"is_regex": map[string]any{"type": "boolean", "description": "Use regex matching"},
				"glob":     map[string]any{"type": "string", "description": "File glob filter when searching directories (e.g. '*.go')"},
			},
			"required": []string{"path", "pattern"},
		},
		Execute: r.execGrep,
	}
}

// isDir returns true if path is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// containsGlobMeta returns true if the path contains glob meta characters.
func containsGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

func (r *ToolRegistry) execGrep(ctx context.Context, args map[string]any) (string, error) {
	p, _ := args["path"].(string)
	pattern, _ := args["pattern"].(string)
	isRegex, _ := args["is_regex"].(bool)
	globFilter, _ := args["glob"].(string)
	if p == "" || pattern == "" {
		return "", fmt.Errorf("path and pattern required")
	}

	full := r.resolvePath(p)

	// Compile regex once if needed
	var re *regexp.Regexp
	if isRegex {
		var err error
		re, err = regexp.Compile(pattern)
		if err != nil {
			return "", fmt.Errorf("invalid regex: %v", err)
		}
	}

	// Single file mode
	if !isDir(full) && !containsGlobMeta(full) {
		return r.grepFile(full, pattern, re)
	}

	// Directory or glob mode — collect files
	var files []string
	if containsGlobMeta(p) || containsGlobMeta(full) {
		// Glob pattern
		matches, err := filepath.Glob(full)
		if err != nil {
			return "", fmt.Errorf("glob error: %v", err)
		}
		files = matches
	} else {
		// Recursive walk
		err := filepath.WalkDir(full, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if globFilter != "" {
				matched, _ := filepath.Match(globFilter, d.Name())
				if !matched {
					return nil
				}
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			return "", fmt.Errorf("walk error: %v", err)
		}
	}

	if len(files) == 0 {
		return fmt.Sprintf("no files found matching %q", p), nil
	}

	var b strings.Builder
	totalMatches := 0
	filesMatched := 0
	for _, f := range files {
		if totalMatches >= 500 {
			fmt.Fprintf(&b, "\n... (>500 matches across %d files, truncated)\n", filesMatched)
			break
		}
		result, count := grepSingleFile(f, pattern, re, 200-totalMatches)
		if count > 0 {
			rel, _ := filepath.Rel(r.cfg.DataDir, f)
			if rel == "" {
				rel = f
			}
			if filesMatched > 0 {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "%s (%d matches):\n", rel, count)
			b.WriteString(result)
			totalMatches += count
			filesMatched++
		}
	}

	if totalMatches == 0 {
		return fmt.Sprintf("no matches for %q in %s", pattern, p), nil
	}
	return fmt.Sprintf("%d matches across %d files:\n%s", totalMatches, filesMatched, b.String()), nil
}

func (r *ToolRegistry) grepFile(full, pattern string, re *regexp.Regexp) (string, error) {
	result, count := grepSingleFile(full, pattern, re, 200)
	if count == 0 {
		return fmt.Sprintf("no matches for %q in %s", pattern, full), nil
	}
	return fmt.Sprintf("%d matches:\n%s", count, result), nil
}

func grepSingleFile(path, pattern string, re *regexp.Regexp, maxMatches int) (string, int) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0
	}
	lines := strings.Split(string(data), "\n")
	var b strings.Builder
	count := 0
	for i, line := range lines {
		if count >= maxMatches {
			break
		}
		matched := false
		if re != nil {
			matched = re.MatchString(line)
		} else {
			matched = strings.Contains(line, pattern)
		}
		if matched {
			trimmed := line
			if len(trimmed) > 150 {
				trimmed = trimmed[:150] + "..."
			}
			fmt.Fprintf(&b, "%d: %s\n", i+1, trimmed)
			count++
		}
	}
	return b.String(), count
}
