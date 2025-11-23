package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/autocli/v3"
)

func main() {
	cmd := cf.NewCommand("process").
		Version("1.0.0").
		Description("Process multiple files").
		Flag("-verbose", "-v").
			Bool().
			Global().
			Help("Enable verbose output").
			Done().
		Flag("FILES").
			StringSlice().
			Variadic().
			Required().
			Global().
			Help("Files to process").
			FilePattern("*.{json,yaml,txt}").
			Done().
		Handler(func(ctx *cf.Context) error {
			// Extract values from context
			verbose := ctx.GetBool("-verbose", false)

			// StringSlice values are stored as []interface{}, need to convert
			var files []string
			if val, ok := ctx.GlobalFlags["FILES"]; ok {
				if interfaceSlice, ok := val.([]interface{}); ok {
					files = make([]string, len(interfaceSlice))
					for i, v := range interfaceSlice {
						files[i] = v.(string)
					}
				}
			}

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
