package passes

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// An argument binds to the parameter it fills (KerML 8.3.4.8.3), and that implied
// binding is judged like a written one (W9CBoundFeatureTypesPass).

// argumentBinding is an argument judged non-conforming: the value judged, which
// an error of the type checker may cover, and where the warning is reported.
type argumentBinding struct {
	judged, at source.Span
}

// judgeOperatorBindings judges each operand against the parameter of the library
// function the operator names, reporting at the expression as the reference does.
func (ec *exprChecker) judgeOperatorBindings(scope *symbols.Scope, e *ast.OperatorExpr) {
	// Type operators and indexing bind no feature argument.
	if w8cTypeOperators[e.Operator] || e.Operator == ast.OpIndex {
		return
	}
	params := ec.model.InputParametersOf(ec.model.OperatorFunction(e.Operator))
	for i, operand := range e.Operands {
		if i >= len(params) {
			return
		}
		if !expressionOperand(e.Operator, i) {
			ec.judgeArgumentBinding(scope, operand, ec.model.FeatureTypeSet(params[i]), e.Span())
		}
	}
}

// expressionOperand reports whether operand i is passed as an unevaluated
// expression: the branches of `if` and the right operand of `and`, `or`, `implies`, `??`.
func expressionOperand(op ast.OperatorKind, i int) bool {
	switch op {
	case ast.OpConditional, ast.OpConditionalAnd, ast.OpConditionalOr, ast.OpImplies, ast.OpNullCoalesce:
		return i > 0
	}
	return false
}

// judgeParameterBinding judges an argument of call against the parameter p it fills.
func (ec *exprChecker) judgeParameterBinding(scope *symbols.Scope, call invocation, value ast.Node, p parameter, at source.Span) {
	ec.judgeArgumentBinding(scope, value, ec.parameterTypes(p), at)
}

// positionalArgSpan is where a positional argument of call is reported: the
// receiver of `x->f(a)`, written apart from the call, at the invoked type.
func positionalArgSpan(call invocation, arg ast.Node) source.Span {
	if arg == call.e.Operand && call.e.Type != nil {
		return call.e.Type.Span()
	}
	return arg.Span()
}

// parameterTypes are the types p declares, else those of the parameter it redefines.
func (ec *exprChecker) parameterTypes(p parameter) []*symbols.Symbol {
	for q := &p; q != nil; q = q.redefined {
		if types := ec.declaredTypeSymbols(q.scope(), q.usage.Relationships); len(types) > 0 {
			return types
		}
	}
	return nil
}

// namedArgSpan covers `name = value`, the argument feature a named argument declares.
func namedArgSpan(na ast.NamedArg) source.Span {
	if na.Value == nil {
		return na.Name.Span()
	}
	start := na.Name.Span().Offset
	return source.Span{Offset: start, Len: na.Value.Span().End() - start}
}

// judgeArgumentBinding records value bound to a parameter typed want when their
// types conform in neither direction; unknown types and open parameters are not judged.
func (ec *exprChecker) judgeArgumentBinding(scope *symbols.Scope, value ast.Node, want []*symbols.Symbol, at source.Span) {
	if value == nil || len(want) == 0 || openParameter(want) || ec.warned[value] {
		return
	}
	if ec.warned == nil {
		ec.warned = map[ast.Node]bool{}
	}
	ec.warned[value] = true
	// A sequence or collection value results in an Anything, which conforms.
	if _, ok := value.(*ast.SequenceExpr); ok {
		return
	}
	if _, collection := ec.model.CollectionElements(scope, value); collection {
		return
	}
	got := ec.model.ExprResultTypes(scope, value)
	if len(got) > 0 && !w9cTypesConform(ec.model, got, want) {
		ec.bindings = append(ec.bindings, argumentBinding{value.Span(), at})
	}
}

// diagnostics are the checker's, with the warning of each non-conforming
// argument binding no error of the checker covers.
func (ec *exprChecker) diagnostics() []Diagnostic {
	diags := make([]Diagnostic, len(ec.diags), len(ec.diags)+len(ec.bindings))
	copy(diags, ec.diags)
	for _, b := range ec.bindings {
		if !ec.errorCovers(b.judged) {
			diags = append(diags, w9cDiagnostic(b.at))
		}
	}
	return diags
}

// errorCovers reports whether an error of the checker spans all of span.
func (ec *exprChecker) errorCovers(span source.Span) bool {
	for _, d := range ec.diags {
		if d.Severity == SeverityError && d.Span.Offset <= span.Offset && span.End() <= d.Span.End() {
			return true
		}
	}
	return false
}

// openParameter reports a parameter type any argument fills, as the type checker
// binds them: a Collection takes any sequence, Element the element an argument names.
func openParameter(want []*symbols.Symbol) bool {
	for _, t := range want {
		if semantics.IsCollection(t) || semantics.IsElementType(t) {
			return true
		}
	}
	return false
}
