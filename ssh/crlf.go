package ssh

import "io"

// crlfWriter wraps an io.Writer (an SSH channel) and translates bare
// `\n` to `\r\n` so output renders correctly on the client's terminal.
//
// Why this is needed: an SSH session with a pty-req puts the CLIENT's
// terminal in raw mode (no kernel-side line discipline). Bytes the
// server writes are placed on the screen verbatim — a `\n` advances
// the cursor one row down without returning to column 0, producing
// staircase output. A real sshd avoids this because the user's app
// (bash, vim, …) emits `\r\n` itself, or the user runs a shell that
// has its own line discipline. autocli/shell handlers write plain
// `\n` via stdlib `fmt.Println` / `fmt.Fprintln`, so we translate at
// the channel boundary.
//
// `\r\n` that's already in the stream is preserved (we only translate
// `\n` that isn't preceded by `\r`).
type crlfWriter struct {
	w io.Writer
}

func (c crlfWriter) Write(p []byte) (int, error) {
	// Walk the buffer; emit segments delimited by translated newlines.
	// Tracks how many input bytes the caller logically wrote so Write
	// returns (len(p), nil) on success — preserves the io.Writer
	// contract even though we sometimes emit more bytes downstream.
	consumed := 0
	start := 0
	var prev byte
	for i, b := range p {
		if b == '\n' && prev != '\r' {
			// Flush [start:i], then write \r\n in place of \n.
			if i > start {
				n, err := c.w.Write(p[start:i])
				consumed += n
				if err != nil {
					return consumed, err
				}
			}
			if _, err := c.w.Write([]byte{'\r', '\n'}); err != nil {
				return consumed, err
			}
			consumed++ // logical \n consumed; the extra \r is invisible to caller
			start = i + 1
		}
		prev = b
	}
	if start < len(p) {
		n, err := c.w.Write(p[start:])
		consumed += n
		if err != nil {
			return consumed, err
		}
	}
	return consumed, nil
}
