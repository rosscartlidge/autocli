package completionflags

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestGenerateHelpEmbedded asserts the embedded variant strips the
// bash-completion footer and the `-man` reference, while preserving
// the COMMANDS / USAGE sections.
func TestGenerateHelpEmbedded(t *testing.T) {
	cmd := NewCommand("demo").
		Description("test demo").
		Subcommand("ping").
		Description("respond pong").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	bash := cmd.GenerateHelp()
	emb := cmd.GenerateHelpEmbedded()

	for _, banned := range []string{
		"SHELL COMPLETION",
		"-completion-script",
		"-man",
	} {
		if strings.Contains(emb, banned) {
			t.Errorf("embedded help contains banned text %q\nhelp:\n%s", banned, emb)
		}
		if !strings.Contains(bash, banned) {
			t.Errorf("bash help missing %q (regression)", banned)
		}
	}

	// Both forms should still include the COMMANDS list — that's the
	// whole point of help.
	for _, want := range []string{"COMMANDS:", "ping"} {
		if !strings.Contains(emb, want) {
			t.Errorf("embedded help missing %q", want)
		}
	}
}

// TestErrUnknownCommand_FromExecuteWith asserts ExecuteWith returns
// ErrUnknownCommand for a non-flag input that doesn't match any
// subcommand when the root has no handler. Previously this silently
// fell through to printing help.
func TestErrUnknownCommand_FromExecuteWith(t *testing.T) {
	cmd := NewCommand("demo").
		Subcommand("ping").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	var buf bytes.Buffer
	base := (&Context{}).SetStdout(&buf)

	err := cmd.ExecuteWith([]string{"junk"}, base)
	if err == nil {
		t.Fatal("expected ErrUnknownCommand, got nil")
	}
	var unknown ErrUnknownCommand
	if !errors.As(err, &unknown) {
		t.Fatalf("expected ErrUnknownCommand, got %T: %v", err, err)
	}
	if string(unknown) != "junk" {
		t.Errorf("unknown.Name = %q, want %q", string(unknown), "junk")
	}
	if buf.Len() != 0 {
		t.Errorf("expected no stdout output, got: %q", buf.String())
	}
}

// TestErrUnknownCommand_NotForFlags asserts a leading flag (like
// `-foo`) does NOT trigger ErrUnknownCommand. Unknown top-level
// flags hit the existing parser path (which currently falls through
// to printing help; that's a separate cleanup, not part of this
// change). The important property here is: we don't misclassify a
// flag as an unknown subcommand.
func TestErrUnknownCommand_NotForFlags(t *testing.T) {
	cmd := NewCommand("demo").
		Subcommand("ping").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	var buf bytes.Buffer
	base := (&Context{}).SetStdout(&buf)

	err := cmd.ExecuteWith([]string{"-unknownflag"}, base)
	var unknown ErrUnknownCommand
	if errors.As(err, &unknown) {
		t.Errorf("flag misclassified as ErrUnknownCommand: %v", err)
	}
}

// TestErrUnknownCommand_NotForEmptyArgs asserts empty args still
// shows help (existing behaviour) rather than erroring.
func TestErrUnknownCommand_NotForEmptyArgs(t *testing.T) {
	cmd := NewCommand("demo").
		Subcommand("ping").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	var buf bytes.Buffer
	base := (&Context{}).SetStdout(&buf)

	if err := cmd.ExecuteWith(nil, base); err != nil {
		t.Errorf("empty args should not error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected help on empty args, got empty stdout")
	}
}

// TestErrUnknownCommand_NotForRootHandler asserts a root command with
// a handler accepts unknown positional args and routes to the handler
// (it might consume them as positional input) rather than erroring.
func TestErrUnknownCommand_NotForRootHandler(t *testing.T) {
	called := false
	cmd := NewCommand("demo").
		Handler(func(ctx *Context) error {
			called = true
			return nil
		}).
		Build()

	if err := cmd.ExecuteWith([]string{"random-arg"}, &Context{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("root handler not called")
	}
}

// TestComplete_DashHIncludesHelp asserts that -h at the root of a
// subcommand-having command completes the built-in -help. Previously
// completeRootGlobalFlags walked only user-declared global flags and
// missed the built-ins, even though the leaf-level completer (after
// a subcommand name) included them — typing `myapp -h<TAB>` was
// inconsistent with `myapp sub -h<TAB>`.
func TestComplete_DashHIncludesHelp(t *testing.T) {
	cmd := NewCommand("myapp").
		Subcommand("ping").
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	got, err := cmd.Complete([]string{"-h"}, 1)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	found := map[string]bool{}
	for _, c := range got {
		found[c] = true
	}
	for _, w := range []string{"-help", "-h"} {
		if !found[w] {
			t.Errorf("-h<TAB> missing %q in %v", w, got)
		}
	}
}
