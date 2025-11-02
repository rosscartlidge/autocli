# completionflags - Project Context

This file contains important context and decisions for working with the completionflags Go library.

## Project Overview

A powerful Go package for building sophisticated CLI applications with:
- Clause-based parsing (Boolean logic with `+` and `-` separators)
- Subcommands (git/docker/kubectl-style distributed commands)
- Multi-argument flags with per-argument completion
- Intelligent bash completion
- Auto-generated help and man pages
- Zero dependencies (stdlib only)

## Key Architecture Decisions

### 1. Fluent Builder API

The library uses a fluent builder pattern for clean, chainable configuration:

```go
cmd := cf.NewCommand("myapp").
    Flag("-verbose").Bool().Global().Done().
    Subcommand("query").
        Flag("-filter").Arg("COL").Done().Arg("OP").Done().Arg("VAL").Done().
        Done().
    Build()
```

**Important**: Always use `.Done()` to return to the parent builder.

### 2. Subcommand Implementation

**Pattern**: `SubcommandFlagBuilder` and `SubcommandArgBuilder`
- Problem: `Flag().Done()` needs to return different types (CommandBuilder vs SubcommandBuilder)
- Solution: Created separate builder types that mirror FlagBuilder methods but maintain subcommand context
- Alternative considered: Interfaces/type assertions - rejected for type safety

**Parser Reuse**:
- Subcommands create temporary `Command` with `append(cmd.rootGlobalFlags(), subcmd.Flags...)`
- This allows root globals to work before or after subcommand name
- Parser.go line ~752: Critical that tempCmd includes both root globals and subcommand flags

**Key Files**:
- `subcommand.go` (562 lines) - Core infrastructure
- `parser.go` - `parseSubcommand()`, `parseRootGlobalFlags()`
- `completion_script.go` - `completeWithSubcommands()`

### 3. Three-Level Flag Scoping

1. **Root Global**: Available to all subcommands, all clauses
   - Defined on root command with `.Global()`
   - Parsed before subcommand routing

2. **Subcommand Global**: Available across all clauses in that subcommand
   - Defined on subcommand with `.Global()`

3. **Subcommand Local**: Per-clause only
   - Defined on subcommand with `.Local()`

### 4. Completion Script Design

**Important**: Uses `filepath.Base(os.Args[0])` for binary name detection
- Allows renamed binaries to work correctly
- Shared `_completionflags_complete` function for all programs
- Single script works for multiple binaries

**Position Calculation**: COMP_WORDS includes command at index 0
- When passing to `analyzeCompletionContext`, adjust positions carefully
- See completion_script.go:395 for subcommand position calculation

### 5. Unix `--` Convention

**Implementation**: parser.go detects `--` and stops all parsing
- Everything after `--` goes to `Context.RemainingArgs`
- Works at root and subcommand level
- Terminates clause parsing

**Behavior**:
- Before subcommand: args go to root context
- After subcommand: args go to subcommand context
- After clauses: terminates parsing, args to current context

## API Conventions

### Flag Definition

**Use Help() not Description()** for flag descriptions:
```go
Flag("-verbose").Help("Enable verbose output").Done()  // Correct
Flag("-verbose").Description("...").Done()              // Wrong - doesn't exist
```

### Multi-Argument Flags

Multi-arg flags store values as `map[string]interface{}` where keys are arg names:

```go
// Single occurrence
clause.Flags["-filter"] = map[string]interface{}{
    "COLUMN": "name",
    "OPERATOR": "eq",
    "VALUE": "Alice",
}

// With Accumulate()
clause.Flags["-filter"] = []interface{}{
    map[string]interface{}{"COLUMN": "a", "OPERATOR": "eq", "VALUE": "1"},
    map[string]interface{}{"COLUMN": "b", "OPERATOR": "ne", "VALUE": "2"},
}
```

**Always handle both cases** when using Accumulate():
```go
switch v := filters.(type) {
case []interface{}:
    // Multiple
case map[string]interface{}:
    // Single
}
```

### Safe Type Assertions

Always check for nil before type assertions:
```go
verbose := false
if v, ok := ctx.GlobalFlags["-verbose"]; ok && v != nil {
    verbose = v.(bool)
}
```

## Documentation Standards

### User-Facing Documentation

**Primary**: `doc/USAGE.md` - Comprehensive guide for library users
- Table of contents with all major features
- Core concepts explained first
- Complete working examples
- Best practices sections
- Troubleshooting guides

**Design Docs**: `docs/` directory for implementation details
- Design specifications before implementation
- Implementation summaries after completion
- Architecture decision records

### What Users Need

1. **Quick examples** - Show common patterns immediately
2. **Complete examples** - Full working programs they can copy
3. **API reference** - All builder methods documented
4. **Troubleshooting** - Common errors and solutions
5. **Best practices** - Recommended patterns and anti-patterns

## Testing Strategy

### Manual Testing Required For

1. **Shell completion** - Must test in actual bash
   ```bash
   go build && eval "$(./<binary> -completion-script)"
   ```

2. **Root globals in subcommands** - Test all positions:
   ```bash
   # Before subcommand
   ./app -verbose query ...

   # After subcommand
   ./app query -verbose ...
   ```

3. **Edge cases**:
   - `--` at beginning (no flags parsed)
   - Empty RemainingArgs
   - Nil flag values
   - Single vs accumulated multi-arg flags

## Common Pitfalls

### 1. Parser Temporary Commands

When creating temporary commands for parsing (subcommands, completion), **MUST** include root global flags:

```go
// WRONG - only subcommand flags
tempCmd := &Command{
    flags: subcmd.Flags,
}

// CORRECT - includes root globals
tempCmd := &Command{
    flags: append(cmd.rootGlobalFlags(), subcmd.Flags...),
}
```

### 2. Position Calculations in Completion

COMP_WORDS indexing is tricky:
- COMP_WORDS[0] is the command name
- args passed to Execute/Parse exclude command name
- When calculating positions, account for this offset

### 3. Context Preservation in Builders

SubcommandFlagBuilder exists specifically to preserve context:
- Cannot just return *CommandBuilder from subcommand flag methods
- Must return to *SubcommandBuilder
- Pattern: Mirror all methods with different return type

## Version History

- **v0.1.0** - Initial release with clauses, positional args
- **v0.2.0** - Subcommand support (major feature)
  - Three-level flag scoping
  - Full clause support in subcommands
  - Shell completion for subcommands
  - Comprehensive help generation
- **v0.2.1** - Unix `--` support
  - Context.RemainingArgs field
  - Stops flag parsing at `--`
  - Works with root, subcommands, clauses

## Building and Testing

```bash
# Build examples
cd examples/<name> && go build

# Test completion manually
./<binary> -completion-script > /tmp/comp.sh
source /tmp/comp.sh
./<binary> <TAB>

# Build main library
go build

# Push with tags
git add <files>
git commit -m "feat: Description"
git tag -a v0.X.Y -m "Release notes"
git push && git push --tags
```

## References

- **Primary doc**: doc/USAGE.md - Start here for usage
- **Design docs**: docs/SUBCOMMAND_DESIGN.md, docs/SUBCOMMAND_IMPLEMENTATION_SUMMARY.md
- **Examples**: examples/ directory - Working code samples
- **API comparison**: doc/ARG_API_COMPARISON.md - Fluent vs index-based API

## Notes for Future Work

### Potential Enhancements

1. **Nested subcommands** - `git remote add` style (not implemented)
2. **Subcommand aliases** - Short names for subcommands
3. **Command groups** - Organize subcommands in help output
4. **Hidden subcommands** - For debug/internal commands
5. **Environment variable support** - Flag values from env vars

### Not Needed (Simplified Instead)

- Complex validation frameworks - use simple Validator functions
- Configuration file parsing - out of scope
- Plugin system - keep it simple

## Important Files

Core implementation:
- `flag.go` - Core types (Command, Context, FlagSpec, Clause)
- `builder.go` - Fluent builder API
- `parser.go` - Argument parsing and execution
- `subcommand.go` - Subcommand infrastructure (562 lines)
- `completion_script.go` - Shell completion
- `help.go` - Help and man page generation

User documentation:
- `doc/USAGE.md` - **Primary user documentation**
- `README.md` - Project overview and quick start

Design documentation:
- `docs/SUBCOMMAND_DESIGN.md` - Subcommand design spec
- `docs/SUBCOMMAND_IMPLEMENTATION_SUMMARY.md` - Implementation details
- `doc/ARG_API_COMPARISON.md` - API design decisions

Examples:
- `examples/subcommand_clauses/` - Complex subcommand with clauses
- `examples/remaining_args/` - Demonstrates `--` usage
- `examples/subcommand/` - Simple subcommand example

## Context Tags

- time_duration_man
- positional
- subcommands
- unix_double_dash
