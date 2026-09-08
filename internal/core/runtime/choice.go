package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// ChoiceKind names what an executor chose among at a choice point.
type ChoiceKind int

const (
	// ChoiceTokenOrder: several tokens were steppable in one step, and the step
	// advanced them in an order the library does not fix.
	ChoiceTokenOrder ChoiceKind = iota
	// ChoiceDecisionBranch: several guards of one decision node held.
	ChoiceDecisionBranch
	// ChoiceWriteOrder: two tokens wrote one feature within one step, so which
	// value the feature holds afterwards is the order the writes were applied in.
	ChoiceWriteOrder
	// ChoiceTransition: several transitions out of one state were enabled for
	// one event.
	ChoiceTransition
)

// String is the kind as a trace or diagnostic names it.
func (k ChoiceKind) String() string {
	switch k {
	case ChoiceTokenOrder:
		return "token order"
	case ChoiceDecisionBranch:
		return "decision branch"
	case ChoiceWriteOrder:
		return "write order"
	case ChoiceTransition:
		return "transition"
	}
	return fmt.Sprintf("ChoiceKind(%d)", int(k))
}

// ChoiceDiagnosticCode is the code every choice-point diagnostic carries.
const ChoiceDiagnosticCode = "choice-point"

// UnevaluableGuardCode is the code a diagnostic about a guard the run could not
// evaluate carries.
const UnevaluableGuardCode = "guard-unevaluable"

// RunNote is a finding a run records about itself without changing it: a choice
// point it made, or a guard it read only to report one and could not evaluate.
type RunNote interface {
	// Describe renders the note for a diagnostic; String is its trace line.
	Describe() string
	String() string
	// Diagnostic is the note as an informational finding about the run.
	Diagnostic() passes.Diagnostic
	// Location is the file and span of the declaration the note is about; file is
	// "" when the runtime could not name one.
	Location() (file string, span source.Span)
}

// ChoicePoint is one point where an executor had several enabled alternatives the
// Kernel Semantic Library leaves unordered and took one by its own scheduling rule.
type ChoicePoint struct {
	Kind ChoiceKind
	// Step is the action step the choice was made in, 0 for a state machine.
	Step int
	// Where names the decision node or the state and event; empty for a token order.
	Where string
	// Alternatives are canonical: tokens by ID, branches and transitions by
	// declaration position, writes by writing token. Taken indexes the one taken.
	Alternatives []string
	Taken        int
	// File and Span locate the declaration the choice was made at; File is ""
	// when the runtime could not name one.
	File string
	Span source.Span
}

// Describe renders the choice for a diagnostic: the alternatives in canonical
// order and which one the executor took.
func (c ChoicePoint) Describe() string {
	alts := strings.Join(c.Alternatives, ", ")
	taken := ""
	if c.Taken >= 0 && c.Taken < len(c.Alternatives) {
		taken = c.Alternatives[c.Taken]
	}
	switch c.Kind {
	case ChoiceTokenOrder:
		return fmt.Sprintf("step %d: tokens %s (unordered; took %s first)", c.Step, alts, taken)
	case ChoiceDecisionBranch:
		return fmt.Sprintf("step %d: %s branches %s hold (unordered; took %s)", c.Step, c.Where, alts, taken)
	case ChoiceWriteOrder:
		return fmt.Sprintf("step %d: writes %s (unordered; %s stood)", c.Step, alts, taken)
	case ChoiceTransition:
		return fmt.Sprintf("%s: transitions %s (unordered; took %s)", c.Where, alts, taken)
	}
	return fmt.Sprintf("%s: %s (unordered; took %s)", c.Kind, alts, taken)
}

// String is the trace line the choice is recorded as.
func (c ChoicePoint) String() string {
	return "choice " + c.Describe()
}

// Location is where the choice was made.
func (c ChoicePoint) Location() (string, source.Span) {
	return c.File, c.Span
}

// Diagnostic is the choice as a finding about the run: informational, since a
// model is not wrong for admitting several orders and the run took one of them.
func (c ChoicePoint) Diagnostic() passes.Diagnostic {
	return passes.Diagnostic{
		Severity: passes.SeverityInfo,
		Span:     c.Span,
		Message:  "choice point: " + c.Describe(),
		Code:     ChoiceDiagnosticCode,
		Source:   "runtime",
	}
}

// UnevaluableGuard is a guard an executor read only to report a choice, once a
// branch or transition already held, and could not evaluate. A guard with no
// result is not true, so its succession is not selected; the run is unchanged.
type UnevaluableGuard struct {
	// Step is the action step the guard was read in, 0 for a state machine.
	Step int
	// Where names the decision node or the state and event, as a ChoicePoint does.
	Where string
	// Alternative is the branch or transition by declaration position, as a
	// ChoicePoint lists it.
	Alternative string
	// Reason is the evaluation error.
	Reason string
	File   string
	Span   source.Span
}

// Describe renders the guard for a diagnostic: where it was read, which
// alternative it guards and why it has no result.
func (g UnevaluableGuard) Describe() string {
	if g.Step > 0 {
		return fmt.Sprintf("step %d: %s branch %s: %s (not selected)", g.Step, g.Where, g.Alternative, g.Reason)
	}
	return fmt.Sprintf("%s: transition %s: %s (not selected)", g.Where, g.Alternative, g.Reason)
}

// String is the trace line the guard is recorded as.
func (g UnevaluableGuard) String() string {
	return "unevaluable guard " + g.Describe()
}

// Location is where the guard was declared.
func (g UnevaluableGuard) Location() (string, source.Span) {
	return g.File, g.Span
}

// Diagnostic is the guard as a finding about the run: informational, since the
// library selects no succession whose guard is not true and defines no failure.
func (g UnevaluableGuard) Diagnostic() passes.Diagnostic {
	return passes.Diagnostic{
		Severity: passes.SeverityInfo,
		Span:     g.Span,
		Message:  "guard not evaluable: " + g.Describe(),
		Code:     UnevaluableGuardCode,
		Source:   "runtime",
	}
}

// noteChoice keeps a choice point for the run's diagnostics and, when tracing,
// writes it to the trace where it was made.
func (ctx *Context) noteChoice(c ChoicePoint) {
	ctx.note(c)
}

// noteUnevaluableGuard keeps a guard the run could not evaluate, as noteChoice does.
func (ctx *Context) noteUnevaluableGuard(g UnevaluableGuard) {
	ctx.note(g)
}

// noteAll records notes in order.
func (ctx *Context) noteAll(notes []RunNote) {
	for _, n := range notes {
		ctx.note(n)
	}
}

// note keeps n for the run's diagnostics and, when tracing, writes it to the
// trace where it was made. A probe's preview is not a run.
func (ctx *Context) note(n RunNote) {
	if ctx.probes > 0 {
		return
	}
	ctx.notes = append(ctx.notes, n)
	if ctx.trace != nil {
		ctx.trace.RecordNote(n)
	}
}

// Notes returns what the latest run noted about itself, in order: its choice
// points and the guards it could not evaluate.
func (ctx *Context) Notes() []RunNote {
	out := make([]RunNote, len(ctx.notes))
	copy(out, ctx.notes)
	return out
}

// NoteCount is how many notes the latest run has made so far, so a caller
// stepping an executor can tell what one of its steps noted.
func (ctx *Context) NoteCount() int {
	return len(ctx.notes)
}

// Choices returns the choice points made since the latest run began, in order.
func (ctx *Context) Choices() []ChoicePoint {
	var out []ChoicePoint
	for _, n := range ctx.notes {
		if c, ok := n.(ChoicePoint); ok {
			out = append(out, c)
		}
	}
	return out
}

// UnevaluableGuards returns the guards the latest run could not evaluate, in order.
func (ctx *Context) UnevaluableGuards() []UnevaluableGuard {
	var out []UnevaluableGuard
	for _, n := range ctx.notes {
		if g, ok := n.(UnevaluableGuard); ok {
			out = append(out, g)
		}
	}
	return out
}
