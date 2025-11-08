# Nested Subcommands - Formal Design Document

**Date:** 2025-11-08
**Status:** Design Phase
**Author:** Claude Code

## Table of Contents

1. [Overview](#overview)
2. [Motivation](#motivation)
3. [Real-World Examples](#real-world-examples)
4. [Design Goals](#design-goals)
5. [Core Concepts](#core-concepts)
6. [API Design](#api-design)
7. [Flag Scoping](#flag-scoping)
8. [Positional Arguments and Ambiguity](#positional-arguments-and-ambiguity)
9. [Execution Model](#execution-model)
10. [Shell Completion](#shell-completion)
11. [Help Generation](#help-generation)
12. [Implementation Considerations](#implementation-considerations)
13. [Edge Cases](#edge-cases)
14. [Breaking Changes](#breaking-changes)
15. [Alternatives Considered](#alternatives-considered)

## Overview

This document proposes adding support for nested (multi-level) subcommands to completionflags. Nested subcommands allow commands to be organized hierarchically (e.g., `git remote add`, `docker container exec`, `kubectl config set-context`).

**Key Features:**
- Arbitrary nesting depth (though 2-3 levels is most common)
- Full clause-based parsing at leaf nodes
- Clean builder API with proper parent context
- Intelligent completion at every nesting level
- Automatic help generation for entire tree

## Motivation

### Current Limitation

The current implementation supports single-level subcommands only:

```bash
myapp query -filter ...     # ✅ Supported
myapp remote add origin ... # ❌ Not supported (2 levels)
```

Users wanting multi-level commands must:
1. Manually parse subcommand hierarchies
2. Build complex routing logic
3. Manage help generation manually
4. Implement custom completion

### Industry Standard Pattern

Nested subcommands are prevalent in modern CLI tools:

**Git:**
```bash
git remote add <name> <url>
git remote remove <name>
git remote rename <old> <new>
git config get <key>
git config set <key> <value>
git stash push -m "message"
git stash pop
```

**Docker:**
```bash
docker container ls
docker container exec <container> <command>
docker image build -t <tag> .
docker image rm <image>
docker network create <name>
docker network ls
```

**Kubernetes:**
```bash
kubectl config set-context <context>
kubectl config use-context <context>
kubectl config view
kubectl get pods
kubectl describe pod <name>
```

**AWS CLI:**
```bash
aws s3 ls
aws s3 cp <src> <dst>
aws ec2 describe-instances
aws ec2 start-instances
```

### User Request

A user noticed that completionflags supports subcommands but not nested subcommands, which are common in real-world CLIs. They want this feature to build more complex, well-organized applications.

## Real-World Examples

### Example 1: Git-Style Remote Management

```go
cmd := cf.NewCommand("mygit").
    Version("1.0.0").

    // Root global flag (available everywhere)
    Flag("-config", "-c").
        String().
        Global().
        Help("Config file path").
        Done().

    // Top-level subcommand: "remote"
    Subcommand("remote").
        Description("Manage remote repositories").

        // Flag for "remote" level only (like git remote -v)
        Flag("-verbose", "-v").
            Bool().
            Help("Show remote URLs").
            Done().

        // Nested subcommand: "remote add"
        Subcommand("add").
            Description("Add a new remote").

            Flag("-fetch", "-f").
                Bool().
                Help("Fetch after adding").
                Done().

            Handler(func(ctx *cf.Context) error {
                // ctx.SubcommandPath = ["remote", "add"]
                name := ctx.Clauses[0].Positional[0]
                url := ctx.Clauses[0].Positional[1]
                fetch := ctx.GlobalFlags["-fetch"].(bool)

                fmt.Printf("Adding remote %s -> %s (fetch=%v)\n", name, url, fetch)
                return nil
            }).
            Done(). // Return to "remote" subcommand

        // Nested subcommand: "remote remove"
        Subcommand("remove").
            Description("Remove a remote").

            Handler(func(ctx *cf.Context) error {
                name := ctx.Clauses[0].Positional[0]
                fmt.Printf("Removing remote %s\n", name)
                return nil
            }).
            Done(). // Return to "remote" subcommand

        // Handler for "remote" with no nested subcommand (e.g., "remote -v")
        Handler(func(ctx *cf.Context) error {
            verbose := false
            if v, ok := ctx.GlobalFlags["-verbose"]; ok && v != nil {
                verbose = v.(bool)
            }

            if verbose {
                fmt.Println("Listing remotes with URLs...")
            } else {
                fmt.Println("Listing remotes...")
            }
            return nil
        }).
        Done(). // Return to root command

    Build()
```

**Usage:**
```bash
mygit remote                         # List remotes
mygit remote -v                      # List remotes with URLs (uses "remote" flag)
mygit remote -help                   # Help for "remote" subcommand
mygit remote add origin <url>        # Add remote
mygit remote add -f origin <url>     # Add and fetch (uses "add" flag)
mygit remote add -help               # Help for "remote add"
mygit -config cfg remote add <url>   # With root global flag
mygit remote remove origin           # Remove remote
```

### Example 2: Docker-Style Container Management

```go
cmd := cf.NewCommand("mycontainer").
    Version("2.0.0").

    Subcommand("container").
        Description("Manage containers").

        Subcommand("ls").
            Description("List containers").

            Flag("-all", "-a").
                Bool().
                Help("Show all containers").
                Done().

            Handler(func(ctx *cf.Context) error {
                all := ctx.GlobalFlags["-all"].(bool)
                fmt.Printf("Listing containers (all=%v)\n", all)
                return nil
            }).
            Done().

        Subcommand("exec").
            Description("Execute command in container").

            Flag("-interactive", "-i").
                Bool().
                Help("Keep STDIN open").
                Done().

            Flag("-tty", "-t").
                Bool().
                Help("Allocate pseudo-TTY").
                Done().

            Handler(func(ctx *cf.Context) error {
                container := ctx.Clauses[0].Positional[0]
                command := ctx.Clauses[0].Positional[1:]

                interactive := ctx.GlobalFlags["-interactive"].(bool)
                tty := ctx.GlobalFlags["-tty"].(bool)

                fmt.Printf("Exec in %s: %v (i=%v, t=%v)\n",
                    container, command, interactive, tty)
                return nil
            }).
            Done().

        Done().

    Subcommand("image").
        Description("Manage images").

        Subcommand("ls").
            Description("List images").
            Handler(func(ctx *cf.Context) error {
                fmt.Println("Listing images...")
                return nil
            }).
            Done().

        Subcommand("build").
            Description("Build an image").

            Flag("-tag", "-t").
                String().
                Help("Tag for image").
                Done().

            Handler(func(ctx *cf.Context) error {
                tag := ctx.GlobalFlags["-tag"].(string)
                path := ctx.Clauses[0].Positional[0]
                fmt.Printf("Building image %s from %s\n", tag, path)
                return nil
            }).
            Done().

        Done().

    Build()
```

**Usage:**
```bash
mycontainer container ls              # List containers
mycontainer container ls -all         # List all containers
mycontainer container exec <id> bash  # Execute in container
mycontainer image ls                  # List images
mycontainer image build -t name .     # Build image
```

### Example 3: Config Management with Clauses

```go
cmd := cf.NewCommand("myapp").
    Version("1.0.0").

    Subcommand("config").
        Description("Manage configuration").

        Subcommand("search").
            Description("Search config with filters").

            // Multi-argument flag with clauses
            Flag("-filter").
                Arg("KEY").
                    Completer(&cf.StaticCompleter{
                        Options: []string{"region", "env", "tier"},
                    }).
                    Done().
                Arg("OP").
                    Completer(&cf.StaticCompleter{
                        Options: []string{"eq", "ne", "contains"},
                    }).
                    Done().
                Arg("VALUE").
                    Completer(cf.NoCompleter{Hint: "<VALUE>"}).
                    Done().
                Local(). // Per-clause
                Accumulate().
                Done().

            Handler(func(ctx *cf.Context) error {
                // Support for OR logic with clauses
                for i, clause := range ctx.Clauses {
                    fmt.Printf("Clause %d (OR):\n", i+1)

                    if filters, ok := clause.Flags["-filter"]; ok {
                        // Handle accumulated filters (AND within clause)
                        for _, filter := range filters.([]interface{}) {
                            f := filter.(map[string]interface{})
                            fmt.Printf("  Filter: %s %s %s (AND)\n",
                                f["KEY"], f["OP"], f["VALUE"])
                        }
                    }
                }
                return nil
            }).
            Done().

        Subcommand("set").
            Description("Set configuration value").
            Handler(func(ctx *cf.Context) error {
                key := ctx.Clauses[0].Positional[0]
                value := ctx.Clauses[0].Positional[1]
                fmt.Printf("Setting %s = %s\n", key, value)
                return nil
            }).
            Done().

        Done().

    Build()
```

**Usage:**
```bash
# Simple nested command
myapp config set debug true

# Complex clause-based search
myapp config search -filter region eq us-east -filter env eq prod + -filter tier eq critical

# Help at different levels
myapp config -help
myapp config search -help
```

## Design Goals

1. **Backward Compatibility** - Existing single-level subcommands work unchanged
2. **Natural Nesting** - Use same `.Subcommand()` method recursively
3. **Clear Parent Context** - `.Done()` always returns to parent (Command or Subcommand)
4. **Full Feature Support** - All flag types, clauses, completion work at every level
5. **Automatic Help** - Generate hierarchical help automatically
6. **Path Tracking** - Context knows full subcommand path (e.g., `["remote", "add"]`)
7. **Flexible Handlers** - Intermediate subcommands can have handlers (e.g., `git remote` lists remotes)
8. **Completion Intelligence** - Tab completion at every nesting level

## Core Concepts

### 1. Subcommand Tree Structure

```
Root Command ("myapp")
├── Root Global Flags (-verbose, -config)
├── Root Handler (optional)
└── Subcommands
    ├── "remote" (intermediate node)
    │   ├── Subcommand Flags
    │   ├── Handler (optional - for "myapp remote")
    │   └── Nested Subcommands
    │       ├── "add" (leaf node)
    │       │   ├── Flags
    │       │   └── Handler (required)
    │       └── "remove" (leaf node)
    │           ├── Flags
    │           └── Handler (required)
    └── "config" (intermediate node)
        └── Nested Subcommands
            ├── "get" (leaf node)
            └── "set" (leaf node)
```

### 2. Path-Based Execution

When user runs: `myapp remote add origin https://...`

1. Parse: `myapp` (root)
2. Find subcommand: `remote`
3. Find nested subcommand: `add`
4. Execute handler at leaf: `remote add` handler
5. Context contains: `SubcommandPath = ["remote", "add"]`

### 3. Intermediate vs Leaf Handlers

**Intermediate subcommand with handler:**
```bash
git remote       # Calls "remote" handler (lists remotes)
git remote add   # Calls "remote add" handler (adds remote)
```

**Intermediate subcommand without handler:**
```bash
docker container       # Shows help (no handler defined)
docker container ls    # Calls "container ls" handler
```

## API Design

### Data Structure Changes

```go
// Subcommand struct - ADD nested subcommands
type Subcommand struct {
    Name              string
    Description       string
    Author            string
    Examples          []Example
    Flags             []*FlagSpec
    Handler           ClauseHandlerFunc
    Separators        []string
    ClauseDescription string

    // NEW: Support nested subcommands
    Subcommands       map[string]*Subcommand  // Nested subcommands
}

// Context struct - UPDATE to track full path
type Context struct {
    Command        *Command
    SubcommandPath []string  // Changed from: Subcommand string
    Clauses        []Clause
    GlobalFlags    map[string]interface{}
    RemainingArgs  []string
    RawArgs        []string
    deferredValues map[string]*deferredValue
}
```

### Builder API

**Key insight:** `SubcommandBuilder.parent` needs to support both `*CommandBuilder` and `*SubcommandBuilder`.

**Solution: Parent Interface**

```go
// Parent interface for subcommands
type SubcommandParent interface {
    addSubcommand(name string, subcmd *Subcommand)
    getRootGlobalFlags() []*FlagSpec
    getCommandName() string
}

// CommandBuilder implements SubcommandParent
func (cb *CommandBuilder) addSubcommand(name string, subcmd *Subcommand) {
    cb.cmd.subcommands[name] = subcmd
}

func (cb *CommandBuilder) getRootGlobalFlags() []*FlagSpec {
    return cb.rootGlobalFlags()
}

func (cb *CommandBuilder) getCommandName() string {
    return cb.cmd.name
}

// SubcommandBuilder ALSO implements SubcommandParent (for nesting)
func (sb *SubcommandBuilder) addSubcommand(name string, subcmd *Subcommand) {
    if sb.subcmd.Subcommands == nil {
        sb.subcmd.Subcommands = make(map[string]*Subcommand)
    }
    sb.subcmd.Subcommands[name] = subcmd
}

func (sb *SubcommandBuilder) getRootGlobalFlags() []*FlagSpec {
    // Delegate to actual parent
    return sb.parent.getRootGlobalFlags()
}

func (sb *SubcommandBuilder) getCommandName() string {
    return sb.parent.getCommandName()
}

// SubcommandBuilder updated
type SubcommandBuilder struct {
    name   string
    parent SubcommandParent  // Can be CommandBuilder OR SubcommandBuilder
    subcmd *Subcommand
}

// NEW: SubcommandBuilder can create nested subcommands
func (sb *SubcommandBuilder) Subcommand(name string) *SubcommandBuilder {
    // Validate
    if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "+") {
        panic(fmt.Sprintf("subcommand name cannot start with - or +: %s", name))
    }

    // Initialize nested subcommands map
    if sb.subcmd.Subcommands == nil {
        sb.subcmd.Subcommands = make(map[string]*Subcommand)
    }

    // Check for duplicates
    if _, exists := sb.subcmd.Subcommands[name]; exists {
        panic(fmt.Sprintf("nested subcommand %q already defined", name))
    }

    // Create nested subcommand builder with THIS subcommand as parent
    nested := &SubcommandBuilder{
        name:   name,
        parent: sb,  // Parent is another SubcommandBuilder!
        subcmd: &Subcommand{
            Name:       name,
            Flags:      []*FlagSpec{},
            Separators: sb.subcmd.Separators, // Inherit from parent
            Examples:   []Example{},
        },
    }

    return nested
}
```

### Example Usage

```go
cmd := cf.NewCommand("myapp").
    Subcommand("remote").           // Returns *SubcommandBuilder (parent = CommandBuilder)
        Subcommand("add").          // Returns *SubcommandBuilder (parent = SubcommandBuilder!)
            Handler(...).
            Done().                 // Returns to "remote" SubcommandBuilder
        Subcommand("remove").
            Handler(...).
            Done().                 // Returns to "remote" SubcommandBuilder
        Handler(...).               // Handler for "remote" itself
        Done().                     // Returns to CommandBuilder
    Build()
```

**Type flow:**
```
CommandBuilder
    .Subcommand("remote") → SubcommandBuilder (parent = CommandBuilder)
        .Subcommand("add") → SubcommandBuilder (parent = SubcommandBuilder)
            .Done() → SubcommandBuilder ("remote")
        .Done() → CommandBuilder
```

## Flag Scoping

### Challenge: Multiple Nesting Levels

With nested subcommands, we need clear rules about flag visibility:
- Root globals
- Level-1 subcommand flags
- Level-2 subcommand flags
- Level-N subcommand flags
- Clause locals

### Proposed Scoping Model: **Level-Local Flags**

Each level of the subcommand hierarchy can define its own flags, but they are **local to that level only** (not inherited by nested subcommands).

**Flag Scoping Rules:**

1. **Root global flags** - Available EVERYWHERE (all subcommands, all nesting levels)
   - Can be specified before, after, or between subcommand names
   - Applied to the entire command invocation

2. **Subcommand flags** - Available ONLY when calling that specific subcommand level
   - NOT inherited by nested subcommands
   - Only accessible in that level's handler

3. **Clause local flags** - Per-clause (as current implementation)

### Example: Real Git Behavior

```bash
# Git's actual behavior
git -C /path remote -v           # -C is root global, -v is "remote" flag
git remote -v                    # -v flag belongs to "remote" command
git remote add origin <url>      # "add" does NOT inherit -v flag
git remote add -v origin <url>   # ERROR: -v not recognized by "add"
```

### API Example

```go
cmd := cf.NewCommand("myapp").
    // Root global - available EVERYWHERE
    Flag("-config", "-c").
        String().
        Global().  // Root global
        Help("Config file path").
        Done().

    Subcommand("remote").
        Description("Manage remotes").

        // Flag for "remote" level ONLY (not inherited by "add")
        Flag("-verbose", "-v").
            Bool().
            Help("Show remote URLs").
            Done().

        // Handler for "myapp remote" or "myapp remote -v"
        Handler(func(ctx *cf.Context) error {
            // This handler CAN access:
            // - Root global "-config" (if specified)
            // - Level flag "-verbose"

            verbose := false
            if v, ok := ctx.GlobalFlags["-verbose"]; ok && v != nil {
                verbose = v.(bool)
            }

            listRemotes(verbose)
            return nil
        }).

        Subcommand("add").
            Description("Add a new remote").

            // Flag for "remote add" ONLY
            Flag("-fetch", "-f").
                Bool().
                Help("Fetch after adding").
                Done().

            Handler(func(ctx *cf.Context) error {
                // This handler CAN access:
                // - Root global "-config" (if specified)
                // - Level flag "-fetch"
                //
                // This handler CANNOT access:
                // - Parent flag "-verbose" (belongs to "remote" level)

                fetch := false
                if f, ok := ctx.GlobalFlags["-fetch"]; ok && f != nil {
                    fetch = f.(bool)
                }

                name := ctx.Clauses[0].Positional[0]
                url := ctx.Clauses[0].Positional[1]
                addRemote(name, url, fetch)
                return nil
            }).
            Done().

        Done().

    Build()
```

### Usage Examples

**Valid:**
```bash
myapp -config cfg.json remote -v              # Root global + remote flag
myapp remote -v                                # Remote flag only
myapp remote                                   # No flags
myapp -config cfg.json remote add -fetch ...  # Root global + add flag
myapp remote add -fetch origin <url>          # Add flag
```

**Invalid:**
```bash
myapp remote add -v origin <url>              # ❌ -v belongs to "remote", not "add"
```

### Rationale

1. **Matches Real CLIs** - Git, Docker, kubectl all use level-local flags
   - `git remote -v` has `-v` flag
   - `git remote add` does NOT have `-v` flag

2. **Clear Scope** - Each subcommand level defines its own interface
   - No confusion about flag inheritance
   - Explicit over implicit

3. **Flexibility** - Intermediate handlers can have their own flags
   - `myapp remote -v` lists remotes verbosely
   - `myapp remote add` adds a remote (different interface)

4. **Implementation** - Each level parses its own flags independently
   - No complex inheritance chains
   - Clean separation of concerns

## Positional Arguments and Ambiguity

### The Ambiguity Problem

If intermediate subcommands can accept positional arguments, we face an ambiguity:

```bash
myapp remote add

# Is this:
# A) "remote" subcommand with positional argument "add"
# B) "remote add" nested subcommand
```

This is a fundamental parsing challenge.

### Real CLI Behavior

Let's examine how production CLIs handle this:

**Git:**
```bash
git remote                   # Lists remotes (no positionals)
git remote -v                # Flag for "remote" itself
git remote show origin       # "show" is a nested subcommand
git remote add origin <url>  # "add" is a nested subcommand
```

Git's `remote` command does NOT accept positionals like `git remote <name>`. All operations use nested subcommands.

**Docker:**
```bash
docker container             # Shows help (no positionals)
docker container ls          # Nested subcommand
docker container exec <id>   # Nested subcommand
```

Docker's intermediate commands do NOT accept positionals.

**kubectl:**
```bash
kubectl config               # Shows help
kubectl config view          # Nested subcommand
kubectl config set <key>     # Nested subcommand
```

Kubectl's intermediate commands do NOT accept positionals.

**Pattern:** Intermediate nodes (that have nested subcommands) **do not accept positional arguments**.

### Proposed Solution: **Subcommand Names Take Precedence**

**Rule 1: Parsing Priority**

During parsing, if an argument matches a nested subcommand name, it is **always** treated as a subcommand (never a positional value).

```go
// During parsing
if hasNestedSubcommand(arg) {
    // Route to nested subcommand
} else {
    // Treat as positional or flag
}
```

**Rule 2: Design Constraint**

**Intermediate subcommands (with nested subcommands) should NOT use positional arguments in their handlers.**

This must be documented clearly as a design pattern, though it's difficult to enforce at compile time.

### What Each Level CAN Have

**Root Command:**
- ✅ Root global flags
- ✅ Positionals (if no subcommands, or with `--` separator)
- ✅ Subcommands

**Intermediate Subcommand** (has nested subcommands):
- ✅ **Flags** (local to that level)
- ✅ **Handler** (executes when called directly without nested subcommand)
- ⚠️ **Positionals** - Strongly discouraged, behavior undefined
  - Subcommand names always take precedence
  - Positionals can only be used if they don't conflict with subcommand names
  - Better: use flags instead

**Leaf Subcommand** (no nested subcommands):
- ✅ **Flags** (local to that level)
- ✅ **Positionals** (full support)
- ✅ **Clauses** (full support)
- ✅ **Handler** (required)

### Examples

**Good: Intermediate with flags only**
```go
Subcommand("remote").
    Description("Manage remotes").

    Flag("-verbose", "-v").
        Bool().
        Help("Show URLs").
        Done().

    Handler(func(ctx *cf.Context) error {
        // Uses flag, not positionals
        verbose := ctx.GlobalFlags["-verbose"].(bool)
        listRemotes(verbose)
        return nil
    }).

    Subcommand("add").
        Handler(func(ctx *cf.Context) error {
            name := ctx.Clauses[0].Positional[0]  // ✅ OK - leaf node
            url := ctx.Clauses[0].Positional[1]
            addRemote(name, url)
            return nil
        }).
        Done().

    Done()
```

**Bad: Intermediate with positionals (DON'T DO THIS)**
```go
Subcommand("remote").
    Handler(func(ctx *cf.Context) error {
        // ❌ BAD: What if user types "myapp remote add"?
        // Is ctx.Clauses[0].Positional[0] == "add" (positional)?
        // Or should we route to the "add" subcommand?
        // AMBIGUOUS!

        if len(ctx.Clauses[0].Positional) > 0 {
            name := ctx.Clauses[0].Positional[0]  // ❌ AVOID
            showRemote(name)
        } else {
            listRemotes()
        }
        return nil
    }).

    Subcommand("add").
        // Conflict: "add" as positional vs subcommand
        Handler(...).
        Done().

    Done()
```

**Workaround: Use flags instead of positionals**
```go
Subcommand("remote").
    Flag("-name", "-n").
        String().
        Help("Show specific remote").
        Done().

    Handler(func(ctx *cf.Context) error {
        if name, ok := ctx.GlobalFlags["-name"]; ok {
            showRemote(name.(string))  // ✅ GOOD - explicit flag
        } else {
            listRemotes()
        }
        return nil
    }).

    Subcommand("add").
        Handler(...).
        Done().

    Done()
```

**Usage:**
```bash
myapp remote              # List all remotes
myapp remote -name origin # Show specific remote (no ambiguity!)
myapp remote add origin   # Add remote (unambiguous)
```

### Validation

**Build-time validation (difficult):**

We cannot easily validate positional usage at build time because the handler is a closure. The handler might or might not use `ctx.Clauses[].Positional`.

**Documentation approach:**

1. **Document clearly** in API docs that intermediate subcommands should not use positionals
2. **Provide examples** showing the recommended pattern (flags instead)
3. **Runtime behavior** is well-defined: subcommand names always take precedence

**Possible future enhancement:**

Add a warning or validation method:

```go
Subcommand("remote").
    // Explicitly declare "this intermediate node accepts no positionals"
    NoPositionals().  // Optional marker for clarity

    Subcommand("add").
        // ...
        Done().
    Done()
```

### Summary: Positional Rules

1. **Subcommand names always take precedence** during parsing
2. **Leaf subcommands** can freely use positionals (no ambiguity)
3. **Intermediate subcommands** should avoid positionals (use flags instead)
4. **Documentation** will strongly recommend this pattern
5. **No build-time enforcement** (can't inspect handler closures), but clear runtime behavior

## Execution Model

### Parsing Algorithm

```go
func (cmd *Command) Execute(args []string) error {
    // 1. Parse root global flags
    rootGlobals, remaining := cmd.parseRootGlobalFlags(args)

    // 2. Walk down the subcommand tree
    path := []string{}
    currentNode := cmd
    currentSubcommands := cmd.subcommands
    var leafSubcmd *Subcommand
    var leafParentPath []*Subcommand

    for len(remaining) > 0 {
        if subcmd := currentSubcommands[remaining[0]]; subcmd != nil {
            // Found a subcommand at this level
            path = append(path, remaining[0])
            leafParentPath = append(leafParentPath, subcmd)
            leafSubcmd = subcmd

            // Move down the tree
            currentSubcommands = subcmd.Subcommands
            remaining = remaining[1:]
        } else {
            // Not a subcommand - must be arguments
            break
        }
    }

    // 3. Execute the appropriate handler
    if leafSubcmd != nil {
        // We have a subcommand (possibly nested)

        // Check for help flags
        if hasHelpFlag(remaining) {
            printSubcommandHelp(cmd, leafSubcmd, path)
            return nil
        }

        // Parse subcommand arguments with clauses
        ctx := parseSubcommandArgs(cmd, leafSubcmd, rootGlobals, remaining)
        ctx.SubcommandPath = path

        // Validate and bind
        validate(ctx)
        bind(ctx)

        // Execute handler
        if leafSubcmd.Handler != nil {
            return leafSubcmd.Handler(ctx)
        } else if len(leafSubcmd.Subcommands) > 0 {
            // Intermediate node with no handler - show help
            printSubcommandHelp(cmd, leafSubcmd, path)
            return nil
        }
    }

    // 4. No subcommand - execute root handler or show help
    if cmd.handler != nil {
        ctx := cmd.Parse(args)
        // ... execute root handler
    } else {
        // No handler - show root help
        printHelp(cmd)
    }

    return nil
}
```

### Handler Execution Examples

**Example 1: Leaf handler**
```bash
myapp remote add origin <url>
# Path: ["remote", "add"]
# Executes: "add" handler
# GlobalFlags: {"-verbose": false, "-fetch": false}
```

**Example 2: Intermediate handler**
```bash
myapp remote
# Path: ["remote"]
# Executes: "remote" handler (if defined)
# Or shows help for "remote" (if no handler)
```

**Example 3: Unknown nested command**
```bash
myapp remote unknown
# Error: unknown subcommand "unknown" for "remote"
# Suggestion: Did you mean: add, remove, rename?
```

## Shell Completion

### Recursive Completion Algorithm

```go
func (cmd *Command) completeWithSubcommands(args []string, pos int) ([]string, error) {
    // 1. Parse root global flags
    rootGlobals, remaining := cmd.parseRootGlobalFlags(args)

    // 2. Walk down the subcommand tree as far as possible
    path := []string{}
    currentSubcommands := cmd.subcommands
    var currentSubcmd *Subcommand

    remainingPos := calculatePosition(args, remaining, pos)
    argIndex := 0

    for argIndex < len(remaining) && argIndex < remainingPos {
        if subcmd := currentSubcommands[remaining[argIndex]]; subcmd != nil {
            // Confirmed subcommand
            path = append(path, remaining[argIndex])
            currentSubcmd = subcmd
            currentSubcommands = subcmd.Subcommands
            argIndex++
        } else {
            // Not a subcommand - must be arguments to current subcommand
            break
        }
    }

    // 3. Determine what we're completing
    partial := getPartial(remaining, remainingPos)

    // Case A: Completing at subcommand level (no args yet)
    if argIndex == remainingPos {
        if strings.HasPrefix(partial, "-") {
            // Complete flags for current level
            if currentSubcmd != nil {
                return completeSubcommandFlags(cmd, currentSubcmd, partial)
            }
            return completeRootGlobalFlags(cmd, partial)
        }

        // Complete subcommand names
        if currentSubcommands != nil {
            return completeSubcommandNames(currentSubcommands, partial)
        }
    }

    // Case B: Completing arguments to leaf subcommand
    if currentSubcmd != nil {
        // Create temporary command with merged flags
        tempCmd := createTempCommand(cmd, currentSubcmd)
        subArgs := remaining[argIndex:]
        subPos := remainingPos - argIndex

        ctx := tempCmd.analyzeCompletionContext(subArgs, subPos)

        // Merge root globals
        for k, v := range rootGlobals {
            ctx.GlobalFlags[k] = v
        }

        return tempCmd.executeCompletion(ctx)
    }

    return []string{}, nil
}
```

### Completion Examples

**Level 1: Completing subcommand names**
```bash
myapp rem<TAB>
# Shows: remote
```

**Level 2: Completing nested subcommand names**
```bash
myapp remote a<TAB>
# Shows: add
```

**Level 3: Completing flags**
```bash
myapp remote add -<TAB>
# Shows: -fetch -verbose (root global also shown)
```

**Level 4: Completing flag arguments**
```bash
myapp remote add origin <TAB>
# Shows: file completions or custom completions
```

**Mixed: Root global at any position**
```bash
myapp -v<TAB>
# Shows: -verbose

myapp remote -v<TAB>
# Shows: -verbose (root global available)

myapp remote add -v<TAB>
# Shows: -verbose (root global available)
```

## Help Generation

### Hierarchical Help

**Root help** (`myapp -help`):
```
myapp - Multi-command application

USAGE:
    myapp [OPTIONS] <COMMAND>

GLOBAL OPTIONS:
    -verbose, -v    Enable verbose output

COMMANDS:
    remote          Manage remote repositories
    config          Configuration management

Use "myapp <command> -help" for more information about a command.
```

**Subcommand help** (`myapp remote -help`):
```
myapp remote - Manage remote repositories

USAGE:
    myapp remote [OPTIONS] <COMMAND>
    myapp remote [OPTIONS]

COMMANDS:
    add             Add a new remote
    remove          Remove a remote
    rename          Rename a remote

GLOBAL OPTIONS:
    -verbose, -v    Enable verbose output (root global)

Use "myapp remote <command> -help" for more information.
```

**Leaf help** (`myapp remote add -help`):
```
myapp remote add - Add a new remote

USAGE:
    myapp remote add [OPTIONS] <NAME> <URL>

OPTIONS:
    -fetch, -f      Fetch after adding

GLOBAL OPTIONS:
    -verbose, -v    Enable verbose output (root global)

EXAMPLES:
    myapp remote add origin https://github.com/user/repo.git
    myapp remote add -fetch upstream https://github.com/upstream/repo.git
```

### Man Page Generation

Similar hierarchical structure for man pages:
- Root command gets main man page: `myapp.1`
- Each subcommand level gets its own section
- Cross-references between levels

## Implementation Considerations

### 1. Breaking Changes (See next section)

**Context.Subcommand → Context.SubcommandPath**

This is a breaking change but necessary for path tracking.

**Migration path:**
```go
// Old code
if ctx.Subcommand == "query" {
    // ...
}

// New code (v2.0.0)
if len(ctx.SubcommandPath) > 0 && ctx.SubcommandPath[0] == "query" {
    // ...
}

// Or use helper
if ctx.IsSubcommand("query") {  // NEW helper method
    // ...
}
if ctx.IsSubcommandPath("remote", "add") {  // NEW helper
    // ...
}
```

### 2. Implementation Order

1. **Phase 1: Data structures** (2h)
   - Add `Subcommands map[string]*Subcommand` to Subcommand struct
   - Change `Context.Subcommand` to `SubcommandPath []string`
   - Add helper methods to Context

2. **Phase 2: Builder API** (4-6h)
   - Create `SubcommandParent` interface
   - Implement interface on `CommandBuilder` and `SubcommandBuilder`
   - Add `Subcommand()` method to `SubcommandBuilder`
   - Update `Done()` type returns

3. **Phase 3: Parser** (5-7h)
   - Implement tree walking in `Execute()`
   - Update `parseSubcommand()` to handle nested paths
   - Add validation for intermediate vs leaf flags

4. **Phase 4: Completion** (4-6h)
   - Update `completeWithSubcommands()` for recursive descent
   - Test completion at each nesting level
   - Ensure root globals complete everywhere

5. **Phase 5: Help generation** (3-4h)
   - Update help templates for hierarchical structure
   - Add nested command sections
   - Test help at each level

6. **Phase 6: Testing** (6-8h)
   - Write comprehensive tests for 2-level nesting
   - Test 3-level nesting
   - Edge cases (help flags, unknown commands, etc.)
   - Real-world examples (git, docker style)

7. **Phase 7: Documentation** (2-3h)
   - Update USAGE.md
   - Update examples
   - Write migration guide

### 3. Validation Rules

**Build-time validations:**

1. **Subcommand name validation**
   - Cannot start with `-` or `+`
   - Cannot be empty string
   - No duplicate names at same level

2. **Flag conflict detection**
   - Subcommand flags cannot conflict with root global flags
   - Flag names must be unique within a subcommand level
   - Warning: Sibling subcommands CAN have same flag names (different scopes)

3. **Positional argument warnings** (documentation only)
   - Cannot enforce at build time (handlers are closures)
   - Document clearly: intermediate nodes should not use positionals
   - Runtime behavior: subcommand names always take precedence

**Runtime validations:**

1. **Unknown subcommand detection**
   ```go
   // When user types: myapp remote unknown
   if subcmd := currentSubcommands[arg]; subcmd == nil {
       return fmt.Errorf("unknown command %q for %q", arg, currentPath)
   }
   ```

2. **Flag parsing**
   - Each level parses its own flags independently
   - Root globals merged into context at execution time

3. **Help flag detection**
   - Check for `-help`, `--help`, `-h` at every level
   - Show appropriate help for current level

**Example validation in builder:**

```go
func (sb *SubcommandBuilder) Flag(names ...string) *SubcommandFlagBuilder {
    // Check for conflicts with root global flags
    for _, name := range names {
        if sb.parent.hasRootGlobalFlag(name) {
            panic(fmt.Sprintf("subcommand %q flag %s conflicts with root global flag",
                sb.name, name))
        }
    }

    // Check for duplicate flags at this level
    for _, existing := range sb.subcmd.Flags {
        for _, ename := range existing.Names {
            for _, nname := range names {
                if ename == nname {
                    panic(fmt.Sprintf("duplicate flag %s in subcommand %q",
                        nname, sb.name))
                }
            }
        }
    }

    // Create flag builder...
}
```

**Validation summary:**

| Rule | When | How Enforced |
|------|------|--------------|
| Subcommand name format | Build time | Panic in `Subcommand()` |
| Duplicate subcommands | Build time | Panic in `Subcommand()` |
| Flag conflicts with root globals | Build time | Panic in `Flag()` |
| Duplicate flags at same level | Build time | Panic in `Flag()` |
| Intermediate positionals | Runtime | Documentation only (subcommand precedence) |
| Unknown subcommands | Runtime | Error message with suggestions |
| Required flags | Runtime | Validation after parsing |

### 4. Testing Strategy

**Unit tests:**
- Parser tree walking
- Flag scoping validation
- Completion at each level
- Help generation

**Integration tests:**
- Full git-style example
- Full docker-style example
- Mixed root globals and leaf flags
- Clause support in nested commands

**Edge cases:**
- Help at every level
- Unknown subcommands
- Conflicting flag names
- Empty intermediate handlers

### 4. Performance

**Concern:** Deep nesting could slow parsing

**Analysis:**
- Most CLIs have 2-3 levels max
- Tree walking is O(depth) where depth is small (< 5)
- No performance concern for realistic use

**Optimization:**
- Cache subcommand lookups if needed
- Lazy initialization of completion data

## Edge Cases

### 1. Help Flags at Different Levels

```bash
myapp -help              # Root help
myapp remote -help       # "remote" subcommand help
myapp remote add -help   # "remote add" help
```

**Implementation:** Check for help flags after each level of subcommand parsing.

### 2. Intermediate Node Without Handler

```bash
myapp remote             # What happens?
```

**Solution:** If intermediate node has no handler, show help listing nested subcommands.

### 3. Flag Name Conflicts

```bash
myapp -verbose remote add -verbose <name> <url>
```

**Validation:** Root globals cannot conflict with leaf flags (same as current behavior).

### 4. Completion in Middle of Path

```bash
myapp rem<TAB> add       # Cursor at "rem"
# Should complete: remote
```

**Implementation:** Track cursor position separately from argument list end.

### 5. Unknown Nested Subcommand

```bash
myapp remote unknown
```

**Error message:**
```
Error: unknown command "unknown" for "myapp remote"

Available commands:
  add           Add a new remote
  remove        Remove a remote
  rename        Rename a remote

Use "myapp remote -help" for more information.
```

### 6. Root Handler with Subcommands

```bash
myapp                    # Root has handler AND subcommands
myapp query              # Subcommand
```

**Behavior:** If args are empty, execute root handler. If first arg is subcommand name, route to subcommand.

### 7. Positional Args vs Subcommand Names

```bash
myapp add <file>         # "add" is a positional arg
myapp remote add <name>  # "add" is a subcommand
```

**Resolution:** Subcommands are always checked first. If you need "add" as a positional, use:
```bash
myapp -- add <file>      # Everything after -- is literal
```

## Breaking Changes

### Context Structure Change

**Old (v1.x):**
```go
type Context struct {
    Command     *Command
    Subcommand  string    // Single subcommand name
    Clauses     []Clause
    GlobalFlags map[string]interface{}
    // ...
}
```

**New (v2.0.0):**
```go
type Context struct {
    Command        *Command
    SubcommandPath []string  // Full path: ["remote", "add"]
    Clauses        []Clause
    GlobalFlags    map[string]interface{}
    // ...
}
```

### Migration Guide

**Check for single-level subcommands:**
```go
// v1.x
if ctx.Subcommand == "query" {
    // ...
}

// v2.0.0 - Option 1: Check first element
if len(ctx.SubcommandPath) > 0 && ctx.SubcommandPath[0] == "query" {
    // ...
}

// v2.0.0 - Option 2: Use helper (NEW)
if ctx.IsSubcommand("query") {
    // ...
}
```

**Check for nested subcommands:**
```go
// v2.0.0 - NEW: Check full path
if ctx.IsSubcommandPath("remote", "add") {
    // ...
}

// Or manually
if len(ctx.SubcommandPath) == 2 &&
   ctx.SubcommandPath[0] == "remote" &&
   ctx.SubcommandPath[1] == "add" {
    // ...
}
```

**Helper methods to add:**
```go
// IsSubcommand checks if the command is using a specific first-level subcommand
func (ctx *Context) IsSubcommand(name string) bool {
    return len(ctx.SubcommandPath) > 0 && ctx.SubcommandPath[0] == name
}

// IsSubcommandPath checks if the command matches a specific subcommand path
func (ctx *Context) IsSubcommandPath(names ...string) bool {
    if len(ctx.SubcommandPath) != len(names) {
        return false
    }
    for i, name := range names {
        if ctx.SubcommandPath[i] != name {
            return false
        }
    }
    return true
}

// SubcommandName returns the leaf subcommand name (last element of path)
func (ctx *Context) SubcommandName() string {
    if len(ctx.SubcommandPath) == 0 {
        return ""
    }
    return ctx.SubcommandPath[len(ctx.SubcommandPath)-1]
}
```

### Deprecation Strategy

**Option 1: Hard break in v2.0.0**
- Remove `Context.Subcommand` entirely
- Provide migration guide
- Update all examples

**Option 2: Deprecation period (RECOMMENDED)**
- v1.1.0: Add `SubcommandPath`, keep `Subcommand` (populated with first element)
- v1.1.0: Add deprecation warning in docs
- v2.0.0: Remove `Subcommand` field

**Recommendation:** Use Option 2 with a clear migration guide.

## Alternatives Considered

### Alternative 1: Separate NestedSubcommand() Method

```go
Subcommand("remote").
    NestedSubcommand("add").  // Different method name
```

**Rejected:** Inconsistent API. Better to use the same method recursively.

### Alternative 2: Flat Definition with Paths

```go
Subcommand("remote add").  // Path as string
    Handler(...).
    Done()
```

**Rejected:**
- No hierarchy enforcement
- Hard to generate help
- Ugly completion logic
- Can't have intermediate handlers

### Alternative 3: Full Intermediate Globals

Allow every intermediate subcommand to define Global() flags that apply to nested children.

**Rejected:**
- Too complex
- Confusing scoping rules
- Doesn't match real-world patterns
- Implementation complexity high

### Alternative 4: No Intermediate Handlers

Require all intermediate nodes to have no handler (only route to leaves).

**Rejected:**
- Git uses intermediate handlers (`git remote` lists remotes)
- Docker uses intermediate handlers (`docker container` shows help)
- Too restrictive

## Summary

Nested subcommands are a valuable addition to completionflags that will enable users to build more sophisticated CLI applications matching industry standards (git, docker, kubectl, aws-cli).

**Key decisions:**
- Recursive `Subcommand()` method on both CommandBuilder and SubcommandBuilder
- Parent interface to support both types of parents
- Simplified flag scoping: root globals + leaf flags only
- `Context.SubcommandPath` tracks full path
- Intermediate handlers optional (show help if missing)
- Breaking change in v2.0.0 with deprecation period

**Estimated effort:** 25-35 hours

**Risk level:** Moderate (well-contained changes, clear architecture)

**Value:** High (enables real-world CLI patterns)
