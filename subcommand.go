package completionflags

import (
	"fmt"
	"strings"
)

// Subcommand represents a subcommand with its own flags, positionals, and handler
type Subcommand struct {
	Name              string
	Description       string
	Author            string
	Examples          []Example
	Flags             []*FlagSpec
	Handler           ClauseHandlerFunc
	Separators        []string
	ClauseDescription string // Custom description for CLAUSES section (optional)
}

// SubcommandBuilder provides a fluent API for building subcommands
type SubcommandBuilder struct {
	name       string
	parent     *CommandBuilder
	subcmd     *Subcommand
}

// Subcommand creates a new subcommand builder
func (cb *CommandBuilder) Subcommand(name string) *SubcommandBuilder {
	// Validate subcommand name
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "+") {
		panic(fmt.Sprintf("subcommand name cannot start with - or +: %s", name))
	}
	if name == "" {
		panic("subcommand name cannot be empty")
	}

	// Check for duplicate subcommand names
	if cb.cmd.subcommands == nil {
		cb.cmd.subcommands = make(map[string]*Subcommand)
	}
	if _, exists := cb.cmd.subcommands[name]; exists {
		panic(fmt.Sprintf("subcommand %q already defined", name))
	}

	sb := &SubcommandBuilder{
		name:   name,
		parent: cb,
		subcmd: &Subcommand{
			Name:       name,
			Flags:      []*FlagSpec{},
			Separators: cb.cmd.separators, // Inherit from parent by default
			Examples:   []Example{},
		},
	}

	return sb
}

// Description sets the subcommand description
func (sb *SubcommandBuilder) Description(desc string) *SubcommandBuilder {
	sb.subcmd.Description = desc
	return sb
}

// Author sets the subcommand author
func (sb *SubcommandBuilder) Author(author string) *SubcommandBuilder {
	sb.subcmd.Author = author
	return sb
}

// Example adds a usage example
func (sb *SubcommandBuilder) Example(command, description string) *SubcommandBuilder {
	sb.subcmd.Examples = append(sb.subcmd.Examples, Example{
		Command:     command,
		Description: description,
	})
	return sb
}

// ClauseDescription sets a custom description for the CLAUSES section
func (sb *SubcommandBuilder) ClauseDescription(desc string) *SubcommandBuilder {
	sb.subcmd.ClauseDescription = desc
	return sb
}

// Separators configures clause separators (overrides parent)
func (sb *SubcommandBuilder) Separators(seps ...string) *SubcommandBuilder {
	sb.subcmd.Separators = seps
	return sb
}

// Flag starts defining a new flag for this subcommand
func (sb *SubcommandBuilder) Flag(names ...string) *SubcommandFlagBuilder {
	// Check for conflicts with root global flags
	for _, name := range names {
		if sb.parent.hasRootGlobalFlag(name) {
			panic(fmt.Sprintf("subcommand %q flag %s conflicts with root global flag", sb.name, name))
		}
	}

	spec := &FlagSpec{
		Names:         names,
		Scope:         ScopeLocal, // Default to local scope
		ArgCount:      1,
		ArgTypes:      []ArgType{ArgString},
		ArgNames:      []string{"VALUE"},
		ArgCompleters: []Completer{NoCompleter{}},
	}

	return &SubcommandFlagBuilder{
		sb:   sb,
		spec: spec,
	}
}

// Handler sets the subcommand handler function
func (sb *SubcommandBuilder) Handler(h ClauseHandlerFunc) *SubcommandBuilder {
	sb.subcmd.Handler = h
	return sb
}

// Done finalizes the subcommand and returns to the command builder
func (sb *SubcommandBuilder) Done() *CommandBuilder {
	// Validate subcommand
	if sb.subcmd.Handler == nil {
		panic(fmt.Sprintf("subcommand %q requires a handler", sb.name))
	}

	// Validate positional arguments
	if err := validateSubcommandPositionals(sb.subcmd); err != nil {
		panic(fmt.Sprintf("subcommand %q positional validation failed: %v", sb.name, err))
	}

	// Add to parent command
	sb.parent.cmd.subcommands[sb.name] = sb.subcmd

	return sb.parent
}

// hasRootGlobalFlag checks if a flag name is defined as a global flag in the root command
func (cb *CommandBuilder) hasRootGlobalFlag(name string) bool {
	for _, spec := range cb.cmd.flags {
		if spec.Scope == ScopeGlobal {
			for _, n := range spec.Names {
				if n == name {
					return true
				}
			}
		}
	}
	return false
}

// validateSubcommandPositionals validates positional argument constraints for a subcommand
func validateSubcommandPositionals(subcmd *Subcommand) error {
	var positionals []*FlagSpec
	for _, spec := range subcmd.Flags {
		if spec.isPositional() {
			positionals = append(positionals, spec)
		}
	}

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

	return nil
}

// rootGlobalFlags returns all global flags from the root command
func (cmd *Command) rootGlobalFlags() []*FlagSpec {
	var globals []*FlagSpec
	for _, spec := range cmd.flags {
		if spec.Scope == ScopeGlobal {
			globals = append(globals, spec)
		}
	}
	return globals
}

// hasSubcommand checks if a subcommand name is registered
func (cmd *Command) hasSubcommand(name string) bool {
	if cmd.subcommands == nil {
		return false
	}
	_, exists := cmd.subcommands[name]
	return exists
}

// getSubcommand retrieves a subcommand by name
func (cmd *Command) getSubcommand(name string) *Subcommand {
	if cmd.subcommands == nil {
		return nil
	}
	return cmd.subcommands[name]
}

// GenerateHelp generates help text for a subcommand
func (subcmd *Subcommand) GenerateHelp(parentName string) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("%s %s", parentName, subcmd.Name))
	if subcmd.Description != "" {
		sb.WriteString(fmt.Sprintf(" - %s", subcmd.Description))
	}
	sb.WriteString("\n\n")

	// Usage
	sb.WriteString("USAGE:\n")
	usageLine := fmt.Sprintf("    %s %s [OPTIONS]", parentName, subcmd.Name)

	// Add positional arguments to usage line
	var positionals []*FlagSpec
	for _, spec := range subcmd.Flags {
		if spec.isPositional() {
			positionals = append(positionals, spec)
		}
	}

	for _, spec := range positionals {
		usageLine += " "
		argName := spec.Names[0]
		if spec.IsVariadic {
			if spec.Required {
				usageLine += argName + "..."
			} else {
				usageLine += "[" + argName + "...]"
			}
		} else {
			if spec.Required {
				usageLine += argName
			} else {
				usageLine += "[" + argName + "]"
			}
		}
	}

	// Add clause separator info to usage if applicable
	if len(subcmd.Separators) > 0 {
		usageLine += " [" + strings.Join(subcmd.Separators, "|") + " ...]"
	}

	sb.WriteString(usageLine + "\n\n")

	// Description
	if subcmd.Description != "" {
		sb.WriteString("DESCRIPTION:\n")
		sb.WriteString(fmt.Sprintf("    %s\n\n", subcmd.Description))
	}

	// Arguments (positional)
	if len(positionals) > 0 {
		sb.WriteString("ARGUMENTS:\n")
		for _, spec := range positionals {
			if spec.Hidden {
				continue
			}
			sb.WriteString(formatPositionalForSubcommand(spec))
			sb.WriteString("\n")
		}
	}

	// Separate global and local flags
	var globalFlags []*FlagSpec
	var localFlags []*FlagSpec
	for _, spec := range subcmd.Flags {
		if spec.isPositional() {
			continue
		}
		if spec.Scope == ScopeGlobal {
			globalFlags = append(globalFlags, spec)
		} else {
			localFlags = append(localFlags, spec)
		}
	}

	// Global options (for this subcommand)
	if len(globalFlags) > 0 {
		sb.WriteString("OPTIONS:\n")
		for _, spec := range globalFlags {
			if spec.Hidden {
				continue
			}
			sb.WriteString(formatFlagForSubcommand(spec))
			sb.WriteString("\n")
		}
	}

	// Local options (per-clause)
	if len(localFlags) > 0 {
		sb.WriteString("PER-CLAUSE OPTIONS:\n")
		for _, spec := range localFlags {
			if spec.Hidden {
				continue
			}
			sb.WriteString(formatFlagForSubcommand(spec))
			sb.WriteString("\n")
		}
	}

	// Clauses explanation (only show if command has per-clause flags)
	if len(subcmd.Separators) > 0 && len(localFlags) > 0 {
		sb.WriteString("CLAUSES:\n")
		if subcmd.ClauseDescription != "" {
			// Use custom clause description if provided
			sb.WriteString("    ")
			sb.WriteString(subcmd.ClauseDescription)
			sb.WriteString("\n\n")
		} else {
			// Use default clause description
			sb.WriteString("    Arguments can be grouped into clauses using separators.\n")
			sb.WriteString(fmt.Sprintf("    Separators: %s\n", strings.Join(subcmd.Separators, ", ")))
			sb.WriteString("    Each clause is processed independently (typically with OR logic).\n\n")
		}
	}

	// Examples
	if len(subcmd.Examples) > 0 {
		sb.WriteString("EXAMPLES:\n")
		for _, example := range subcmd.Examples {
			sb.WriteString(fmt.Sprintf("    %s\n", example.Command))
			if example.Description != "" {
				sb.WriteString(fmt.Sprintf("        %s\n", example.Description))
			}
			sb.WriteString("\n")
		}
	}

	// Footer
	sb.WriteString(fmt.Sprintf("Use '%s -help' to see all available commands.\n", parentName))
	sb.WriteString(fmt.Sprintf("Use '%s %s -man' to view the full manual page for this command.\n", parentName, subcmd.Name))

	return sb.String()
}

// formatPositionalForSubcommand formats a positional argument for subcommand help
func formatPositionalForSubcommand(spec *FlagSpec) string {
	var sb strings.Builder

	// Positional name
	sb.WriteString("    ")
	argName := spec.Names[0]
	if spec.IsVariadic {
		sb.WriteString(argName + "...")
	} else {
		sb.WriteString(argName)
	}
	sb.WriteString("\n")

	// Description
	if spec.Description != "" {
		sb.WriteString(fmt.Sprintf("        %s\n", spec.Description))
	}

	// Type information
	if len(spec.ArgTypes) > 0 && spec.ArgTypes[0] != ArgString {
		var typeName string
		switch spec.ArgTypes[0] {
		case ArgInt:
			typeName = "integer"
		case ArgFloat:
			typeName = "float"
		case ArgBool:
			typeName = "boolean"
		case ArgDuration:
			typeName = "duration"
		case ArgTime:
			typeName = "time"
		default:
			typeName = "string"
		}
		sb.WriteString(fmt.Sprintf("        Type: %s\n", typeName))
	}

	// Default value
	if spec.Default != nil {
		sb.WriteString(fmt.Sprintf("        Default: %v\n", spec.Default))
	}

	// Required
	if spec.Required {
		sb.WriteString("        Required\n")
	}

	return sb.String()
}

// formatFlagForSubcommand formats a flag for subcommand help
func formatFlagForSubcommand(spec *FlagSpec) string {
	var sb strings.Builder

	// Flag names
	sb.WriteString("    ")
	sb.WriteString(strings.Join(spec.Names, ", "))

	// Arguments
	if spec.ArgCount > 0 {
		for i := 0; i < spec.ArgCount; i++ {
			sb.WriteString(" ")
			if i < len(spec.ArgNames) {
				sb.WriteString(spec.ArgNames[i])
			} else {
				sb.WriteString(fmt.Sprintf("ARG%d", i))
			}
		}
	}

	sb.WriteString("\n")

	// Description
	if spec.Description != "" {
		sb.WriteString(fmt.Sprintf("        %s\n", spec.Description))
	}

	// Default value
	if spec.Default != nil {
		sb.WriteString(fmt.Sprintf("        Default: %v\n", spec.Default))
	}

	// Required
	if spec.Required {
		sb.WriteString("        Required: yes\n")
	}

	// Multi-value
	if spec.IsSlice {
		sb.WriteString("        Can be specified multiple times\n")
	}

	return sb.String()
}

// GenerateManPage generates a man page for a subcommand
func (subcmd *Subcommand) GenerateManPage(parentName string) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf(".TH %s-%s 1\n",
		strings.ToUpper(parentName),
		strings.ToUpper(subcmd.Name)))

	// NAME section
	sb.WriteString(".SH NAME\n")
	if subcmd.Description != "" {
		sb.WriteString(fmt.Sprintf("%s %s \\- %s\n", parentName, subcmd.Name, subcmd.Description))
	} else {
		sb.WriteString(fmt.Sprintf("%s %s\n", parentName, subcmd.Name))
	}

	// SYNOPSIS section
	sb.WriteString(".SH SYNOPSIS\n")
	sb.WriteString(fmt.Sprintf(".B %s %s\n", parentName, subcmd.Name))
	sb.WriteString("[\\fIOPTIONS\\fR]")

	// Add positional arguments
	for _, spec := range subcmd.Flags {
		if !spec.isPositional() {
			continue
		}
		argName := spec.Names[0]
		if spec.IsVariadic {
			if spec.Required {
				sb.WriteString(fmt.Sprintf("\n.I %s...", argName))
			} else {
				sb.WriteString(fmt.Sprintf("\n[\\fI%s...\\fR]", argName))
			}
		} else {
			if spec.Required {
				sb.WriteString(fmt.Sprintf("\n.I %s", argName))
			} else {
				sb.WriteString(fmt.Sprintf("\n[\\fI%s\\fR]", argName))
			}
		}
	}

	if len(subcmd.Separators) > 0 {
		sb.WriteString(fmt.Sprintf("\n[\\fI%s ...\\fR]", strings.Join(subcmd.Separators, "|")))
	}
	sb.WriteString("\n")

	// DESCRIPTION section
	if subcmd.Description != "" {
		sb.WriteString(".SH DESCRIPTION\n")
		sb.WriteString(subcmd.Description)
		sb.WriteString("\n")
	}

	// OPTIONS section
	sb.WriteString(".SH OPTIONS\n")
	for _, spec := range subcmd.Flags {
		if spec.isPositional() || spec.Hidden {
			continue
		}

		// Flag names
		sb.WriteString(".TP\n")
		sb.WriteString(".B ")
		sb.WriteString(strings.Join(spec.Names, ", "))

		// Arguments
		if spec.ArgCount > 0 {
			for i := 0; i < spec.ArgCount; i++ {
				sb.WriteString(" ")
				if i < len(spec.ArgNames) {
					sb.WriteString(spec.ArgNames[i])
				} else {
					sb.WriteString(fmt.Sprintf("ARG%d", i))
				}
			}
		}
		sb.WriteString("\n")

		// Description
		if spec.Description != "" {
			sb.WriteString(spec.Description)
			sb.WriteString("\n")
		}

		// Scope
		if spec.Scope == ScopeLocal {
			sb.WriteString("(per-clause)\n")
		}
	}

	// EXAMPLES section
	if len(subcmd.Examples) > 0 {
		sb.WriteString(".SH EXAMPLES\n")
		for _, example := range subcmd.Examples {
			sb.WriteString(".TP\n")
			sb.WriteString(example.Command)
			sb.WriteString("\n")
			if example.Description != "" {
				sb.WriteString(example.Description)
				sb.WriteString("\n")
			}
		}
	}

	// SEE ALSO section
	sb.WriteString(".SH SEE ALSO\n")
	sb.WriteString(fmt.Sprintf(".BR %s (1)\n", parentName))

	return sb.String()
}

// SubcommandFlagBuilder provides a fluent API for configuring flags within subcommands
// It wraps FlagBuilder but returns to SubcommandBuilder instead of CommandBuilder
type SubcommandFlagBuilder struct {
	sb   *SubcommandBuilder
	spec *FlagSpec
}

// SubcommandArgBuilder provides a fluent API for configuring arguments within subcommand flags
type SubcommandArgBuilder struct {
	sfb      *SubcommandFlagBuilder
	argIndex int
}

// Global sets the flag scope to global
func (sfb *SubcommandFlagBuilder) Global() *SubcommandFlagBuilder {
	sfb.spec.Scope = ScopeGlobal
	return sfb
}

// Local sets the flag scope to local
func (sfb *SubcommandFlagBuilder) Local() *SubcommandFlagBuilder {
	sfb.spec.Scope = ScopeLocal
	return sfb
}

// Help sets the flag description
func (sfb *SubcommandFlagBuilder) Help(description string) *SubcommandFlagBuilder {
	sfb.spec.Description = description
	return sfb
}

// Args sets the number of arguments
func (sfb *SubcommandFlagBuilder) Args(count int) *SubcommandFlagBuilder {
	sfb.spec.ArgCount = count
	sfb.spec.ArgTypes = make([]ArgType, count)
	sfb.spec.ArgNames = make([]string, count)
	sfb.spec.ArgCompleters = make([]Completer, count)
	for i := 0; i < count; i++ {
		sfb.spec.ArgTypes[i] = ArgString
		sfb.spec.ArgNames[i] = "ARG"
		sfb.spec.ArgCompleters[i] = NoCompleter{}
	}
	return sfb
}

// Bool is a shorthand for boolean flags
func (sfb *SubcommandFlagBuilder) Bool() *SubcommandFlagBuilder {
	sfb.spec.ArgCount = 0
	sfb.spec.ArgTypes = []ArgType{}
	sfb.spec.ArgNames = []string{}
	sfb.spec.ArgCompleters = []Completer{}
	return sfb
}

// String is a shorthand for single string argument
func (sfb *SubcommandFlagBuilder) String() *SubcommandFlagBuilder {
	return sfb.Args(1).ArgType(0, ArgString).ArgName(0, "VALUE")
}

// Int is a shorthand for single int argument
func (sfb *SubcommandFlagBuilder) Int() *SubcommandFlagBuilder {
	return sfb.Args(1).ArgType(0, ArgInt).ArgName(0, "NUM")
}

// Float is a shorthand for single float argument
func (sfb *SubcommandFlagBuilder) Float() *SubcommandFlagBuilder {
	return sfb.Args(1).ArgType(0, ArgFloat).ArgName(0, "NUM")
}

// StringSlice is a shorthand for accumulating string values
func (sfb *SubcommandFlagBuilder) StringSlice() *SubcommandFlagBuilder {
	sfb.spec.IsSlice = true
	return sfb.Args(1).ArgType(0, ArgString).ArgName(0, "VALUE")
}

// Duration is a shorthand for duration argument
func (sfb *SubcommandFlagBuilder) Duration() *SubcommandFlagBuilder {
	return sfb.Args(1).ArgType(0, ArgDuration).ArgName(0, "DURATION")
}

// Time is a shorthand for time argument
func (sfb *SubcommandFlagBuilder) Time() *SubcommandFlagBuilder {
	return sfb.Args(1).ArgType(0, ArgTime).ArgName(0, "TIME")
}

// ArgName sets the name for a specific argument
func (sfb *SubcommandFlagBuilder) ArgName(index int, name string) *SubcommandFlagBuilder {
	if index >= 0 && index < len(sfb.spec.ArgNames) {
		sfb.spec.ArgNames[index] = name
	}
	return sfb
}

// ArgType sets the type for a specific argument
func (sfb *SubcommandFlagBuilder) ArgType(index int, t ArgType) *SubcommandFlagBuilder {
	if index >= 0 && index < len(sfb.spec.ArgTypes) {
		sfb.spec.ArgTypes[index] = t
	}
	return sfb
}

// ArgCompleter sets the completer for a specific argument
func (sfb *SubcommandFlagBuilder) ArgCompleter(index int, c Completer) *SubcommandFlagBuilder {
	if index >= 0 && index < len(sfb.spec.ArgCompleters) {
		sfb.spec.ArgCompleters[index] = c
	}
	return sfb
}

// Required marks the flag as required
func (sfb *SubcommandFlagBuilder) Required() *SubcommandFlagBuilder {
	sfb.spec.Required = true
	return sfb
}

// Default sets the default value
func (sfb *SubcommandFlagBuilder) Default(value interface{}) *SubcommandFlagBuilder {
	sfb.spec.Default = value
	return sfb
}

// Validate sets a validation function
func (sfb *SubcommandFlagBuilder) Validate(fn ValidatorFunc) *SubcommandFlagBuilder {
	sfb.spec.Validator = fn
	return sfb
}

// Completer sets the completer for single-argument flags
func (sfb *SubcommandFlagBuilder) Completer(c Completer) *SubcommandFlagBuilder {
	if sfb.spec.ArgCount == 1 {
		sfb.spec.ArgCompleters[0] = c
	}
	return sfb
}

// CompleterFunc sets a function-based completer
func (sfb *SubcommandFlagBuilder) CompleterFunc(f CompletionFunc) *SubcommandFlagBuilder {
	return sfb.Completer(f)
}

// FilePattern sets a file completer with pattern
func (sfb *SubcommandFlagBuilder) FilePattern(pattern string) *SubcommandFlagBuilder {
	return sfb.Completer(&FileCompleter{Pattern: pattern, Hint: "<FILE>"})
}

// Options sets a static completer
func (sfb *SubcommandFlagBuilder) Options(opts ...string) *SubcommandFlagBuilder {
	return sfb.Completer(&StaticCompleter{Options: opts})
}

// Hidden hides the flag from help
func (sfb *SubcommandFlagBuilder) Hidden() *SubcommandFlagBuilder {
	sfb.spec.Hidden = true
	return sfb
}

// Accumulate marks the flag to accumulate multiple values
func (sfb *SubcommandFlagBuilder) Accumulate() *SubcommandFlagBuilder {
	sfb.spec.IsSlice = true
	return sfb
}

// Variadic marks positional to consume all remaining arguments
func (sfb *SubcommandFlagBuilder) Variadic() *SubcommandFlagBuilder {
	sfb.spec.IsVariadic = true
	sfb.spec.IsSlice = true
	return sfb
}

// TimeFormats sets time formats
func (sfb *SubcommandFlagBuilder) TimeFormats(layouts ...string) *SubcommandFlagBuilder {
	sfb.spec.TimeFormats = append(sfb.spec.TimeFormats, layouts...)
	return sfb
}

// TimeZone sets default timezone
func (sfb *SubcommandFlagBuilder) TimeZone(tz string) *SubcommandFlagBuilder {
	sfb.spec.TimeZone = tz
	return sfb
}

// TimeZoneFromFlag sets timezone from another flag
func (sfb *SubcommandFlagBuilder) TimeZoneFromFlag(flagName string) *SubcommandFlagBuilder {
	sfb.spec.TimeZoneFromFlag = flagName
	return sfb
}

// Arg starts defining a new argument
func (sfb *SubcommandFlagBuilder) Arg(name string) *SubcommandArgBuilder {
	// On first call, clear default setup
	if sfb.spec.ArgCount == 1 && len(sfb.spec.ArgNames) == 1 && sfb.spec.ArgNames[0] == "VALUE" {
		sfb.spec.ArgCount = 0
		sfb.spec.ArgNames = []string{}
		sfb.spec.ArgTypes = []ArgType{}
		sfb.spec.ArgCompleters = []Completer{}
	}

	argIndex := len(sfb.spec.ArgNames)
	sfb.spec.ArgCount = argIndex + 1
	sfb.spec.ArgNames = append(sfb.spec.ArgNames, name)
	sfb.spec.ArgTypes = append(sfb.spec.ArgTypes, ArgString)
	sfb.spec.ArgCompleters = append(sfb.spec.ArgCompleters, NoCompleter{})

	return &SubcommandArgBuilder{
		sfb:      sfb,
		argIndex: argIndex,
	}
}

// Done finalizes the flag and returns to subcommand builder
func (sfb *SubcommandFlagBuilder) Done() *SubcommandBuilder {
	sfb.sb.subcmd.Flags = append(sfb.sb.subcmd.Flags, sfb.spec)
	return sfb.sb
}

// Type sets the type for this argument
func (sab *SubcommandArgBuilder) Type(t ArgType) *SubcommandArgBuilder {
	if sab.argIndex >= 0 && sab.argIndex < len(sab.sfb.spec.ArgTypes) {
		sab.sfb.spec.ArgTypes[sab.argIndex] = t
	}
	return sab
}

// Completer sets the completer for this argument
func (sab *SubcommandArgBuilder) Completer(c Completer) *SubcommandArgBuilder {
	if sab.argIndex >= 0 && sab.argIndex < len(sab.sfb.spec.ArgCompleters) {
		sab.sfb.spec.ArgCompleters[sab.argIndex] = c
	}
	return sab
}

// TimeFormats sets time formats for this argument
func (sab *SubcommandArgBuilder) TimeFormats(layouts ...string) *SubcommandArgBuilder {
	sab.sfb.spec.TimeFormats = append(sab.sfb.spec.TimeFormats, layouts...)
	return sab
}

// TimeZone sets the default timezone for this argument
func (sab *SubcommandArgBuilder) TimeZone(tz string) *SubcommandArgBuilder {
	sab.sfb.spec.TimeZone = tz
	return sab
}

// TimeZoneFromFlag sets timezone from another flag
func (sab *SubcommandArgBuilder) TimeZoneFromFlag(flagName string) *SubcommandArgBuilder {
	sab.sfb.spec.TimeZoneFromFlag = flagName
	return sab
}

// Done finalizes the argument and returns to flag builder
func (sab *SubcommandArgBuilder) Done() *SubcommandFlagBuilder {
	return sab.sfb
}
