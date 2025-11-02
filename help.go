package completionflags

import (
	"fmt"
	"strings"
)

// GenerateHelp generates usage text for -help
func (cmd *Command) GenerateHelp() string {
	// If we have subcommands, generate subcommand-aware help
	if len(cmd.subcommands) > 0 {
		return cmd.generateHelpWithSubcommands()
	}

	// Standard help generation (no subcommands)
	var sb strings.Builder

	// Header
	if cmd.version != "" {
		sb.WriteString(fmt.Sprintf("%s v%s", cmd.name, cmd.version))
	} else {
		sb.WriteString(cmd.name)
	}

	if cmd.description != "" {
		sb.WriteString(fmt.Sprintf(" - %s", cmd.description))
	}
	sb.WriteString("\n\n")

	// Usage
	sb.WriteString("USAGE:\n")
	usageLine := fmt.Sprintf("    %s [OPTIONS]", cmd.name)

	// Add positional arguments to usage line
	positionals := cmd.positionalFlags()
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
	sb.WriteString(usageLine + "\n\n")

	// Arguments (positional)
	if len(positionals) > 0 {
		sb.WriteString("ARGUMENTS:\n")
		for _, spec := range positionals {
			if spec.Hidden {
				continue
			}
			sb.WriteString(cmd.formatPositional(spec))
			sb.WriteString("\n")
		}
	}

	// Options (named flags)
	namedFlags := cmd.namedFlags()
	if len(namedFlags) > 0 {
		sb.WriteString("OPTIONS:\n")
		for _, spec := range namedFlags {
			if spec.Hidden {
				continue
			}
			sb.WriteString(cmd.formatFlag(spec))
			sb.WriteString("\n")
		}
	}

	// Clauses explanation
	if len(cmd.separators) > 0 {
		sb.WriteString("CLAUSES:\n")
		sb.WriteString("    Arguments can be grouped into clauses using separators.\n")
		sb.WriteString(fmt.Sprintf("    Separators: %s\n", strings.Join(cmd.separators, ", ")))
		sb.WriteString("    Each clause is processed according to the command's logic.\n\n")
	}

	// Examples
	if len(cmd.examples) > 0 {
		sb.WriteString("EXAMPLES:\n")
		for _, example := range cmd.examples {
			sb.WriteString(fmt.Sprintf("    %s\n", example.Command))
			if example.Description != "" {
				sb.WriteString(fmt.Sprintf("        %s\n", example.Description))
			}
			sb.WriteString("\n")
		}
	}

	// Footer
	sb.WriteString(fmt.Sprintf("Use '%s -man' to view the full manual page.\n", cmd.name))

	return sb.String()
}

// generateHelpWithSubcommands generates help for a command with subcommands
func (cmd *Command) generateHelpWithSubcommands() string {
	var sb strings.Builder

	// Header
	if cmd.version != "" {
		sb.WriteString(fmt.Sprintf("%s v%s", cmd.name, cmd.version))
	} else {
		sb.WriteString(cmd.name)
	}

	if cmd.description != "" {
		sb.WriteString(fmt.Sprintf(" - %s", cmd.description))
	}
	sb.WriteString("\n\n")

	// Usage
	sb.WriteString("USAGE:\n")
	sb.WriteString(fmt.Sprintf("    %s [GLOBAL OPTIONS] <COMMAND> [COMMAND OPTIONS]\n\n", cmd.name))

	// Subcommands
	sb.WriteString("COMMANDS:\n")
	for name, subcmd := range cmd.subcommands {
		sb.WriteString(fmt.Sprintf("    %-15s %s\n", name, subcmd.Description))
	}
	sb.WriteString("\n")

	// Global options
	globalFlags := cmd.rootGlobalFlags()
	if len(globalFlags) > 0 {
		sb.WriteString("GLOBAL OPTIONS:\n")
		for _, spec := range globalFlags {
			if spec.Hidden {
				continue
			}
			sb.WriteString(cmd.formatFlag(spec))
			sb.WriteString("\n")
		}
	}

	// Examples
	if len(cmd.examples) > 0 {
		sb.WriteString("EXAMPLES:\n")
		for _, example := range cmd.examples {
			sb.WriteString(fmt.Sprintf("    %s\n", example.Command))
			if example.Description != "" {
				sb.WriteString(fmt.Sprintf("        %s\n", example.Description))
			}
			sb.WriteString("\n")
		}
	}

	// Footer
	sb.WriteString(fmt.Sprintf("Use '%s <command> -help' for detailed help on a specific command.\n", cmd.name))
	sb.WriteString(fmt.Sprintf("Use '%s -man' to view the full manual page.\n", cmd.name))

	return sb.String()
}

// formatPositional formats a single positional argument for display in help text
func (cmd *Command) formatPositional(spec *FlagSpec) string {
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

	// Scope
	if spec.Scope == ScopeGlobal {
		sb.WriteString("        Scope: global\n")
	} else {
		sb.WriteString("        Scope: per-clause\n")
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

// formatFlag formats a single flag for display in help text
func (cmd *Command) formatFlag(spec *FlagSpec) string {
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

	// Scope
	if spec.Scope == ScopeGlobal {
		sb.WriteString("        Scope: global\n")
	} else {
		sb.WriteString("        Scope: per-clause\n")
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
