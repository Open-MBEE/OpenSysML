package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ElementFilterPass checks the element-filter conditions a document writes: the
// `filter` members of a namespace (KerML 8.2.4) and the `[...]` clause of an
// import or expose (SysML v2 7.4.4). A condition is a predicate over one
// candidate element, so it has to be boolean-valued and it has to be a form the
// filter evaluator can decide; a condition that is neither selects nothing, and
// name resolution keeps every candidate rather than hiding model content on a
// verdict it could not reach.
//
// It runs at LevelType, and its subject is one condition. A classification test
// is Boolean whatever type it names, so it rests on that type and says nothing
// more once the type fails to resolve; every other form is judged on itself,
// where an unresolved name is the verdict — not model-level evaluable — rather
// than a reason to withhold it.
type ElementFilterPass struct{}

func (ElementFilterPass) Level() PassLevel { return LevelType }

func (ElementFilterPass) ElementScoped() { /* marker: per-element gating */ }

func (ElementFilterPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	fc := &filterChecker{
		ctx:   ctx,
		model: ctx.Model(),
		expr:  &exprChecker{resolver: ctx.Resolver(), model: ctx.Model(), lang: ctx.Kind},
		seen:  make(map[*symbols.Scope]bool),
	}
	fc.expr.walkMembers = fc.expr.checkMemberOperators
	fc.walk(rootScope)
	return append(fc.diags, fc.expr.diagnostics()...)
}

type filterChecker struct {
	ctx   *Context
	model *semantics.Model
	// expr applies the operator-expression rules to the condition's operators.
	expr  *exprChecker
	seen  map[*symbols.Scope]bool
	diags []Diagnostic
}

// walk checks the conditions every namespace in the scope subtree writes. A
// definition or usage body is a namespace too, and the corpus writes a filter in
// one, so every scope is visited rather than only the packages.
func (fc *filterChecker) walk(scope *symbols.Scope) {
	if scope == nil || fc.seen[scope] {
		return
	}
	fc.seen[scope] = true
	for _, f := range symbols.NamespaceFiltersIn(scope) {
		fc.check(f)
	}
	for _, f := range symbols.ImportFiltersIn(scope) {
		fc.check(f)
	}
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if sym != nil {
			fc.walk(sym.Scope)
		}
		return true
	})
}

// check reports the faults of one condition.
func (fc *filterChecker) check(f symbols.ElementFilter) {
	if fc.model == nil || fc.gated(f.Expr) {
		return
	}
	for _, p := range fc.model.CheckElementFilter(f) {
		fc.diags = append(fc.diags, filterDiagnostic(p))
	}
	fc.expr.checkOperatorRules(f.Scope, f.Expr)
}

// gated reports whether the condition rests on something a lower tier could not
// resolve: the type a classification test names, or any operand of an operator
// whose own result is Boolean whatever its operands turn out to be.
func (fc *filterChecker) gated(expr ast.Node) bool {
	op, ok := expr.(*ast.OperatorExpr)
	if !ok {
		return false
	}
	switch op.Operator {
	case ast.OpAt, ast.OpMetaAt, ast.OpIsType, ast.OpHasType:
		// A type the parser never produced is a resolution failure of its own.
		if op.TypeRef == nil || fc.ctx.DownstreamOfFailure(op.TypeRef) {
			return true
		}
	case ast.OpNot, ast.OpAnd, ast.OpConditionalAnd, ast.OpOr, ast.OpConditionalOr,
		ast.OpXor, ast.OpImplies,
		ast.OpEq, ast.OpNeq, ast.OpEqEqEq, ast.OpNeqEqEq,
		ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe:
	default:
		return false
	}
	for _, o := range op.Operands {
		if fc.gated(o) || fc.ctx.DownstreamOfFailure(o) {
			return true
		}
	}
	return false
}

// Pilot KerMLValidator (2026-05) validateElementFilterMembershipIsBoolean and
// the model-level evaluability check behind it: a condition must yield a truth
// value, and must do so without running the model.
const (
	msgFilterNotBoolean   = "Must have a Boolean result"
	msgFilterNotEvaluable = "Must be model-level evaluable"
	msgFilterNotEvaluated = "Filter condition is not evaluated by OpenSysML; all candidates are kept"
)

// filterDiagnostic renders one filter condition fault.
func filterDiagnostic(p semantics.FilterProblem) Diagnostic {
	switch p.Kind {
	case semantics.FilterProblemNotBoolean:
		return Diagnostic{
			Severity: SeverityError,
			Span:     p.Span,
			Message:  msgFilterNotBoolean,
			Code:     "filter-not-boolean",
			Source:   "type",
		}
	case semantics.FilterProblemUnsupported:
		return Diagnostic{
			Severity: SeverityWarning,
			Span:     p.Span,
			Message:  msgFilterNotEvaluated,
			Code:     "filter-not-evaluated",
			Source:   "type",
		}
	}
	return Diagnostic{
		Severity: SeverityError,
		Span:     p.Span,
		Message:  msgFilterNotEvaluable,
		Code:     "filter-not-evaluable",
		Source:   "type",
	}
}
