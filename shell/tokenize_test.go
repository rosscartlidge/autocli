package shell

import (
	"reflect"
	"strings"
	"testing"

	cf "github.com/rosscartlidge/autocli/v4"
)

// TestTabComplete_TrailingSpace asserts that the cursor sitting
// immediately AFTER a word boundary triggers completion of the NEXT
// word, not re-completion of the previous one.
//
// Single-match case: `to ` should auto-insert `table ` (line becomes
// "to table " with cursor after the trailing space). Multi-match
// would print a list and return ok=false; we set up exactly one
// match to keep this deterministic.
func TestTabComplete_TrailingSpace(t *testing.T) {
	cli := cf.NewCommand("svc").
		Subcommand("to").
		Subcommand("table").
		Handler(func(ctx *cf.Context) error { return nil }).
		Done().
		Done().
		Build()

	var listSink, termSink strings.Builder
	newLine, newPos, ok := tabComplete(cli, "to ", 3, &termSink, &listSink)
	if !ok {
		t.Fatalf("`to <TAB>` did not produce an insertion (multi-match list: %q)", listSink.String())
	}
	if newLine != "to table " {
		t.Errorf("newLine = %q, want %q", newLine, "to table ")
	}
	if newPos != len(newLine) {
		t.Errorf("newPos = %d, want %d (end of line)", newPos, len(newLine))
	}

	// And confirm without the trailing space, the in-progress word
	// `t` still completes to `to` (insertion path, single match).
	listSink.Reset()
	termSink.Reset()
	newLine, _, ok = tabComplete(cli, "t", 1, &termSink, &listSink)
	if !ok {
		t.Fatalf("`t<TAB>` did not produce an insertion")
	}
	if !strings.HasPrefix(newLine, "to") {
		t.Errorf("newLine = %q, want prefix `to`", newLine)
	}
}

func TestTokenize(t *testing.T) {
	cases := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{"", []string{}, false},
		{"  ", []string{}, false},
		{"a", []string{"a"}, false},
		{"a b c", []string{"a", "b", "c"}, false},
		{"  a   b  ", []string{"a", "b"}, false},

		// Single quotes — literal, no escapes.
		{`'hello world'`, []string{"hello world"}, false},
		{`a 'b c' d`, []string{"a", "b c", "d"}, false},
		{`'a\nb'`, []string{`a\nb`}, false}, // backslash literal inside single quotes

		// Double quotes — escapes for \" \\ \$ \`
		{`"hello world"`, []string{"hello world"}, false},
		{`"a\"b"`, []string{`a"b`}, false},
		{`"a\\b"`, []string{`a\b`}, false},
		{`"a\$b"`, []string{"a$b"}, false},
		{`"a\nb"`, []string{`a\nb`}, false}, // \n not recognised — kept literal

		// Backslash escapes outside quotes.
		{`a\ b`, []string{"a b"}, false},
		{`a\\b`, []string{`a\b`}, false},

		// Mixed.
		{`echo "hello, $world" 'literal'`, []string{"echo", "hello, $world", "literal"}, false},
		{`-if 'salary / 10'`, []string{"-if", "salary / 10"}, false},

		// Errors.
		{`'unclosed`, nil, true},
		{`"unclosed`, nil, true},
	}
	for _, tc := range cases {
		got, err := Tokenize(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Tokenize(%q): expected error, got %v", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Tokenize(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			// Treat both nil and empty-slice as equivalent for the empty cases.
			if !(len(got) == 0 && len(tc.want) == 0) {
				t.Errorf("Tokenize(%q) = %#v, want %#v", tc.in, got, tc.want)
			}
		}
	}
}
