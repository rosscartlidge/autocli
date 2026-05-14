// Package shell drives an autocli Command from an interactive
// readline loop instead of the bash completion protocol.
//
// It's the layer-2 driver from the autocli-shell proposal: TAB hits
// Command.Complete to fetch suggestions, Enter parses the line and
// runs Command.ExecuteWith. The same command tree powers both a
// bash-CLI invocation and an embedded interactive session — no
// duplication.
//
// Lives in a sub-module so autocli core stays stdlib-only; embedded
// callers opt in to chzyer/readline by importing this package.
package shell

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	cf "github.com/rosscartlidge/autocli/v4"
)

// isHelpToken returns true for the conventional help-request words
// users might type at a prompt. `help` is included so users without
// the dash habit don't get told their command is unknown when they
// were really asking for the menu.
func isHelpToken(s string) bool {
	switch s {
	case "-help", "--help", "-h", "help", "?":
		return true
	}
	return false
}

// EditingMode selects the readline keybinding set.
type EditingMode int

const (
	// EditingEmacs is GNU readline emacs-mode (Ctrl-A/E/K/W/R/etc.).
	// The default for autocli/shell.
	EditingEmacs EditingMode = iota
	// EditingVi is GNU readline vi-mode (normal/insert, hjkl motions, dd etc.).
	EditingVi
)

// Options configures a shell session.
type Options struct {
	// Prompt is the readline prompt. Defaults to "> ".
	Prompt string

	// HistoryFile, if non-empty, persists the session's command history.
	// Empty means in-memory only.
	HistoryFile string

	// EditingMode picks emacs (default) or vi keybindings at startup.
	// Operators flip at runtime with :set vi / :set emacs — the choice
	// is written back to PrefsFile so the next session reads it.
	// Currently :set takes effect on next session, not mid-session,
	// because chzyer/readline's runtime SetVimMode races with its own
	// input goroutine. See shell/README.md "Editing modes".
	EditingMode EditingMode

	// PrefsFile, if non-empty, is the path to a per-user JSON file
	// holding shell preferences (currently just the editing mode).
	// Serve reads it on session start (any value found overrides
	// Options.EditingMode); :set vi/emacs writes it back. Empty =
	// no persistence, :set only affects the current session's
	// bookkeeping. autocli/ssh sets this per-session under
	// Options.HistoryDir/$user/prefs.json when HistoryDir is set.
	PrefsFile string

	// State is the caller-supplied service handle threaded through to
	// every handler via Context.State. Type-asserted by the handler.
	State any

	// Welcome banner printed once when the loop starts. Defaults to none.
	Welcome string

	// Goodbye banner printed on :exit / Ctrl-D. Defaults to none.
	Goodbye string

	// Stdin / Stdout / Stderr override the streams the readline loop
	// reads from and writes to. Defaults: os.Stdin / os.Stdout / os.Stderr.
	// SSH adapters override these with the session's channel.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// Ctx, if set, is observed by the loop — cancelling it stops the
	// session at the next readline iteration. Defaults to context.Background.
	Ctx context.Context

	// OnError, if set, is called for every non-nil handler/tokenize
	// error. Useful for structured logging in a service. The error is
	// also printed to Stderr regardless.
	OnError func(error)
}

// Serve runs the shell loop until :exit, :quit, Ctrl-D, or Ctx
// cancellation. Returns nil for clean exit; non-nil only on
// readline-init errors or fatal IO failure (handler errors are
// reported to the user and the loop continues).
func Serve(cli *cf.Command, opts Options) error {
	if opts.Prompt == "" {
		opts.Prompt = "> "
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	if opts.Ctx == nil {
		opts.Ctx = context.Background()
	}

	// Per-user prefs override Options.EditingMode when present.
	// The :set vi/emacs builtin writes back to the same file so the
	// next session opens in the chosen mode.
	if mode, ok := loadPrefs(opts.PrefsFile); ok {
		opts.EditingMode = mode
	}

	stdinCloser, _ := opts.Stdin.(io.ReadCloser)
	if stdinCloser == nil {
		stdinCloser = io.NopCloser(opts.Stdin)
	}
	rlCfg := &readline.Config{
		Prompt:          opts.Prompt,
		HistoryFile:     opts.HistoryFile,
		VimMode:         opts.EditingMode == EditingVi,
		Stdin:           stdinCloser,
		Stdout:          opts.Stdout,
		Stderr:          opts.Stderr,
		InterruptPrompt: "^C",
		EOFPrompt:       "",

		AutoComplete: &autocliCompleter{cli: cli},
	}

	// chzyer/readline always calls MakeRaw on os.Stdin (FD 0)
	// regardless of Config.Stdin — its GetStdin() hardcodes
	// syscall.Stdin. When we're driving readline from an SSH channel
	// (not os.Stdin), that puts the SERVER process's controlling
	// terminal into raw mode, which disables ISIG, which makes
	// Ctrl-C in the terminal where `myapp` was launched stop
	// generating SIGINT. `kill -INT` still works because it bypasses
	// the tty driver, but Ctrl-C silently becomes a literal byte.
	//
	// Detect non-os.Stdin and override FuncMakeRaw/FuncExitRaw to
	// no-ops — the SSH client side already manages its own terminal
	// (it put the user's local terminal in raw mode for the session),
	// so the server side shouldn't touch any termios.
	if opts.Stdin != os.Stdin {
		rlCfg.FuncMakeRaw = func() error { return nil }
		rlCfg.FuncExitRaw = func() error { return nil }
	}
	rl, err := readline.NewEx(rlCfg)
	if err != nil {
		return fmt.Errorf("shell: readline init: %w", err)
	}
	defer rl.Close()

	if opts.Welcome != "" {
		fmt.Fprintln(opts.Stdout, opts.Welcome)
	}

	// Loop until the user (or our caller) signals shutdown.
	for {
		if err := opts.Ctx.Err(); err != nil {
			break
		}
		line, err := rl.Readline()
		if err == readline.ErrInterrupt {
			// Ctrl-C with no in-flight command: clear line, prompt again.
			continue
		}
		if err == io.EOF {
			// Ctrl-D or stdin closed.
			break
		}
		if err != nil {
			return fmt.Errorf("shell: readline: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Built-in :commands handled before tokenisation so users can
		// always escape (e.g. :exit even if the command tree has been
		// misconfigured).
		if exit, handled := dispatchBuiltin(line, rl, &opts); handled {
			if exit {
				break
			}
			continue
		}

		// Tokenise + execute via autocli.
		args, err := Tokenize(line)
		if err != nil {
			fmt.Fprintf(opts.Stderr, "shell: %v\n", err)
			if opts.OnError != nil {
				opts.OnError(err)
			}
			continue
		}

		// Intercept top-level help requests and emit the embedded
		// help text (no bash-completion footer / -man reference) —
		// the regular autocli path would dump the bash-flavoured form.
		if len(args) == 1 && isHelpToken(args[0]) {
			fmt.Fprintln(opts.Stdout, cli.GenerateHelpEmbedded())
			continue
		}

		base := (&cf.Context{State: opts.State}).
			SetStdin(opts.Stdin).
			SetStdout(opts.Stdout).
			SetStderr(opts.Stderr).
			SetCtx(opts.Ctx)
		if err := cli.ExecuteWith(args, base); err != nil {
			// Friendly message for unknown commands instead of dumping
			// the full help screen on every typo.
			var unknown cf.ErrUnknownCommand
			if errors.As(err, &unknown) {
				fmt.Fprintf(opts.Stderr, "unknown command: %q (try -help or :help)\n", string(unknown))
			} else {
				fmt.Fprintf(opts.Stderr, "%v\n", err)
			}
			if opts.OnError != nil {
				opts.OnError(err)
			}
		}
	}

	if opts.Goodbye != "" {
		fmt.Fprintln(opts.Stdout, opts.Goodbye)
	}
	return nil
}

// autocliCompleter adapts cli.Complete into the readline.AutoCompleter
// interface. Readline gives us the line as a []rune and the cursor
// position; we hand back candidate strings and the length of the
// prefix already typed so readline replaces just the partial word.
type autocliCompleter struct {
	cli *cf.Command
}

func (c *autocliCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	// Tokenise the line up to pos so we can mirror the bash protocol:
	// args = words before/at cursor; completion position = how many
	// words we've consumed including the partial.
	prefix := string(line[:pos])
	args, partialStart := tokenizePartial(prefix)

	// If the cursor is immediately AFTER a word-separator, the user
	// is starting a new word. Bash's COMP_WORDS would have an extra
	// empty entry at the cursor position; mirror that so autocli's
	// Complete sees "user is now typing word #(N+1)" and offers
	// completions for that slot (e.g. children of the previous
	// subcommand) rather than re-suggesting the previous word.
	// Without this, `to <TAB>` echoed `to` instead of showing
	// children like `table`.
	if len(prefix) > 0 {
		last := prefix[len(prefix)-1]
		if last == ' ' || last == '\t' {
			args = append(args, "")
		}
	}

	// Bash's COMP_WORDS includes the program name at index 0; the
	// args slice we pass to Complete is COMP_WORDS[1:], so the
	// position-in-COMP_WORDS for the word being completed is
	// len(args). That maps to len(args)-1 inside args itself, which
	// is the partial word the user is typing.
	pos1based := len(args)
	completions, err := c.cli.Complete(args, pos1based)
	if err != nil || len(completions) == 0 {
		return nil, 0
	}

	// Compute the partial word length so readline replaces only the
	// trailing word, not the whole line.
	partialLen := len(prefix) - partialStart
	if partialLen < 0 {
		partialLen = 0
	}
	partial := prefix[partialStart:]

	out := make([][]rune, 0, len(completions))
	for _, c := range completions {
		// Readline expects the SUFFIX (what to append after the
		// partial). Strip the partial prefix from each suggestion if
		// present; otherwise emit the whole suggestion.
		suffix := c
		if strings.HasPrefix(c, partial) {
			suffix = c[len(partial):]
		}
		out = append(out, []rune(suffix))
	}
	return out, partialLen
}

// tokenizePartial returns the args parsed from prefix and the byte
// offset where the trailing partial word starts. Used by the
// completer to figure out how much to replace.
func tokenizePartial(prefix string) (args []string, partialStart int) {
	// Find the last unquoted whitespace boundary.
	args, _ = Tokenize(prefix)
	if len(prefix) == 0 {
		return args, 0
	}
	// Walk back from end to find where the current word starts.
	i := len(prefix)
	for i > 0 {
		r := prefix[i-1]
		if r == ' ' || r == '\t' {
			break
		}
		i--
	}
	return args, i
}
