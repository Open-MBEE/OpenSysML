package solve

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Analysis translates an analysis case as an optimization query: what its
// conditions permit, and the objectives to improve within that.
//
// The contract, which SysML v2 leaves to a tool since it defines no solving:
// direction comes from the trade-study definition the objective is typed by
// (TradeStudies::MinimizeObjective or MaximizeObjective), the value to improve
// from the expression the objective's redefinition of the library's `eval`
// calculation returns (`in calc :>> eval { expression }`), and what is feasible
// from the conditions the case requires or assumes together with the ones each
// objective states in its own body.
func Analysis(ctx *runtime.Context, sym *symbols.Symbol, scope *symbols.Scope) (*Query, error) {
	return AnalysisWith(ctx, sym, scope, nil)
}

// AnalysisWith translates an analysis case with values already fixed, so the
// optimum is the best one consistent with what the model already fixes. With no
// pins it is Analysis.
func AnalysisWith(ctx *runtime.Context, sym *symbols.Symbol, scope *symbols.Scope, pins []Pin) (*Query, error) {
	if ctx == nil {
		return nil, fmt.Errorf("solve: no runtime context")
	}
	if err := runtime.RequireAnalysis(sym); err != nil {
		return nil, err
	}
	conds := ctx.CaseConditionsOf(sym, scope)
	objectives := ctx.ObjectivesOf(sym, scope)
	// A shape validation rejects is refused as such even when no goal is stated.
	if len(objectives) == 0 && !conflictsResult(conds) {
		return nil, &NoObjectiveError{Element: sym.Name}
	}
	subject := Subject{Kind: "analysis", Name: sym.Name, Symbol: sym}
	// An objective's own conditions bound it as the case's do, so they are part
	// of what is feasible; a case stating none is unbounded rather than refused.
	t := newTranslator(ctx, subject)
	if err := t.translate(conds); err != nil {
		return nil, err
	}
	for _, obj := range objectives {
		t.within = obj.Symbol
		err := t.translate(obj.Conditions)
		t.within = nil
		if err != nil {
			return nil, err
		}
	}
	if err := t.optimize(objectives); err != nil {
		return nil, err
	}
	if err := t.fix(pins); err != nil {
		return nil, err
	}
	return t.query(), nil
}

// conflictsResult reports whether any condition, nested or not, records a second
// owned or inherited result expression.
func conflictsResult(conds []runtime.Condition) bool {
	for _, cond := range conds {
		if cond.Conflict != nil || conflictsResult(cond.Group) {
			return true
		}
	}
	return false
}

// optimize translates the objectives in declaration order, which is the order
// they are optimized in.
func (t *translator) optimize(objectives []runtime.Objective) error {
	for _, obj := range objectives {
		t.within = obj.Symbol
		translated, err := t.objective(obj)
		t.within = nil
		if err != nil {
			return err
		}
		t.objectives = append(t.objectives, translated)
	}
	return nil
}

// objective translates one objective: its direction, the term stating its value,
// and where it was written. An objective whose direction the model does not
// state, whose value it does not state, or whose value is not a number the
// optimizer can improve linearly refuses rather than being guessed at.
func (t *translator) objective(obj runtime.Objective) (Objective, error) {
	t.condLabel = objectiveLabel(obj)
	t.condFile = ""
	if obj.Symbol != nil {
		t.condFile = obj.Symbol.DocName
	}
	direction, err := t.direction(obj)
	if err != nil {
		return Objective{}, err
	}
	spelling := "`objective o : " + directionTypeName(direction) +
		" { subject :>> selectedAlternative; in calc :>> eval { expression } }`"
	if obj.Evaluates != nil {
		return Objective{}, t.refuseObjective(obj,
			"applies the calculation `"+obj.Evaluates.Name+"` to each alternative the case's subject lists, "+
				"a choice among listed alternatives rather than an optimum over a continuous domain",
			"run the trade study as an analysis (`-analysis`, `%analysis` or RunAnalysis), which evaluates "+
				"every alternative and reports the one selected")
	}
	if obj.ReboundBest != nil {
		return Objective{}, t.refuseObjective(obj,
			"gives the library's bound `best` a value of its own (`attribute :>> best = expression;`), "+
				"which validation rejects",
			"state the value to optimize as the objective's evaluation instead ("+spelling+")")
	}
	if obj.StepwiseEval {
		return Objective{}, t.refuseObjective(obj, "computes its evaluation in steps rather than stating it as one expression",
			"write the value to optimize as the body of `eval` ("+spelling+")")
	}
	if obj.Value == nil {
		return Objective{}, t.refuseObjective(obj, "states no value to improve",
			"state the value to optimize as the objective's evaluation ("+spelling+")")
	}
	// The flag is query-wide, so it is judged of the objective's own term alone:
	// a condition that is already nonlinear says nothing about the objective.
	outer := t.nonlinear
	t.nonlinear = false
	term, err := t.expr(obj.Value, obj.Scope)
	nonlinear := t.nonlinear
	t.nonlinear = outer || nonlinear
	if err != nil {
		return Objective{}, err
	}
	if !term.Sort.Numeric() {
		return Objective{}, t.refuseObjective(obj,
			"states a value of sort "+term.Sort.Name+" rather than a number", "")
	}
	if nonlinear {
		return Objective{}, t.refuseObjective(obj, "states a nonlinear value",
			"an optimizer improves a linear objective; a product or quotient of two "+
				"computed values is not one")
	}
	if err := t.bindBest(obj, term); err != nil {
		return Objective{}, err
	}
	return Objective{
		Direction:  direction,
		Term:       term,
		Name:       obj.Name,
		Symbol:     obj.Symbol,
		Expression: obj.Text(),
		Dimension:  t.dimensionText(obj.Scope, obj.Value),
		File:       t.condFile,
		Span:       objectiveSpan(obj),
		Location:   t.ctx.SourceLocation(t.condFile, objectiveSpan(obj)),
	}, nil
}

// bindBest asserts that the objective's `best` holds the value it improves, so a
// condition over `best` bounds that value. A `best` no condition reads needs no variable.
func (t *translator) bindBest(obj runtime.Objective, term *Term) error {
	if obj.Best == nil || obj.Best.Decl == nil || obj.Symbol == nil {
		return nil
	}
	chain := []*symbols.Symbol{obj.Symbol, obj.Best}
	if _, read := t.vars[variableName(t, chain)]; !read {
		return nil
	}
	best, err := t.variableOf(obj.Best.Decl, obj.Scope, chain, []string{obj.Best.Name})
	if err != nil {
		return err
	}
	left, right := promote(best, term)
	if !left.Sort.Equal(right.Sort) {
		return t.refuseObjective(obj, "states a value of sort "+right.Sort.Name+
			" for a `best` of sort "+left.Sort.Name, "")
	}
	t.guard(Binary(OpEq, Bool, left, right), obj.Best.Decl)
	return nil
}

// direction reads which way the objective's value is to be improved from the
// definition it is typed by.
func (t *translator) direction(obj runtime.Objective) (Direction, error) {
	switch obj.Direction {
	case runtime.Minimize:
		return Minimize, nil
	case runtime.Maximize:
		return Maximize, nil
	}
	return 0, t.refuseObjective(obj, "states no direction to improve its value in",
		"type it by TradeStudies::MinimizeObjective or TradeStudies::MaximizeObjective, "+
			"which is what says whether the least or the greatest value is wanted")
}

// directionTypeName is the trade-study definition an objective of a direction is
// typed by, for a message showing how the objective is written.
func directionTypeName(d Direction) string {
	if d == Maximize {
		return "TradeStudies::MaximizeObjective"
	}
	return "TradeStudies::MinimizeObjective"
}

// objectiveLabel names an objective as a message about it reads, since an
// objective is not a condition and has no condition text.
func objectiveLabel(obj runtime.Objective) string {
	label := "objective"
	if obj.Name != "" {
		label += " " + obj.Name
	}
	if text := obj.Text(); text != "" {
		label += " = " + text
	}
	return label
}

// objectiveSpan is where the objective's value was written, falling back to the
// objective's own declaration when it states none.
func objectiveSpan(obj runtime.Objective) source.Span {
	if obj.Value != nil {
		return obj.Value.Span()
	}
	if obj.Symbol != nil {
		return obj.Symbol.DeclSpan
	}
	return source.Span{}
}

// refuseObjective reports that an objective cannot be optimized as written.
func (t *translator) refuseObjective(obj runtime.Objective, reason, remedy string) error {
	span := objectiveSpan(obj)
	return &ObjectiveError{
		Objective: objectiveLabel(obj),
		Reason:    reason,
		Remedy:    remedy,
		Element:   t.subject.Kind + " " + t.subject.Name,
		File:      t.condFile,
		Span:      span,
		Location:  t.ctx.SourceLocation(t.condFile, span),
	}
}
