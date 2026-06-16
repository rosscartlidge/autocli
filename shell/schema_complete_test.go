package shell

import (
	"reflect"
	"strings"
	"testing"

	cf "github.com/rosscartlidge/autocli/v4"
)

// groupByTree mirrors the shape ssql's serve CLI exposes: a from-loaded
// source and a group-by whose FIELDS positional completes from the
// upstream schema (UpstreamFieldsCompleter, seeded via SchemaWalk).
func groupByTree() *cf.Command {
	noop := func(ctx *cf.Context) error { return nil }
	return cf.NewCommand("svc").
		Subcommand("from-loaded").
		Handler(noop).
		Done().
		Subcommand("group-by").
		Flag("FIELDS").
		String().
		Variadic().
		Completer(cf.UpstreamFieldsCompleter{}).
		Global().
		Done().
		Handler(noop).
		Done().
		Build()
}

// TestTabComplete_SchemaWalkSeedsUpstreamFields is the slice-4 payoff:
// `from-loaded | group-by <TAB>` offers a field the SchemaWalk hook
// reports, and the hook is handed the correct upstream stages + state.
func TestTabComplete_SchemaWalkSeedsUpstreamFields(t *testing.T) {
	cli := groupByTree()

	type svcState struct{ name string }
	state := &svcState{name: "loaded"}

	var gotUpstream [][]string
	var gotState any
	walk := func(s any, upstream [][]string) ([]string, bool) {
		gotState = s
		gotUpstream = upstream
		return []string{"salary"}, true // single match → inserted
	}

	line := "from-loaded | group-by "
	var termSink, listSink strings.Builder
	newLine, newPos, ok := tabComplete(cli, line, len(line), &termSink, &listSink, state, walk)

	if !ok {
		t.Fatalf("expected an insertion; got none (list=%q)", listSink.String())
	}
	if want := "from-loaded | group-by salary "; newLine != want {
		t.Errorf("newLine = %q, want %q", newLine, want)
	}
	if newPos != len(newLine) {
		t.Errorf("newPos = %d, want %d", newPos, len(newLine))
	}

	// The hook must receive only the upstream stage(s) and the session state.
	if want := [][]string{{"from-loaded"}}; !reflect.DeepEqual(gotUpstream, want) {
		t.Errorf("walk upstream = %v, want %v", gotUpstream, want)
	}
	if gotState != state {
		t.Errorf("walk state = %v, want the session state %v", gotState, state)
	}
}

// TestTabComplete_SchemaWalkUndeterminable confirms ok=false from the
// hook seeds no fields, so the field completer offers nothing rather
// than wrong suggestions.
func TestTabComplete_SchemaWalkUndeterminable(t *testing.T) {
	cli := groupByTree()
	walk := func(any, [][]string) ([]string, bool) { return nil, false }

	line := "from-loaded | group-by "
	var termSink, listSink strings.Builder
	_, _, ok := tabComplete(cli, line, len(line), &termSink, &listSink, nil, walk)
	if ok {
		t.Errorf("expected no insertion when SchemaWalk is undeterminable")
	}
}

// TestTabComplete_NoSchemaWalk confirms that without a hook, a field
// position offers nothing from upstream — completion is unchanged from
// pre-slice-4 behaviour (the bash/CLI path).
func TestTabComplete_NoSchemaWalk(t *testing.T) {
	cli := groupByTree()

	line := "from-loaded | group-by "
	var termSink, listSink strings.Builder
	_, _, ok := tabComplete(cli, line, len(line), &termSink, &listSink, nil, nil)
	if ok {
		t.Errorf("expected no insertion without a SchemaWalk hook")
	}
}

// TestTabComplete_FirstStageNoUpstream confirms the hook is not called
// when the cursor is on the first stage (no upstream to walk).
func TestTabComplete_FirstStageNoUpstream(t *testing.T) {
	cli := groupByTree()
	called := false
	walk := func(any, [][]string) ([]string, bool) { called = true; return nil, true }

	line := "group-by "
	var termSink, listSink strings.Builder
	tabComplete(cli, line, len(line), &termSink, &listSink, nil, walk)
	if called {
		t.Errorf("SchemaWalk should not be called when there is no upstream stage")
	}
}

func TestSplitStagesForCompletion(t *testing.T) {
	cases := []struct {
		in   []string
		want [][]string
	}{
		{nil, [][]string{{}}},
		{[]string{"group-by", ""}, [][]string{{"group-by", ""}}},
		{[]string{"from-loaded", "|", "group-by", ""}, [][]string{{"from-loaded"}, {"group-by", ""}}},
		{[]string{"from-loaded", "|"}, [][]string{{"from-loaded"}, {}}},
		{[]string{"a", "|", "b", "|", "c"}, [][]string{{"a"}, {"b"}, {"c"}}},
	}
	for _, c := range cases {
		got := splitStagesForCompletion(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("split(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
