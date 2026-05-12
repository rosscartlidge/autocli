// Example: drive an autocli Command tree in-process without going
// through the bash protocol. Demonstrates the Complete + ExecuteWith
// API used by autocli/shell and autocli/ssh.
//
// For a real interactive shell, use github.com/rosscartlidge/autocli/shell.
// For a real SSH-accessible console, use github.com/rosscartlidge/autocli/ssh.
//
// This example just shows what the two APIs return so library
// authors can build their own driver.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"

	cf "github.com/rosscartlidge/autocli/v4"
)

type appState struct{ counter int }

func main() {
	cli := cf.NewCommand("demo").
		Subcommand("inc").
		Description("increment the in-memory counter").
		Handler(func(ctx *cf.Context) error {
			s := ctx.State.(*appState)
			s.counter++
			fmt.Fprintf(ctx.Stdout(), "counter = %d\n", s.counter)
			return nil
		}).
		Done().
		Subcommand("status").
		Description("report counter value").
		Handler(func(ctx *cf.Context) error {
			s := ctx.State.(*appState)
			fmt.Fprintf(ctx.Stdout(), "counter = %d\n", s.counter)
			return nil
		}).
		Done().
		Build()

	// --- 1) Pure completion: ask the engine what completes "inc"
	//        without printing anything to stdout / exiting / etc.
	completions, _ := cli.Complete([]string{"inc"}, 1)
	fmt.Println("Suggestions for partial 'inc':", completions)

	// --- 2) Dispatch via ExecuteWith — caller supplies the Context
	//        with its own Stdout, State, cancellation, etc.
	state := &appState{}
	var buf bytes.Buffer
	base := (&cf.Context{State: state}).
		SetStdout(&buf).
		SetCtx(context.Background())

	for _, line := range [][]string{
		{"inc"},
		{"inc"},
		{"inc"},
		{"status"},
	} {
		if err := cli.ExecuteWith(line, base); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
	}

	fmt.Println("--- captured handler output ---")
	fmt.Print(buf.String())
}
