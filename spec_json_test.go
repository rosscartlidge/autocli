package completionflags

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestSpecJSON pins the -spec-json dump: declarations flow through —
// subcommands, flag metadata, static enums inline, dynamic
// vocabularies marked by kind, expression args, clause separators.
func TestSpecJSON(t *testing.T) {
	cmd := NewCommand("svc").
		Subcommand("read").
		Description("read things").
		Flag("FILE").
		String().
		Completer(&FileCompleter{Pattern: "*.csv"}).
		Global().
		Required().
		Done().
		Flag("-if").
		Arg("field").
		FieldsFromFlag("FILE").
		Done().
		Arg("op").
		Completer(&StaticCompleter{Options: []string{"eq", "ne"}}).
		Done().
		Arg("value").
		Done().
		Accumulate().
		Done().
		Flag("-x").
		Arg("EXPR").
		Expression().
		Done().
		Done().
		Flag("-fast").
		Bool().
		Done().
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	var buf bytes.Buffer
	if err := cmd.WriteSpecJSON(&buf); err != nil {
		t.Fatal(err)
	}
	var spec SpecCommand
	if err := json.Unmarshal(buf.Bytes(), &spec); err != nil {
		t.Fatalf("dump is not valid JSON: %v", err)
	}
	read := spec.Subcommands["read"]
	if read == nil || read.Help != "read things" {
		t.Fatalf("read subcommand missing/wrong: %+v", spec.Subcommands)
	}
	byName := map[string]SpecFlag{}
	for _, f := range read.Flags {
		byName[f.Names[0]] = f
	}
	if f := byName["FILE"]; !f.Required || f.Scope != "global" || f.Args[0].Completer != "file" || f.Args[0].Pattern != "*.csv" {
		t.Errorf("FILE spec wrong: %+v", f)
	}
	fi := byName["-if"]
	if !fi.Accumulate || len(fi.Args) != 3 {
		t.Fatalf("-if spec wrong: %+v", fi)
	}
	if fi.Args[0].Completer != "fields" || fi.Args[0].SourceFlag != "FILE" {
		t.Errorf("-if field arg wrong: %+v", fi.Args[0])
	}
	if fi.Args[1].Completer != "static" || strings.Join(fi.Args[1].Options, ",") != "eq,ne" {
		t.Errorf("-if op enum not inlined: %+v", fi.Args[1])
	}
	if x := byName["-x"]; len(x.Args) == 0 || !x.Args[0].Expression {
		t.Errorf("-x expression mark lost: %+v", x)
	}
	if f := byName["-fast"]; !f.Bool {
		t.Errorf("-fast should be bool: %+v", f)
	}
}
