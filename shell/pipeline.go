package shell

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	cf "github.com/rosscartlidge/autocli/v4"
)

// splitOnPipe splits a tokenised argv on the literal "|" token,
// returning one slice per stage. A trailing or leading pipe — or two
// consecutive pipes — returns an error (the empty stage between them
// has nothing to dispatch).
//
//	[]string{"from-loaded", "|", "where", "-if", "x"}
//	  → [][]string{{"from-loaded"}, {"where", "-if", "x"}}, nil, true
//
// Returns hasPipe=false when no "|" token is present, so callers can
// take the single-command fast path.
func splitOnPipe(args []string) (stages [][]string, hasPipe bool, err error) {
	var current []string
	for i, a := range args {
		if a != "|" {
			current = append(current, a)
			continue
		}
		hasPipe = true
		if len(current) == 0 {
			return nil, true, fmt.Errorf("empty stage before `|` at position %d", i)
		}
		stages = append(stages, current)
		current = nil
	}
	if !hasPipe {
		return nil, false, nil
	}
	if len(current) == 0 {
		return nil, true, fmt.Errorf("empty stage after `|`")
	}
	stages = append(stages, current)
	return stages, true, nil
}

// splitStagesForCompletion splits tokenised args on "|" into stages
// for TAB completion. Unlike splitOnPipe (the execution splitter), it
// TOLERATES empty stages — a trailing "|" or an empty stage under the
// cursor is normal mid-typing, not an error — and always returns at
// least one stage. The final element is the stage being completed; the
// preceding elements are its upstream.
//
//	[]string{"from-loaded", "|", "group-by", ""}
//	  → [][]string{{"from-loaded"}, {"group-by", ""}}
func splitStagesForCompletion(args []string) [][]string {
	stages := [][]string{}
	current := []string{}
	for _, a := range args {
		if a == "|" {
			stages = append(stages, current)
			current = []string{}
			continue
		}
		current = append(current, a)
	}
	return append(stages, current)
}

// runPipeline runs a multi-stage pipeline. Stages run concurrently
// in goroutines; adjacent stages are connected by io.Pipe. Stage 0
// reads from an empty reader (sources don't need stdin; non-sources
// at position 0 see EOF immediately, which is correct since there's
// nothing upstream). Stage N-1 writes to base.Stdout(). Errors from
// any stage are collected; the first non-nil error is returned.
//
// Closing the write end of each pipe when its stage finishes is
// what signals EOF to the next stage. Closing the read end on
// upstream error is what makes upstream stages unblock when their
// downstream goes away.
func runPipeline(cli *cf.Command, stages [][]string, base *cf.Context) error {
	n := len(stages)
	if n == 0 {
		return nil
	}

	// Allocate pipes. readers[i] is stage i's stdin; writers[i] is
	// stage i's stdout. readers[0] is empty-EOF; writers[n-1] is
	// base.Stdout(). The intermediate pairs share PipeReader/Writer.
	readers := make([]io.Reader, n)
	writers := make([]io.Writer, n)
	pipeWriters := make([]*io.PipeWriter, n)
	pipeReaders := make([]*io.PipeReader, n)

	readers[0] = bytes.NewReader(nil)
	writers[n-1] = base.Stdout()
	for i := 0; i < n-1; i++ {
		pr, pw := io.Pipe()
		writers[i] = pw
		readers[i+1] = pr
		pipeWriters[i] = pw
		pipeReaders[i+1] = pr
	}

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i, stage := range stages {
		wg.Add(1)
		go func(i int, stage []string) {
			defer wg.Done()

			// Close my pipe writer when I finish so the next stage
			// gets EOF on its stdin. Last stage's writer is the
			// shell's output — leave it alone.
			if i < n-1 {
				defer pipeWriters[i].Close()
			}
			// Close my pipe reader on exit so an upstream stage that
			// errored mid-write unblocks. First stage's reader is the
			// empty bytes.Reader — no-op to close.
			if i > 0 {
				defer pipeReaders[i].Close()
			}

			// Derive a per-stage Context with the right Stdin/Stdout.
			// Inheriting State + Stderr + Ctx from base means the
			// stage sees the same service handle and the same
			// cancellation signal.
			stageCtx := &cf.Context{State: base.State}
			stageCtx.SetStdin(readers[i])
			stageCtx.SetStdout(writers[i])
			stageCtx.SetStderr(base.Stderr())
			stageCtx.SetCtx(base.Ctx())

			errs[i] = cli.ExecuteWith(stage, stageCtx)
		}(i, stage)
	}
	wg.Wait()

	// Surface the first non-nil error. Friendly wrapper for unknown
	// commands so the user sees the same message they'd get for a
	// single-command typo.
	//
	// io.ErrClosedPipe from upstream stages is suppressed: when a
	// downstream stage (e.g. `limit 3`) finishes early, the pipe runner
	// closes the read end, and upstream writes fail with "io: read/write
	// on closed pipe". That's the Unix SIGPIPE convention — the consumer
	// is done, not an error — so we treat it as a normal early exit.
	for i, err := range errs {
		if err == nil {
			continue
		}
		if errors.Is(err, io.ErrClosedPipe) {
			continue
		}
		var unknown cf.ErrUnknownCommand
		if errors.As(err, &unknown) {
			return fmt.Errorf("stage %d: unknown command %q (try -help or :help)", i+1, string(unknown))
		}
		return fmt.Errorf("stage %d (%s): %w", i+1, strings.Join(stages[i], " "), err)
	}
	return nil
}
