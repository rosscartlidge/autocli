# CompletionFlags

A general-purpose Go package for building sophisticated CLI applications with:

- **Fluent API** for flag definition
- **Automatic bash completion** via simple script
- **Clause-based argument grouping** (using `+` and `-` separators)
- **Multi-argument flags** with per-argument completion strategies
- **Auto-generated help** and **man pages**
- **Pluggable completion system**
- **User-defined prefix semantics** (`-flag` vs `+flag`)

## Features

### 1. Sophisticated Flags

Each flag can have 0 or more arguments, with custom completion for each:

```go
Flag("-filter").
    Args(3).
    ArgName(0, "FIELD").
    ArgName(1, "OPERATOR").
    ArgName(2, "VALUE").
    ArgCompleter(0, FieldCompleter{}).
    ArgCompleter(1, OperatorCompleter{}).
    ArgCompleter(2, ValueCompleter{}).
    Done()
```

### 2. Automatic Completion

Simple bash completion script that calls your binary:

```bash
# Generate completion script
$ mytool -completion-script > ~/.bash_completion.d/mytool

# Now tab completion works!
$ mytool -format <TAB>
json  xml  yaml
```

### 3. Help and Man Pages

Automatically generated from flag definitions:

```bash
$ mytool -help        # Usage text
$ mytool -man         # Traditional groff man page
```

### 4. Clause Support

Group arguments using `+` and `-` separators for complex logic:

```bash
# Two clauses (separated by +)
mytool -filter status eq active + -filter role eq admin

# Your handler receives both clauses
# Interpretation is up to you (OR, AND, sequential, etc.)
```

### 5. Pluggable Completion

Easy to add custom completers:

```go
// Built-in completers
FileCompleter{Pattern: "*.json"}
StaticCompleter{Options: []string{"foo", "bar"}}

// Custom completer
type MyCompleter struct{}
func (c MyCompleter) Complete(ctx CompletionContext) ([]string, error) {
    // Your logic here
    return []string{"option1", "option2"}, nil
}
```

## Quick Start

### Installation

```bash
go get github.com/rosscartlidge/completionflags
```

### Basic Example

```go
package main

import (
    "fmt"
    "os"
    cf "github.com/rosscartlidge/completionflags"
)

type Config struct {
    Input   string
    Format  string
    Verbose bool
}

func main() {
    config := &Config{}

    cmd := cf.NewCommand("mytool").
        Version("1.0.0").
        Description("Process files").

        Flag("-input", "-i").
            Bind(&config.Input).
            String().
            Global().
            Required().
            Help("Input file").
            FilePattern("*.txt").
            Done().

        Flag("-format", "-f").
            Bind(&config.Format).
            String().
            Global().
            Default("json").
            Help("Output format").
            Options("json", "yaml", "xml").
            Done().

        Flag("-verbose", "-v").
            Bind(&config.Verbose).
            Bool().
            Global().
            Help("Verbose output").
            Done().

        Handler(func(ctx *cf.Context) error {
            fmt.Printf("Processing %s in %s format\n",
                config.Input, config.Format)
            return nil
        }).

        Build()

    if err := cmd.Execute(os.Args[1:]); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

## Concepts

### Scopes: Global vs Local

- **Global flags**: Apply to the entire command (e.g., input file, output format)
- **Local flags**: Apply within each clause (e.g., filters, transformations)

```go
Flag("-input").Global().Done()   // Same for all clauses
Flag("-filter").Local().Done()   // Different per clause
```

### Clauses

Arguments are grouped into clauses separated by `+` or `-`:

```bash
mytool -x foo -y bar + -x baz -y qux
#      ^Clause 1^   ^ ^Clause 2^
```

Your handler receives all clauses and decides how to interpret them:

```go
Handler(func(ctx *cf.Context) error {
    for i, clause := range ctx.Clauses {
        fmt.Printf("Clause %d:\n", i+1)
        // Process clause.Flags
    }
    return nil
})
```

### Prefix Semantics: `-` vs `+`

The package supports both `-flag` and `+flag` prefixes. The meaning is defined by you:

```go
// Example: + means "negate"
cmd.PrefixHandler(func(flagName string, hasPlus bool, value interface{}) interface{} {
    if hasPlus {
        return !value.(bool)  // Negate boolean
    }
    return value
})

// Now: -verbose = true, +verbose = false
```

### Multi-Argument Flags

Flags can take multiple arguments, each with its own completer:

```go
Flag("-match").
    Args(2).
    ArgName(0, "FIELD").
    ArgName(1, "VALUE").
    ArgCompleter(0, FieldCompleter{}).
    ArgCompleter(1, ValueCompleter{}).
    Done()

// Usage: mytool -match name "John" -match age "25"
```

## Completion System

### Built-in Completers

#### FileCompleter

```go
FileCompleter{Pattern: "*.json"}      // Match pattern
FileCompleter{DirsOnly: true}         // Only directories
```

#### StaticCompleter

```go
StaticCompleter{Options: []string{"foo", "bar", "baz"}}
```

#### ChainCompleter

```go
ChainCompleter{
    Completers: []Completer{
        &FileCompleter{Pattern: "*.json"},
        &StaticCompleter{Options: []string{"-"}},
    },
}
```

#### DynamicCompleter

```go
DynamicCompleter{
    Chooser: func(ctx CompletionContext) Completer {
        // Choose completer based on context
        if ctx.GlobalFlags["format"] == "json" {
            return &FileCompleter{Pattern: "*.json"}
        }
        return &FileCompleter{Pattern: "*.yaml"}
    },
}
```

### Custom Completers

Implement the `Completer` interface:

```go
type Completer interface {
    Complete(ctx CompletionContext) ([]string, error)
}
```

Example - Git branch completer:

```go
type GitBranchCompleter struct{}

func (g GitBranchCompleter) Complete(ctx CompletionContext) ([]string, error) {
    cmd := exec.Command("git", "branch", "--format=%(refname:short)")
    output, err := cmd.Output()
    if err != nil {
        return []string{}, nil
    }

    branches := strings.Split(strings.TrimSpace(string(output)), "\n")
    var matches []string
    for _, branch := range branches {
        if strings.HasPrefix(branch, ctx.Partial) {
            matches = append(matches, branch)
        }
    }
    return matches, nil
}

// Use it
Flag("-branch").
    String().
    Completer(GitBranchCompleter{}).
    Done()
```

## Examples

See the `examples/` directory:

- **simple/** - Basic flag usage
- **datatool/** - Multi-argument flags and clauses

### Running Examples

```bash
cd examples/simple
go build
./simple -help
./simple -completion-script
```

## API Reference

### CommandBuilder

```go
NewCommand(name string) *CommandBuilder
    .Version(version string)
    .Description(desc string)
    .Author(author string)
    .Example(command, description string)
    .Separators(seps ...string)          // Default: ["+", "-"]
    .PrefixHandler(h PrefixHandler)
    .Flag(names ...string) *FlagBuilder
    .Handler(h ClauseHandlerFunc)
    .Build() *Command
```

### FlagBuilder

```go
Flag(names ...string) *FlagBuilder
    .Global()                            // Global scope
    .Local()                             // Per-clause scope (default)
    .Help(description string)
    .Args(count int)                     // Number of arguments
    .ArgName(index int, name string)
    .ArgType(index int, t ArgType)
    .ArgCompleter(index int, c Completer)
    .Bool()                              // Shorthand: 0 args
    .String()                            // Shorthand: 1 string arg
    .Int()                               // Shorthand: 1 int arg
    .Float()                             // Shorthand: 1 float arg
    .StringSlice()                       // Accumulate multiple values
    .Bind(ptr interface{})
    .Required()
    .Default(value interface{})
    .Validate(fn ValidatorFunc)
    .Completer(c Completer)              // For single-arg flags
    .CompleterFunc(f CompletionFunc)
    .FilePattern(pattern string)         // Creates FileCompleter
    .Options(opts ...string)             // Creates StaticCompleter
    .Hidden()                            // Hide from help/man
    .Done() *CommandBuilder
```

### Handler

```go
func(ctx *Context) error

type Context struct {
    Command     *Command
    Clauses     []Clause
    GlobalFlags map[string]interface{}
    RawArgs     []string
}

type Clause struct {
    Separator  string                    // "+" or "-"
    Flags      map[string]interface{}
    Positional []string
}
```

## Auto-Generated Features

### Help Text

```bash
$ mytool -help
```

Shows:
- Version and description
- Usage line
- All flags with descriptions, defaults, scope
- Clause explanation
- Examples

### Man Page

```bash
$ mytool -man | man -l -
```

Traditional groff format with:
- NAME, SYNOPSIS, DESCRIPTION sections
- OPTIONS with formatting
- EXAMPLES
- AUTHOR

### Bash Completion

```bash
$ mytool -completion-script
```

Generates universal bash completion script.

## License

MIT

## Contributing

Contributions welcome! Please open an issue or PR.
