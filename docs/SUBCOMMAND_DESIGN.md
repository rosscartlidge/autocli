# Subcommand Support - Formal Design Document

**Date:** 2025-11-02
**Status:** Design Phase
**Author:** Claude Code

## Table of Contents

1. [Overview](#overview)
2. [Motivation](#motivation)
3. [Design Goals](#design-goals)
4. [Core Concepts](#core-concepts)
5. [API Design](#api-design)
6. [Execution Model](#execution-model)
7. [Shell Completion](#shell-completion)
8. [Help Generation](#help-generation)
9. [Implementation Plan](#implementation-plan)
10. [Examples](#examples)
11. [Edge Cases](#edge-cases)
12. [Alternatives Considered](#alternatives-considered)

## Overview

This document proposes adding first-class subcommand support to completionflags. Subcommands enable a single binary to distribute multiple related commands (e.g., `git clone`, `git commit`, `docker build`, `kubectl get`).

**Key Requirement:** Subcommands MUST support the full clause-based parsing system, allowing complex queries within each subcommand.

## Motivation

### Common Pattern

Many CLI tools use the subcommand pattern:

```bash
git clone <url>              # Simple subcommand
docker build -t name .       # Subcommand with flags
kubectl get pods -n default  # Subcommand with flags and positionals
```

### Current Workaround

Without subcommand support, users must:
1. Parse the first positional argument manually
2. Define all flags for all subcommands in one namespace (conflicts likely)
3. Write custom routing logic
4. Manually implement subcommand-specific help

```go
// Current workaround - manual routing
var subcommand string
Flag("COMMAND").String().Bind(&subcommand).Done()

Handler(func(ctx *cf.Context) error {
    switch subcommand {
    case "clone":
        // Manual flag parsing for clone
    case "commit":
        // Manual flag parsing for commit
    }
})
```

### Why Clauses in Subcommands?

Clauses enable powerful query-like commands within subcommands:

```bash
# Query with OR logic across clauses
kubectl get pods -filter status eq running -filter age lt 1h + -filter namespace eq prod

# Data tool with multiple input sources
datatool query -input file1.tsv -filter col1 eq "value" + -input file2.tsv -filter col2 gt 100

# Complex filtering in subcommands
myapp search -filter type eq bug -priority eq high + -filter type eq feature -priority eq critical
```

Without clause support, these would require multiple command invocations or complex flag syntax.

## Design Goals

1. **Zero Breaking Changes** - Existing commands work exactly as before
2. **Natural Extension** - Subcommands use the same fluent builder API
3. **Full Feature Parity** - Subcommands support all flag features (clauses, multi-arg, completion, etc.)
4. **Isolated Namespaces** - Each subcommand has its own flag namespace
5. **Global Flag Inheritance** - Global flags apply across all subcommands
6. **Automatic Help** - Generate help for root command and each subcommand
7. **Shell Completion** - Tab completion for subcommand names, flags, and arguments
8. **Composability** - Subcommands can be defined independently and combined

## Core Concepts

### 1. Command Hierarchy

```
Root Command (e.g., "myapp")
├── Global Flags (-verbose, -config)
├── Root Handler (optional - runs if no subcommand)
└── Subcommands
    ├── Subcommand 1 (e.g., "query")
    │   ├── Description
    │   ├── Positional Args
    │   ├── Local Flags (per-clause)
    │   ├── Global Flags (inherited from root)
    │   ├── Clause Support (+ and - separators)
    │   └── Handler (receives Context with Clauses)
    └── Subcommand 2 (e.g., "import")
        └── ...
```

### 2. Flag Scoping

Three levels of scope:

1. **Root Global Flags** - Apply to all subcommands (e.g., `-verbose`)
2. **Subcommand Global Flags** - Apply to all clauses within a subcommand
3. **Subcommand Local Flags** - Apply per-clause within a subcommand

```go
cmd := cf.NewCommand("myapp").
    // Root global - applies to ALL subcommands
    Flag("-verbose").Global().Done().

    Subcommand("query").
        // Subcommand global - applies to all clauses in "query"
        Flag("-output").Global().Done().

        // Subcommand local - per-clause in "query"
        Flag("-filter").Local().Done().
```

### 3. Execution Flow

```
User: myapp -verbose query -output out.json -filter a eq 1 + -filter b gt 2

1. Parse root global flags: {-verbose: true}
2. Detect subcommand: "query"
3. Parse subcommand with clauses:
   - Global: {-output: "out.json"}
   - Clause 1: {-filter: map[...]}
   - Clause 2: {-filter: map[...]}
4. Execute subcommand handler with Context
```

### 4. Context Structure

Subcommand handlers receive the same `Context` type as regular handlers:

```go
type Context struct {
    Command      *Command                // Root command
    Subcommand   string                  // Name of subcommand (new field)
    GlobalFlags  map[string]interface{}  // Root + Subcommand global flags
    Clauses      []Clause                // Parsed clauses
    RawArgs      []string                // Original arguments
}

// Handler signature stays the same
type ClauseHandlerFunc func(*Context) error
```

**Key Decision:** We use the existing `Context` type, not a new `SubcommandContext`. This means:
- Same handler signature for root and subcommands
- `GlobalFlags` combines root global + subcommand global flags
- New `Subcommand` field identifies which subcommand is executing
- Full clause support via `Clauses` field

## API Design

### SubcommandBuilder

```go
type SubcommandBuilder struct {
    name          string
    description   string
    author        string
    examples      []Example
    parent        *CommandBuilder
    flags         []*FlagSpec      // All flags (global + local)
    positionals   []*FlagSpec      // Positional arguments
    handler       ClauseHandlerFunc
    separators    []string         // Inherited from parent by default
}

// Creation
func (cb *CommandBuilder) Subcommand(name string) *SubcommandBuilder

// Metadata
func (sb *SubcommandBuilder) Description(desc string) *SubcommandBuilder
func (sb *SubcommandBuilder) Author(author string) *SubcommandBuilder
func (sb *SubcommandBuilder) Example(cmd, desc string) *SubcommandBuilder

// Configuration
func (sb *SubcommandBuilder) Separators(seps ...string) *SubcommandBuilder

// Flags (same API as CommandBuilder)
func (sb *SubcommandBuilder) Flag(names ...string) *FlagBuilder

// Handler
func (sb *SubcommandBuilder) Handler(h ClauseHandlerFunc) *SubcommandBuilder

// Finalize
func (sb *SubcommandBuilder) Done() *CommandBuilder
```

### FlagBuilder Extensions

No changes needed! `FlagBuilder` already supports:
- `.Global()` / `.Local()` - Scoping
- `.Arg(name)` - Multi-argument flags
- `.Bind()`, `.Default()`, `.Required()`, `.Accumulate()` - Value handling
- `.Completer()`, `.FilePattern()`, `.Options()` - Completion
- `.Help()` - Documentation

### CommandBuilder Extensions

```go
type CommandBuilder struct {
    // Existing fields...
    subcommands []*SubcommandBuilder  // NEW: List of subcommands
}

// NEW: Define a subcommand
func (cb *CommandBuilder) Subcommand(name string) *SubcommandBuilder {
    sb := &SubcommandBuilder{
        name:       name,
        parent:     cb,
        separators: cb.separators, // Inherit by default
    }
    cb.subcommands = append(cb.subcommands, sb)
    return sb
}
```

### Command Extensions

```go
type Command struct {
    // Existing fields...
    subcommands map[string]*Subcommand  // NEW: Map of subcommand name -> spec
}

type Subcommand struct {
    Name        string
    Description string
    Author      string
    Examples    []Example
    Flags       []*FlagSpec
    Positionals []*FlagSpec
    Handler     ClauseHandlerFunc
    Separators  []string
}
```

## Execution Model

### Parse Flow

```go
func (c *Command) Execute(args []string) error {
    // 1. Check for built-in flags (-help, -man, -completion-script, -complete)
    if hasBuiltinFlag(args) {
        return c.handleBuiltin(args)
    }

    // 2. Parse root global flags (stop at first non-flag)
    rootGlobalFlags, remaining, err := c.parseRootGlobalFlags(args)
    if err != nil {
        return err
    }

    // 3. Check if first remaining arg is a subcommand
    if len(remaining) > 0 && c.hasSubcommand(remaining[0]) {
        subcommandName := remaining[0]
        subcommand := c.subcommands[subcommandName]

        // 4. Parse subcommand (with clauses)
        ctx, err := c.parseSubcommand(subcommand, rootGlobalFlags, remaining[1:])
        if err != nil {
            return err
        }

        ctx.Subcommand = subcommandName

        // 5. Execute subcommand handler
        return subcommand.Handler(ctx)
    }

    // 6. No subcommand - execute root handler or show help
    if c.handler != nil {
        ctx := &Context{
            GlobalFlags: rootGlobalFlags,
            RawArgs:     remaining,
            Clauses:     nil, // No clauses for root
        }
        return c.handler(ctx)
    }

    // 7. No handler and no subcommand - show help
    fmt.Fprintln(os.Stderr, c.GenerateHelp())
    return nil
}
```

### parseRootGlobalFlags

```go
func (c *Command) parseRootGlobalFlags(args []string) (map[string]interface{}, []string, error) {
    flags := make(map[string]interface{})
    i := 0

    for i < len(args) {
        arg := args[i]

        // Stop at first non-flag
        if !strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "+") {
            break
        }

        // Check if this is a root global flag
        spec := c.getRootGlobalFlag(arg)
        if spec == nil {
            // Unknown flag - might be subcommand flag, stop here
            break
        }

        // Parse flag value
        value, consumed, err := c.parseFlagValue(spec, args[i:])
        if err != nil {
            return nil, nil, err
        }

        flags[spec.Names[0]] = value
        i += consumed
    }

    return flags, args[i:], nil
}
```

### parseSubcommand

```go
func (c *Command) parseSubcommand(subcmd *Subcommand, rootGlobals map[string]interface{}, args []string) (*Context, error) {
    // Use existing parser with subcommand's flags
    tempCmd := &Command{
        Flags:      subcmd.Flags,
        Separators: subcmd.Separators,
    }

    ctx, err := tempCmd.Parse(args)
    if err != nil {
        return nil, err
    }

    // Merge root globals into context
    for k, v := range rootGlobals {
        ctx.GlobalFlags[k] = v
    }

    ctx.Command = c

    return ctx, nil
}
```

### Key Insight

The subcommand parsing **reuses the existing parser**! We construct a temporary `Command` with the subcommand's flags and separators, parse normally, then merge in root globals.

## Shell Completion

### Subcommand Name Completion

When completing the first positional argument (after root globals), suggest subcommands:

```bash
$ myapp <TAB>
query  import  export

$ myapp qu<TAB>
query
```

### Subcommand Flag Completion

After a subcommand is detected, route completion to that subcommand's flags:

```bash
$ myapp query -<TAB>
-filter  -output  -sort  -verbose  # includes root globals

$ myapp query -filter <TAB>
field1  field2  field3  # from -filter's completer
```

### Implementation

```go
func (c *Command) handleCompletion(args []string) {
    // Parse root globals to find where they end
    _, remaining, _ := c.parseRootGlobalFlags(args[:len(args)-1])

    // Check if we're completing the subcommand name
    if len(remaining) == 0 || (len(remaining) == 1 && args[len(args)-1] == remaining[0]) {
        partial := ""
        if len(remaining) == 1 {
            partial = remaining[0]
        }

        // Suggest subcommands
        for name := range c.subcommands {
            if strings.HasPrefix(name, partial) {
                fmt.Println(name)
            }
        }
        return
    }

    // Subcommand is already specified
    subcommandName := remaining[0]
    subcmd, exists := c.subcommands[subcommandName]
    if !exists {
        return // Unknown subcommand
    }

    // Create temporary command for subcommand completion
    tempCmd := &Command{
        Flags:      append(c.rootGlobalFlags(), subcmd.Flags...),  // Merge
        Separators: subcmd.Separators,
    }

    // Complete using subcommand's context
    tempCmd.completeFlags(remaining[1:])
}
```

## Help Generation

### Root Help

Shows overview of all subcommands:

```
NAME
    myapp - Data processing tool

VERSION
    1.0.0

USAGE
    myapp [global-options] <command> [command-options]

COMMANDS
    query       Query data with filters
    import      Import data from files
    export      Export data to various formats

GLOBAL OPTIONS
    -verbose, -v       Enable verbose output
    -config FILE       Configuration file

Run 'myapp <command> -help' for command-specific help.
```

### Subcommand Help

Shows detailed help for a specific subcommand:

```
NAME
    myapp query - Query data with filters

USAGE
    myapp query [options] [<clause-separator> <clause>...]

DESCRIPTION
    Query data using flexible filter expressions. Multiple clauses
    can be combined with OR logic using the '+' separator.

POSITIONAL ARGUMENTS
    INPUT       Input file to query (required)

GLOBAL OPTIONS
    -output, -o FILE   Output file (default: stdout)
    -format FMT        Output format: json, csv, tsv (default: json)

LOCAL OPTIONS (per-clause)
    -filter FIELD OPERATOR VALUE
                       Filter condition. Can specify multiple per clause.
                       Operators: eq, ne, gt, lt, contains

    -sort FIELD        Sort by field

    -limit N           Limit results to N rows

CLAUSE SEPARATORS
    +                  OR logic between clauses
    -                  (reserved)

EXAMPLES
    # Single filter
    myapp query -filter status eq active

    # Multiple filters in one clause (AND)
    myapp query -filter status eq active -filter age gt 18

    # Multiple clauses (OR)
    myapp query -filter status eq active + -filter role eq admin

    # Complex query with sorting
    myapp query -filter type eq bug -sort priority -limit 10

Use 'myapp -help' to see all available commands.
```

### Implementation

```go
func (c *Command) GenerateHelp() string {
    var buf bytes.Buffer

    // If no subcommands, use existing help generation
    if len(c.subcommands) == 0 {
        return c.generateStandardHelp()
    }

    // Root help with subcommand list
    buf.WriteString("NAME\n")
    buf.WriteString(fmt.Sprintf("    %s - %s\n\n", c.Name, c.Description))

    if c.Version != "" {
        buf.WriteString(fmt.Sprintf("VERSION\n    %s\n\n", c.Version))
    }

    buf.WriteString("USAGE\n")
    buf.WriteString(fmt.Sprintf("    %s [global-options] <command> [command-options]\n\n", c.Name))

    buf.WriteString("COMMANDS\n")
    for _, subcmd := range c.subcommands {
        buf.WriteString(fmt.Sprintf("    %-15s %s\n", subcmd.Name, subcmd.Description))
    }
    buf.WriteString("\n")

    // Root global flags
    if len(c.rootGlobalFlags()) > 0 {
        buf.WriteString("GLOBAL OPTIONS\n")
        for _, flag := range c.rootGlobalFlags() {
            buf.WriteString(formatFlag(flag))
        }
        buf.WriteString("\n")
    }

    buf.WriteString(fmt.Sprintf("Run '%s <command> -help' for command-specific help.\n", c.Name))

    return buf.String()
}

func (subcmd *Subcommand) GenerateHelp(cmdName string) string {
    // Similar to Command.GenerateHelp() but for a single subcommand
    // Shows positionals, global flags, local flags, examples, etc.
}
```

### Subcommand Help Invocation

```bash
# These should all show subcommand-specific help:
myapp query -help
myapp query --help
myapp query -h

# This shows root help:
myapp -help
```

Implementation:

```go
func (c *Command) Execute(args []string) error {
    // Check for root help
    if hasHelpFlag(args) && !hasSubcommand(args) {
        fmt.Println(c.GenerateHelp())
        return nil
    }

    // Parse to find subcommand
    _, remaining, _ := c.parseRootGlobalFlags(args)

    if len(remaining) > 0 && c.hasSubcommand(remaining[0]) {
        subcommandName := remaining[0]
        subcommand := c.subcommands[subcommandName]

        // Check for subcommand help
        if hasHelpFlag(remaining[1:]) {
            fmt.Println(subcommand.GenerateHelp(c.Name))
            return nil
        }

        // Execute subcommand...
    }
}
```

## Implementation Plan

### Phase 1: Core Subcommand Support

**Files to Modify:**
- `builder.go`: Add `SubcommandBuilder` type and methods
- `command.go`: Add `Subcommand` type, `subcommands` map
- `command.go`: Add `Subcommand()` method to `CommandBuilder`
- `parser.go`: Add `parseRootGlobalFlags()` and `parseSubcommand()`
- `command.go`: Modify `Execute()` to route to subcommands

**New Files:**
- `subcommand.go`: Core subcommand logic and helpers

**Tests:**
- Basic subcommand execution
- Root global flag parsing
- Subcommand flag isolation
- Multiple subcommands

### Phase 2: Clause Support in Subcommands

**Files to Modify:**
- `parser.go`: Ensure clause parsing works in subcommand context
- `command.go`: Pass separators to subcommand parser

**Tests:**
- Single clause in subcommand
- Multiple clauses with separators
- Local vs global flags in clauses
- Accumulation within clauses

### Phase 3: Shell Completion

**Files to Modify:**
- `completion_script.go`: Add subcommand name completion
- `completion_script.go`: Route to subcommand completion

**Tests:**
- Subcommand name completion
- Root global flag completion before subcommand
- Subcommand flag completion
- Positional completion in subcommands

### Phase 4: Help & Man Pages

**Files to Modify:**
- `help.go`: Add root help with subcommand list
- `help.go`: Add `Subcommand.GenerateHelp()`
- `man.go`: Add subcommand support to man page generation

**Tests:**
- Root help output
- Subcommand help output
- Man page for root and subcommands

### Phase 5: Examples & Documentation

**New Files:**
- `examples/subcommand/main.go`: Working example with subcommands
- `docs/SUBCOMMAND_USAGE.md`: User-facing documentation

**Tests:**
- Full integration test with example app

## Examples

### Example 1: Data Query Tool (with Clauses)

```go
package main

import (
    "fmt"
    "os"
    cf "github.com/rosscartlidge/completionflags"
)

func main() {
    cmd := cf.NewCommand("datatool").
        Version("2.0.0").
        Description("Query and transform tabular data").

        // Root global flags
        Flag("-verbose", "-v").
            Bool().
            Global().
            Help("Enable verbose logging").
            Done().

        Flag("-config").
            String().
            Global().
            Help("Configuration file").
            FilePattern("*.{json,yaml}").
            Done().

        // Subcommand: query
        Subcommand("query").
            Description("Query data with complex filters").
            Example(
                "datatool query -input data.tsv -filter col1 eq value",
                "Simple filter query",
            ).
            Example(
                "datatool query -input data.tsv -filter col1 eq A + -filter col2 gt 100",
                "OR query across clauses",
            ).

            // Positional argument
            Flag("INPUT").
                String().
                Required().
                Global().
                Help("Input TSV file").
                FilePattern("*.tsv").
                Done().

            // Global flags for query subcommand
            Flag("-output", "-o").
                String().
                Global().
                Help("Output file (default: stdout)").
                FilePattern("*.{tsv,csv,json}").
                Done().

            Flag("-format").
                String().
                Global().
                Default("tsv").
                Options("tsv", "csv", "json").
                Help("Output format").
                Done().

            // Local flags (per-clause)
            Flag("-filter").
                Arg("COLUMN").
                    Completer(cf.NoCompleter{Hint: "<COLUMN>"}).
                    Done().
                Arg("OPERATOR").
                    Completer(&cf.StaticCompleter{
                        Options: []string{"eq", "ne", "gt", "lt", "contains"},
                    }).
                    Done().
                Arg("VALUE").
                    Completer(cf.NoCompleter{Hint: "<VALUE>"}).
                    Done().
                Accumulate().
                Local().
                Help("Filter rows by condition").
                Done().

            Flag("-sort").
                String().
                Local().
                Help("Sort by column").
                Completer(cf.NoCompleter{Hint: "<COLUMN>"}).
                Done().

            Flag("-limit").
                Int().
                Local().
                Help("Limit number of rows").
                Done().

            Handler(func(ctx *cf.Context) error {
                input := ctx.GlobalFlags["INPUT"].(string)
                output := ctx.GlobalFlags["-output"]
                verbose := ctx.GlobalFlags["-verbose"].(bool)

                if verbose {
                    fmt.Printf("Querying %s with %d clauses\n", input, len(ctx.Clauses))
                }

                // Process each clause (OR logic)
                for i, clause := range ctx.Clauses {
                    fmt.Printf("Clause %d:\n", i+1)

                    // Get filters (AND logic within clause)
                    if filterVal, ok := clause.Flags["-filter"]; ok {
                        filters := filterVal.([]interface{})
                        for _, f := range filters {
                            fm := f.(map[string]interface{})
                            fmt.Printf("  Filter: %s %s %s\n",
                                fm["COLUMN"], fm["OPERATOR"], fm["VALUE"])
                        }
                    }

                    if sort, ok := clause.Flags["-sort"]; ok {
                        fmt.Printf("  Sort: %s\n", sort)
                    }

                    if limit, ok := clause.Flags["-limit"]; ok {
                        fmt.Printf("  Limit: %d\n", limit)
                    }
                }

                return nil
            }).
            Done().

        // Subcommand: import
        Subcommand("import").
            Description("Import data from various sources").

            Flag("SOURCE").
                String().
                Required().
                Global().
                Help("Source file or URL").
                Done().

            Flag("-type").
                String().
                Global().
                Required().
                Options("csv", "json", "xml").
                Help("Source data type").
                Done().

            Handler(func(ctx *cf.Context) error {
                source := ctx.GlobalFlags["SOURCE"].(string)
                dataType := ctx.GlobalFlags["-type"].(string)

                fmt.Printf("Importing %s as %s\n", source, dataType)
                return nil
            }).
            Done().

        // Root handler (no subcommand)
        Handler(func(ctx *cf.Context) error {
            fmt.Println("Usage: datatool <command> [options]")
            fmt.Println("Run 'datatool -help' for more information")
            return nil
        }).

        Build()

    if err := cmd.Execute(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

Usage:

```bash
# Root help
datatool -help

# Subcommand help
datatool query -help

# Simple query
datatool query data.tsv -filter col1 eq active

# Complex query with OR
datatool -v query data.tsv -filter type eq bug -filter priority eq high + -filter type eq feature

# Import
datatool import data.csv -type csv
```

### Example 2: Git-like Tool

```go
cmd := cf.NewCommand("git").
    Version("2.0.0").

    Subcommand("clone").
        Flag("REPO").String().Required().Global().Done().
        Flag("DIR").String().Global().Done().
        Flag("-depth").Int().Global().Done().
        Handler(cloneHandler).
        Done().

    Subcommand("log").
        Flag("-format").String().Global().Default("oneline").Done().
        Flag("-author").String().Local().Done().  // Filter per clause
        Handler(logHandler).
        Done().

    Build()
```

### Example 3: Docker-like Tool

```go
cmd := cf.NewCommand("docker").
    Subcommand("build").
        Flag("PATH").String().Required().Global().Done().
        Flag("-tag", "-t").String().Global().Accumulate().Done().
        Flag("-file", "-f").String().Global().Done().
        Handler(buildHandler).
        Done().

    Subcommand("run").
        Flag("IMAGE").String().Required().Global().Done().
        Flag("-port", "-p").String().Global().Accumulate().Done().
        Flag("-env", "-e").String().Global().Accumulate().Done().
        Handler(runHandler).
        Done().

    Build()
```

## Edge Cases

### 1. Subcommand Name Conflicts with Flags

```bash
myapp -query  # Is this flag "-query" or subcommand "query"?
```

**Solution:** Subcommands MUST NOT start with `-` or `+`. Enforce at build time.

```go
func (cb *CommandBuilder) Subcommand(name string) *SubcommandBuilder {
    if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "+") {
        panic(fmt.Sprintf("subcommand name cannot start with - or +: %s", name))
    }
    // ...
}
```

### 2. Root Global Flag with Same Name as Subcommand Flag

```bash
myapp -verbose query -verbose  # Two different -verbose flags?
```

**Solution:** Build-time validation - error if names conflict.

```go
func (sb *SubcommandBuilder) Flag(names ...string) *FlagBuilder {
    for _, name := range names {
        if sb.parent.hasRootGlobalFlag(name) {
            panic(fmt.Sprintf("subcommand flag %s conflicts with root global flag", name))
        }
    }
    // ...
}
```

### 3. Subcommand Name Looks Like Positional Argument

```bash
myapp query data.tsv  # Is "query" a subcommand or positional arg?
```

**Solution:** Subcommands are detected BEFORE positional arguments. If a registered subcommand name appears after root globals, it's treated as a subcommand.

**Consequence:** Cannot have a positional argument that matches a subcommand name.

### 4. No Subcommand Specified

```bash
myapp -verbose  # Root global flag but no subcommand
```

**Solution:** Execute root handler if defined, otherwise show help.

### 5. Unknown Subcommand

```bash
myapp unknowncommand -flag value
```

**Solution:** Check if "unknowncommand" is a registered subcommand. If not, either:
- Execute root handler (if defined) treating "unknowncommand" as a positional
- Show error: "unknown subcommand: unknowncommand"

**Recommended:** Show error to avoid confusion.

### 6. Subcommand Help Flag

```bash
myapp -help          # Root help
myapp query -help    # Query subcommand help
```

**Solution:** Detect help flag after parsing root globals and determining subcommand.

### 7. Completion Script for Subcommands

```bash
myapp -completion-script  # Should include all subcommands
```

**Solution:** Generate single completion script that handles both root and all subcommands.

### 8. Man Pages for Subcommands

```bash
myapp -man           # Root man page
myapp query -man     # Query subcommand man page
```

**Solution:** Detect `-man` flag in subcommand context and generate subcommand-specific man page.

## Alternatives Considered

### Alternative 1: Separate Context Type

Create `SubcommandContext` separate from `Context`:

```go
type SubcommandContext struct {
    SubcommandName string
    GlobalFlags    map[string]interface{}  // Root + Subcommand globals
    Clauses        []Clause
}

type SubcommandHandlerFunc func(*SubcommandContext) error
```

**Pros:**
- Clear separation between root and subcommand handlers
- Explicit API

**Cons:**
- Duplicates Context fields
- Different handler signatures complicate API
- Breaks symmetry with rest of library

**Decision:** Rejected. Use existing `Context` with new `Subcommand` field.

### Alternative 2: Nested Commands (Unlimited Depth)

Allow subcommands to have their own subcommands:

```bash
myapp level1 level2 level3 -flag value
```

**Pros:**
- Maximum flexibility
- Handles complex hierarchies

**Cons:**
- Rarely needed in practice
- Significantly more complex implementation
- Harder to understand for users

**Decision:** Rejected. Single-level subcommands cover 99% of use cases. Can be added later if needed.

### Alternative 3: Subcommands Without Clauses

Simplify by removing clause support in subcommands:

**Pros:**
- Simpler implementation
- Clearer mental model

**Cons:**
- Loses major library feature
- Forces complex queries into flags or multiple invocations
- Inconsistent with library's core value proposition

**Decision:** Rejected per user requirement.

### Alternative 4: Positional Subcommand Spec

Treat subcommand as a special positional argument:

```go
cmd := cf.NewCommand("myapp").
    Flag("SUBCOMMAND").
        Subcommand().
        Option("query", queryHandler).
        Option("import", importHandler).
        Done()
```

**Pros:**
- Uses existing positional system
- Minimal new concepts

**Cons:**
- Doesn't allow per-subcommand flags
- Can't inherit properly
- Awkward API

**Decision:** Rejected. Too limiting.

## Open Questions

### Q1: Should root command support clauses?

**Options:**
1. Yes - root handler gets full clause support
2. No - only subcommands get clauses
3. Configurable - allow but don't require

**Recommendation:** Option 1 (Yes). Keep consistency - root command is just a command. If it has local flags, it should support clauses.

### Q2: Should subcommands inherit separators?

**Options:**
1. Always inherit from root
2. Can override per-subcommand
3. Independent by default

**Recommendation:** Option 2. Inherit by default, allow override via `.Separators()`.

### Q3: How to handle subcommand aliases?

**Options:**
1. Not supported initially
2. `.Subcommand("query", "q")` - multiple names
3. Separate `.Alias()` method

**Recommendation:** Option 2 for initial implementation. Simple and consistent with flag aliases.

```go
Subcommand("query", "q").  // "query" or "q" both work
```

### Q4: Should `-help` show all subcommand help?

**Options:**
1. No - only list subcommands, user must drill down
2. Yes - show complete help for all subcommands
3. Configurable

**Recommendation:** Option 1. Long help is overwhelming. Root help shows overview, user drills down for details.

### Q5: Validation - when to check flag conflicts?

**Options:**
1. Build time - panic on conflict
2. Runtime - error on conflict
3. Allow conflicts, use scoping rules

**Recommendation:** Option 1 (build time). Fail fast during development.

## Summary

This design adds first-class subcommand support to completionflags with:

1. **Full clause support** - Each subcommand can use the clause-based parsing system
2. **Clean API** - Fluent builder pattern consistent with existing design
3. **Isolated namespaces** - Each subcommand has its own flags
4. **Global flag inheritance** - Root globals apply to all subcommands
5. **Automatic help** - Generated for root and each subcommand
6. **Shell completion** - For subcommand names, flags, and arguments
7. **Zero breaking changes** - Existing commands continue to work

**Next Steps:**
1. Review and approve this design
2. Implement Phase 1 (core subcommand support)
3. Add tests
4. Continue through remaining phases

---

**End of Design Document**
