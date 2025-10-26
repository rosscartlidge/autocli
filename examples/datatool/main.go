package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/completionflags"
)

type Config struct {
	InputFile  string
	OutputFile string
	Format     string
	Verbose    bool
}

func main() {
	config := &Config{}

	cmd := cf.NewCommand("datatool").
		Version("1.0.0").
		Description("Process data files with powerful filtering and clause support").
		Author("Example Developer <dev@example.com>").

		Example(
			"datatool -input data.json -format yaml",
			"Convert JSON to YAML",
		).
		Example(
			"datatool -input data.json -filter status eq active -filter age gt 18",
			"Filter records where status=active AND age>18 (single clause)",
		).
		Example(
			"datatool -input data.json -filter status eq active + -filter role eq admin",
			"Filter records where status=active OR role=admin (multiple clauses with OR)",
		).

		// Global flags
		Flag("-input", "-i").
			Bind(&config.InputFile).
			String().
			Global().
			Required().
			Help("Input file path").
			FilePattern("*.{json,yaml,xml}").
			Done().

		Flag("-output", "-o").
			Bind(&config.OutputFile).
			String().
			Global().
			Help("Output file path (default: stdout)").
			FilePattern("*.{json,yaml,xml}").
			Done().

		Flag("-format", "-f").
			Bind(&config.Format).
			String().
			Global().
			Default("json").
			Help("Output format").
			Options("json", "yaml", "xml").
			Done().

		Flag("-verbose", "-v").
			Bind(&config.Verbose).
			Bool().
			Global().
			Help("Enable verbose output").
			Done().

		// Local flags (per-clause)
		Flag("-filter").
			Args(3).
			ArgName(0, "FIELD").
			ArgName(1, "OPERATOR").
			ArgName(2, "VALUE").
			ArgType(0, cf.ArgString).
			ArgType(1, cf.ArgString).
			ArgType(2, cf.ArgString).
			ArgCompleter(0, &cf.StaticCompleter{
				Options: []string{"status", "age", "role", "name", "email"},
			}).
			ArgCompleter(1, &cf.StaticCompleter{
				Options: []string{"eq", "ne", "gt", "lt", "gte", "lte", "contains"},
			}).
			ArgCompleter(2, cf.NoCompleter{Hint: "<VALUE>"}).
			Accumulate().  // Mark to accumulate multiple values
			Local().
			Help("Add filter condition: field operator value (can be specified multiple times per clause)").
			Done().

		Flag("-sort").
			String().
			Local().
			Help("Sort by field in this clause").
			Options("name", "age", "status", "date").
			Done().

		Handler(func(ctx *cf.Context) error {
			if config.Verbose {
				fmt.Printf("Global configuration:\n")
				fmt.Printf("  Input: %s\n", ctx.GlobalFlags["-input"])
				if output, ok := ctx.GlobalFlags["-output"]; ok && output != "" {
					fmt.Printf("  Output: %s\n", output)
				} else {
					fmt.Printf("  Output: stdout\n")
				}
				fmt.Printf("  Format: %s\n", ctx.GlobalFlags["-format"])
				fmt.Printf("  Clauses: %d\n\n", len(ctx.Clauses))
			}

			// Process each clause
			for i, clause := range ctx.Clauses {
				fmt.Printf("Clause %d", i+1)
				if clause.Separator != "" {
					fmt.Printf(" (separator: %s)", clause.Separator)
				}
				fmt.Printf(":\n")

				// Get filters from this clause
				if filterVal, ok := clause.Flags["-filter"]; ok {
					if filters, ok := filterVal.([]interface{}); ok {
						for _, f := range filters {
							if filterMap, ok := f.(map[string]interface{}); ok {
								fmt.Printf("  Filter: %s %s %s\n",
									filterMap["FIELD"],
									filterMap["OPERATOR"],
									filterMap["VALUE"])
							}
						}
					}
				}

				// Get sort from this clause
				if sortVal, ok := clause.Flags["-sort"]; ok {
					fmt.Printf("  Sort by: %s\n", sortVal)
				}

				fmt.Println()
			}

			// In a real program, you would:
			// 1. Load data from config.InputFile
			// 2. For each clause:
			//    - Apply filters
			//    - Apply sort
			//    - Collect results
			// 3. Combine results (e.g., OR logic across clauses)
			// 4. Convert to config.Format
			// 5. Write to config.OutputFile or stdout

			fmt.Printf("Processing complete. Would write output in %s format.\n", config.Format)

			return nil
		}).

		Build()

	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
