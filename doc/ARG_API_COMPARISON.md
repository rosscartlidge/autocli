# Argument Configuration API Comparison

There are two ways to configure multi-argument flags: the **index-based API** and the **fluent Arg() API**.

> **⚠️ Most users should use the fluent Arg() API**. The index-based API is provided for advanced use cases like building frameworks or code generators. If you're building a normal application, skip to the fluent API examples below.

## Index-Based API (Original)

```go
Flag("-filter").
    Args(3).
    ArgName(0, "FIELD").
    ArgName(1, "OPERATOR").
    ArgName(2, "VALUE").
    ArgType(0, cf.ArgString).
    ArgType(1, cf.ArgString).
    ArgType(2, cf.ArgString).
    ArgCompleter(0, &cf.StaticCompleter{
        Options: []string{"status", "age", "role"},
    }).
    ArgCompleter(1, &cf.StaticCompleter{
        Options: []string{"eq", "ne", "gt", "lt"},
    }).
    ArgCompleter(2, cf.NoCompleter{Hint: "<VALUE>"}).
    Done()
```

**Pros:**
- Explicit control over indices
- Can set properties in any order

**Cons:**
- Verbose and repetitive
- Fragile - wrong indices silently ignored
- Easy to forget configuring an argument
- Not obvious which settings go with which argument

## Fluent Arg() API (Recommended)

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

**Pros:**
- ✅ No index errors possible
- ✅ Arguments auto-counted
- ✅ Clear visual grouping
- ✅ More readable
- ✅ Each argument clearly configured
- ✅ Type defaults to ArgString (most common case)

**Cons:**
- Slightly more `.Done()` calls

## When to Use Each

### Use Fluent Arg() API (Recommended)
- ✅ **Use this for all normal application development**
- You have 2+ arguments
- You want type safety and clarity
- You're building a new application

### Use Index-Based API (Advanced/Framework Use Only)
- ⚠️ **Only needed for advanced use cases:**
  - Building frameworks or code generators on top of completionflags
  - CLI argument structure comes from external config/schema with **variable argument counts**
  - Maintaining existing code that uses the index-based API
- Most applications will never need this

**Example Advanced Use Case:**
```go
// Framework that generates CLIs from OpenAPI schemas
func BuildFromSchema(schema OpenAPISchema) *Command {
    argCount := len(schema.Parameters)  // Variable count!

    fb := Flag("-call").Args(argCount)
    for i, param := range schema.Parameters {
        fb.ArgName(i, param.Name)
        fb.ArgType(i, convertType(param.Type))
        fb.ArgCompleter(i, buildCompleter(param))
    }
    return fb.Done()
}
```

If your argument count is **fixed/known at compile time**, use the fluent API instead.

## Examples

### Two-Argument Flag

**Index-Based:**
```go
Flag("-connect").
    Args(2).
    ArgName(0, "HOST").
    ArgName(1, "PORT").
    ArgType(0, cf.ArgString).
    ArgType(1, cf.ArgInt).
    Done()
```

**Fluent:**
```go
Flag("-connect").
    Arg("HOST").
        Done().
    Arg("PORT").
        Type(cf.ArgInt).
        Done().
    Done()
```

### Complex Multi-Argument with Types and Completers

**Index-Based:**
```go
Flag("-range").
    Args(3).
    ArgName(0, "START").
    ArgName(1, "END").
    ArgName(2, "STEP").
    ArgType(0, cf.ArgInt).
    ArgType(1, cf.ArgInt).
    ArgType(2, cf.ArgInt).
    ArgCompleter(0, cf.NoCompleter{Hint: "<NUMBER>"}).
    ArgCompleter(1, cf.NoCompleter{Hint: "<NUMBER>"}).
    ArgCompleter(2, cf.NoCompleter{Hint: "<NUMBER>"}).
    Done()
```

**Fluent:**
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

## ArgBuilder Methods

When using `.Arg(name)`, you get an `ArgBuilder` with these methods:

- `.Type(t ArgType)` - Set argument type (default: ArgString)
- `.Completer(c Completer)` - Set completer
- `.Done()` - Finalize argument and return to FlagBuilder

## Migration Guide

To migrate from index-based to fluent API:

1. Remove the `.Args(count)` call
2. For each argument index 0..count-1:
   - Add `.Arg(name)` with the ArgName value
   - Move `.ArgType(i, type)` → `.Type(type)` 
   - Move `.ArgCompleter(i, completer)` → `.Completer(completer)`
   - Add `.Done()` after each argument

**Before:**
```go
Flag("-cmd").
    Args(2).
    ArgName(0, "ACTION").
    ArgName(1, "TARGET").
    ArgCompleter(0, &cf.StaticCompleter{Options: []string{"start", "stop"}}).
    ArgCompleter(1, &cf.FileCompleter{Pattern: "*.sh"}).
    Done()
```

**After:**
```go
Flag("-cmd").
    Arg("ACTION").
        Completer(&cf.StaticCompleter{Options: []string{"start", "stop"}}).
        Done().
    Arg("TARGET").
        Completer(&cf.FileCompleter{Pattern: "*.sh"}).
        Done().
    Done()
```

## Both APIs Work Together

You can mix both styles in the same command:

```go
cmd := cf.NewCommand("app").
    // Simple flag - use convenience methods
    Flag("-verbose").Bool().Done().
    
    // Single arg flag - use String()
    Flag("-output").String().FilePattern("*.txt").Done().
    
    // Multi-arg flag - use fluent Arg() API
    Flag("-filter").
        Arg("FIELD").Completer(fieldCompleter).Done().
        Arg("OPERATOR").Completer(opCompleter).Done().
        Arg("VALUE").Done().
        Done().
    
    Build()
```

## Recommendation

**Use the fluent Arg() API for all multi-argument flags**. It's safer, clearer, and less error-prone.
