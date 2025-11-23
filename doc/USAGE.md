# autocli - Comprehensive Usage Guide

A powerful, general-purpose Go package for building command-line applications with advanced flag parsing, clause-based argument grouping, and intelligent bash completion.

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Core Concepts](#core-concepts)
4. [Building Commands](#building-commands)
5. [Subcommands](#subcommands)
6. [Nested Subcommands](#nested-subcommands)
7. [Flag Configuration](#flag-configuration)
8. [Completion System](#completion-system)
9. [Advanced Features](#advanced-features)
10. [Complete Examples](#complete-examples)
11. [Best Practices](#best-practices)

## Overview

### Key Features

- **Fluent Builder API**: Chain methods to configure commands and flags elegantly
- **Fluent Arg() API**: Safe, index-free multi-argument configuration (recommended!)
- **Subcommands**: Build distributed CLI tools like git, docker, kubectl
- **Nested Subcommands**: Multi-level command hierarchies (git remote add, docker container exec)
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
    cf "github.com/rosscartlidge/autocli/v3"
)

func main() {
    cmd := cf.NewCommand("myapp").
        Version("1.0.0").
        Description("A simple example application").

        Flag("-verbose", "-v").
            Bool().
            Global().
            Help("Enable verbose output").
            Done().

        Flag("-output", "-o").
            String().
            Global().
            Default("stdout").
            Help("Output destination").
            FilePattern("*.txt").
            Done().

        Handler(func(ctx *cf.Context) error {
            // Extract values from context
            verbose := ctx.GetBool("-verbose", false)
            output := ctx.GetString("-output", "stdout")

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

### End of Flags Marker (`--`)

Following Unix convention, `--` stops flag parsing. Everything after `--` is treated as literal arguments, even if they look like flags:

```bash
# All args after -- go to ctx.RemainingArgs
myapp -verbose -- arg1 arg2 arg3

# Even flag-like strings are literal
myapp -verbose -- -not-a-flag --also-not-a-flag

# Works with clauses - terminates clause parsing
myapp -filter a eq 1 + -filter b eq 2 -- extra1 extra2

# Works with subcommands
myapp query -filter name eq Alice -- file1.txt file2.txt
```

Access remaining args in your handler:

```go
Handler(func(ctx *cf.Context) error {
    if len(ctx.RemainingArgs) > 0 {
        fmt.Println("Extra arguments:", ctx.RemainingArgs)
        // ["arg1", "arg2", "arg3"]
    }
    return nil
})
```

**Scope**: `--` works at both root command and subcommand level:
- Before subcommand: Args go to root context
- After subcommand: Args go to subcommand context
- After clauses: Terminates all clause parsing, args go to current context

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

## Subcommands

Subcommands allow you to build distributed command-line tools where the first argument determines which command to execute, similar to `git`, `docker`, or `kubectl`.

### Pattern

```bash
command [root-global-flags] subcommand [subcommand-flags] [clauses]
```

### Key Features

- **Full clause support** - Use `+` and `-` separators for Boolean logic in subcommands
- **Three-level flag scoping** - Root global, subcommand global, and per-clause local flags
- **Flexible flag placement** - Root globals work before or after the subcommand name
- **Shell completion** - Context-aware completion for subcommands, flags, and arguments
- **Auto-generated help** - Comprehensive help for root and each subcommand

### Quick Example

```go
cmd := cf.NewCommand("myapp").
    Version("1.0.0").
    Description("My application with subcommands").

    // Root global flag (available to all subcommands)
    Flag("-verbose", "-v").
        Description("Enable verbose output").
        Bool().
        Global().
        Done().

    // Query subcommand
    Subcommand("query").
        Description("Query data with filters").

        Flag("-output", "-o").
            Description("Output file").
            Arg("FILE").Done().
            String().
            Global().
            Done().

        Flag("-filter").
            Description("Filter condition").
            Arg("COLUMN").Done().
            Arg("OPERATOR").Values("eq", "ne", "gt", "lt").Done().
            Arg("VALUE").Done().
            Accumulate().
            Local().
            Done().

        Handler(func(ctx *cf.Context) error {
            verbose := ctx.GlobalFlags["-verbose"].(bool)
            output := ctx.GlobalFlags["-output"].(string)

            fmt.Printf("Query command (verbose=%v, output=%s)\n", verbose, output)

            for _, clause := range ctx.Clauses {
                filters := clause.Flags["-filter"].([]interface{})
                for _, f := range filters {
                    args := f.([]string)
                    fmt.Printf("  Filter: %s %s %s\n", args[0], args[1], args[2])
                }
            }
            return nil
        }).
        Done().

    // Import subcommand
    Subcommand("import").
        Description("Import data from files").

        Positional("FILE").
            Description("File to import").
            Required().
            Done().

        Handler(func(ctx *cf.Context) error {
            file := ctx.GlobalFlags["FILE"].(string)
            fmt.Printf("Importing from: %s\n", file)
            return nil
        }).
        Done().

    Build()
```

### Example Usage

```bash
# Show available subcommands
$ myapp -help

# Query with filters
$ myapp query -filter name eq "Alice" -filter age gt 30

# Use root global flag before subcommand
$ myapp -verbose query -output results.txt -filter status eq active

# Use root global flag after subcommand
$ myapp query -verbose -filter status eq active

# Multiple clauses (OR logic)
$ myapp query -filter status eq active + -filter priority eq high

# Import subcommand
$ myapp import data.csv
```

### Three-Level Flag Scoping

Flags can be scoped at three levels:

#### 1. Root Global Flags

Available to all subcommands, parsed before routing. Define on root command with `.Global()`:

```go
cmd := cf.NewCommand("myapp").
    Flag("-verbose", "-v").
        Description("Enable verbose output").
        Bool().
        Global().  // Available to all subcommands
        Done()
```

**Usage:**
```bash
# Before subcommand
$ myapp -verbose query -filter ...

# After subcommand
$ myapp query -verbose -filter ...

# Both work identically!
```

#### 2. Subcommand Global Flags

Apply across all clauses but only within that subcommand:

```go
Subcommand("query").
    Flag("-output", "-o").
        Description("Output file").
        Arg("FILE").Done().
        String().
        Global().  // Global within this subcommand
        Done()
```

**Usage:**
```bash
# Applied to all clauses
$ myapp query -output results.txt -filter a eq 1 + -filter b eq 2
#              ^-- applies to both clauses
```

#### 3. Subcommand Local Flags

Clause-specific within a subcommand:

```go
Subcommand("query").
    Flag("-filter").
        Description("Filter condition").
        Arg("COLUMN").Done().
        Arg("OPERATOR").Done().
        Arg("VALUE").Done().
        Accumulate().
        Local().  // Per-clause only
        Done()
```

**Usage:**
```bash
# Each clause has its own filters
$ myapp query -filter a eq 1 -filter b eq 2 + -filter c eq 3
#              ^-- clause 1: 2 filters      ^-- clause 2: 1 filter
```

### Scope Comparison Table

| Scope | Defined On | Available In | Use Case |
|-------|-----------|--------------|----------|
| Root Global | Root command | All subcommands, all clauses | App-wide settings (verbose, config file) |
| Subcommand Global | Subcommand | That subcommand, all clauses | Subcommand-wide settings (output file, format) |
| Subcommand Local | Subcommand | That subcommand, per clause | Clause-specific options (filters, conditions) |

### Subcommand Builder Methods

- `.Description(string)` - Set subcommand description
- `.Author(string)` - Set author information
- `.Example(usage, description)` - Add usage examples
- `.Flag(names...)` - Define a flag (returns SubcommandFlagBuilder)
- `.Positional(name)` - Define positional argument
- `.Separators(seps...)` - Define clause separators (default: `+`, `-`)
- `.Handler(func(*Context) error)` - Set the handler function
- `.Done()` - Return to CommandBuilder

### Handler Context

The handler receives a `Context` with:

```go
type Context struct {
    Command        *Command
    SubcommandPath []string                  // Full path for nested subcommands (e.g., ["remote", "add"])
    Clauses        []Clause                  // Parsed clauses
    GlobalFlags    map[string]interface{}    // All global flags
    RemainingArgs  []string                  // Arguments after -- (literal)
    ExecutionError error
}
```

**Helper Methods** (v2.1.0+):
```go
// Check if a specific subcommand path is active
ctx.IsSubcommandPath("remote", "add")  // true if path is ["remote", "add"]

// Check if any subcommand is active
ctx.IsSubcommand("remote")             // true if path is ["remote"] or ["remote", ...]

// Get the leaf subcommand name
ctx.SubcommandName()                   // "add" for path ["remote", "add"]
```

### Accessing Values in Handler

```go
Handler(func(ctx *cf.Context) error {
    // Access root globals
    verbose := ctx.GlobalFlags["-verbose"].(bool)
    config := ctx.GlobalFlags["-config"].(string)

    // Access subcommand globals
    output := ctx.GlobalFlags["-output"].(string)
    format := ctx.GlobalFlags["-format"].(string)

    // Iterate through clauses
    for i, clause := range ctx.Clauses {
        fmt.Printf("Clause %d (separator: %s):\n", i+1, clause.Separator)

        // Access local flags for this clause
        if filters, ok := clause.Flags["-filter"]; ok {
            for _, filter := range filters.([]interface{}) {
                args := filter.([]string)
                fmt.Printf("  Filter: %s %s %s\n", args[0], args[1], args[2])
            }
        }
    }

    // Access remaining arguments (after --)
    if len(ctx.RemainingArgs) > 0 {
        fmt.Printf("Extra files to process: %v\n", ctx.RemainingArgs)
    }

    return nil
})
```

### Clauses in Subcommands

Subcommands fully support clause-based parsing for building complex queries with Boolean logic:

```bash
# Single clause
$ myapp query -filter status eq active -filter age gt 30

# Multiple clauses with OR logic (+)
$ myapp query -filter status eq active + -filter priority eq high

# Multiple clauses with AND logic (-)
$ myapp query -filter status eq active - -filter verified eq true

# Complex: (active AND age>30) OR (priority=high)
$ myapp query -filter status eq active -filter age gt 30 + -filter priority eq high
```

### Shell Completion

Completion works automatically for subcommands:

```bash
$ myapp <TAB>
query  import

$ myapp -<TAB>
-verbose  -v  -help  --help  -h  -man

$ myapp query -<TAB>
-verbose  -v  -output  -o  -filter  -sort  -help  --help  -h  -man

$ myapp query -filter <TAB>
<COLUMN>

$ myapp query -filter status <TAB>
eq  ne  gt  lt

$ myapp query -filter status eq active + <TAB>
-verbose  -v  -output  -o  -filter  -sort  -help  --help  -h  -man
```

### Help Generation

#### Root Help

Shows available subcommands and root global flags:

```bash
$ myapp -help
```

Output:
```
myapp v1.0.0 - My application with subcommands

USAGE:
    myapp [GLOBAL OPTIONS] <COMMAND> [COMMAND OPTIONS]

COMMANDS:
    import          Import data from files
    query           Query data with filters using clauses

GLOBAL OPTIONS:
    -verbose, -v
        Enable verbose output

Use 'myapp <command> -help' for detailed help on a specific command.
```

#### Subcommand Help

Shows detailed help for a specific subcommand:

```bash
$ myapp query -help
```

Output:
```
myapp query - Query data with filters using clauses

USAGE:
    myapp query [OPTIONS] [+|- ...]

OPTIONS:
    -output, -o FILE
        Output file

PER-CLAUSE OPTIONS:
    -filter COLUMN OPERATOR VALUE
        Filter condition
        Can be specified multiple times

CLAUSES:
    Arguments can be grouped into clauses using separators.
    Separators: +, -
    Each clause is processed independently (typically with OR logic).
```

### Complete Subcommand Example

```go
package main

import (
    "fmt"
    "os"
    cf "github.com/rosscartlidge/autocli/v3"
)

func main() {
    cmd := cf.NewCommand("datatool").
        Version("2.0.0").
        Description("Data query tool with subcommands and clauses").
        Author("Your Name").

        // Root global flags
        Flag("-verbose", "-v").
            Description("Enable verbose logging").
            Bool().
            Global().
            Done().

        Flag("-config", "-c").
            Description("Configuration file").
            Arg("FILE").
                Hint("path/to/config.json").
                Done().
            String().
            Global().
            Done().

        // Query subcommand
        Subcommand("query").
            Description("Query data with filters using clauses").
            Example("datatool query -filter name eq Alice", "Find records where name equals Alice").
            Example("datatool query -filter status eq active + -filter priority eq high", "OR query").

            // Subcommand global flag
            Flag("-output", "-o").
                Description("Output file").
                Arg("FILE").
                    Hint("results.txt").
                    Done().
                String().
                Global().
                Done().

            Flag("-format", "-f").
                Description("Output format").
                Arg("FORMAT").
                    Values("json", "csv", "table").
                    Done().
                String().
                Global().
                Done().

            // Per-clause local flags
            Flag("-filter").
                Description("Filter condition (can specify multiple per clause)").
                Arg("COLUMN").
                    Completer(cf.StaticCompleter("name", "email", "status", "age", "created")).
                    Done().
                Arg("OPERATOR").
                    Completer(cf.StaticCompleter("eq", "ne", "gt", "lt", "gte", "lte", "contains")).
                    Done().
                Arg("VALUE").
                    Hint("<value>").
                    Done().
                Accumulate().
                Local().
                Done().

            Flag("-sort").
                Description("Sort by column").
                Arg("COLUMN").
                    Completer(cf.StaticCompleter("name", "email", "status", "age", "created")).
                    Done().
                String().
                Local().
                Done().

            Flag("-limit").
                Description("Limit results per clause").
                Arg("N").Done().
                Int().
                Local().
                Done().

            Handler(func(ctx *cf.Context) error {
                // Access root globals
                verbose := ctx.GlobalFlags["-verbose"].(bool)
                config := ctx.GlobalFlags["-config"].(string)

                // Access subcommand globals
                output := ctx.GlobalFlags["-output"].(string)
                format := ctx.GlobalFlags["-format"].(string)

                if verbose {
                    fmt.Printf("Query Subcommand\n")
                    fmt.Printf("Verbose: %v\n", verbose)
                    fmt.Printf("Config: %s\n", config)
                    fmt.Printf("Output: %s\n", output)
                    fmt.Printf("Format: %s\n", format)
                    fmt.Printf("Number of clauses: %d\n\n", len(ctx.Clauses))
                }

                // Process each clause
                for i, clause := range ctx.Clauses {
                    if i == 0 {
                        fmt.Printf("Clause %d (initial):\n", i+1)
                    } else {
                        fmt.Printf("Clause %d (separator: %s):\n", i+1, clause.Separator)
                    }

                    // Local flags for this clause
                    if filters, ok := clause.Flags["-filter"]; ok {
                        for _, filter := range filters.([]interface{}) {
                            args := filter.([]string)
                            fmt.Printf("  Filter: %s %s %s\n", args[0], args[1], args[2])
                        }
                    }

                    if sort, ok := clause.Flags["-sort"].(string); ok && sort != "" {
                        fmt.Printf("  Sort: %s\n", sort)
                    }

                    if limit, ok := clause.Flags["-limit"].(int); ok && limit > 0 {
                        fmt.Printf("  Limit: %d\n", limit)
                    }

                    fmt.Println()
                }

                return nil
            }).
            Done().

        // Import subcommand
        Subcommand("import").
            Description("Import data from files").
            Example("datatool import data.csv", "Import from CSV file").

            Positional("FILE").
                Description("File to import").
                Required().
                Done().

            Flag("-type", "-t").
                Description("File type").
                Arg("TYPE").
                    Values("csv", "json", "xml").
                    Done().
                String().
                Global().
                Done().

            Flag("-skip-errors").
                Description("Skip rows with errors").
                Bool().
                Global().
                Done().

            Handler(func(ctx *cf.Context) error {
                file := ctx.GlobalFlags["FILE"].(string)
                fileType := ctx.GlobalFlags["-type"].(string)
                skipErrors := ctx.GlobalFlags["-skip-errors"].(bool)
                verbose := ctx.GlobalFlags["-verbose"].(bool)

                if verbose {
                    fmt.Printf("Import Subcommand\n")
                    fmt.Printf("File: %s\n", file)
                    fmt.Printf("Type: %s\n", fileType)
                    fmt.Printf("Skip Errors: %v\n", skipErrors)
                }

                fmt.Printf("Importing from: %s\n", file)
                return nil
            }).
            Done().

        Build()

    if err := cmd.Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

### Usage Examples

```bash
# Simple query
$ datatool query -filter name eq Alice

# Complex query with multiple clauses
$ datatool -v query -output results.json -format json \
    -filter status eq active -filter age gt 30 -sort name -limit 10 + \
    -filter priority eq high -sort created -limit 5

# Import with options
$ datatool -verbose import -type csv -skip-errors data.csv

# Root global can go anywhere
$ datatool query -verbose -filter status eq active
$ datatool -verbose query -filter status eq active
# Both work identically!
```

## Nested Subcommands

**NEW in v2.1.0**: Build multi-level command hierarchies like `git remote add` or `docker container exec`.

### Overview

Nested subcommands allow you to organize complex CLIs into hierarchical structures:

```bash
gitlike remote add origin https://...
gitlike remote list
gitlike branch delete -force old-feature
gitlike config set user.name "John"
```

### Building Nested Hierarchies

Chain `.Subcommand()` calls to create nested structures:

```go
cmd := cf.NewCommand("gitlike").
    Version("1.0.0").
    Description("A git-like CLI with nested subcommands").

    // Root global flag
    Flag("-verbose", "-v").Bool().Global().Help("Verbose output").Done().

    // Top-level subcommand: remote
    Subcommand("remote").
        Description("Manage remote repositories").

        // Nested subcommand: remote add
        Subcommand("add").
            Description("Add a new remote repository").
            Flag("-fetch", "-f").Bool().Help("Fetch after adding").Done().
            Handler(func(ctx *cf.Context) error {
                fetch := ctx.GetBool("-fetch", false)
                fmt.Println("Adding remote")
                if fetch {
                    fmt.Println("  Will fetch")
                }
                return nil
            }).
            Done().

        // Nested subcommand: remote remove
        Subcommand("remove").
            Description("Remove a remote repository").
            Handler(func(ctx *cf.Context) error {
                fmt.Println("Removing remote")
                return nil
            }).
            Done().

        Done().  // Return to root CommandBuilder

    // Another top-level subcommand
    Subcommand("branch").
        Description("Manage branches").

        Subcommand("list").
            Description("List all branches").
            Handler(func(ctx *cf.Context) error {
                fmt.Println("Listing branches")
                return nil
            }).
            Done().

        Done().

    Build()
```

### Usage Examples

```bash
# Execute nested subcommands
gitlike remote add origin https://github.com/user/repo.git
gitlike remote remove origin
gitlike branch list

# Root globals work at any position
gitlike -verbose remote add origin https://...
gitlike remote add -verbose origin https://...
gitlike remote -verbose add origin https://...

# Help at each level
gitlike -help                # Shows: remote, branch
gitlike remote -help         # Shows: add, remove
gitlike remote add -help     # Shows: specific flags for 'add'
```

### Handler Patterns

#### Pattern 1: Single Handler with Path Checking

Use `IsSubcommandPath()` to route within one handler:

```go
Handler(func(ctx *cf.Context) error {
    switch {
    case ctx.IsSubcommandPath("remote", "add"):
        return handleRemoteAdd(ctx)
    case ctx.IsSubcommandPath("remote", "remove"):
        return handleRemoteRemove(ctx)
    case ctx.IsSubcommandPath("branch", "list"):
        return handleBranchList(ctx)
    default:
        return fmt.Errorf("unknown subcommand: %v", ctx.SubcommandPath)
    }
})
```

#### Pattern 2: Individual Handlers per Subcommand

Define handlers directly on each nested subcommand:

```go
Subcommand("remote").
    Subcommand("add").
        Handler(func(ctx *cf.Context) error {
            // Handle remote add
            return nil
        }).
        Done().
    Subcommand("remove").
        Handler(func(ctx *cf.Context) error {
            // Handle remote remove
            return nil
        }).
        Done().
    Done()
```

#### Pattern 3: Hybrid Approach

Use individual handlers for leaf nodes, shared handlers for intermediate nodes:

```go
// Shared handler for all 'remote' commands
func handleRemote(ctx *cf.Context) error {
    // Common setup
    verbose := ctx.GetBool("-verbose", false)

    // Dispatch to specific handlers
    switch ctx.SubcommandName() {
    case "add":
        return handleRemoteAdd(ctx, verbose)
    case "remove":
        return handleRemoteRemove(ctx, verbose)
    case "list":
        return handleRemoteList(ctx, verbose)
    default:
        return fmt.Errorf("unknown remote subcommand: %s", ctx.SubcommandName())
    }
}

Subcommand("remote").
    Handler(handleRemote).
    Subcommand("add").Done().
    Subcommand("remove").Done().
    Subcommand("list").Done().
    Done()
```

### Helper Methods

**`IsSubcommandPath(path ...string) bool`**

Check if the current subcommand path exactly matches:

```go
// User runs: gitlike remote add
ctx.IsSubcommandPath("remote", "add")     // true
ctx.IsSubcommandPath("remote")            // false (not exact match)
ctx.IsSubcommandPath("remote", "remove")  // false
```

**`IsSubcommand(name string) bool`**

Check if a subcommand is in the path at any level:

```go
// User runs: gitlike remote add
ctx.IsSubcommand("remote")  // true (remote is in path)
ctx.IsSubcommand("add")     // true (add is in path)
ctx.IsSubcommand("branch")  // false

// User runs: gitlike branch
ctx.IsSubcommand("branch")  // true
```

**`SubcommandName() string`**

Get the leaf (final) subcommand name:

```go
// User runs: gitlike remote add
ctx.SubcommandName()  // "add"

// User runs: gitlike branch
ctx.SubcommandName()  // "branch"

// User runs: gitlike (no subcommand)
ctx.SubcommandName()  // ""
```

### Complete Example

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
        Description("Git-like CLI with nested subcommands").
        Flag("-verbose", "-v").Bool().Global().Help("Verbose output").Done().

        Subcommand("remote").
            Description("Manage remote repositories").

            Subcommand("add").
                Description("Add a remote").
                Flag("-fetch", "-f").Bool().Help("Fetch after adding").Done().
                Handler(handleCommand).
                Done().

            Subcommand("remove").
                Description("Remove a remote").
                Handler(handleCommand).
                Done().

            Subcommand("list").
                Description("List remotes").
                Handler(handleCommand).
                Done().

            Done().

        Subcommand("branch").
            Description("Manage branches").

            Subcommand("list").
                Description("List branches").
                Flag("-all", "-a").Bool().Help("List all branches").Done().
                Handler(handleCommand).
                Done().

            Subcommand("delete").
                Description("Delete a branch").
                Flag("-force", "-f").Bool().Help("Force deletion").Done().
                Handler(handleCommand).
                Done().

            Done().

        Build()

    if err := cmd.Execute(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}

func handleCommand(ctx *cf.Context) error {
    verbose := ctx.GetBool("-verbose", false)

    switch {
    case ctx.IsSubcommandPath("remote", "add"):
        fetch := ctx.GetBool("-fetch", false)
        fmt.Printf("Adding remote (fetch=%v, verbose=%v)\n", fetch, verbose)
        return nil

    case ctx.IsSubcommandPath("remote", "remove"):
        fmt.Printf("Removing remote (verbose=%v)\n", verbose)
        return nil

    case ctx.IsSubcommandPath("remote", "list"):
        fmt.Printf("Listing remotes (verbose=%v)\n", verbose)
        return nil

    case ctx.IsSubcommandPath("branch", "list"):
        all := ctx.GetBool("-all", false)
        fmt.Printf("Listing branches (all=%v, verbose=%v)\n", all, verbose)
        return nil

    case ctx.IsSubcommandPath("branch", "delete"):
        force := ctx.GetBool("-force", false)
        fmt.Printf("Deleting branch (force=%v, verbose=%v)\n", force, verbose)
        return nil

    default:
        return fmt.Errorf("unknown subcommand path: %v", ctx.SubcommandPath)
    }
}
```

### Shell Completion for Nested Subcommands

Completion works automatically at each level:

```bash
$ gitlike <TAB>
remote  branch

$ gitlike remote <TAB>
add  remove  list

$ gitlike remote add <TAB>
-fetch  -f  -verbose  -v  -help  -man
```

### Help for Nested Subcommands

Help is hierarchical:

```bash
# Root help shows top-level subcommands
$ gitlike -help
gitlike v1.0.0 - Git-like CLI with nested subcommands

USAGE:
    gitlike [OPTIONS] <COMMAND>

COMMANDS:
    branch          Manage branches
      delete        Delete a branch
      list          List branches
    remote          Manage remote repositories
      add           Add a remote
      list          List remotes
      remove        Remove a remote

GLOBAL OPTIONS:
    -verbose, -v    Verbose output

# Intermediate level shows nested subcommands
$ gitlike remote -help
gitlike remote - Manage remote repositories

COMMANDS:
    add             Add a remote
    list            List remotes
    remove          Remove a remote

# Leaf level shows specific flags
$ gitlike remote add -help
gitlike remote add - Add a remote

OPTIONS:
    -fetch, -f      Fetch after adding
    -verbose, -v    Verbose output (global)
```

### Migration from v2.0 to v2.1

**Breaking Change**: `Context.Subcommand` (string) → `Context.SubcommandPath` ([]string)

**Before (v2.0)**:
```go
if ctx.Subcommand == "query" {
    // Handle query
}
```

**After (v2.1)**:
```go
// For simple subcommands
if ctx.IsSubcommand("query") {
    // Handle query
}

// For nested subcommands
if ctx.IsSubcommandPath("remote", "add") {
    // Handle remote add
}

// Or access directly
if len(ctx.SubcommandPath) == 2 && ctx.SubcommandPath[0] == "remote" && ctx.SubcommandPath[1] == "add" {
    // Handle remote add
}
```

See [docs/MIGRATION_v2.0_to_v2.1.md](../docs/MIGRATION_v2.0_to_v2.1.md) for complete migration guide.

### Best Practices for Nested Subcommands

#### 1. Keep Hierarchies Shallow

Prefer 2-3 levels maximum for usability:

```bash
# Good - 2 levels
gitlike remote add

# Good - 3 levels (if necessary)
docker container network connect

# Avoid - too deep
app level1 level2 level3 level4  # Hard to remember
```

#### 2. Use Intermediate Nodes for Grouping Only

Intermediate subcommands should not have positional arguments - only flags:

```go
// Good - leaf nodes have positionals
Subcommand("remote").
    Subcommand("add").
        Handler(func(ctx *cf.Context) error {
            // Positionals: name, url
            return nil
        }).
        Done().
    Done()

// Avoid - intermediate node with positionals
Subcommand("remote").
    Handler(func(ctx *cf.Context) error {
        // Positionals here conflict with nested subcommand names
        return nil
    }).
    Subcommand("add").Done().
    Done()
```

#### 3. Provide Help at All Levels

Ensure each level has a description:

```go
Subcommand("remote").
    Description("Manage remote repositories").  // Important!
    Subcommand("add").
        Description("Add a remote").            // Important!
        Done().
    Done()
```

#### 4. Use Consistent Naming

Follow naming patterns from popular CLIs:

```bash
# git-style
app remote add/remove/list
app branch create/delete/list

# docker-style
app container start/stop/list
app image build/push/pull

# kubectl-style
app get/describe/delete resource
```

#### 5. Root Globals Work Everywhere

Root global flags can appear before or after any subcommand level:

```bash
app -verbose remote add ...
app remote -verbose add ...
app remote add -verbose ...
# All equivalent!
```

### Best Practices for Subcommands

#### 1. Scope Selection

- **Root Global**: App-wide settings that apply everywhere
  - `-verbose`, `-config`, `-debug`
- **Subcommand Global**: Settings that affect the whole subcommand
  - `-output`, `-format`, `-timeout`
- **Subcommand Local**: Clause-specific options
  - `-filter`, `-sort`, `-limit`

#### 2. Handler Design

```go
Handler(func(ctx *cf.Context) error {
    // 1. Extract globals first
    verbose := ctx.GlobalFlags["-verbose"].(bool)

    // 2. Validate inputs
    if len(ctx.Clauses) == 0 {
        return fmt.Errorf("at least one clause required")
    }

    // 3. Process clauses
    for _, clause := range ctx.Clauses {
        // Process each clause
    }

    // 4. Return error or nil
    return nil
})
```

#### 3. Error Handling

```go
// Don't print errors in handler - let Execute() handle it
Handler(func(ctx *cf.Context) error {
    if err := validate(input); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    return nil
})

// In main()
if err := cmd.Execute(); err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    os.Exit(1)
}
```

#### 4. Testing

Test your subcommands with various flag positions:

```bash
# Root globals before subcommand
$ myapp -verbose query ...

# Root globals after subcommand
$ myapp query -verbose ...

# Mixed positions
$ myapp -verbose query -output file.txt -verbose ...

# Multiple clauses
$ myapp query -filter a eq 1 + -filter b eq 2

# Edge cases
$ myapp query  # No filters
$ myapp -help  # Root help
$ myapp query -help  # Subcommand help
```

### Troubleshooting Subcommands

#### "unknown flag" Error

If you get `unknown flag` errors:

- Check flag scope: Is it Global or Local?
- Root globals must be defined on root command with `.Global()`
- Subcommand globals must be defined on subcommand with `.Global()`
- Local flags are per-clause only

#### Completion Not Working

- Ensure completion script is sourced: `source ~/.bash_completion.d/myapp`
- Regenerate after changes: `myapp -completion-script > ~/.bash_completion.d/myapp`
- Check binary name matches: Use the actual binary name in commands
- Reload shell or run: `source ~/.bashrc`

#### Help Not Showing Subcommands

- Ensure subcommands are defined before `.Build()`
- Check that `.Done()` is called on each subcommand
- Run `myapp -help` for root help, `myapp subcommand -help` for subcommand help

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

**Shorthand Methods** (for simple flags):
```go
.Bool()             // No arguments (presence = true)
.String()           // Single string argument
.Int()              // Single int argument
.Float()            // Single float argument
.StringSlice()      // Accumulate multiple string values
```

**Fluent Arg() API** (for multi-argument flags):
```go
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

**Benefits**:
- ✅ No index errors possible (arguments added in order)
- ✅ Auto-counted
- ✅ Clear visual grouping
- ✅ Type defaults to `ArgString` (most common case)

> **Note**: An index-based API exists for advanced/framework use (e.g., code generators, dynamic schema). Most applications won't need it. See [ARG_API_COMPARISON.md](ARG_API_COMPARISON.md) for details.

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

#### FieldCompleter

**NEW in v3.1.0:** Complete field names from data files (CSV, TSV, JSON, JSONL).

The FieldCompleter reads a data file and extracts field names from the header or first record, providing intelligent field name completion for data processing tools.

**Basic Usage:**

```go
Flag("-input", "-i").
    String().
    Global().
    Required().
    Help("Input data file").
    FilePattern("*.{csv,tsv,json,jsonl}").
    Done().

Flag("-group").
    String().
    FieldsFromFlag("-input").  // References the -input flag
    Help("Field to group by").
    Done()
```

When the user types:
```bash
$ myapp -input data.csv -group <TAB>
# Completes: name, age, salary, department (from data.csv header)
```

**Supported File Formats:**

- **CSV** (`.csv`) - Reads comma-separated header line
- **TSV** (`.tsv`) - Reads tab-separated header line
- **JSONL** (`.jsonl`, `.ndjson`) - Reads first line, extracts object keys
- **JSON** (`.json`) - Handles arrays (first object keys) or single objects

**Multi-Argument Flags:**

Works with multi-argument flags too:

```go
Flag("-sum").
    Arg("FIELD").
        FieldsFromFlag("-input").  // Complete field names
        Done().
    Arg("RESULT").Done().
    Done()
```

**Environment Variable Caching:**

The FieldCompleter caches extracted fields in environment variables for performance and to support pipelines:

```bash
# First command reads file and caches fields
$ ssql -input data.csv -select name,age | \
# Second command uses cached fields (no -input specified)
  ssql -group <TAB>
# Completes: name, age (from cache)
```

**Environment variables set:**
- `AUTOCLI_FIELDS` - Generic "last used" fields
- `AUTOCLI_FIELDS_<filename>` - File-specific cache

This allows completion to work in piped commands where the input file isn't directly specified.

**Fallback Behavior:**

The completer tries multiple strategies in order:

1. **Read from file** - If `-input` flag value is available
2. **File-specific cache** - `AUTOCLI_FIELDS_data_csv`
3. **Generic cache** - `AUTOCLI_FIELDS`
4. **Hint** - Shows `<FIELD>` if all else fails

**Complete Example:**

```go
cmd := cf.NewCommand("datatool").
    Flag("-input", "-i").
        String().
        Global().
        Required().
        Help("Input data file").
        FilePattern("*.{csv,tsv,json,jsonl}").
        Done().

    Flag("-select").
        String().
        FieldsFromFlag("-input").
        Accumulate().
        Help("Fields to select").
        Done().

    Flag("-group").
        String().
        FieldsFromFlag("-input").
        Help("Field to group by").
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
        inputFile, _ := ctx.RequireString("-input")
        groupBy := ctx.GetString("-group", "")

        // Process data...
        // When you read the file, also cache fields for pipelines:
        fields := readFieldsFromFile(inputFile)
        os.Setenv("AUTOCLI_FIELDS", strings.Join(fields, ","))

        return nil
    }).

    Build()
```

**Note:** For best pipeline support, your program should also set `AUTOCLI_FIELDS` when reading data files during execution, not just during completion.

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

### Understanding Multi-Argument Flag Values

When you define a flag with multiple arguments, the parsed value is stored as a **`map[string]interface{}`** where the keys are the argument names you defined.

#### Single Occurrence

For a flag defined with multiple arguments:
```go
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

When user runs: `myapp -filter status eq active`

The value is stored as:
```go
clause.Flags["-filter"] = map[string]interface{}{
    "FIELD":    "status",
    "OPERATOR": "eq",
    "VALUE":    "active",
}
```

Access it in your handler:
```go
if filterVal, ok := clause.Flags["-filter"]; ok {
    filterMap := filterVal.(map[string]interface{})
    field := filterMap["FIELD"].(string)
    operator := filterMap["OPERATOR"].(string)
    value := filterMap["VALUE"].(string)

    fmt.Printf("Filter: %s %s %s\n", field, operator, value)
}
```

#### Multiple Occurrences with Accumulate()

When using `.Accumulate()`, multiple occurrences create a slice of maps:

```go
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
    Accumulate().  // Enable accumulation
    Local().
    Done()
```

User runs: `myapp -filter status eq active -filter age gt 18`

The value is stored as:
```go
clause.Flags["-filter"] = []interface{}{
    map[string]interface{}{
        "FIELD":    "status",
        "OPERATOR": "eq",
        "VALUE":    "active",
    },
    map[string]interface{}{
        "FIELD":    "age",
        "OPERATOR": "gt",
        "VALUE":    "18",
    },
}
```

Access it in your handler:
```go
if filterVal, ok := clause.Flags["-filter"]; ok {
    filters := filterVal.([]interface{})  // Slice of maps
    for _, f := range filters {
        filterMap := f.(map[string]interface{})
        field := filterMap["FIELD"].(string)
        operator := filterMap["OPERATOR"].(string)
        value := filterMap["VALUE"].(string)

        fmt.Printf("Filter: %s %s %s\n", field, operator, value)
    }
}
```

#### Value Types in the Map

The map values are typed according to the argument type you specify:

```go
Flag("-range").
    Arg("START").
        Type(cf.ArgInt).
        Completer(cf.NoCompleter{Hint: "<NUMBER>"}).
        Done().
    Arg("END").
        Type(cf.ArgInt).
        Completer(cf.NoCompleter{Hint: "<NUMBER>"}).
        Done().
    Arg("STEP").
        Type(cf.ArgInt).
        Completer(cf.NoCompleter{Hint: "<NUMBER>"}).
        Done().
    Done()
```

Results in:
```go
rangeMap := clause.Flags["-range"].(map[string]interface{})
start := rangeMap["START"].(int)        // int, not string
end := rangeMap["END"].(int)            // int, not string
step := rangeMap["STEP"].(int)          // int, not string
```

#### Type Assertion Safety

Always use type assertions safely:

```go
// Safe with ok check
if filterVal, ok := clause.Flags["-filter"]; ok {
    if filterMap, ok := filterVal.(map[string]interface{}); ok {
        if field, ok := filterMap["FIELD"].(string); ok {
            // Use field safely
        }
    }
}

// Or with accumulate
if filterVal, ok := clause.Flags["-filter"]; ok {
    if filters, ok := filterVal.([]interface{}); ok {
        for _, f := range filters {
            if filterMap, ok := f.(map[string]interface{}); ok {
                // Process filterMap
            }
        }
    }
}
```

#### Complete Example

```go
Handler(func(ctx *cf.Context) error {
    for i, clause := range ctx.Clauses {
        fmt.Printf("Clause %d:\n", i+1)

        // Handle accumulated multi-arg flag
        if filterVal, ok := clause.Flags["-filter"]; ok {
            // Check if it's accumulated (slice) or single
            switch v := filterVal.(type) {
            case []interface{}:
                // Multiple filters
                for _, f := range v {
                    filterMap := f.(map[string]interface{})
                    fmt.Printf("  Filter: %s %s %s\n",
                        filterMap["FIELD"],
                        filterMap["OPERATOR"],
                        filterMap["VALUE"])
                }
            case map[string]interface{}:
                // Single filter (without Accumulate)
                fmt.Printf("  Filter: %s %s %s\n",
                    v["FIELD"],
                    v["OPERATOR"],
                    v["VALUE"])
            }
        }
    }
    return nil
})
```

### Accumulating Values

See "Understanding Multi-Argument Flag Values" above for complete details on how accumulation works with multi-argument flags

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
    cf "github.com/rosscartlidge/autocli/v3"
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
    cf "github.com/rosscartlidge/autocli/v3"
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
            Arg("FIELD").
                Completer(&cf.StaticCompleter{
                    Options: []string{"status", "age", "role", "email", "name"},
                }).
                Done().
            Arg("OPERATOR").
                Completer(&cf.StaticCompleter{
                    Options: []string{"eq", "ne", "gt", "lt", "gte", "lte", "contains"},
                }).
                Done().
            Arg("VALUE").
                Completer(cf.NoCompleter{Hint: "<VALUE>"}).
                Done().
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
// Good - fluent Arg() API
Flag("-filter").
    Arg("FIELD").Done().
    Arg("OPERATOR").Done().
    Arg("VALUE").Done().
    Done()

// Bad
Flag("-filter").
    Arg("ARG1").Done().
    Arg("ARG2").Done().
    Arg("ARG3").Done().
    Done()
```

### 3. Provide Helpful Hints

```go
// For free-form input (in multi-arg flags)
Arg("VALUE").
    Completer(cf.NoCompleter{Hint: "<VALUE>"}).
    Done()

// For file paths (single-arg flag)
Flag("-input").
    String().
    FilePattern("*.json").  // Shows <*.json> hint when no files
    Done()

// For specific types
Arg("EMAIL").Completer(cf.NoCompleter{Hint: "<EMAIL>"}).Done()
Arg("URL").Completer(cf.NoCompleter{Hint: "<URL>"}).Done()
Arg("PORT").Type(cf.ArgInt).Completer(cf.NoCompleter{Hint: "<NUMBER>"}).Done()
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
    Arg("FIELD").Done().
    Arg("OPERATOR").Done().
    Arg("VALUE").Done().
    Accumulate().  // Creates slice of filter maps
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
    Arg("FIELD").Done().
    Arg("OPERATOR").Done().
    Arg("VALUE").Done().
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

**Simple Arguments**: `.Bool()`, `.String()`, `.Int()`, `.Float()`, `.StringSlice()`

**Multi-Argument API**: `.Arg(name) *ArgBuilder` - Returns ArgBuilder for fluent configuration

**Values**: `.Bind(ptr)`, `.Default(val)`, `.Required()`, `.Accumulate()`

**Validation**: `.Validate(ValidatorFunc)`

**Completion**: `.FilePattern(pattern)`, `.Options(...string)`, `.Completer(c)`, `.CompleterFunc(f)`

**Documentation**: `.Help(string)`

**Visibility**: `.Hidden()`

**Finalize**: `.Done() *CommandBuilder`

> **Note**: Index-based methods (`.Args()`, `.ArgName()`, `.ArgType()`, `.ArgCompleter()`) exist for advanced/framework use only. Most applications won't need them. See [ARG_API_COMPARISON.md](ARG_API_COMPARISON.md).

### Arg Builder

**Type**: `.Type(ArgType) *ArgBuilder` - Set argument type (default: ArgString)

**Completion**: `.Completer(Completer) *ArgBuilder` - Set completer for this argument

**Finalize**: `.Done() *FlagBuilder` - Return to flag builder

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
    Command        *Command
    SubcommandPath []string                  // Full path for nested subcommands (e.g., ["remote", "add"])
    Clauses        []Clause
    GlobalFlags    map[string]interface{}
    RemainingArgs  []string                  // Arguments after -- (literal)
}

type Clause struct {
    Flags     map[string]interface{}
    Separator string
}
```

**Helper Methods** (v2.1.0+):
- `ctx.IsSubcommandPath("remote", "add")` - Check exact path match
- `ctx.IsSubcommand("remote")` - Check if subcommand is in path at any level
- `ctx.SubcommandName()` - Get leaf subcommand name

---

For more examples, see the `examples/` directory in the repository.
