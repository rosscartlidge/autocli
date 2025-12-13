package completionflags

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Completer provides completion suggestions for flag arguments
type Completer interface {
	Complete(ctx CompletionContext) ([]string, error)
}

// CompletionContext provides everything needed for completion
type CompletionContext struct {
	// What we're completing
	Partial      string

	// Position
	Args         []string
	Position     int

	// Flag context
	FlagName     string
	ArgIndex     int             // Which argument of the flag (0-based)
	PreviousArgs []string        // Previous args of this multi-arg flag

	// Command state
	Command         *Command
	CurrentClause   *Clause
	ParsedClauses   []Clause
	GlobalFlags     map[string]interface{}
}

// CompletionFunc is a function-based completer
type CompletionFunc func(ctx CompletionContext) ([]string, error)

// Complete implements Completer interface
func (f CompletionFunc) Complete(ctx CompletionContext) ([]string, error) {
	return f(ctx)
}

// FileCompleter completes file and directory names with optional pattern filtering
type FileCompleter struct {
	Pattern  string  // Glob pattern: "*.txt", "*.{json,yaml}", etc.
	DirsOnly bool    // Only complete directories
	Hint     string  // Hint to show when no files match (default: "<FILE>")
}

// Complete implements Completer interface
func (fc *FileCompleter) Complete(ctx CompletionContext) ([]string, error) {
	partial := ctx.Partial

	// Determine directory and file pattern
	var dir string
	var pattern string

	if strings.HasSuffix(partial, "/") {
		dir = partial
		pattern = ""
	} else if strings.Contains(partial, "/") {
		dir = filepath.Dir(partial)
		pattern = filepath.Base(partial)
	} else {
		dir = "."
		pattern = partial
	}

	// Read directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Directory doesn't exist or can't be read
		// Return hint if available
		if fc.Hint != "" {
			hint := fc.buildHint(dir)
			return []string{hint}, nil
		}
		return []string{}, nil
	}

	var matches []string

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless explicitly requested
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(pattern, ".") {
			continue
		}

		// Check if name matches partial pattern
		if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(pattern)) {
			continue
		}

		// Build full path
		var fullPath string
		if dir == "." {
			fullPath = name
		} else {
			fullPath = filepath.Join(dir, name)
		}

		// Handle directories
		if entry.IsDir() {
			fullPath += "/"
			matches = append(matches, fullPath)
			continue
		}

		// Skip files if DirsOnly
		if fc.DirsOnly {
			continue
		}

		// Apply pattern filtering if specified
		if fc.Pattern != "" {
			if !matchesPattern(name, fc.Pattern) {
				continue
			}
		}

		matches = append(matches, fullPath)
	}

	// If no matches and a hint is provided, return the hint with the directory path
	if len(matches) == 0 && fc.Hint != "" {
		hint := fc.buildHint(dir)
		return []string{hint}, nil
	}

	// If single data file match, emit field cache directive for downstream commands
	// This allows pipelines like: ssql from users.csv<TAB> | ssql where -where <TAB>
	// to have field completion without explicit -cache DONE step
	if len(matches) == 1 && !strings.HasSuffix(matches[0], "/") && isDataFile(matches[0]) {
		fields, err := extractFields(matches[0])
		if err == nil && len(fields) > 0 {
			absPath, _ := filepath.Abs(matches[0])
			directive := CompletionDirective{
				Type:     "field_cache",
				Fields:   fields,
				Filepath: absPath,
			}
			directiveJSON, err := json.Marshal(directive)
			if err == nil {
				// Prepend directive to matches
				return append([]string{string(directiveJSON)}, matches...), nil
			}
		}
	}

	return matches, nil
}

// isDataFile checks if a file extension indicates a data file (CSV, TSV, JSON, JSONL)
func isDataFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".csv", ".tsv", ".json", ".jsonl", ".ndjson":
		return true
	default:
		return false
	}
}

// buildHint constructs a hint showing the expected file pattern
func (fc *FileCompleter) buildHint(dir string) string {
	var hint string

	// If there's a pattern, show it in angle brackets
	// Escape brace expansion to prevent compgen from expanding it
	if fc.Pattern != "" {
		// Replace { and } with escaped versions to prevent bash expansion
		pattern := strings.ReplaceAll(fc.Pattern, "{", "\\{")
		pattern = strings.ReplaceAll(pattern, "}", "\\}")
		hint = "<" + pattern + ">"
	} else {
		hint = fc.Hint
	}

	// Prepend directory path if not current directory
	if dir != "." {
		hint = filepath.Join(dir, hint)
	}

	return hint
}

// StaticCompleter completes from a fixed list of options
type StaticCompleter struct {
	Options []string
}

// Complete implements Completer interface
func (sc *StaticCompleter) Complete(ctx CompletionContext) ([]string, error) {
	var matches []string
	partial := strings.ToLower(ctx.Partial)

	for _, option := range sc.Options {
		if strings.HasPrefix(strings.ToLower(option), partial) {
			matches = append(matches, option)
		}
	}

	return matches, nil
}

// ChainCompleter tries multiple completers in order and returns first non-empty result
type ChainCompleter struct {
	Completers []Completer
}

// Complete implements Completer interface
func (cc *ChainCompleter) Complete(ctx CompletionContext) ([]string, error) {
	for _, completer := range cc.Completers {
		results, err := completer.Complete(ctx)
		if err != nil {
			continue // Try next completer on error
		}
		if len(results) > 0 {
			return results, nil
		}
	}
	return []string{}, nil
}

// DynamicCompleter chooses a completer based on context
type DynamicCompleter struct {
	Chooser func(ctx CompletionContext) Completer
}

// Complete implements Completer interface
func (dc *DynamicCompleter) Complete(ctx CompletionContext) ([]string, error) {
	completer := dc.Chooser(ctx)
	if completer == nil {
		return []string{}, nil
	}
	return completer.Complete(ctx)
}

// NoCompleter explicitly provides no completions
// Optionally shows a hint message to the user
type NoCompleter struct {
	Hint string // Optional hint to display (e.g., "<VALUE>", "<NUMBER>")
}

// Complete implements Completer interface
func (nc NoCompleter) Complete(ctx CompletionContext) ([]string, error) {
	if nc.Hint != "" {
		return []string{nc.Hint}, nil
	}
	return []string{}, nil
}

// matchesPattern checks if a filename matches a glob pattern
// Supports patterns like "*.txt", "*.{json,yaml}", ".[tc]sv", etc.
func matchesPattern(filename, pattern string) bool {
	filename = strings.ToLower(filename)
	pattern = strings.ToLower(pattern)

	// Handle brace expansion: .{json,yaml}
	if strings.Contains(pattern, "{") && strings.Contains(pattern, "}") {
		return matchesBracePattern(filename, pattern)
	}

	// Use filepath.Match for glob patterns
	matched, err := filepath.Match(pattern, filename)
	if err != nil {
		// If pattern is invalid, fall back to simple suffix match
		return strings.HasSuffix(filename, pattern)
	}
	return matched
}

// matchesBracePattern handles brace expansion patterns like .{json,yaml}
func matchesBracePattern(filename, pattern string) bool {
	start := strings.Index(pattern, "{")
	end := strings.Index(pattern, "}")
	if start == -1 || end == -1 || start >= end {
		return false
	}

	prefix := pattern[:start]
	suffix := pattern[end+1:]
	options := strings.Split(pattern[start+1:end], ",")

	// Test each option
	for _, option := range options {
		testPattern := prefix + strings.TrimSpace(option) + suffix
		if matched, err := filepath.Match(testPattern, filename); err == nil && matched {
			return true
		}
	}

	return false
}

// DurationCompleter suggests common duration values
type DurationCompleter struct {
	Suggestions []string // Custom suggestions (optional)
}

// Complete implements Completer interface
func (dc *DurationCompleter) Complete(ctx CompletionContext) ([]string, error) {
	suggestions := dc.Suggestions
	if len(suggestions) == 0 {
		// Default common durations
		suggestions = []string{
			"1s", "5s", "10s", "30s",
			"1m", "5m", "10m", "30m",
			"1h", "2h", "6h", "12h", "24h",
		}
	}

	var matches []string
	partial := strings.ToLower(ctx.Partial)

	for _, suggestion := range suggestions {
		if strings.HasPrefix(strings.ToLower(suggestion), partial) {
			matches = append(matches, suggestion)
		}
	}

	// If no matches and user has typed something, show hint
	if len(matches) == 0 && ctx.Partial != "" {
		return []string{"<DURATION>"}, nil
	}

	return matches, nil
}
