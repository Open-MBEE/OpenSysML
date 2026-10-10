package repl

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
)

// progressPrinter writes the progress an external engine reports while a plan runs, one
// line per coalesced report, serialized since the reports arrive from the engines' readers.
type progressPrinter struct {
	mu sync.Mutex
	w  io.Writer
}

// report is the analysis.Reporter the printer installs: `engine <name>: runs 120, depth 17`.
func (p *progressPrinter) report(r analysis.ProgressReport) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.w, "engine %s: %s\n", r.Engine, analysis.ProgressText(r.Progress))
}

// SetProgress names where the progress external engines report while a plan runs is
// printed — the CLI's standard error; a nil writer prints none, the default.
func (s *Session) SetProgress(w io.Writer) {
	defer s.enter()()
	s.progress = nil
	if w != nil {
		s.progress = &progressPrinter{w: w}
	}
	if s.rtCtx != nil {
		s.attachTools(s.rtCtx)
	}
}

// planContext is the context every plan of the session runs under, with the progress
// reporter installed when the session prints progress.
func (s *Session) planContext() context.Context {
	return s.reporting(s.command.context())
}

// heldPlanContext is planContext for a plan put from state kept across commands —
// the runtime the debuggers step — so it follows the command under way when put.
func (s *Session) heldPlanContext() context.Context {
	return s.reporting(s.command.scoped())
}

func (s *Session) reporting(ctx context.Context) context.Context {
	if s.progress == nil {
		return ctx
	}
	return analysis.WithReporter(ctx, s.progress.report)
}
