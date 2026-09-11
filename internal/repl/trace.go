package repl

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// noteSummary is the line a debugger command adds when the steps it ran noted
// choice points, guards it could not evaluate or tools that answered equal inputs
// differently beyond the before notes the executor's run started with: how many
// of each, and how to see them.
func (s *Session) noteSummary(notes []runtime.RunNote, before int) []string {
	if before >= len(notes) {
		return nil
	}
	var choices, unevaluable, diverged int
	for _, n := range notes[before:] {
		switch n.(type) {
		case runtime.ChoicePoint:
			choices++
		case runtime.UnevaluableGuard:
			unevaluable++
		case runtime.ToolDivergence:
			diverged++
		}
	}
	var parts []string
	if choices > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", choices, plural(choices, "choice point", "choice points")))
	}
	if unevaluable > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", unevaluable, plural(unevaluable, "guard not evaluable", "guards not evaluable")))
	}
	if diverged > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", diverged, plural(diverged, "tool answer diverged", "tool answers diverged")))
	}
	line := "  " + strings.Join(parts, "; ")
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
	defer s.reading()()
	return s.trace != nil
}

// SetTracing turns recording of execution steps on or off. It takes effect at
// once, on the session's runtime context and on a debugging session already
// under way as well as on everything created afterwards.
func (s *Session) SetTracing(on bool) {
	defer s.enter()()
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
