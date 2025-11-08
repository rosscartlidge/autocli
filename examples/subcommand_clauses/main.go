package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/completionflags/v2"
)

func main() {
	var verbose bool
	var outputFile string

	cmd := cf.NewCommand("datatool").
		Version("1.0.0").
		Description("Data query tool with subcommands and clauses").

		// Root global flags
		Flag("-verbose", "-v").
			Bool().
			Bind(&verbose).
			Global().
			Help("Enable verbose output").
			Done().

		// Subcommand: query (with clause support)
		Subcommand("query").
			Description("Query data with filters using clauses").

			// Global flag for this subcommand
			Flag("-output", "-o").
				String().
				Bind(&outputFile).
				Global().
				Help("Output file").
				Done().

			// Local flag (per-clause)
			Flag("-filter").
				Arg("COLUMN").
					Completer(cf.NoCompleter{Hint: "<COLUMN>"}).
					Done().
				Arg("OPERATOR").
					Completer(&cf.StaticCompleter{
						Options: []string{"eq", "ne", "gt", "lt"},
					}).
					Done().
				Arg("VALUE").
					Completer(cf.NoCompleter{Hint: "<VALUE>"}).
					Done().
				Accumulate().
				Local().
				Help("Filter condition (can specify multiple per clause)").
				Done().

			Flag("-sort").
				String().
				Local().
				Help("Sort by column").
				Done().

			Handler(func(ctx *cf.Context) error {
				fmt.Printf("Query Subcommand\n")
				fmt.Printf("Verbose: %v\n", verbose)
				fmt.Printf("Output: %s\n", outputFile)
				fmt.Printf("Number of clauses: %d\n\n", len(ctx.Clauses))

				for i, clause := range ctx.Clauses {
					if i == 0 {
						fmt.Printf("Clause %d (initial):\n", i+1)
					} else {
						fmt.Printf("Clause %d (separator: %s):\n", i+1, clause.Separator)
					}

					// Get filters
					if filterVal, ok := clause.Flags["-filter"]; ok {
						filters := filterVal.([]interface{})
						for _, f := range filters {
							fm := f.(map[string]interface{})
							fmt.Printf("  Filter: %s %s %s\n",
								fm["COLUMN"], fm["OPERATOR"], fm["VALUE"])
						}
					}

					// Get sort
					if sort, ok := clause.Flags["-sort"]; ok {
						fmt.Printf("  Sort: %s\n", sort)
					}

					fmt.Println()
				}

				return nil
			}).
			Done().

		// Subcommand: import (no clauses needed)
		Subcommand("import").
			Description("Import data from files").

			Flag("SOURCE").
				String().
				Required().
				Global().
				Help("Source file").
				Done().

			Handler(func(ctx *cf.Context) error {
				source := ctx.GlobalFlags["SOURCE"].(string)
				fmt.Printf("Import Subcommand\n")
				fmt.Printf("Verbose: %v\n", verbose)
				fmt.Printf("Source: %s\n", source)
				return nil
			}).
			Done().

		Build()

	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
