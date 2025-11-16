# autocli - Project Context

This file contains important context and decisions for working with the autocli Go library.

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
- `subcommand.go` - Core infrastructure
- `parser.go` - `parseSubcommand()`, `parseRootGlobalFlags()`
- `completion_script.go` - `completeWithSubcommands()`

### 2a. Nested Subcommands (v2.1.0)

**Implementation Pattern**: Tree walking with polymorphic parent interface

**Key Design Decisions**:

1. **SubcommandParent Interface**: Allows both CommandBuilder and SubcommandBuilder to have nested subcommands
   ```go
   type SubcommandParent interface {
       addSubcommand(name string, subcmd *Subcommand)
       getRootGlobalFlags() []*FlagSpec
       getCommandName() string
   }
   ```

2. **Builder Interface for Done() Return Type**:
   - Initially over-applied to `Handler()` and `Example()` - **WRONG**
   - Only needed for `Done()` to support fluent chaining through nested levels
   - **Lesson learned**: Don't make methods return `Builder` interface unless necessary
   - `Handler()` and `Example()` should return concrete types for full method chaining

3. **Root Field in SubcommandBuilder**:
   - Every SubcommandBuilder maintains `root *CommandBuilder` reference
   - Enables Done() to return root when at top level
   - Enables Build() to delegate to root from any nesting level
   - Critical for fluent API: `Subcommand().Subcommand().Done().Done().Build()`

4. **Tree Walking Pattern**:
   - Parser walks subcommand tree iteratively (not recursively)
   - Tracks `currentSubcommands` as it descends
   - Builds `path []string` for Context.SubcommandPath
   - Stops when no matching subcommand found or at leaf

5. **Completion for Nested Subcommands**:
   - Walks tree similarly to parser
   - Determines if at intermediate node (show nested names) or leaf (show arguments)
   - Helper methods: `completeNestedSubcommandNames()`, `completeFlagNames()`

6. **Help/Man Page Formatting**:
   - Recursive formatting with depth tracking
   - Indentation shows hierarchy
   - Full paths displayed (e.g., "remote add" not just "add")
   - Alphabetical sorting at each level

**Context.Subcommand → Context.SubcommandPath Breaking Change**:
- v2.0.0: `ctx.Subcommand` (string) - single level only
- v2.1.0: `ctx.SubcommandPath` ([]string) - full path for nested
- Helper methods for migration: `IsSubcommand()`, `IsSubcommandPath()`, `SubcommandName()`

**Positional Arguments in Nested Subcommands**:
- Subcommand names take precedence over positionals
- Intermediate nodes should avoid positionals (use flags instead)
- Only leaf subcommands should have positionals
- Design decision documented in docs/NESTED_SUBCOMMANDS_DESIGN.md

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
- Shared `_autocli_complete` function for all programs
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

3. **Nested subcommands** - Test at all levels:
   ```bash
   # Completion at each level
   ./app <TAB>              # Should show top-level subcommands
   ./app remote <TAB>       # Should show nested subcommands
   ./app remote add <TAB>   # Should show flags/arguments

   # Help at each level
   ./app -help
   ./app remote -help
   ./app remote add -help

   # Execution
   ./app remote add origin https://...
   ./app -verbose remote add origin https://...  # Root global
   ```

4. **Edge cases**:
   - `--` at beginning (no flags parsed)
   - Empty RemainingArgs
   - Nil flag values
   - Single vs accumulated multi-arg flags
   - Nested subcommands with root globals at different positions
   - Flag name conflicts between levels

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

### 4. Builder Interface Over-Application

**Problem**: Don't make every method return `Builder` interface

**What went wrong in v2.1.0**:
```go
// BAD - breaks method chaining
type Builder interface {
    Example(string, string) Builder  // Returns interface
    Handler(ClauseHandlerFunc) Builder
}

// User tries:
Subcommand("cmd").Example("...").Flag("-x")
//                                ^^^ Error: Builder has no Flag method
```

**Correct approach**:
```go
// GOOD - concrete types preserve full API
func (sb *SubcommandBuilder) Example(...) *SubcommandBuilder
func (cb *CommandBuilder) Example(...) *CommandBuilder

// Builder interface ONLY for methods that need polymorphism
type Builder interface {
    Done() Builder      // Needs to return different types
    Subcommand(string) *SubcommandBuilder
    Build() *Command
}
```

**Lesson**: Use interface returns only when absolutely necessary for polymorphism. Otherwise, return concrete types to preserve full method availability.

**When to use interface returns**:
- ✅ `Done()` - Returns different types (SubcommandBuilder or CommandBuilder)
- ✅ Methods that need to work polymorphically across multiple types

**When NOT to use interface returns**:
- ❌ `Example()` - Should return concrete type for chaining
- ❌ `Handler()` - Should return concrete type for chaining
- ❌ `Description()` - Should return concrete type for chaining
- ❌ Most builder methods - Return concrete type unless polymorphism needed

### 5. Nested Subcommands and Flag Conflicts

When creating nested subcommands, watch for flag name conflicts:

```go
// WRONG - conflict with root global
NewCommand("app").
    Flag("-verbose").Global().Done().
    Subcommand("remote").
        Subcommand("list").
            Flag("-verbose").Done().  // PANIC: conflicts with root global
```

**Solution**: Use root global flags throughout, or use different flag names at each level.

### 6. Tree Walking State Management

When implementing tree walking (parser, completion):
- Track `currentSubcommands` as you descend
- Build `path []string` incrementally
- Don't lose track of which level you're at
- Remember `leafSubcmd` to distinguish intermediate vs leaf nodes

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
- **v1.0.0** - First stable release
  - Added package-level `Version` constant
  - Production-ready API
  - Comprehensive documentation and examples
- **v2.0.0** - Remove Bind() method
  - Breaking: Removed Bind() method and reflection
  - Added type-safe helper methods (GetBool, GetString, RequireString, etc.)
  - No more global state anti-pattern
  - Better testability and composability
- **v2.1.0** - Nested subcommands support
  - Multi-level command hierarchies (git remote add, docker container exec)
  - Breaking: Context.Subcommand → Context.SubcommandPath
  - Added helper methods: IsSubcommand(), IsSubcommandPath(), SubcommandName()
  - Builder interface for fluent nested API
  - Tree walking in parser and completion
  - Recursive help/man page formatting
  - Migration guide: docs/MIGRATION_v2.0_to_v2.1.md
- **v2.1.1** - Fix fluent API return types (user contribution)
  - Handler() and Example() return concrete types, not Builder interface
  - Enables full method chaining: Subcommand().Example().Flag()
  - Updated to /v2 module path for Go modules

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

## Go Modules and Versioning

**v2+ Module Path**: Starting with v2.0.0, the module path includes `/v2`:
```go
import "github.com/rosscartlidge/autocli/v3"
```

**Why**: Go modules require `/vN` suffix for major versions 2+

**Migration**:
- v1.x users: `import "github.com/rosscartlidge/autocli/v3"`
- v2.x users: `import "github.com/rosscartlidge/autocli/v3"`

**Tagging**:
- v1.x: `git tag v1.2.3`
- v2.x: `git tag v2.1.0` (module path has /v2 in go.mod)

**Best Practice**: Use aliases to keep code clean:
```go
import cf "github.com/rosscartlidge/autocli/v3"
```

## References

- **Primary doc**: doc/USAGE.md - Start here for usage
- **Design docs**: docs/SUBCOMMAND_DESIGN.md, docs/SUBCOMMAND_IMPLEMENTATION_SUMMARY.md
- **Examples**: examples/ directory - Working code samples
- **API comparison**: doc/ARG_API_COMPARISON.md - Fluent vs index-based API

## Notes for Future Work

### Potential Enhancements

1. ~~**Nested subcommands**~~ - ✅ Implemented in v2.1.0
2. **Subcommand aliases** - Short names for subcommands (e.g., `ls` → `list`)
3. **Command groups** - Organize subcommands in help output (BASIC, ADVANCED, etc.)
4. **Hidden subcommands** - For debug/internal commands (`.Hidden()` method)
5. **Environment variable support** - Flag values from env vars
6. **Completion descriptions** - Show help text during tab completion (fish/zsh style)
7. **Default subcommand** - Run a subcommand when none specified

### Not Needed (Simplified Instead)

- Complex validation frameworks - use simple Validator functions
- Configuration file parsing - out of scope
- Plugin system - keep it simple
- Bind() method - removed in v2.0.0, use helper methods instead

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
- `docs/NESTED_SUBCOMMANDS_DESIGN.md` - Nested subcommands design spec
- `docs/REMOVE_BIND_DESIGN.md` - Rationale for removing Bind()
- `docs/MIGRATION_v2.0_to_v2.1.md` - Migration guide from v2.0 to v2.1
- `doc/ARG_API_COMPARISON.md` - API design decisions

Examples:
- `examples/nested_subcommands/` - Git-like CLI with nested subcommands (v2.1.0)
- `examples/subcommand_clauses/` - Complex subcommand with clauses
- `examples/remaining_args/` - Demonstrates `--` usage
- `examples/subcommand/` - Simple subcommand example

## Context Tags

- time_duration_man
- positional
- subcommands
- nested_subcommands
- unix_double_dash
- fluent_api
- builder_pattern
- tree_walking
- v2_migration
