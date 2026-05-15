# autocli/shell

An interactive line-editing driver for an [autocli](https://github.com/rosscartlidge/autocli) `Command`, built on `golang.org/x/term`. Phase B of the autocli-shell proposal.

The same command tree powers both a bash CLI invocation and an embedded interactive session — no duplication. Tab calls `cli.Complete`, Enter parses the line and runs `cli.ExecuteWith`. Handlers get the same `*Context` they'd get from a bash invocation, plus the caller-supplied `Stdin`/`Stdout`/`Stderr`/`State`/`Ctx`.

This is a sub-module so the [autocli core](https://github.com/rosscartlidge/autocli) stays stdlib-only — only embedded callers pay for the `golang.org/x/term` dependency.

## v0.2 migration

shell v0.1 was built on `chzyer/readline`. Three concurrency/correctness bugs in that library (it hardcodes `os.Stdin` regardless of `Config.Stdin`) drove the switch to `golang.org/x/term` in v0.2.

**Behavioural changes from v0.1:**

- **No vi mode.** `golang.org/x/term` is emacs-only. The `EditingMode` field on `Options` remains for API stability; `:set vi` is accepted but prints a deprecation note. Operators who really need vi can run their editor of choice; the shell prompt itself is emacs-keybindings.
- **No reverse-incremental-search (Ctrl-R).** x/term has line history via Up/Down arrows but no Ctrl-R search. Tradeoff for the simpler/correct termios handling.
- **`PrefsFile` is now a no-op.** It used to persist the vi/emacs choice; that's gone. Field preserved for backward-compat compilation.

Everything else (tab completion, history file, `:exit`/`:help`/`:history` built-ins, friendly unknown-command messages, intercept-help-token, etc.) is unchanged.

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
- `:history` — show history file info (Up/Down arrows browse history; no Ctrl-R search)
- `:set` — show current editing mode (always emacs)

## Tokenisation

Input lines are split with shell-style quoting:

- whitespace separates tokens
- `'literal'` — single quotes, no escapes
- `"double quotes"` — supports `\"`, `\\`, `\$`, `` \` ``
- `\<char>` outside quotes escapes the next character

Matches bash word-splitting closely enough that quoting rules transfer between the bash-CLI and the embedded shell.

## Keybindings (emacs-style, via x/term)

- `Ctrl-A` / `Ctrl-E` — beginning / end of line
- `Ctrl-K` / `Ctrl-U` — kill to end / kill to start
- `Ctrl-W` — kill previous word
- `Ctrl-L` — clear screen
- `Up` / `Down` — browse history
- `Ctrl-C` / `Ctrl-D` — close the session (signals EOF to ReadLine)
- `TAB` — autocli completion

## Terminal handling

- **Local mode** (`Options.Stdin == os.Stdin` AND it's a real terminal): `Serve` puts the terminal in raw mode for the session and restores on exit. Standard.
- **Embedded mode** (`Options.Stdin` is anything else — typically an SSH channel from `autocli/ssh`): no termios handling at all. The SSH client manages the user's local terminal; the server passes bytes through.

This is the v0.2 fix for the recurring "Ctrl-C on the server stops working when sessions connect" bug — `chzyer/readline` always called `MakeRaw` on FD 0, even when a non-terminal `Config.Stdin` was provided. `x/term` correctly takes the caller's `io.ReadWriter` and never touches `os.Stdin` itself.

## Cancellation

Handlers should observe `ctx.Ctx().Done()` for long-running operations. The shell wires it up automatically — SSH session close, parent shutdown, or `:exit` cancels the active handler.

`Serve` itself observes `Options.Ctx` between line reads. To interrupt a blocking `ReadLine` from outside, the caller closes `Stdin` (which is what `autocli/ssh` does — closing the SSH channel surfaces as EOF to ReadLine).

## What's next

Position 2 (`io.Pipe`-based pipelines at the prompt) — let users run `from-loaded | where ... | to table` inside the shell — is the natural next layer. See [`autocli-shell-proposal.md`](https://github.com/rosscartlidge/ssql/blob/main/doc/research/autocli-shell-proposal.md) in the ssql repo.
