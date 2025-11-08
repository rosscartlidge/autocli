# Migration Guide: v2.0.0 to v2.1.0

This guide helps you migrate your code from autocli v2.0.0 to v2.1.0.

## What's New in v2.1.0

**Nested Subcommands Support** - You can now create multi-level command hierarchies like `git remote add`, `docker container exec`, or `kubectl config set-context`.

This is a **backwards-compatible** feature addition with **one breaking change** for existing subcommand users.

## Breaking Change

### Context.Subcommand → Context.SubcommandPath

**What changed:**
- `Context.Subcommand` (string) → `Context.SubcommandPath` ([]string)

**Why:**
To support nested subcommands, we need to track the full path (e.g., `["remote", "add"]` instead of just `"add"`).

## Migration Steps

### Option 1: Use Helper Methods (Recommended)

We added helper methods to make migration easier. This is the recommended approach:

**Before (v2.0.0):**
```go
func handler(ctx *autocli.Context) error {
    if ctx.Subcommand == "query" {
        // Handle query
    } else if ctx.Subcommand == "insert" {
        // Handle insert
    }
    return nil
}
```

**After (v2.1.0):**
```go
func handler(ctx *autocli.Context) error {
    if ctx.IsSubcommand("query") {
        // Handle query
    } else if ctx.IsSubcommand("insert") {
        // Handle insert
    }
    return nil
}
```

**Migration:** Replace `ctx.Subcommand == "name"` with `ctx.IsSubcommand("name")`

### Option 2: Use SubcommandPath Directly

If you need to access the subcommand path directly:

**Before (v2.0.0):**
```go
func handler(ctx *autocli.Context) error {
    switch ctx.Subcommand {
    case "query":
        return handleQuery(ctx)
    case "insert":
        return handleInsert(ctx)
    default:
        return fmt.Errorf("unknown subcommand: %s", ctx.Subcommand)
    }
}
```

**After (v2.1.0):**
```go
func handler(ctx *autocli.Context) error {
    subcommand := ctx.SubcommandName() // Gets the leaf subcommand
    switch subcommand {
    case "query":
        return handleQuery(ctx)
    case "insert":
        return handleInsert(ctx)
    default:
        return fmt.Errorf("unknown subcommand: %s", subcommand)
    }
}
```

**Migration:** Replace `ctx.Subcommand` with `ctx.SubcommandName()`

### Option 3: Direct Array Access

For advanced use cases:

**Before (v2.0.0):**
```go
if ctx.Subcommand != "" {
    fmt.Printf("Running subcommand: %s\n", ctx.Subcommand)
}
```

**After (v2.1.0):**
```go
if len(ctx.SubcommandPath) > 0 {
    fmt.Printf("Running subcommand: %s\n", strings.Join(ctx.SubcommandPath, " "))
}
```

## Complete Migration Examples

### Example 1: Simple Subcommand Switch

**Before (v2.0.0):**
```go
cmd := autocli.NewCommand("myapp").
    Subcommand("status").Description("Show status").Done().
    Subcommand("start").Description("Start service").Done().
    Handler(func(ctx *autocli.Context) error {
        switch ctx.Subcommand {
        case "status":
            fmt.Println("Status: running")
        case "start":
            fmt.Println("Starting...")
        }
        return nil
    }).
    Build()
```

**After (v2.1.0):**
```go
cmd := autocli.NewCommand("myapp").
    Subcommand("status").Description("Show status").Done().
    Subcommand("start").Description("Start service").Done().
    Handler(func(ctx *autocli.Context) error {
        switch {
        case ctx.IsSubcommand("status"):
            fmt.Println("Status: running")
        case ctx.IsSubcommand("start"):
            fmt.Println("Starting...")
        }
        return nil
    }).
    Build()
```

### Example 2: Conditional Logic

**Before (v2.0.0):**
```go
func handler(ctx *autocli.Context) error {
    if ctx.Subcommand == "migrate" {
        dryRun := ctx.GetBool("-dry-run", false)
        if dryRun {
            fmt.Println("Dry run mode")
        }
        return runMigration(dryRun)
    }
    return fmt.Errorf("unknown command")
}
```

**After (v2.1.0):**
```go
func handler(ctx *autocli.Context) error {
    if ctx.IsSubcommand("migrate") {
        dryRun := ctx.GetBool("-dry-run", false)
        if dryRun {
            fmt.Println("Dry run mode")
        }
        return runMigration(dryRun)
    }
    return fmt.Errorf("unknown command")
}
```

### Example 3: Logging/Debugging

**Before (v2.0.0):**
```go
func handler(ctx *autocli.Context) error {
    log.Printf("Executing subcommand: %s", ctx.Subcommand)
    // ... rest of handler
}
```

**After (v2.1.0):**
```go
func handler(ctx *autocli.Context) error {
    log.Printf("Executing subcommand: %s", ctx.SubcommandName())
    // ... rest of handler
}
```

## New Helper Methods

v2.1.0 adds three helper methods to make working with subcommands easier:

### IsSubcommand(name string) bool
Checks if the first-level subcommand matches the given name.

```go
if ctx.IsSubcommand("query") {
    // Handle query subcommand
}
```

### IsSubcommandPath(names ...string) bool
Checks if the full subcommand path matches. Useful for nested subcommands.

```go
// For nested subcommands (new in v2.1.0)
if ctx.IsSubcommandPath("remote", "add") {
    // Handle "remote add" subcommand
}

// Also works for single-level (backwards compatible)
if ctx.IsSubcommandPath("query") {
    // Handle "query" subcommand
}
```

### SubcommandName() string
Returns the leaf (last) subcommand name, or empty string if no subcommand.

```go
subcommand := ctx.SubcommandName()
switch subcommand {
case "query":
    return handleQuery(ctx)
case "insert":
    return handleInsert(ctx)
}
```

## Using Nested Subcommands (Optional)

If you want to add nested subcommands to your application:

```go
cmd := autocli.NewCommand("myapp").
    Subcommand("remote").
        Description("Manage remotes").

        // Nested subcommand
        Subcommand("add").
            Description("Add a remote").
            Handler(handleCommand).
            Done().

        // Another nested subcommand
        Subcommand("remove").
            Description("Remove a remote").
            Handler(handleCommand).
            Done().

        Done(). // Returns to parent

    Build()
```

Then in your handler:

```go
func handleCommand(ctx *autocli.Context) error {
    switch {
    case ctx.IsSubcommandPath("remote", "add"):
        return handleRemoteAdd(ctx)
    case ctx.IsSubcommandPath("remote", "remove"):
        return handleRemoteRemove(ctx)
    }
    return nil
}
```

## Quick Reference

| v2.0.0 | v2.1.0 (Recommended) | Notes |
|--------|---------------------|-------|
| `ctx.Subcommand == "name"` | `ctx.IsSubcommand("name")` | Cleanest |
| `ctx.Subcommand` | `ctx.SubcommandName()` | Get leaf name |
| `ctx.Subcommand != ""` | `len(ctx.SubcommandPath) > 0` | Check if any subcommand |
| N/A | `ctx.IsSubcommandPath("a", "b")` | Nested subcommands |
| N/A | `ctx.SubcommandPath` | Full path array |

## Checklist

- [ ] Replace all `ctx.Subcommand == "name"` with `ctx.IsSubcommand("name")`
- [ ] Replace all `ctx.Subcommand` reads with `ctx.SubcommandName()`
- [ ] Replace all `ctx.Subcommand != ""` checks with `len(ctx.SubcommandPath) > 0`
- [ ] Test your application with existing subcommands
- [ ] Update tests that reference `ctx.Subcommand`
- [ ] (Optional) Consider adding nested subcommands where appropriate

## FAQ

### Q: Do I need to use nested subcommands?
**A:** No! Nested subcommands are optional. You only need to update how you access the subcommand name in your handlers.

### Q: Will my existing single-level subcommands still work?
**A:** Yes! After updating `ctx.Subcommand` → `ctx.IsSubcommand()` or `ctx.SubcommandName()`, everything works as before.

### Q: What if I don't update my code?
**A:** Your code will fail to compile because `Context.Subcommand` no longer exists. You must migrate to use the new field/methods.

### Q: Can I mix single-level and nested subcommands?
**A:** Yes! You can have some top-level subcommands and others with nesting:
```go
Subcommand("status").Done().  // Single-level
Subcommand("remote").         // Has nested subcommands
    Subcommand("add").Done().
    Done()
```

### Q: How do I check if I'm at the root (no subcommand)?
**A:** Use `len(ctx.SubcommandPath) == 0` or `ctx.SubcommandName() == ""`

## Examples in the Repository

- **Single-level subcommands:** `examples/subcommand/` (updated for v2.1.0)
- **Nested subcommands:** `examples/nested_subcommands/` (new in v2.1.0)

## Getting Help

If you encounter issues during migration:
1. Check the examples in the `examples/` directory
2. Review the full documentation in `doc/USAGE.md`
3. Report issues at https://github.com/rosscartlidge/completionflags/issues

## Summary

The migration from v2.0.0 to v2.1.0 requires **one simple change**: replace `ctx.Subcommand` with helper methods. The recommended approach is:

```diff
- if ctx.Subcommand == "query" {
+ if ctx.IsSubcommand("query") {
      // Handle query
  }
```

This change enables the powerful new nested subcommands feature while keeping your existing code working with minimal modifications.
