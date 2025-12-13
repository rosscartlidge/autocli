package completionflags

import (
	"os"
	"strings"
	"testing"
)

func TestFileCompleter_AutoCache_SingleDataFile(t *testing.T) {
	// Create a temp directory with a single CSV file
	dir, err := os.MkdirTemp("", "autocli_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a CSV file in the temp directory
	csvPath := dir + "/users.csv"
	f, err := os.Create(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("name,age,email\n")
	f.WriteString("Alice,30,alice@example.com\n")
	f.Close()

	// Create FileCompleter with CSV pattern
	fc := &FileCompleter{Pattern: "*.csv"}

	// Complete with partial "users" which should match only users.csv
	ctx := CompletionContext{
		Partial: "users",
	}

	// Change to temp directory for completion
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	results, err := fc.Complete(ctx)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Should have 2 results: JSON directive + filename
	if len(results) != 2 {
		t.Errorf("expected 2 results (directive + filename), got %d: %v", len(results), results)
		return
	}

	// First result should be JSON directive
	if !strings.HasPrefix(results[0], `{"type":"field_cache"`) {
		t.Errorf("first result should be JSON field_cache directive, got: %s", results[0])
	}

	// Verify directive contains fields
	if !strings.Contains(results[0], `"name"`) || !strings.Contains(results[0], `"age"`) || !strings.Contains(results[0], `"email"`) {
		t.Errorf("directive should contain field names, got: %s", results[0])
	}

	// Verify directive contains filepath
	if !strings.Contains(results[0], `"filepath"`) {
		t.Errorf("directive should contain filepath, got: %s", results[0])
	}

	// Second result should be the filename
	if results[1] != "users.csv" {
		t.Errorf("second result should be 'users.csv', got: %s", results[1])
	}
}

func TestFileCompleter_NoCache_MultipleMatches(t *testing.T) {
	// Create a temp directory with multiple CSV files
	dir, err := os.MkdirTemp("", "autocli_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create multiple CSV files
	for _, name := range []string{"users.csv", "orders.csv"} {
		f, err := os.Create(dir + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		f.WriteString("col1,col2\n")
		f.Close()
	}

	fc := &FileCompleter{Pattern: "*.csv"}

	ctx := CompletionContext{
		Partial: "", // Match all CSVs
	}

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	results, err := fc.Complete(ctx)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Multiple matches - should NOT emit cache directive
	for _, result := range results {
		if strings.HasPrefix(result, `{"type":"field_cache"`) {
			t.Errorf("should NOT emit cache directive with multiple matches, got: %s", result)
		}
	}
}

func TestFileCompleter_NoCache_Directory(t *testing.T) {
	// Create a temp directory with a subdirectory
	dir, err := os.MkdirTemp("", "autocli_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a subdirectory
	subdir := dir + "/subdir"
	os.Mkdir(subdir, 0755)

	fc := &FileCompleter{}

	ctx := CompletionContext{
		Partial: "sub", // Should match subdir/
	}

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	results, err := fc.Complete(ctx)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Directory match - should NOT emit cache directive
	for _, result := range results {
		if strings.HasPrefix(result, `{"type":"field_cache"`) {
			t.Errorf("should NOT emit cache directive for directory, got: %s", result)
		}
	}
}

func TestFileCompleter_NoCache_NonDataFile(t *testing.T) {
	// Create a temp directory with a non-data file
	dir, err := os.MkdirTemp("", "autocli_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create a Go file (not a data file)
	f, err := os.Create(dir + "/main.go")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("package main\n")
	f.Close()

	fc := &FileCompleter{Pattern: "*.go"}

	ctx := CompletionContext{
		Partial: "main",
	}

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	results, err := fc.Complete(ctx)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	// Non-data file - should NOT emit cache directive
	for _, result := range results {
		if strings.HasPrefix(result, `{"type":"field_cache"`) {
			t.Errorf("should NOT emit cache directive for non-data file, got: %s", result)
		}
	}
}

func TestIsDataFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"data.csv", true},
		{"data.CSV", true},
		{"data.tsv", true},
		{"data.json", true},
		{"data.jsonl", true},
		{"data.ndjson", true},
		{"data.txt", false},
		{"data.go", false},
		{"data.xml", false},
		{"users", false},
		{"/path/to/data.csv", true},
		{"/path/to/data.JSON", true},
	}

	for _, tt := range tests {
		result := isDataFile(tt.path)
		if result != tt.expected {
			t.Errorf("isDataFile(%q): got %v, want %v", tt.path, result, tt.expected)
		}
	}
}
