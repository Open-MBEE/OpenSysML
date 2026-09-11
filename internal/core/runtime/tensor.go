package runtime

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TensorQuantity is a Quantities::TensorQuantityValue of order two or more: row-major
// `num` over `dimensions`, each in its component's unit. Orders 0 and 1 are Quantity and VectorQuantity.
type TensorQuantity struct {
	Dimensions []int64
	Num        []semantics.Value
	Units      []Unit
	// IsBound is `isBound` when BoundKnown; a calculation the library gives no body
	// leaves its result's boundness unstated.
	IsBound    bool
	BoundKnown bool
	// MRef is the TensorMeasurementReference object the tensor was built over, whose
	// members `mRef` keeps answering; 0 for one known by its components only.
	MRef int64
}

// NewTensorQuantityValue wraps a tensor quantity; num and units hold one entry per
// component, flattenedSize of dimensions in all.
func NewTensorQuantityValue(dimensions []int64, num []semantics.Value, units []Unit) Value {
	return Value{Kind: ValTensorQuantity, ref: &TensorQuantity{Dimensions: dimensions, Num: num, Units: units}}
}

// Rank is the number of dimensions, the tensor's `order`.
func (tq *TensorQuantity) Rank() int {
	return len(tq.Dimensions)
}

// FlattenedSize is the number of components.
func (tq *TensorQuantity) FlattenedSize() int64 {
	return int64(len(tq.Num))
}

// component is the scalar quantity at one row-major offset.
func (tq *TensorQuantity) component(i int) *Quantity {
	return &Quantity{Num: tq.Num[i], Unit: tq.Units[i]}
}

// components is every component as a scalar quantity value, in row-major order.
func (tq *TensorQuantity) components() []Value {
	out := make([]Value, len(tq.Num))
	for i := range out {
		out[i] = NewQuantityValue(tq.component(i))
	}
	return out
}

// componentArray is the Array of the components' quantities, which indexes as
// Collections::Array does.
func (tq *TensorQuantity) componentArray() *Array {
	return &Array{Dimensions: tq.Dimensions, Elements: tq.components()}
}

// sharedUnit is the one unit every component measures in, when their references are equal.
func (tq *TensorQuantity) sharedUnit() (Unit, bool) {
	return (&VectorQuantity{Num: tq.Num, Units: tq.Units}).sharedUnit()
}

// format renders `Tensor(2, 2)[1.0, 2.0, 3.0, 4.0] [Pa]`, or each component with
// its own unit, `Tensor(2, 2)[1.0 [Pa], 2.0 [kPa], …]`, when the units differ.
func (tq *TensorQuantity) format(element func(semantics.Value) string) string {
	dims := make([]string, len(tq.Dimensions))
	for i, d := range tq.Dimensions {
		dims[i] = fmt.Sprintf("%d", d)
	}
	shape := "Tensor(" + strings.Join(dims, ", ") + ")"
	parts := make([]string, len(tq.Num))
	if unit, ok := tq.sharedUnit(); ok {
		for i, n := range tq.Num {
			parts[i] = element(n)
		}
		return shape + "[" + strings.Join(parts, ", ") + "] [" + unit.String() + "]"
	}
	for i := range tq.Num {
		parts[i] = tq.component(i).TextWithMagnitude(element(tq.Num[i]))
	}
	return shape + "[" + strings.Join(parts, ", ") + "]"
}

// tensorQuantityEqual holds for tensor quantities of the same dimensions whose
// components are equal quantities.
func (ctx *Context) tensorQuantityEqual(a, b *TensorQuantity) bool {
	if a == nil || b == nil {
		return a == b
	}
	if !equalInt64s(a.Dimensions, b.Dimensions) || len(a.Num) != len(b.Num) {
		return false
	}
	for i := range a.Num {
		if !ctx.valueEqual(NewQuantityValue(a.component(i)), NewQuantityValue(b.component(i))) {
			return false
		}
	}
	return true
}

// tensorQuantityHeldSame is heldSame over tensor quantities: the same shape,
// boundness and reference object, and every component held the same.
func tensorQuantityHeldSame(prior, now *TensorQuantity) bool {
	if prior == nil || now == nil || !equalInt64s(prior.Dimensions, now.Dimensions) {
		return prior == now
	}
	if prior.BoundKnown != now.BoundKnown || prior.IsBound != now.IsBound || prior.MRef != now.MRef {
		return false
	}
	for i := range prior.Num {
		if !heldSame(NewQuantityValue(prior.component(i)), NewQuantityValue(now.component(i))) {
			return false
		}
	}
	return true
}

// Library type a tensor quantity is a value of.
const tensorQuantityTypeFQN = "Quantities::TensorQuantityValue"

// tensorQuantityFeature answers a tensor quantity's Array, TensorQuantityValue
// and `mRef` features from the value; a covariance the model never set is a typed error.
func (ctx *Context) tensorQuantityFeature(val Value, name string) (Value, bool, error) {
	tq := val.TensorQuantity()
	if base, ok := vectorQuantityAliases[name]; ok {
		name = base
	}
	switch name {
	case arrayDimensionsFeature:
		return ctx.answerSequence(integerValues(tq.Dimensions))
	case arrayRankFeature:
		return integerValue(int64(tq.Rank())), true, nil
	case arrayFlattenedSizeFeature:
		return integerValue(tq.FlattenedSize()), true, nil
	case arrayElementsFeature:
		return ctx.answerSequence(constValues(tq.Num))
	case mRefIsBoundFeature:
		if !tq.BoundKnown {
			return Value{}, true, fmt.Errorf(
				"%w: %s::isBound: the tensor is the result of a TensorCalculations calculation the library gives no body, which states nothing about whether its result is bound",
				ErrUnevaluableLibraryFunction, tensorQuantityTypeFQN,
			)
		}
		return boolValue(tq.IsBound), true, nil
	case vectorQuantityMRefFeature:
		ref, err := ctx.tensorQuantityMRef(tq)
		return ref, true, err
	case tensorContravariantOrder, tensorCovariantOrder:
		return Value{}, true, fmt.Errorf(
			"%w: %s::%s: the library declares it a Natural with no default, TensorCalculations::'[' sets none, and only their sum is constrained (orderSum: contravariantOrder + covariantOrder == order, here %d)",
			ErrUnevaluableLibraryFunction, tensorQuantityTypeFQN, name, tq.Rank(),
		)
	}
	return Value{}, false, nil
}

// tensorQuantityMRef is the TensorMeasurementReference object the tensor was built
// over while it lives, else the Array of its components' scalar references.
func (ctx *Context) tensorQuantityMRef(tq *TensorQuantity) (Value, error) {
	if inst, ok := ctx.instances[tq.MRef]; ok && tq.MRef != 0 {
		if val, isArray, err := ctx.arrayOfObject(inst); isArray {
			return val, err
		}
	}
	if err := ctx.chargeElements(int64(len(tq.Units))); err != nil {
		return Value{}, err
	}
	refs := make([]Value, len(tq.Units))
	for i, unit := range tq.Units {
		refs[i] = measurementRefOf(unit)
	}
	return NewArrayValue(tq.Dimensions, refs), nil
}

// tensorQuantityConforms judges a tensor written to a feature: by its type,
// and by each component's unit against the dimension the declared type fixes.
func (ctx *Context) tensorQuantityConforms(tq *TensorQuantity, declared *symbols.Symbol) (bool, string, error) {
	for i := range tq.Num {
		if ok, refusal, err := ctx.quantityConforms(NewQuantityValue(tq.component(i)), declared); !ok || err != nil {
			return ok, refusal, err
		}
	}
	return true, "", nil
}
