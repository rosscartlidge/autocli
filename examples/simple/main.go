package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/autocli/v3"
)

type Config struct {
	InputFile  string
	OutputFile string
	Format     string
	Verbose    bool
}

func main() {
	config := &Config{}

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
			Bind(&config.InputFile).
			String().
			Global().
			Required().
			Help("Input file path").
			FilePattern("*.{txt,json,yaml}").
			Done().

		// Output file flag
		Flag("-output", "-o").
			Bind(&config.OutputFile).
			String().
			Global().
			Help("Output file path (default: stdout)").
			FilePattern("*.{json,yaml,xml}").
			Done().

		// Format flag
		Flag("-format", "-f").
			Bind(&config.Format).
			String().
			Global().
			Default("json").
			Help("Output format").
			Options("json", "yaml", "xml").
			Done().

		// Verbose flag
		Flag("-verbose", "-v").
			Bind(&config.Verbose).
			Bool().
			Global().
			Help("Enable verbose output").
			Done().

		Handler(func(ctx *cf.Context) error {
			if config.Verbose {
				fmt.Printf("Input: %s\n", config.InputFile)
				fmt.Printf("Output: %s\n", config.OutputFile)
				fmt.Printf("Format: %s\n", config.Format)
				fmt.Printf("Clauses: %d\n", len(ctx.Clauses))
			}

			// Actual processing would go here
			fmt.Printf("Processing %s -> %s format\n", config.InputFile, config.Format)

			return nil
		}).

		Build()

	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
