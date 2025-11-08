package main

import (
	"fmt"
	"os"
	cf "github.com/rosscartlidge/completionflags/v2"
)

func main() {
	// Test 1: Root command with --
	rootCmd := cf.NewCommand("testroot").
		Version("1.0.0").
		Description("Test -- with root command").

		Flag("-verbose", "-v").
			Help("Enable verbose output").
			Bool().
			Global().
			Done().

		Flag("-filter").
			Help("Filter value").
			Arg("VALUE").Done().
			String().
			Local().
			Done().

		Handler(func(ctx *cf.Context) error {
			verbose := false
			if v, ok := ctx.GlobalFlags["-verbose"]; ok && v != nil {
				verbose = v.(bool)
			}
			fmt.Printf("Root Command\n")
			fmt.Printf("Verbose: %v\n", verbose)
			fmt.Printf("Clauses: %d\n", len(ctx.Clauses))

			for i, clause := range ctx.Clauses {
				fmt.Printf("\nClause %d:\n", i+1)
				if filter, ok := clause.Flags["-filter"].(string); ok && filter != "" {
					fmt.Printf("  Filter: %s\n", filter)
				}
			}

			if len(ctx.RemainingArgs) > 0 {
				fmt.Printf("\nRemaining Args (after --):\n")
				for i, arg := range ctx.RemainingArgs {
					fmt.Printf("  [%d]: %s\n", i, arg)
				}
			}

			return nil
		}).

		Build()

	// Test 2: Subcommand with --
	subCmd := cf.NewCommand("testsub").
		Version("1.0.0").
		Description("Test -- with subcommands").

		Flag("-verbose", "-v").
			Help("Enable verbose output").
			Bool().
			Global().
			Done().

		Subcommand("query").
			Description("Query with filters").

			Flag("-output", "-o").
				Help("Output file").
				Arg("FILE").Done().
				String().
				Global().
				Done().

			Flag("-filter").
				Help("Filter condition").
				Arg("COLUMN").Done().
				Arg("OPERATOR").Done().
				Arg("VALUE").Done().
				Accumulate().
				Local().
				Done().

			Handler(func(ctx *cf.Context) error {
				verbose := false
				if v, ok := ctx.GlobalFlags["-verbose"]; ok && v != nil {
					verbose = v.(bool)
				}
				output := ""
				if o, ok := ctx.GlobalFlags["-output"]; ok && o != nil {
					output = o.(string)
				}

				fmt.Printf("Query Subcommand\n")
				fmt.Printf("Verbose: %v\n", verbose)
				fmt.Printf("Output: %s\n", output)
				fmt.Printf("Clauses: %d\n", len(ctx.Clauses))

				for i, clause := range ctx.Clauses {
					fmt.Printf("\nClause %d:\n", i+1)
					if filters, ok := clause.Flags["-filter"]; ok {
						// Handle both single and accumulated
						switch v := filters.(type) {
						case []interface{}:
							for _, f := range v {
								args := f.(map[string]interface{})
								fmt.Printf("  Filter: %s %s %s\n", args["COLUMN"], args["OPERATOR"], args["VALUE"])
							}
						case map[string]interface{}:
							fmt.Printf("  Filter: %s %s %s\n", v["COLUMN"], v["OPERATOR"], v["VALUE"])
						}
					}
				}

				if len(ctx.RemainingArgs) > 0 {
					fmt.Printf("\nRemaining Args (after --):\n")
					for i, arg := range ctx.RemainingArgs {
						fmt.Printf("  [%d]: %s\n", i, arg)
					}
				}

				return nil
			}).
			Done().

		Build()

	// Determine which test to run based on args
	if len(os.Args) > 1 && os.Args[1] == "sub" {
		// Run subcommand test
		if err := subCmd.Execute(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Run root test
		if err := rootCmd.Execute(os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}
