package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
)

func (r *ToolRegistry) registerGrep() {
	r.tools["grep"] = ToolDef{
		Name:        "grep",
		Description: "Search for pattern in a file. Returns matching lines with line numbers. Set is_regex=true for regexp.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":     map[string]any{"type": "string"},
				"pattern":  map[string]any{"type": "string"},
				"is_regex": map[string]any{"type": "boolean"},
			},
			"required": []string{"path", "pattern"},
		},
		Execute: r.execGrep,
	}
}

func (r *ToolRegistry) execGrep(ctx context.Context, args map[string]any) (string, error) {
	p, _ := args["path"].(string)
	pattern, _ := args["pattern"].(string)
	isRegex, _ := args["is_regex"].(bool)
	if p == "" || pattern == "" {
		return "", fmt.Errorf("path and pattern required")
	}
	full := r.resolvePath(p)
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")
	var b strings.Builder
	count := 0
	for i, line := range lines {
		matched := false
		if isRegex {
			re, reErr := regexp.Compile(pattern)
			if reErr == nil {
				matched = re.MatchString(line)
			}
		} else {
			matched = strings.Contains(line, pattern)
		}
		if matched {
			trimmed := line
			if len(trimmed) > 120 {
				trimmed = trimmed[:120] + "..."
			}
			fmt.Fprintf(&b, "%d: %s\n", i+1, trimmed)
			count++
			if count > 200 {
				fmt.Fprintf(&b, "... (>200 matches)\n")
				break
			}
		}
	}
	if count == 0 {
		return fmt.Sprintf("no matches for %q in %s", pattern, p), nil
	}
	return fmt.Sprintf("%d matches:\n%s", count, b.String()), nil
}
