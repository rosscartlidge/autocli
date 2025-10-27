# Positional Arguments Proposal for completionflags

**Status:** Draft
**Author:** AI Assistant
**Date:** 2025-10-28
**Target Version:** v0.2.0

## Overview

This proposal adds first-class support for positional arguments in completionflags, making them as easy to use as flags. Positional arguments are matched **in order of definition** and use the same `.Bind()` API as flags for consistency.

## Motivation

Currently, users must manually filter `ctx.RawArgs` to extract positional arguments:

```go
Handler(func(ctx *cf.Context) error {
    var inputFile string
    for _, arg := range ctx.RawArgs {
        if !strings.HasPrefix(arg, "-") {
            inputFile = arg
            break
        }
    }
    // Use inputFile...
})
```

This approach has several problems:
- **Boilerplate**: Every command needs this filtering logic
- **Error-prone**: Easy to get wrong (what about legitimate args starting with `-`?)
- **No type safety**: Everything is a string
- **No validation**: Can't mark as required or set defaults
- **No help text**: Can't document positional args
- **Inconsistent**: Flags use `.Bind()`, positionals use manual parsing

## Key Principle

**A `Flag()` without a leading `-` defines a positional argument.** Positional flags are matched to command-line arguments in the order they are defined, after all named flags are consumed.

## Basic Syntax

```go
Flag("NAME").
    String().
    Bind(&variable).
    Help("description").
    Done()
```

Key differences from named flags:
- **No leading `-`** in the flag name (e.g., `"FILE"` not `"-file"`)
- Matched **by position**, not by name
- Order of definition determines matching order

## Examples

### Example 1: Single Optional Positional

```go
var inputFile string

cmd := cf.NewCommand("read-csv").
    Flag("FILE").
        String().
        Bind(&inputFile).
        Global().
        Default("").
        Help("Input CSV file (or stdin if not specified)").
        Done().
    Handler(func(ctx *cf.Context) error {
        // inputFile is automatically populated
        records, err := streamv3.ReadCSV(inputFile)
        // ...
    }).
    Build()
```

**Usage:**
```bash
$ read-csv data.csv       # inputFile = "data.csv"
$ read-csv                 # inputFile = ""
```

### Example 2: Multiple Positional Arguments

```go
var source, dest string

cmd := cf.NewCommand("copy").
    Flag("SOURCE").
        String().
        Bind(&source).
        Required().
        Help("Source file").
        Done().
    Flag("DEST").
        String().
        Bind(&dest).
        Required().
        Help("Destination file").
        Done().
    Handler(func(ctx *cf.Context) error {
        // source and dest are both populated
        return copyFile(source, dest)
    }).
    Build()
```

**Usage:**
```bash
$ copy input.txt output.txt   # source = "input.txt", dest = "output.txt"
$ copy input.txt               # Error: DEST required
```

### Example 3: Mixed Flags and Positionals

```go
var verbose bool
var inputFile, outputFile string

cmd := cf.NewCommand("convert").
    Flag("-verbose", "-v").
        Bool().
        Bind(&verbose).
        Help("Verbose output").
        Done().
    Flag("INPUT").
        String().
        Bind(&inputFile).
        Required().
        Help("Input file").
        Done().
    Flag("OUTPUT").
        String().
        Bind(&outputFile).
        Default("output.txt").
        Help("Output file").
        Done().
    Handler(func(ctx *cf.Context) error {
        if verbose {
            fmt.Printf("Converting %s to %s\n", inputFile, outputFile)
        }
        return convert(inputFile, outputFile)
    }).
    Build()
```

**Usage:**
```bash
$ convert input.json                      # verbose=false, input="input.json", output="output.txt"
$ convert -v input.json output.json       # verbose=true, input="input.json", output="output.json"
$ convert input.json -v output.json       # Order of flags doesn't matter, positionals matched in order
```

### Example 4: Variadic Positionals (Multiple Values)

```go
var files []string

cmd := cf.NewCommand("process").
    Flag("FILES").
        StringSlice().
        Bind(&files).
        Variadic().  // Consumes all remaining positional args
        Required().
        Help("Files to process").
        Done().
    Handler(func(ctx *cf.Context) error {
        for _, file := range files {
            process(file)
        }
        return nil
    }).
    Build()
```

**Usage:**
```bash
$ process file1.txt file2.txt file3.txt   # files = ["file1.txt", "file2.txt", "file3.txt"]
$ process *.txt                           # Shell expands, all files processed
```

### Example 5: Optional Positional After Required

```go
var command string
var args []string

cmd := cf.NewCommand("exec").
    Flag("COMMAND").
        String().
        Bind(&command).
        Required().
        Help("Command to execute").
        Done().
    Flag("ARGS").
        StringSlice().
        Bind(&args).
        Variadic().
        Default([]string{}).
        Help("Arguments to command").
        Done().
    Handler(func(ctx *cf.Context) error {
        return exec(command, args...)
    }).
    Build()
```

**Usage:**
```bash
$ exec ls                          # command = "ls", args = []
$ exec ls -la /tmp                 # command = "ls", args = ["-la", "/tmp"]
```

## Semantics

### Matching Algorithm

1. **Parse named flags** (starting with `-`) from command line
2. **Collect remaining args** as positional candidates
3. **Match positionals** in order of definition:
   - For each positional Flag() in definition order
   - If `Variadic()`, consume all remaining args into slice
   - Otherwise, consume next arg and populate bound variable
4. **Validate**:
   - Check required positionals are present
   - Error if too many positionals provided (unless last is Variadic)

### Type Support

Positional arguments support the same types as flags:
- `String()` - single string value
- `Int()` - single integer value
- `Bool()` - single boolean value
- `StringSlice()` - multiple strings (requires `Variadic()`)
- `IntSlice()` - multiple integers (requires `Variadic()`)

### Modifiers

Standard flag modifiers work with positionals:

| Modifier | Meaning for Positionals |
|----------|------------------------|
| `Required()` | Error if not provided |
| `Default(val)` | Value if not provided |
| `Global()` | Applies across clauses |
| `Local()` | Per-clause (for clause-based commands) |
| `Variadic()` | Consumes remaining args (must be last positional) |
| `Help(msg)` | Description for help text |
| `FilePattern(pat)` | Hint for shell completion |

### Constraints

1. **Variadic must be last**: Only the last positional can be `Variadic()`
2. **Required before optional**: Required positionals should come before optional ones (enforced at build time)
3. **One variadic max**: Only one positional can be `Variadic()`

## Help Text Generation

Positional arguments appear in usage line:

```
USAGE:
    convert [OPTIONS] INPUT [OUTPUT]

ARGUMENTS:
    INPUT
        Input file
        Required

    OUTPUT
        Output file
        Default: output.txt

OPTIONS:
    -verbose, -v
        Verbose output
```

For variadic:
```
USAGE:
    process [OPTIONS] FILES...

ARGUMENTS:
    FILES...
        Files to process
        Required
```

## Edge Cases

### Too Many Positionals Provided

```go
Flag("FILE").String().Bind(&file).Done()

// Command: mytool file1.txt file2.txt
// Error: unexpected positional argument: file2.txt
```

Unless last positional is `Variadic()`:
```go
Flag("FILES").StringSlice().Bind(&files).Variadic().Done()

// Command: mytool file1.txt file2.txt
// Result: files = ["file1.txt", "file2.txt"]
```

### Positional That Looks Like Flag

```go
Flag("FILE").String().Bind(&file).Done()

// Command: mytool -input.txt
// Result: file = "-input.txt"  (after all flags are parsed)
```

To avoid confusion, use `--` separator:
```bash
$ mytool -- -input.txt    # Explicitly marks end of flags
```

### Empty String vs Not Provided

```go
Flag("FILE").String().Bind(&file).Default("stdin").Done()

// Command: mytool ""
// Result: file = "" (explicitly provided empty string)

// Command: mytool
// Result: file = "stdin" (not provided, default used)
```

### Interaction with Clauses

Positional arguments work with clause-based commands:

```go
// Global positional applies once
Flag("INPUT").String().Bind(&input).Global().Done()

// Local positional per clause
Flag("-file").String().Local().Done()
```

## Implementation Considerations

### Parsing Strategy

1. **Two-pass parsing**:
   - Pass 1: Extract all named flags (`-*`) into clauses
   - Pass 2: Match remaining args to positional definitions in order

2. **Context updates**:
   - Add `PositionalArgs []string` field to Context (filtered RawArgs)
   - Bind mechanism calls setter on bound variables during Pass 2

3. **Build-time validation**:
   - Check Variadic is last
   - Warn if optional positionals before required ones
   - Error if multiple Variadic positionals

### Backward Compatibility

- **Fully backward compatible**: Existing code continues to work
- `RawArgs` still available for manual parsing
- No breaking changes to existing API

### Performance

- Minimal overhead: One additional filter pass over arguments
- Binding happens once at parse time (same as flags)

## Comparison with Current Approach

### Before (Manual Filtering)

```go
Handler(func(ctx *cf.Context) error {
    var inputFile string
    for _, arg := range ctx.RawArgs {
        if !strings.HasPrefix(arg, "-") {
            inputFile = arg
            break
        }
    }
    // Use inputFile...
})
```

**Issues:**
- Boilerplate in every handler
- Error-prone filtering logic
- No type safety
- No validation
- No help text generation

### After (Proposed)

```go
var inputFile string

Flag("FILE").
    String().
    Bind(&inputFile).
    Help("Input file").
    Done()

Handler(func(ctx *cf.Context) error {
    // inputFile already populated!
})
```

**Benefits:**
- Zero boilerplate
- Type-safe
- Automatic validation
- Help text generation
- Consistent with flags API

## Migration Path

### Phase 1: Core Positional Matching

Implement basic positional argument support:
- Recognize non-hyphenated Flag names as positional
- Match in order to remaining args after flags parsed
- Populate bound variables
- Add `PositionalArgs []string` to Context

**Deliverable:** Basic `.Bind()` for positionals works

### Phase 2: Validation

Add constraint checking:
- Check `Required()` constraint
- Validate too many positionals error
- Enforce `Variadic()` constraints (must be last, only one)
- Warn on optional before required

**Deliverable:** Full validation with helpful error messages

### Phase 3: Help Text Generation

Update help system:
- Add positionals to usage line
- Add ARGUMENTS section to help output
- Mark optional with `[BRACKETS]`
- Mark variadic with `...` suffix

**Deliverable:** `-help` shows positional args properly

### Phase 4: Shell Completion

Extend completion support:
- Use `FilePattern()` for path completion
- Support custom completers for positionals
- Handle variadic completion

**Deliverable:** Tab completion works for positionals

## Testing Strategy

### Unit Tests

```go
func TestSinglePositional(t *testing.T) {
    var file string
    cmd := NewCommand("test").
        Flag("FILE").String().Bind(&file).Done().
        Handler(func(ctx *Context) error { return nil }).
        Build()

    cmd.Execute([]string{"input.txt"})
    assert.Equal(t, "input.txt", file)
}

func TestMultiplePositionals(t *testing.T) {
    var src, dst string
    cmd := NewCommand("test").
        Flag("SRC").String().Bind(&src).Done().
        Flag("DST").String().Bind(&dst).Done().
        Handler(func(ctx *Context) error { return nil }).
        Build()

    cmd.Execute([]string{"a.txt", "b.txt"})
    assert.Equal(t, "a.txt", src)
    assert.Equal(t, "b.txt", dst)
}

func TestVariadic(t *testing.T) {
    var files []string
    cmd := NewCommand("test").
        Flag("FILES").StringSlice().Bind(&files).Variadic().Done().
        Handler(func(ctx *Context) error { return nil }).
        Build()

    cmd.Execute([]string{"a.txt", "b.txt", "c.txt"})
    assert.Equal(t, []string{"a.txt", "b.txt", "c.txt"}, files)
}

func TestMixedFlagsAndPositionals(t *testing.T) {
    var verbose bool
    var file string
    cmd := NewCommand("test").
        Flag("-v").Bool().Bind(&verbose).Done().
        Flag("FILE").String().Bind(&file).Done().
        Handler(func(ctx *Context) error { return nil }).
        Build()

    cmd.Execute([]string{"-v", "input.txt"})
    assert.True(t, verbose)
    assert.Equal(t, "input.txt", file)
}
```

### Integration Tests

Test real-world command patterns from streamv3 migration.

## Open Questions

### 1. Named Positionals in Usage?

**Current proposal:** `mytool FILE`
**Alternative:** `mytool <file>`

**Recommendation:** Keep uppercase convention, matches argument name in code.

### 2. How to Handle `--` Separator?

Standard Unix convention: everything after `--` treated as positional, even if starts with `-`.

```bash
$ mytool -- -weird-filename.txt
```

**Recommendation:** Explicitly support `--` separator.

### 3. Should Default() Work with Required()?

```go
Flag("FILE").String().Required().Default("").Done()  // Valid?
```

**Recommendation:** Error at build time - these are mutually exclusive.

### 4. Type Conversion Errors?

```go
Flag("COUNT").Int().Bind(&count).Done()
// mytool abc  <- not an int
```

**Recommendation:** Return parse error with helpful message: "COUNT: invalid integer 'abc'"

## Success Criteria

1. **API Consistency**: Positionals use same `.Bind()` pattern as flags
2. **Zero Boilerplate**: No manual `RawArgs` filtering needed
3. **Type Safety**: Full type system support (string, int, slices, etc.)
4. **Validation**: Required, defaults, variadic all work correctly
5. **Help Text**: Automatic documentation in `-help` output
6. **Backward Compat**: Existing code continues to work unchanged
7. **Performance**: Negligible overhead vs manual parsing

## Alternatives Considered

### Alternative 1: ctx.Args Slice

```go
// In Context
Args []string  // Just the positional args

// Usage
if len(ctx.Args) > 0 {
    inputFile = ctx.Args[0]
}
```

**Rejected because:**
- Still requires manual indexing
- No type safety
- No validation
- No help text
- Inconsistent with flags API

### Alternative 2: ctx.PositionalArgs Map

```go
// In Context
PositionalArgs map[string]string  // "FILE" -> "/path/to/file"

// Usage
inputFile, _ := ctx.PositionalArgs["FILE"]
```

**Rejected because:**
- Still requires handler logic
- String-only (no type safety)
- No automatic validation
- Less clean than `.Bind()`

## Summary

This proposal makes positional arguments first-class citizens in completionflags by:
- Using the familiar `.Bind()` API
- Matching arguments by definition order
- Supporting all standard modifiers (Required, Default, etc.)
- Generating proper help text
- Maintaining backward compatibility

The result is cleaner, safer, and more maintainable code with zero runtime overhead.

## Next Steps

1. Review and approve proposal
2. Create feature branch
3. Implement Phase 1 (core matching)
4. Write tests
5. Update documentation
6. Release v0.2.0

---

**Questions or Feedback?**
Please open an issue or PR in the completionflags repository.
