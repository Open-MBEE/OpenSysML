package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// evalCast evaluates `x as T`: a CastExpression's result is the values of x that
// the type T classifies, so it selects values and never converts one (KerML 1.1
// §8.3.4.9). Converting between types is what the library functions do.
func (ec *EvalContext) evalCast(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 1 || n.TypeRef == nil {
		return Value{}, fmt.Errorf("%w: '%s' requires one value and one type",
			ErrTypeMismatch, n.Operator)
	}
	target, ok := ec.resolveClassificationType(n.TypeRef)
	if !ok {
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedType,
			qualifiedNameToString(n.TypeRef))
	}
	value, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	return ec.castValue(value, target)
}

// castValue keeps the values of value that target classifies: element-wise and in
// order for a collection, the value itself or the empty sequence for one value.
func (ec *EvalContext) castValue(value Value, target *symbols.Symbol) (Value, error) {
	switch value.Kind {
	case ValNull, ValInvalid:
		return sequenceOf(nil), nil
	case ValSequence, ValSet:
		elements := elementsOf(value)
		kept := make([]Value, 0, len(elements))
		for _, element := range elements {
			keep, err := ec.castKeeps(element, target)
			if err != nil {
				return Value{}, err
			}
			if keep {
				kept = append(kept, element)
			}
		}
		if value.Kind == ValSet {
			set := NewSet()
			for _, element := range kept {
				set.Add(element)
			}
			if err := ec.ctx.chargeElements(int64(set.Size())); err != nil {
				return Value{}, err
			}
			return NewSetValue(set), nil
		}
		return ec.sequenceFrom(kept, value)
	}
	keep, err := ec.castKeeps(value, target)
	if err != nil {
		return Value{}, err
	}
	if !keep {
		return ec.sequenceFrom(nil, value)
	}
	return value, nil
}

// castKeeps reports whether target classifies one value. The types the value is
// of decide it wherever they are enough; where target is narrower than all of
// them, the value's own content does (castNarrowerKeeps).
func (ec *EvalContext) castKeeps(value Value, target *symbols.Symbol) (bool, error) {
	types, err := ec.castTypes(value)
	if err != nil {
		return false, err
	}
	switch ec.ctx.model.ClassifiesTypes(types, target) {
	case semantics.ClassifiesAll:
		return true, nil
	case semantics.ClassifiesNone:
		return false, nil
	}
	return ec.castNarrowerKeeps(value, target)
}

// castTypes names the types a value is of for a cast: a quantity value is the
// quantity type it is, whose dimension castNarrowerKeeps then judges, and every
// other value is of the types a classification reads it as.
func (ec *EvalContext) castTypes(value Value) ([]*symbols.Symbol, error) {
	if value.Kind == ValQuantity {
		quantity, err := ec.ctx.loadedLibraryType(scalarQuantityTypeFQN)
		if err != nil {
			return nil, err
		}
		return []*symbols.Symbol{quantity}, nil
	}
	types, err := ec.ctx.directValueTypes(ec.scope, value)
	if err != nil {
		if scalar := ec.scalarLibraryType(value); scalar != nil {
			return []*symbols.Symbol{scalar}, nil
		}
		return nil, err
	}
	return types, nil
}

// scalarLibraryType is the ScalarValues type a literal value is of, for a scope
// that does not import the library under the name the value's type is written by.
func (ec *EvalContext) scalarLibraryType(value Value) *symbols.Symbol {
	var prim semantics.PrimType
	switch value.Kind {
	case ValString:
		prim = semantics.PrimString
	case ValComplex:
		prim = semantics.PrimComplex
	case ValConst:
		switch value.Const.Kind {
		case semantics.ValInt:
			prim = semantics.PrimInteger
		case semantics.ValReal:
			prim = semantics.PrimReal
		case semantics.ValBool:
			prim = semantics.PrimBoolean
		default:
			return nil
		}
	default:
		return nil
	}
	return ec.ctx.model.ScalarSymbol(prim)
}

// positiveValue reports whether a numeric constant is greater than zero.
func positiveValue(value Value) bool {
	switch value.Const.Kind {
	case semantics.ValInt:
		return value.Const.Int > 0
	case semantics.ValReal:
		return value.Const.Real > 0
	}
	return false
}

// castNarrowerKeeps decides a target narrower than every type the value is of,
// which only the value itself settles: a scalar by its own magnitude against the
// ScalarValues lattice, a quantity by the dimension the target fixes, an object
// and an enumeration literal by the types they carry — which already said no.
func (ec *EvalContext) castNarrowerKeeps(value Value, target *symbols.Symbol) (bool, error) {
	switch value.Kind {
	case ValConst, ValComplex, ValString:
		prim, ok := ec.ctx.model.ScalarLatticeElement(target)
		got := valuePrimType(&value)
		if !ok || got == semantics.PrimUnknown {
			return false, ec.undecidedCast(value, target)
		}
		if !semantics.PrimConforms(got, prim) {
			return false, nil
		}
		// Positive shares Natural's lattice element but not its zero.
		return !ec.ctx.model.PositiveScalar(target) || positiveValue(value), nil
	case ValQuantity:
		return ec.quantityCastKeeps(value, target)
	case ValEnumLiteral, ValVariant:
		return false, nil
	}
	if _, isObject := value.Object(); isObject {
		// An object is classified by the types it was declared and classified by;
		// a specialization of those does not classify it.
		return false, nil
	}
	return false, ec.undecidedCast(value, target)
}

// quantityCastKeeps judges a quantity against a narrower target by dimension:
// commensurable quantities are values of the same quantity type. A target fixing
// no dimension, or a unit reducing to none, leaves the question undecided.
func (ec *EvalContext) quantityCastKeeps(value Value, target *symbols.Symbol) (bool, error) {
	want, ok := ec.ctx.model.DimensionOfType(target)
	if !ok || value.Quantity() == nil {
		return false, ec.undecidedCast(value, target)
	}
	got, ok := ec.ctx.model.DimensionOfUnit(value.Quantity().Unit.Term)
	if !ok {
		return false, ec.undecidedCast(value, target)
	}
	return want.Term.Commensurable(got.Term), nil
}

// undecidedCast reports a cast whose verdict the value does not settle, so the
// cast fails rather than dropping a value that may well be one of the target's.
func (ec *EvalContext) undecidedCast(value Value, target *symbols.Symbol) error {
	return fmt.Errorf("%w: whether %s (%s) is a value of %s is not stated by the value",
		ErrUndecidedClassification, FormatValue(value), describeValue(value), symbolText(target))
}
