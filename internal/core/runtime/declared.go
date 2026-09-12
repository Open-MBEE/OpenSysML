package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// DeclaredReader reads feature values as the model declares them for a concrete
// carrier, materializing each element once and starting no behavior.
type DeclaredReader struct {
	ctx     *Context
	objects map[*symbols.Symbol]*Instance
}

// NewDeclaredReader creates a reader over a fresh, behavior-free runtime context.
func NewDeclaredReader(model *semantics.Model, resolver *resolve.Resolver) *DeclaredReader {
	ctx := NewContext(NewModel(model, resolver), DefaultMaxSteps)
	ctx.declarative = true
	return &DeclaredReader{ctx: ctx, objects: make(map[*symbols.Symbol]*Instance)}
}

// Read evaluates the named feature of the element sym denotes, resolving every
// leaf through that element's redefinitions. An unbound leaf is a *NoValueError.
func (r *DeclaredReader) Read(sym *symbols.Symbol, name string) (Value, error) {
	inst, err := r.objectOf(sym)
	if err != nil {
		return Value{}, err
	}
	fv, err := inst.GetFeatureValue(r.ctx, name)
	if err != nil {
		return Value{}, err
	}
	val, err := r.ctx.readFeatureValue(fv, name)
	if err != nil {
		return Value{}, err
	}
	if r.ctx.HoldsNoValue(val) {
		return Value{}, &NoValueError{Feature: name, Symbol: fv.Feature.Symbol}
	}
	return val, nil
}

// objectOf materializes the element once, so all its features read from one object.
func (r *DeclaredReader) objectOf(sym *symbols.Symbol) (*Instance, error) {
	if sym == nil {
		return nil, fmt.Errorf("%w: no element to read", ErrUnresolvedReference)
	}
	if inst, ok := r.objects[sym]; ok {
		return inst, nil
	}
	mark := len(r.ctx.created)
	inst, err := r.ctx.materialize(sym, 0, nil, "")
	if err != nil {
		r.ctx.abandonInstancesSince(mark)
		return nil, err
	}
	if r.ctx.registersOccurrence(sym) {
		r.ctx.occurrences[sym] = []int64{inst.ID}
	}
	r.objects[sym] = inst
	return inst, nil
}

// QuantityUnary applies a unary operator to a quantity as evaluation does: a
// point on a measurement scale (`26.85 [SI::'°C_abs']`) has no negative.
func (r *DeclaredReader) QuantityUnary(op ast.OperatorKind, q semantics.Quantity) (semantics.Quantity, error) {
	if op == ast.OpNeg && q.Num.IsNumeric() {
		return scalarQuantity(r.ctx.negateQuantity(&q))
	}
	return semantics.QuantityUnary(op, q)
}

// QuantityBinary applies a binary operator to two quantities as evaluation does,
// the affine cases of a point on a measurement scale included.
func (r *DeclaredReader) QuantityBinary(op ast.OperatorKind, left, right semantics.Quantity) (semantics.Quantity, error) {
	if left.Unit.None() && right.Unit.None() || !left.Num.IsNumeric() || !right.Num.IsNumeric() {
		return semantics.QuantityBinary(op, left, right)
	}
	switch op {
	case ast.OpAdd, ast.OpSub:
		return scalarQuantity(r.ctx.addQuantities(op, &left, &right))
	case ast.OpMul, ast.OpDiv:
		return scalarQuantity(r.ctx.scaleQuantities(op, &left, &right))
	case ast.OpPow:
		if !right.Unit.None() {
			return semantics.Quantity{}, semantics.ErrQuantityOperand
		}
		return scalarQuantity(r.ctx.powQuantity(&left, right.Num))
	case ast.OpEq, ast.OpNeq:
		return scalarQuantity(r.ctx.equalQuantities(op, &left, &right))
	case ast.OpLt, ast.OpGt, ast.OpLe, ast.OpGe:
		return scalarQuantity(r.ctx.compareQuantities(op, &left, &right))
	}
	return semantics.Quantity{}, semantics.ErrQuantityOperand
}

// CompareMagnitudes orders two quantities on the left one's reference, a point
// on a measurement scale carried through its anchor.
func (r *DeclaredReader) CompareMagnitudes(left, right semantics.Quantity) (int, error) {
	return r.ctx.compareMagnitudes(&left, &right)
}

// scalarQuantity reads an evaluated scalar result back as a quantity: a bare
// constant is one in no unit.
func scalarQuantity(val Value, err error) (semantics.Quantity, error) {
	if err != nil {
		return semantics.Quantity{}, err
	}
	switch val.Kind {
	case ValQuantity:
		return *val.Quantity(), nil
	case ValConst:
		return semantics.Quantity{Num: val.Const, Unit: semantics.UnitOne()}, nil
	}
	return semantics.Quantity{}, fmt.Errorf("%w: %s is no quantity", ErrTypeMismatch, val.Kind)
}
