package runtime

import (
	"fmt"
	"sort"
	"strings"
)

// body spells a paused body: its work, the frames it paused through innermost
// first, and why it paused. The frames point into the model's lowered
// statements, so a body is spelled by position; the counters that order the
// paused runs and the trace levels they hold are scheduling detail and dropped.
func (s *stateSpeller) body(run *bodyRun) string {
	if host, ok := run.work.(*stateStmtHost); ok {
		defer s.enter(host.flow)()
	}
	var b strings.Builder
	b.WriteString(run.work.spell(s))
	for _, f := range run.cursor {
		b.WriteString("; ")
		b.WriteString(f.spell(s))
	}
	b.WriteString("; ")
	b.WriteString(s.pause(run.paused))
	return b.String()
}

// pause spells why a body paused: the statement boundary it yielded at, the token
// move it stepped, the breakpoint it met, or the wait it is in — the callee held by
// its caller, whole, else the performance whose flow waits.
func (s *stateSpeller) pause(p bodyPause) string {
	if p.yielded {
		return "yielded between statements"
	}
	if p.tokenStep {
		return "paused between two moves of its flow"
	}
	if !p.onWait {
		return fmt.Sprintf("at breakpoint %q", p.breakpoint)
	}
	if p.wait.held != nil {
		return "holds " + s.nested(p.wait.held)
	}
	kind := "clock"
	if p.wait.onMessage {
		kind = "message"
	}
	return "waits on " + kind + " in " + s.frameLabel(p.wait.perf)
}

// nested spells an executor a body holds — a callee, or the flow of a do
// behavior — whole: its state, frames and tokens, with their own paused work.
func (s *stateSpeller) nested(e *ActionExecutor) string {
	defer s.enter(e)()
	var b strings.Builder
	fmt.Fprintf(&b, "%s{state %s", symbolText(e.action), e.state)
	if e.pausedAt.name != "" {
		fmt.Fprintf(&b, " at %s", e.pausedAt.name)
	}
	for _, perf := range s.frames {
		b.WriteString("; ")
		b.WriteString(s.frame(perf))
	}
	for _, line := range s.tokenLines() {
		b.WriteString("; ")
		b.WriteString(line)
	}
	b.WriteString("}")
	return b.String()
}

func (w *usageWork) spell(s *stateSpeller) string {
	return fmt.Sprintf("usage %s phase %d", s.frameLabel(w.perf), w.phase)
}

func (w *statementWork) spell(s *stateSpeller) string {
	return fmt.Sprintf("statement %s in %s done %t", nodeKey(w.node), s.frameLabel(w.frame), w.done)
}

func (w *executionWork) spell(s *stateSpeller) string {
	return fmt.Sprintf("execution %s in %s invoked %t {%s}",
		nodeKey(w.node), s.frameLabel(w.frame), w.invoked, s.values(w.outputs))
}

func (h *stateStmtHost) spell(s *stateSpeller) string {
	return "behavior " + nodeKey(h.behavior.Node) + " " + s.nested(h.flow)
}

// spell of an engine is the frames its statements read past its data — the
// blocks it paused in keep theirs, so these are the values declared outside any.
func (f *engineFrame) spell(s *stateSpeller) string {
	return "engine{" + s.localBodyFrames(f.engine.env) + "}"
}

func (f *stmtListFrame) spell(s *stateSpeller) string {
	if f.done == nil {
		return fmt.Sprintf("stmt %d", f.i)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "stmt %d of ", f.i)
	for i, done := range f.done {
		switch {
		case done:
			b.WriteByte('+')
		case f.blocked[i]:
			b.WriteByte('!')
		default:
			b.WriteByte('-')
		}
	}
	if f.switched >= 0 {
		fmt.Fprintf(&b, " after %d", f.switched)
	}
	for i, strand := range f.strands {
		if strand == nil {
			continue
		}
		fmt.Fprintf(&b, " [%d:", i)
		for _, inner := range strand.cursor {
			b.WriteString(" ")
			b.WriteString(inner.spell(s))
		}
		b.WriteString("]")
	}
	return b.String()
}

func (f *branchFrame) spell(*stateSpeller) string {
	if f.elseBranch {
		return "else"
	}
	return "then"
}

func (f *blockFrame) spell(s *stateSpeller) string {
	return "block{" + s.bodyLocals(f.locals, f.cells, f.unvalued) + "}"
}

func (f *flowNodeFrame) spell(*stateSpeller) string { return "flow at " + nodeKey(f.node) }

func (f *loopFrame) spell(s *stateSpeller) string {
	return fmt.Sprintf("loop %d of (%s){%s}", f.iteration, s.elements(f.elements), s.bodyLocals(f.locals, f.cells, f.unvalued))
}

func (s *stateSpeller) bodyLocals(locals map[string]Value, cells *bodyCells, unvalued map[string]bool) string {
	spelled := s.bodyValues(locals, cells)
	names := make([]string, 0, len(unvalued))
	for name := range unvalued {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) > 0 {
		spelled += " unvalued " + strings.Join(names, ", ")
	}
	return spelled
}

func (s *stateSpeller) localBodyFrames(env *stmtEnv) string {
	locals, cells, _ := env.localFrames()
	frames := make([]string, len(locals))
	for i, values := range locals {
		var unvalued map[string]bool
		if i == 0 {
			unvalued = env.unvaluedLocal
		} else if i-1 < len(env.unvalued) {
			unvalued = env.unvalued[i-1]
		}
		var localCells *bodyCells
		if i < len(cells) {
			localCells = cells[i]
		}
		frames[i] = "{" + s.bodyLocals(values, localCells, unvalued) + "}"
	}
	return strings.Join(frames, " ")
}

func (f *performFrame) spell(s *stateSpeller) string {
	ended := ""
	if f.ended {
		ended = " ended"
	}
	return fmt.Sprintf("perform %s phase %d%s", s.frameLabel(f.perf), f.phase, ended)
}

func (f *blockFlowFrame) spell(s *stateSpeller) string {
	ended := ""
	if f.perf != nil && f.perf.ended {
		ended = " ended"
	}
	return fmt.Sprintf("block flow %s%s", s.frameLabel(f.perf), ended)
}

func (f *subflowFrame) spell(s *stateSpeller) string {
	settled := make([]string, 0, len(f.progress.settled))
	for w := range f.progress.settled {
		name := w.dueLabel()
		if exec, ok := w.(checkedExecutor); ok {
			if named, ok := s.names[exec]; ok {
				name = named
			}
		}
		settled = append(settled, name)
	}
	sort.Strings(settled)
	return fmt.Sprintf("subflow %s dropped %d settled [%s]",
		s.frameLabel(f.perf), len(f.progress.dropped), strings.Join(settled, ", "))
}

func (f *calleeFrame) spell(s *stateSpeller) string {
	return "callee " + f.name + " " + s.nested(f.exec)
}
