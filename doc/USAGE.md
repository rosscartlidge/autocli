# completionflags - Comprehensive Usage Guide

A powerful, general-purpose Go package for building command-line applications with advanced flag parsing, clause-based argument grouping, and intelligent bash completion.

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Core Concepts](#core-concepts)
4. [Building Commands](#building-commands)
5. [Flag Configuration](#flag-configuration)
6. [Completion System](#completion-system)
7. [Advanced Features](#advanced-features)
8. [Complete Examples](#complete-examples)
9. [Best Practices](#best-practices)

## Overview

### Key Features

- **Fluent Builder API**: Chain methods to configure commands and flags elegantly
- **Clause-based Grouping**: Group flags into clauses separated by `+` or `-` for Boolean logic
- **Intelligent Completion**: Context-aware bash completion with pluggable completers
- **Multi-argument Flags**: Flags can take multiple arguments with per-argument completion
- **Global vs Local Scope**: Flags can apply to entire command or per-clause
- **Auto-generated Help**: Automatic `-help` and `-man` page generation
- **Universal Bash Completion**: Single completion script works for all programs
- **Zero Dependencies**: Only uses Go standard library

### Philosophy

This package enables you to build sophisticated CLIs where:
- Users can express complex queries with Boolean logic using clauses
- Tab completion guides users through available options
- Multi-argument flags make commands more intuitive
- Help documentation is generated automatically

## Quick Start

### Installation

```bash
go get github.com/rosscartlidge/completionflags
```

### Minimal Example

```go
package main

import (
    "fmt"
    "os"
    cf "github.com/rosscartlidge/completionflags"
)

func main() {
    var verbose bool
    var output string

    cmd := cf.NewCommand("myapp").
        Version("1.0.0").
        Description("A simple example application").

        Flag("-verbose", "-v").
            Bind(&verbose).
            Bool().
            Global().
            Help("Enable verbose output").
            Done().

        Flag("-output", "-o").
            Bind(&output).
            String().
            Global().
            Default("stdout").
            Help("Output destination").
            FilePattern("*.txt").
            Done().

        Handler(func(ctx *cf.Context) error {
            if verbose {
                fmt.Println("Verbose mode enabled")
            }
            fmt.Printf("Output: %s\n", output)
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

# Now tab completion works
myapp -o<TAB>        # completes to -output
myapp -output <TAB>  # shows *.txt files
```

## Core Concepts

### Clauses

Clauses are groups of flags separated by special separators (`+` or `-` by default). They enable Boolean logic in command lines:

```bash
# Single clause (AND logic within clause)
myapp -filter status eq active -filter age gt 18

# Multiple clauses (OR logic between clauses)
myapp -filter status eq active + -filter role eq admin
```

Each clause is processed independently, and results can be combined (typically with OR logic).

### Flag Scopes

**Global Flags**: Apply to the entire command
```go
Flag("-verbose").Global()  // Applies to all clauses
```

**Local Flags**: Apply per-clause
```go
Flag("-filter").Local()    // Each clause can have its own filters
```

### Flag Prefixes

Flags can use `-` or `+` prefixes with user-defined semantics:

```bash
myapp -verbose      # Normal mode
myapp +verbose      # Inverse mode (could mean "quiet")
```

Define behavior with a `PrefixHandler`:
```go
cmd.PrefixHandler(func(flagName string, hasPlus bool) interface{} {
    // Return modified value based on prefix
})
```

## Building Commands

### Command Builder Methods

```go
cmd := cf.NewCommand("name")
```

**Metadata Methods**:
- `.Version(string)` - Set version
- `.Description(string)` - Set description
- `.Author(string)` - Set author info
- `.Example(command, description string)` - Add usage example

**Configuration Methods**:
- `.Separators(seps ...string)` - Set clause separators (default: `["+", "-"]`)
- `.PrefixHandler(h PrefixHandler)` - Handle `+` prefix on flags
- `.Handler(h ClauseHandlerFunc)` - Set the main handler function
- `.Build()` - Finalize and return the command

**Adding Flags**:
```go
.Flag(names ...string)  // Start defining a flag with one or more names
```

### Handler Function

The handler receives a `Context` with parsed data:

```go
Handler(func(ctx *cf.Context) error {
    // Access global flags
    inputFile := ctx.GlobalFlags["-input"].(string)
    verbose := ctx.GlobalFlags["-verbose"].(bool)

    // Process each clause
    for i, clause := range ctx.Clauses {
        fmt.Printf("Clause %d:\n", i+1)

        // Access local flags for this clause
        if filters, ok := clause.Flags["-filter"]; ok {
            // Process filters for this clause
        }
    }

    return nil
})
```

## Flag Configuration

### Flag Builder Methods

Start with `.Flag(names...)`:

```go
Flag("-input", "-i")  // Multiple names/aliases
```

### Scope and Visibility

```go
.Global()           // Flag applies to entire command (default: Local)
.Local()            // Flag applies per-clause
.Hidden()           // Hide from help/completion
```

### Argument Configuration

**Shorthand Methods**:
```go
.Bool()             // No arguments (presence = true)
.String()           // Single string argument
.Int()              // Single int argument
.Float()            // Single float argument
.StringSlice()      // Accumulate multiple string values
```

**Explicit Configuration**:
```go
.Args(count int)                    // Set number of arguments
.ArgName(index int, name string)    // Set display name for argument
.ArgType(index int, type ArgType)   // Set type (ArgString, ArgInt, ArgFloat)
.ArgCompleter(index int, completer) // Set completer for specific argument
```

**Multi-argument Example**:
```go
Flag("-filter").
    Args(3).
    ArgName(0, "FIELD").
    ArgName(1, "OPERATOR").
    ArgName(2, "VALUE").
    ArgCompleter(0, &cf.StaticCompleter{
        Options: []string{"status", "age", "role"},
    }).
    ArgCompleter(1, &cf.StaticCompleter{
        Options: []string{"eq", "ne", "gt", "lt"},
    }).
    ArgCompleter(2, cf.NoCompleter{Hint: "<VALUE>"}).
    Done()
```

### Value Handling

```go
.Bind(ptr interface{})           // Bind to a variable
.Default(value interface{})      // Set default value
.Required()                      // Mark as required
.Accumulate()                    // Allow multiple occurrences (creates slice)
.Validate(fn ValidatorFunc)      // Add validation function
```

### Validation Example

```go
Flag("-port").
    Int().
    Validate(func(value interface{}) error {
        port := value.(int)
        if port < 1 || port > 65535 {
            return fmt.Errorf("port must be between 1 and 65535")
        }
        return nil
    }).
    Done()
```

### Help Text

```go
.Help("Description of this flag")
```

### Completion Configuration

**Simple Completers**:
```go
.FilePattern("*.{json,yaml}")      // File completion with pattern
.Options("json", "yaml", "xml")    // Static list of options
.Completer(completer)              // Custom completer
.CompleterFunc(func)               // Function-based completer
```

## Completion System

### Built-in Completers

#### FileCompleter

Complete file and directory paths with optional pattern filtering:

```go
&cf.FileCompleter{
    Pattern:  "*.{json,yaml,xml}",  // Glob pattern
    DirsOnly: false,                 // Only directories?
    Hint:     "<FILE>",              // Shown when no matches
}
```

Pattern syntax:
- `*.txt` - Single extension
- `*.{json,yaml,xml}` - Multiple extensions
- `.[tc]sv` - Character class
- Leave empty for all files

When no files match, shows: `/path/to/dir/<*.{json,yaml,xml}>`

#### StaticCompleter

Complete from a fixed list:

```go
&cf.StaticCompleter{
    Options: []string{"json", "yaml", "xml"},
}
```

#### NoCompleter

No completions, optionally show a hint:

```go
cf.NoCompleter{Hint: "<VALUE>"}     // Shows <VALUE>
cf.NoCompleter{Hint: "<NUMBER>"}    // Shows <NUMBER>
cf.NoCompleter{}                     // Shows nothing
```

#### ChainCompleter

Try multiple completers in order:

```go
&cf.ChainCompleter{
    Completers: []cf.Completer{
        &cf.FileCompleter{Pattern: "*.json"},
        &cf.StaticCompleter{Options: []string{"stdin", "stdout"}},
    },
}
```

Returns first non-empty result.

#### DynamicCompleter

Choose completer based on context:

```go
&cf.DynamicCompleter{
    Chooser: func(ctx cf.CompletionContext) cf.Completer {
        // Look at previous arguments to decide
        if len(ctx.PreviousArgs) > 0 && ctx.PreviousArgs[0] == "remote" {
            return &cf.StaticCompleter{Options: []string{"ssh", "http"}}
        }
        return &cf.StaticCompleter{Options: []string{"local", "remote"}}
    },
}
```

### Custom Completers

Implement the `Completer` interface:

```go
type MyCompleter struct {
    // Your fields
}

func (mc *MyCompleter) Complete(ctx cf.CompletionContext) ([]string, error) {
    // ctx.Partial - what user has typed
    // ctx.FlagName - which flag is being completed
    // ctx.ArgIndex - which argument (for multi-arg flags)
    // ctx.PreviousArgs - previous args of this flag
    // ctx.GlobalFlags - parsed global flags

    var matches []string

    // Your completion logic here
    // For example, query a database, API, etc.

    return matches, nil
}
```

### Function-based Completer

```go
Flag("-branch").
    CompleterFunc(func(ctx cf.CompletionContext) ([]string, error) {
        // Run git command to get branches
        output, _ := exec.Command("git", "branch", "--list").Output()
        branches := strings.Split(string(output), "\n")

        // Filter based on partial input
        var matches []string
        for _, branch := range branches {
            branch = strings.TrimSpace(branch)
            if strings.HasPrefix(branch, ctx.Partial) {
                matches = append(matches, branch)
            }
        }
        return matches, nil
    }).
    Done()
```

### CompletionContext

Available in all completers:

```go
type CompletionContext struct {
    Partial      string                    // Current partial input
    Args         []string                  // All arguments
    Position     int                       // Current position
    FlagName     string                    // Which flag (e.g., "-filter")
    ArgIndex     int                       // Which argument of flag (0-based)
    PreviousArgs []string                  // Previous args of multi-arg flag
    Command      *Command                  // The command
    CurrentClause *Clause                  // Current clause being parsed
    ParsedClauses []Clause                 // All parsed clauses
    GlobalFlags  map[string]interface{}   // Parsed global flags
}
```

## Advanced Features

### Accumulating Values

When a flag appears multiple times, accumulate values into a slice:

```go
Flag("-filter").
    Args(3).
    Accumulate().  // Key method
    Local().
    Done()
```

Usage:
```bash
myapp -filter status eq active -filter age gt 18
```

Accessing in handler:
```go
if filterVal, ok := clause.Flags["-filter"]; ok {
    filters := filterVal.([]interface{})  // Slice of filter values
    for _, f := range filters {
        filterMap := f.(map[string]interface{})
        field := filterMap["FIELD"]
        operator := filterMap["OPERATOR"]
        value := filterMap["VALUE"]
    }
}
```

### Custom Separators

```go
cmd := cf.NewCommand("myapp").
    Separators("OR", "AND", ";")  // Custom separators
```

Usage:
```bash
myapp -filter foo OR -filter bar
```

### Prefix Handlers

Define custom behavior for `+` prefix:

```go
cmd.PrefixHandler(func(flagName string, hasPlus bool) interface{} {
    if flagName == "-verbose" {
        if hasPlus {
            return false  // +verbose means quiet
        }
        return true
    }
    return nil
})
```

### Custom Argument Types

```go
const (
    ArgString ArgType = iota
    ArgInt
    ArgFloat
)

Flag("-custom").
    Args(1).
    ArgType(0, ArgInt).
    Done()
```

### Generating Help

Built-in flags automatically available:
- `-help`, `--help`, `-h` - Show help text
- `-man` - Show man page (groff format)
- `-completion-script` - Generate bash completion script

### Manual Help Generation

```go
helpText := cmd.GenerateHelp()
fmt.Println(helpText)

manPage := cmd.GenerateMan()
fmt.Println(manPage)

completionScript := cmd.GenerateCompletionScript()
fmt.Println(completionScript)
```

## Complete Examples

### Example 1: Simple Filter Tool

```go
package main

import (
    "fmt"
    "os"
    cf "github.com/rosscartlidge/completionflags"
)

type Config struct {
    Input   string
    Format  string
    Verbose bool
}

func main() {
    config := &Config{}

    cmd := cf.NewCommand("filter").
        Version("1.0.0").
        Description("Filter and transform data files").

        Flag("-input", "-i").
            Bind(&config.Input).
            String().
            Global().
            Required().
            Help("Input file path").
            FilePattern("*.{json,yaml}").
            Done().

        Flag("-format", "-f").
            Bind(&config.Format).
            String().
            Global().
            Default("json").
            Help("Output format").
            Options("json", "yaml", "xml").
            Done().

        Flag("-verbose", "-v").
            Bind(&config.Verbose).
            Bool().
            Global().
            Help("Enable verbose output").
            Done().

        Handler(func(ctx *cf.Context) error {
            fmt.Printf("Processing %s -> %s\n", config.Input, config.Format)
            // Your processing logic here
            return nil
        }).

        Build()

    if err := cmd.Execute(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Example 2: Complex Multi-Clause Query Tool

```go
package main

import (
    "fmt"
    "os"
    cf "github.com/rosscartlidge/completionflags"
)

func main() {
    cmd := cf.NewCommand("query").
        Version("1.0.0").
        Description("Query data with complex filters").

        Example(
            "query -input data.json -filter status eq active -filter age gt 18",
            "Find active users over 18 (AND logic)",
        ).
        Example(
            "query -input data.json -filter status eq active + -filter role eq admin",
            "Find active users OR admins (OR logic with + separator)",
        ).

        // Global flags
        Flag("-input", "-i").
            String().
            Global().
            Required().
            Help("Input data file").
            FilePattern("*.json").
            Done().

        Flag("-output", "-o").
            String().
            Global().
            Help("Output file (default: stdout)").
            FilePattern("*.json").
            Done().

        // Local flags (per-clause)
        Flag("-filter").
            Args(3).
            ArgName(0, "FIELD").
            ArgName(1, "OPERATOR").
            ArgName(2, "VALUE").
            ArgType(0, cf.ArgString).
            ArgType(1, cf.ArgString).
            ArgType(2, cf.ArgString).
            ArgCompleter(0, &cf.StaticCompleter{
                Options: []string{"status", "age", "role", "email", "name"},
            }).
            ArgCompleter(1, &cf.StaticCompleter{
                Options: []string{"eq", "ne", "gt", "lt", "gte", "lte", "contains"},
            }).
            ArgCompleter(2, cf.NoCompleter{Hint: "<VALUE>"}).
            Accumulate().
            Local().
            Help("Add filter condition (can specify multiple per clause)").
            Done().

        Flag("-sort").
            String().
            Local().
            Help("Sort field for this clause").
            Options("name", "age", "status", "created").
            Done().

        Flag("-limit").
            Int().
            Local().
            Help("Limit results for this clause").
            Done().

        Handler(func(ctx *cf.Context) error {
            inputFile := ctx.GlobalFlags["-input"].(string)
            fmt.Printf("Processing: %s\n", inputFile)

            // Process each clause (OR logic between clauses)
            for i, clause := range ctx.Clauses {
                fmt.Printf("\nClause %d:\n", i+1)

                // Get filters (AND logic within clause)
                if filterVal, ok := clause.Flags["-filter"]; ok {
                    if filters, ok := filterVal.([]interface{}); ok {
                        for _, f := range filters {
                            filterMap := f.(map[string]interface{})
                            fmt.Printf("  Filter: %s %s %s\n",
                                filterMap["FIELD"],
                                filterMap["OPERATOR"],
                                filterMap["VALUE"])
                        }
                    }
                }

                // Get sort
                if sortVal, ok := clause.Flags["-sort"]; ok {
                    fmt.Printf("  Sort by: %s\n", sortVal)
                }

                // Get limit
                if limitVal, ok := clause.Flags["-limit"]; ok {
                    fmt.Printf("  Limit: %d\n", limitVal)
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
```

### Example 3: Dynamic Completion

```go
Flag("-host").
    String().
    CompleterFunc(func(ctx cf.CompletionContext) ([]string, error) {
        // Read from SSH config or known_hosts
        file, err := os.ReadFile(os.ExpandEnv("$HOME/.ssh/config"))
        if err != nil {
            return []string{}, nil
        }

        var hosts []string
        lines := strings.Split(string(file), "\n")
        for _, line := range lines {
            line = strings.TrimSpace(line)
            if strings.HasPrefix(line, "Host ") {
                host := strings.TrimPrefix(line, "Host ")
                host = strings.TrimSpace(host)
                if strings.HasPrefix(host, ctx.Partial) {
                    hosts = append(hosts, host)
                }
            }
        }
        return hosts, nil
    }).
    Done()
```

## Best Practices

### 1. Organize Global vs Local Flags

```go
// Global flags that apply to entire command
Flag("-input").Global().Done()
Flag("-output").Global().Done()
Flag("-verbose").Global().Done()

// Local flags that vary per clause
Flag("-filter").Local().Done()
Flag("-sort").Local().Done()
```

### 2. Use Meaningful Argument Names

```go
// Good
.Args(3).
    ArgName(0, "FIELD").
    ArgName(1, "OPERATOR").
    ArgName(2, "VALUE")

// Bad
.Args(3).
    ArgName(0, "ARG1").
    ArgName(1, "ARG2").
    ArgName(2, "ARG3")
```

### 3. Provide Helpful Hints

```go
// For free-form input
ArgCompleter(2, cf.NoCompleter{Hint: "<VALUE>"})

// For file paths
FilePattern("*.json")  // Shows <*.json> hint when no files

// For specific types
cf.NoCompleter{Hint: "<EMAIL>"}
cf.NoCompleter{Hint: "<URL>"}
cf.NoCompleter{Hint: "<NUMBER>"}
```

### 4. Add Usage Examples

```go
cmd.Example(
    "myapp -input data.json -filter status eq active",
    "Filter for active records",
).
Example(
    "myapp -input data.json -filter status eq active + -filter role eq admin",
    "Active records OR admin role",
)
```

### 5. Validate Input

```go
Flag("-port").
    Int().
    Validate(func(v interface{}) error {
        port := v.(int)
        if port < 1 || port > 65535 {
            return fmt.Errorf("invalid port")
        }
        return nil
    }).
    Done()
```

### 6. Use Accumulate for Repeated Flags

```go
// Allow: -filter a -filter b -filter c
Flag("-filter").
    Args(3).
    Accumulate().  // Creates slice of filters
    Local().
    Done()
```

### 7. Bind to Structs for Clean Code

```go
type Config struct {
    Input   string
    Output  string
    Verbose bool
    Format  string
}

config := &Config{}

Flag("-input").Bind(&config.Input).Done()
Flag("-output").Bind(&config.Output).Done()
Flag("-verbose").Bind(&config.Verbose).Done()
Flag("-format").Bind(&config.Format).Done()
```

### 8. Chain Completers for Flexibility

```go
// Try files first, then special values
ArgCompleter(0, &cf.ChainCompleter{
    Completers: []cf.Completer{
        &cf.FileCompleter{Pattern: "*.json"},
        &cf.StaticCompleter{Options: []string{"stdin", "stdout"}},
    },
})
```

### 9. Document with Help Text

```go
Flag("-filter").
    Args(3).
    Help("Filter records: -filter FIELD OPERATOR VALUE. " +
         "Operators: eq, ne, gt, lt, gte, lte, contains. " +
         "Can specify multiple times per clause.").
    Done()
```

### 10. Test Completion Interactively

```bash
# Generate and load completion
eval "$(myapp -completion-script)"

# Test various scenarios
myapp -<TAB>
myapp -input <TAB>
myapp -filter <TAB>
myapp -filter status <TAB>
myapp -filter status eq <TAB>
```

## Troubleshooting

### Completion Not Working

1. Ensure completion script is loaded:
   ```bash
   eval "$(myapp -completion-script)"
   ```

2. Check if completion is registered:
   ```bash
   complete -p myapp
   ```

3. Test completion directly:
   ```bash
   myapp -complete 1 -
   ```

### Flag Not Recognized

- Ensure flag name starts with `-`
- Check if flag is defined with `.Done()`
- Verify scope (Global vs Local)

### Values Not Binding

- Check pointer type matches flag type
- Ensure flag has `.Bind(&variable)`
- Verify argument count matches

### Completion Shows Wrong Results

- Check `ArgCompleter` index matches argument position
- Verify completer logic with test cases
- Use `-complete` flag to debug directly

## API Reference Summary

### Command Builder

- `NewCommand(name string) *CommandBuilder`
- `.Version(string) *CommandBuilder`
- `.Description(string) *CommandBuilder`
- `.Author(string) *CommandBuilder`
- `.Example(cmd, desc string) *CommandBuilder`
- `.Separators(...string) *CommandBuilder`
- `.PrefixHandler(PrefixHandler) *CommandBuilder`
- `.Flag(...string) *FlagBuilder`
- `.Handler(ClauseHandlerFunc) *CommandBuilder`
- `.Build() *Command`

### Flag Builder

**Scope**: `.Global()`, `.Local()`

**Arguments**: `.Bool()`, `.String()`, `.Int()`, `.Float()`, `.StringSlice()`, `.Args(int)`

**Argument Details**: `.ArgName(idx, name)`, `.ArgType(idx, type)`, `.ArgCompleter(idx, completer)`

**Values**: `.Bind(ptr)`, `.Default(val)`, `.Required()`, `.Accumulate()`

**Validation**: `.Validate(ValidatorFunc)`

**Completion**: `.FilePattern(pattern)`, `.Options(...string)`, `.Completer(c)`, `.CompleterFunc(f)`

**Documentation**: `.Help(string)`

**Visibility**: `.Hidden()`

**Finalize**: `.Done() *CommandBuilder`

### Completers

- `FileCompleter{Pattern, DirsOnly, Hint}`
- `StaticCompleter{Options}`
- `NoCompleter{Hint}`
- `ChainCompleter{Completers}`
- `DynamicCompleter{Chooser}`
- Custom: Implement `Complete(CompletionContext) ([]string, error)`

### Context

```go
type Context struct {
    Clauses     []Clause
    GlobalFlags map[string]interface{}
}

type Clause struct {
    Flags     map[string]interface{}
    Separator string
}
```

---

For more examples, see the `examples/` directory in the repository.
