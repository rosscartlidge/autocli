# Removing Bind() Method - Formal Design Document

**Date:** 2025-11-08
**Status:** Design Phase
**Author:** Claude Code

## Table of Contents

1. [Overview](#overview)
2. [Motivation](#motivation)
3. [Problems with Bind()](#problems-with-bind)
4. [Design Goals](#design-goals)
5. [Proposed Alternatives](#proposed-alternatives)
6. [Migration Path](#migration-path)
7. [Deprecation Timeline](#deprecation-timeline)
8. [Implementation Plan](#implementation-plan)
9. [Examples and Comparisons](#examples-and-comparisons)
10. [Breaking Changes](#breaking-changes)
11. [Alternatives Considered](#alternatives-considered)

## Overview

This document proposes deprecating and eventually removing the `Bind()` method from completionflags. The Bind() method allows binding flag values directly to variables, but creates significant architectural and usability problems that conflict with the library's core features.

**Key Decision:** Remove Bind() in favor of explicit value extraction from `Context`, with optional helper methods for convenience.

## Motivation

### The Promise of Bind()

Bind() was likely added to provide convenience similar to the standard library's `flag` package:

```go
// Standard library pattern
var verbose bool
flag.BoolVar(&verbose, "verbose", false, "verbose output")

// completionflags Bind() pattern
var verbose bool
Flag("-verbose").Bind(&verbose).Bool().Done()
```

This seems convenient for simple cases, but completionflags is not a simple flag parser.

### Why completionflags is Different

Completionflags has unique features that make Bind() problematic:

1. **Clause-based parsing** - Multiple groups with OR logic
2. **Multi-argument flags** - Flags with multiple typed arguments
3. **Subcommands** - Distributed command architecture (now with nesting!)
4. **Accumulate()** - Multiple occurrences of the same flag
5. **Global vs Local scoping** - Three-level flag visibility

These features are **incompatible** with the simple variable binding model.

### Current State

Bind() exists today and is documented in examples, but:
- Unclear semantics with clauses
- Confusing lifecycle with subcommands
- Requires global/package state
- Makes testing harder
- Conflicts with idiomatic Go patterns

## Problems with Bind()

### Problem 1: Incompatible with Clause-Based Parsing

**The Core Issue:** Clauses are completionflags' killer feature, but Bind() doesn't work with them.

```go
var age int
Flag("-filter").
    Arg("FIELD").Done().
    Arg("OPERATOR").Done().
    Arg("VALUE").Bind(&age).Done().  // Which clause's value?
    Local().
    Done()

// User runs: myapp -filter age gt 25 + -filter age lt 65
// What is the value of `age` after parsing?
// - First clause's value (25)?
// - Last clause's value (65)?
// - Error because there are multiple?
// - Undefined behavior?
```

**Without Bind() - Clear semantics:**

```go
Flag("-filter").
    Arg("FIELD").Done().
    Arg("OPERATOR").Done().
    Arg("VALUE").Done().
    Local().
    Done()

Handler(func(ctx *cf.Context) error {
    // Process each clause explicitly
    for i, clause := range ctx.Clauses {
        filter := clause.Flags["-filter"].(map[string]interface{})
        field := filter["FIELD"].(string)
        op := filter["OPERATOR"].(string)
        value := filter["VALUE"].(string)

        // Clear what we're doing with each clause
        processFilter(field, op, value)
    }
    return nil
})
```

### Problem 2: Encourages Global State Anti-Pattern

**The Issue:** Bind() requires package-level or global variables.

```go
// BAD: Package-level state
var (
    verbose bool
    input   string
    output  string
    limit   int
)

func main() {
    cmd := cf.NewCommand("myapp").
        Flag("-verbose").Bind(&verbose).Bool().Done().
        Flag("-input").Bind(&input).String().Done().
        Flag("-output").Bind(&output).String().Done().
        Flag("-limit").Bind(&limit).Int().Done().
        Handler(processData).  // Implicitly uses globals
        Build()

    cmd.Execute(os.Args[1:])
}

func processData(ctx *cf.Context) error {
    // Handler depends on external state
    // Not self-contained
    // Can't see what inputs it needs
    if verbose {
        log.Printf("Processing %s -> %s (limit=%d)", input, output, limit)
    }
    return process(input, output, limit)
}
```

**GOOD: Explicit extraction (idiomatic Go):**

```go
func main() {
    cmd := cf.NewCommand("myapp").
        Flag("-verbose").Bool().Done().
        Flag("-input").String().Done().
        Flag("-output").String().Done().
        Flag("-limit").Int().Done().
        Handler(func(ctx *cf.Context) error {
            // All dependencies are explicit and local
            verbose := ctx.GlobalFlags["-verbose"].(bool)
            input := ctx.GlobalFlags["-input"].(string)
            output := ctx.GlobalFlags["-output"].(string)
            limit := ctx.GlobalFlags["-limit"].(int)

            if verbose {
                log.Printf("Processing %s -> %s (limit=%d)", input, output, limit)
            }
            return process(input, output, limit)
        }).
        Build()

    cmd.Execute(os.Args[1:])
}
```

**Why this matters:**

1. **Testability** - Can call handler directly with mock Context
2. **Clarity** - Can see all dependencies in one place
3. **Concurrency** - No shared state to worry about
4. **Modularity** - Handler is self-contained

### Problem 3: Testing Becomes Difficult

```go
// With Bind() - must parse to populate globals
var verbose bool
var input string

func TestProcessing(t *testing.T) {
    cmd := cf.NewCommand("test").
        Flag("-verbose").Bind(&verbose).Bool().Done().
        Flag("-input").Bind(&input).String().Done().
        Handler(myHandler).
        Build()

    // Can't test handler directly - must invoke full parse
    err := cmd.Execute([]string{"-verbose", "-input", "test.txt"})
    if err != nil {
        t.Fatal(err)
    }
    // Now verbose and input are set... but handler already ran
}

// Without Bind() - clean unit tests
func TestProcessing(t *testing.T) {
    ctx := &cf.Context{
        GlobalFlags: map[string]interface{}{
            "-verbose": true,
            "-input":   "test.txt",
        },
        Clauses: []cf.Clause{
            {Flags: map[string]interface{}{}},
        },
    }

    // Direct handler invocation with controlled input
    err := myHandler(ctx)
    if err != nil {
        t.Fatal(err)
    }

    // Can test multiple scenarios without rebuilding command
    ctx.GlobalFlags["-verbose"] = false
    err = myHandler(ctx)
    // ...
}
```

### Problem 4: Unclear Semantics with Accumulate()

```go
var filters []string
Flag("-filter").
    Bind(&filters).
    String().
    Accumulate().
    Done()

// User runs: myapp -filter "*.json" -filter "*.yaml"
// What is the value of filters?
// - []string{"*.json", "*.yaml"}?  (seems logical)
// - "*.yaml" (last value)?
// - Error?

// How is this different from:
var filter string
Flag("-filter").
    Bind(&filter).
    String().
    Accumulate().
    Done()
// Now what happens with multiple values?
```

**Without Bind() - explicit and clear:**

```go
Flag("-filter").
    String().
    Accumulate().
    Done()

Handler(func(ctx *cf.Context) error {
    filterVal := ctx.GlobalFlags["-filter"]

    // Handle both cases explicitly
    switch v := filterVal.(type) {
    case []interface{}:
        // Multiple occurrences
        for _, f := range v {
            filter := f.(string)
            processFilter(filter)
        }
    case string:
        // Single occurrence
        processFilter(v)
    }

    return nil
})
```

### Problem 5: Confusing with Nested Subcommands

```go
var remoteName string

Subcommand("remote").
    Flag("-name").Bind(&remoteName).String().Done().

    Handler(func(ctx *cf.Context) error {
        // remoteName is set here if user specified -name
        showRemote(remoteName)
        return nil
    }).

    Subcommand("add").
        Handler(func(ctx *cf.Context) error {
            // What is remoteName here?
            // - Still set from parent subcommand?
            // - Cleared?
            // - Empty?
            // CONFUSING LIFECYCLE!
            addRemote(remoteName)  // Bug waiting to happen
            return nil
        }).
        Done().

    Done()
```

**Without Bind() - clear scoping:**

```go
Subcommand("remote").
    Flag("-name").String().Done().

    Handler(func(ctx *cf.Context) error {
        name, ok := ctx.GlobalFlags["-name"]
        if !ok {
            listRemotes()
        } else {
            showRemote(name.(string))
        }
        return nil
    }).

    Subcommand("add").
        Handler(func(ctx *cf.Context) error {
            // Clear: this handler only sees flags in ctx
            // No confusion about parent state
            name := ctx.Clauses[0].Positional[0]
            url := ctx.Clauses[0].Positional[1]
            addRemote(name, url)
            return nil
        }).
        Done().

    Done()
```

### Problem 6: Unclear Dependencies

```go
// Which variables does this handler use?
Handler(func(ctx *cf.Context) error {
    // Have to search the entire file to find:
    // - What globals exist
    // - Which are bound
    // - What types they are

    if verbose && debug {
        log.Printf("Processing %s to %s with %d workers",
            input, output, workers)
    }

    return process(input, output, workers, timeout, retries)
})

// vs explicit - all dependencies visible
Handler(func(ctx *cf.Context) error {
    verbose := ctx.GlobalFlags["-verbose"].(bool)
    debug := ctx.GlobalFlags["-debug"].(bool)
    input := ctx.GlobalFlags["-input"].(string)
    output := ctx.GlobalFlags["-output"].(string)
    workers := ctx.GlobalFlags["-workers"].(int)
    timeout := ctx.GlobalFlags["-timeout"].(time.Duration)
    retries := ctx.GlobalFlags["-retries"].(int)

    // Crystal clear what this handler needs
    if verbose && debug {
        log.Printf("Processing %s to %s with %d workers",
            input, output, workers)
    }

    return process(input, output, workers, timeout, retries)
})
```

### Problem 7: Multi-Argument Flags

```go
var field, operator, value string

Flag("-filter").
    Arg("FIELD").Bind(&field).Done().
    Arg("OPERATOR").Bind(&operator).Done().
    Arg("VALUE").Bind(&value).Done().
    Done()

// This is awkward! Multi-arg flags return map[string]interface{}
// but we're binding to three separate variables
// How does this even work?
```

**Without Bind() - natural for multi-arg flags:**

```go
Flag("-filter").
    Arg("FIELD").Done().
    Arg("OPERATOR").Done().
    Arg("VALUE").Done().
    Done()

Handler(func(ctx *cf.Context) error {
    filter := ctx.GlobalFlags["-filter"].(map[string]interface{})
    field := filter["FIELD"].(string)
    operator := filter["OPERATOR"].(string)
    value := filter["VALUE"].(string)

    // Natural structure matches flag definition
})
```

### Summary of Problems

| Problem | Severity | Affects |
|---------|----------|---------|
| Incompatible with clauses | **CRITICAL** | Core feature |
| Global state anti-pattern | **HIGH** | Code quality, testing |
| Testing difficulty | **HIGH** | Maintainability |
| Unclear with Accumulate() | **HIGH** | Correctness |
| Confusing with subcommands | **MEDIUM** | New nested feature |
| Unclear dependencies | **MEDIUM** | Code readability |
| Multi-arg flag awkwardness | **MEDIUM** | Ergonomics |
| Reflection overhead | **LOW** | Performance (negligible) |

## Design Goals

1. **Remove architectural problems** - Eliminate global state encouragement
2. **Improve testability** - Handlers should be unit-testable with mock Context
3. **Maintain clarity** - Make flag dependencies explicit and obvious
4. **Preserve ergonomics** - Provide convenience without compromising design
5. **Gradual migration** - Give users time to migrate existing code
6. **Clear documentation** - Update all examples to show best practices

## Proposed Alternatives

### Alternative 1: Explicit Extraction (Recommended)

**The baseline - always works, always clear:**

```go
Handler(func(ctx *cf.Context) error {
    // Type assertion with explicit types
    verbose := ctx.GlobalFlags["-verbose"].(bool)
    input := ctx.GlobalFlags["-input"].(string)
    limit := ctx.GlobalFlags["-limit"].(int)

    // Safe extraction with existence check
    var format string = "json"  // default
    if f, ok := ctx.GlobalFlags["-format"]; ok && f != nil {
        format = f.(string)
    }

    return process(verbose, input, limit, format)
})
```

**Pros:**
- ✅ Always clear what's happening
- ✅ Works with all features (clauses, accumulate, subcommands)
- ✅ No reflection
- ✅ Type visible at extraction point
- ✅ Easy to test

**Cons:**
- ⚠️ Slightly verbose
- ⚠️ Type assertions can panic if wrong type

### Alternative 2: Type-Safe Helper Methods (NEW)

**Add convenience methods to Context that are explicit and safe:**

```go
// Add these methods to Context type
func (ctx *Context) GetBool(name string, defaultValue bool) bool
func (ctx *Context) GetString(name string, defaultValue string) string
func (ctx *Context) GetInt(name string, defaultValue int) int
func (ctx *Context) GetFloat(name string, defaultValue float64) float64
func (ctx *Context) GetDuration(name string, defaultValue time.Duration) time.Duration

// Required variants (return error if missing)
func (ctx *Context) RequireBool(name string) (bool, error)
func (ctx *Context) RequireString(name string) (string, error)
func (ctx *Context) RequireInt(name string) (int, error)
// etc.
```

**Usage:**

```go
Handler(func(ctx *cf.Context) error {
    // With defaults - never panics
    verbose := ctx.GetBool("-verbose", false)
    input := ctx.GetString("-input", "")
    limit := ctx.GetInt("-limit", 10)
    format := ctx.GetString("-format", "json")

    // Required values - returns error
    output, err := ctx.RequireString("-output")
    if err != nil {
        return err
    }

    return process(verbose, input, output, limit, format)
})
```

**Pros:**
- ✅ Clean and concise
- ✅ Type-safe (returns typed values)
- ✅ Never panics (with Get* variants)
- ✅ No global state
- ✅ Works with all features
- ✅ Self-documenting defaults

**Cons:**
- ⚠️ Still requires naming the flag twice
- ⚠️ One method per type (some code duplication)

### Alternative 3: Struct Extraction Pattern (OPTIONAL)

**For complex commands, use a struct:**

```go
type QueryConfig struct {
    Verbose bool
    Input   string
    Output  string
    Limit   int
    Format  string
}

func (c *QueryConfig) FromContext(ctx *cf.Context) error {
    c.Verbose = ctx.GetBool("-verbose", false)
    c.Input = ctx.GetString("-input", "")
    c.Limit = ctx.GetInt("-limit", 10)
    c.Format = ctx.GetString("-format", "json")

    var err error
    c.Output, err = ctx.RequireString("-output")
    return err
}

Handler(func(ctx *cf.Context) error {
    var cfg QueryConfig
    if err := cfg.FromContext(ctx); err != nil {
        return err
    }

    return processQuery(cfg)
})
```

**Pros:**
- ✅ Groups related configuration
- ✅ Reusable across handlers
- ✅ Easy to test (can construct QueryConfig directly)
- ✅ Clear structure

**Cons:**
- ⚠️ More boilerplate for simple commands
- ⚠️ Only beneficial for complex commands

## Migration Path

### Phase 1: Deprecation (v1.1.0)

1. **Add deprecation notice** to Bind() documentation
2. **Add helper methods** (GetBool, GetString, etc.)
3. **Update all examples** to use explicit extraction or helpers
4. **Add migration guide** to documentation

```go
// Deprecated: Bind() will be removed in v2.0.0.
// Use ctx.GetString(), ctx.GetBool(), etc. instead.
func (fb *FlagBuilder) Bind(ptr interface{}) *FlagBuilder {
    // Keep working but marked deprecated
    fb.spec.Pointer = ptr
    return fb
}
```

### Phase 2: Deprecation Warning (v1.2.0)

1. **Log warning** when Bind() is used (optional, can be disabled)
2. **Provide clear migration examples** in docs

```go
func (fb *FlagBuilder) Bind(ptr interface{}) *FlagBuilder {
    if os.Getenv("COMPLETIONFLAGS_WARN_BIND") != "" {
        fmt.Fprintf(os.Stderr, "Warning: Bind() is deprecated and will be removed in v2.0.0\n")
    }
    fb.spec.Pointer = ptr
    return fb
}
```

### Phase 3: Removal (v2.0.0)

1. **Remove Bind() method entirely**
2. **Remove binding logic from parser**
3. **Clean up FlagSpec** (remove Pointer field)

## Deprecation Timeline

```
v1.0.0 (Current)
├─ Bind() exists and works
├─ Documentation shows Bind() usage
└─ No warnings

v1.1.0 (Deprecation Start) - 2-3 months
├─ Mark Bind() as deprecated in docs
├─ Add helper methods (GetBool, GetString, etc.)
├─ Update all examples to NOT use Bind()
├─ Add migration guide
└─ Bind() still works (no warnings)

v1.2.0 (Deprecation Warning) - 6 months
├─ Optional warning when Bind() is used
├─ Clear error messages pointing to alternatives
└─ Bind() still works

v2.0.0 (Removal) - 12 months
├─ Remove Bind() entirely
├─ Remove binding logic from parser
├─ Clean up internal structures
└─ Breaking change - major version bump
```

## Implementation Plan

### Step 1: Add Helper Methods (1-2 hours)

Add to `flag.go`:

```go
// GetBool retrieves a boolean flag value, returning defaultValue if not found
func (ctx *Context) GetBool(name string, defaultValue bool) bool {
    if v, ok := ctx.GlobalFlags[name]; ok && v != nil {
        if b, ok := v.(bool); ok {
            return b
        }
    }
    return defaultValue
}

// GetString retrieves a string flag value, returning defaultValue if not found
func (ctx *Context) GetString(name string, defaultValue string) string {
    if v, ok := ctx.GlobalFlags[name]; ok && v != nil {
        if s, ok := v.(string); ok {
            return s
        }
    }
    return defaultValue
}

// GetInt retrieves an int flag value, returning defaultValue if not found
func (ctx *Context) GetInt(name string, defaultValue int) int {
    if v, ok := ctx.GlobalFlags[name]; ok && v != nil {
        if i, ok := v.(int); ok {
            return i
        }
    }
    return defaultValue
}

// GetFloat retrieves a float64 flag value, returning defaultValue if not found
func (ctx *Context) GetFloat(name string, defaultValue float64) float64 {
    if v, ok := ctx.GlobalFlags[name]; ok && v != nil {
        if f, ok := v.(float64); ok {
            return f
        }
    }
    return defaultValue
}

// GetDuration retrieves a time.Duration flag value, returning defaultValue if not found
func (ctx *Context) GetDuration(name string, defaultValue time.Duration) time.Duration {
    if v, ok := ctx.GlobalFlags[name]; ok && v != nil {
        if d, ok := v.(time.Duration); ok {
            return d
        }
    }
    return defaultValue
}

// RequireString retrieves a string flag value, returning error if not found
func (ctx *Context) RequireString(name string) (string, error) {
    v, ok := ctx.GlobalFlags[name]
    if !ok || v == nil {
        return "", fmt.Errorf("required flag %s not provided", name)
    }
    s, ok := v.(string)
    if !ok {
        return "", fmt.Errorf("flag %s is not a string", name)
    }
    return s, nil
}

// RequireInt retrieves an int flag value, returning error if not found
func (ctx *Context) RequireInt(name string) (int, error) {
    v, ok := ctx.GlobalFlags[name]
    if !ok || v == nil {
        return 0, fmt.Errorf("required flag %s not provided", name)
    }
    i, ok := v.(int)
    if !ok {
        return 0, fmt.Errorf("flag %s is not an int", name)
    }
    return i, nil
}

// Similar for RequireBool, RequireFloat, RequireDuration...
```

### Step 2: Mark Bind() as Deprecated (30 minutes)

Update documentation in `builder.go`:

```go
// Bind binds the flag value to a variable pointer.
//
// Deprecated: Bind() will be removed in v2.0.0. It encourages global state
// and doesn't work well with clauses, accumulate, or subcommands.
//
// Use Context helper methods instead:
//   ctx.GetBool("-verbose", false)
//   ctx.GetString("-input", "")
//   input, err := ctx.RequireString("-input")
//
// Or use explicit extraction:
//   verbose := ctx.GlobalFlags["-verbose"].(bool)
//
func (fb *FlagBuilder) Bind(ptr interface{}) *FlagBuilder {
    fb.spec.Pointer = ptr
    return fb
}
```

### Step 3: Update All Examples (2-3 hours)

Update every example in:
- `examples/` directory
- `doc/USAGE.md`
- `README.md`
- Code comments

Replace Bind() usage with helper methods.

### Step 4: Write Migration Guide (1-2 hours)

Create `docs/BIND_MIGRATION.md` with before/after examples.

### Step 5: Update Tests (1-2 hours)

Add tests for new helper methods, update existing tests that use Bind().

## Examples and Comparisons

### Example 1: Simple Command

**Before (with Bind):**

```go
var verbose bool
var input string
var output string

func main() {
    cmd := cf.NewCommand("process").
        Flag("-verbose", "-v").Bind(&verbose).Bool().Done().
        Flag("-input", "-i").Bind(&input).String().Required().Done().
        Flag("-output", "-o").Bind(&output).String().Required().Done().
        Handler(process).
        Build()

    cmd.Execute(os.Args[1:])
}

func process(ctx *cf.Context) error {
    if verbose {
        log.Printf("Processing %s -> %s", input, output)
    }
    return doProcess(input, output)
}
```

**After (with helpers):**

```go
func main() {
    cmd := cf.NewCommand("process").
        Flag("-verbose", "-v").Bool().Done().
        Flag("-input", "-i").String().Required().Done().
        Flag("-output", "-o").String().Required().Done().
        Handler(func(ctx *cf.Context) error {
            verbose := ctx.GetBool("-verbose", false)
            input, err := ctx.RequireString("-input")
            if err != nil {
                return err
            }
            output, err := ctx.RequireString("-output")
            if err != nil {
                return err
            }

            if verbose {
                log.Printf("Processing %s -> %s", input, output)
            }
            return doProcess(input, output)
        }).
        Build()

    cmd.Execute(os.Args[1:])
}
```

### Example 2: Command with Clauses

**Before (BROKEN with Bind):**

```go
var field, op, value string

cmd := cf.NewCommand("query").
    Flag("-filter").
        Arg("FIELD").Bind(&field).Done().
        Arg("OP").Bind(&op).Done().
        Arg("VALUE").Bind(&value).Done().
        Local().
        Done().
    Handler(query).
    Build()

func query(ctx *cf.Context) error {
    // BUG: What are field, op, value with multiple clauses?
    // myapp -filter age gt 25 + -filter role eq admin
    fmt.Printf("Filter: %s %s %s\n", field, op, value)  // WRONG!
    return nil
}
```

**After (WORKS correctly):**

```go
cmd := cf.NewCommand("query").
    Flag("-filter").
        Arg("FIELD").Done().
        Arg("OP").Done().
        Arg("VALUE").Done().
        Local().
        Done().
    Handler(func(ctx *cf.Context) error {
        // Correctly process each clause
        for i, clause := range ctx.Clauses {
            if filter, ok := clause.Flags["-filter"]; ok {
                f := filter.(map[string]interface{})
                field := f["FIELD"].(string)
                op := f["OP"].(string)
                value := f["VALUE"].(string)

                fmt.Printf("Clause %d: %s %s %s\n", i+1, field, op, value)
            }
        }
        return nil
    }).
    Build()
```

### Example 3: Subcommands

**Before (confusing lifecycle):**

```go
var verbose bool
var name string

cmd := cf.NewCommand("git").
    Flag("-verbose").Bind(&verbose).Bool().Global().Done().

    Subcommand("remote").
        Flag("-name").Bind(&name).String().Done().
        Handler(listRemote).  // Uses verbose, name

        Subcommand("add").
            Handler(addRemote).  // What are verbose, name here?
            Done().
        Done().

    Build()
```

**After (clear scoping):**

```go
cmd := cf.NewCommand("git").
    Flag("-verbose").Bool().Global().Done().

    Subcommand("remote").
        Flag("-name").String().Done().

        Handler(func(ctx *cf.Context) error {
            verbose := ctx.GetBool("-verbose", false)
            name := ctx.GetString("-name", "")

            if name != "" {
                return showRemote(name, verbose)
            }
            return listRemotes(verbose)
        }).

        Subcommand("add").
            Handler(func(ctx *cf.Context) error {
                verbose := ctx.GetBool("-verbose", false)
                name := ctx.Clauses[0].Positional[0]
                url := ctx.Clauses[0].Positional[1]

                return addRemote(name, url, verbose)
            }).
            Done().
        Done().

    Build()
```

## Breaking Changes

### v2.0.0 Breaking Changes

**Removed:**
- `FlagBuilder.Bind(ptr interface{})` method
- `FlagSpec.Pointer` field
- `FlagSpec.IsSlice` field (was only used for Bind)
- Binding logic in parser.go

**Migration Required:**

Every use of `.Bind(&variable)` must be replaced with explicit extraction:

```go
// Old
var input string
Flag("-input").Bind(&input).String().Done()

// New - Option 1 (helpers)
Flag("-input").String().Done()
Handler(func(ctx *cf.Context) error {
    input := ctx.GetString("-input", "")
    // ...
})

// New - Option 2 (explicit)
Flag("-input").String().Done()
Handler(func(ctx *cf.Context) error {
    input := ctx.GlobalFlags["-input"].(string)
    // ...
})
```

### Compatibility

**v1.x versions:**
- Bind() continues to work
- New helper methods available
- Can migrate gradually

**v2.0.0:**
- Bind() removed
- Must use helpers or explicit extraction
- Major version bump signals breaking change

## Alternatives Considered

### Alternative 1: Keep Bind() but Document Limitations

**Rejected:** Doesn't solve the fundamental problems. Users will continue to be confused by:
- Behavior with clauses
- Lifecycle with subcommands
- Testing difficulties

Keeping it "but documented" leads to continued poor user experience.

### Alternative 2: Make Bind() Work with Clauses

**Rejected:** There's no good semantics. Should we:
- Bind the first clause's value? (Surprising)
- Bind the last clause's value? (Surprising)
- Error if multiple clauses? (Breaks existing code)
- Bind a slice of all values? (Type confusion)

No solution makes sense. The features are fundamentally incompatible.

### Alternative 3: Separate Bind() for Simple vs Complex Commands

**Rejected:** Too confusing. Having two APIs for the same thing:
- `Bind()` for simple commands
- Manual extraction for complex commands

Users won't know which to use when, leading to bugs when they add clauses later.

### Alternative 4: Use Tags/Reflection like encoding/json

**Rejected:** Even more reflection, more magic, harder to debug:

```go
type Config struct {
    Verbose bool   `flag:"-verbose"`
    Input   string `flag:"-input"`
}

var cfg Config
cmd.BindStruct(&cfg)  // Auto-bind based on tags
```

This has all the problems of Bind() plus:
- More reflection
- Less explicit
- Harder to debug
- Doesn't solve clause problem

## Summary

**The Case for Removal:**

1. **Architectural:** Bind() encourages global state anti-pattern
2. **Feature conflict:** Doesn't work with clauses (core feature)
3. **Testability:** Makes unit testing harder
4. **Complexity:** Unclear semantics with accumulate, subcommands
5. **Maintenance:** Simpler library without binding logic

**The Migration Path:**

1. Add helper methods (GetBool, GetString, etc.)
2. Deprecate Bind() with clear messaging
3. Update all documentation and examples
4. Remove in v2.0.0 (12+ months)

**The Result:**

- Clearer, more testable code
- Better alignment with library features
- Idiomatic Go patterns
- Simpler implementation
- Better user experience (fewer surprises)

**Recommendation:** Proceed with deprecation in next minor release (v1.1.0).
