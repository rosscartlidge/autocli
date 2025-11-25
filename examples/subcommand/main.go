package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/autocli/v4"
)

func main() {
	cmd := cf.NewCommand("myapp").
		Version("1.0.0").
		Description("A sample application with subcommands").

		// Root global flags
		Flag("-verbose", "-v").
			Bool().
			Global().
			Help("Enable verbose output").
			Done().

		Flag("-config").
			String().
			Global().
			Help("Configuration file").
			Done().

		// Subcommand: query
		Subcommand("query").
			Description("Query data with filters").

			Handler(func(ctx *cf.Context) error {
				// Extract values from context
				verbose := ctx.GetBool("-verbose", false)
				configFile := ctx.GetString("-config", "")

				fmt.Printf("Executing query subcommand\n")
				fmt.Printf("Verbose: %v\n", verbose)
				fmt.Printf("Config: %s\n", configFile)
				fmt.Printf("Clauses: %d\n", len(ctx.Clauses))
				return nil
			}).
			Done().

		// Subcommand: import
		Subcommand("import").
			Description("Import data from files").

			Handler(func(ctx *cf.Context) error {
				// Extract values from context
				verbose := ctx.GetBool("-verbose", false)

				fmt.Printf("Executing import subcommand\n")
				fmt.Printf("Verbose: %v\n", verbose)
				return nil
			}).
			Done().

		// Root handler (optional)
		Handler(func(ctx *cf.Context) error {
			fmt.Println("No subcommand specified")
			fmt.Println("Available commands: query, import")
			fmt.Println("Use -help for more information")
			return nil
		}).

		Build()

	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
