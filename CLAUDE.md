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

### 4a. Embedded drivers (v4.5.0+)

The completion engine and dispatch are decoupled from `os.Args` /
`os.Stdout` / `os.Exit` so the same Command tree can drive non-bash
interfaces (readline REPL, SSH server, tests, …).

Public APIs added in v4.5.0:

- `Command.Complete(args, pos) ([]string, error)` — pure completion
  engine, side-effect free, concurrent-safe. The bash `-complete N`
  protocol is now a thin wrapper that prints the results.
- `Command.ExecuteWith(args, base *Context) error` — dispatch using
  the caller-supplied Context for IO/State/Ctx. `Execute(args)`
  keeps working as a wrapper that uses `os.*` defaults.
- `Context.Stdin()/Stdout()/Stderr()/Ctx()` accessors with matching
  `Set*` setters; `Context.State any` pass-through field.

Zero-value Context still defaults to `os.*` — bash CLI path
unchanged. Handlers should read `ctx.Stdout()` / `ctx.State`
instead of `os.Stdout` / a global so they work under either driver.

**Sub-modules (own go.mod each — opt-in deps):**

- `github.com/rosscartlidge/autocli/shell` (v0.1.1) — `chzyer/readline`
  loop. TAB → `cli.Complete`, Enter → `cli.ExecuteWith`. Built-in
  `:exit`, `:help`, `:history`, `:set vi/emacs` commands. Sub-module
  lives at `shell/` (the `/v4` is intentionally absent from its
  module path because Go's tooling expects filesystem layout to
  mirror module path for nested modules under `/vN` parents).
- `github.com/rosscartlidge/autocli/ssh` (v0.1.0) — wraps
  `golang.org/x/crypto/ssh` around `autocli/shell`. ed25519 host
  key load-or-generate, OpenSSH `authorized_keys` parsing,
  `AuthCallback` override hook, graceful shutdown via ctx + grace
  timeout, `ConnMeta` for audit hooks. Refuse-to-start safety:
  empty auth config errors before binding.

**Known issue:** `chzyer/readline.SetVimMode` races with its own
input goroutine when called at runtime (library bug). The `shell`
module works around it by deferring runtime mode switches to next
session; immediate bookkeeping update keeps per-user prefs
persistence working.

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

### Walking args by hand instead of by arity

Anything that scans an argument list (completion, help-at, a new
analyser) must skip a flag's arguments by `ArgCount` and must treat
`-arg VALUE` (`ArgFlag`, the reserved flag form of a positional) as ONE
positional whatever VALUE looks like. Use `cmd.positionalValues(args)` /
`cmd.isArgFlagValue(args, idx)` in `flag.go` rather than a fresh
`strings.HasPrefix(arg, "-")` loop: `findFlagSpec` answers for `-arg` on
every command, so arity-aware walkers handle it for free, and naive ones
are exactly how a value becomes syntax. Never add a second reserved
positional mechanism (`--` already means RemainingArgs).

## Version History

See `CHANGELOG.md` (Keep a Changelog), which carries every release from
v4.0.0 and a summary of v0–v3.

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

```

## Release Process

Released the same way as ssql. The version is the `Version` constant in
`flag.go`, WITHOUT the "v"; the tag is `vX.Y.Z`, annotated.

1. As you work, add entries under `## [Unreleased]` in `CHANGELOG.md`
   (Added / Changed / Fixed), written for a user of the library.
2. To release: rename `[Unreleased]` to `## [X.Y.Z] - YYYY-MM-DD`, add a
   fresh empty `## [Unreleased]` above it, and set `Version = "X.Y.Z"`.
3. `go vet ./... && go test ./...` — `TestVersionMatchesChangelog` fails
   if `Version` and the newest entry disagree.
4. Commit (`release: vX.Y.Z`), then `git tag -a vX.Y.Z -m "vX.Y.Z: summary"`.
5. Run `go test -run TestVersionMatchesChangelog .` again on the clean
   tagged checkout: it now also checks the tag on HEAD names `Version`.
6. `git push && git push origin vX.Y.Z`, then verify the proxy serves it:
   `GOPROXY=direct go list -m github.com/rosscartlidge/autocli/v4@vX.Y.Z`.
7. In ssql: `go get github.com/rosscartlidge/autocli/v4@vX.Y.Z`, and
   remove any `replace` pointing at this checkout (a release never ships
   with one).

**The nested modules `shell/` and `ssh/`** are released the same way,
each with its own `CHANGELOG.md`, its own `Version` constant (in
`version.go`) and its own `TestVersionMatchesChangelog`; their tags carry
the directory: `shell/vX.Y.Z`, `ssh/vX.Y.Z`. Run the steps inside the
module's directory. Release `shell` before `ssh` (ssh requires shell, and
both require autocli), and raise a dependent's `require` when it needs the
new release. Verify with
`GOPROXY=direct go list -m github.com/rosscartlidge/autocli/shell@vX.Y.Z`.

**Never re-tag or move a published tag** — the Go checksum database is
immutable. A mistake in a release is fixed by the next patch release
(v4.21.1 shipped with `Version = "4.0.0"`; v4.21.2 fixed it).

## Go Modules and Versioning

The module path carries the major version: `github.com/rosscartlidge/autocli/v4`
(Go requires the `/vN` suffix from v2). A major bump means a new path
(`/v5`) and is made only when explicitly requested. Import with an alias:
```go
import cf "github.com/rosscartlidge/autocli/v4"
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
