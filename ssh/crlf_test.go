package ssh

import (
	"bytes"
	"testing"
)

func TestCrlfWriter(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"no newline", "hello", "hello"},
		{"bare newline", "a\nb\n", "a\r\nb\r\n"},
		{"already crlf preserved", "a\r\nb\r\n", "a\r\nb\r\n"},
		{"mixed", "a\nb\r\nc\n", "a\r\nb\r\nc\r\n"},
		{"just lf at end", "x\n", "x\r\n"},
		{"empty", "", ""},
		{"multiple lf in a row", "a\n\nb\n", "a\r\n\r\nb\r\n"},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		w := crlfWriter{w: &buf}
		n, err := w.Write([]byte(tc.in))
		if err != nil {
			t.Errorf("%s: unexpected err: %v", tc.name, err)
			continue
		}
		if n != len(tc.in) {
			t.Errorf("%s: wrote %d, want input len %d", tc.name, n, len(tc.in))
		}
		if got := buf.String(); got != tc.want {
			t.Errorf("%s: out = %q, want %q", tc.name, got, tc.want)
		}
	}
}
