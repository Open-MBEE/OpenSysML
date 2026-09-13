package runtime

import (
	"errors"
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Holds evaluates a requirement or constraint against the run's state as the
// executor now holds it: a feature the condition names resolves to the
// performance's value first, then to the performer's, then to the declaration.
// It reports whether the conditions hold as CheckRequirementOn and
// CheckConstraintOn decide, the condition that failed carried as a
// ViolationError, and a runtime failure of the evaluation as its own error.
func (e *ActionExecutor) Holds(sym *symbols.Symbol, scope *symbols.Scope) (bool, error) {
	if sym == nil {
		return false, fmt.Errorf("%w: no condition named", ErrNoConditions)
	}
	defer e.ctx.beginExecutorRun(&e.driven)()
	defer e.ctx.beginProbe()()

	kind, what := "constraint", "assertion"
	if err := RequireConstraint(sym); err != nil {
		if RequireRequirement(sym) != nil {
			return false, err
		}
		kind, what = "requirement", "require condition"
	}
	subject, err := e.ctx.checkSubject(kind, sym.Name, sym, e.self)
	if err != nil {
		return false, err
	}
	members := e.ctx.chainMembers(sym, scope)
	check := conditionCheck{
		sym:     sym,
		kind:    kind,
		what:    what,
		self:    subject.instance,
		frames:  e.root.lexicalFrames(),
		negated: NegatedDecl(sym),
	}
	if kind == "requirement" {
		bindings, err := e.ctx.memberBindings(sym, kind, sym.Name, members, subject.instance, nil, frame{})
		if err != nil {
			return false, err
		}
		check.bindings = mapFrame(bindings)
	}
	holds, err := e.ctx.evaluateConditions(check, e.ctx.conditionsOf(sym, members))
	if err != nil {
		var violation *ViolationError
		if errors.As(err, &violation) {
			return false, err
		}
		if kind == "requirement" {
			err = unboundSubjectError(err, kind, sym.Name, e.ctx.unboundSubjectNames(sym, members, subject.instance))
		}
	}
	return holds, err
}
