package passes

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// msgIncommensurableBinding reports a value of one dimension, unit or frame
// bound to a feature typed by another.
const msgIncommensurableBinding = "cannot bind %s to a feature typed by %s"

// checkDimensions warns when an operator that requires commensurable operands
// combines quantities of statically known, incommensurable dimensions —
// `mass < 1000.0[m]` — which evaluation rejects with an incommensurable-units
// error. It stays silent whenever either dimension is not statically determined,
// so a unit that only evaluation knows is never guessed at.
func (ec *exprChecker) checkDimensions(scope *symbols.Scope, e *ast.OperatorExpr) {
	if ec.model == nil || !commensurabilityRequired(e.Operator) || len(e.Operands) != 2 {
		return
	}
	// A bare zero is the null quantity of every dimension, so a comparison reads
	// it in the other operand's unit (`length > 0`), as evaluation does.
	if comparesOperands(e.Operator) && (ec.isBareZero(scope, e.Operands[0]) || ec.isBareZero(scope, e.Operands[1])) {
		return
	}
	lhs, ok := ec.model.DimensionOfExpr(scope, e.Operands[0])
	if !ok {
		return
	}
	rhs, ok := ec.model.DimensionOfExpr(scope, e.Operands[1])
	if !ok {
		return
	}
	if lhs.Term.Commensurable(rhs.Term) {
		return
	}
	ec.warnf(e.Span(), "operator '%s' combines incommensurable quantities: %s and %s",
		e.Operator, describeDimension(lhs), describeDimension(rhs))
}

// checkValueDimension reports a bound value measured in a dimension the
// target's declared quantity value type does not measure in — a speed bound to
// a DurationValue, whose mRef the libraries narrow to DurationUnit. A target
// declaring no quantity kind, and a value stating no measurement, are silent.
func (ec *exprChecker) checkValueDimension(valueScope, declScope *symbols.Scope, d featureDecl, value ast.Node) {
	if ec.model == nil {
		return
	}
	declared := ec.declaredTypeSymbol(declScope, d.relationships)
	if declared == nil {
		return
	}
	want, known := ec.model.DimensionOfType(declared)
	for _, element := range valueElements(value) {
		// A collection value binds each element it may hold, which is measured on its own.
		if elements, collection := ec.model.CollectionElements(valueScope, element); collection {
			for _, produced := range elements {
				if produced.Node != nil {
					ec.checkElementDimension(produced.Scope, declared, want, known, produced.Node)
				}
			}
			continue
		}
		ec.checkElementDimension(valueScope, declared, want, known, element)
	}
}

// checkElementDimension reports one bound element measured in a dimension the target's
// declared type, of dimension want where known, does not measure in.
func (ec *exprChecker) checkElementDimension(scope *symbols.Scope, declared *symbols.Symbol, want semantics.Dimension, known bool, element ast.Node) {
	if ec.judgedByType(scope, element) {
		// A named value is judged against the target by specialization,
		// which reports the same mismatch as a clash of types.
		return
	}
	if statesNoMeasurement(element) {
		return
	}
	if ec.judgedAsMeasurementRef(scope, declared, element) || ec.judgedAsFramedQuantity(scope, declared, element) || !known {
		return
	}
	got, ok := ec.model.DimensionOfExpr(scope, element)
	if !ok || want.Term.Commensurable(got.Term) {
		return
	}
	ec.errorf(element.Span(), msgIncommensurableBinding,
		describeDimension(got), describeDimension(want))
}

// judgedAsMeasurementRef judges a unit composed by `*`, `/` or `**` as the DerivedUnit
// it evaluates to (`m * s` refused by AreaUnit, `m * m` by AreaValue), and a frame
// composed by `*` or `/` as the CoordinateFrame it evaluates to; false otherwise.
func (ec *exprChecker) judgedAsMeasurementRef(scope *symbols.Scope, declared *symbols.Symbol, element ast.Node) bool {
	e, ok := element.(*ast.OperatorExpr)
	if !ok {
		return false
	}
	c, ok := ec.model.MeasurementRefExprConformance(scope, e, declared)
	if !ok {
		c, ok = ec.model.CoordinateFrameExprConformance(scope, e, declared)
	}
	if !ok {
		return false
	}
	if c.Known && !c.Holds {
		ec.errorf(element.Span(), msgIncommensurableBinding, c.Found, declared.Name)
	}
	return true
}

// judgedAsFramedQuantity judges numbers written in a coordinate frame, named or
// composed (`(1, 2, 3) [spatialCF / s]`), as the VectorQuantityValue they are;
// false for a literal in a scalar unit, which is judged by dimension.
func (ec *exprChecker) judgedAsFramedQuantity(scope *symbols.Scope, declared *symbols.Symbol, element ast.Node) bool {
	n, ok := element.(*ast.IndexExpr)
	if !ok {
		return false
	}
	c, ok := ec.model.FramedQuantityConformance(scope, n, declared)
	if !ok {
		return false
	}
	if c.Known && !c.Holds {
		ec.errorf(element.Span(), msgIncommensurableBinding, c.Found, declared.Name)
	}
	return true
}

// judgedByType reports whether value conformance already types the element,
// which it does for a name and for an invocation of a behavior.
func (ec *exprChecker) judgedByType(scope *symbols.Scope, element ast.Node) bool {
	return ec.valueTypeSymbol(scope, element) != nil || ec.invocationResultTypeSymbol(scope, element) != nil
}

// statesNoMeasurement reports an expression written out of plain numbers alone:
// it names no unit, so it is read in the unit its target fixes rather than
// judged against it.
func statesNoMeasurement(element ast.Node) bool {
	switch n := element.(type) {
	case *ast.LiteralInteger, *ast.LiteralReal:
		return true
	case *ast.OperatorExpr:
		for _, operand := range n.Operands {
			if !statesNoMeasurement(operand) {
				return false
			}
		}
		return len(n.Operands) > 0
	}
	return false
}

// isBareZero reports an operand folding to zero and naming no unit: a literal
// or constant arithmetic such as `1 - 1`.
func (ec *exprChecker) isBareZero(scope *symbols.Scope, element ast.Node) bool {
	q, ok := ec.model.EvalQuantity(scope, element)
	return ok && q.Unit.None() && q.Num.IsNumeric() && q.Num.AsReal() == 0
}

// commensurabilityRequired reports whether an operator only relates operands of
// one dimension. A product or quotient combines dimensions instead, and a
// conditional does not relate its branches to the condition.
func commensurabilityRequired(op ast.OperatorKind) bool {
	switch op {
	case ast.OpAdd, ast.OpSub, ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe, ast.OpEq, ast.OpNeq:
		return true
	default:
		return false
	}
}

// comparesOperands reports an operator whose result is a truth value about its
// operands rather than a quantity of their dimension.
func comparesOperands(op ast.OperatorKind) bool {
	switch op {
	case ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe, ast.OpEq, ast.OpNeq:
		return true
	default:
		return false
	}
}

// describeDimension names the unit an operand was written in and the dimension it
// measures in, so the message says both what was written and why it clashes.
func describeDimension(d semantics.Dimension) string {
	if d.Term.Dimensionless() {
		if d.Unit == "" {
			return "a dimensionless value"
		}
		return fmt.Sprintf("%s (dimensionless)", d.Unit)
	}
	if d.Unit == "" {
		return fmt.Sprintf("a value of dimension %s", d)
	}
	return fmt.Sprintf("%s (dimension %s)", d.Unit, d)
}
