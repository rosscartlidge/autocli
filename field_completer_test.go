package completionflags

import (
	"os"
	"path/filepath"
	"reflect"
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

	// Extract and cache fields
	fields, _ := extractFields(f.Name())
	cacheFields(f.Name(), fields)

	// Check generic cache
	genericCache := os.Getenv("AUTOCLI_FIELDS")
	if genericCache != "field1,field2,field3" {
		t.Errorf("generic cache: got %q, want %q", genericCache, "field1,field2,field3")
	}

	// Check file-specific cache
	baseName := filepath.Base(f.Name())
	safeName := sanitizeForEnv(baseName)
	fileCache := os.Getenv("AUTOCLI_FIELDS_" + safeName)
	if fileCache != "field1,field2,field3" {
		t.Errorf("file-specific cache: got %q, want %q", fileCache, "field1,field2,field3")
	}

	// Test retrieval
	cached := getCachedFields(f.Name())
	if !reflect.DeepEqual(cached, fields) {
		t.Errorf("getCachedFields: got %v, want %v", cached, fields)
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

	// Check fields
	expected := []string{"name", "age", "email", "department"}
	if !reflect.DeepEqual(fields, expected) {
		t.Errorf("completion fields: got %v, want %v", fields, expected)
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
