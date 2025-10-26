package completionflags

import "fmt"

// Command represents a CLI command with flags, clauses, and completion support
type Command struct {
	name          string
	version       string
	description   string
	author        string
	flags         []*FlagSpec
	handler       ClauseHandlerFunc
	separators    []string
	prefixHandler PrefixHandler
	examples      []Example
}

// FlagSpec defines a flag with 0 or more arguments
type FlagSpec struct {
	Names       []string    // e.g., ["-input", "-i"]
	Description string      // For help and man pages
	Scope       Scope       // Global or Local (for clauses)

	// Arguments (can be 0 for boolean flags, 1+ for others)
	ArgCount      int
	ArgNames      []string      // Descriptive names: ["FILE", "PATTERN"]
	ArgTypes      []ArgType     // Type of each argument
	ArgCompleters []Completer   // Completion strategy for each argument

	// Binding
	Pointer     interface{}   // Where to store parsed value
	IsSlice     bool          // Accumulate multiple values

	// Validation and defaults
	Required    bool
	Default     interface{}
	Validator   ValidatorFunc

	// Display
	Hidden      bool          // Hide from help/man (for internal flags)
}

// ArgType represents the type of a flag argument
type ArgType int

const (
	ArgString ArgType = iota
	ArgInt
	ArgFloat
	ArgBool
)

// Scope determines if flag is global or per-clause
type Scope int

const (
	ScopeGlobal Scope = iota  // Applies to entire command
	ScopeLocal                // Applies within each clause
)

// Clause represents a group of arguments separated by + or -
type Clause struct {
	Separator  string                    // "+" or "-" that started this clause (empty for first)
	Flags      map[string]interface{}    // Parsed flag values for this clause
	Positional []string                  // Unparsed positional arguments in this clause
}

// Context is passed to handler with all parsed clauses
type Context struct {
	Command     *Command
	Clauses     []Clause                  // All parsed clauses
	GlobalFlags map[string]interface{}    // Flags marked as global (apply to all clauses)
	RawArgs     []string                  // Original arguments
}

// ClauseHandlerFunc processes all clauses
type ClauseHandlerFunc func(ctx *Context) error

// PrefixHandler interprets - vs + prefix on flags
// flagName: the canonical flag name (e.g., "-verbose")
// hasPlus: true if flag was specified with + prefix (e.g., "+verbose")
// value: the parsed value
// Returns: the value to actually store (can modify, wrap, or return as-is)
type PrefixHandler func(flagName string, hasPlus bool, value interface{}) interface{}

// ValidatorFunc validates a parsed value
type ValidatorFunc func(value interface{}) error

// Example represents a usage example for help and man pages
type Example struct {
	Command     string
	Description string
}

// ParseError represents an error during argument parsing
type ParseError struct {
	Flag    string
	Message string
}

func (e ParseError) Error() string {
	if e.Flag != "" {
		return fmt.Sprintf("flag %s: %s", e.Flag, e.Message)
	}
	return e.Message
}

// ValidationError represents a validation error
type ValidationError struct {
	Flag    string
	Message string
}

func (e ValidationError) Error() string {
	if e.Flag != "" {
		return fmt.Sprintf("validation failed for %s: %s", e.Flag, e.Message)
	}
	return fmt.Sprintf("validation failed: %s", e.Message)
}

// defaultPrefixHandler is used when no custom prefix handler is set
// It simply returns the value unchanged, ignoring the prefix
func defaultPrefixHandler(flagName string, hasPlus bool, value interface{}) interface{} {
	return value
}

// defaultSeparators are the default clause separators
func defaultSeparators() []string {
	return []string{"+", "-"}
}

// isSeparator checks if a string is a clause separator
func (cmd *Command) isSeparator(s string) bool {
	for _, sep := range cmd.separators {
		if s == sep {
			return true
		}
	}
	return false
}

// findFlagSpec finds a flag spec by any of its names
func (cmd *Command) findFlagSpec(name string) *FlagSpec {
	for _, spec := range cmd.flags {
		for _, n := range spec.Names {
			if n == name {
				return spec
			}
		}
	}
	return nil
}

// getPrefixHandler returns the command's prefix handler or default
func (cmd *Command) getPrefixHandler() PrefixHandler {
	if cmd.prefixHandler != nil {
		return cmd.prefixHandler
	}
	return defaultPrefixHandler
}
