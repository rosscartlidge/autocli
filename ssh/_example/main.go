// Example: a complete, minimal SSH-accessible operator console.
//
// This file is intended as a TEMPLATE. Copy it into a new project and
// replace the example commands with your own — see the inline
// comments below for the why-of-each-piece.
//
// Run it:
//
//	# One-time setup
//	cp ~/.ssh/id_ed25519.pub ./authorized_keys   # whose key gets in
//	go run ./_example                            # generates ./host_key on first run
//
//	# Connect from another shell (any username string works — pubkey authenticates)
//	ssh -p 2222 you@127.0.0.1
//
// Try at the prompt:
//
//	status                       # service-defined subcommand
//	echo hello "world from ssh"  # variadic positional args (quoting works)
//	:help                        # shell built-ins (always start with ":")
//	:set vi                      # switch to vi keybindings (next session)
//	:exit                        # close
//
// First run generates the SSH host key as ./host_key (ed25519, 0600).
// Subsequent runs reuse it — same contract as a real sshd. Rotate by
// replacing the file.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	cf "github.com/rosscartlidge/autocli/v4"
	ssh "github.com/rosscartlidge/autocli/ssh"
)

// service is the per-instance value that every handler can reach via
// ctx.State (set once in main below, passed through ssh.Options.State).
// In a real service this is where you put: the loaded dataset, mutable
// config, counters, references to long-lived clients, etc.
//
// All sessions share one *service pointer by default. If you need
// per-session isolation (e.g. one tenant per connection) use
// ssh.Options.StatePerConn instead — it runs per accepted connection
// and its return value becomes that session's ctx.State.
//
// You're responsible for synchronisation on mutable shared state —
// autocli/ssh imposes no locking. Standard Go: mutexes, channels,
// atomic, whatever fits your access pattern.
type service struct {
	started time.Time
}

func main() {
	// Standard stdlib flag parsing for the server-side knobs.
	// Three-tier override pattern (flag > env > default) is documented
	// in the autocli/ssh README under "Multiple instances on one host".
	listen := flag.String("listen", ":2222", "SSH listen address")
	hostKey := flag.String("host-key", "./host_key", "SSH host key file (generated on first run, 0600)")
	authKeys := flag.String("keys", "./authorized_keys", "OpenSSH authorized_keys file (refuses to start if missing)")
	flag.Parse()

	svc := &service{started: time.Now()}

	// Build the autocli Command tree shown at the SSH prompt.
	// The fluent builder pattern: NewCommand → Subcommand → Flag/Handler
	// → Done (returns to parent) → Subcommand → ... → Build.
	cli := cf.NewCommand("demo").

		// --- subcommand 1: status ---
		//
		// Zero-flag subcommand. The handler reads from ctx.State and
		// writes to ctx.Stdout() — not os.Stdout. Output flows into
		// the SSH channel so each operator sees their own session
		// output, not someone else's. (os.Stdout points at the SERVER
		// process's stdout, which goes to whoever ran the binary.)
		Subcommand("status").
		Description("show service uptime").
		Handler(func(ctx *cf.Context) error {
			s := ctx.State.(*service)
			fmt.Fprintf(ctx.Stdout(), "uptime: %v\n", time.Since(s.started).Round(time.Second))
			return nil
		}).
		Done().

		// --- subcommand 2: echo with variadic positional args ---
		//
		// .Variadic().Global() is the autocli idiom for "the rest of
		// the positional args go into this flag, accessible via
		// ctx.GlobalFlags". Why .Global()? Because variadic positional
		// flags must land in GlobalFlags (not the per-clause map) — it's
		// a parser-level requirement. Forget this and the handler sees
		// nil and silently prints empty lines (it bit Phase D testing).
		//
		// The value comes back as either []string or []interface{}
		// depending on parsing path — handle both, as below.
		Subcommand("echo").
		Description("echo positional args").
		Flag("WORDS").String().Variadic().Global().Done().
		Handler(func(ctx *cf.Context) error {
			raw := ctx.GlobalFlags["WORDS"]
			var parts []string
			switch v := raw.(type) {
			case []string:
				parts = v
			case []interface{}:
				parts = make([]string, len(v))
				for i, x := range v {
					parts[i] = fmt.Sprintf("%v", x)
				}
			}
			fmt.Fprintln(ctx.Stdout(), strings.Join(parts, " "))
			return nil
		}).
		Done().

		// Build seals the tree. After this point the Command is
		// immutable and safe to share across goroutines (each SSH
		// session uses its own Context with its own IO/State).
		Build()

	// Server boot message goes to the server's stderr, not to any SSH
	// session — there's no session yet at this point.
	fmt.Printf("listening on %s — ssh -p%s any@127.0.0.1\n", *listen, strings.TrimPrefix(*listen, ":"))

	// Serve blocks until ctx is cancelled (Ctrl-C on the server,
	// SIGTERM from systemd, etc.). For a real service, wire ctx to
	// signal.NotifyContext(os.Interrupt, syscall.SIGTERM) so SIGTERM
	// triggers graceful shutdown via ssh.Options.GraceTimeout.
	err := ssh.Serve(context.Background(), cli, ssh.Options{
		Addr:           *listen,
		HostKeyPath:    *hostKey,
		AuthorizedKeys: *authKeys,
		State:          svc,
		Welcome:        "demo console — :help for built-ins, :exit to quit",
	})
	if err != nil {
		log.Fatal(err)
	}
}
