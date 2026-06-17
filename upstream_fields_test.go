package completionflags

import (
	"reflect"
	"testing"
)

// TestFieldsFromFlag_SeedsUpstream confirms that a flag wired with
// FieldsFromFlag automatically offers seeded upstream fields (because
// FieldsFromFlag now installs a ChainCompleter{UpstreamFieldsCompleter,
// FieldCompleter}). This is what gives every ssql field flag
// pipeline-aware completion without per-flag changes. Without a seed
// and without a source file, it falls back to nothing.
func TestFieldsFromFlag_SeedsUpstream(t *testing.T) {
	cmd := NewCommand("svc").
		Subcommand("pick").
		Flag("-field").
		String().
		FieldsFromFlag(""). // no source flag → file fallback yields nothing
		Done().
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()

	// Seeded: upstream fields surface through the chain.
	got, err := cmd.CompleteWithContext(
		[]string{"pick", "-field", ""}, 3,
		CompletionContext{UpstreamFields: []string{"name", "dept"}})
	if err != nil {
		t.Fatalf("CompleteWithContext: %v", err)
	}
	if want := []string{"name", "dept"}; !reflect.DeepEqual(got, want) {
		t.Errorf("seeded: got %v, want %v", got, want)
	}

	// Unseeded (bash path): the chain falls through to FieldCompleter.
	// With no source file it yields the file-fallback hint, NOT the
	// upstream fields — proving the seed is what surfaces them.
	got, err = cmd.Complete([]string{"pick", "-field", ""}, 3)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if reflect.DeepEqual(got, []string{"name", "dept"}) {
		t.Errorf("unseeded should not return upstream fields, got %v", got)
	}
}

// TestUpstreamFieldsCompleter exercises the completer in isolation:
// prefix matching (case-insensitive), the empty-partial "offer all"
// case, and the no-upstream-fields case (returns nothing so it can
// fall back under a ChainCompleter).
func TestUpstreamFieldsCompleter(t *testing.T) {
	c := UpstreamFieldsCompleter{}
	fields := []string{"name", "dept", "salary", "Department"}

	cases := []struct {
		name    string
		partial string
		fields  []string
		want    []string
	}{
		{"empty partial offers all", "", fields, []string{"name", "dept", "salary", "Department"}},
		{"prefix match", "de", fields, []string{"dept", "Department"}},
		{"case-insensitive", "DE", fields, []string{"dept", "Department"}},
		{"no match", "zzz", fields, nil},
		{"no upstream fields", "any", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.Complete(CompletionContext{Partial: tc.partial, UpstreamFields: tc.fields})
			if err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// pickCmd builds: svc pick -field <UpstreamFieldsCompleter> -x <reads State>
func pickCmd() *Command {
	return NewCommand("svc").
		Subcommand("pick").
		Flag("-field").
		String().
		Completer(UpstreamFieldsCompleter{}).
		Done().
		Flag("-x").
		String().
		CompleterFunc(func(ctx CompletionContext) ([]string, error) {
			if s, ok := ctx.State.([]string); ok {
				return s, nil
			}
			return nil, nil
		}).
		Done().
		Handler(func(ctx *Context) error { return nil }).
		Done().
		Build()
}

// TestCompleteWithContext_UpstreamFields verifies the seeded
// UpstreamFields reach a leaf subcommand's flag completer — i.e. the
// seed threads through completeWithSubcommands, not just the top level.
func TestCompleteWithContext_UpstreamFields(t *testing.T) {
	cmd := pickCmd()
	seed := CompletionContext{UpstreamFields: []string{"name", "dept", "salary"}}

	// Complete `svc pick -field <TAB>`: COMP_WORDS = svc pick -field "",
	// so args = [pick -field ""], pos = 3.
	got, err := cmd.CompleteWithContext([]string{"pick", "-field", ""}, 3, seed)
	if err != nil {
		t.Fatalf("CompleteWithContext: %v", err)
	}
	want := []string{"name", "dept", "salary"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// With a partial, only the prefix match comes back.
	got, err = cmd.CompleteWithContext([]string{"pick", "-field", "d"}, 3, seed)
	if err != nil {
		t.Fatalf("CompleteWithContext: %v", err)
	}
	if want := []string{"dept"}; !reflect.DeepEqual(got, want) {
		t.Errorf("partial 'd': got %v, want %v", got, want)
	}
}

// TestCompleteWithContext_State verifies seed.State reaches a completer
// (the completion-time analogue of Context.State on the exec path).
func TestCompleteWithContext_State(t *testing.T) {
	cmd := pickCmd()
	seed := CompletionContext{State: []string{"alpha", "beta"}}

	got, err := cmd.CompleteWithContext([]string{"pick", "-x", ""}, 3, seed)
	if err != nil {
		t.Fatalf("CompleteWithContext: %v", err)
	}
	if want := []string{"alpha", "beta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestComplete_NoSeed_LeavesContextZero confirms the plain Complete
// path (bash / CLI) injects neither UpstreamFields nor State, so an
// UpstreamFieldsCompleter offers nothing and behaviour is unchanged.
func TestComplete_NoSeed_LeavesContextZero(t *testing.T) {
	cmd := pickCmd()
	got, err := cmd.Complete([]string{"pick", "-field", ""}, 3)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no completions without a seed, got %v", got)
	}
}
