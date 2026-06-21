package completionflags

import (
	"strings"
	"testing"
)

// helpAtTestCmd builds a small subcommand CLI with a documented two-arg
// flag, mirroring the shape of a real ssql stage (`prog group-by FIELD
// -sum FIELD RESULT`). Used by the HelpAt tests below.
func helpAtTestCmd() *Command {
	return NewCommand("prog").
		Subcommand("group-by").
		Description("Group records and aggregate").
		Example("prog group-by dept -sum salary total", "Total salary per department").
		Flag("FIELD").
		String().
		Variadic().
		Help("Fields to group by").
		Done().
		Flag("-sum", "-s").
		Args(2).
		ArgName(0, "FIELD").
		ArgName(1, "RESULT").
		ArgType(1, ArgString).
		Help("Sum field values across each group").
		Local().
		Done().
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()
}

func TestHelpAt_FlagToken(t *testing.T) {
	cmd := helpAtTestCmd()
	// Cursor on the "-sum" token: COMP_WORDS = [prog group-by dept -sum],
	// caret on "-sum" at index 3.
	text, err := cmd.HelpAt([]string{"group-by", "dept", "-sum"}, 3)
	if err != nil {
		t.Fatalf("HelpAt: %v", err)
	}
	for _, want := range []string{"-sum", "-s", "FIELD", "RESULT", "Sum field values"} {
		if !strings.Contains(text, want) {
			t.Errorf("flag-token help missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "USAGE:") {
		t.Errorf("flag-token help should be flag-focused, not full command help:\n%s", text)
	}
}

func TestHelpAt_FlagArgument(t *testing.T) {
	cmd := helpAtTestCmd()
	// Cursor on "total" — the second argument (RESULT) of -sum.
	// COMP_WORDS = [prog group-by dept -sum salary total], caret idx 5.
	text, err := cmd.HelpAt([]string{"group-by", "dept", "-sum", "salary", "total"}, 5)
	if err != nil {
		t.Fatalf("HelpAt: %v", err)
	}
	if !strings.Contains(text, "Sum field values") {
		t.Errorf("flag-argument help should describe -sum:\n%s", text)
	}
	// The RESULT argument (index 1) is under the cursor → marked with →.
	if !strings.Contains(text, "→ RESULT") {
		t.Errorf("expected current-argument marker on RESULT:\n%s", text)
	}
}

func TestHelpAt_CommandLevel(t *testing.T) {
	cmd := helpAtTestCmd()
	// Cursor on a positional ("dept"), not on any flag → full command help.
	text, err := cmd.HelpAt([]string{"group-by", "dept"}, 2)
	if err != nil {
		t.Fatalf("HelpAt: %v", err)
	}
	for _, want := range []string{"group-by", "Group records and aggregate", "USAGE:"} {
		if !strings.Contains(text, want) {
			t.Errorf("command-level help missing %q:\n%s", want, text)
		}
	}
}

func TestHelpAt_RootLevel(t *testing.T) {
	cmd := helpAtTestCmd()
	// Cursor on the subcommand-name word → root help listing commands.
	text, err := cmd.HelpAt([]string{"group-by"}, 1)
	if err != nil {
		t.Fatalf("HelpAt: %v", err)
	}
	if !strings.Contains(text, "COMMANDS:") {
		t.Errorf("root-level help should list COMMANDS:\n%s", text)
	}
}

// TestHelpAt_Protocol exercises the bash `-help-at <pos> <words...>`
// dispatch through ExecuteWith, capturing to a buffer.
func TestHelpAt_Protocol(t *testing.T) {
	cmd := helpAtTestCmd()
	var buf strings.Builder
	base := &Context{}
	base.SetStdout(&buf)
	err := cmd.ExecuteWith([]string{"-help-at", "3", "group-by", "dept", "-sum"}, base)
	if err != nil {
		t.Fatalf("ExecuteWith -help-at: %v", err)
	}
	if !strings.Contains(buf.String(), "Sum field values") {
		t.Errorf("protocol help missing flag description:\n%s", buf.String())
	}
}
