package main

import (
	"fmt"
	"os"

	cf "github.com/rosscartlidge/autocli/v4"
)

func main() {
	cmd := cf.NewCommand("fieldtest").
		Version("1.0.0").
		Description("Test field completion").

		Flag("-input", "-i").
			String().
			Global().
			Required().
			Help("Input data file").
			FilePattern("*.{csv,tsv,json,jsonl}").
			Done().

		CacheFieldsFrom("-input").

		Flag("-group").
			String().
			FieldsFromFlag("-input").
			Help("Field to group by").
			Done().

		Flag("-select").
			String().
			FieldsFromFlag("-input").
			Accumulate().
			Help("Fields to select").
			Done().

		Flag("-sum").
			Arg("FIELD").
				FieldsFromFlag("-input").
				Done().
			Arg("RESULT").Done().
			Accumulate().
			Help("Sum field as result").
			Done().

		Handler(func(ctx *cf.Context) error {
			inputFile, err := ctx.RequireString("-input")
			if err != nil {
				return err
			}

			fmt.Printf("Processing file: %s\n", inputFile)

			if group := ctx.GetString("-group", ""); group != "" {
				fmt.Printf("Group by: %s\n", group)
			}

			// Print select fields
			if selectVal, ok := ctx.GlobalFlags["-select"]; ok {
				if selects, ok := selectVal.([]interface{}); ok {
					fmt.Printf("Select fields:\n")
					for _, s := range selects {
						fmt.Printf("  - %s\n", s)
					}
				}
			}

			// Print sum specs
			if sumVal, ok := ctx.GlobalFlags["-sum"]; ok {
				if sums, ok := sumVal.([]interface{}); ok {
					fmt.Printf("Sum operations:\n")
					for _, s := range sums {
						if m, ok := s.(map[string]interface{}); ok {
							fmt.Printf("  - Sum %s as %s\n", m["FIELD"], m["RESULT"])
						}
					}
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
