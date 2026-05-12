# autocli/shell

A readline-based interactive shell that drives an [autocli](https://github.com/rosscartlidge/autocli) `Command` from a process-embedded REPL rather than the bash completion protocol. Phase B of the autocli-shell proposal.

The same command tree powers both a bash CLI invocation and an embedded interactive session — no duplication. Tab calls `cli.Complete`, Enter parses the line and runs `cli.ExecuteWith`. Handlers get the same `*Context` they'd get from a bash invocation, plus the caller-supplied `Stdin`/`Stdout`/`Stderr`/`State`/`Ctx`.

This is a sub-module so the [autocli core](https://github.com/rosscartlidge/autocli) stays stdlib-only — only embedded callers pay for the `chzyer/readline` dependency.

## Usage

```go
package main

import (
    cf "github.com/rosscartlidge/autocli/v4"
    "github.com/rosscartlidge/autocli/shell"
)

func main() {
    svc := myService{...}

    cli := cf.NewCommand("demo").
        Subcommand("status").
        Handler(func(ctx *cf.Context) error {
            s := ctx.State.(*myService)
            fmt.Fprintln(ctx.Stdout(), s.Status())
            return nil
        }).Done().
        Build()

    shell.Serve(cli, shell.Options{
        Prompt:  "demo> ",
        State:   svc,
        Welcome: "demo console — :help for built-ins",
    })
}
```

Try the runnable example: `go run ./_example`.

## Built-in commands

All start with `:` so they can't collide with user-defined subcommands.

- `:help` — built-in help summary
- `:exit` / `:quit` / `:q` — close the session
- `:history` — show session history info
- `:set` — show current editing mode
- `:set vi` / `:set emacs` — change editing mode (takes effect on next session — see note below)

## Tokenisation

Input lines are split with shell-style quoting:

- whitespace separates tokens
- `'literal'` — single quotes, no escapes
- `"double quotes"` — supports `\"`, `\\`, `\$`, `` \` ``
- `\<char>` outside quotes escapes the next character

Matches bash word-splitting closely enough that quoting rules transfer between the bash-CLI and the embedded shell.

## Editing modes

`chzyer/readline` supports both emacs (default) and vi modes. The proposal's recommended pattern is to default to whatever fits your operator base (emacs for general services, vi for vi-leaning teams) and let the user override with `:set` at runtime.

**Note:** runtime `SetVimMode` in `chzyer/readline` races with its own input goroutine (library issue, not autocli/shell's), so we defer the actual mode switch until the next session — the user `:exit`s and reconnects to get the new bindings. The `Options.EditingMode` bookkeeping is updated immediately so per-user preference persistence works.

## Cancellation

Handlers should observe `ctx.Ctx().Done()` for long-running operations. The shell wires it up automatically — SSH session close, parent shutdown, or `:exit` cancels the active handler.

## What's next

This is Phase B (the readline driver). Phase C wraps this in an SSH server using `golang.org/x/crypto/ssh` for the multi-operator "service console over SSH" deployment story. See [`autocli-shell-proposal.md`](https://github.com/rosscartlidge/ssql/blob/main/doc/research/autocli-shell-proposal.md) in the ssql repo.
