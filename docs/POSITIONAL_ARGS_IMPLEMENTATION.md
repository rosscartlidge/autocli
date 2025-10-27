# Positional Arguments Implementation Summary

**Date:** 2025-10-28
**Status:** Complete (Phases 1-3)

## Overview

Positional arguments have been successfully implemented in completionflags, following the design outlined in `POSITIONAL_ARGS_PROPOSAL.md`. This implementation makes positional arguments first-class citizens with the same powerful features as named flags.

## What Was Implemented

### Phase 1: Core Positional Matching ✅

- **Flag Detection**: Added `isPositional()` method to `FlagSpec` to detect positional arguments (flags without `-` or `+` prefix)
- **Helper Methods**: Added `positionalFlags()` and `namedFlags()` to separate positional from named flag specs
- **Positional Matching**: Implemented `matchPositionals()` and `matchPositionalsToSpecs()` in parser to match remaining args to positional specs in definition order
- **Binding**: Positional arguments are stored in `GlobalFlags` or `Clause.Flags` just like regular flags, so existing `bindValues()` works automatically
- **Variadic Support**: Added `IsVariadic` field to `FlagSpec` and `Variadic()` method to `FlagBuilder`

**Key Files Modified:**
- `flag.go`: Added `IsVariadic` field, helper methods
- `builder.go`: Added `Variadic()` method
- `parser.go`: Added `matchPositionals()` and `matchPositionalsToSpecs()`

### Phase 2: Validation ✅

Build-time validation ensures positional arguments are correctly configured:

- **Variadic Constraints**:
  - Only one positional can be variadic
  - Variadic must be last in the list
- **Required vs Default**: Mutually exclusive (enforced at build time)
- **Optional Before Required**: Warning printed to stderr (non-fatal)

**Key Files Modified:**
- `builder.go`: Added validation call in `Build()`
- `flag.go`: Added `validatePositionals()` method

### Phase 3: Help Text Generation ✅

Positional arguments are now properly documented in help and man pages:

- **Usage Line**: Shows positional arguments with appropriate syntax:
  - Required: `INPUT`
  - Optional: `[OUTPUT]`
  - Variadic: `FILES...` or `[FILES...]`
- **ARGUMENTS Section**: Separate section showing:
  - Argument name
  - Description
  - Type (if not string)
  - Scope (global/per-clause)
  - Default value (if specified)
  - Required status
- **Man Page**: Similar treatment in groff format

**Key Files Modified:**
- `help.go`: Updated `GenerateHelp()`, added `formatPositional()`
- `man.go`: Updated `GenerateManPage()`, added `formatManPositional()`

## API Examples

### Simple Positional Argument

```go
var inputFile string

cmd := cf.NewCommand("read").
    Flag("FILE").
        String().
        Bind(&inputFile).
        Global().
        Help("Input file to read").
        Done().
    Handler(func(ctx *cf.Context) error {
        // inputFile is automatically populated
        return process(inputFile)
    }).
    Build()
```

Usage: `read data.txt`

### Multiple Positionals

```go
var src, dst string

cmd := cf.NewCommand("copy").
    Flag("SOURCE").
        String().
        Bind(&src).
        Required().
        Global().
        Help("Source file").
        Done().
    Flag("DEST").
        String().
        Bind(&dest).
        Required().
        Global().
        Help("Destination file").
        Done().
    Handler(func(ctx *cf.Context) error {
        return copyFile(src, dest)
    }).
    Build()
```

Usage: `copy input.txt output.txt`

### Variadic Positional

```go
var files []string

cmd := cf.NewCommand("process").
    Flag("FILES").
        StringSlice().
        Bind(&files).
        Variadic().
        Required().
        Global().
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

Usage: `process file1.txt file2.txt file3.txt`

### Mixed Flags and Positionals

```go
var verbose bool
var inputFile, outputFile string

cmd := cf.NewCommand("convert").
    Flag("-verbose", "-v").
        Bool().
        Bind(&verbose).
        Global().
        Help("Verbose output").
        Done().
    Flag("INPUT").
        String().
        Bind(&inputFile).
        Required().
        Global().
        Help("Input file").
        Done().
    Flag("OUTPUT").
        String().
        Bind(&outputFile).
        Default("output.txt").
        Global().
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

Usage:
- `convert input.json` (uses default output)
- `convert -v input.json output.yaml`
- `convert input.json -v output.yaml` (flags can be anywhere)

## Test Coverage

All core functionality is tested in `positional_test.go`:

- ✅ Single positional argument
- ✅ Multiple positional arguments
- ✅ Variadic positionals
- ✅ Mixed flags and positionals
- ✅ Required positionals
- ✅ Optional positionals with defaults
- ✅ Integer positionals (type safety)
- ✅ Build-time validation:
  - Variadic must be last
  - Only one variadic
  - Required and Default mutually exclusive

**Test Results:** All 10 tests passing

## Phase 4: Shell Completion ✅

Shell completion for positional arguments is now fully implemented!

**Features:**
- ✅ Uses `FilePattern()` from positional specs for path completion
- ✅ Supports custom completers via `Completer()` method
- ✅ Handles variadic completion (continues completing all remaining args)
- ✅ Correctly skips flags and their arguments when counting positionals
- ✅ Works seamlessly with mixed flags and positionals

**Implementation:**
- Added `completePositional()` method in `completion_script.go`
- Counts positional arguments by skipping flags and their arguments
- Matches position to positional spec index
- Handles variadic positionals (always uses last spec if variadic)
- Calls the spec's completer for suggestions

**Examples:**

```bash
# Complete first positional (INPUT with *.{json,yaml,txt} pattern)
$ convert <TAB>
test.json  test.yaml  other.txt

# Complete second positional (OUTPUT with *.{json,yaml,xml} pattern)
$ convert test.json <TAB>
output.xml  result.yaml

# Works with flags mixed in
$ convert -v <TAB>
test.json  test.yaml

# Variadic completion continues for all args
$ process file1.txt file2.txt <TAB>
file3.txt  file4.txt  # ... continues suggesting files
```

## Design Decisions

### 1. Position-based Matching
Positional arguments are matched in **definition order**, not by position on the command line. After all named flags are parsed, remaining args are matched to positional specs in the order they were defined.

Example:
```bash
$ convert input.json -v output.yaml
```
After parsing `-v`, remaining args `[input.json, output.yaml]` are matched to INPUT and OUTPUT in that order.

### 2. Global vs Local Scope
Positional arguments support both Global and Local scope:
- **Global**: Consumed once from the first clause's positional args
- **Local**: Matched in each clause separately

This is consistent with named flags.

### 3. Backward Compatibility
- Existing `ctx.RawArgs` still available for manual parsing
- Remaining positional args stay in `Clause.Positional` after matching
- No breaking changes to existing API

### 4. Type Safety
Positional arguments support all the same types as flags:
- `String()`, `Int()`, `Float()`, `Bool()`
- `Duration()`, `Time()`
- `StringSlice()` with `Variadic()`

### 5. Validation Philosophy
- **Hard errors** for structural problems (variadic not last, multiple variadics, Required+Default)
- **Warnings** for questionable but valid patterns (optional before required)
- **Runtime errors** for user input problems (missing required, wrong type, too many args)

## Migration Guide

### Before (Manual Parsing)

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

### After (With Positionals)

```go
var inputFile string

Flag("FILE").String().Bind(&inputFile).Global().Done()

Handler(func(ctx *cf.Context) error {
    // inputFile already populated!
})
```

## Example Application

A working example is available in `examples/positional/main.go`:

```bash
# View help
go run examples/positional/main.go -help

# Run with required argument
go run examples/positional/main.go input.json

# Run with both arguments
go run examples/positional/main.go input.json output.yaml

# With flags mixed in
go run examples/positional/main.go -v input.json output.yaml
```

## Future Enhancements

1. **Shell Completion**: Implement `handleCompletion()` to enable tab completion for positional args
2. **Better Error Messages**: Include positional name in "too many arguments" errors
3. **`--` Separator**: Explicitly support `--` to mark end of flags (currently works implicitly)
4. **`.MinArgs()` / `.MaxArgs()`**: For variadic positionals requiring at least/at most N args
5. **Positional Validators**: Custom validation functions for positional arguments (already supported via existing `Validate()` method)

## Files Changed

### Core Implementation
- `flag.go`: Added positional detection and validation
- `builder.go`: Added `Variadic()` method and build validation
- `parser.go`: Added positional matching logic

### Documentation
- `help.go`: Updated help generation
- `man.go`: Updated man page generation

### Tests & Examples
- `positional_test.go`: Comprehensive test suite (new)
- `examples/positional/main.go`: Working example application (new)

## Performance

- **Negligible overhead**: One additional filter pass over arguments
- **Same binding cost**: Uses existing reflection-based binding
- **No allocation waste**: Reuses existing slice for remaining positionals

## Conclusion

Positional arguments are now **fully implemented** in completionflags with all 4 phases complete:
- ✅ Zero boilerplate
- ✅ Type safety
- ✅ Automatic validation
- ✅ Help text generation
- ✅ Man page generation
- ✅ **Shell completion (NEW!)**
- ✅ Backward compatibility
- ✅ Comprehensive testing

The implementation follows Unix conventions, maintains API consistency with named flags, provides intelligent tab completion, and delivers a clean, ergonomic API for CLI tools.
