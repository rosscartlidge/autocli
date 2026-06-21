package completionflags

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// HelpAt renders contextual help for the command-line state described by
// args + pos — the help-at-cursor analogue of Complete. Where Complete
// answers "what can go here", HelpAt answers "what is the thing under the
// cursor and what does it expect".
//
//   - args is the same slice the bash protocol passes to Complete: the
//     command-line words MINUS the program name. For `myprog sub -fla<caret>`
//     args is ["sub", "-fla"] and pos = 2 (COMP_WORDS index, program at 0).
//   - The returned string is rendered, ready-to-display help text.
//
// Resolution mirrors Complete: walk the subcommand tree to the leaf the
// cursor sits in, then reuse analyzeCompletionContext to locate the flag /
// argument. When the cursor is on (or inside the arguments of) a specific
// flag, HelpAt returns flag-focused help; otherwise it falls back to the
// resolved command's full help. Side-effect free; safe to call concurrently.
func (cmd *Command) HelpAt(args []string, pos int) (string, error) {
	// No subcommands: analyze directly against this command.
	if len(cmd.subcommands) == 0 {
		ctx := cmd.analyzeCompletionContext(args, pos)
		if text, ok := cmd.flagHelpAt(ctx); ok {
			return text, nil
		}
		return cmd.GenerateHelpEmbedded(), nil
	}

	// Resolve the leaf subcommand the cursor is in (same walk Complete uses).
	rootGlobals, remaining, err := cmd.parseRootGlobalFlags(args)
	if err != nil {
		remaining = args
	}
	consumed := len(args) - len(remaining)
	remainingPos := pos - consumed - 1 // -1 for the program word (COMP_WORDS)

	// Cursor still on root globals or the bare subcommand name → root help.
	if remainingPos < 0 || len(remaining) == 0 {
		return cmd.GenerateHelpEmbedded(), nil
	}

	path := []string{}
	currentSubcommands := cmd.subcommands
	var leafSubcmd *Subcommand
	argIndex := 0
	for argIndex < len(remaining) && argIndex < remainingPos {
		subcmd := currentSubcommands[remaining[argIndex]]
		if subcmd == nil {
			break // not a confirmed subcommand at this level
		}
		path = append(path, remaining[argIndex])
		leafSubcmd = subcmd
		argIndex++
		if subcmd.Subcommands != nil {
			currentSubcommands = subcmd.Subcommands
		} else {
			break
		}
	}

	// Cursor on an unresolved / partial subcommand name → root help.
	if leafSubcmd == nil {
		return cmd.GenerateHelpEmbedded(), nil
	}

	// Build the synthetic leaf command (root globals + this subcommand's
	// flags), exactly as completeWithSubcommands does, then analyze.
	tempCmd := &Command{
		name:       leafSubcmd.Name,
		flags:      append(cmd.rootGlobalFlags(), leafSubcmd.Flags...),
		separators: leafSubcmd.Separators,
	}
	subArgs := remaining[argIndex:]
	subPos := remainingPos - argIndex + 1
	ctx := tempCmd.analyzeCompletionContext(subArgs, subPos)
	_ = rootGlobals

	if text, ok := tempCmd.flagHelpAt(ctx); ok {
		return text, nil
	}
	// No specific flag under the cursor → the leaf command's full help.
	return leafSubcmd.GenerateHelp(cmd.name), nil
}

// flagHelpAt returns flag-focused help when the cursor sits on a flag token
// or one of its arguments, and ok=false otherwise (caller falls back to
// command-level help). The receiver carries the in-scope flag set (the
// synthetic leaf command for subcommand CLIs, or the command itself).
func (cmd *Command) flagHelpAt(ctx CompletionContext) (string, bool) {
	// Case 1: cursor is on the flag token itself (e.g. "-sum"). analyze
	// returns early for flag-like partials without setting FlagName, so
	// match the partial against a known flag name directly.
	if strings.HasPrefix(ctx.Partial, "-") || strings.HasPrefix(ctx.Partial, "+") {
		if spec := cmd.findFlagByName(ctx.Partial); spec != nil {
			return cmd.renderFlagHelp(spec, -1), true
		}
		return "", false
	}
	// Case 2: cursor is on an argument of a flag (FlagName set by analyze).
	if ctx.FlagName != "" {
		if spec := cmd.findFlagByName(ctx.FlagName); spec != nil {
			return cmd.renderFlagHelp(spec, ctx.ArgIndex), true
		}
	}
	return "", false
}

// findFlagByName resolves a flag spec by any of its names (e.g. "-sum" or
// "-s"). A leading "+" clause-separator form is normalized to "-".
func (cmd *Command) findFlagByName(name string) *FlagSpec {
	if strings.HasPrefix(name, "+") {
		name = "-" + name[1:]
	}
	for _, spec := range cmd.flags {
		for _, n := range spec.Names {
			if n == name {
				return spec
			}
		}
	}
	return nil
}

// renderFlagHelp renders compact, popup-friendly help for a single flag.
// currentArg is the 0-based index of the argument under the cursor, or -1
// when the cursor is on the flag name itself.
func (cmd *Command) renderFlagHelp(spec *FlagSpec, currentArg int) string {
	var sb strings.Builder

	// Signature line: "-sum, -s FIELD RESULT"
	sb.WriteString(strings.Join(spec.Names, ", "))
	for i := 0; i < spec.ArgCount; i++ {
		sb.WriteString(" ")
		if i < len(spec.ArgNames) && spec.ArgNames[i] != "" {
			sb.WriteString(spec.ArgNames[i])
		} else {
			sb.WriteString(fmt.Sprintf("ARG%d", i))
		}
	}
	sb.WriteString("\n")

	if spec.Description != "" {
		sb.WriteString(fmt.Sprintf("    %s\n", spec.Description))
	}

	// Per-argument detail, marking the one under the cursor.
	for i := 0; i < spec.ArgCount; i++ {
		name := fmt.Sprintf("ARG%d", i)
		if i < len(spec.ArgNames) && spec.ArgNames[i] != "" {
			name = spec.ArgNames[i]
		}
		marker := "  "
		if i == currentArg {
			marker = "→ "
		}
		typeName := "string"
		if i < len(spec.ArgTypes) {
			typeName = argTypeName(spec.ArgTypes[i])
		}
		sb.WriteString(fmt.Sprintf("    %s%s (%s)\n", marker, name, typeName))
	}

	// Scope / required / multi-value, mirroring formatFlag's vocabulary.
	if spec.Scope == ScopeGlobal {
		sb.WriteString("    Scope: global\n")
	} else {
		sb.WriteString("    Scope: per-clause\n")
	}
	if spec.Required {
		sb.WriteString("    Required: yes\n")
	}
	if spec.IsSlice {
		sb.WriteString("    Can be specified multiple times\n")
	}
	if spec.Default != nil {
		sb.WriteString(fmt.Sprintf("    Default: %v\n", spec.Default))
	}

	return sb.String()
}

// argTypeName renders an ArgType as a human label, matching the vocabulary
// used by formatPositional / formatPositionalForSubcommand.
func argTypeName(t ArgType) string {
	switch t {
	case ArgInt:
		return "integer"
	case ArgFloat:
		return "float"
	case ArgBool:
		return "boolean"
	case ArgDuration:
		return "duration"
	case ArgTime:
		return "time"
	default:
		return "string"
	}
}

// handleHelpAtTo implements the bash `-help-at` protocol: args is
// [position, word1, word2, ...] (same shape as -complete). It writes the
// rendered help for the cursor to w. The io.Writer-aware form lets embedded
// callers capture output, exactly like handleCompletionTo.
func (cmd *Command) handleHelpAtTo(args []string, w io.Writer) error {
	if len(args) < 1 {
		return fmt.Errorf("help-at requires position argument")
	}
	pos, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid position: %s", args[0])
	}
	helpArgs := []string{}
	if len(args) > 1 {
		helpArgs = args[1:]
	}
	text, err := cmd.HelpAt(helpArgs, pos)
	if err != nil {
		return err
	}
	fmt.Fprint(w, text)
	return nil
}
