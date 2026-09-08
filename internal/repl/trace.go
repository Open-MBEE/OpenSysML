package repl

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// choiceSummary is the line a debugger command adds when the steps it ran made
// choice points beyond the before it started with: how many, and how to see them.
func (s *Session) choiceSummary(ctx *runtime.Context, before int) []string {
	n := ctx.ChoiceCount() - before
	if n <= 0 {
		return nil
	}
	line := fmt.Sprintf("  %d choice point", n)
	if n > 1 {
		line += "s"
	}
	if s.trace == nil {
		line += "; %trace on to see them"
	}
	return []string{line}
}

// tracePrefix marks a recorded execution step, so a trace is distinguishable
// from a command's own output.
const tracePrefix = "[trace] "

// Tracing reports whether execution steps are being recorded.
func (s *Session) Tracing() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.trace != nil
}

// SetTracing turns recording of execution steps on or off. It takes effect at
// once, on the session's runtime context and on a debugging session already
// under way as well as on everything created afterwards.
func (s *Session) SetTracing(on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setTracing(on)
}

func (s *Session) setTracing(on bool) {
	switch {
	case on && s.trace == nil:
		s.trace = runtime.NewTraceRecorder()
	case !on:
		s.trace = nil
	}
	if s.rtCtx != nil {
		s.rtCtx.SetTrace(s.trace)
	}
	if s.actionExec != nil {
		s.actionExec.executor.SetTrace(s.trace)
	}
	if s.stateExec != nil {
		s.stateExec.executor.SetTrace(s.trace)
	}
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// drainTrace returns what was recorded since the last command, prefixed, and
// resets the recorder so each command reports only its own steps.
func (s *Session) drainTrace() []string {
	if s.trace == nil {
		return nil
	}
	entries := s.trace.Entries()
	if len(entries) == 0 {
		return nil
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = tracePrefix + e
	}
	s.trace.Clear()
	return out
}
