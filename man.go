package completionflags

import (
	"fmt"
	"strings"
	"time"
)

// GenerateManPage generates a groff man page
func (cmd *Command) GenerateManPage() string {
	var sb strings.Builder

	// Header
	version := cmd.version
	if version == "" {
		version = "1.0"
	}

	date := time.Now().Format("2006-01-02")
	sb.WriteString(fmt.Sprintf(".TH %s 1 \"%s\" \"%s v%s\"\n",
		escapeGroff(strings.ToUpper(cmd.name)),
		date,
		escapeGroff(cmd.name),
		version))

	// NAME section
	sb.WriteString(".SH NAME\n")
	if cmd.description != "" {
		sb.WriteString(fmt.Sprintf("%s \\- %s\n", escapeGroff(cmd.name), escapeGroff(cmd.description)))
	} else {
		sb.WriteString(fmt.Sprintf("%s\n", escapeGroff(cmd.name)))
	}

	// SYNOPSIS section
	sb.WriteString(".SH SYNOPSIS\n")
	sb.WriteString(fmt.Sprintf(".B %s\n", escapeGroff(cmd.name)))
	sb.WriteString("[\\fIOPTIONS\\fR]")

	// Add positional arguments to synopsis
	positionals := cmd.positionalFlags()
	for _, spec := range positionals {
		argName := spec.Names[0]
		if spec.IsVariadic {
			if spec.Required {
				sb.WriteString(fmt.Sprintf("\n.I %s...", escapeGroff(argName)))
			} else {
				sb.WriteString(fmt.Sprintf("\n[.I %s...]", escapeGroff(argName)))
			}
		} else {
			if spec.Required {
				sb.WriteString(fmt.Sprintf("\n.I %s", escapeGroff(argName)))
			} else {
				sb.WriteString(fmt.Sprintf("\n[.I %s]", escapeGroff(argName)))
			}
		}
	}
	sb.WriteString("\n")

	// DESCRIPTION section
	if cmd.description != "" {
		sb.WriteString(".SH DESCRIPTION\n")
		sb.WriteString(".B ")
		sb.WriteString(escapeGroff(cmd.name))
		sb.WriteString("\n")
		sb.WriteString(escapeGroff(cmd.description))
		sb.WriteString("\n")
	}

	// ARGUMENTS section (positional)
	if len(positionals) > 0 {
		sb.WriteString(".SH ARGUMENTS\n")
		for _, spec := range positionals {
			if spec.Hidden {
				continue
			}
			sb.WriteString(cmd.formatManPositional(spec))
		}
	}

	// OPTIONS section (named flags)
	namedFlags := cmd.namedFlags()
	if len(namedFlags) > 0 {
		sb.WriteString(".SH OPTIONS\n")
		for _, spec := range namedFlags {
			if spec.Hidden {
				continue
			}
			sb.WriteString(cmd.formatManFlag(spec))
		}
	}

	// CLAUSES section
	if len(cmd.separators) > 0 {
		sb.WriteString(".SH CLAUSES\n")
		sb.WriteString("Arguments can be grouped into clauses using separators.\n")
		sb.WriteString("The following separators are recognized:\n")
		for _, sep := range cmd.separators {
			sb.WriteString(fmt.Sprintf(".B %s\n", escapeGroff(sep)))
		}
		sb.WriteString(".PP\n")
		sb.WriteString("Each clause is processed according to the command's logic.\n")
		sb.WriteString("Flags with global scope apply to all clauses,\n")
		sb.WriteString("while flags with per-clause scope apply only within their clause.\n")
	}

	// EXAMPLES section
	if len(cmd.examples) > 0 {
		sb.WriteString(".SH EXAMPLES\n")
		for _, example := range cmd.examples {
			sb.WriteString(".TP\n")
			sb.WriteString(fmt.Sprintf("%s\n", escapeGroff(example.Command)))
			if example.Description != "" {
				sb.WriteString(fmt.Sprintf("%s\n", escapeGroff(example.Description)))
			}
		}
	}

	// AUTHOR section
	if cmd.author != "" {
		sb.WriteString(".SH AUTHOR\n")
		sb.WriteString(fmt.Sprintf("%s\n", escapeGroff(cmd.author)))
	}

	return sb.String()
}

// formatManPositional formats a single positional argument for man page
func (cmd *Command) formatManPositional(spec *FlagSpec) string {
	var sb strings.Builder

	sb.WriteString(".TP\n")

	// Positional name
	argName := escapeGroff(spec.Names[0])
	if spec.IsVariadic {
		sb.WriteString(fmt.Sprintf(".I %s...\n", argName))
	} else {
		sb.WriteString(fmt.Sprintf(".I %s\n", argName))
	}

	// Description
	if spec.Description != "" {
		sb.WriteString(escapeGroff(spec.Description))
		sb.WriteString("\n")
	}

	// Additional details
	var details []string

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
		details = append(details, fmt.Sprintf("Type: %s", typeName))
	}

	if spec.Scope == ScopeGlobal {
		details = append(details, "Scope: global")
	} else {
		details = append(details, "Scope: per-clause")
	}

	if spec.Default != nil {
		details = append(details, fmt.Sprintf("Default: %v", spec.Default))
	}

	if spec.Required {
		details = append(details, "Required")
	}

	if len(details) > 0 {
		sb.WriteString(".RS\n")
		sb.WriteString(escapeGroff(strings.Join(details, ". ")))
		sb.WriteString(".\n")
		sb.WriteString(".RE\n")
	}

	return sb.String()
}

// formatManFlag formats a single flag for man page
func (cmd *Command) formatManFlag(spec *FlagSpec) string {
	var sb strings.Builder

	sb.WriteString(".TP\n")

	// Join all flag names with |
	escapedNames := make([]string, len(spec.Names))
	for i, name := range spec.Names {
		escapedNames[i] = escapeGroff(name)
	}
	flagNames := strings.Join(escapedNames, "|")

	if spec.ArgCount > 0 {
		// Flags with arguments: .BI \-flag1|\-flag2 " ARG1 ARG2"
		args := make([]string, spec.ArgCount)
		for j := 0; j < spec.ArgCount; j++ {
			argName := "ARG"
			if j < len(spec.ArgNames) {
				argName = spec.ArgNames[j]
			}
			args[j] = argName
		}
		sb.WriteString(fmt.Sprintf(".BI %s \" %s\"\n",
			flagNames,
			strings.Join(args, " ")))
	} else {
		// Boolean flags: .B \-flag1|\-flag2
		sb.WriteString(fmt.Sprintf(".B %s\n", flagNames))
	}

	// Description
	if spec.Description != "" {
		sb.WriteString(escapeGroff(spec.Description))
		sb.WriteString("\n")
	}

	// Additional details
	var details []string

	if spec.Scope == ScopeGlobal {
		details = append(details, "Scope: global")
	} else {
		details = append(details, "Scope: per-clause")
	}

	if spec.Default != nil {
		details = append(details, fmt.Sprintf("Default: %v", spec.Default))
	}

	if spec.Required {
		details = append(details, "Required")
	}

	if spec.IsSlice {
		details = append(details, "Can be specified multiple times")
	}

	if len(details) > 0 {
		sb.WriteString(".RS\n")
		sb.WriteString(escapeGroff(strings.Join(details, ". ")))
		sb.WriteString(".\n")
		sb.WriteString(".RE\n")
	}

	return sb.String()
}

// escapeGroff escapes special characters for groff format
func escapeGroff(s string) string {
	// Escape backslashes first
	s = strings.ReplaceAll(s, "\\", "\\\\")

	// Escape hyphens (to prevent them being treated as minus signs)
	s = strings.ReplaceAll(s, "-", "\\-")

	// Escape dots at start of line (would be treated as command)
	if strings.HasPrefix(s, ".") {
		s = "\\&" + s
	}

	return s
}
