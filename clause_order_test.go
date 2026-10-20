package completionflags

import (
	"reflect"
	"testing"
)

// Clause.Order: local flags by occurrence, per clause; global flags and
// positionals are not in it; a + spelling records the canonical name.
func TestClauseOrder(t *testing.T) {
	var got []Clause
	cmd := NewCommand("test").
		Flag("-set").Arg("field").Done().Arg("value").Done().Accumulate().Local().Done().
		Flag("-set-field").Arg("field").Done().Arg("source").Done().Accumulate().Local().Done().
		Flag("-if-expr").Arg("expr").Done().Accumulate().Local().Done().
		Flag("-desc").Bool().Local().Done().
		Flag("-generate").Bool().Global().Done().
		Handler(func(ctx *Context) error {
			got = append([]Clause{}, ctx.Clauses...)
			return nil
		}).
		Build()
	args := []string{"-set-field", "who", "name", "-generate", "-set", "tag", "1", "+if-expr", "a > 1", "x", "-set", "k", "2",
		"+", "-desc", "-set", "z", "3", "-set-field", "w", "name"}
	if err := cmd.Execute(args); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("clauses = %d", len(got))
	}
	if want := []string{"-set-field", "-set", "-if-expr", "-set"}; !reflect.DeepEqual(got[0].Order, want) {
		t.Errorf("clause 0 order = %q, want %q", got[0].Order, want)
	}
	if want := []string{"-desc", "-set", "-set-field"}; !reflect.DeepEqual(got[1].Order, want) {
		t.Errorf("clause 1 order = %q, want %q", got[1].Order, want)
	}
	// The counters line up with the accumulated lists.
	sets := got[0].Flags["-set"].([]interface{})
	if len(sets) != 2 || sets[1].(map[string]interface{})["field"] != "k" {
		t.Errorf("clause 0 -set = %v", sets)
	}
	if _, in := got[0].Flags["-generate"]; in {
		t.Error("a global flag landed in the clause")
	}
}
