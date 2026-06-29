package completionflags

import (
	"bytes"
	"strings"
	"testing"
)

// dispatcherCmd builds `demo to {table,csv}` — a subcommand that dispatches to
// nested subcommands (like ssql's `to`).
func dispatcherCmd() *Command {
	to := NewCommand("demo").
		Subcommand("to").
		Description("write output in various formats")
	to.Subcommand("table").
		Description("display as a table").
		Handler(func(ctx *Context) error { return nil }).
		Done()
	to.Subcommand("csv").
		Description("write as CSV").
		Handler(func(ctx *Context) error { return nil }).
		Done()
	return to.Done().Build()
}

// TestSubcommandHelpListsNestedSubcommands: a dispatcher subcommand's help must
// list the subcommands it dispatches to (the gap behind `ssql to -help`), with
// a sensible USAGE and no misleading "does not support clauses".
func TestSubcommandHelpListsNestedSubcommands(t *testing.T) {
	cmd := dispatcherCmd()
	var buf bytes.Buffer
	base := &Context{}
	base.SetStdout(&buf)
	if err := cmd.ExecuteWith([]string{"to", "-help"}, base); err != nil {
		t.Fatalf("to -help: %v", err)
	}
	help := buf.String()

	for _, want := range []string{
		"COMMANDS:",
		"table", "display as a table",
		"csv", "write as CSV",
		"demo to <COMMAND>",       // usage shows it dispatches
		"demo to <command> -help", // footer hint
	} {
		if !strings.Contains(help, want) {
			t.Errorf("`to -help` missing %q:\n%s", want, help)
		}
	}
	if strings.Contains(help, "does not support clauses") {
		t.Errorf("dispatcher help should not claim 'does not support clauses':\n%s", help)
	}
}

// TestNestedSubcommandHelpFullPath: `demo to table -help` must render the full
// path ("demo to table"), not drop the intermediate ("demo table").
func TestNestedSubcommandHelpFullPath(t *testing.T) {
	cmd := dispatcherCmd()
	var buf bytes.Buffer
	base := &Context{}
	base.SetStdout(&buf)
	if err := cmd.ExecuteWith([]string{"to", "table", "-help"}, base); err != nil {
		t.Fatalf("to table -help: %v", err)
	}
	help := buf.String()
	if !strings.Contains(help, "demo to table") {
		t.Errorf("nested help should show the full path 'demo to table':\n%s", help)
	}
	if strings.Contains(help, "demo table -") {
		t.Errorf("nested help dropped the intermediate command:\n%s", help)
	}
}
