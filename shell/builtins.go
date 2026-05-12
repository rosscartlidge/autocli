package shell

import (
	"fmt"
	"strings"

	"github.com/chzyer/readline"
)

// dispatchBuiltin handles shell-level :commands that don't go through
// the autocli Command tree. Returns (exitRequested, handled).
//
// Built-ins always start with ":" so they can't collide with
// user-defined subcommands.
func dispatchBuiltin(line string, rl *readline.Instance, opts *Options) (exit, handled bool) {
	if !strings.HasPrefix(line, ":") {
		return false, false
	}
	fields := strings.Fields(line)
	switch fields[0] {
	case ":exit", ":quit", ":q":
		return true, true
	case ":help":
		fmt.Fprintln(opts.Stdout, builtinHelpText())
		return false, true
	case ":history":
		// Readline writes history to its file/buffer; show what we have.
		// chzyer/readline doesn't expose a public iterator, so we just
		// surface the file path if persistent, else a placeholder.
		if opts.HistoryFile != "" {
			fmt.Fprintf(opts.Stdout, "history file: %s\n", opts.HistoryFile)
		} else {
			fmt.Fprintln(opts.Stdout, "history is session-only (no HistoryFile configured)")
		}
		return false, true
	case ":set":
		return false, dispatchSet(fields[1:], rl, opts)
	}
	// Unknown :command — flag it so the user doesn't accidentally
	// silent-ignore a typo of an autocli command.
	fmt.Fprintf(opts.Stderr, "unknown built-in: %s (try :help)\n", fields[0])
	return false, true
}

// dispatchSet implements `:set vi` / `:set emacs` and bare `:set`
// (show current mode). Returns true to signal the line was handled.
//
// NOTE: chzyer/readline's runtime SetVimMode races with its own
// input goroutine (writes to a shared opVim state while readline
// reads it). The race is library-internal, not autocli-shell's, but
// we surface it by deferring the actual mode switch until the next
// readline session — i.e. the user must :exit and reconnect for the
// new bindings to take effect. The bookkeeping is updated immediately
// so per-user-prefs persistence works.
func dispatchSet(args []string, rl *readline.Instance, opts *Options) bool {
	_ = rl // unused — see note above
	if len(args) == 0 {
		mode := "emacs"
		if opts.EditingMode == EditingVi {
			mode = "vi"
		}
		fmt.Fprintf(opts.Stdout, "editing-mode: %s\n", mode)
		return true
	}
	switch args[0] {
	case "vi":
		prev := opts.EditingMode
		opts.EditingMode = EditingVi
		fmt.Fprintln(opts.Stdout, "editing-mode: vi")
		if prev != EditingVi {
			fmt.Fprintln(opts.Stdout, "(takes effect on next session)")
		}
	case "emacs":
		prev := opts.EditingMode
		opts.EditingMode = EditingEmacs
		fmt.Fprintln(opts.Stdout, "editing-mode: emacs")
		if prev != EditingEmacs {
			fmt.Fprintln(opts.Stdout, "(takes effect on next session)")
		}
	default:
		fmt.Fprintf(opts.Stderr, ":set: unknown option %q (try vi or emacs)\n", args[0])
	}
	return true
}

func builtinHelpText() string {
	return strings.TrimLeft(`
shell built-ins (start with :)
  :help              show this help
  :exit / :quit      close the session
  :history           show session history info
  :set               show current editing mode
  :set vi            switch to vi keybindings
  :set emacs         switch to emacs keybindings (default)

everything else routes to the autocli command tree.
`, "\n")
}
