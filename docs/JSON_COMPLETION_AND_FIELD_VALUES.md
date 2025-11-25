# JSON Completion Format and Field Value Completion

**Status:** Design Proposal
**Version:** v4.0.0 (Major - Breaking Change)
**Author:** Design Discussion
**Date:** 2025-11-25

## Overview

This document proposes migrating from the current ad-hoc completion interchange format to JSON, and adding field value completion to enable intelligent tab-completion based on actual data content.

## Motivation

### Current Limitations

**1. Ad-hoc Format Can't Handle Special Characters**
```bash
# Current format:
__AUTOCLI_CACHE__:name,city,department

# Breaks with field values containing special characters:
__AUTOCLI_CACHE__:New York,Los Angeles,O'Brien,Smith, John
#                                              ^^^ Ambiguous!
```

**2. Limited to Field Names**
Users can complete field names but not field values:
```bash
$ ssql where -match city <TAB>
# Currently completes: <FIELD>
# Should complete: "New York"  "Los Angeles"  "Chicago"
```

**3. Not Extensible**
Hard to add new directive types without parsing conflicts.

### Why JSON + jq?

✅ **Safe escaping** - JSON libraries handle all special characters correctly
✅ **Standard format** - well-understood, debuggable, future-proof
✅ **Rich data structures** - can pass arrays, objects, nested data
✅ **Widely available** - jq is in most package managers
✅ **Graceful degradation** - can fall back when jq unavailable

## Proposed Solution

### JSON Directive Format

The Go binary outputs **JSON Lines** (one JSON object per line) mixed with plain completion strings:

```
{"type":"field_cache","fields":["name","city","age","department"]}
{"type":"field_values","field":"city","values":["New York","Los Angeles","Chicago"]}
{"type":"field_values","field":"name","values":["Alice","Bob","Charlie","Alice Smith"]}
name
city
age
department
```

**Key principles:**
- One JSON object per line (JSON Lines format)
- JSON lines start with `{` (easy to detect)
- Plain completion strings (field names, etc.) are non-JSON
- Bash script filters and processes separately

### JSON Directive Types

#### 1. Field Cache Directive
Caches field names for downstream commands in pipelines.

```json
{
  "type": "field_cache",
  "fields": ["name", "city", "age", "department"]
}
```

**Bash behavior:**
```bash
export AUTOCLI_FIELDS="name,city,age,department"
```

#### 2. Field Values Directive
Provides actual field values for intelligent completion.

```json
{
  "type": "field_values",
  "field": "city",
  "values": ["New York", "Los Angeles", "Chicago", "San Francisco"]
}
```

**Bash behavior:**
```bash
# Add to completions when completing the "city" field
COMPREPLY+=("New York" "Los Angeles" "Chicago" "San Francisco")
```

#### 3. Hint Directive (Future)
Provides user-facing hints or messages.

```json
{
  "type": "hint",
  "message": "No file specified - reading from stdin"
}
```

#### 4. Environment Directive (Future)
Sets arbitrary environment variables.

```json
{
  "type": "env",
  "key": "AUTOCLI_DEBUG",
  "value": "1"
}
```

## Field Value Completion

### Use Case

When a user is constructing a query, they should be able to tab-complete with actual values from the data:

```bash
# User has a CSV file with cities: New York, Los Angeles, Chicago
$ ssql read-csv users.csv | ssql where -match city <TAB>
# Completes with actual values:
"New York"  "Los Angeles"  "Chicago"

# Even with special characters:
$ ssql where -match name <TAB>
"O'Brien"  "Smith, Jr."  "Alice Smith"
```

### How It Works

**1. User triggers completion on a field argument**
```bash
$ ssql where -match city <TAB>
                    ^^^^ completing this argument
```

**2. Binary is called with `-complete`**
```bash
$ ssql -complete 4 where -match city ""
```

**3. Binary recognizes this is a field value completion**
- Checks if "city" is a known field (from cache or previous arg)
- If yes, reads sample values from the data source
- Returns JSON directive with sample values

**4. Completion script parses and uses values**
```bash
# Bash parses JSON and adds to completions
COMPREPLY+=("New York" "Los Angeles" "Chicago")
```

## Sampling Strategy for Large Files

### The Problem

Reading all values from a large file is:
- **Slow** - completion must be fast (<100ms)
- **Memory intensive** - millions of unique values
- **Unnecessary** - users just need representative samples

### Proposed Solutions

#### Option 1: Head Sampling (Simple)
Read first N records (e.g., 100-1000).

```go
func sampleFieldValues(filePath, fieldName string, maxSamples int) ([]string, error) {
    records := readCSV(filePath)
    seen := make(map[string]bool)
    values := []string{}

    count := 0
    for record := range records {
        if count >= maxSamples {
            break
        }
        value := record[fieldName]
        if !seen[value] {
            seen[value] = true
            values = append(values, value)
        }
        count++
    }

    return values, nil
}
```

**Pros:**
- Simple, fast
- Sequential read (good for CSV/streaming formats)
- Deterministic (same samples each time)

**Cons:**
- Biased (only sees beginning of file)
- May miss values that appear later

#### Option 2: Random Sampling with Reservoir (Better)
Use reservoir sampling to get random representative sample.

```go
func reservoirSample(filePath, fieldName string, maxSamples int) ([]string, error) {
    records := readCSV(filePath)
    reservoir := make([]string, 0, maxSamples)
    seen := make(map[string]bool)

    count := 0
    for record := range records {
        value := record[fieldName]

        // Skip duplicates
        if seen[value] {
            continue
        }
        seen[value] = true

        if len(reservoir) < maxSamples {
            // Fill reservoir
            reservoir = append(reservoir, value)
        } else {
            // Random replacement
            j := rand.Intn(count + 1)
            if j < maxSamples {
                reservoir[j] = value
            }
        }
        count++
    }

    return reservoir, nil
}
```

**Pros:**
- Statistically representative
- Unbiased sample
- Still sequential read (works with streaming)

**Cons:**
- Non-deterministic (different samples each time)
- More complex

#### Option 3: Seek-Based Sampling (For Large Binary Files)
For binary formats (not CSV/JSONL), seek to random positions.

```go
func seekSample(filePath, fieldName string, maxSamples int) ([]string, error) {
    file, _ := os.Open(filePath)
    defer file.Close()

    stat, _ := file.Stat()
    fileSize := stat.Size()

    seen := make(map[string]bool)
    values := []string{}

    for i := 0; i < maxSamples*10 && len(values) < maxSamples; i++ {
        // Seek to random position
        offset := rand.Int63n(fileSize)
        file.Seek(offset, 0)

        // Find next record boundary
        record := findNextRecord(file)
        value := record[fieldName]

        if !seen[value] {
            seen[value] = true
            values = append(values, value)
        }
    }

    return values, nil
}
```

**Pros:**
- Fast for large files
- Good distribution

**Cons:**
- Only works for seekable formats (not streams)
- Complex (need to find record boundaries)
- Not suitable for CSV/JSONL

#### Recommendation: **Reservoir Sampling**

Use reservoir sampling as the default:
- Works with streaming formats (CSV, JSONL)
- Provides unbiased representative sample
- Configurable sample size
- Good trade-off between complexity and quality

**Configuration:**
```go
const (
    DefaultMaxFieldSamples = 100    // Max unique values to return
    DefaultMaxRecordsToScan = 10000 // Stop after scanning this many records
)
```

### Deduplication Strategy

**Problem:** A field might have millions of records but only dozens of unique values.

**Solution:** Track unique values with a map, stop when we have enough:

```go
func sampleUniqueValues(records iter.Seq[Record], fieldName string, maxUnique int, maxRecords int) []string {
    seen := make(map[string]bool)
    values := []string{}

    count := 0
    for record := range records {
        if count >= maxRecords || len(values) >= maxUnique {
            break
        }

        value := record[fieldName]
        if !seen[value] {
            seen[value] = true
            values = append(values, value)
        }
        count++
    }

    return values
}
```

**Early termination:**
- Stop after finding `maxUnique` distinct values (e.g., 100)
- Or after scanning `maxRecords` records (e.g., 10,000)
- Whichever comes first

### Caching Strategy

**Problem:** Re-scanning the file on every completion is slow.

**Solutions:**

#### 1. In-Memory Cache (Process-Level)
Cache field values in the Go binary process.

```go
var fieldValueCache = make(map[string]map[string][]string)

func getCachedFieldValues(filePath, fieldName string) ([]string, bool) {
    if fileCache, ok := fieldValueCache[filePath]; ok {
        if values, ok := fileCache[fieldName]; ok {
            return values, true
        }
    }
    return nil, false
}
```

**Limitation:** Each completion call is a new process, so cache doesn't persist.

#### 2. File-Based Cache
Cache in `$TMPDIR` or `~/.cache/autocli/`.

```go
cacheFile := filepath.Join(os.TempDir(), "autocli_values_" + hash(filePath) + "_" + fieldName)
if values := readCacheFile(cacheFile); values != nil {
    return values
}
```

**Benefits:**
- Persists across completion calls
- Fast lookups

**Drawbacks:**
- Stale data if file changes
- Need cache invalidation

#### 3. Timestamp-Based Invalidation
Include file modification time in cache key.

```go
stat, _ := os.Stat(filePath)
cacheKey := fmt.Sprintf("%s_%s_%d", filePath, fieldName, stat.ModTime().Unix())
```

**Recommendation:** Start simple (no cache), add file-based cache if performance is an issue.

## Implementation Plan

### Phase 1: JSON Output (Go Binary)

**1. Add JSON directive support to FieldCompleter**

```go
// In field_completer.go
type CompletionDirective struct {
    Type   string   `json:"type"`
    Fields []string `json:"fields,omitempty"`
    Field  string   `json:"field,omitempty"`
    Values []string `json:"values,omitempty"`
}

func (fc *FieldCompleter) Complete(ctx CompletionContext) ([]string, error) {
    // ... existing logic ...

    // Return JSON directive for caching
    directive := CompletionDirective{
        Type:   "field_cache",
        Fields: fields,
    }
    jsonBytes, _ := json.Marshal(directive)

    results := []string{string(jsonBytes)}
    results = append(results, fields...)
    return results, nil
}
```

**2. Add FieldValueCompleter**

```go
// New completer for field values
type FieldValueCompleter struct {
    SourceFlag string // Flag containing file path
    FieldName  string // Which field to complete (or auto-detect from context)
    MaxSamples int    // Max values to sample (default 100)
}

func (fvc *FieldValueCompleter) Complete(ctx CompletionContext) ([]string, error) {
    filePath := getFilePathFromContext(ctx, fvc.SourceFlag)
    if filePath == "" {
        return nil, nil
    }

    // Sample field values
    values := sampleFieldValues(filePath, fvc.FieldName, fvc.MaxSamples)

    // Return JSON directive
    directive := CompletionDirective{
        Type:   "field_values",
        Field:  fvc.FieldName,
        Values: values,
    }
    jsonBytes, _ := json.Marshal(directive)

    return []string{string(jsonBytes)}, nil
}
```

**3. Update FieldCompleter to return JSON**

```go
func (fc *FieldCompleter) Complete(ctx CompletionContext) ([]string, error) {
    filePath := fc.getFilePathFromContext(ctx)
    if filePath != "" {
        fields, err := extractFields(filePath)
        if err == nil && len(fields) > 0 {
            // Return JSON directive + field names
            directive := CompletionDirective{
                Type:   "field_cache",
                Fields: fields,
            }
            jsonBytes, _ := json.Marshal(directive)

            results := []string{string(jsonBytes)}
            results = append(results, filterFields(fields, ctx.Partial)...)
            return results, nil
        }
    }

    // Fallback to cached fields
    if cachedFields := getCachedFields(filePath); len(cachedFields) > 0 {
        return filterFields(cachedFields, ctx.Partial), nil
    }

    return []string{"<FIELD>"}, nil
}
```

### Phase 2: JSON Parsing (Bash Completion Script)

**Update completion script template:**

```bash
if ! type _autocli_complete &>/dev/null; then
    _autocli_complete() {
        local cur prev words cword
        cur="${COMP_WORDS[COMP_CWORD]}"

        local output
        output=$(${COMP_WORDS[0]} -complete $COMP_CWORD "${COMP_WORDS[@]:1}" 2>/dev/null)

        # Check if jq is available
        if command -v jq &>/dev/null; then
            # Parse JSON directives
            local json_lines=$(echo "$output" | grep '^{')

            if [[ -n "$json_lines" ]]; then
                while IFS= read -r line; do
                    local type=$(echo "$line" | jq -r '.type // empty')

                    case "$type" in
                        field_cache)
                            # Cache field names
                            local fields=$(echo "$line" | jq -r '.fields | join(",")')
                            export AUTOCLI_FIELDS="$fields"
                            ;;
                        field_values)
                            # Add field values to completions
                            local field=$(echo "$line" | jq -r '.field')
                            mapfile -t values < <(echo "$line" | jq -r '.values[]')
                            COMPREPLY+=($(compgen -W "${values[*]}" -- "$cur"))
                            ;;
                        env)
                            # Set environment variable
                            local key=$(echo "$line" | jq -r '.key')
                            local value=$(echo "$line" | jq -r '.value')
                            export "$key=$value"
                            ;;
                    esac
                done <<< "$json_lines"

                # Remove JSON lines from plain completions
                output=$(echo "$output" | grep -v '^{')
            fi
        else
            # Fallback: look for old-style cache directives
            if echo "$output" | grep -q "^__AUTOCLI_CACHE__:"; then
                local cache_line=$(echo "$output" | grep "^__AUTOCLI_CACHE__:")
                local cache_data="${cache_line#__AUTOCLI_CACHE__:}"
                export AUTOCLI_FIELDS="$cache_data"
                output=$(echo "$output" | grep -v "^__AUTOCLI_CACHE__:")
            fi
        fi

        # Plain string completions
        if [[ -n "$output" ]]; then
            COMPREPLY+=($(compgen -W "$output" -- "$cur"))
        fi
    }
fi
```

### Phase 3: Builder API

**Add methods to fluent API:**

```go
// For field value completion
Flag("-match").
    Arg("FIELD").
        FieldsFromFlag("-input").  // Existing: field names
        Done().
    Arg("VALUE").
        FieldValuesFrom("-input", "FIELD").  // NEW: field values
        Done().
    Done()
```

**Implementation:**

```go
// Add to ArgBuilder
func (ab *ArgBuilder) FieldValuesFrom(sourceFlag, fieldArg string) *ArgBuilder {
    // Set completer for this argument
    ab.fb.spec.ArgCompleters[ab.argIndex] = &FieldValueCompleter{
        SourceFlag: sourceFlag,
        FieldName:  fieldArg,
        MaxSamples: 100,
    }
    return ab
}
```

### Phase 4: Documentation

**Update USAGE.md:**
- Document JSON directive format
- Explain field value completion
- Show examples with special characters
- Document jq requirement (optional but recommended)
- Explain sampling strategy

## Migration Plan

### Backward Compatibility

**Keep old format during transition period:**

Version 4.0.0:
- Output **both** JSON and old-style directives
- Completion script tries JSON first, falls back to old format
- Deprecation warning in docs

Version 5.0.0:
- Remove old-style directives
- JSON only

**Example transition output:**
```
{"type":"field_cache","fields":["name","city"]}
__AUTOCLI_CACHE__:name,city
name
city
```

### User Migration

**For users without jq:**
- Field name completion still works (plain strings)
- Field value completion is skipped
- Clear documentation that jq is recommended

**Installation instructions:**
```bash
# Debian/Ubuntu
sudo apt install jq

# macOS
brew install jq

# RHEL/CentOS
sudo yum install jq

# Alpine
apk add jq
```

## Examples

### Example 1: Basic Field Value Completion

**CSV file (users.csv):**
```csv
name,city,age
Alice,New York,30
Bob,Los Angeles,25
Charlie,Chicago,35
```

**Command:**
```bash
$ ssql read-csv users.csv | ssql where -match city <TAB>
```

**Binary outputs:**
```
{"type":"field_cache","fields":["name","city","age"]}
{"type":"field_values","field":"city","values":["New York","Los Angeles","Chicago"]}
city
```

**User sees:**
```
"New York"  "Los Angeles"  "Chicago"
```

### Example 2: Special Characters

**CSV file (people.csv):**
```csv
name
O'Brien
Smith, Jr.
Alice Smith
```

**Command:**
```bash
$ ssql where -match name <TAB>
```

**Binary outputs:**
```json
{"type":"field_values","field":"name","values":["O'Brien","Smith, Jr.","Alice Smith"]}
```

**User sees:**
```
"O'Brien"  "Smith, Jr."  "Alice Smith"
```

All properly quoted and escaped by JSON!

### Example 3: Large File Sampling

**Large CSV (10M records):**
```csv
user_id,country,product
1,USA,Widget
2,Canada,Gadget
...
10000000,Mexico,Thing
```

**Command:**
```bash
$ ssql read-csv huge.csv | ssql where -match country <TAB>
```

**Binary behavior:**
- Uses reservoir sampling
- Scans first 10,000 records
- Returns up to 100 unique countries
- Completes in <100ms

**Binary outputs:**
```json
{"type":"field_values","field":"country","values":["USA","Canada","Mexico","UK","France",...]}
```

## Performance Considerations

### Completion Speed Targets

- **Without value sampling:** <50ms
- **With value sampling (small files):** <100ms
- **With value sampling (large files):** <200ms

### Optimization Strategies

1. **Early termination** - Stop after N unique values
2. **Sampling limits** - Don't scan entire file
3. **Caching** - Cache sampled values (future enhancement)
4. **Async/background** - Pre-compute in background (advanced)

### Benchmark Tests

Should add benchmarks:
```go
func BenchmarkFieldValueSampling(b *testing.B) {
    // Test with various file sizes
    // - 100 records
    // - 10,000 records
    // - 1,000,000 records
}
```

## Security Considerations

### JSON Injection

✅ **Safe** - Using `encoding/json` standard library
✅ **No eval** - Bash uses `jq` to parse (no code execution)
✅ **No escaping issues** - JSON handles all special characters

### Malicious Completions

**Concern:** Could a malicious file inject commands via completions?

**Example:**
```csv
name
$(rm -rf /)
`whoami`
```

**Mitigation:**
- Completions are just strings, not executed
- Bash's `compgen` doesn't execute
- User must explicitly type/select completion

**Still safe!** ✅

### File Access

Field value sampling requires reading the file:
- Only reads files specified in command arguments
- User already has access to these files
- No additional security concerns

## Testing Strategy

### Unit Tests

```go
func TestFieldValueCompleter(t *testing.T) {
    // Test with various file formats
    // Test with special characters
    // Test with large files (sampling)
    // Test with empty files
    // Test with malformed data
}
```

### Integration Tests

```bash
# Test actual completion with jq
source <(./mycmd -completion-script)
completions=$(./mycmd -complete 4 where -match city "")
# Verify JSON parsing
# Verify values are returned
```

### Evil String Tests

Test with intentionally problematic values:
- Quotes: `"O'Brien"`, `Smith "Jr."`
- Commas: `"Smith, John"`
- Newlines: `"Line1\nLine2"`
- Shell metacharacters: `$(whoami)`, `` `ls` ``, `; rm -rf /`
- Unicode: `"José"`, `"北京"`
- Empty strings: `""`
- Very long strings: `"A" * 10000`

## Open Questions

1. **Should we cache sampled values?**
   - Pro: Faster subsequent completions
   - Con: Complexity, stale data
   - **Recommendation:** Start without, add if needed

2. **Should sampling be configurable?**
   - Per-command setting for max samples?
   - Environment variable `AUTOCLI_MAX_SAMPLES`?
   - **Recommendation:** Start with sensible defaults, add config if needed

3. **Should we support multiple fields at once?**
   ```json
   {"type":"field_values","values":{"city":["NYC","LA"],"name":["Alice","Bob"]}}
   ```
   - **Recommendation:** Start simple (one field per directive)

4. **What about streaming data (stdin)?**
   - Can't sample from stdin (not seekable)
   - Could buffer first N records
   - **Recommendation:** Document limitation, add if needed

## Version and Release

**Target Version:** v4.0.0 (major version bump - breaking change)

**Breaking Changes:**
- Completion script format changes
- Requires jq for field value completion
- Old `__AUTOCLI_CACHE__` format deprecated

**Migration Period:**
- v4.0.0: Support both formats, deprecation warning
- v5.0.0: Remove old format

## References

- [JSON Lines Format](https://jsonlines.org/)
- [jq Manual](https://stedolan.github.io/jq/manual/)
- [Reservoir Sampling Algorithm](https://en.wikipedia.org/wiki/Reservoir_sampling)
- [Bash Programmable Completion](https://www.gnu.org/software/bash/manual/html_node/Programmable-Completion.html)

## Appendix: Alternative Considered

See discussion of bash associative arrays approach (rejected due to eval security concerns).
