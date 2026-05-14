package shell

import (
	"reflect"
	"testing"

	cf "github.com/rosscartlidge/autocli/v4"
)

// TestAutocliCompleter_TrailingSpace asserts that the cursor sitting
// immediately AFTER a word boundary triggers completion of the NEXT
// word, not re-completion of the previous one.
//
// Before the v0.1.3 fix, `to <TAB>` produced `[to]` because the
// completer told autocli "user is on word #1 and typed 'to'" rather
// than "user is on word #2 and typed ''". With this fix, autocli
// sees the empty word and offers child subcommands (e.g. `table`).
func TestAutocliCompleter_TrailingSpace(t *testing.T) {
	cli := cf.NewCommand("svc").
		Subcommand("to").
		Subcommand("table").
		Handler(func(ctx *cf.Context) error { return nil }).
		Done().
		Done().
		Build()

	comp := &autocliCompleter{cli: cli}
	suggestions, _ := comp.Do([]rune("to "), 3)

	// Each suggestion is the SUFFIX to append (partial is empty here
	// since the cursor is after the space), so we look for "table".
	found := false
	for _, s := range suggestions {
		if string(s) == "table" {
			found = true
			break
		}
	}
	if !found {
		var strs []string
		for _, s := range suggestions {
			strs = append(strs, string(s))
		}
		t.Errorf("`to <TAB>` did not suggest `table`; got %v", strs)
	}

	// And confirm without the trailing space, `to` still completes
	// itself (the previous behaviour).
	suggestions, _ = comp.Do([]rune("t"), 1)
	foundTo := false
	for _, s := range suggestions {
		// "o" is the suffix for "to" given partial "t".
		if string(s) == "o" {
			foundTo = true
		}
	}
	if !foundTo {
		t.Errorf("`t<TAB>` did not suggest `to` (suffix `o`); regression in non-trailing-space path")
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
