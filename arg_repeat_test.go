package completionflags

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// A repeating last Arg (FlagSpec.ArgRepeat): the command decides, from the
// fixed args typed before it, how many values follow. These tests pin the
// parser (exact count, too few, callback refusal, a clause separator and a
// following flag after the last value, accumulation), completion (the last
// Arg's completer for every value within the count, flag names after it),
// help rendering and the spec.

// macroArity is the shape ssql uses: NAME is the fixed arg before the
// repeat; the count comes from a table.
func macroArity(prev []string) (int, error) {
	table := map[string]int{"add3": 3, "neg": 1, "none": 0}
	name := prev[len(prev)-1]
	n, ok := table[name]
	if !ok {
		return 0, fmt.Errorf("unknown macro %s; defined: add3, neg, none", name)
	}
	return n, nil
}

func repeatCmd(got *[]Clause) *Command {
	return NewCommand("test").
		Flag("-set-macro").
		Arg("field").Done().
		Arg("macro").Completer(&StaticCompleter{Options: []string{"add3", "neg", "none"}}).Done().
		Arg("args").Completer(&StaticCompleter{Options: []string{"f1", "f2", "f3"}}).Repeat(macroArity).Done().
		Accumulate().Local().Done().
		// Only a repeating arg: `-pick N v1..vN`.
		Flag("-pick").
		Arg("n").Done().
		Arg("v").Type(ArgInt).Repeat(func(prev []string) (int, error) { return strconv.Atoi(prev[0]) }).Done().
		Local().Done().
		Flag("-if").Arg("FIELD").Done().Arg("OP").Done().Arg("VALUE").Done().Local().Done().
		Flag("-desc").Bool().Local().Done().
		Handler(func(ctx *Context) error {
			*got = append([]Clause{}, ctx.Clauses...)
			return nil
		}).
		Build()
}

func TestArgRepeatParse(t *testing.T) {
	macro := func(field, name string, args ...string) map[string]interface{} {
		vals := make([]interface{}, 0, len(args))
		for _, a := range args {
			vals = append(vals, a)
		}
		return map[string]interface{}{"field": field, "macro": name, "args": vals}
	}
	tests := []struct {
		name    string
		args    []string
		want    []map[string]interface{} // the -set-macro entries of clause 0
		pick    map[string]interface{}   // the -pick entry of clause 0, if any
		clauses int
		desc    bool // -desc seen in the LAST clause
		wantErr string
	}{
		{
			name: "exact count",
			args: []string{"-set-macro", "s3", "add3", "a", "b", "c"},
			want: []map[string]interface{}{macro("s3", "add3", "a", "b", "c")},
		},
		{
			name: "a following flag is not swallowed",
			args: []string{"-set-macro", "s3", "add3", "a", "b", "c", "-desc"},
			want: []map[string]interface{}{macro("s3", "add3", "a", "b", "c")},
			desc: true,
		},
		{
			name:    "a clause separator after the last value starts a clause",
			args:    []string{"-set-macro", "n", "neg", "a", "+", "-desc"},
			want:    []map[string]interface{}{macro("n", "neg", "a")},
			clauses: 2,
			desc:    true,
		},
		{
			name: "two calls accumulate in order",
			args: []string{"-set-macro", "n", "neg", "a", "-set-macro", "s3", "add3", "x", "y", "z"},
			want: []map[string]interface{}{macro("n", "neg", "a"), macro("s3", "add3", "x", "y", "z")},
		},
		{
			name: "zero values is a legal count",
			args: []string{"-set-macro", "k", "none", "-desc"},
			want: []map[string]interface{}{macro("k", "none")},
			desc: true,
		},
		{
			name:    "too few values",
			args:    []string{"-set-macro", "s3", "add3", "a", "b"},
			wantErr: "requires 3 value(s) for args, got 2",
		},
		{
			// Values are bound by count and never read as syntax (the
			// -arg rule): the third value IS "-desc", and no -desc flag.
			name: "a value that looks like a flag is a value",
			args: []string{"-set-macro", "s3", "add3", "a", "b", "-desc"},
			want: []map[string]interface{}{macro("s3", "add3", "a", "b", "-desc")},
		},
		{
			name:    "callback refuses the name",
			args:    []string{"-set-macro", "s3", "add4", "a", "b", "c"},
			wantErr: "unknown macro add4; defined: add3, neg, none",
		},
		{
			name:    "fixed args missing",
			args:    []string{"-set-macro", "s3"},
			wantErr: "requires 2 argument(s) before args",
		},
		{
			name: "repeat-only flag, typed values",
			args: []string{"-pick", "2", "10", "20", "-desc"},
			pick: map[string]interface{}{"n": "2", "v": []interface{}{10, 20}},
			desc: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []Clause
			cmd := repeatCmd(&got)
			err := cmd.Execute(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			if tt.clauses == 0 {
				tt.clauses = 1
			}
			if len(got) != tt.clauses {
				t.Fatalf("clauses = %d, want %d", len(got), tt.clauses)
			}
			if tt.want != nil {
				raw, _ := got[0].Flags["-set-macro"].([]interface{})
				var have []map[string]interface{}
				for _, r := range raw {
					have = append(have, r.(map[string]interface{}))
				}
				if !reflect.DeepEqual(have, tt.want) {
					t.Errorf("-set-macro = %#v, want %#v", have, tt.want)
				}
			}
			if tt.pick != nil {
				if have := got[0].Flags["-pick"]; !reflect.DeepEqual(have, tt.pick) {
					t.Errorf("-pick = %#v, want %#v", have, tt.pick)
				}
			}
			last := got[len(got)-1]
			if _, ok := last.Flags["-desc"]; ok != tt.desc {
				t.Errorf("-desc in last clause = %v, want %v", ok, tt.desc)
			}
		})
	}
}

func TestArgRepeatCompletion(t *testing.T) {
	var got []Clause
	cmd := repeatCmd(&got)
	complete := func(args ...string) []string {
		out, err := cmd.Complete(args, len(args))
		if err != nil {
			t.Fatalf("Complete(%q): %v", args, err)
		}
		sort.Strings(out)
		return out
	}
	fields := []string{"f1", "f2", "f3"}
	// NAME completes from its own completer.
	if got := complete("-set-macro", "s3", ""); !reflect.DeepEqual(got, []string{"add3", "neg", "none"}) {
		t.Errorf("macro names = %v", got)
	}
	// Values 1..3 of add3 complete as the repeated Arg.
	for i, args := range [][]string{
		{"-set-macro", "s3", "add3", ""},
		{"-set-macro", "s3", "add3", "a", ""},
		{"-set-macro", "s3", "add3", "a", "b", ""},
	} {
		if got := complete(args...); !reflect.DeepEqual(got, fields) {
			t.Errorf("value %d: completions = %v, want %v", i+1, got, fields)
		}
	}
	// After the third, the flag's arguments are done: fields are not offered.
	after := complete("-set-macro", "s3", "add3", "a", "b", "c", "")
	for _, f := range after {
		if f == "f1" {
			t.Errorf("value 4 of add3 still offers fields: %v", after)
		}
	}
	// A flag prefix after the last value offers flag names.
	if got := complete("-set-macro", "s3", "add3", "a", "b", "c", "-d"); !reflect.DeepEqual(got, []string{"-desc"}) {
		t.Errorf("flag after values = %v", got)
	}
	// neg takes one value: the second position is not a field.
	second := complete("-set-macro", "n", "neg", "a", "")
	for _, f := range second {
		if f == "f1" {
			t.Errorf("value 2 of neg still offers fields: %v", second)
		}
	}
	// An unknown name: no field completions, no crash.
	unknown := complete("-set-macro", "s3", "add4", "")
	for _, f := range unknown {
		if f == "f1" {
			t.Errorf("unknown macro offers fields: %v", unknown)
		}
	}
}

func TestArgRepeatHelpAndSpec(t *testing.T) {
	var got []Clause
	cmd := repeatCmd(&got)
	help := cmd.GenerateHelp()
	if !strings.Contains(help, "-set-macro field macro args...") {
		t.Errorf("help lacks the repeated arg marker:\n%s", help)
	}
	if !strings.Contains(help, "-pick n v...") {
		t.Errorf("help lacks the repeat-only marker:\n%s", help)
	}
	var setMacro *SpecFlag
	for i := range cmd.Spec().Flags {
		if cmd.Spec().Flags[i].Names[0] == "-set-macro" {
			setMacro = &cmd.Spec().Flags[i]
		}
	}
	if setMacro == nil {
		t.Fatal("spec lacks -set-macro")
	}
	if n := len(setMacro.Args); n != 3 {
		t.Fatalf("spec args = %d, want 3", n)
	}
	if setMacro.Args[1].Repeat || !setMacro.Args[2].Repeat {
		t.Errorf("spec repeat markers = %v %v, want false true", setMacro.Args[1].Repeat, setMacro.Args[2].Repeat)
	}
	// Help-at on a value past the fixed arity marks the repeated Arg.
	text, err := cmd.HelpAt([]string{"-set-macro", "s3", "add3", "a", "b"}, 5)
	if err != nil {
		t.Fatalf("HelpAt: %v", err)
	}
	if !strings.Contains(text, "→ args...") {
		t.Errorf("help-at does not mark the repeated arg:\n%s", text)
	}
}

func TestArgRepeatBuilderMisuse(t *testing.T) {
	mustPanic := func(name string, f func()) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("%s: no panic", name)
			}
		}()
		f()
	}
	mustPanic("Arg after Repeat", func() {
		NewCommand("x").Flag("-m").Arg("a").Repeat(macroArity).Done().Arg("b").Done().Done()
	})
	mustPanic("Repeat on a non-last Arg", func() {
		fb := NewCommand("x").Flag("-m").Arg("a").Done().Arg("b").Done()
		fb.Arg("c").Done()
		// Reach back to the first Arg: the builder for index 0 is not the last.
		(&ArgBuilder{fb: fb, argIndex: 0}).Repeat(macroArity)
	})
}
