package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/autocli/v3"
)

func main() {
	var verbose bool
	var inputFile, outputFile string

	cmd := cf.NewCommand("convert").
		Version("1.0.0").
		Description("Convert files between formats").
		Flag("-verbose", "-v").
			Bool().
			Bind(&verbose).
			Global().
			Help("Enable verbose output").
			Done().
		Flag("INPUT").
			String().
			Bind(&inputFile).
			Required().
			Global().
			Help("Input file to convert").
			FilePattern("*.{json,yaml,txt}").
			Done().
		Flag("OUTPUT").
			String().
			Bind(&outputFile).
			Default("output.txt").
			Global().
			Help("Output file path").
			FilePattern("*.{json,yaml,xml}").
			Done().
		Handler(func(ctx *cf.Context) error {
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
