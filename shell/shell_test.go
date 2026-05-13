package shell

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	cf "github.com/rosscartlidge/autocli/v4"
)

// buildTestCLI returns a tiny command tree useful for shell loop tests.
// The increment / read pair lets a test verify per-line dispatch AND
// State persistence across invocations.
func buildTestCLI(state *testState) *cf.Command {
	return cf.NewCommand("svc").
		Subcommand("inc").
		Handler(func(ctx *cf.Context) error {
			s := ctx.State.(*testState)
			s.n++
			fmt.Fprintf(ctx.Stdout(), "n=%d\n", s.n)
			return nil
		}).
		Done().
		Subcommand("read").
		Handler(func(ctx *cf.Context) error {
			s := ctx.State.(*testState)
			fmt.Fprintf(ctx.Stdout(), "current=%d\n", s.n)
			return nil
		}).
		Done().
		Subcommand("echo").
		Flag("MSG").String().Variadic().Global().Required().Done().
		Handler(func(ctx *cf.Context) error {
			raw := ctx.GlobalFlags["MSG"]
			var parts []string
			switch v := raw.(type) {
			case []string:
				parts = v
			case []interface{}:
				parts = make([]string, len(v))
				for i, x := range v {
					parts[i] = fmt.Sprintf("%v", x)
				}
			}
			fmt.Fprintln(ctx.Stdout(), strings.Join(parts, " "))
			return nil
		}).
		Done().
		Build()
}

type testState struct{ n int }

// runShellWithInput drives Serve against scripted stdin and captures
// stdout for assertions. Returns when Serve returns (EOF or :exit).
func runShellWithInput(t *testing.T, cli *cf.Command, opts Options, input string) string {
	t.Helper()
	var stdout bytes.Buffer
	opts.Stdin = io.NopCloser(strings.NewReader(input))
	opts.Stdout = &stdout
	opts.Stderr = &stdout // co-located to simplify assertions

	// Safety net: shells that don't terminate within 2s fail the test.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	opts.Ctx = ctx

	if err := Serve(cli, opts); err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
	return stdout.String()
}

// TestServe_DispatchesCommands runs three lines through Serve and
// asserts each handler ran and saw the same State.
func TestServe_DispatchesCommands(t *testing.T) {
	state := &testState{}
	cli := buildTestCLI(state)

	out := runShellWithInput(t, cli, Options{State: state}, "inc\ninc\nread\n")

	if state.n != 2 {
		t.Errorf("state.n = %d, want 2", state.n)
	}
	if !strings.Contains(out, "n=1") || !strings.Contains(out, "n=2") {
		t.Errorf("incremental output missing in: %q", out)
	}
	if !strings.Contains(out, "current=2") {
		t.Errorf("read output missing in: %q", out)
	}
}

// TestServe_BuiltinExit asserts :exit terminates the loop even if
// more lines follow.
func TestServe_BuiltinExit(t *testing.T) {
	state := &testState{}
	cli := buildTestCLI(state)

	out := runShellWithInput(t, cli, Options{State: state}, "inc\n:exit\ninc\n")

	if state.n != 1 {
		t.Errorf("state.n = %d, want 1 (loop must stop after :exit)", state.n)
	}
	if strings.Count(out, "n=") != 1 {
		t.Errorf("expected exactly one inc output, got: %q", out)
	}
}

// TestServe_BuiltinHelp asserts :help prints something.
func TestServe_BuiltinHelp(t *testing.T) {
	cli := buildTestCLI(&testState{})

	out := runShellWithInput(t, cli, Options{}, ":help\n:exit\n")

	if !strings.Contains(out, ":exit") {
		t.Errorf(":help output missing :exit reference: %q", out)
	}
}

// TestServe_BuiltinSetMode toggles vi/emacs and verifies the
// Options-side state changes (readline-side is internal).
func TestServe_BuiltinSetMode(t *testing.T) {
	cli := buildTestCLI(&testState{})

	out := runShellWithInput(t, cli, Options{}, ":set\n:set vi\n:set\n:set emacs\n:set\n:exit\n")

	// First :set reports emacs (default), then vi, then emacs.
	gotVi := strings.Index(out, "editing-mode: vi")
	emacs := []int{}
	idx := 0
	for {
		j := strings.Index(out[idx:], "editing-mode: emacs")
		if j < 0 {
			break
		}
		emacs = append(emacs, idx+j)
		idx += j + 1
	}
	if gotVi < 0 || len(emacs) < 2 {
		t.Errorf("expected one vi + two emacs reports, got: %q", out)
	}
}

// TestServe_QuotedArgs verifies the tokeniser cooperates with the
// dispatch path on quoted variadic input.
func TestServe_QuotedArgs(t *testing.T) {
	cli := buildTestCLI(&testState{})

	out := runShellWithInput(t, cli, Options{}, `echo "hello world" 'literal $x'`+"\n:exit\n")

	if !strings.Contains(out, "hello world literal $x") {
		t.Errorf("quoted-arg output wrong: %q", out)
	}
}

// TestServe_HandlerErrorContinues asserts a handler error doesn't
// kill the loop — gets reported, prompt comes back.
func TestServe_HandlerErrorContinues(t *testing.T) {
	cli := cf.NewCommand("svc").
		Subcommand("boom").
		Handler(func(ctx *cf.Context) error {
			return fmt.Errorf("simulated failure")
		}).
		Done().
		Subcommand("ping").
		Handler(func(ctx *cf.Context) error {
			fmt.Fprintln(ctx.Stdout(), "pong")
			return nil
		}).
		Done().
		Build()

	out := runShellWithInput(t, cli, Options{}, "boom\nping\n:exit\n")

	if !strings.Contains(out, "simulated failure") {
		t.Errorf("error not surfaced: %q", out)
	}
	if !strings.Contains(out, "pong") {
		t.Errorf("loop didn't continue after error: %q", out)
	}
}

// TestServe_UnknownCommandShowsFriendlyError asserts that typing
// a non-flag word that isn't a registered subcommand produces a
// "unknown command: …" line rather than dumping the full autocli
// help screen. Validates the v4.6.0 ErrUnknownCommand path.
func TestServe_UnknownCommandShowsFriendlyError(t *testing.T) {
	cli := buildTestCLI(&testState{})

	out := runShellWithInput(t, cli, Options{}, "junk\n:exit\n")

	if !strings.Contains(out, "unknown command") {
		t.Errorf("expected friendly unknown-command message; got: %q", out)
	}
	// And specifically NOT the bash-completion footer that would have
	// shown up if autocli's GenerateHelp() fell through.
	if strings.Contains(out, "SHELL COMPLETION") {
		t.Errorf("unknown command path leaked bash help: %q", out)
	}
}

// TestServe_DashHelpUsesEmbeddedForm asserts typing `-help` at the
// prompt emits the embedded help (no SHELL COMPLETION footer / no
// -man reference) rather than the bash flavoured form.
func TestServe_DashHelpUsesEmbeddedForm(t *testing.T) {
	cli := buildTestCLI(&testState{})

	out := runShellWithInput(t, cli, Options{}, "-help\n:exit\n")

	if !strings.Contains(out, "COMMANDS:") {
		t.Errorf("expected -help to print the COMMANDS section; got: %q", out)
	}
	if strings.Contains(out, "SHELL COMPLETION") {
		t.Errorf("-help leaked the bash-completion footer: %q", out)
	}
	if strings.Contains(out, "-man") {
		t.Errorf("-help mentioned -man (bash-only feature): %q", out)
	}
}

// TestServe_EOFExits asserts Ctrl-D (EOF on stdin) ends the loop.
func TestServe_EOFExits(t *testing.T) {
	state := &testState{}
	cli := buildTestCLI(state)

	// Input has no :exit — only EOF.
	out := runShellWithInput(t, cli, Options{State: state}, "inc\n")

	if state.n != 1 {
		t.Errorf("state.n = %d, want 1", state.n)
	}
	if !strings.Contains(out, "n=1") {
		t.Errorf("missing inc output: %q", out)
	}
}
