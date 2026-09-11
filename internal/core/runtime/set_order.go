package runtime

import (
	"cmp"
	"math"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// canonicalCompare is the total order a set enumerates its elements in, so that
// every operation consuming a set — a trace rendering it, `collect` over it, a
// sequence it flows into — sees equal sets alike. Values order by class first
// (null, Booleans, numbers, complex numbers, strings, quantities, enumeration
// literals, objects, then every other kind), then within a class by their own
// order where they have one (numeric, lexicographic, dimension then magnitude,
// declaration, object identity), and otherwise by kind and then by what
// valueEqual compares, so that only equal values compare as neither before nor
// after, whichever kind carries them. It orders a before b as negative, after as
// positive, and equal values — valueEqual ones — as zero.
func (ctx *Context) canonicalCompare(a, b Value) int {
	a, b = canonicalRepresentative(a), canonicalRepresentative(b)
	ca, cb := canonicalClass(a), canonicalClass(b)
	if ca != cb {
		return cmp.Compare(ca, cb)
	}
	switch ca {
	case classNull:
		return cmp.Compare(a.Kind, b.Kind)
	case classBool:
		return compareBool(a.Const.Bool, b.Const.Bool)
	case classNumber:
		return compareNumbers(a.Const, b.Const)
	case classComplex:
		x, y := a.Complex(), b.Complex()
		if c := cmp.Compare(real(x), real(y)); c != 0 {
			return c
		}
		return cmp.Compare(imag(x), imag(y))
	case classString:
		return strings.Compare(a.Str(), b.Str())
	case classQuantity:
		qa, qb := ctx.canonicalQuantity(*a.Quantity()), ctx.canonicalQuantity(*b.Quantity())
		if c := strings.Compare(qa.Unit.Term.DimensionKey(), qb.Unit.Term.DimensionKey()); c != 0 {
			return c
		}
		if c, err := semantics.CompareMagnitudes(qa, qb); err == nil {
			return c
		}
	case classEnumLiteral:
		return compareSymbols(a.Literal(), b.Literal())
	case classObject:
		if a.Kind != b.Kind {
			return cmp.Compare(a.Kind, b.Kind)
		}
		if a.Kind == ValVariant {
			return compareSymbols(a.Variant(), b.Variant())
		}
		return cmp.Compare(a.Instance, b.Instance)
	}
	if a.Kind != b.Kind {
		return cmp.Compare(a.Kind, b.Kind)
	}
	return ctx.compareContents(a, b)
}

// canonicalCompare is the canonical order judged with no context.
func canonicalCompare(a, b Value) int {
	return (*Context)(nil).canonicalCompare(a, b)
}

// canonicalQuantity is the quantity as its reference measures it: a point on an interval
// scale carried through its anchor as `==` does; a point on any other scale stays as written.
func (ctx *Context) canonicalQuantity(q Quantity) Quantity {
	if ctx == nil || ctx.model == nil {
		return q
	}
	if scale, ok, err := ctx.pointOf(&q); err != nil || (ok && !ctx.isIntervalScale(scale)) {
		return q
	}
	if ratio, err := ctx.toRatioReference(q); err == nil {
		return ratio
	}
	return q
}

// canonicalMagnitudes compares two quantities as their references measure them.
func (ctx *Context) canonicalMagnitudes(a, b Quantity) (int, error) {
	return semantics.CompareMagnitudes(ctx.canonicalQuantity(a), ctx.canonicalQuantity(b))
}

// canonicalRepresentative is the value valueEqual reads v as, so equal values
// of different kinds take one place: null for every empty collection, the Real
// for a complex number on the real axis.
func canonicalRepresentative(v Value) Value {
	if isEmptyValue(v) {
		return Value{Kind: ValNull}
	}
	if v.Kind == ValComplex {
		if re, ok := v.realPart(); ok {
			return realConst(re)
		}
	}
	return v
}

// compareContents orders two structured values of one kind by what valueEqual
// compares: shape and elements, components, unit, reference key, or the calc,
// object and run a function is a value of.
func (ctx *Context) compareContents(a, b Value) int {
	if a.ref == nil || b.ref == nil {
		return compareBool(a.ref != nil, b.ref != nil)
	}
	switch a.Kind {
	case ValSequence, ValSet:
		return ctx.compareElements(elementsOf(a), elementsOf(b))
	case ValArray:
		x, y := a.Array(), b.Array()
		return cmp.Or(compareInt64s(x.Dimensions, y.Dimensions), ctx.compareElements(x.Elements, y.Elements))
	case ValVector:
		return ctx.compareElements(constValues(a.Vector().Elements), constValues(b.Vector().Elements))
	case ValVectorQuantity:
		x, y := a.VectorQuantity(), b.VectorQuantity()
		return cmp.Or(
			ctx.compareElements(vectorComponents(x), vectorComponents(y)),
			strings.Compare(x.Frame.key(), y.Frame.key()),
		)
	case ValTensorQuantity:
		x, y := a.TensorQuantity(), b.TensorQuantity()
		return cmp.Or(compareInt64s(x.Dimensions, y.Dimensions), ctx.compareElements(x.components(), y.components()))
	case ValQuantity:
		// Two quantities of one dimension whose magnitudes will not compare: by
		// unit, then by the number written.
		x, y := a.Quantity(), b.Quantity()
		return cmp.Or(
			strings.Compare((&MeasurementRef{Unit: x.Unit}).key(), (&MeasurementRef{Unit: y.Unit}).key()),
			compareNumbers(x.Num, y.Num),
		)
	case ValMeasurementRef:
		return strings.Compare(a.MeasurementRef().key(), b.MeasurementRef().key())
	case ValCoordinateFrame:
		return strings.Compare(a.CoordinateFrame().key(), b.CoordinateFrame().key())
	case ValCoordinateTransformation:
		return strings.Compare(a.CoordinateTransformation().key(), b.CoordinateTransformation().key())
	case ValExpr:
		if a.Expr() == nil || b.Expr() == nil {
			return compareBool(a.Expr() != nil, b.Expr() != nil)
		}
		x, y := a.Expr().Span(), b.Expr().Span()
		return cmp.Or(
			cmp.Compare(x.Offset, y.Offset),
			cmp.Compare(x.Len, y.Len),
			strings.Compare(FormatTraceValue(a), FormatTraceValue(b)),
		)
	case ValFunction:
		return cmp.Or(
			compareSymbols(a.Function(), b.Function()),
			cmp.Compare(instanceID(a.FunctionSelf()), instanceID(b.FunctionSelf())),
			cmp.Compare(a.functionRun(), b.functionRun()),
		)
	case ValMetaobject:
		return cmp.Or(
			compareSymbols(a.MetaobjectElement(), b.MetaobjectElement()),
			compareSymbols(a.MetaobjectClass(), b.MetaobjectClass()),
		)
	}
	return 0
}

// instanceID is the identity of the object a function closes over, 0 for none.
func instanceID(inst *Instance) int64 {
	if inst == nil {
		return 0
	}
	return inst.ID
}

// vectorComponents is every axis of the vector as a scalar quantity value.
func vectorComponents(vq *VectorQuantity) []Value {
	out := make([]Value, len(vq.Num))
	for i := range out {
		out[i] = NewQuantityValue(vq.component(i))
	}
	return out
}

// compareInt64s orders two shapes lexicographically, a shorter prefix first.
func compareInt64s(a, b []int64) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := cmp.Compare(a[i], b[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a), len(b))
}

// compareElements orders two element lists lexicographically, a shorter prefix first.
func (ctx *Context) compareElements(a, b []Value) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if c := ctx.canonicalCompare(a[i], b[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(a), len(b))
}

// compareSymbols orders declarations by qualified name, then by the document
// and position declaring them, so same-named literals of different
// enumerations still order deterministically.
func compareSymbols(a, b *symbols.Symbol) int {
	if a == b {
		return 0
	}
	if a == nil || b == nil {
		return compareBool(a != nil, b != nil)
	}
	if c := strings.Compare(symbols.FQNOf(a), symbols.FQNOf(b)); c != 0 {
		return c
	}
	if c := strings.Compare(a.DocName, b.DocName); c != 0 {
		return c
	}
	return cmp.Compare(a.DeclSpan.Offset, b.DeclSpan.Offset)
}

func compareBool(a, b bool) int {
	if a == b {
		return 0
	}
	if !a {
		return -1
	}
	return 1
}

const (
	classNull = iota
	classBool
	classNumber
	classComplex
	classString
	classQuantity
	classEnumLiteral
	classObject
	classOther
)

// canonicalClass groups values so that only values with an order of their own
// are compared by it.
func canonicalClass(v Value) int {
	switch v.Kind {
	case ValNull, ValInvalid:
		return classNull
	case ValConst:
		if v.Const.Kind == semantics.ValBool {
			return classBool
		}
		return classNumber
	case ValComplex:
		return classComplex
	case ValString:
		return classString
	case ValQuantity:
		if v.Quantity() != nil {
			return classQuantity
		}
	case ValEnumLiteral:
		return classEnumLiteral
	case ValInstance, ValVariant:
		return classObject
	}
	return classOther
}

// compareNumbers orders the numeric constants exactly, infinity above every finite number.
func compareNumbers(a, b semantics.Value) int {
	switch {
	case a.Kind == semantics.ValInt && b.Kind == semantics.ValInt:
		return cmp.Compare(a.Int, b.Int)
	case a.Kind == semantics.ValInt && b.Kind == semantics.ValReal:
		return compareIntReal(a.Int, b.Real)
	case a.Kind == semantics.ValReal && b.Kind == semantics.ValInt:
		return -compareIntReal(b.Int, a.Real)
	}
	return cmp.Compare(numberOf(a), numberOf(b))
}

// compareIntReal orders an Integer against a Real without rounding the Integer
// to float64: by whole part first, then by the Real's fraction.
func compareIntReal(i int64, r float64) int {
	switch {
	case math.IsNaN(r):
		return cmp.Compare(0.0, r)
	case r >= -float64(math.MinInt64):
		return -1
	case r < float64(math.MinInt64):
		return 1
	}
	whole := math.Trunc(r)
	if c := cmp.Compare(i, int64(whole)); c != 0 {
		return c
	}
	return cmp.Compare(0, r-whole)
}

func numberOf(v semantics.Value) float64 {
	if v.Kind == semantics.ValInfinity {
		return math.Inf(1)
	}
	return v.AsReal()
}
