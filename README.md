# autocli

A powerful, automatic CLI framework for Go that builds sophisticated command-line applications with **git/docker/kubectl-style subcommands**, advanced flag parsing, clause-based argument grouping, auto-generated help/man pages, and intelligent bash completion.

> **Note**: Renamed from `completionflags` to better reflect the comprehensive feature set. This is v3.0.0 with a new module path.

[![Go Reference](https://pkg.go.dev/badge/github.com/rosscartlidge/autocli.svg)](https://pkg.go.dev/github.com/rosscartlidge/autocli)

## Features

- **⚡ Nested Subcommands** - Multi-level command hierarchies (git remote add, docker container exec)
- **🎯 Three-Level Flag Scoping** - Root global, subcommand global, and per-clause flags
- **🔧 Fluent Builder API** - Chain methods to configure commands elegantly
- **🎯 Fluent Arg() API** - Safe, index-free multi-argument configuration (NEW!)
- **📋 Clause-based Grouping** - Boolean logic with `+`/`-` separators
- **✨ Smart Completion** - Context-aware with helpful pattern hints
- **🔍 Pattern Hints** - Shows `/path/<*.json>` when no files match
- **🎨 Multi-argument Flags** - Per-argument types and completers
- **🌐 Global vs Local Scope** - Root, subcommand, and per-clause flags
- **📖 Auto-generated Help** - `-help` and `-man` pages
- **🚀 Universal Completion** - Single script for all programs
- **0️⃣ Zero Dependencies** - Pure Go stdlib

## Quick Start

### Installation

```bash
go get github.com/rosscartlidge/autocli/v3
```

### Simple Example

```go
package main

import (
    "fmt"
    "os"
    cf "github.com/rosscartlidge/autocli/v3"
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

### Subcommand Example

Build git/docker/kubectl-style commands with distributed subcommands:

```go
package main

import (
    "fmt"
    "os"
    cf "github.com/rosscartlidge/autocli/v3"
)

func main() {
    cmd := cf.NewCommand("myapp").
        Version("1.0.0").
        Description("Multi-command application").

        // Root-level global flag (available to all subcommands)
        Flag("-verbose", "-v").
            Bool().
            Global().
            Help("Enable verbose output").
            Done().

        // 'query' subcommand
        Subcommand("query").
            Description("Query data with filters").

            // Subcommand-level flag
            Flag("-limit").
                Int().
                Default(10).
                Help("Limit results").
                Done().

            Handler(func(ctx *cf.Context) error {
                verbose := ctx.GlobalFlags["-verbose"].(bool)
                limit := ctx.GlobalFlags["-limit"].(int)

                if verbose {
                    fmt.Printf("Querying with limit=%d\n", limit)
                }
                // Query logic here
                return nil
            }).
            Done().

        // 'import' subcommand
        Subcommand("import").
            Description("Import data from file").

            Flag("-file", "-f").
                String().
                Required().
                FilePattern("*.{json,csv}").
                Help("File to import").
                Done().

            Handler(func(ctx *cf.Context) error {
                file := ctx.GlobalFlags["-file"].(string)
                fmt.Printf("Importing from %s\n", file)
                // Import logic here
                return nil
            }).
            Done().

        Build()

    if err := cmd.Execute(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

**Usage:**
```bash
myapp query -limit 20              # Use query subcommand
myapp -verbose query -limit 20     # Root global flag before subcommand
myapp query -verbose -limit 20     # Root global flag after subcommand
myapp import -file data.json       # Use import subcommand
myapp -help                        # Shows all subcommands
myapp query -help                  # Shows query-specific help
```

### Nested Subcommand Example

Build multi-level command hierarchies like `git remote add` or `docker container exec`:

```go
package main

import (
    "fmt"
    "os"
    cf "github.com/rosscartlidge/autocli/v3"
)

func main() {
    cmd := cf.NewCommand("gitlike").
        Version("1.0.0").
        Description("A git-like CLI with nested subcommands").

        // Root global flag (available everywhere)
        Flag("-verbose", "-v").
            Bool().
            Global().
            Help("Enable verbose output").
            Done().

        // Top-level subcommand: remote
        Subcommand("remote").
            Description("Manage remote repositories").

            // Nested subcommand: remote add
            Subcommand("add").
                Description("Add a new remote repository").
                Flag("-fetch", "-f").Bool().Help("Fetch after adding").Done().
                Handler(func(ctx *cf.Context) error {
                    fmt.Println("Adding remote repository")
                    fetch := ctx.GetBool("-fetch", false)
                    if fetch {
                        fmt.Println("  Will fetch after adding")
                    }
                    return nil
                }).
                Done().

            // Nested subcommand: remote list
            Subcommand("list").
                Description("List all remote repositories").
                Handler(func(ctx *cf.Context) error {
                    verbose := ctx.GetBool("-verbose", false)
                    fmt.Println("Listing remotes")
                    if verbose {
                        fmt.Println("  origin\thttps://github.com/user/repo.git")
                    } else {
                        fmt.Println("  origin")
                    }
                    return nil
                }).
                Done().

            Done().

        // Top-level subcommand: branch
        Subcommand("branch").
            Description("Manage branches").

            Subcommand("list").
                Description("List all branches").
                Flag("-all", "-a").Bool().Help("List all branches").Done().
                Handler(func(ctx *cf.Context) error {
                    all := ctx.GetBool("-all", false)
                    fmt.Println("Listing branches")
                    if all {
                        fmt.Println("  Including remote branches")
                    }
                    return nil
                }).
                Done().

            Done().

        Build()

    if err := cmd.Execute(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

**Usage:**
```bash
gitlike remote add origin https://github.com/user/repo.git
gitlike remote add -fetch origin https://...   # With flag
gitlike -verbose remote list                   # Root global before subcommand
gitlike remote list -verbose                   # Root global after subcommand
gitlike branch list -all                       # Nested subcommand with flag

gitlike -help                # Shows top-level subcommands (remote, branch)
gitlike remote -help         # Shows nested subcommands (add, list)
gitlike remote add -help     # Shows help for specific nested command
```

**Handler Pattern:**
```go
Handler(func(ctx *cf.Context) error {
    // Use IsSubcommandPath to check the full path
    switch {
    case ctx.IsSubcommandPath("remote", "add"):
        return handleRemoteAdd(ctx)
    case ctx.IsSubcommandPath("remote", "list"):
        return handleRemoteList(ctx)
    case ctx.IsSubcommandPath("branch", "list"):
        return handleBranchList(ctx)
    default:
        return fmt.Errorf("unknown subcommand: %v", ctx.SubcommandPath)
    }
})
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
- **[subcommand/](examples/subcommand/)** - Git-style subcommands with global flags
- **[nested_subcommands/](examples/nested_subcommands/)** - Multi-level command hierarchies (git remote add, docker container exec)
- **[subcommand_clauses/](examples/subcommand_clauses/)** - Subcommands with clause-based parsing
- **[datatool/](examples/datatool/)** - Advanced multi-clause query tool with fluent Arg() API
- **[remaining_args/](examples/remaining_args/)** - Unix `--` convention support

### Running Examples

```bash
# Subcommand example
cd examples/subcommand
go build
./subcommand -help                 # See all subcommands
./subcommand list -help            # Subcommand-specific help
eval "$(./subcommand -completion-script)"
./subcommand list -verbose         # Run with flags

# Advanced datatool example
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
    Flag(...).                        // Root command flag
    Subcommand("subcmd").             // Add subcommand
        Description("...").
        Flag(...).                    // Subcommand-specific flag
        Handler(...).
        Done().
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
    .Global()                        // Root global (all subcommands) or subcommand global
    .Local()                         // Per-clause only

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

### Subcommand Builder

```go
Subcommand("name").
    Description("...")               // Subcommand description
    Flag(...).Done()                 // Subcommand-specific flag
    Handler(func(ctx *cf.Context) error {
        // ctx.GlobalFlags - root and subcommand globals
        // ctx.Clauses - clause data
        return nil
    })
    Done()                           // Return to command builder
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
