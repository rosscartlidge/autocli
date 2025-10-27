# Shared Completion Function

## Overview

All programs built with completionflags now share a single bash completion function (`_completionflags_complete`), rather than each program having its own completion function.

## Benefits

### 1. Memory Efficiency
Bash only needs to load the completion function **once**, no matter how many completionflags-based programs you have installed.

**Before:**
```bash
# Each program had its own function
_convert_complete() { ... }    # ~15 lines
_process_complete() { ... }    # ~15 lines
_analyze_complete() { ... }    # ~15 lines
# = 45 lines of identical code in memory
```

**After:**
```bash
# All programs share one function
_completionflags_complete() { ... }  # ~15 lines
# = 15 lines total, regardless of number of programs
```

### 2. Simpler Script Management
Users can source multiple completion scripts without worrying about duplicate definitions:

```bash
# Source completions for all your tools
source <(convert -completion-script)
source <(process -completion-script)
source <(analyze -completion-script)

# The _completionflags_complete function is defined once
# Each command is registered to use it:
# complete -F _completionflags_complete convert
# complete -F _completionflags_complete process
# complete -F _completionflags_complete analyze
```

### 3. Installation Simplicity
System-wide installations become cleaner:

```bash
# /etc/bash_completion.d/completionflags-tools
# Define the shared function once
_completionflags_complete() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    local completions
    completions=$(${COMP_WORDS[0]} -complete $COMP_CWORD "${COMP_WORDS[@]:1}" 2>/dev/null)
    if [[ -n "$completions" ]]; then
        COMPREPLY=($(compgen -W "$completions" -- "$cur"))
    fi
}

# Register all installed tools
complete -F _completionflags_complete convert
complete -F _completionflags_complete process
complete -F _completionflags_complete analyze
```

## Implementation Details

### Conditional Definition
The generated script checks if the function already exists:

```bash
if ! type _completionflags_complete &>/dev/null; then
    _completionflags_complete() {
        # ... function body ...
    }
fi
```

This means:
- **First script sourced**: Defines the function
- **Subsequent scripts**: Skip the definition, just register their command

### Generic Design
The completion function is completely generic - it calls `${COMP_WORDS[0]}` (the actual command being completed) with `-complete`, so each program handles its own completion logic:

```bash
completions=$(${COMP_WORDS[0]} -complete $COMP_CWORD "${COMP_WORDS[@]:1}" 2>/dev/null)
```

This allows:
- One function for all programs
- Each program has unique completions
- No conflicts or shared state

## Example

Generate and compare completion scripts:

```bash
# Generate completion for two different programs
go run examples/positional/main.go -completion-script > /tmp/convert.bash
go run examples/simple/main.go -completion-script > /tmp/simple.bash

# Source both
source /tmp/convert.bash
source /tmp/simple.bash

# Both commands now have completion
convert <TAB>        # completes convert-specific options
simple <TAB>         # completes simple-specific options

# But only ONE _completionflags_complete function is loaded
type _completionflags_complete   # shows the function exists once
```

## Migration from Previous Versions

If you previously generated completion scripts with command-specific function names (e.g., `_mytool_complete`), you can safely regenerate them. The new scripts are backward compatible:

1. Old scripts with `_mytool_complete` will continue to work
2. New scripts with `_completionflags_complete` will work alongside them
3. Regenerating all scripts gives you the memory benefits immediately

## For Package Maintainers

When packaging completionflags-based tools:

1. **Option 1: One completion file per tool**
   ```bash
   mytool -completion-script > /etc/bash_completion.d/mytool
   ```
   The shared function is automatically handled.

2. **Option 2: Single master file** (recommended for multiple tools)
   ```bash
   # /etc/bash_completion.d/completionflags-suite
   # Manually create a file that:
   # 1. Defines _completionflags_complete once
   # 2. Registers all your tools
   ```

## Performance

The shared function has:
- **No performance penalty**: Same execution time per completion
- **Memory savings**: Linear with number of installed tools
- **Faster shell startup**: Fewer functions to parse and load

For a system with 10 completionflags-based tools:
- **Before**: ~150 lines of bash completion code
- **After**: ~15 lines of bash completion code
- **Memory saved**: ~90%
