package completionflags

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestComplete_PublicAPI exercises the new Command.Complete entrypoint
// (added for autocli-shell embedding). It must return the same string
// list the bash protocol's `-complete N` flow would have printed,
// without writing anything.
func TestComplete_PublicAPI(t *testing.T) {
	cmd := NewCommand("test").
		Subcommand("hello").
		Description("greet").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Subcommand("status").
		Description("show status").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	// User typed `test hel<TAB>` — caret at position 1 of COMP_WORDS.
	// args (COMP_WORDS[1:]) = ["hel"], pos = 1.
	completions, err := cmd.Complete([]string{"hel"}, 1)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(completions) == 0 {
		t.Fatal("expected at least one completion for partial 'hel', got none")
	}
	found := false
	for _, c := range completions {
		if strings.HasPrefix(c, "hello") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'hello' in completions, got %v", completions)
	}
}

// TestComplete_MatchesInternalComplete confirms the public Complete is
// a pure pass-through over the internal complete() — the engine split
// is by design a rename, not a rewrite.
func TestComplete_MatchesInternalComplete(t *testing.T) {
	cmd := NewCommand("test").
		Subcommand("foo").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Subcommand("bar").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	cases := []struct {
		args []string
		pos  int
	}{
		{[]string{""}, 1},
		{[]string{"f"}, 1},
		{[]string{"foo"}, 1},
		{[]string{"foo", ""}, 2},
	}
	// Compare as sets — both paths walk Go maps internally, so order
	// across calls is not guaranteed (a quirk worth documenting but
	// not fixing here).
	asSet := func(s []string) map[string]struct{} {
		m := make(map[string]struct{}, len(s))
		for _, x := range s {
			m[x] = struct{}{}
		}
		return m
	}
	for _, tc := range cases {
		pub, perr := cmd.Complete(tc.args, tc.pos)
		priv, perr2 := cmd.complete(tc.args, tc.pos, completionSeed{})
		if (perr == nil) != (perr2 == nil) {
			t.Errorf("err divergence for args=%v pos=%d: pub=%v priv=%v", tc.args, tc.pos, perr, perr2)
			continue
		}
		pubSet, privSet := asSet(pub), asSet(priv)
		if len(pubSet) != len(privSet) {
			t.Errorf("count divergence for args=%v pos=%d: pub=%v priv=%v", tc.args, tc.pos, pub, priv)
			continue
		}
		for k := range pubSet {
			if _, ok := privSet[k]; !ok {
				t.Errorf("entry %q missing in priv for args=%v pos=%d", k, tc.args, tc.pos)
			}
		}
	}
}

// TestContext_IODefaults asserts the zero-value Context returns the
// standard os streams from its accessor methods — the bash-CLI path
// never sets these explicitly and must keep working.
func TestContext_IODefaults(t *testing.T) {
	var c Context
	if c.Stdin() != os.Stdin {
		t.Error("Stdin() default should be os.Stdin")
	}
	if c.Stdout() != os.Stdout {
		t.Error("Stdout() default should be os.Stdout")
	}
	if c.Stderr() != os.Stderr {
		t.Error("Stderr() default should be os.Stderr")
	}
	if c.Ctx() == nil {
		t.Error("Ctx() default should not be nil")
	}
}

// TestContext_IOOverride asserts the Set*() setters route reads through
// the embedded caller's streams.
func TestContext_IOOverride(t *testing.T) {
	var buf bytes.Buffer
	c := (&Context{}).
		SetStdout(&buf).
		SetStderr(&buf).
		SetStdin(strings.NewReader("hello\n"))

	if _, err := c.Stdout().Write([]byte("out\n")); err != nil {
		t.Fatalf("write Stdout: %v", err)
	}
	if _, err := c.Stderr().Write([]byte("err\n")); err != nil {
		t.Fatalf("write Stderr: %v", err)
	}
	got := buf.String()
	if got != "out\nerr\n" {
		t.Errorf("buf = %q, want %q", got, "out\nerr\n")
	}

	in := make([]byte, 5)
	n, err := c.Stdin().Read(in)
	if err != nil {
		t.Fatalf("read Stdin: %v", err)
	}
	if string(in[:n]) != "hello" {
		t.Errorf("Stdin first 5 = %q, want %q", in[:n], "hello")
	}
}

// TestContext_CtxOverride asserts the embedded caller's cancellation
// context flows through to handlers via Ctx().
func TestContext_CtxOverride(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	c := (&Context{}).SetCtx(parent)
	if c.Ctx() != parent {
		t.Error("Ctx() should return the SetCtx value")
	}
	cancel()
	if c.Ctx().Err() == nil {
		t.Error("Ctx() should observe parent cancellation")
	}
}

// TestContext_State asserts the State pass-through field is untouched
// by the dispatch machinery.
func TestContext_State(t *testing.T) {
	type myState struct{ value int }
	s := &myState{value: 42}
	c := &Context{State: s}
	got, ok := c.State.(*myState)
	if !ok {
		t.Fatalf("State type assertion failed: %T", c.State)
	}
	if got.value != 42 {
		t.Errorf("State.value = %d, want 42", got.value)
	}
}

// TestExecuteWith_HelpCapturedToBuffer verifies the help-output sites
// in Execute (-help, subcommand -help, no-handler fallback, etc.) all
// route through the supplied Context's Stdout. Pre-Iter-3 they wrote
// to os.Stdout via fmt.Println — embeddable now.
func TestExecuteWith_HelpCapturedToBuffer(t *testing.T) {
	cmd := NewCommand("svc").
		Description("test service").
		Subcommand("status").
		Description("show status").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	var buf bytes.Buffer
	base := (&Context{}).SetStdout(&buf)

	if err := cmd.ExecuteWith([]string{"-help"}, base); err != nil {
		t.Fatalf("ExecuteWith(-help): %v", err)
	}
	if !strings.Contains(buf.String(), "test service") {
		t.Errorf("help output missing description; got: %q", buf.String())
	}
}

// TestExecuteWith_SubcommandHelp asserts that a subcommand's -help flag
// also routes through the supplied Stdout.
func TestExecuteWith_SubcommandHelp(t *testing.T) {
	cmd := NewCommand("svc").
		Subcommand("status").
		Description("inspect things").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	var buf bytes.Buffer
	base := (&Context{}).SetStdout(&buf)

	if err := cmd.ExecuteWith([]string{"status", "-help"}, base); err != nil {
		t.Fatalf("ExecuteWith(status -help): %v", err)
	}
	if !strings.Contains(buf.String(), "inspect things") {
		t.Errorf("subcommand help missing; got: %q", buf.String())
	}
}

// TestExecuteWith_HandlerInheritsStateAndIO verifies that a handler
// receiving a parsed *Context sees the IO+State the caller supplied
// via the base Context — i.e. handlers operate in the embedded
// caller's environment, not os.*.
func TestExecuteWith_HandlerInheritsStateAndIO(t *testing.T) {
	type myState struct{ value int }
	var captured *Context
	cmd := NewCommand("svc").
		Subcommand("ping").
		Handler(func(ctx *Context) error {
			captured = ctx
			fmt.Fprintln(ctx.Stdout(), "pong")
			return nil
		}).
		Done().
		Build()

	state := &myState{value: 7}
	var buf bytes.Buffer
	base := (&Context{State: state}).SetStdout(&buf)

	if err := cmd.ExecuteWith([]string{"ping"}, base); err != nil {
		t.Fatalf("ExecuteWith(ping): %v", err)
	}
	if buf.String() != "pong\n" {
		t.Errorf("captured stdout = %q, want %q", buf.String(), "pong\n")
	}
	if captured == nil || captured.State != state {
		t.Errorf("handler did not see base.State (got %v)", captured)
	}
}

// TestExecute_BackwardsCompat asserts the unchanged Execute(args)
// surface still works — the bash-CLI entrypoint is unaffected.
func TestExecute_BackwardsCompat(t *testing.T) {
	called := false
	cmd := NewCommand("svc").
		Subcommand("ping").
		Handler(func(ctx *Context) error {
			called = true
			return nil
		}).
		Done().
		Build()

	if err := cmd.Execute([]string{"ping"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Error("handler not invoked via Execute")
	}
}

// TestExecuteWith_HandlerObservesCtxCancellation models a long-running
// service handler that respects Ctx() cancellation — the embedded
// caller (e.g. an SSH shell on Ctrl-C) signals shutdown via the
// supplied context.
func TestExecuteWith_HandlerObservesCtxCancellation(t *testing.T) {
	cmd := NewCommand("svc").
		Subcommand("wait").
		Handler(func(ctx *Context) error {
			// Pretend to do work; bail when our caller cancels.
			<-ctx.Ctx().Done()
			fmt.Fprintln(ctx.Stdout(), "cancelled")
			return nil
		}).
		Done().
		Build()

	parent, cancel := context.WithCancel(context.Background())
	var buf bytes.Buffer
	base := (&Context{}).SetStdout(&buf).SetCtx(parent)

	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteWith([]string{"wait"}, base) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handler returned %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("handler did not return after cancellation")
	}
	if buf.String() != "cancelled\n" {
		t.Errorf("captured = %q, want %q", buf.String(), "cancelled\n")
	}
}

// TestEmbeddedScenario_CompleteThenExecute simulates one tick of an
// embedded-shell loop: take a partial line, ask Complete() for
// suggestions, accept the first one, hand the full line to
// ExecuteWith. The same Command instance and (importantly) the same
// State pointer survive both calls — the whole point of an in-process
// shell.
func TestEmbeddedScenario_CompleteThenExecute(t *testing.T) {
	type counter struct{ n int }
	state := &counter{}

	cmd := NewCommand("svc").
		Subcommand("inc").
		Handler(func(ctx *Context) error {
			s := ctx.State.(*counter)
			s.n++
			fmt.Fprintf(ctx.Stdout(), "n=%d\n", s.n)
			return nil
		}).
		Done().
		Subcommand("incremental-other").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	// Step 1: completion for `inc<TAB>` returns at least "inc".
	completions, err := cmd.Complete([]string{"inc"}, 1)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(completions) == 0 {
		t.Fatal("expected suggestions for 'inc', got none")
	}
	// Take exact match if present, else first suggestion.
	chosen := completions[0]
	for _, c := range completions {
		if c == "inc" {
			chosen = c
			break
		}
	}

	// Step 2: execute the chosen line through the same Command, same State.
	var buf bytes.Buffer
	base := (&Context{State: state}).SetStdout(&buf)
	for i := 0; i < 3; i++ {
		if err := cmd.ExecuteWith([]string{chosen}, base); err != nil {
			t.Fatalf("ExecuteWith iteration %d: %v", i, err)
		}
	}

	if state.n != 3 {
		t.Errorf("state.n = %d, want 3 (state must persist across invocations)", state.n)
	}
	if !strings.Contains(buf.String(), "n=1") || !strings.Contains(buf.String(), "n=3") {
		t.Errorf("output missing incremental counts; got: %q", buf.String())
	}
}
