package completionflags

import (
	"strings"
	"testing"
)

// StrictPositionals: an argument no positional spec consumes is a parse
// error, in subcommands with no positionals, with N positionals, and
// at the root; a Variadic positional takes them all; off, the leftovers
// stay in the clause (the historical behaviour).
func TestStrictPositionals(t *testing.T) {
	build := func(strict bool) *Command {
		cb := NewCommand("tool").Description("t")
		if strict {
			cb = cb.StrictPositionals()
		}
		cb.Subcommand("count").Description("no positionals").Handler(func(*Context) error { return nil }).Done()
		cb.Subcommand("one").Description("one positional").
			Flag("FIELD").String().Required().Help("f").Done().
			Handler(func(*Context) error { return nil }).Done()
		cb.Subcommand("many").Description("variadic").
			Flag("FIELDS").String().Variadic().Help("f").Done().
			Handler(func(*Context) error { return nil }).Done()
		return cb.Build()
	}
	strict := build(true)
	cases := []struct {
		args []string
		want string // "" = parses
	}{
		{[]string{"count"}, ""},
		{[]string{"count", "foo"}, `unexpected argument "foo": count takes no positional arguments`},
		{[]string{"one", "a"}, ""},
		{[]string{"one", "a", "b"}, `unexpected argument "b": one takes no further positional arguments`},
		{[]string{"many", "a", "b", "c"}, ""},
	}
	for _, c := range cases {
		err := strict.Execute(c.args)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%v: unexpected error %v", c.args, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), c.want)):
			t.Errorf("%v: got %v, want %q", c.args, err, c.want)
		}
	}
	// Off: the stray argument is kept in the clause, not refused.
	lax := build(false)
	ctx, err := lax.parseSubcommand(lax.subcommands["count"], nil, []string{"foo"})
	if err != nil {
		t.Fatalf("lax: %v", err)
	}
	kept := false
	for _, c := range ctx.Clauses {
		for _, p := range c.Positional {
			kept = kept || p == "foo"
		}
	}
	if !kept {
		t.Errorf("lax: stray positional should stay in the clause, got %+v", ctx.Clauses)
	}
}
