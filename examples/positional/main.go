package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/autocli/v4"
)

func main() {
	cmd := cf.NewCommand("convert").
		Version("1.0.0").
		Description("Convert files between formats").
		Flag("-verbose", "-v").
			Bool().
			Global().
			Help("Enable verbose output").
			Done().
		Flag("INPUT").
			String().
			Required().
			Global().
			Help("Input file to convert").
			FilePattern("*.{json,yaml,txt}").
			Done().
		Flag("OUTPUT").
			String().
			Default("output.txt").
			Global().
			Help("Output file path").
			FilePattern("*.{json,yaml,xml}").
			Done().
		Handler(func(ctx *cf.Context) error {
			// Extract values from context
			verbose := ctx.GetBool("-verbose", false)
			inputFile, err := ctx.RequireString("INPUT")
			if err != nil {
				return err
			}
			outputFile := ctx.GetString("OUTPUT", "output.txt")

			if verbose {
				fmt.Printf("Converting %s to %s\n", inputFile, outputFile)
			}
			fmt.Printf("Success: %s -> %s\n", inputFile, outputFile)
			return nil
		}).
		Build()

	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
