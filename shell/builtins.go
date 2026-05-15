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

// dispatchSet implements `:set vi` / `:set emacs` and bare `:set`
// (show current mode).
//
// v0.2 note: the underlying line editor (golang.org/x/term) is
// emacs-only. We accept the `:set vi` command for backward
// compatibility with v0.1 muscle memory but print a deprecation
// notice. EditingMode is recorded so prefs.json round-trips remain
// stable for callers that wrote a `vi` choice under v0.1.
func dispatchSet(args []string, w io.Writer, opts *Options) {
	if len(args) == 0 {
		mode := "emacs"
		if opts.EditingMode == EditingVi {
			mode = "vi (no-op — see below)"
		}
		fmt.Fprintf(w, "editing-mode: %s\n", mode)
		if opts.EditingMode == EditingVi {
			fmt.Fprintln(w, "note: vi mode is not implemented in shell v0.2+ (the underlying line editor is x/term, which is emacs-only). The setting is accepted but has no effect.")
		}
		return
	}
	switch args[0] {
	case "vi":
		fmt.Fprintln(w, "editing-mode: vi (accepted but inactive — see :set without args)")
		fmt.Fprintln(w, "note: shell v0.2 dropped vi support when migrating to x/term. The keybindings remain emacs-style.")
		opts.EditingMode = EditingVi
	case "emacs":
		fmt.Fprintln(w, "editing-mode: emacs")
		opts.EditingMode = EditingEmacs
	default:
		fmt.Fprintf(opts.Stderr, ":set: unknown option %q (try vi or emacs)\n", args[0])
	}
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
