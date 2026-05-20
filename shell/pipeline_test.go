package shell

import (
	"bufio"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	cf "github.com/rosscartlidge/autocli/v4"
)

func TestSplitOnPipe(t *testing.T) {
	cases := []struct {
		in         []string
		wantStages [][]string
		wantHas    bool
		wantErr    bool
	}{
		{nil, nil, false, false},
		{[]string{"foo"}, nil, false, false},
		{[]string{"foo", "|", "bar"}, [][]string{{"foo"}, {"bar"}}, true, false},
		{[]string{"a", "b", "|", "c", "d", "|", "e"}, [][]string{{"a", "b"}, {"c", "d"}, {"e"}}, true, false},
		{[]string{"|", "bar"}, nil, true, true},     // leading pipe
		{[]string{"foo", "|"}, nil, true, true},     // trailing pipe
		{[]string{"a", "|", "|", "b"}, nil, true, true}, // empty stage
	}
	for _, tc := range cases {
		stages, has, err := splitOnPipe(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("splitOnPipe(%v): err=%v, wantErr=%v", tc.in, err, tc.wantErr)
			continue
		}
		if has != tc.wantHas {
			t.Errorf("splitOnPipe(%v): hasPipe=%v, want %v", tc.in, has, tc.wantHas)
		}
		if !tc.wantErr && !reflect.DeepEqual(stages, tc.wantStages) {
			t.Errorf("splitOnPipe(%v): stages=%v, want %v", tc.in, stages, tc.wantStages)
		}
	}
}

// pipelineTestCLI builds a tree with three commands that read/write
// JSONL-style lines and behave well in pipelines:
//
//	emit  — source: prints lines "1".."N" (default 5; -n N to change)
//	double — transform: reads lines, prints each twice
//	tag   — transform: reads lines, prepends a string (default ">")
//
// Each command uses ctx.Stdin/Stdout via the Phase A engine split,
// matching the contract a real ssql command would have.
func pipelineTestCLI() *cf.Command {
	return cf.NewCommand("svc").
		Subcommand("emit").
		Flag("-n").Int().Global().Default(int64(5)).Done().
		Handler(func(ctx *cf.Context) error {
			n := 5
			switch v := ctx.GlobalFlags["-n"].(type) {
			case int:
				n = v
			case int64:
				n = int(v)
			}
			for i := 1; i <= n; i++ {
				fmt.Fprintln(ctx.Stdout(), i)
			}
			return nil
		}).
		Done().
		Subcommand("double").
		Handler(func(ctx *cf.Context) error {
			sc := bufio.NewScanner(ctx.Stdin())
			for sc.Scan() {
				fmt.Fprintln(ctx.Stdout(), sc.Text())
				fmt.Fprintln(ctx.Stdout(), sc.Text())
			}
			return sc.Err()
		}).
		Done().
		Subcommand("tag").
		Flag("-p").String().Global().Default(">").Done().
		Handler(func(ctx *cf.Context) error {
			p, _ := ctx.GlobalFlags["-p"].(string)
			sc := bufio.NewScanner(ctx.Stdin())
			for sc.Scan() {
				fmt.Fprintln(ctx.Stdout(), p, sc.Text())
			}
			return sc.Err()
		}).
		Done().
		Subcommand("boom").
		Handler(func(ctx *cf.Context) error {
			return fmt.Errorf("boom intentionally errored")
		}).
		Done().
		Build()
}

// TestServe_Pipeline_TwoStages exercises the happy path:
// `emit -n 3 | tag` runs both stages, lines flow through.
func TestServe_Pipeline_TwoStages(t *testing.T) {
	cli := pipelineTestCLI()
	out := runShellWithInput(t, cli, Options{}, "emit -n 3 | tag\n:exit\n")
	for _, want := range []string{"> 1", "> 2", "> 3"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output; got: %q", want, out)
		}
	}
}

// TestServe_Pipeline_ThreeStages with branching: emit -n 2 | double | tag -p '+'.
// Should produce six lines starting with "+".
func TestServe_Pipeline_ThreeStages(t *testing.T) {
	cli := pipelineTestCLI()
	out := runShellWithInput(t, cli, Options{}, "emit -n 2 | double | tag -p +\n:exit\n")
	count := strings.Count(out, "+ ")
	if count != 4 {
		t.Errorf("expected 4 `+ ` prefixed lines (2 emits × 2 doubled); got %d in: %q", count, out)
	}
}

// TestServe_Pipeline_ErrorAtStage asserts a non-nil error from any
// stage is surfaced to the operator without ending the session.
func TestServe_Pipeline_ErrorAtStage(t *testing.T) {
	cli := pipelineTestCLI()
	out := runShellWithInput(t, cli, Options{}, "emit -n 3 | boom\n:exit\n")
	if !strings.Contains(out, "boom intentionally errored") {
		t.Errorf("error not surfaced: %q", out)
	}
}

// TestServe_Pipeline_UnknownCommand asserts a typo'd stage surfaces
// the friendly "unknown command" message scoped to its stage number.
func TestServe_Pipeline_UnknownCommand(t *testing.T) {
	cli := pipelineTestCLI()
	out := runShellWithInput(t, cli, Options{}, "emit | nosuchstage | tag\n:exit\n")
	if !strings.Contains(out, "unknown command") || !strings.Contains(out, "nosuchstage") {
		t.Errorf("expected friendly unknown-command-at-stage; got: %q", out)
	}
}

// TestServe_Pipeline_EmptyStageError asserts `foo | | bar` is
// rejected with a clear error.
func TestServe_Pipeline_EmptyStageError(t *testing.T) {
	cli := pipelineTestCLI()
	out := runShellWithInput(t, cli, Options{}, "emit | | tag\n:exit\n")
	if !strings.Contains(out, "empty stage") {
		t.Errorf("expected `empty stage` error; got: %q", out)
	}
}

// TestServe_SinglePipeUnchanged ensures the no-pipe fast path still
// works after the pipeline detection was added.
func TestServe_SinglePipeUnchanged(t *testing.T) {
	cli := pipelineTestCLI()
	out := runShellWithInput(t, cli, Options{}, "emit -n 2\n:exit\n")
	if !strings.Contains(out, "1\r\n") || !strings.Contains(out, "2\r\n") {
		t.Errorf("single-command path broken; got: %q", out)
	}
}

// Unused but here so io is "used" for documentation parity; trimmer
// for future maintenance.
var _ = io.EOF
