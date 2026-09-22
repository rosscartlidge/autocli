package completionflags

import (
	"errors"
	"strings"
	"testing"
)

// Check is Execute's parse and validation without the handler: what a
// runner asks before starting any stage of a pipeline.
func TestCheck(t *testing.T) {
	ran := false
	cmd := NewCommand("app").
		Flag("-verbose").Bool().Global().Done().
		Subcommand("from").
			Flag("FILE").String().Global().Required().Done().
			Flag("-n").Int().Global().Done().
			Handler(func(*Context) error { ran = true; return nil }).
			Subcommand("csv").
				Flag("FILE").String().Global().Required().Done().
				Handler(func(*Context) error { ran = true; return nil }).
				Done().
			Done().
		Subcommand("group").
			Subcommand("leaf").
				Handler(func(*Context) error { ran = true; return nil }).
				Done().
			Done().
		Build()

	ok := [][]string{
		{"from", "a.csv"}, {"from", "csv", "a.csv"}, {"from", "-arg", "-weird.csv", "-n", "3"},
		{"-verbose", "from", "a.csv"}, {"from", "-verbose", "a.csv"},
		{"group", "leaf"}, {"from", "-help"}, {"-help"}, {"-spec-json"},
	}
	for _, args := range ok {
		if err := cmd.Check(args); err != nil {
			t.Errorf("Check(%q): %v", args, err)
		}
	}
	bad := map[string][]string{
		"unknown command":   {"nope", "x"},
		"unknown flag":      {"from", "a.csv", "-bogus"},
		"missing required":  {"from", "-n", "3"},
		"flag needs value":  {"from", "a.csv", "-n"},
		"not an int":        {"from", "a.csv", "-n", "three"},
		"intermediate node": {"group"},
	}
	for name, args := range bad {
		if err := cmd.Check(args); err == nil {
			t.Errorf("%s: Check(%q) accepted", name, args)
		}
	}
	if err := cmd.Check([]string{"nope"}); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("unknown command error must name it: %v", err)
	}
	if ran {
		t.Error("Check ran a handler")
	}

	// A root WITH a handler but no positionals of its own, and
	// subcommands: a bare unknown word is still an unknown command, for
	// Check and for Execute alike.
	banner := NewCommand("app").
		Subcommand("from").Flag("FILE").String().Global().Done().Handler(func(*Context) error { return nil }).Done().
		Handler(func(*Context) error { ran = true; return nil }).
		Build()
	if err := banner.Check([]string{"frob"}); err == nil {
		t.Error("Check accepted an unknown word on a root with a handler")
	}
	var unknown ErrUnknownCommand
	if err := banner.Execute([]string{"frob"}); !errors.As(err, &unknown) {
		t.Errorf("Execute: want ErrUnknownCommand, got %v", err)
	}
	if err := banner.Execute(nil); err != nil || !ran {
		t.Errorf("bare root must still run the root handler: %v ran=%v", err, ran)
	}
}
