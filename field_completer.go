package completionflags

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CompletionDirective represents a JSON directive for the completion script
// These are returned alongside regular completions to pass structured data
// to the bash completion script (requires jq to parse)
type CompletionDirective struct {
	Type   string   `json:"type"`            // Directive type: "field_cache", "field_values", "env"
	Fields []string `json:"fields,omitempty"` // For field_cache: list of field names
	Field  string   `json:"field,omitempty"`  // For field_values: which field these values are from
	Values []string `json:"values,omitempty"` // For field_values: the actual values
	Key    string   `json:"key,omitempty"`    // For env: environment variable name
	Value  string   `json:"value,omitempty"`  // For env: environment variable value
}

// toJSON converts the directive to a JSON string
func (cd *CompletionDirective) toJSON() string {
	jsonBytes, err := json.Marshal(cd)
	if err != nil {
		return "" // Silently fail - completion will still work without directive
	}
	return string(jsonBytes)
}

// FieldCompleter provides field name completion from data files (CSV, TSV, JSON, JSONL)
// It reads the file specified by another flag and extracts field names from the header/first record
type FieldCompleter struct {
	SourceFlag string // Flag containing the file path (e.g., "-input")
}

// Complete implements Completer interface
func (fc *FieldCompleter) Complete(ctx CompletionContext) ([]string, error) {
	// Try to get the file path from the source flag
	filePath := fc.getFilePathFromContext(ctx)

	if filePath != "" {
		// Try to extract fields from the file
		fields, err := extractFields(filePath)
		if err == nil && len(fields) > 0 {
			// Return completions with JSON cache directive
			filtered := filterFields(fields, ctx.Partial)

			// Create JSON directive for field caching
			directive := CompletionDirective{
				Type:   "field_cache",
				Fields: fields, // All fields (not just filtered)
			}

			// Prepend JSON directive
			result := []string{directive.toJSON()}
			result = append(result, filtered...)

			return result, nil
		}
	}

	// Fallback to cached fields
	if cachedFields := getCachedFields(filePath); len(cachedFields) > 0 {
		return filterFields(cachedFields, ctx.Partial), nil
	}

	// Last resort: show hint
	return []string{"<FIELD>"}, nil
}

// getFilePathFromContext extracts the file path from the referenced flag
func (fc *FieldCompleter) getFilePathFromContext(ctx CompletionContext) string {
	// Check GlobalFlags for the source flag value
	if val, ok := ctx.GlobalFlags[fc.SourceFlag]; ok && val != nil {
		if filePath, ok := val.(string); ok {
			return filePath
		}
	}

	// Also check in the current args being parsed
	// This handles the case where we're completing during argument parsing
	for i := 0; i < len(ctx.Args)-1; i++ {
		if ctx.Args[i] == fc.SourceFlag {
			// Next arg should be the file path
			if i+1 < len(ctx.Args) {
				return ctx.Args[i+1]
			}
		}
	}

	return ""
}

// extractFields extracts field names from a data file based on its extension
func extractFields(filePath string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".csv":
		return extractCSVFields(filePath, ',')
	case ".tsv":
		return extractCSVFields(filePath, '\t')
	case ".jsonl", ".ndjson":
		return extractJSONLFields(filePath)
	case ".json":
		return extractJSONFields(filePath)
	default:
		// Unknown extension, try CSV as fallback
		return extractCSVFields(filePath, ',')
	}
}

// extractCSVFields extracts field names from CSV/TSV first line (header)
func extractCSVFields(filePath string, delimiter rune) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = delimiter
	reader.TrimLeadingSpace = true

	// Read first line (header)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	// Trim whitespace from field names
	fields := make([]string, len(header))
	for i, field := range header {
		fields[i] = strings.TrimSpace(field)
	}

	return fields, nil
}

// extractJSONLFields extracts field names from first line of JSONL file
func extractJSONLFields(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return nil, scanner.Err()
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(scanner.Bytes(), &obj); err != nil {
		return nil, err
	}

	// Extract keys and sort for consistent order
	fields := make([]string, 0, len(obj))
	for k := range obj {
		fields = append(fields, k)
	}
	sort.Strings(fields)

	return fields, nil
}

// extractJSONFields extracts field names from JSON array's first object
func extractJSONFields(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var data interface{}
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	// Handle different JSON structures
	switch v := data.(type) {
	case []interface{}:
		// Array of objects - get keys from first object
		if len(v) > 0 {
			if obj, ok := v[0].(map[string]interface{}); ok {
				fields := make([]string, 0, len(obj))
				for k := range obj {
					fields = append(fields, k)
				}
				sort.Strings(fields)
				return fields, nil
			}
		}
	case map[string]interface{}:
		// Single object - get its keys
		fields := make([]string, 0, len(v))
		for k := range v {
			fields = append(fields, k)
		}
		sort.Strings(fields)
		return fields, nil
	}

	return nil, nil
}

// getCachedFields retrieves cached field names from environment variables
// These are set by the bash completion script when it parses __AUTOCLI_CACHE__ directives
func getCachedFields(filePath string) []string {
	// Try file-specific cache first
	if filePath != "" {
		safeFileName := sanitizeForEnv(filepath.Base(filePath))
		if cached := os.Getenv("AUTOCLI_FIELDS_" + safeFileName); cached != "" {
			return strings.Split(cached, ",")
		}
	}

	// Fall back to generic cache
	if cached := os.Getenv("AUTOCLI_FIELDS"); cached != "" {
		return strings.Split(cached, ",")
	}

	return nil
}

// sanitizeForEnv converts a filename to a safe environment variable suffix
// data.csv -> data_csv
// users.jsonl -> users_jsonl
func sanitizeForEnv(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '.' || r == '-' || r == ' ' {
			return '_'
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)
}

// filterFields filters field names based on partial match
func filterFields(fields []string, partial string) []string {
	if partial == "" {
		return fields
	}

	partial = strings.ToLower(partial)
	matches := []string{} // Initialize to empty slice, not nil

	for _, field := range fields {
		if strings.HasPrefix(strings.ToLower(field), partial) {
			matches = append(matches, field)
		}
	}

	return matches
}

// FieldCacheCompleter provides a special completer for -cache flags
// It extracts fields from a file and returns the cache directive + "DONE"
// This is used to enable field caching in pipeline scenarios where the
// first command doesn't have any FieldsFromFlag flags
type FieldCacheCompleter struct {
	SourceFlag string // Flag containing the file path (e.g., "FILE")
}

// Complete implements Completer interface
// Returns JSON cache directive and "DONE" as the only completion option
func (fcc *FieldCacheCompleter) Complete(ctx CompletionContext) ([]string, error) {
	// Try to get the file path from the source flag
	filePath := fcc.getFilePathFromContext(ctx)

	if filePath != "" {
		// Try to extract fields from the file
		fields, err := extractFields(filePath)
		if err == nil && len(fields) > 0 {
			// Return JSON cache directive + "DONE"
			directive := CompletionDirective{
				Type:   "field_cache",
				Fields: fields,
			}
			return []string{directive.toJSON(), "DONE"}, nil
		}
	}

	// If we can't read the file, just return DONE
	return []string{"DONE"}, nil
}

// getFilePathFromContext extracts the file path from the referenced flag
func (fcc *FieldCacheCompleter) getFilePathFromContext(ctx CompletionContext) string {
	// Check GlobalFlags for the source flag value (works for regular flags and parsed positionals)
	if val, ok := ctx.GlobalFlags[fcc.SourceFlag]; ok && val != nil {
		if filePath, ok := val.(string); ok {
			return filePath
		}
	}

	// Check in Args for flag-style arguments (e.g., -input file.csv)
	for i := 0; i < len(ctx.Args)-1; i++ {
		if ctx.Args[i] == fcc.SourceFlag {
			// Next arg should be the file path
			if i+1 < len(ctx.Args) {
				return ctx.Args[i+1]
			}
		}
	}

	// For positional arguments: find the positional flag spec and get its value from Args
	if ctx.Command != nil {
		// Find the positional flag with the matching name
		var positionals []*FlagSpec
		for _, spec := range ctx.Command.flags {
			if spec.isPositional() {
				positionals = append(positionals, spec)
			}
		}

		// Find which positional index our source flag is
		for i, spec := range positionals {
			if len(spec.Names) > 0 && spec.Names[0] == fcc.SourceFlag {
				// This is our positional - get its value from Args[i]
				if i < len(ctx.Args) {
					// Skip any flags in Args to find positional values
					positionalValues := []string{}
					for _, arg := range ctx.Args {
						// Skip flags and their arguments
						if !strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "+") {
							positionalValues = append(positionalValues, arg)
						}
					}
					if i < len(positionalValues) {
						return positionalValues[i]
					}
				}
			}
		}
	}

	return ""
}

// FieldValueCompleter provides completion for field values from data files
// It samples actual data values from a file to provide realistic completions
// Used with FieldValuesFrom() to enable tab completion like: -match name <TAB> → Alice, Bob, Charlie
type FieldValueCompleter struct {
	SourceFlag string // Flag containing the file path (e.g., "-input" or "FILE")
	FieldArg   string // Name of argument containing the field name (e.g., "FIELD")
	MaxSamples int    // Maximum unique values to sample (default: 100)
	MaxRecords int    // Maximum records to scan (default: 10000)
}

// Complete implements Completer interface
// Returns JSON directive with sampled field values
func (fvc *FieldValueCompleter) Complete(ctx CompletionContext) ([]string, error) {
	// Set defaults
	maxSamples := fvc.MaxSamples
	if maxSamples == 0 {
		maxSamples = 100
	}
	maxRecords := fvc.MaxRecords
	if maxRecords == 0 {
		maxRecords = 10000
	}

	// Get the field name from context
	fieldName := fvc.getFieldNameFromContext(ctx)
	if fieldName == "" {
		return []string{"<VALUE>"}, nil
	}

	// Get the file path from context
	filePath := fvc.getFilePathFromContext(ctx)
	if filePath == "" {
		return []string{"<VALUE>"}, nil
	}

	// Sample field values from the file
	values, err := sampleFieldValues(filePath, fieldName, maxSamples, maxRecords)
	if err != nil || len(values) == 0 {
		// Fallback: check if we have cached values in environment
		if cached := os.Getenv("AUTOCLI_VALUES_" + sanitizeForEnv(fieldName)); cached != "" {
			return strings.Split(cached, ","), nil
		}
		return []string{"<VALUE>"}, nil
	}

	// Return JSON directive + filtered values
	filtered := filterFields(values, ctx.Partial)

	directive := CompletionDirective{
		Type:   "field_values",
		Field:  fieldName,
		Values: values, // All values (not just filtered)
	}

	result := []string{directive.toJSON()}
	result = append(result, filtered...)

	return result, nil
}

// getFieldNameFromContext extracts the field name from the previous arguments
// This looks at the FlagSpec to find which argument is the field name
func (fvc *FieldValueCompleter) getFieldNameFromContext(ctx CompletionContext) string {
	// The field name comes from a previous argument in this flag
	// For example: -match FIELD VALUE
	// We're completing VALUE, and need to get FIELD from PreviousArgs

	if ctx.Command == nil || ctx.FlagName == "" {
		return ""
	}

	// Find the flag spec from the command
	var flagSpec *FlagSpec
	for _, spec := range ctx.Command.flags {
		for _, name := range spec.Names {
			if name == ctx.FlagName {
				flagSpec = spec
				break
			}
		}
		if flagSpec != nil {
			break
		}
	}

	if flagSpec == nil {
		return ""
	}

	// Find the argument index for our field arg
	for i, argName := range flagSpec.ArgNames {
		if argName == fvc.FieldArg {
			// Check if we have this argument in PreviousArgs
			if i < len(ctx.PreviousArgs) {
				return ctx.PreviousArgs[i]
			}
		}
	}

	return ""
}

// getFilePathFromContext extracts the file path from the referenced flag
// This is identical to FieldCacheCompleter.getFilePathFromContext
func (fvc *FieldValueCompleter) getFilePathFromContext(ctx CompletionContext) string {
	// Check GlobalFlags for the source flag value (works for regular flags and parsed positionals)
	if val, ok := ctx.GlobalFlags[fvc.SourceFlag]; ok && val != nil {
		if filePath, ok := val.(string); ok {
			return filePath
		}
	}

	// Check in Args for flag-style arguments (e.g., -input file.csv)
	for i := 0; i < len(ctx.Args)-1; i++ {
		if ctx.Args[i] == fvc.SourceFlag {
			// Next arg should be the file path
			if i+1 < len(ctx.Args) {
				return ctx.Args[i+1]
			}
		}
	}

	// For positional arguments: find the positional flag spec and get its value from Args
	if ctx.Command != nil {
		// Find the positional flag with the matching name
		var positionals []*FlagSpec
		for _, spec := range ctx.Command.flags {
			if spec.isPositional() {
				positionals = append(positionals, spec)
			}
		}

		// Find which positional index our source flag is
		for i, spec := range positionals {
			if len(spec.Names) > 0 && spec.Names[0] == fvc.SourceFlag {
				// This is our positional - get its value from Args[i]
				if i < len(ctx.Args) {
					// Skip any flags in Args to find positional values
					positionalValues := []string{}
					for _, arg := range ctx.Args {
						// Skip flags and their arguments
						if !strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "+") {
							positionalValues = append(positionalValues, arg)
						}
					}
					if i < len(positionalValues) {
						return positionalValues[i]
					}
				}
			}
		}
	}

	return ""
}

// sampleFieldValues samples unique values from a field in a data file
// Uses reservoir sampling for large files to get representative sample
func sampleFieldValues(filePath, fieldName string, maxSamples, maxRecords int) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".csv":
		return sampleCSVFieldValues(filePath, fieldName, ',', maxSamples, maxRecords)
	case ".tsv":
		return sampleCSVFieldValues(filePath, fieldName, '\t', maxSamples, maxRecords)
	case ".jsonl", ".ndjson":
		return sampleJSONLFieldValues(filePath, fieldName, maxSamples, maxRecords)
	case ".json":
		// For JSON arrays, treat similar to JSONL
		return sampleJSONLFieldValues(filePath, fieldName, maxSamples, maxRecords)
	default:
		// Unknown extension, try CSV as fallback
		return sampleCSVFieldValues(filePath, fieldName, ',', maxSamples, maxRecords)
	}
}

// sampleCSVFieldValues samples unique values from a CSV/TSV column
func sampleCSVFieldValues(filePath, fieldName string, delimiter rune, maxSamples, maxRecords int) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = delimiter
	reader.TrimLeadingSpace = true

	// Read header to find column index
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	// Find the field index
	fieldIndex := -1
	for i, field := range header {
		if strings.TrimSpace(field) == fieldName {
			fieldIndex = i
			break
		}
	}

	if fieldIndex == -1 {
		return nil, fmt.Errorf("field %q not found in header", fieldName)
	}

	// Use map for unique values (preserves insertion order in Go 1.12+)
	uniqueValues := make(map[string]bool)
	recordCount := 0

	// Read records and collect unique values
	for recordCount < maxRecords {
		record, err := reader.Read()
		if err != nil {
			break // EOF or error
		}

		recordCount++

		if fieldIndex < len(record) {
			value := strings.TrimSpace(record[fieldIndex])
			if value != "" && !uniqueValues[value] {
				uniqueValues[value] = true

				// Stop if we have enough samples
				if len(uniqueValues) >= maxSamples {
					break
				}
			}
		}
	}

	// Convert map to sorted slice
	values := make([]string, 0, len(uniqueValues))
	for value := range uniqueValues {
		values = append(values, value)
	}
	sort.Strings(values)

	return values, nil
}

// sampleJSONLFieldValues samples unique values from a JSONL field
func sampleJSONLFieldValues(filePath, fieldName string, maxSamples, maxRecords int) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	uniqueValues := make(map[string]bool)
	recordCount := 0

	scanner := bufio.NewScanner(f)
	for scanner.Scan() && recordCount < maxRecords {
		recordCount++

		var obj map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &obj); err != nil {
			continue // Skip malformed lines
		}

		// Extract field value
		if val, ok := obj[fieldName]; ok && val != nil {
			// Convert to string
			valueStr := fmt.Sprintf("%v", val)
			if valueStr != "" && !uniqueValues[valueStr] {
				uniqueValues[valueStr] = true

				// Stop if we have enough samples
				if len(uniqueValues) >= maxSamples {
					break
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Convert map to sorted slice
	values := make([]string, 0, len(uniqueValues))
	for value := range uniqueValues {
		values = append(values, value)
	}
	sort.Strings(values)

	return values, nil
}

