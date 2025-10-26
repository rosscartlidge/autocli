package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/completionflags"
)

// Example using the new fluent Arg() API
func main() {
	var inputFile, outputFile, format string
	var verbose bool

	cmd := cf.NewCommand("datatool").
		Version("1.0.0").
		Description("Process data files with fluent arg API").

		Flag("-input", "-i").
			Bind(&inputFile).
			String().
			Global().
			Required().
			Help("Input file path").
			FilePattern("*.{json,yaml,xml}").
			Done().

		Flag("-output", "-o").
			Bind(&outputFile).
			String().
			Global().
			Help("Output file path (default: stdout)").
			FilePattern("*.{json,yaml,xml}").
			Done().

		Flag("-format", "-f").
			Bind(&format).
			String().
			Global().
			Default("json").
			Help("Output format").
			Options("json", "yaml", "xml").
			Done().

		Flag("-verbose", "-v").
			Bind(&verbose).
			Bool().
			Global().
			Help("Enable verbose output").
			Done().

		// NEW FLUENT ARG API - much cleaner!
		Flag("-filter").
			Arg("FIELD").
				Completer(&cf.StaticCompleter{
					Options: []string{"status", "age", "role", "name", "email"},
				}).
				Done().
			Arg("OPERATOR").
				Completer(&cf.StaticCompleter{
					Options: []string{"eq", "ne", "gt", "lt", "gte", "lte", "contains"},
				}).
				Done().
			Arg("VALUE").
				Completer(cf.NoCompleter{Hint: "<VALUE>"}).
				Done().
			Accumulate().
			Local().
			Help("Add filter condition: field operator value").
			Done().

		Flag("-sort").
			String().
			Local().
			Help("Sort by field in this clause").
			Options("name", "age", "status", "date").
			Done().

		Handler(func(ctx *cf.Context) error {
			if verbose {
				fmt.Printf("Input: %s\n", inputFile)
				fmt.Printf("Format: %s\n", format)
			}

			for i, clause := range ctx.Clauses {
				fmt.Printf("Clause %d:\n", i+1)

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

				if sortVal, ok := clause.Flags["-sort"]; ok {
					fmt.Printf("  Sort by: %s\n", sortVal)
				}
			}

			return nil
		}).

		Build()

	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
