# completionflags

A powerful, general-purpose Go package for building sophisticated command-line applications with advanced flag parsing, clause-based argument grouping, and intelligent bash completion.

[![Go Reference](https://pkg.go.dev/badge/github.com/rosscartlidge/completionflags.svg)](https://pkg.go.dev/github.com/rosscartlidge/completionflags)

## Features

- **🔧 Fluent Builder API** - Chain methods to configure commands elegantly
- **🎯 Fluent Arg() API** - Safe, index-free multi-argument configuration (NEW!)
- **📋 Clause-based Grouping** - Boolean logic with `+`/`-` separators
- **✨ Smart Completion** - Context-aware with helpful pattern hints
- **🔍 Pattern Hints** - Shows `/path/<*.json>` when no files match
- **🎨 Multi-argument Flags** - Per-argument types and completers
- **🌐 Global vs Local Scope** - Command-wide or per-clause flags
- **📖 Auto-generated Help** - `-help` and `-man` pages
- **🚀 Universal Completion** - Single script for all programs
- **0️⃣ Zero Dependencies** - Pure Go stdlib

## Quick Start

### Installation

```bash
go get github.com/rosscartlidge/completionflags
```

### Simple Example

```go
package main

import (
    "fmt"
    "os"
    cf "github.com/rosscartlidge/completionflags"
)

func main() {
    var input, format string

    cmd := cf.NewCommand("myapp").
        Version("1.0.0").
        Description("Process data files").

        Flag("-input", "-i").
            Bind(&input).
            String().
            Required().
            Help("Input file").
            FilePattern("*.{json,yaml,xml}").
            Done().

        Flag("-format", "-f").
            Bind(&format).
            String().
            Default("json").
            Help("Output format").
            Options("json", "yaml", "xml").
            Done().

        Handler(func(ctx *cf.Context) error {
            fmt.Printf("Processing %s as %s\n", input, format)
            return nil
        }).

        Build()

    if err := cmd.Execute(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Enable Bash Completion

```bash
# Add to ~/.bashrc or run manually
eval "$(myapp -completion-script)"

# Now tab completion works!
myapp -i<TAB>              # completes to -input
myapp -input <TAB>         # shows *.{json,yaml,xml} files
myapp -format <TAB>        # shows: json yaml xml
```

## Key Features Explained

### 1. Fluent Arg() API (Recommended for Multi-Argument Flags)

**NEW!** Configure multi-argument flags safely without index errors:

```go
// NEW fluent API - safe and readable
Flag("-filter").
    Arg("FIELD").
        Completer(&cf.StaticCompleter{
            Options: []string{"status", "age", "role"},
        }).
        Done().
    Arg("OPERATOR").
        Completer(&cf.StaticCompleter{
            Options: []string{"eq", "ne", "gt", "lt"},
        }).
        Done().
    Arg("VALUE").
        Completer(cf.NoCompleter{Hint: "<VALUE>"}).
        Done().
    Done()
```

**Benefits:**
- ✅ No index errors (arguments added in order)
- ✅ Auto-counted
- ✅ Clear visual grouping
- ✅ Type-safe with defaults

See [doc/ARG_API_COMPARISON.md](doc/ARG_API_COMPARISON.md) for details.

### 2. Multi-Argument Flag Values

Multi-arg flags are stored as `map[string]interface{}`:

```go
// Command: myapp -filter status eq active
Handler(func(ctx *cf.Context) error {
    if filterVal, ok := clause.Flags["-filter"]; ok {
        filterMap := filterVal.(map[string]interface{})
        field := filterMap["FIELD"].(string)      // "status"
        operator := filterMap["OPERATOR"].(string) // "eq"
        value := filterMap["VALUE"].(string)      // "active"
    }
    return nil
})
```

With `.Accumulate()`, multiple occurrences create a slice:

```go
// Command: myapp -filter status eq active -filter age gt 18
if filterVal, ok := clause.Flags["-filter"]; ok {
    filters := filterVal.([]interface{})
    for _, f := range filters {
        filterMap := f.(map[string]interface{})
        // Process each filter
    }
}
```

### 3. Smart Completion Hints

When no matches are found, helpful hints guide users:

```bash
myapp -output /tmp/nonexistent/     # Shows: /tmp/nonexistent/<*.json>
myapp -filter age gt                # Shows: <VALUE>
```

### 4. Clause-Based Logic

Group flags into clauses for Boolean expressions:

```bash
# Single clause (AND logic within clause)
myapp -filter status eq active -filter age gt 18

# Multiple clauses (OR logic between clauses)
myapp -filter status eq active + -filter role eq admin
```

```go
Handler(func(ctx *cf.Context) error {
    // Process each clause (OR logic)
    for i, clause := range ctx.Clauses {
        fmt.Printf("Clause %d:\n", i+1)
        // Filters within clause have AND logic
    }
    return nil
})
```

## Built-in Completers

### FileCompleter
```go
&cf.FileCompleter{
    Pattern:  "*.{json,yaml,xml}",  // Glob pattern
    DirsOnly: false,                 // Only directories?
    Hint:     "<FILE>",              // Shown when no matches
}
```

### StaticCompleter
```go
&cf.StaticCompleter{
    Options: []string{"json", "yaml", "xml"},
}
```

### NoCompleter
```go
cf.NoCompleter{Hint: "<VALUE>"}     // Shows hint
cf.NoCompleter{}                     // Shows nothing
```

### Custom Completers

Implement the `Completer` interface:

```go
type MyCompleter struct{}

func (mc *MyCompleter) Complete(ctx cf.CompletionContext) ([]string, error) {
    // ctx.Partial - what user typed
    // ctx.FlagName - which flag
    // ctx.ArgIndex - which argument
    // ctx.PreviousArgs - previous arguments

    var matches []string
    // Your logic here
    return matches, nil
}
```

## Documentation

📖 **[Comprehensive Usage Guide](doc/USAGE.md)** - Complete API reference, examples, and best practices

📋 **[Arg API Comparison](doc/ARG_API_COMPARISON.md)** - Index-based vs Fluent Arg() API

## Examples

See the `examples/` directory:

- **[simple/](examples/simple/)** - Basic flag usage with completion
- **[datatool/](examples/datatool/)** - Advanced multi-clause query tool with fluent Arg() API

### Running Examples

```bash
cd examples/datatool
go build
./datatool -help
eval "$(./datatool -completion-script)"
./datatool -input sample_data.json -filter age gt 25
```

## Quick API Reference

### Command Builder

```go
cf.NewCommand("name").
    Version("1.0.0").
    Description("...").
    Flag(...).
    Handler(func(ctx *cf.Context) error { ... }).
    Build()
```

### Flag Builder

```go
Flag("-name", "-n").
    // Argument types
    .Bool()                          // No arguments
    .String()                        // Single string
    .Int()                           // Single int
    .Arg("NAME").Done()              // Multi-arg (fluent API)

    // Scope
    .Global()                        // Command-wide
    .Local()                         // Per-clause

    // Values
    .Bind(&variable)                 // Bind to variable
    .Default(value)                  // Default value
    .Required()                      // Mark required
    .Accumulate()                    // Multiple occurrences

    // Completion
    .FilePattern("*.json")           // File completer
    .Options("a", "b", "c")          // Static options
    .Completer(myCompleter)          // Custom completer

    // Documentation
    .Help("Description")             // Help text
    .Done()                          // Finalize
```

### Handler Context

```go
func(ctx *cf.Context) error {
    // Global flags
    input := ctx.GlobalFlags["-input"].(string)

    // Process clauses
    for _, clause := range ctx.Clauses {
        // Local flags per clause
        if val, ok := clause.Flags["-filter"]; ok {
            // Process flag value
        }
    }

    return nil
}
```

## License

MIT License - see [LICENSE](LICENSE) file

## Contributing

Contributions welcome! Please open an issue or submit a pull request.

## Links

- [GitHub Repository](https://github.com/rosscartlidge/completionflags)
- [Full Documentation](doc/USAGE.md)
- [Examples](examples/)
