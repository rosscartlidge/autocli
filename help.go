package completionflags

import (
	"fmt"
	"strings"
)

// GenerateHelp generates usage text for -help
func (cmd *Command) GenerateHelp() string {
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
	sb.WriteString(fmt.Sprintf("    %s [OPTIONS]\n\n", cmd.name))

	// Options
	if len(cmd.flags) > 0 {
		sb.WriteString("OPTIONS:\n")
		for _, spec := range cmd.flags {
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
