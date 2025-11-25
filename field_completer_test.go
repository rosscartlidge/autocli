package completionflags

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFieldCompleter_CSV(t *testing.T) {
	// Create temp CSV file
	f, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("name,age,salary,department\n")
	f.WriteString("Alice,30,50000,Engineering\n")
	f.WriteString("Bob,25,45000,Sales\n")
	f.Close()

	// Test field extraction
	fields, err := extractFields(f.Name())
	if err != nil {
		t.Fatalf("extractFields failed: %v", err)
	}

	expected := []string{"name", "age", "salary", "department"}
	if !reflect.DeepEqual(fields, expected) {
		t.Errorf("got %v, want %v", fields, expected)
	}
}

func TestFieldCompleter_TSV(t *testing.T) {
	// Create temp TSV file
	f, err := os.CreateTemp("", "test*.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("id\tname\temail\tcreated_at\n")
	f.WriteString("1\tAlice\talice@example.com\t2024-01-01\n")
	f.Close()

	// Test field extraction
	fields, err := extractFields(f.Name())
	if err != nil {
		t.Fatalf("extractFields failed: %v", err)
	}

	expected := []string{"id", "name", "email", "created_at"}
	if !reflect.DeepEqual(fields, expected) {
		t.Errorf("got %v, want %v", fields, expected)
	}
}

func TestFieldCompleter_JSONL(t *testing.T) {
	// Create temp JSONL file
	f, err := os.CreateTemp("", "test*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString(`{"id":1,"username":"alice","email":"alice@example.com","active":true}` + "\n")
	f.WriteString(`{"id":2,"username":"bob","email":"bob@example.com","active":false}` + "\n")
	f.Close()

	// Test field extraction
	fields, err := extractFields(f.Name())
	if err != nil {
		t.Fatalf("extractFields failed: %v", err)
	}

	// Fields should be sorted
	expected := []string{"active", "email", "id", "username"}
	if !reflect.DeepEqual(fields, expected) {
		t.Errorf("got %v, want %v", fields, expected)
	}
}

func TestFieldCompleter_JSON_Array(t *testing.T) {
	// Create temp JSON file with array
	f, err := os.CreateTemp("", "test*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString(`[
		{"order_id": 1, "customer_id": 100, "total": 99.99},
		{"order_id": 2, "customer_id": 101, "total": 150.00}
	]`)
	f.Close()

	// Test field extraction
	fields, err := extractFields(f.Name())
	if err != nil {
		t.Fatalf("extractFields failed: %v", err)
	}

	// Fields should be sorted
	expected := []string{"customer_id", "order_id", "total"}
	if !reflect.DeepEqual(fields, expected) {
		t.Errorf("got %v, want %v", fields, expected)
	}
}

func TestFieldCompleter_JSON_Object(t *testing.T) {
	// Create temp JSON file with single object
	f, err := os.CreateTemp("", "test*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString(`{"name": "Alice", "age": 30, "city": "NYC"}`)
	f.Close()

	// Test field extraction
	fields, err := extractFields(f.Name())
	if err != nil {
		t.Fatalf("extractFields failed: %v", err)
	}

	// Fields should be sorted
	expected := []string{"age", "city", "name"}
	if !reflect.DeepEqual(fields, expected) {
		t.Errorf("got %v, want %v", fields, expected)
	}
}

func TestFieldCompleter_Caching(t *testing.T) {
	// Clean environment
	os.Unsetenv("AUTOCLI_FIELDS")
	os.Unsetenv("AUTOCLI_FIELDS_test_csv")

	// Create temp CSV file
	f, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("field1,field2,field3\n")
	f.Close()

	// Extract fields
	fields, _ := extractFields(f.Name())
	expectedFields := []string{"field1", "field2", "field3"}
	if !reflect.DeepEqual(fields, expectedFields) {
		t.Errorf("extractFields: got %v, want %v", fields, expectedFields)
	}

	// Simulate what the bash completion script does:
	// Set environment variables manually (as the completion script would)
	fieldsStr := strings.Join(fields, ",")
	os.Setenv("AUTOCLI_FIELDS", fieldsStr)

	baseName := filepath.Base(f.Name())
	safeName := sanitizeForEnv(baseName)
	os.Setenv("AUTOCLI_FIELDS_"+safeName, fieldsStr)

	// Test getCachedFields retrieves from file-specific cache
	cached := getCachedFields(f.Name())
	if !reflect.DeepEqual(cached, fields) {
		t.Errorf("getCachedFields (file-specific): got %v, want %v", cached, fields)
	}

	// Test getCachedFields falls back to generic cache
	os.Unsetenv("AUTOCLI_FIELDS_" + safeName)
	cached = getCachedFields(f.Name())
	if !reflect.DeepEqual(cached, fields) {
		t.Errorf("getCachedFields (generic fallback): got %v, want %v", cached, fields)
	}

	// Test getCachedFields with empty filename uses generic cache
	cached = getCachedFields("")
	if !reflect.DeepEqual(cached, fields) {
		t.Errorf("getCachedFields (empty filename): got %v, want %v", cached, fields)
	}
}

func TestFieldCompleter_Complete(t *testing.T) {
	// Create temp CSV file
	f, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("name,age,email,department\n")
	f.Close()

	// Test the completer directly
	fc := &FieldCompleter{SourceFlag: "-input"}

	completionCtx := CompletionContext{
		Partial: "",
		Args:    []string{"-input", f.Name(), "-field"},
		GlobalFlags: map[string]interface{}{
			"-input": f.Name(),
		},
	}

	fields, err := fc.Complete(completionCtx)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Check fields - should include JSON directive and field names
	// Format: ["{\"type\":\"field_cache\",...}", "name", "age", "email", "department"]
	if len(fields) != 5 {
		t.Errorf("expected 5 results (JSON directive + 4 fields), got %d: %v", len(fields), fields)
	}

	// First result should be JSON directive
	if !strings.HasPrefix(fields[0], "{") {
		t.Errorf("first result should be JSON directive, got: %s", fields[0])
	}

	// Check JSON directive is valid and contains fields
	if !strings.Contains(fields[0], `"type":"field_cache"`) {
		t.Errorf("JSON directive should have type field_cache, got: %s", fields[0])
	}
	if !strings.Contains(fields[0], `"fields"`) {
		t.Errorf("JSON directive should have fields array, got: %s", fields[0])
	}

	// Remaining results should be the field names
	expectedFields := []string{"name", "age", "email", "department"}
	if !reflect.DeepEqual(fields[1:], expectedFields) {
		t.Errorf("field names: got %v, want %v", fields[1:], expectedFields)
	}
}

func TestFieldCompleter_PartialMatch(t *testing.T) {
	fields := []string{"name", "namespace", "created_at", "updated_at"}

	tests := []struct {
		partial  string
		expected []string
	}{
		{"", fields}, // Empty partial returns all
		{"n", []string{"name", "namespace"}},
		{"na", []string{"name", "namespace"}},
		{"nam", []string{"name", "namespace"}},
		{"name", []string{"name", "namespace"}},
		{"names", []string{"namespace"}},
		{"c", []string{"created_at"}},
		{"created", []string{"created_at"}},
		{"u", []string{"updated_at"}},
		{"xyz", []string{}}, // No matches
	}

	for _, tt := range tests {
		result := filterFields(fields, tt.partial)
		if !reflect.DeepEqual(result, tt.expected) {
			t.Errorf("filterFields(%q): got %v, want %v", tt.partial, result, tt.expected)
		}
	}
}

func TestSanitizeForEnv(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"data.csv", "data_csv"},
		{"users.jsonl", "users_jsonl"},
		{"my-file.txt", "my_file_txt"},
		{"file with spaces.json", "file_with_spaces_json"},
		{"file@special#chars.csv", "file_special_chars_csv"},
		{"UPPERCASE.TSV", "UPPERCASE_TSV"},
	}

	for _, tt := range tests {
		result := sanitizeForEnv(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeForEnv(%q): got %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFieldValueCompleter_CSV_WithSpecialCharacters(t *testing.T) {
	// Create temp CSV file with special characters in values
	f, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	// Values with spaces, quotes, commas, and other special chars (properly CSV escaped)
	f.WriteString("name,city,status\n")
	f.WriteString("Alice Smith,New York,active\n")
	f.WriteString("Bob O'Brien,San Francisco,pending\n")
	f.WriteString("\"Charlie \"\"Chuck\"\" Brown\",Los Angeles,inactive\n") // CSV escaped quotes
	f.WriteString("\"Diana, Princess\",London,active\n")                     // CSV escaped comma
	f.WriteString("Eve,Tel Aviv,active\n")
	f.Close()

	// Test sampling name field
	values, err := sampleCSVFieldValues(f.Name(), "name", ',', 100, 10000)
	if err != nil {
		t.Fatalf("sampleCSVFieldValues failed: %v", err)
	}

	// Should have all 5 unique names
	if len(values) != 5 {
		t.Errorf("expected 5 unique names, got %d: %v", len(values), values)
	}

	// Check that special characters are preserved (CSV reader unescapes them)
	expected := []string{
		"Alice Smith",
		"Bob O'Brien",
		"Charlie \"Chuck\" Brown",
		"Diana, Princess",
		"Eve",
	}
	if !reflect.DeepEqual(values, expected) {
		t.Errorf("got %v, want %v", values, expected)
	}

	// Test sampling city field
	cities, err := sampleCSVFieldValues(f.Name(), "city", ',', 100, 10000)
	if err != nil {
		t.Fatalf("sampleCSVFieldValues failed: %v", err)
	}

	// Should have 5 unique cities (sorted)
	expectedCities := []string{"London", "Los Angeles", "New York", "San Francisco", "Tel Aviv"}
	if !reflect.DeepEqual(cities, expectedCities) {
		t.Errorf("cities: got %v, want %v", cities, expectedCities)
	}
}

func TestFieldValueCompleter_CSV_MaxSamples(t *testing.T) {
	// Create temp CSV file with many unique values
	f, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("id,category\n")
	for i := 0; i < 200; i++ {
		f.WriteString(strings.Repeat("a", i%10+1) + "," + "cat" + strings.Repeat("x", i%5) + "\n")
	}
	f.Close()

	// Test with maxSamples = 50
	values, err := sampleCSVFieldValues(f.Name(), "id", ',', 50, 10000)
	if err != nil {
		t.Fatalf("sampleCSVFieldValues failed: %v", err)
	}

	// Should stop at 50 unique values
	if len(values) > 50 {
		t.Errorf("expected at most 50 values, got %d", len(values))
	}
}

func TestFieldValueCompleter_CSV_MaxRecords(t *testing.T) {
	// Create temp CSV file with many records
	f, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("id,value\n")
	for i := 0; i < 20000; i++ {
		f.WriteString(strings.Repeat("x", i) + ",val" + "\n")
	}
	f.Close()

	// Test with maxRecords = 100 (should stop scanning after 100 records)
	values, err := sampleCSVFieldValues(f.Name(), "id", ',', 200, 100)
	if err != nil {
		t.Fatalf("sampleCSVFieldValues failed: %v", err)
	}

	// Should have at most 100 unique values (limited by maxRecords)
	if len(values) > 100 {
		t.Errorf("expected at most 100 values, got %d", len(values))
	}
}

func TestFieldValueCompleter_JSONL_WithSpecialCharacters(t *testing.T) {
	// Create temp JSONL file with special characters in values
	f, err := os.CreateTemp("", "test*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	// Values with quotes, backslashes, unicode, etc.
	f.WriteString(`{"name":"Alice \"Admin\"","role":"admin"}` + "\n")
	f.WriteString(`{"name":"Bob\\nNewline","role":"user"}` + "\n")
	f.WriteString(`{"name":"Charlie 中文","role":"user"}` + "\n")
	f.WriteString(`{"name":"Diana's Account","role":"admin"}` + "\n")
	f.Close()

	// Test sampling name field
	values, err := sampleJSONLFieldValues(f.Name(), "name", 100, 10000)
	if err != nil {
		t.Fatalf("sampleJSONLFieldValues failed: %v", err)
	}

	// Should have all 4 unique names (sorted)
	if len(values) != 4 {
		t.Errorf("expected 4 unique names, got %d: %v", len(values), values)
	}

	// Verify special characters are preserved
	expected := []string{
		"Alice \"Admin\"",
		"Bob\\nNewline",
		"Charlie 中文",
		"Diana's Account",
	}
	if !reflect.DeepEqual(values, expected) {
		t.Errorf("got %v, want %v", values, expected)
	}
}

func TestFieldValueCompleter_JSONL_MaxSamples(t *testing.T) {
	// Create temp JSONL file with many unique values
	f, err := os.CreateTemp("", "test*.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	for i := 0; i < 150; i++ {
		f.WriteString(`{"id":"` + strings.Repeat("x", i+1) + `","value":` + strings.Repeat("1", i%10+1) + `}` + "\n")
	}
	f.Close()

	// Test with maxSamples = 30
	values, err := sampleJSONLFieldValues(f.Name(), "id", 30, 10000)
	if err != nil {
		t.Fatalf("sampleJSONLFieldValues failed: %v", err)
	}

	// Should stop at 30 unique values
	if len(values) != 30 {
		t.Errorf("expected exactly 30 values, got %d", len(values))
	}
}

func TestFieldValueCompleter_FieldNotFound(t *testing.T) {
	// Create temp CSV file
	f, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("name,age\n")
	f.WriteString("Alice,30\n")
	f.Close()

	// Try to sample a field that doesn't exist
	values, err := sampleCSVFieldValues(f.Name(), "nonexistent", ',', 100, 10000)
	if err == nil {
		t.Error("expected error for nonexistent field, got nil")
	}
	if values != nil {
		t.Errorf("expected nil values for error case, got %v", values)
	}
}

func TestFieldValueCompleter_EmptyValues(t *testing.T) {
	// Create temp CSV file with some empty values
	f, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("name,status\n")
	f.WriteString("Alice,active\n")
	f.WriteString("Bob,\n") // Empty status
	f.WriteString("Charlie,inactive\n")
	f.WriteString(" ,pending\n") // Name with only whitespace (will be trimmed to empty)
	f.Close()

	// Sample name field - should skip empty values
	values, err := sampleCSVFieldValues(f.Name(), "name", ',', 100, 10000)
	if err != nil {
		t.Fatalf("sampleCSVFieldValues failed: %v", err)
	}

	// Should have all non-empty names
	expected := []string{"Alice", "Bob", "Charlie"}
	if !reflect.DeepEqual(values, expected) {
		t.Errorf("got %v, want %v", values, expected)
	}
}

func TestFieldValueCompleter_DuplicateValues(t *testing.T) {
	// Create temp CSV file with duplicate values
	f, err := os.CreateTemp("", "test*.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	f.WriteString("status,count\n")
	f.WriteString("active,10\n")
	f.WriteString("inactive,5\n")
	f.WriteString("active,15\n") // Duplicate status
	f.WriteString("pending,3\n")
	f.WriteString("active,20\n") // Another duplicate
	f.Close()

	// Sample status field - should have unique values only
	values, err := sampleCSVFieldValues(f.Name(), "status", ',', 100, 10000)
	if err != nil {
		t.Fatalf("sampleCSVFieldValues failed: %v", err)
	}

	// Should have 3 unique statuses
	expected := []string{"active", "inactive", "pending"}
	if !reflect.DeepEqual(values, expected) {
		t.Errorf("got %v, want %v", values, expected)
	}
}
