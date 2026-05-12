package shell

import (
	"fmt"
	"strings"
)

// Tokenize splits a shell-style command line into argv-style tokens.
// Supports:
//   - whitespace separation (spaces and tabs)
//   - single-quoted strings (literal, no escapes)
//   - double-quoted strings (backslash escapes \" \\ \$)
//   - backslash escapes outside quotes (\<space> = literal space)
//
// Returns an error for unterminated quotes. Matches bash's
// word-splitting closely enough that quoting rules transfer between
// the bash-CLI path and the embedded shell.
func Tokenize(line string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inToken := false

	flush := func() {
		if inToken {
			tokens = append(tokens, cur.String())
			cur.Reset()
			inToken = false
		}
	}

	i := 0
	for i < len(line) {
		c := line[i]
		switch {
		case c == ' ' || c == '\t':
			flush()
			i++
		case c == '\'':
			// Single-quoted: literal until next '.
			end := strings.IndexByte(line[i+1:], '\'')
			if end == -1 {
				return nil, fmt.Errorf("unterminated single quote")
			}
			cur.WriteString(line[i+1 : i+1+end])
			inToken = true
			i += 1 + end + 1
		case c == '"':
			// Double-quoted: escapes for \" \\ \$ \`; everything else literal.
			j := i + 1
			for j < len(line) && line[j] != '"' {
				if line[j] == '\\' && j+1 < len(line) {
					next := line[j+1]
					if next == '"' || next == '\\' || next == '$' || next == '`' {
						cur.WriteByte(next)
						j += 2
						continue
					}
				}
				cur.WriteByte(line[j])
				j++
			}
			if j >= len(line) {
				return nil, fmt.Errorf("unterminated double quote")
			}
			inToken = true
			i = j + 1
		case c == '\\':
			// Outside quotes: backslash escapes the next character.
			if i+1 < len(line) {
				cur.WriteByte(line[i+1])
				inToken = true
				i += 2
			} else {
				// Trailing backslash — treat as literal.
				cur.WriteByte('\\')
				inToken = true
				i++
			}
		default:
			cur.WriteByte(c)
			inToken = true
			i++
		}
	}
	flush()
	return tokens, nil
}
