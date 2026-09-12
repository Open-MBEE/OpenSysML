package runtime

import (
	"hash/fnv"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// valueKey is a comparable projection of Value for use as a map key.
type valueKey struct {
	kind    ValueKind
	intVal  int64
	realVal float64
	imagVal float64
	boolVal bool
	infVal  bool
	strVal  string
	instID  int64
	colHash uint64
	variant *symbols.Symbol
	literal *symbols.Symbol
	calc    *symbols.Symbol
	run     int64 // the body run a function closes over (Value.functionRun)
}

// valueKeyFunc extracts a comparable key from a Value. Values valueEqual holds
// equal share a key: a whole number has the Integer's whatever kind carries it,
// every empty value has null's, and a set hashes its members in canonical order.
func valueKeyFunc(v Value) valueKey {
	return (*Context)(nil).valueKey(v)
}

// valueKey is valueKeyFunc in the context: a point on a measurement scale keys
// by its magnitude on the ratio reference, as ctx.valueEqual compares it.
func (ctx *Context) valueKey(v Value) valueKey {
	if isEmptyValue(v) {
		return valueKey{kind: ValNull}
	}
	key := valueKey{kind: v.Kind}
	switch v.Kind {
	case ValConst:
		switch v.Const.Kind {
		case semantics.ValInt:
			key.intVal = v.Const.Int
		case semantics.ValReal:
			if n, ok := v.Const.WholeNumber(); ok {
				key.intVal = n
			} else {
				key.realVal = v.Const.Real
			}
		case semantics.ValBool:
			key.boolVal = v.Const.Bool
		case semantics.ValInfinity:
			key.infVal = true
		}
	case ValComplex:
		// A complex number on the real axis is the number it equals and has its key.
		if re, ok := v.realPart(); ok {
			return valueKeyFunc(realConst(re))
		}
		key.realVal, key.imagVal = real(v.Complex()), imag(v.Complex())
	case ValString:
		key.strVal = v.Str()
	case ValInstance:
		key.instID = v.Instance
	case ValSequence, ValSet:
		key.colHash = ctx.hashElements(elementsOf(v))
	case ValVariant:
		key.variant = v.Variant()
	case ValEnumLiteral:
		key.literal = v.Literal()
	case ValQuantity:
		if v.Quantity() != nil {
			q := ctx.canonicalQuantity(*v.Quantity())
			key.realVal = q.BaseMagnitude()
			key.strVal = q.Unit.Term.DimensionKey()
		}
	case ValArray, ValVector, ValVectorQuantity, ValTensorQuantity:
		key.colHash = ctx.structuredKey(v)
	case ValMeasurementRef:
		key.strVal = v.MeasurementRef().key()
	case ValCoordinateFrame:
		key.strVal = v.CoordinateFrame().key()
	case ValCoordinateTransformation:
		key.strVal = v.CoordinateTransformation().key()
	case ValFunction:
		key.calc, key.run = v.Function(), v.functionRun()
		if self := v.FunctionSelf(); self != nil {
			key.instID = self.ID
		}
	case ValUndetermined:
		key.strVal = v.Undetermined().Reason()
	}
	return key
}

// hashElements computes a content-based hash over elements in order.
func (ctx *Context) hashElements(elements []Value) uint64 {
	h := fnv.New64a()
	for _, elem := range elements {
		k := ctx.valueKey(elem)
		// #nosec G115 G104 -- truncation is deliberate for a hash, and
		// hash.Hash.Write is documented never to return an error.
		h.Write([]byte{byte(k.kind)})
		if k.intVal != 0 {
			// #nosec G115 G104 -- see above.
			h.Write([]byte{byte(k.intVal), byte(k.intVal >> 8)})
		}
	}
	return h.Sum64()
}
