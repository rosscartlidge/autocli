package completionflags

import (
	"testing"
)

func TestSinglePositional(t *testing.T) {
	var file string
	cmd := NewCommand("test").
		Flag("FILE").String().Global().Done().
		Handler(func(ctx *Context) error {
			file = ctx.GetString("FILE", "")
			return nil
		}).
		Build()

	err := cmd.Execute([]string{"input.txt"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if file != "input.txt" {
		t.Errorf("Expected file='input.txt', got %q", file)
	}
}

func TestMultiplePositionals(t *testing.T) {
	var src, dst string
	cmd := NewCommand("test").
		Flag("SRC").String().Global().Done().
		Flag("DST").String().Global().Done().
		Handler(func(ctx *Context) error {
			src = ctx.GetString("SRC", "")
			dst = ctx.GetString("DST", "")
			return nil
		}).
		Build()

	err := cmd.Execute([]string{"a.txt", "b.txt"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if src != "a.txt" {
		t.Errorf("Expected src='a.txt', got %q", src)
	}
	if dst != "b.txt" {
		t.Errorf("Expected dst='b.txt', got %q", dst)
	}
}

func TestVariadic(t *testing.T) {
	var files []string
	cmd := NewCommand("test").
		Flag("FILES").StringSlice().Variadic().Global().Done().
		Handler(func(ctx *Context) error {
			// StringSlice values are stored as []interface{}, need to convert
			if val, ok := ctx.GlobalFlags["FILES"]; ok {
				if interfaceSlice, ok := val.([]interface{}); ok {
					files = make([]string, len(interfaceSlice))
					for i, v := range interfaceSlice {
						files[i] = v.(string)
					}
				}
			}
			return nil
		}).
		Build()

	err := cmd.Execute([]string{"a.txt", "b.txt", "c.txt"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("Expected 3 files, got %d", len(files))
	}
	if files[0] != "a.txt" || files[1] != "b.txt" || files[2] != "c.txt" {
		t.Errorf("Expected [a.txt b.txt c.txt], got %v", files)
	}
}

func TestMixedFlagsAndPositionals(t *testing.T) {
	var verbose bool
	var file string
	cmd := NewCommand("test").
		Flag("-v").Bool().Global().Done().
		Flag("FILE").String().Global().Done().
		Handler(func(ctx *Context) error {
			verbose = ctx.GetBool("-v", false)
			file = ctx.GetString("FILE", "")
			return nil
		}).
		Build()

	err := cmd.Execute([]string{"-v", "input.txt"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !verbose {
		t.Error("Expected verbose=true")
	}
	if file != "input.txt" {
		t.Errorf("Expected file='input.txt', got %q", file)
	}
}

func TestRequiredPositional(t *testing.T) {
	cmd := NewCommand("test").
		Flag("FILE").String().Required().Global().Done().
		Handler(func(ctx *Context) error { return nil }).
		Build()

	// Should fail without argument
	err := cmd.Execute([]string{})
	if err == nil {
		t.Error("Expected error for missing required positional")
	}
}

func TestPositionalWithDefault(t *testing.T) {
	var file string
	cmd := NewCommand("test").
		Flag("FILE").String().Default("default.txt").Global().Done().
		Handler(func(ctx *Context) error {
			file = ctx.GetString("FILE", "default.txt")
			return nil
		}).
		Build()

	err := cmd.Execute([]string{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if file != "default.txt" {
		t.Errorf("Expected file='default.txt', got %q", file)
	}
}

func TestPositionalInteger(t *testing.T) {
	var count int
	cmd := NewCommand("test").
		Flag("COUNT").Int().Global().Done().
		Handler(func(ctx *Context) error {
			count = ctx.GetInt("COUNT", 0)
			return nil
		}).
		Build()

	err := cmd.Execute([]string{"42"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if count != 42 {
		t.Errorf("Expected count=42, got %d", count)
	}
}

// Build-time validation tests

func TestVariadicMustBeLast(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for variadic not last")
		}
	}()

	NewCommand("test").
		Flag("FILES").StringSlice().Variadic().Global().Done().
		Flag("OTHER").String().Global().Done().
		Handler(func(ctx *Context) error { return nil }).
		Build()
}

func TestOnlyOneVariadic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for multiple variadics")
		}
	}()

	NewCommand("test").
		Flag("FILES1").StringSlice().Variadic().Global().Done().
		Flag("FILES2").StringSlice().Variadic().Global().Done().
		Handler(func(ctx *Context) error { return nil }).
		Build()
}

func TestRequiredAndDefaultMutuallyExclusive(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic for Required + Default")
		}
	}()

	NewCommand("test").
		Flag("FILE").String().Required().Default("x").Global().Done().
		Handler(func(ctx *Context) error { return nil }).
		Build()
}
