package completionflags

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// -arg VALUE is the flag form of a positional: VALUE is bound by arity and
// never read as syntax. These tests pin that for every shape a hostile
// value can take (a real flag of the command, a separator, "--", "-arg"
// itself, a subcommand name), and pin the hole it closes so nobody
// "simplifies" the bare form into looking safe.

// argFlagCmd has everything a hostile positional could collide with: a bool
// flag, a multi-argument flag, clause separators and a variadic positional.
func argFlagCmd(got *[][]string, generate *bool, conds *[]string) *Command {
	return NewCommand("test").
		Flag("FIELDS").StringSlice().Variadic().Local().Done().
		Flag("-generate").Bool().Global().Done().
		Flag("-desc").Bool().Local().Done().
		Flag("-if").Arg("FIELD").Done().Arg("OP").Done().Arg("VALUE").Done().Local().Done().
		Handler(func(ctx *Context) error {
			*generate = ctx.GetBool("-generate", false)
			for _, c := range ctx.Clauses {
				*got = append(*got, append([]string{}, c.Positional...))
				if v, ok := c.Flags["-if"]; ok {
					// A multi-argument flag is stored as a map keyed by
					// argument name; the values are what matters here.
					for _, x := range v.(map[string]interface{}) {
						*conds = append(*conds, x.(string))
					}
					sort.Strings(*conds)
				}
			}
			return nil
		}).
		Build()
}

func TestArgFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		want     [][]string
		generate bool
		conds    []string
	}{
		{
			// The hole (DFC134 §3.2): a bare positional that looks like a
			// flag IS a flag. Kept as a case so the contrast stays visible.
			name:     "bare positional is read as a flag",
			args:     []string{"name", "-generate"},
			want:     [][]string{{"name"}},
			generate: true,
		},
		{
			name: "the same value through -arg is data",
			args: []string{"-arg", "name", "-arg", "-generate"},
			want: [][]string{{"name", "-generate"}},
		},
		{
			name: "separators, --, and -arg itself are data",
			args: []string{"-arg", "+", "-arg", "-", "-arg", "--", "-arg", "-arg", "-arg", "+arg"},
			want: [][]string{{"+", "-", "--", "-arg", "+arg"}},
		},
		{
			name: "bare and -arg mix, order of appearance kept",
			args: []string{"a", "-arg", "-b", "c", "-arg", "d"},
			want: [][]string{{"a", "-b", "c", "d"}},
		},
		{
			name: "clauses still separate; the real flag still works",
			args: []string{"-arg", "dept", "-", "-arg", "-desc", "-desc"},
			want: [][]string{{"dept"}, {"-desc"}},
		},
		{
			name:  "-arg as the argument of another flag is that flag's data",
			args:  []string{"-if", "name", "eq", "-arg", "x"},
			want:  [][]string{{"x"}},
			conds: []string{"-arg", "eq", "name"},
		},
		{
			name: "empty string and whitespace survive",
			args: []string{"-arg", "", "-arg", " a b "},
			want: [][]string{{"", " a b "}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got [][]string
			var generate bool
			var conds []string
			if err := argFlagCmd(&got, &generate, &conds).Execute(tt.args); err != nil {
				t.Fatalf("Execute(%q): %v", tt.args, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("positionals = %q, want %q", got, tt.want)
			}
			if generate != tt.generate {
				t.Errorf("-generate = %v, want %v", generate, tt.generate)
			}
			if !reflect.DeepEqual(conds, tt.conds) {
				t.Errorf("-if = %q, want %q", conds, tt.conds)
			}
		})
	}
}

// -arg values reach declared positional specs exactly as bare ones do,
// including typed ones, and a "--" later on the line keeps its meaning.
func TestArgFlagFillsDeclaredPositionals(t *testing.T) {
	var file string
	var n int
	var rest []string
	cmd := NewCommand("test").
		Flag("FILE").String().Global().Done().
		Flag("N").Int().Global().Done().
		Handler(func(ctx *Context) error {
			file = ctx.GetString("FILE", "")
			n = ctx.GetInt("N", 0)
			rest = ctx.RemainingArgs
			return nil
		}).
		Build()

	if err := cmd.Execute([]string{"-arg", "-weird.csv", "-arg", "-7", "--", "-arg", "x"}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if file != "-weird.csv" || n != -7 {
		t.Errorf("FILE=%q N=%d, want -weird.csv and -7", file, n)
	}
	if want := []string{"-arg", "x"}; !reflect.DeepEqual(rest, want) {
		t.Errorf("RemainingArgs = %q, want %q", rest, want)
	}

	err := cmd.Execute([]string{"-arg", "f.csv", "-arg", "seven"})
	if err == nil || !strings.Contains(err.Error(), "seven") {
		t.Errorf("a non-integer N through -arg must fail naming the value, got %v", err)
	}
}

// A positional equal to a subcommand's name is data when carried by -arg;
// bare, it would select the subcommand.
func TestArgFlagValueIsNotASubcommand(t *testing.T) {
	var where, file string
	cmd := NewCommand("test").
		Subcommand("from").
			Flag("FILE").String().Global().Done().
			Handler(func(ctx *Context) error {
				where, file = "from", ctx.GetString("FILE", "")
				return nil
			}).
			Subcommand("csv").
				Flag("FILE").String().Global().Done().
				Handler(func(ctx *Context) error {
					where, file = "from csv", ctx.GetString("FILE", "")
					return nil
				}).
				Done().
			Done().
		Build()

	if err := cmd.Execute([]string{"from", "csv", "-arg", "csv"}); err != nil {
		t.Fatal(err)
	}
	if where != "from csv" || file != "csv" {
		t.Errorf("got %q FILE=%q, want from csv / csv", where, file)
	}
	if err := cmd.Execute([]string{"from", "-arg", "csv"}); err != nil {
		t.Fatal(err)
	}
	if where != "from" || file != "csv" {
		t.Errorf("got %q FILE=%q, want from / csv", where, file)
	}
}

func TestArgFlagErrors(t *testing.T) {
	var got [][]string
	var generate bool
	var conds []string
	cmd := argFlagCmd(&got, &generate, &conds)

	if err := cmd.Execute([]string{"name", "-arg"}); err == nil || !strings.Contains(err.Error(), "-arg") {
		t.Errorf("a trailing -arg must fail naming the flag, got %v", err)
	}
	if err := cmd.Execute([]string{"+arg", "name"}); err == nil || !strings.Contains(err.Error(), "+arg") {
		t.Errorf("+arg must be rejected, got %v", err)
	}
}

func TestArgFlagNameIsReserved(t *testing.T) {
	for _, name := range []string{"-arg", "+arg"} {
		t.Run("root "+name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil || !strings.Contains(r.(string), "reserved") {
					t.Errorf("declaring %s must panic as reserved, got %v", name, r)
				}
			}()
			NewCommand("test").Flag(name)
		})
		t.Run("subcommand "+name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil || !strings.Contains(r.(string), "reserved") {
					t.Errorf("declaring %s must panic as reserved, got %v", name, r)
				}
			}()
			NewCommand("test").Subcommand("s").Flag("X", name)
		})
	}
	// Exact match only: a longer name is an ordinary flag.
	NewCommand("test").Flag("-arg-max").Bool().Global().Done().Handler(func(*Context) error { return nil }).Build()
}

// The value of -arg completes as the positional slot it fills, counted
// with every earlier positional however it was written, and is never
// completed as a flag name even when it starts with a dash.
func TestArgFlagCompletion(t *testing.T) {
	build := func() *Command {
		return NewCommand("test").
			Subcommand("pick").
				Flag("FIRST").String().Global().Options("alpha", "-almost").Done().
				Flag("SECOND").String().Global().Options("beta", "-between").Done().
				Flag("-between-flag").Bool().Global().Done().
				Handler(func(*Context) error { return nil }).
				Done().
			Build()
	}
	tests := []struct {
		name string
		args []string
		pos  int
		want []string
	}{
		{"first slot", []string{"pick", "-arg", ""}, 3, []string{"alpha", "-almost"}},
		{"first slot, dash prefix", []string{"pick", "-arg", "-al"}, 3, []string{"-almost"}},
		{"second slot after a bare first", []string{"pick", "alpha", "-arg", ""}, 4, []string{"beta", "-between"}},
		{"second slot after an -arg first", []string{"pick", "-arg", "-almost", "-arg", "-be"}, 5, []string{"-between"}},
		{"bare second slot after an -arg first", []string{"pick", "-arg", "-almost", "b"}, 4, []string{"beta"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := build().Complete(tt.args, tt.pos)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Complete(%q, %d) = %q, want %q", tt.args, tt.pos, got, tt.want)
			}
		})
	}
}

func TestArgFlagIsAdvertised(t *testing.T) {
	cmd := NewCommand("test").
		Subcommand("sub").
			Flag("INPUT").String().Global().Help("Input file").Done().
			Handler(func(*Context) error { return nil }).
			Done().
		Build()

	var buf bytes.Buffer
	if err := cmd.WriteSpecJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var spec struct {
		PositionalFlag string `json:"positionalFlag"`
	}
	if err := json.Unmarshal(buf.Bytes(), &spec); err != nil {
		t.Fatal(err)
	}
	if spec.PositionalFlag != ArgFlag {
		t.Errorf("spec positionalFlag = %q, want %q", spec.PositionalFlag, ArgFlag)
	}

	flat := NewCommand("flat").
		Flag("FILE").String().Global().Help("Input file").Done().
		Handler(func(*Context) error { return nil }).
		Build()
	if help := flat.GenerateHelp(); !strings.Contains(help, "-arg VALUE") {
		t.Errorf("root help does not mention -arg VALUE:\n%s", help)
	}
	if help := cmd.subcommands["sub"].GenerateHelp("test"); !strings.Contains(help, "-arg VALUE") {
		t.Errorf("subcommand help does not mention -arg VALUE:\n%s", help)
	}
}
