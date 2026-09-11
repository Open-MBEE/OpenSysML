package runtime

import (
	"fmt"
	"math"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// Quantity is a scalar quantity value, the representation the semantic layer
// shares with the query engine: a magnitude in a measurement reference.
type Quantity = semantics.Quantity

// Unit is a measurement reference as a quantity carries it.
type Unit = semantics.Unit

// quantityResult is the runtime value of a quantity computation: a number when
// its unit cancelled, else the quantity.
func quantityResult(q semantics.Quantity, err error) (Value, error) {
	if err != nil {
		return Value{}, err
	}
	if q.Unit.None() {
		return Value{Kind: ValConst, Const: q.Num}, nil
	}
	return NewQuantityValue(&q), nil
}

// coherentResult is quantityResult over a quantity whose unit an operation
// composed, re-expressed in the coherent unit the library declares for its dimension.
func (ctx *Context) coherentResult(q semantics.Quantity, err error) (Value, error) {
	if err == nil && ctx != nil {
		q, err = ctx.model.CoherentQuantity(q, nil)
	}
	return quantityResult(q, err)
}

// composedQuantity is a result in the canonical form of its composed unit (`m**2`, `m`).
func (ctx *Context) composedQuantity(num semantics.Value, product semantics.UnitProduct, term semantics.UnitTerm) (Value, error) {
	return ctx.coherentResult(semantics.ComposedQuantity(num, product, term))
}

// evalIndexExpr evaluates `magnitude [unit]`, the quantity expression: a
// number and the measurement reference it is expressed in. The sequence index
// `seq#(i)` shares the node but not the meaning; the bracket the notation was
// written with is what tells the two apart, and the index is evaluated as the
// sequence operation it is (collections.go).
func (ec *EvalContext) evalIndexExpr(n *ast.IndexExpr) (Value, error) {
	if !n.Bracket {
		return ec.evalSequenceIndex(n)
	}
	// A frame or scale in unit position is the VectorCalculations::'[' case.
	if frame, ok, err := ec.frameIndex(n.Index); err != nil {
		return Value{}, err
	} else if ok {
		return ec.framedQuantity(n, frame)
	}
	term, err := ec.ctx.model.UnitTermOfExpr(ec.scope, n.Index)
	if err != nil {
		return Value{}, ec.notAQuantityError(n, err)
	}

	magnitude, err := ec.valueOperand(n.Operand)
	if err != nil {
		return Value{}, err
	}
	if magnitude.Kind != ValVector && (magnitude.Kind != ValConst || !magnitude.Const.IsNumeric()) {
		return Value{}, fmt.Errorf("%w: magnitude of a quantity is %s, want a number or a vector", ErrNotAQuantity, magnitude.Kind)
	}

	product, err := ec.ctx.model.UnitProductOfExpr(ec.scope, n.Index)
	if err != nil {
		return Value{}, fmt.Errorf("%w: %w", ErrNotAQuantity, err)
	}
	unit := Unit{Text: semantics.UnitExprText(n.Index), Product: product, Term: term}
	if magnitude.Kind == ValVector {
		// A vector in a unit is the vector quantity with that unit on every axis.
		num := magnitude.Vector().Elements
		units := make([]Unit, len(num))
		for i := range units {
			units[i] = unit
		}
		return ec.ctx.vectorQuantityValue(num, units)
	}
	return NewQuantityValue(&Quantity{Num: magnitude.Const, Unit: unit}), nil
}

// notAQuantityError reports a bracket whose index is no unit; over an operand
// declared a collection it adds that `#(…)` indexes. The operand is not evaluated.
func (ec *EvalContext) notAQuantityError(n *ast.IndexExpr, cause error) error {
	err := fmt.Errorf("%w: %w", ErrNotAQuantity, cause)
	if what, ok := ec.declaredCollection(n.Operand); ok {
		return fmt.Errorf("%w; `[…]` is the quantity notation `num [unit]`, index %s with `#(…)`", err, what)
	}
	return err
}

// declaredCollection names the collection an operand's declaration makes it:
// an Array, a vector, or a feature of more than one value.
func (ec *EvalContext) declaredCollection(operand ast.Node) (string, bool) {
	var qn *ast.QualifiedName
	switch node := operand.(type) {
	case *ast.QualifiedName:
		qn = node
	case *ast.FeatureReference:
		qn = node.Name
	}
	if qn == nil || ec.ctx.resolver == nil {
		return "", false
	}
	sym, ok := ec.ctx.resolver.ResolveQualified(ec.scope, qn)
	if !ok || sym == nil || !semantics.IsShapeFeature(sym) {
		return "", false
	}
	if typ := ec.ctx.extractType(sym); typ != nil {
		for _, lib := range []struct{ fqn, what string }{{vectorTypeFQN, "a vector"}, {arrayTypeFQN, "an array"}} {
			if libSym := ec.ctx.librarySymbol(lib.fqn); libSym != nil && ec.ctx.model.Conforms(typ, libSym) {
				return lib.what, true
			}
		}
	}
	if !ec.ctx.occursOnce(sym) {
		return "a sequence", true
	}
	return "", false
}

// asQuantity views a value as a quantity: a quantity as itself, and a bare
// number as a magnitude of dimension one, since a number is commensurable with
// a count or a ratio of like quantities but with nothing else.
func asQuantity(val Value) (*Quantity, bool) {
	switch val.Kind {
	case ValQuantity:
		return val.Quantity(), true
	case ValConst:
		if val.Const.IsNumeric() {
			return &Quantity{Num: val.Const, Unit: semantics.UnitOne()}, true
		}
	}
	return nil, false
}

// inUnit is a magnitude in the unit of a quantity asQuantity read; the unit a bare
// number was read with is no unit at all, so the result is a bare number again.
func inUnit(num semantics.Value, unit Unit) (Value, error) {
	return quantityResult(semantics.InUnit(num, unit))
}

// quantityOperands views both operands of an operation as quantities, reporting
// false when neither is one — then the operation is ordinary arithmetic.
func quantityOperands(left, right Value) (*Quantity, *Quantity, bool) {
	if left.Kind != ValQuantity && right.Kind != ValQuantity {
		return nil, nil, false
	}
	lq, lok := asQuantity(left)
	rq, rok := asQuantity(right)
	if !lok || !rok {
		return nil, nil, false
	}
	return lq, rq, true
}

// scaleQuantities evaluates a product or quotient of quantities, whose unit is
// the product or quotient of theirs — `10 [m] / 2 [s]` is `5 ['m/s']`. A point
// on a scale has no multiple and is no unit factor, so it is refused.
func (ctx *Context) scaleQuantities(op ast.OperatorKind, left, right *Quantity) (Value, error) {
	if err := ctx.refusePoints(operatorText(op), scaleNotAFactor, left, right); err != nil {
		return Value{}, err
	}
	return ctx.coherentResult(semantics.ScaleQuantities(op, *left, *right))
}

// scaleNotAFactor is why a point enters no product, quotient, power or root.
const scaleNotAFactor = "a point has no multiple and its scale is no unit to compose; convert it to a unit with ConvertQuantity first"

// powQuantity raises a quantity to a constant exponent, its unit included.
func (ctx *Context) powQuantity(base *Quantity, exponent semantics.Value) (Value, error) {
	if !exponent.IsNumeric() {
		return Value{}, fmt.Errorf("%w: exponent of a quantity is not a number", ErrTypeMismatch)
	}
	if err := ctx.refusePoints(operatorText(ast.OpPow), scaleNotAFactor, base); err != nil {
		return Value{}, err
	}
	return ctx.coherentResult(semantics.PowQuantity(*base, exponent))
}

// sqrtQuantity is the square root of a quantity, `9 [m**2]` giving `3.0 [m]`;
// a unit with a base unit at an odd power has no root, so `sqrt(9 [m])` is rejected.
// A root the named units cannot spell at whole powers (`km*m`) is taken over the
// base units instead, unless a dimension-one unit (`rad`, `°`) would be lost there.
func (ctx *Context) sqrtQuantity(q *Quantity) (Value, error) {
	if err := ctx.refusePoints("sqrt", scaleNotAFactor, q); err != nil {
		return Value{}, err
	}
	for _, f := range q.Unit.Term.Factors {
		if math.Mod(f.Exponent, 2) != 0 {
			return Value{}, fmt.Errorf("%w: %s (%s) raises %s to the odd power %g",
				ErrUnitRoot, q.Unit, q.Unit.Term, f.Unit.Name, f.Exponent)
		}
	}
	magnitude, term := toReal(q.Num), q.Unit.Term
	if magnitude < 0 {
		return Value{}, fmt.Errorf("%w: sqrt of a negative quantity %s", semantics.ErrArithmeticDomain, q)
	}
	root, ok := q.Unit.Product.Root(2)
	if !ok {
		for _, f := range q.Unit.Product.Powers {
			if f.DimensionOne && math.Mod(f.Exponent, 2) != 0 {
				return Value{}, fmt.Errorf("%w: %s raises the dimension-one unit %s to the odd power %g",
					ErrUnitRoot, q.Unit, f.Name, f.Exponent)
			}
		}
		magnitude = q.BaseMagnitude()
		term = semantics.UnitTerm{Scale: semantics.UnitScale(1), Factors: term.Factors}
		root, _ = term.BaseProduct().Root(2)
	}
	num, err := semantics.RealResult(math.Sqrt(magnitude))
	if err != nil {
		return Value{}, err
	}
	return ctx.composedQuantity(num, root, term.Pow(0.5))
}
