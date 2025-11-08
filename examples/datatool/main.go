package main

import (
	"fmt"
	"os"
	"time"

	cf "github.com/rosscartlidge/autocli/v3"
)

type Config struct {
	InputFile  string
	OutputFile string
	Format     string
	Verbose    bool
	Timeout    time.Duration
	Timezone   string
	StartTime  time.Time
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

		Flag("-timeout", "-t").
			Bind(&config.Timeout).
			Duration().
			Global().
			Default(time.Duration(30 * time.Second)).
			Help("Timeout for processing").
			Completer(&cf.DurationCompleter{}).
			Done().

		Flag("-timezone", "-tz").
			Bind(&config.Timezone).
			String().
			Global().
			Default("Local").
			Options("Local", "UTC", "Australia/Sydney", "America/New_York", "Europe/London").
			Help("Timezone for time parsing").
			Done().

		Flag("-start-time", "-st").
			Bind(&config.StartTime).
			Time().
			Global().
			TimeFormats(
				"2006-01-02 15:04:05",
				"2006-01-02",
				time.RFC3339,
			).
			TimeZoneFromFlag("-timezone").
			Help("Start time for filtering (format: 2006-01-02 15:04:05 or RFC3339)").
			Completer(cf.NoCompleter{Hint: "<YYYY-MM-DD_HH:MM:SS>"}).
			Done().

		// Local flags (per-clause)
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
				fmt.Printf("  Timeout: %v\n", config.Timeout)
				fmt.Printf("  Timezone: %s\n", config.Timezone)
				if !config.StartTime.IsZero() {
					fmt.Printf("  Start Time: %s\n", config.StartTime.Format(time.RFC3339))
				}
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
