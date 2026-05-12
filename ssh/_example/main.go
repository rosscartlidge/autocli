// Example: SSH-accessible service operator console.
//
// 1. Drop your pubkey into ./authorized_keys (or set -keys to point
//    at one).
// 2. Run: go run ./_example
// 3. Connect: ssh -p 2222 you@127.0.0.1
//
// Try at the prompt:
//
//	status
//	echo hello world
//	:set vi
//	:exit
//
// First run generates the SSH host key as ./host_key.
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

type service struct {
	started time.Time
}

func main() {
	listen := flag.String("listen", ":2222", "SSH listen address")
	hostKey := flag.String("host-key", "./host_key", "SSH host key file (generated on first run)")
	authKeys := flag.String("keys", "./authorized_keys", "OpenSSH authorized_keys file")
	flag.Parse()

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
		Build()

	fmt.Printf("listening on %s — ssh -p%s any@127.0.0.1\n", *listen, strings.TrimPrefix(*listen, ":"))

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
