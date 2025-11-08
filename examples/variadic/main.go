package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/completionflags/v2"
)

func main() {
	var verbose bool
	var files []string

	cmd := cf.NewCommand("process").
		Version("1.0.0").
		Description("Process multiple files").
		Flag("-verbose", "-v").
			Bool().
			Bind(&verbose).
			Global().
			Help("Enable verbose output").
			Done().
		Flag("FILES").
			StringSlice().
			Bind(&files).
			Variadic().
			Required().
			Global().
			Help("Files to process").
			FilePattern("*.{json,yaml,txt}").
			Done().
		Handler(func(ctx *cf.Context) error {
			if verbose {
				fmt.Printf("Processing %d files\n", len(files))
			}
			for _, file := range files {
				fmt.Printf("Processing: %s\n", file)
			}
			return nil
		}).
		Build()

	if err := cmd.Execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
