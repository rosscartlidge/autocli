// Package ssh exposes an autocli Command as an SSH-accessible
// interactive console. Phase C of the autocli-shell proposal.
//
// Service authors get the router-style "ssh into the running service
// for a CLI prompt" deployment pattern (Cisco/Juniper/vault/etcdctl
// shape) by adding ~20 lines of glue: build the autocli Command tree,
// point at an authorized_keys file, call Serve. Operators connect
// with `ssh -p PORT user@host` using their existing keys; sessions
// run shell.Serve with channel-backed IO so completion + history +
// quoting all work exactly as in a local autocli/shell.
//
// Sub-module so the dependency on golang.org/x/crypto/ssh stays
// opt-in.
package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	cf "github.com/rosscartlidge/autocli/v4"
	"github.com/rosscartlidge/autocli/shell"
	gossh "golang.org/x/crypto/ssh"
)

// ConnMeta describes an incoming SSH connection. Passed to
// AuthCallback / OnLogin / OnLogout / StatePerConn so service code can
// audit-log or route per user.
type ConnMeta struct {
	// User is the username the client sent. NOT necessarily an OS
	// account — it's just a label; pubkey is what actually
	// authenticates. See autocli-shell-proposal.md "Users and the
	// authorized_keys model" for the threat model.
	User string

	// RemoteAddr is the client's TCP address.
	RemoteAddr string

	// SessionID is a per-connection stable ID (also used to scope
	// per-user state files when StatePerConn isn't supplied).
	SessionID string

	// Fingerprint is the SHA256 fingerprint of the client's
	// authenticated pubkey, e.g. "SHA256:abc...". Useful for audit
	// logs that need a stable identifier independent of the claimed
	// username.
	Fingerprint string
}

// Options configures the SSH server.
type Options struct {
	// Addr is the standard Go listen address. Empty → ":2222". Use
	// "127.0.0.1:2222" to bind loopback-only for services that
	// should never be reachable off-box.
	Addr string

	// HostKeyPath is the file holding the server's private host key.
	// Generated on first run if absent (ed25519, 0600). Same contract
	// as a real sshd — operators rotate by replacing the file.
	// Required.
	HostKeyPath string

	// AuthorizedKeys is the path to an OpenSSH-style authorized_keys
	// file. Connections whose pubkey appears in the file are
	// accepted. Refuse-to-start safety check: empty or unreadable
	// file is a hard error so operators can't accidentally launch a
	// no-auth service.
	//
	// Ignored when AuthCallback is set (the callback is authoritative).
	AuthorizedKeys string

	// AuthCallback, if non-nil, overrides the default AuthorizedKeys
	// lookup. Service authors can implement key↔username binding,
	// LDAP/OAuth integration, etc. Returned `user` overrides
	// ConnMeta.User if non-empty.
	AuthCallback func(meta ConnMeta, key gossh.PublicKey) (allowed bool, user string, err error)

	// State is the shared per-instance value threaded into every
	// session's autocli Context. Type-asserted by handlers.
	State any

	// StatePerConn, if non-nil, runs per accepted connection and its
	// return value becomes that session's Context.State (overriding
	// the shared State). Use for per-tenant isolation, audit-scoped
	// state, etc.
	StatePerConn func(meta ConnMeta) any

	// Welcome banner printed to the session on connect.
	Welcome string

	// HistoryDir, if non-empty, is the parent directory for per-user
	// command history files. Each session writes to
	// HistoryDir/$USER/history.  Empty = no persistent history.
	HistoryDir string

	// EditingMode picks emacs (default) or vi keybindings.
	EditingMode shell.EditingMode

	// GraceTimeout is how long Serve waits for in-flight sessions to
	// finish after ctx cancellation. Default 5s.
	GraceTimeout time.Duration

	// OnLogin / OnLogout are audit hooks invoked at session start /
	// end. Service decides what to do with them (structured log,
	// metric, etc.). Errors from these hooks are logged but do not
	// affect the session.
	OnLogin  func(meta ConnMeta)
	OnLogout func(meta ConnMeta)

	// Logger receives structured server-level events (accept errors,
	// auth failures, graceful-shutdown progress). Defaults to
	// slog.Default().
	Logger *slog.Logger
}

// applyDefaults fills in unset Options fields with reasonable values
// and returns an error if required fields are missing.
func (o *Options) applyDefaults() error {
	if o.Addr == "" {
		o.Addr = ":2222"
	}
	if o.HostKeyPath == "" {
		return errors.New("autocli/ssh: HostKeyPath is required")
	}
	if o.AuthCallback == nil && o.AuthorizedKeys == "" {
		return errors.New("autocli/ssh: AuthorizedKeys or AuthCallback required (no-auth services are refused)")
	}
	if o.GraceTimeout == 0 {
		o.GraceTimeout = 5 * time.Second
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	return nil
}

// Serve binds the listening socket and runs the SSH server until
// ctx is cancelled. Each accepted connection runs shell.Serve in a
// goroutine with the SSH channel as its IO. Returns nil on clean
// shutdown.
//
// On bind failure, returns the bind error. Other errors during
// connection accept are logged via opts.Logger and don't fail Serve.
func Serve(ctx context.Context, cli *cf.Command, opts Options) error {
	if err := opts.applyDefaults(); err != nil {
		return err
	}

	hostKey, err := loadOrGenerateHostKey(opts.HostKeyPath)
	if err != nil {
		return fmt.Errorf("autocli/ssh: host key: %w", err)
	}

	authorized, err := buildAuthorizedSet(opts.AuthorizedKeys)
	if err != nil && opts.AuthCallback == nil {
		return fmt.Errorf("autocli/ssh: authorized_keys: %w", err)
	}

	sshCfg := &gossh.ServerConfig{
		PublicKeyCallback: func(meta gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
			fp := gossh.FingerprintSHA256(key)
			cm := ConnMeta{
				User:        meta.User(),
				RemoteAddr:  meta.RemoteAddr().String(),
				SessionID:   string(meta.SessionID()),
				Fingerprint: fp,
			}
			if opts.AuthCallback != nil {
				ok, user, err := opts.AuthCallback(cm, key)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, fmt.Errorf("auth rejected")
				}
				perms := &gossh.Permissions{Extensions: map[string]string{"fingerprint": fp}}
				if user != "" {
					perms.Extensions["user"] = user
				}
				return perms, nil
			}
			marshaled := string(key.Marshal())
			if _, ok := authorized[marshaled]; !ok {
				return nil, fmt.Errorf("auth rejected: unknown key")
			}
			return &gossh.Permissions{Extensions: map[string]string{"fingerprint": fp}}, nil
		},
	}
	sshCfg.AddHostKey(hostKey)

	ln, err := net.Listen("tcp", opts.Addr)
	if err != nil {
		return fmt.Errorf("autocli/ssh: listen %s: %w", opts.Addr, err)
	}
	return serveListener(ctx, ln, cli, &opts, sshCfg)
}

// ServeListener is the variant that takes an already-bound listener,
// for callers that want to control binding (e.g. ":0" + announce, or
// share with another service). The listener is closed when Serve
// returns.
func ServeListener(ctx context.Context, ln net.Listener, cli *cf.Command, opts Options) error {
	if err := opts.applyDefaults(); err != nil {
		return err
	}
	hostKey, err := loadOrGenerateHostKey(opts.HostKeyPath)
	if err != nil {
		return fmt.Errorf("autocli/ssh: host key: %w", err)
	}
	authorized, err := buildAuthorizedSet(opts.AuthorizedKeys)
	if err != nil && opts.AuthCallback == nil {
		return fmt.Errorf("autocli/ssh: authorized_keys: %w", err)
	}
	sshCfg := &gossh.ServerConfig{
		PublicKeyCallback: func(meta gossh.ConnMetadata, key gossh.PublicKey) (*gossh.Permissions, error) {
			fp := gossh.FingerprintSHA256(key)
			cm := ConnMeta{
				User:        meta.User(),
				RemoteAddr:  meta.RemoteAddr().String(),
				SessionID:   string(meta.SessionID()),
				Fingerprint: fp,
			}
			if opts.AuthCallback != nil {
				ok, user, err := opts.AuthCallback(cm, key)
				if err != nil {
					return nil, err
				}
				if !ok {
					return nil, fmt.Errorf("auth rejected")
				}
				perms := &gossh.Permissions{Extensions: map[string]string{"fingerprint": fp}}
				if user != "" {
					perms.Extensions["user"] = user
				}
				return perms, nil
			}
			marshaled := string(key.Marshal())
			if _, ok := authorized[marshaled]; !ok {
				return nil, fmt.Errorf("auth rejected: unknown key")
			}
			return &gossh.Permissions{Extensions: map[string]string{"fingerprint": fp}}, nil
		},
	}
	sshCfg.AddHostKey(hostKey)
	return serveListener(ctx, ln, cli, &opts, sshCfg)
}

func serveListener(ctx context.Context, ln net.Listener, cli *cf.Command, opts *Options, sshCfg *gossh.ServerConfig) error {
	var wg sync.WaitGroup

	// Close the listener when ctx cancels; that unblocks Accept().
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			ln.Close()
		case <-done:
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				// Clean shutdown initiated.
				break
			}
			opts.Logger.Warn("autocli/ssh: accept", "err", err)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			handleConn(ctx, conn, cli, opts, sshCfg)
		}()
	}

	// Grace period for in-flight sessions.
	gctx, cancel := context.WithTimeout(context.Background(), opts.GraceTimeout)
	defer cancel()
	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
		opts.Logger.Info("autocli/ssh: clean shutdown")
	case <-gctx.Done():
		opts.Logger.Warn("autocli/ssh: grace timeout — some sessions did not exit cleanly", "timeout", opts.GraceTimeout)
	}
	return nil
}

// handleConn performs the SSH handshake and serves the session
// channel. Each connection runs in its own goroutine; errors are
// logged via opts.Logger but don't propagate (Serve must keep
// accepting).
func handleConn(ctx context.Context, conn net.Conn, cli *cf.Command, opts *Options, sshCfg *gossh.ServerConfig) {
	defer conn.Close()
	serverConn, chans, reqs, err := gossh.NewServerConn(conn, sshCfg)
	if err != nil {
		opts.Logger.Info("autocli/ssh: handshake failed", "err", err, "remote", conn.RemoteAddr())
		return
	}
	defer serverConn.Close()

	// Discard global requests (keepalive etc.).
	go gossh.DiscardRequests(reqs)

	meta := ConnMeta{
		User:        serverConn.User(),
		RemoteAddr:  serverConn.RemoteAddr().String(),
		SessionID:   string(serverConn.SessionID()),
		Fingerprint: serverConn.Permissions.Extensions["fingerprint"],
	}
	if u, ok := serverConn.Permissions.Extensions["user"]; ok && u != "" {
		meta.User = u
	}

	if opts.OnLogin != nil {
		opts.OnLogin(meta)
	}
	defer func() {
		if opts.OnLogout != nil {
			opts.OnLogout(meta)
		}
	}()

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			newChan.Reject(gossh.UnknownChannelType, "only session channels supported")
			continue
		}
		ch, chReqs, err := newChan.Accept()
		if err != nil {
			opts.Logger.Warn("autocli/ssh: channel accept", "err", err)
			continue
		}
		// Run the session inline; one channel per connection is the
		// common case and serialising avoids interleaved output.
		handleSession(ctx, ch, chReqs, cli, opts, meta)
	}
}

// handleSession negotiates pty / shell / window-change requests and
// invokes shell.Serve with the channel as IO.
func handleSession(ctx context.Context, ch gossh.Channel, reqs <-chan *gossh.Request, cli *cf.Command, opts *Options, meta ConnMeta) {
	defer ch.Close()

	// Wait for the client to request a shell (after optional pty-req).
	// window-change requests arrive throughout the session; we don't
	// (yet) propagate them to readline.
	ready := make(chan struct{}, 1)
	go func() {
		for req := range reqs {
			switch req.Type {
			case "pty-req":
				// Accept but ignore details — readline reads bytes
				// from the channel; the client side has put the
				// local terminal in raw mode and sends escape
				// sequences directly.
				req.Reply(true, nil)
			case "shell":
				req.Reply(true, nil)
				select {
				case ready <- struct{}{}:
				default:
				}
			case "window-change":
				// Future: emit on a chan readline can pick up via
				// SetSize/Refresh.
				req.Reply(true, nil)
			default:
				req.Reply(false, nil)
			}
		}
	}()

	// Block until the client requests a shell (or gives up).
	select {
	case <-ready:
	case <-ctx.Done():
		return
	case <-time.After(10 * time.Second):
		// Client never asked for a shell — close quietly.
		return
	}

	// Determine session State.
	sessionState := opts.State
	if opts.StatePerConn != nil {
		sessionState = opts.StatePerConn(meta)
	}

	shellOpts := shell.Options{
		Prompt:      "> ",
		EditingMode: opts.EditingMode,
		State:       sessionState,
		Welcome:     opts.Welcome,
		Stdin:       io.NopCloser(ch),
		Stdout:      ch,
		Stderr:      ch.Stderr(),
		Ctx:         ctx,
	}
	if opts.HistoryDir != "" {
		// Per-user history under the supplied directory. User string
		// is used as-is; service authors are responsible for
		// validating it (no path-traversal worry by default — but
		// reject `..` etc. if needed via AuthCallback).
		shellOpts.HistoryFile = opts.HistoryDir + "/" + meta.User + "/history"
		// Ensure dir exists; best-effort.
		_ = os.MkdirAll(opts.HistoryDir+"/"+meta.User, 0o700)
	}
	if err := shell.Serve(cli, shellOpts); err != nil {
		opts.Logger.Warn("autocli/ssh: session", "err", err, "user", meta.User, "remote", meta.RemoteAddr)
	}
}
