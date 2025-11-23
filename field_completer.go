package completionflags

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
			// Cache fields for future use
			cacheFields(filePath, fields)
			return filterFields(fields, ctx.Partial), nil
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

// cacheFields stores field names in environment variables for later use
func cacheFields(filePath string, fields []string) {
	fieldsStr := strings.Join(fields, ",")

	// Generic cache - "last used"
	os.Setenv("AUTOCLI_FIELDS", fieldsStr)

	// File-specific cache - more accurate
	if filePath != "" {
		safeFileName := sanitizeForEnv(filepath.Base(filePath))
		os.Setenv("AUTOCLI_FIELDS_"+safeFileName, fieldsStr)
	}
}

// getCachedFields retrieves cached field names from environment variables
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
