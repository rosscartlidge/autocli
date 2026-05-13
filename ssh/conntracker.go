package ssh

import (
	"sync"

	gossh "golang.org/x/crypto/ssh"
)

// connTracker is the set of currently-live SSH connections to a
// server. serveListener uses it to force-close every active conn
// when its context is cancelled, so readline.Readline() in each
// session sees EOF and exits immediately rather than holding open
// the grace period.
//
// Safe for concurrent use. closeAll() takes a snapshot of the set
// under the lock so calling Close() on each conn (which can trigger
// callbacks that take the lock back via remove()) doesn't deadlock.
type connTracker struct {
	mu    sync.Mutex
	conns map[*gossh.ServerConn]struct{}
}

func newConnTracker() *connTracker {
	return &connTracker{conns: make(map[*gossh.ServerConn]struct{})}
}

func (t *connTracker) add(c *gossh.ServerConn) {
	t.mu.Lock()
	t.conns[c] = struct{}{}
	t.mu.Unlock()
}

func (t *connTracker) remove(c *gossh.ServerConn) {
	t.mu.Lock()
	delete(t.conns, c)
	t.mu.Unlock()
}

// closeAll snapshots the current set and calls Close() on each conn
// outside the lock. The handlers' own deferred remove() will then run
// to clean up entries; double-remove is a no-op on a map.
func (t *connTracker) closeAll() {
	t.mu.Lock()
	snapshot := make([]*gossh.ServerConn, 0, len(t.conns))
	for c := range t.conns {
		snapshot = append(snapshot, c)
	}
	t.mu.Unlock()
	for _, c := range snapshot {
		_ = c.Close()
	}
}
