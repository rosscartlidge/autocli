package completionflags

import "fmt"

// NewCommand creates a new command builder
func NewCommand(name string) *CommandBuilder {
	return &CommandBuilder{
		cmd: &Command{
			name:       name,
			flags:      []*FlagSpec{},
			separators: defaultSeparators(),
			examples:   []Example{},
		},
	}
}

// CommandBuilder provides a fluent API for building commands
type CommandBuilder struct {
	cmd *Command
}

// Version sets the command version
func (cb *CommandBuilder) Version(version string) *CommandBuilder {
	cb.cmd.version = version
	return cb
}

// Description sets the command description
func (cb *CommandBuilder) Description(desc string) *CommandBuilder {
	cb.cmd.description = desc
	return cb
}

// Author sets the command author
func (cb *CommandBuilder) Author(author string) *CommandBuilder {
	cb.cmd.author = author
	return cb
}

// Example adds a usage example
func (cb *CommandBuilder) Example(command, description string) *CommandBuilder {
	cb.cmd.examples = append(cb.cmd.examples, Example{
		Command:     command,
		Description: description,
	})
	return cb
}

// Separators configures clause separators (default: ["+", "-"])
func (cb *CommandBuilder) Separators(seps ...string) *CommandBuilder {
	cb.cmd.separators = seps
	return cb
}

// PrefixHandler sets how to interpret + prefix on flags
func (cb *CommandBuilder) PrefixHandler(h PrefixHandler) *CommandBuilder {
	cb.cmd.prefixHandler = h
	return cb
}

// Flag starts defining a new flag
func (cb *CommandBuilder) Flag(names ...string) *FlagBuilder {
	spec := &FlagSpec{
		Names:         names,
		Scope:         ScopeLocal, // Default to local scope
		ArgCount:      1,          // Default to single argument
		ArgTypes:      []ArgType{ArgString},
		ArgNames:      []string{"VALUE"},
		ArgCompleters: []Completer{NoCompleter{}},
	}

	return &FlagBuilder{
		cb:   cb,
		spec: spec,
	}
}

// Handler sets the clause handler function
func (cb *CommandBuilder) Handler(h ClauseHandlerFunc) *CommandBuilder {
	cb.cmd.handler = h
	return cb
}

// Build finalizes and returns the command
func (cb *CommandBuilder) Build() *Command {
	// Validate command has required fields
	if cb.cmd.name == "" {
		panic("command name is required")
	}

	// Handler is optional if subcommands are defined
	if cb.cmd.handler == nil && len(cb.cmd.subcommands) == 0 {
		panic("command requires either a handler or subcommands")
	}

	// Validate positional arguments
	if err := cb.cmd.validatePositionals(); err != nil {
		panic(fmt.Sprintf("positional validation failed: %v", err))
	}

	return cb.cmd
}

// FlagBuilder provides a fluent API for configuring a flag
type FlagBuilder struct {
	cb   *CommandBuilder
	sb   *SubcommandBuilder
	spec *FlagSpec
}

// ArgBuilder provides a fluent API for configuring a single argument
type ArgBuilder struct {
	fb       *FlagBuilder
	argIndex int
}

// Global sets the flag scope to global (applies to entire command)
func (fb *FlagBuilder) Global() *FlagBuilder {
	fb.spec.Scope = ScopeGlobal
	return fb
}

// Local sets the flag scope to local (applies per-clause)
func (fb *FlagBuilder) Local() *FlagBuilder {
	fb.spec.Scope = ScopeLocal
	return fb
}

// Help sets the flag description
func (fb *FlagBuilder) Help(description string) *FlagBuilder {
	fb.spec.Description = description
	return fb
}

// Args sets the number of arguments this flag takes
func (fb *FlagBuilder) Args(count int) *FlagBuilder {
	fb.spec.ArgCount = count

	// Initialize arrays to match count
	fb.spec.ArgTypes = make([]ArgType, count)
	fb.spec.ArgNames = make([]string, count)
	fb.spec.ArgCompleters = make([]Completer, count)

	// Set defaults
	for i := 0; i < count; i++ {
		fb.spec.ArgTypes[i] = ArgString
		fb.spec.ArgNames[i] = "ARG"
		fb.spec.ArgCompleters[i] = NoCompleter{}
	}

	return fb
}

// ArgName sets the display name for a specific argument
func (fb *FlagBuilder) ArgName(index int, name string) *FlagBuilder {
	if index >= 0 && index < len(fb.spec.ArgNames) {
		fb.spec.ArgNames[index] = name
	}
	return fb
}

// ArgType sets the type for a specific argument
func (fb *FlagBuilder) ArgType(index int, t ArgType) *FlagBuilder {
	if index >= 0 && index < len(fb.spec.ArgTypes) {
		fb.spec.ArgTypes[index] = t
	}
	return fb
}

// ArgCompleter sets the completer for a specific argument
func (fb *FlagBuilder) ArgCompleter(index int, c Completer) *FlagBuilder {
	if index >= 0 && index < len(fb.spec.ArgCompleters) {
		fb.spec.ArgCompleters[index] = c
	}
	return fb
}

// Bool is a shorthand for a boolean flag (0 arguments)
func (fb *FlagBuilder) Bool() *FlagBuilder {
	fb.spec.ArgCount = 0
	fb.spec.ArgTypes = []ArgType{}
	fb.spec.ArgNames = []string{}
	fb.spec.ArgCompleters = []Completer{}
	return fb
}

// String is a shorthand for a single string argument
func (fb *FlagBuilder) String() *FlagBuilder {
	return fb.Args(1).ArgType(0, ArgString).ArgName(0, "VALUE")
}

// Int is a shorthand for a single int argument
func (fb *FlagBuilder) Int() *FlagBuilder {
	return fb.Args(1).ArgType(0, ArgInt).ArgName(0, "NUM")
}

// Float is a shorthand for a single float argument
func (fb *FlagBuilder) Float() *FlagBuilder {
	return fb.Args(1).ArgType(0, ArgFloat).ArgName(0, "NUM")
}

// StringSlice is a shorthand for accumulating multiple string values
func (fb *FlagBuilder) StringSlice() *FlagBuilder {
	fb.spec.IsSlice = true
	return fb.Args(1).ArgType(0, ArgString).ArgName(0, "VALUE")
}

// Duration is a shorthand for a single duration argument
func (fb *FlagBuilder) Duration() *FlagBuilder {
	return fb.Args(1).ArgType(0, ArgDuration).ArgName(0, "DURATION")
}

// Time is a shorthand for a single time argument
func (fb *FlagBuilder) Time() *FlagBuilder {
	return fb.Args(1).ArgType(0, ArgTime).ArgName(0, "TIME")
}

// Required marks the flag as required
func (fb *FlagBuilder) Required() *FlagBuilder {
	fb.spec.Required = true
	return fb
}

// Default sets the default value
func (fb *FlagBuilder) Default(value interface{}) *FlagBuilder {
	fb.spec.Default = value
	return fb
}

// Validate sets a validation function
func (fb *FlagBuilder) Validate(fn ValidatorFunc) *FlagBuilder {
	fb.spec.Validator = fn
	return fb
}

// Completer sets the completer for a single-argument flag
func (fb *FlagBuilder) Completer(c Completer) *FlagBuilder {
	if fb.spec.ArgCount == 1 {
		fb.spec.ArgCompleters[0] = c
	}
	return fb
}

// CompleterFunc sets a function-based completer for a single-argument flag
func (fb *FlagBuilder) CompleterFunc(f CompletionFunc) *FlagBuilder {
	return fb.Completer(f)
}

// FilePattern sets a file completer with pattern for a single-argument flag
func (fb *FlagBuilder) FilePattern(pattern string) *FlagBuilder {
	return fb.Completer(&FileCompleter{Pattern: pattern, Hint: "<FILE>"})
}

// Options sets a static completer with options for a single-argument flag
func (fb *FlagBuilder) Options(opts ...string) *FlagBuilder {
	return fb.Completer(&StaticCompleter{Options: opts})
}

// Hidden hides the flag from help and man pages
func (fb *FlagBuilder) Hidden() *FlagBuilder {
	fb.spec.Hidden = true
	return fb
}

// Accumulate marks the flag to accumulate multiple values (for multi-arg or single-arg flags)
func (fb *FlagBuilder) Accumulate() *FlagBuilder {
	fb.spec.IsSlice = true
	return fb
}

// Variadic marks the positional flag to consume all remaining arguments
func (fb *FlagBuilder) Variadic() *FlagBuilder {
	fb.spec.IsVariadic = true
	fb.spec.IsSlice = true // Variadic implies slice
	return fb
}

// TimeFormats sets time formats to try when parsing ArgTime (can be called multiple times)
func (fb *FlagBuilder) TimeFormats(layouts ...string) *FlagBuilder {
	fb.spec.TimeFormats = append(fb.spec.TimeFormats, layouts...)
	return fb
}

// TimeZone sets the default timezone for ArgTime parsing
func (fb *FlagBuilder) TimeZone(tz string) *FlagBuilder {
	fb.spec.TimeZone = tz
	return fb
}

// TimeZoneFromFlag sets ArgTime to use timezone from another flag (must be Global flag)
func (fb *FlagBuilder) TimeZoneFromFlag(flagName string) *FlagBuilder {
	fb.spec.TimeZoneFromFlag = flagName
	return fb
}

// Arg starts defining a new argument (fluent API alternative to Args() + ArgName/ArgType/ArgCompleter)
func (fb *FlagBuilder) Arg(name string) *ArgBuilder {
	// On first call, clear the default single-arg setup
	if fb.spec.ArgCount == 1 && len(fb.spec.ArgNames) == 1 && fb.spec.ArgNames[0] == "VALUE" {
		fb.spec.ArgCount = 0
		fb.spec.ArgNames = []string{}
		fb.spec.ArgTypes = []ArgType{}
		fb.spec.ArgCompleters = []Completer{}
	}

	// Add a new argument slot
	argIndex := len(fb.spec.ArgNames)

	fb.spec.ArgCount = argIndex + 1
	fb.spec.ArgNames = append(fb.spec.ArgNames, name)
	fb.spec.ArgTypes = append(fb.spec.ArgTypes, ArgString) // Default to string
	fb.spec.ArgCompleters = append(fb.spec.ArgCompleters, NoCompleter{}) // Default to no completer

	return &ArgBuilder{
		fb:       fb,
		argIndex: argIndex,
	}
}

// Type sets the type for this argument
func (ab *ArgBuilder) Type(t ArgType) *ArgBuilder {
	if ab.argIndex >= 0 && ab.argIndex < len(ab.fb.spec.ArgTypes) {
		ab.fb.spec.ArgTypes[ab.argIndex] = t
	}
	return ab
}

// Completer sets the completer for this argument
func (ab *ArgBuilder) Completer(c Completer) *ArgBuilder {
	if ab.argIndex >= 0 && ab.argIndex < len(ab.fb.spec.ArgCompleters) {
		ab.fb.spec.ArgCompleters[ab.argIndex] = c
	}
	return ab
}

// TimeFormats sets time formats for this ArgTime argument (can be called multiple times)
func (ab *ArgBuilder) TimeFormats(layouts ...string) *ArgBuilder {
	ab.fb.spec.TimeFormats = append(ab.fb.spec.TimeFormats, layouts...)
	return ab
}

// TimeZone sets the default timezone for this ArgTime argument
func (ab *ArgBuilder) TimeZone(tz string) *ArgBuilder {
	ab.fb.spec.TimeZone = tz
	return ab
}

// TimeZoneFromFlag sets this ArgTime to use timezone from another flag (must be Global flag)
func (ab *ArgBuilder) TimeZoneFromFlag(flagName string) *ArgBuilder {
	ab.fb.spec.TimeZoneFromFlag = flagName
	return ab
}

// Done finalizes the argument and returns to the flag builder
func (ab *ArgBuilder) Done() *FlagBuilder {
	return ab.fb
}

// Done finalizes the flag and returns to the command or subcommand builder
func (fb *FlagBuilder) Done() *CommandBuilder {
	if fb.sb != nil {
		// Subcommand flag
		fb.sb.subcmd.Flags = append(fb.sb.subcmd.Flags, fb.spec)
		return fb.sb.root // Return root CommandBuilder for fluent chaining
	}
	// Command flag
	fb.cb.cmd.flags = append(fb.cb.cmd.flags, fb.spec)
	return fb.cb
}
