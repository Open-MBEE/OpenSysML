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

// noteChoice keeps a choice point for the run's diagnostics and, when tracing,
// writes it to the trace where it was made. A probe's preview is not a run.
func (ctx *Context) noteChoice(c ChoicePoint) {
	if ctx.probes > 0 {
		return
	}
	ctx.choices = append(ctx.choices, c)
	if ctx.trace != nil {
		ctx.trace.RecordChoice(c)
	}
}

// Choices returns the choice points made since the latest run began, in order.
func (ctx *Context) Choices() []ChoicePoint {
	out := make([]ChoicePoint, len(ctx.choices))
	copy(out, ctx.choices)
	return out
}

// ChoiceCount is how many choice points the latest run has made so far, so a
// caller stepping an executor can tell what one of its steps chose.
func (ctx *Context) ChoiceCount() int {
	return len(ctx.choices)
}
