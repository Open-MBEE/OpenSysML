package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// BehaviorStartFailureCode is the diagnostic code a behavior that failed to
// start is reported under beside the object that still stands.
const BehaviorStartFailureCode = "behavior-start-failed"

// BehaviorStartFailure is a behavior a type bound to an object that failed to
// start, or failed while the creation ran it to quiescence, as a finding about
// the object: the object stands and the behavior runs no further.
type BehaviorStartFailure struct {
	Behavior *ObjectBehavior
}

// Describe renders the failure for a diagnostic: the behavior and what it failed with.
func (f BehaviorStartFailure) Describe() string {
	what := f.Behavior.Describe()
	return fmt.Sprintf("%s failed to start and runs no further: %s", what, strings.TrimPrefix(f.Behavior.Err.Error(), what+": "))
}

// String is the trace line the failure is recorded as.
func (f BehaviorStartFailure) String() string {
	return "behavior start failure " + f.Describe()
}

// Location is where the behavior was bound to the object's type.
func (f BehaviorStartFailure) Location() (string, source.Span) {
	if f.Behavior.member == nil {
		return "", source.Span{}
	}
	return f.Behavior.member.DocName, f.Behavior.member.DeclSpan
}

// Diagnostic is the failure as a finding about the object: a warning, since the
// object was made and the behavior's failure is a fact about this run of it.
func (f BehaviorStartFailure) Diagnostic() diag.Diagnostic {
	_, span := f.Location()
	return diag.Diagnostic{
		Severity: diag.SeverityWarning,
		Span:     span,
		Message:  f.Describe(),
		Code:     BehaviorStartFailureCode,
		Source:   "runtime",
	}
}

// StartFailureNotes are StartFailures as the notes a creation reports beside the object.
func (ctx *Context) StartFailureNotes(root *Instance) []RunNote {
	failed := ctx.StartFailures(root)
	notes := make([]RunNote, 0, len(failed))
	for _, b := range failed {
		notes = append(notes, BehaviorStartFailure{Behavior: b})
	}
	return notes
}

// A behavior a type binds to its objects that fails to start, or fails while the
// creation runs it to quiescence, ends that behavior, not the object: the object
// stands, the failure is recorded on the behavior (ObjectBehavior.Err), and a
// verdict reading a feature the failed behavior may still have written is
// withheld as undecided, since the run that was to decide it never completed.

// ErrVerdictUndecided marks a verdict withheld because a condition read a feature
// a failed behavior of the object may still write.
var ErrVerdictUndecided = errors.New("verdict undecided: a behavior that failed to start may still write a feature the condition read")

// UndecidedError is a verdict withheld under ErrVerdictUndecided: which check,
// which feature it read, and which failed behavior may write that feature.
type UndecidedError struct {
	Kind     string
	Element  string
	Feature  string
	Behavior string
	Cause    error
}

// Error names the check, the feature and the behavior whose failure withholds the verdict.
func (e *UndecidedError) Error() string {
	return fmt.Sprintf("%s %s: undecided: %s may still be written by %s, which failed to start: %v",
		e.Kind, e.Element, e.Feature, e.Behavior, e.Cause)
}

// Is matches ErrVerdictUndecided.
func (e *UndecidedError) Is(target error) bool { return target == ErrVerdictUndecided }

// recordsFailure reports whether an error of a behavior is recorded on the
// behavior rather than failing what ran it: any failure of a behavior the
// object's type bound, at its start or while the creation ran it to quiescence,
// except a bound of the context's own — on live objects, steps or events —
// which no behavior owns and which stays the creation's failure.
func recordsFailure(b *ObjectBehavior, err error) bool {
	return b != nil && b.typeBound && !exhaustsBudget(err)
}

// exhaustsBudget reports whether err is one of the context's bounds being reached.
func exhaustsBudget(err error) bool {
	var budget *budgetExceededError
	if errors.As(err, &budget) {
		return true
	}
	for _, bound := range []error{ErrInstanceLimitExceeded, ErrStepLimitExceeded, ErrActionStepLimitExceeded, ErrStateEventLimitExceeded, ErrDoStepLimitExceeded, errBehaviorBudget} {
		if errors.Is(err, bound) {
			return true
		}
	}
	return false
}

// unstartedBehavior is the record of a behavior bound by the object's type that
// never got an execution: its binding could not be resolved or performed, so the
// failure has a behavior to be recorded on.
func (ctx *Context) unstartedBehavior(inst *Instance, decl classifierBehaviorDecl) *ObjectBehavior {
	b := &ObjectBehavior{
		Name:   decl.behavior.Name,
		Kind:   decl.behavior.Kind,
		Object: inst,
		member: decl.member,
		ctx:    ctx,
	}
	if chain, err := ctx.classifierBehaviorChain(decl); err == nil {
		b.Symbol = chain[len(chain)-1]
		b.bindings = chain
		b.kinds = ctx.behaviorKinds(chain)
	}
	return b
}

// StartFailures lists the behaviors of root and of the objects it holds, directly
// or below, that failed to start: the diagnostics a creation reports beside the
// object it still made, in attachment order.
func (ctx *Context) StartFailures(root *Instance) []*ObjectBehavior {
	if root == nil {
		return nil
	}
	within := map[int64]bool{root.ID: true}
	for grew := true; grew; {
		grew = false
		for _, inst := range ctx.instances {
			if within[inst.ID] || inst.owner == nil || !within[inst.owner.ID] {
				continue
			}
			within[inst.ID] = true
			grew = true
		}
	}
	var failed []*ObjectBehavior
	for _, b := range ctx.objectBehaviors {
		if b.Err != nil && b.typeBound && within[b.Object.ID] {
			failed = append(failed, b)
		}
	}
	return failed
}

// hasFailedBehavior reports whether any behavior a type bound failed, in which
// case a verdict's reads are checked against what the failed behaviors may write.
func (ctx *Context) hasFailedBehavior() bool {
	for _, b := range ctx.objectBehaviors {
		if b.Err != nil && b.typeBound {
			return true
		}
	}
	return false
}

// beginVerdictReads records the feature values read until the function returned
// is called, which answers them; a verdict evaluated within another is recorded
// by both.
func (ctx *Context) beginVerdictReads() func() []*FeatureValue {
	ctx.verdictReads = append(ctx.verdictReads, nil)
	return func() []*FeatureValue {
		last := len(ctx.verdictReads) - 1
		reads := ctx.verdictReads[last]
		ctx.verdictReads = ctx.verdictReads[:last]
		return reads
	}
}

// noteVerdictRead lists fv among the reads of every verdict being evaluated.
func (ctx *Context) noteVerdictRead(fv *FeatureValue) {
	for i, reads := range ctx.verdictReads {
		if !slices.Contains(reads, fv) {
			ctx.verdictReads[i] = append(reads, fv)
		}
	}
}

// judgeVerdict is judgeConditions for a verdict reported about the model — a
// satisfaction, a requirement or constraint check, a case's objective or
// assertion: one decided from a feature value a failed behavior may still write
// is withheld as undecided, since the behavior's run was to settle that value.
func (ctx *Context) judgeVerdict(check conditionCheck, conds []Condition) (conditionVerdict, error) {
	if !ctx.hasFailedBehavior() {
		return ctx.judgeConditions(check, conds)
	}
	end := ctx.beginVerdictReads()
	verdict, err := ctx.judgeConditions(check, conds)
	reads := end()
	if err != nil && !isViolation(err) {
		return verdict, err
	}
	if undecided := ctx.undecidedBy(check, reads); undecided != nil {
		return conditionVerdict{}, undecided
	}
	return verdict, err
}

// undecidedBy is the UndecidedError a verdict reading reads is withheld with, nil
// when no failed behavior may write any of them. A read of a derived value stands
// on what its derivation read, so those reads count as well.
func (ctx *Context) undecidedBy(check conditionCheck, reads []*FeatureValue) error {
	seen := make(map[*FeatureValue]bool, len(reads))
	for i := 0; i < len(reads); i++ {
		fv := reads[i]
		if seen[fv] {
			continue
		}
		seen[fv] = true
		for _, b := range ctx.objectBehaviors {
			if b.Err == nil || !b.typeBound {
				continue
			}
			if name, ok := ctx.mayWrite(b, fv); ok {
				return &UndecidedError{Kind: check.kind, Element: check.name(), Feature: name, Behavior: b.Describe(), Cause: b.Err}
			}
		}
		reads = append(reads, fv.reads...)
	}
	return nil
}

// mayWrite reports whether the failed behavior's body may write the feature
// value, naming the feature: a plain name the body writes is the performer's own
// feature (or its performance occurrence's), a chained or qualified one any
// object's feature of that declaration or name, and a body whose targets are not
// decidable from the lowered graph (a chain from a computed base, or no graph at
// all) may write any feature of any object.
func (ctx *Context) mayWrite(b *ObjectBehavior, fv *FeatureValue) (string, bool) {
	if fv.Feature == nil {
		return "", false
	}
	fp := b.failedFootprint()
	own := b.holdsOwn(fv)
	if fp.Dynamic {
		if own != "" {
			return own, true
		}
		return fv.Feature.Name, true
	}
	read := lower.Place{Sym: fv.Feature.Symbol, Name: fv.Feature.Name}
	for _, w := range fp.Writes {
		if w.Local || w.State != nil || !w.Conflicts(read) {
			continue
		}
		if w.Direct {
			if own == "" {
				continue
			}
			return own, true
		}
		return fv.Feature.Name, true
	}
	return "", false
}

// holdsOwn is the name under which the behavior's performer or its performance
// occurrence holds the feature value, "" when neither does.
func (b *ObjectBehavior) holdsOwn(fv *FeatureValue) string {
	holders := []*Instance{b.Object}
	switch {
	case b.State != nil && b.State.occurrence != nil:
		holders = append(holders, b.State.occurrence)
	case b.Action != nil && b.Action.occurrence != nil:
		holders = append(holders, b.Action.occurrence)
	}
	for _, holder := range holders {
		for name, held := range holder.FeatureValues {
			if held == fv {
				return name
			}
		}
	}
	return ""
}

// failedFootprint is what the failed behavior's whole body may touch, from the
// lowered graph its execution was built on, computed once; a behavior that got no
// execution has no graph to read, so its touches are not decidable.
func (b *ObjectBehavior) failedFootprint() lower.Footprint {
	if b.footprintKnown {
		return b.footprint
	}
	b.footprintKnown = true
	switch {
	case b.State != nil && b.State.graph != nil:
		b.footprint = machineFootprint(b.State.graph)
	case b.Action != nil && b.Action.graph != nil:
		b.footprint = lower.FlowFootprint(b.Action.graph)
	default:
		b.footprint = unknownFootprint()
	}
	return b.footprint
}

// machineFootprint is what a whole state machine may touch: every transition
// and branch out of its states, every entry transition's effect, and every
// entry, do, exit and terminate behavior.
func machineFootprint(g *lower.StateGraph) lower.Footprint {
	var fp lower.Footprint
	for _, trans := range g.TransitionFootprints() {
		fp = unionFootprints(fp, trans)
	}
	for _, branch := range g.BranchFootprints() {
		fp = unionFootprints(fp, branch)
	}
	for _, behavior := range g.BehaviorFootprints() {
		fp = unionFootprints(fp, behavior)
	}
	for _, entries := range g.EntryTransitions {
		for _, entry := range entries {
			for _, effect := range entry.Effect {
				fp = unionFootprints(fp, g.BehaviorFootprint(effect))
			}
		}
	}
	return fp
}
