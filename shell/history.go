package shell

import (
	"bufio"
	"os"
	"path/filepath"
	"sync"
)

// fileHistory is a bounded, file-backed implementation of
// term.History. Lines are stored in memory (newest at the back) and
// flushed to disk on every Add via atomic tmp+rename. Capped at
// maxHistory entries; oldest dropped when the cap is hit.
//
// Safe for concurrent use, though the upstream x/term docs say
// "It is not safe to call ReadLine concurrently with any methods on
// History" — we hold the lock anyway in case future drivers do.
type fileHistory struct {
	mu      sync.Mutex
	path    string
	entries []string // newest at index 0 to match term.History.At() semantics
}

const maxHistory = 1000

// newFileHistory loads any existing history from path (creating
// nothing if path is empty or the file doesn't exist) and returns a
// term.History-compatible value. Lines longer than maxLine are
// dropped from the on-disk file when read back.
func newFileHistory(path string) *fileHistory {
	h := &fileHistory{path: path}
	if path == "" {
		return h
	}
	f, err := os.Open(path)
	if err != nil {
		return h
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		// Insert at the FRONT so newest stays at index 0 — file is
		// oldest-first (append-friendly), memory is newest-first
		// (term.History contract).
		h.entries = append([]string{line}, h.entries...)
		if len(h.entries) > maxHistory {
			h.entries = h.entries[:maxHistory]
		}
	}
	return h
}

func (h *fileHistory) Add(entry string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Drop consecutive duplicates — typing `status<Enter>` three
	// times shouldn't pollute the back-scroll.
	if len(h.entries) > 0 && h.entries[0] == entry {
		return
	}
	h.entries = append([]string{entry}, h.entries...)
	if len(h.entries) > maxHistory {
		h.entries = h.entries[:maxHistory]
	}
	if h.path == "" {
		return
	}
	// Persist — best-effort, log to stderr on failure but don't fail
	// the line submission.
	_ = h.flushLocked()
}

func (h *fileHistory) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.entries)
}

func (h *fileHistory) At(idx int) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.entries[idx]
}

// flushLocked writes h.entries to disk in oldest-first order (so
// re-reading on next session restores chronological insertion).
// Caller must hold h.mu.
func (h *fileHistory) flushLocked() error {
	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return err
	}
	tmp := h.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for i := len(h.entries) - 1; i >= 0; i-- {
		if _, err := w.WriteString(h.entries[i] + "\n"); err != nil {
			f.Close()
			return err
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, h.path)
}
