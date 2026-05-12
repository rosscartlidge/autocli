package shell

import (
	"reflect"
	"testing"
)

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
