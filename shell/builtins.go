package shell

import (
	"fmt"
	"io"
	"strings"
)

// dispatchBuiltin handles shell-level :commands that don't go through
// the autocli Command tree. Returns (exitRequested, handled).
//
// Built-ins always start with ":" so they can't collide with
// user-defined subcommands. The output sink (w) is the terminal —
// either the x/term.Terminal (which renders ANSI correctly) or an
// io.Writer compatible substitute for tests.
func dispatchBuiltin(line string, w io.Writer, opts *Options) (exit, handled bool) {
	if !strings.HasPrefix(line, ":") {
		return false, false
	}
	fields := strings.Fields(line)
	switch fields[0] {
	case ":exit", ":quit", ":q":
		return true, true
	case ":help":
		fmt.Fprintln(w, builtinHelpText())
		return false, true
	case ":history":
		if opts.HistoryFile != "" {
			fmt.Fprintf(w, "history file: %s\n", opts.HistoryFile)
		} else {
			fmt.Fprintln(w, "history is session-only (no HistoryFile configured)")
		}
		return false, true
	case ":set":
		dispatchSet(fields[1:], w, opts)
		return false, true
	}
	fmt.Fprintf(opts.Stderr, "unknown built-in: %s (try :help)\n", fields[0])
	return false, true
}

// dispatchSet implements the `:set` built-in:
//
//	:set                    list registered settings with current values
//	:set NAME               show the current value of NAME
//	:set NAME VALUE...      apply VALUE (joined with spaces) to NAME
//
// Settings are supplied by the caller via Options.Settings. Empty
// list = "no configurable settings". Setting.Set returning an error
// is surfaced to the operator without ending the session.
//
// v0.2.1 replaced the v0.1-era hard-coded vi/emacs toggle with this
// generic registry. The `vi`/`emacs` words now hit the unknown-
// setting error path; the x/term migration in v0.2 made them no-ops
// anyway.
func dispatchSet(args []string, w io.Writer, opts *Options) {
	if len(opts.Settings) == 0 {
		fmt.Fprintln(w, "(no configurable settings)")
		return
	}
	if len(args) == 0 {
		// Listing form. Two columns: "name: value" then a description
		// underneath. Skip the description line if empty.
		nameWidth := 0
		for _, s := range opts.Settings {
			if n := len(s.Name); n > nameWidth {
				nameWidth = n
			}
		}
		for _, s := range opts.Settings {
			fmt.Fprintf(w, "  %-*s = %s\n", nameWidth, s.Name, s.Get())
			if s.Description != "" {
				fmt.Fprintf(w, "  %-*s   (%s)\n", nameWidth, "", s.Description)
			}
		}
		return
	}
	name := args[0]
	for _, s := range opts.Settings {
		if s.Name != name {
			continue
		}
		if len(args) == 1 {
			fmt.Fprintf(w, "%s = %s\n", s.Name, s.Get())
			return
		}
		value := strings.Join(args[1:], " ")
		if err := s.Set(value); err != nil {
			fmt.Fprintf(opts.Stderr, ":set %s: %v\n", name, err)
			return
		}
		fmt.Fprintf(w, "%s = %s\n", s.Name, s.Get())
		return
	}
	fmt.Fprintf(opts.Stderr, ":set: unknown setting %q (try `:set` with no args to list)\n", name)
}

func builtinHelpText() string {
	return strings.TrimLeft(`
shell built-ins (start with :)
  :help              show this help (built-ins only)
  :exit / :quit / :q close the session
  :history           show session history info
  :set               show current editing mode

For the available service commands type:
  -help              full command tree with descriptions
  <command> -help    detailed help for one command
  <TAB><TAB>         autocomplete options at the current position

(Ctrl-C / Ctrl-D also close the session.)
`, "\n")
}
