# autocli/ssh

Expose an [autocli](https://github.com/rosscartlidge/autocli) `Command` as an SSH-accessible interactive console. Phase C of the autocli-shell proposal.

The router-style "ssh into the running service for a CLI prompt" deployment pattern (Cisco/Juniper/vault/etcdctl shape) drops into any Go service in ~20 lines of glue. Operators connect with their existing ssh keys; sessions run `autocli/shell` with channel-backed IO so completion, history, and quoting all work exactly as in a local shell.

Sub-module so the `golang.org/x/crypto/ssh` dependency stays opt-in.

## Usage

```go
package main

import (
    "context"
    "log"

    cf "github.com/rosscartlidge/autocli/v4"
    ssh "github.com/rosscartlidge/autocli/ssh"
)

func main() {
    state := &MyService{...}

    cli := cf.NewCommand("svc").
        Subcommand("status").
        Handler(func(ctx *cf.Context) error {
            s := ctx.State.(*MyService)
            ctx.Stdout().Write([]byte(s.Status() + "\n"))
            return nil
        }).Done().
        Build()

    err := ssh.Serve(context.Background(), cli, ssh.Options{
        Addr:           ":2222",
        HostKeyPath:    "/var/lib/myservice/ssh_host_key",
        AuthorizedKeys: "/etc/myservice/authorized_keys",
        State:          state,
        Welcome:        "myservice console — :help for built-ins",
    })
    if err != nil { log.Fatal(err) }
}
```

Operator workflow:

```
$ ssh -p 2222 alice@myservice.local
myservice console — :help for built-ins
> sta<TAB>tus
running, uptime 3h17m
> :exit
$
```

Try the runnable example: `go run ./_example`.

## Use this as a template

[`_example/main.go`](_example/main.go) is a complete 84-line runnable program — drop it into a new service repo and start replacing the example commands with your own:

```bash
# Copy the example into a new project
mkdir myservice && cd myservice
curl -O https://raw.githubusercontent.com/rosscartlidge/autocli/main/ssh/_example/main.go
go mod init github.com/me/myservice
go mod tidy

# Generate keys (one-time setup)
ssh-keygen -t ed25519 -f ./host_key -N ""    # server's host key (or let the binary auto-generate)
cp ~/.ssh/id_ed25519.pub ./authorized_keys   # add operator keys

# Run + connect
go run . -listen :2222 &
ssh -p 2222 you@127.0.0.1
```

From there, replace the `service` struct with your real state, replace `status` and `echo` with your own subcommands, and add as many as you need. The example's inline comments call out the patterns that aren't obvious from a single read.

## Defaults

- **Addr:** `":2222"` (no root needed, not a well-known service port). Override for second-instance deployments — see "Multiple instances on one host" below.
- **Host key:** ed25519, generated on first start, persisted to `Options.HostKeyPath` (required) with `0600`. Same contract as a real sshd — rotate by replacing the file.
- **Auth:** pubkey only. Either `Options.AuthorizedKeys` (OpenSSH-format file) or `Options.AuthCallback`. Empty/missing config refuses to start (no-auth services are not allowed).
- **Editing mode:** emacs default. Override with `Options.EditingMode = shell.EditingVi`.
- **Grace timeout:** 5s on shutdown; cancelled `ctx` stops accepting new connections and waits for in-flight sessions.

## Multiple instances on one host

Standard 12-factor shape — recommended pattern:

```go
addr := flag.String("listen", ":2222", "SSH listen address")
if env := os.Getenv("MYSERVICE_LISTEN"); env != "" {
    addr = env
}
ssh.Serve(ctx, cli, ssh.Options{Addr: addr, ...})
```

Three layers of override, priority: flag > env var > built-in default. Second instance: `MYSERVICE_LISTEN=:2223 myservice` without rebuilding.

For ephemeral ports and discovery, use `ServeListener` with a pre-bound `net.Listener`:

```go
ln, _ := net.Listen("tcp", *addr)
os.WriteFile("/var/run/myservice.info",
    []byte(fmt.Sprintf("pid=%d addr=%s\n", os.Getpid(), ln.Addr())), 0644)
ssh.ServeListener(ctx, ln, cli, opts)
```

## Users and the authorized_keys model

`autocli/ssh` is **not** tied to `/etc/passwd`. When a client connects with `ssh alice@host`:

- The string `"alice"` is captured in `ConnMeta.User` as a label only.
- Authentication is pure pubkey — any key in `AuthorizedKeys` is accepted regardless of claimed username.
- The claimed username determines per-user history file path (under `Options.HistoryDir`) and flows into `OnLogin`/`OnLogout` audit hooks.

This means **anyone with a valid key can claim any username**. The audit log captures the pubkey fingerprint alongside the claimed username, so impersonation in logs is traceable. If a service needs strict key↔username binding (compliance, multi-tenancy), use `Options.AuthCallback` to enforce the mapping.

## Per-session state

Default: `Options.State` is shared across all sessions; service code synchronises via its own mutex / channels.

Per-session: provide `Options.StatePerConn` to return a fresh state object per accepted connection (per-tenant isolation, audit-scoped state, etc.).

## Cancellation

Handlers should observe `ctx.Ctx().Done()` for long-running work. SSH session close, server shutdown, and `:exit` all cancel the active handler's context.

## What's not in v0.1

- Window-size propagation to readline (line wrapping uses whatever width readline detects on stdin; usually 80).
- `from=`/options enforcement in `authorized_keys` (parsed but ignored).
- Multiple privilege levels — service authors implement via `ctx.State` lookup against `ConnMeta.User`.
- SCP/SFTP subsystems.

See [`autocli-shell-proposal.md`](https://github.com/rosscartlidge/ssql/blob/main/doc/research/autocli-shell-proposal.md) in the ssql repo for the full proposal.
