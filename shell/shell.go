// Package shell drives an autocli Command from an interactive
// line-editing loop instead of the bash completion protocol.
//
// It's the layer-2 driver from the autocli-shell proposal: TAB hits
// Command.Complete to fetch suggestions, Enter parses the line and
// runs Command.ExecuteWith. The same command tree powers both a
// bash-CLI invocation and an embedded interactive session — no
// duplication.
//
// Lives in a sub-module so autocli core stays stdlib-only; embedded
// callers opt in to golang.org/x/term by importing this package.
//
// v0.2 switched the underlying line editor from chzyer/readline to
// golang.org/x/term. The motivation: chzyer/readline's design
// hardcodes os.Stdin in multiple places (MakeRaw, SetVimMode) which
// broke ssh-channel-backed sessions in three different ways. x/term
// takes an explicit io.ReadWriter, doesn't manage termios itself
// (caller responsibility — well-suited to the ssh-channel case where
// no termios applies), and is maintained by the Go team. Cost: no
// vi mode (x/term is emacs-only).
package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	cf "github.com/rosscartlidge/autocli/v4"
	"golang.org/x/term"
)

// EditingMode is kept for API compatibility with v0.1 callers. As of
// v0.2 only emacs mode is implemented (the underlying line editor is
// x/term, which is emacs-only). `:set vi` prints a deprecation note
// instead of switching keybindings.
type EditingMode int

const (
	// EditingEmacs — the default and only functional mode in v0.2+.
	EditingEmacs EditingMode = iota
	// EditingVi — accepted but not implemented in v0.2. See package
	// docs for the rationale.
	EditingVi
)

// Options configures a shell session.
type Options struct {
	// Prompt is the line-editor prompt. Defaults to "> ".
	Prompt string

	// HistoryFile, if non-empty, persists the session's command history.
	// Empty means in-memory only. File-format is one line per entry,
	// oldest-first; capped at 1000 entries.
	HistoryFile string

	// EditingMode used to switch between emacs and vi keybindings.
	// As of v0.2 only emacs is implemented; the field is preserved
	// for API stability. See package docs.
	EditingMode EditingMode

	// PrefsFile is preserved for v0.1 callers but functionally
	// inactive — the only thing it used to persist was the editing
	// mode, which is now constant.
	PrefsFile string

	// State is the caller-supplied service handle threaded through to
	// every handler via Context.State. Type-asserted by the handler.
	State any

	// Welcome banner printed once when the loop starts. Defaults to none.
	Welcome string

	// Goodbye banner printed on :exit / Ctrl-D. Defaults to none.
	Goodbye string

	// Stdin / Stdout / Stderr override the streams the line editor
	// reads from and writes to. Defaults: os.Stdin / os.Stdout / os.Stderr.
	// SSH adapters override these with the session's channel.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// Ctx, if set, is observed by the loop — cancelling it stops the
	// session at the next iteration. Defaults to context.Background.
	// To actually break the line editor out of a blocking Read, the
	// caller is responsible for closing Stdin (autocli/ssh does this
	// by closing the underlying SSH channel).
	Ctx context.Context

	// OnError, if set, is called for every non-nil handler/tokenize
	// error. Useful for structured logging in a service. The error is
	// also printed to Stderr regardless.
	OnError func(error)

	// Settings is the per-session list of named knobs the operator
	// can read/change via `:set`. Empty/nil means `:set` reports
	// "no configurable settings". See Setting for the contract.
	Settings []Setting

	// ResizeChan, if non-nil, is read by Serve in its own goroutine
	// for terminal-size updates. Each TerminalSize received causes
	// the underlying x/term.Terminal to update its size, which fixes
	// line-wrap behaviour at the operator's actual terminal width.
	//
	// Senders should non-blocking-send (drop on full channel) — we
	// only care about the latest size, and the buffer is small. The
	// caller does NOT need to close the chan; Serve cancels its
	// internal reader on return. Closing is also accepted (reader
	// will see ok=false and exit).
	//
	// autocli/ssh wires this up from pty-req + window-change SSH
	// payloads; local shell.Serve callers can ignore it (x/term
	// inspects the local terminal directly when MakeRaw is used).
	ResizeChan <-chan TerminalSize
}

// TerminalSize is a width+height pair pushed through Options.ResizeChan
// to update the underlying terminal's dimensions mid-session.
type TerminalSize struct {
	Width  int
	Height int
}

// Serve runs the shell loop until :exit, :quit, Ctrl-D / Ctrl-C, or
// Stdin closure. Returns nil for clean exit; non-nil only on fatal
// init failure (handler errors are reported to the user and the loop
// continues).
//
// If Stdin is a real terminal (os.Stdin AND IsTerminal), the
// function puts it in raw mode for the duration and restores on
// exit. SSH-channel callers pass a non-terminal io.Reader and we
// skip the termios dance entirely.
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

	// Put a real local terminal in raw mode if applicable. SSH
	// channels and piped input skip this — x/term will read raw
	// bytes either way.
	if f, ok := opts.Stdin.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		state, err := term.MakeRaw(int(f.Fd()))
		if err != nil {
			return fmt.Errorf("shell: makeraw: %w", err)
		}
		defer term.Restore(int(f.Fd()), state)
	}

	// x/term wants a combined io.ReadWriter; merge the caller's
	// streams. We don't write to Stderr through the terminal —
	// errors go directly to opts.Stderr below.
	//
	// The reader is wrapped in ctrlCReader so we can distinguish
	// Ctrl-C (cancel current line, keep session) from Ctrl-D (exit).
	// x/term returns io.EOF for both — see x/term terminal.go ~L820 —
	// so we sniff the most recent Read for 0x03 (ETX) and consult it
	// in the loop below.
	cr := &ctrlCReader{r: opts.Stdin}
	rw := readWriter{Reader: cr, Writer: opts.Stdout}
	t := term.NewTerminal(rw, opts.Prompt)

	if opts.HistoryFile != "" {
		t.History = newFileHistory(opts.HistoryFile)
	}

	// TAB completion. AutoCompleteCallback fires on EVERY keypress;
	// filter for TAB (rune 9) and pass through otherwise.
	t.AutoCompleteCallback = func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
		if key != '\t' {
			return "", 0, false
		}
		return tabComplete(cli, line, pos, t, opts.Stdout)
	}

	// Resize reader. Per-Serve cancellation so the goroutine exits
	// when Serve returns even if the caller forgot to close the
	// chan. x/term.Terminal.SetSize takes its internal lock so
	// concurrent calls with ReadLine are safe.
	if opts.ResizeChan != nil {
		sessCtx, cancel := context.WithCancel(opts.Ctx)
		defer cancel()
		go func() {
			for {
				select {
				case <-sessCtx.Done():
					return
				case sz, ok := <-opts.ResizeChan:
					if !ok {
						return
					}
					_ = t.SetSize(sz.Width, sz.Height)
				}
			}
		}()
	}

	if opts.Welcome != "" {
		fmt.Fprintln(t, opts.Welcome)
	}

	for {
		if err := opts.Ctx.Err(); err != nil {
			break
		}
		line, err := t.ReadLine()
		if err == io.EOF {
			// Ctrl-D or channel closed. Ctrl-C is handled below via
			// the ctrlCReader translation, so a real io.EOF here means
			// the session should end.
			break
		}
		if cr.consumeCtrlC() {
			// User pressed Ctrl-C — ctrlCReader translated it to
			// Enter, which made ReadLine return the partial line.
			// Discard the line, print ^C, and prompt again.
			fmt.Fprintln(t, "^C")
			continue
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
		if exit, handled := dispatchBuiltin(line, t, &opts); handled {
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
			fmt.Fprintln(t, cli.GenerateHelpEmbedded())
			continue
		}

		// Single commands and pipeline stage 0 read from an empty
		// stdin. Routing opts.Stdin (the SSH channel / TTY) into a
		// command's stdin would let a transform like `where` swallow
		// the operator's next keystrokes and appear to hang the
		// session. Commands that need data are expected to take a
		// pipeline upstream (e.g. `from-loaded | where ...`).
		base := (&cf.Context{State: opts.State}).
			SetStdin(bytes.NewReader(nil)).
			SetStdout(t).
			SetStderr(opts.Stderr).
			SetCtx(opts.Ctx)

		// Pipeline detection. `|` as a standalone token means the
		// user wants Position 2 piping: split into stages, wire
		// io.Pipes between them, run goroutine-per-stage. A line
		// without `|` falls through to the single-command path.
		if stages, hasPipe, perr := splitOnPipe(args); hasPipe {
			if perr != nil {
				fmt.Fprintf(opts.Stderr, "%v\n", perr)
				if opts.OnError != nil {
					opts.OnError(perr)
				}
				continue
			}
			if err := runPipeline(cli, stages, base); err != nil {
				fmt.Fprintf(opts.Stderr, "%v\n", err)
				if opts.OnError != nil {
					opts.OnError(err)
				}
			}
			continue
		}

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
		fmt.Fprintln(t, opts.Goodbye)
	}
	return nil
}

// readWriter trivially composes Reader + Writer into a ReadWriter
// (what x/term.NewTerminal wants). Splitting them in Options is
// nicer for callers since SSH gives them separately anyway.
type readWriter struct {
	io.Reader
	io.Writer
}

// ctrlCReader wraps an io.Reader and translates Ctrl-C (0x03) into
// CR (0x0d). x/term's ReadLine returns io.EOF on Ctrl-C without
// consuming the 0x03 from its internal remainder buffer, which means
// every subsequent ReadLine call also returns io.EOF — fatal to a
// long-lived interactive shell. Translating to CR causes x/term to
// treat the keypress as Enter, return the current line buffer, and
// reset its state cleanly. The Serve loop checks `consumeCtrlC()`
// after each ReadLine and discards the returned line (which is the
// partial input the user had typed) when a Ctrl-C was seen.
type ctrlCReader struct {
	r      io.Reader
	sawETX bool
}

func (c *ctrlCReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	for i := 0; i < n; i++ {
		if p[i] == 0x03 {
			c.sawETX = true
			p[i] = 0x0d
		}
	}
	return n, err
}

// Compile-time check that ctrlCReader matches io.Reader (kept so a
// refactor that drops the method surface fails at build time).
var _ io.Reader = (*ctrlCReader)(nil)

func (c *ctrlCReader) consumeCtrlC() bool {
	had := c.sawETX
	c.sawETX = false
	return had
}

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

// tabComplete is the per-TAB-press completer.
//
//   - Single match → replace the current word with the match + space.
//   - Multiple matches → print them to the writer (newline-aware),
//     leave the line unchanged. Operator sees the options and types
//     more characters to disambiguate.
//   - No match → no-op.
//
// Returns the (newLine, newPos, ok) tuple x/term's AutoCompleteCallback
// expects. When ok is false, the keypress is processed normally.
func tabComplete(cli *cf.Command, line string, pos int, w io.Writer, listSink io.Writer) (string, int, bool) {
	args, partialStart := tokenizePartial(line[:pos])
	// Trailing-whitespace handling — see autocli/shell v0.1.3.
	if len(line[:pos]) > 0 {
		last := line[pos-1]
		if last == ' ' || last == '\t' {
			args = append(args, "")
		}
	}
	// Pipe-aware completion: each `|` starts a fresh command stage, so
	// complete against the args after the last pipe. Without this, the
	// completer would treat `|` as a positional argument to the first
	// command and offer that command's flags instead of the next
	// stage's subcommands.
	for j := len(args) - 1; j >= 0; j-- {
		if args[j] == "|" {
			args = args[j+1:]
			break
		}
	}
	completions, err := cli.Complete(args, len(args))
	if err != nil || len(completions) == 0 {
		return "", 0, false
	}

	partial := line[partialStart:pos]

	if len(completions) == 1 {
		// Replace the current word and add a trailing space so the
		// user can keep typing the next argument.
		head := line[:partialStart]
		tail := line[pos:]
		insert := completions[0]
		if !strings.HasSuffix(insert, " ") {
			insert += " "
		}
		newLine := head + insert + tail
		newPos := partialStart + len(insert)
		return newLine, newPos, true
	}

	// Multiple matches. Filter to those that actually start with the
	// partial — we treat them as candidates only.
	var matches []string
	for _, c := range completions {
		if strings.HasPrefix(c, partial) {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		matches = completions
	}

	// If they share a longer common prefix, insert that.
	common := longestCommonPrefix(matches)
	if len(common) > len(partial) {
		head := line[:partialStart]
		tail := line[pos:]
		newLine := head + common + tail
		newPos := partialStart + len(common)
		return newLine, newPos, true
	}

	// Otherwise list options on a new line; x/term will redraw the
	// prompt + current line below.
	fmt.Fprintln(listSink, "\n"+strings.Join(matches, "  "))
	return "", 0, false
}

func longestCommonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	p := ss[0]
	for _, s := range ss[1:] {
		// Shrink p until s starts with it.
		for !strings.HasPrefix(s, p) {
			p = p[:len(p)-1]
			if p == "" {
				return ""
			}
		}
	}
	return p
}

// tokenizePartial returns the args parsed from prefix and the byte
// offset where the trailing partial word starts. Used by the
// completer to figure out how much of the line to replace.
func tokenizePartial(prefix string) (args []string, partialStart int) {
	args, _ = Tokenize(prefix)
	if len(prefix) == 0 {
		return args, 0
	}
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
