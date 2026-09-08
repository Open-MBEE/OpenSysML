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
	closure *functionValue
}

// valueKeyFunc extracts a comparable key from a Value. Values valueEqual holds
// equal share a key: a whole number has the Integer's whatever kind carries it,
// and every empty value has null's.
func valueKeyFunc(v Value) valueKey {
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
	case ValSequence:
		key.colHash = hashSequence(v.Sequence())
	case ValSet:
		key.colHash = hashSet(v.Set())
	case ValVariant:
		key.variant = v.Variant()
	case ValEnumLiteral:
		key.literal = v.Literal()
	case ValQuantity:
		if v.Quantity() != nil {
			key.realVal = v.Quantity().BaseMagnitude()
			key.strVal = v.Quantity().Unit.Term.DimensionKey()
		}
	case ValArray, ValVector, ValVectorQuantity, ValTensorQuantity:
		key.colHash = structuredKey(v)
	case ValMeasurementRef:
		key.strVal = v.MeasurementRef().key()
	case ValCoordinateFrame:
		key.strVal = v.CoordinateFrame().key()
	case ValCoordinateTransformation:
		key.strVal = v.CoordinateTransformation().key()
	case ValFunction:
		key.calc = v.Function()
		if self := v.FunctionSelf(); self != nil {
			key.instID = self.ID
		}
		// A function closing over a body's bindings is one only with itself.
		if v.FunctionClosesOverBody() {
			key.closure = v.function()
		}
	}
	return key
}

// hashSequence computes a content-based hash for a Sequence.
func hashSequence(seq *Sequence) uint64 {
	if seq == nil {
		return 0
	}
	h := fnv.New64a()
	for _, elem := range seq.elements {
		k := valueKeyFunc(elem)
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

// hashSet computes a content-based hash for a Set (order-invariant).
func hashSet(set *Set) uint64 {
	if set == nil {
		return 0
	}
	var sum uint64
	for _, bucket := range set.elements {
		for _, elem := range bucket {
			k := valueKeyFunc(elem)
			// #nosec G115 -- wrapping is intended: this is a hash, not arithmetic.
			sum += uint64(k.intVal)
		}
	}
	return sum
}
