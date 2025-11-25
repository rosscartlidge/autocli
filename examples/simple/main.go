package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/autocli/v4"
)

func main() {
	cmd := cf.NewCommand("simple").
		Version("1.0.0").
		Description("A simple example of completionflags").
		Author("Example Author <author@example.com>").

		Example(
			"simple -input data.txt -format json",
			"Convert data.txt to JSON format",
		).
		Example(
			"simple -input data.txt -output result.yaml -format yaml",
			"Convert to YAML and save to result.yaml",
		).

		// Input file flag
		Flag("-input", "-i").
			String().
			Global().
			Required().
			Help("Input file path").
			FilePattern("*.{txt,json,yaml}").
			Done().

		// Output file flag
		Flag("-output", "-o").
			String().
			Global().
			Help("Output file path (default: stdout)").
			FilePattern("*.{json,yaml,xml}").
			Done().

		// Format flag
		Flag("-format", "-f").
			String().
			Global().
			Default("json").
			Help("Output format").
			Options("json", "yaml", "xml").
			Done().

		// Verbose flag
		Flag("-verbose", "-v").
			Bool().
			Global().
			Help("Enable verbose output").
			Done().

		Handler(func(ctx *cf.Context) error {
			// Extract values from context
			inputFile, err := ctx.RequireString("-input")
			if err != nil {
				return err
			}
			outputFile := ctx.GetString("-output", "")
			format := ctx.GetString("-format", "json")
			verbose := ctx.GetBool("-verbose", false)

			if verbose {
				fmt.Printf("Input: %s\n", inputFile)
				fmt.Printf("Output: %s\n", outputFile)
				fmt.Printf("Format: %s\n", format)
				fmt.Printf("Clauses: %d\n", len(ctx.Clauses))
			}

			// Actual processing would go here
			fmt.Printf("Processing %s -> %s format\n", inputFile, format)

			return nil
		}).

		Build()

	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
