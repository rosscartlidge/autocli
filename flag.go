package completionflags

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Version is the completionflags library version
const Version = "2.0.0"

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
	subcommands   map[string]*Subcommand // Subcommands for this command
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

	// Accumulation
	IsSlice     bool          // Accumulate multiple values (for Accumulate() method)

	// Positional arguments
	IsVariadic  bool          // Consumes all remaining positional args (must be last)

	// Validation and defaults
	Required    bool
	Default     interface{}
	Validator   ValidatorFunc

	// Time parsing (for ArgTime type)
	TimeFormats      []string // Multiple formats to try in order (uses time.ParseInLocation)
	TimeZone         string   // IANA timezone name or "Local" for formats without TZ info
	TimeZoneFromFlag string   // Flag name to get timezone from (must be Global flag, e.g., "-timezone")

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
	ArgDuration // time.Duration parsed with time.ParseDuration
	ArgTime     // time.Time parsed with time.ParseInLocation
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
	Command        *Command
	Subcommand     string                    // Name of subcommand being executed (empty for root)
	Clauses        []Clause                  // All parsed clauses
	GlobalFlags    map[string]interface{}    // Flags marked as global (apply to all clauses)
	RemainingArgs  []string                  // Arguments after -- (everything after -- is literal)
	RawArgs        []string                  // Original arguments
	deferredValues map[string]*deferredValue // Values that need re-parsing after all flags known
}

// deferredValue tracks a value that needs re-parsing after all flags are available
type deferredValue struct {
	rawString   string
	spec        *FlagSpec
	isGlobal    bool
	clauseIndex int // For local flags, which clause
}

// Context helper methods for type-safe flag value extraction

// GetBool retrieves a boolean flag value from GlobalFlags, returning defaultValue if not found or nil
func (ctx *Context) GetBool(name string, defaultValue bool) bool {
	if v, ok := ctx.GlobalFlags[name]; ok && v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// GetString retrieves a string flag value from GlobalFlags, returning defaultValue if not found or nil
func (ctx *Context) GetString(name string, defaultValue string) string {
	if v, ok := ctx.GlobalFlags[name]; ok && v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

// GetInt retrieves an int flag value from GlobalFlags, returning defaultValue if not found or nil
func (ctx *Context) GetInt(name string, defaultValue int) int {
	if v, ok := ctx.GlobalFlags[name]; ok && v != nil {
		if i, ok := v.(int); ok {
			return i
		}
	}
	return defaultValue
}

// GetFloat retrieves a float64 flag value from GlobalFlags, returning defaultValue if not found or nil
func (ctx *Context) GetFloat(name string, defaultValue float64) float64 {
	if v, ok := ctx.GlobalFlags[name]; ok && v != nil {
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return defaultValue
}

// GetDuration retrieves a time.Duration flag value from GlobalFlags, returning defaultValue if not found or nil
func (ctx *Context) GetDuration(name string, defaultValue time.Duration) time.Duration {
	if v, ok := ctx.GlobalFlags[name]; ok && v != nil {
		if d, ok := v.(time.Duration); ok {
			return d
		}
	}
	return defaultValue
}

// RequireString retrieves a string flag value from GlobalFlags, returning an error if not found
func (ctx *Context) RequireString(name string) (string, error) {
	v, ok := ctx.GlobalFlags[name]
	if !ok || v == nil {
		return "", fmt.Errorf("required flag %s not provided", name)
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("flag %s is not a string", name)
	}
	return s, nil
}

// RequireInt retrieves an int flag value from GlobalFlags, returning an error if not found
func (ctx *Context) RequireInt(name string) (int, error) {
	v, ok := ctx.GlobalFlags[name]
	if !ok || v == nil {
		return 0, fmt.Errorf("required flag %s not provided", name)
	}
	i, ok := v.(int)
	if !ok {
		return 0, fmt.Errorf("flag %s is not an int", name)
	}
	return i, nil
}

// RequireBool retrieves a bool flag value from GlobalFlags, returning an error if not found
func (ctx *Context) RequireBool(name string) (bool, error) {
	v, ok := ctx.GlobalFlags[name]
	if !ok || v == nil {
		return false, fmt.Errorf("required flag %s not provided", name)
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("flag %s is not a bool", name)
	}
	return b, nil
}

// RequireFloat retrieves a float64 flag value from GlobalFlags, returning an error if not found
func (ctx *Context) RequireFloat(name string) (float64, error) {
	v, ok := ctx.GlobalFlags[name]
	if !ok || v == nil {
		return 0, fmt.Errorf("required flag %s not provided", name)
	}
	f, ok := v.(float64)
	if !ok {
		return 0, fmt.Errorf("flag %s is not a float64", name)
	}
	return f, nil
}

// RequireDuration retrieves a time.Duration flag value from GlobalFlags, returning an error if not found
func (ctx *Context) RequireDuration(name string) (time.Duration, error) {
	v, ok := ctx.GlobalFlags[name]
	if !ok || v == nil {
		return 0, fmt.Errorf("required flag %s not provided", name)
	}
	d, ok := v.(time.Duration)
	if !ok {
		return 0, fmt.Errorf("flag %s is not a time.Duration", name)
	}
	return d, nil
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

// isPositional returns true if this flag spec is for a positional argument
func (spec *FlagSpec) isPositional() bool {
	if len(spec.Names) == 0 {
		return false
	}
	// Positional if the first name doesn't start with - or +
	name := spec.Names[0]
	return !strings.HasPrefix(name, "-") && !strings.HasPrefix(name, "+")
}

// positionalFlags returns all positional flag specs in definition order
func (cmd *Command) positionalFlags() []*FlagSpec {
	var positionals []*FlagSpec
	for _, spec := range cmd.flags {
		if spec.isPositional() {
			positionals = append(positionals, spec)
		}
	}
	return positionals
}

// namedFlags returns all named flag specs (those starting with - or +)
func (cmd *Command) namedFlags() []*FlagSpec {
	var named []*FlagSpec
	for _, spec := range cmd.flags {
		if !spec.isPositional() {
			named = append(named, spec)
		}
	}
	return named
}

// validatePositionals validates positional argument constraints at build time
func (cmd *Command) validatePositionals() error {
	positionals := cmd.positionalFlags()
	if len(positionals) == 0 {
		return nil
	}

	// Track variadic count and position
	variadicCount := 0
	variadicIndex := -1

	for i, spec := range positionals {
		// Check for mutually exclusive Required and Default
		if spec.Required && spec.Default != nil {
			return fmt.Errorf("positional %q: cannot be both Required and have a Default", spec.Names[0])
		}

		// Track variadic
		if spec.IsVariadic {
			variadicCount++
			variadicIndex = i
		}
	}

	// Only one variadic allowed
	if variadicCount > 1 {
		return fmt.Errorf("only one positional argument can be Variadic")
	}

	// Variadic must be last
	if variadicCount == 1 && variadicIndex != len(positionals)-1 {
		return fmt.Errorf("Variadic positional %q must be last", positionals[variadicIndex].Names[0])
	}

	// Warn about optional before required (but don't fail)
	// We check if any required positional comes after an optional one
	foundOptional := false
	for i, spec := range positionals {
		if spec.IsVariadic {
			break // Variadic is always last, skip it
		}
		if !spec.Required && spec.Default != nil {
			foundOptional = true
		}
		if foundOptional && spec.Required {
			// Print warning to stderr but don't fail
			fmt.Fprintf(os.Stderr, "Warning: required positional %q at position %d comes after optional positionals\n",
				spec.Names[0], i)
		}
	}

	return nil
}
