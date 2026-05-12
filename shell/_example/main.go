// Example: a tiny "service operator console" using autocli/shell.
//
// Run: go run ./_example
//
// Try at the prompt:
//
//	status
//	echo hello world
//	echo "quoted args" 'work fine'
//	:help
//	:set vi
//	:exit
//
// Ctrl-D also exits cleanly.
package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	cf "github.com/rosscartlidge/autocli/v4"
	"github.com/rosscartlidge/autocli/shell"
)

type service struct {
	started time.Time
}

func main() {
	svc := &service{started: time.Now()}

	cli := cf.NewCommand("demo").
		Subcommand("status").
		Description("show service uptime").
		Handler(func(ctx *cf.Context) error {
			s := ctx.State.(*service)
			fmt.Fprintf(ctx.Stdout(), "uptime: %v\n", time.Since(s.started).Round(time.Second))
			return nil
		}).
		Done().
		Subcommand("echo").
		Description("echo positional args back").
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
		Build()

	err := shell.Serve(cli, shell.Options{
		Prompt:  "demo> ",
		State:   svc,
		Welcome: "demo console — type :help for built-ins, :exit to quit",
		Goodbye: "bye",
	})
	if err != nil {
		log.Fatal(err)
	}
}
