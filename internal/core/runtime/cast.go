package runtime

import (
	"fmt"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// evaluationTypeFQN types a deferred expression: the functions computing a result.
const evaluationTypeFQN = "Performances::Evaluation"

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
	// A written sequence is cast entry by entry, each judged by its own types.
	if _, ok := n.Operands[0].(*ast.SequenceExpr); ok {
		kept, sources, err := ec.castEntries(n.Operands[0], target)
		if err != nil {
			return Value{}, err
		}
		return ec.sequenceFrom(kept, sources...)
	}
	value, err := ec.Eval(n.Operands[0])
	if err != nil {
		return Value{}, err
	}
	return ec.castValue(value, target, ec.declaredCastTypes(n.Operands[0]))
}

// castEntries casts one entry of a written sequence, answering the values target
// keeps of it and the values it holds, whose unit an empty result keeps. A KerML
// sequence is flat, so a nested one contributes its own entries.
func (ec *EvalContext) castEntries(
	entry ast.Node, target *symbols.Symbol,
) (kept, sources []Value, err error) {
	if seq, ok := entry.(*ast.SequenceExpr); ok {
		for _, element := range seq.Elements {
			elementKept, elementSources, err := ec.castEntries(element, target)
			if err != nil {
				return nil, nil, err
			}
			kept = append(kept, elementKept...)
			sources = append(sources, elementSources...)
		}
		return kept, sources, nil
	}
	value, err := ec.Eval(entry)
	if err != nil {
		return nil, nil, err
	}
	out, err := ec.castValue(value, target, ec.declaredCastTypes(entry))
	if err != nil {
		return nil, nil, err
	}
	return elementsOf(out), []Value{value}, nil
}

// declaredCastTypes names every type the cast's operand is declared with, which
// classifies its values where their own content does not state their type.
func (ec *EvalContext) declaredCastTypes(operand ast.Node) []*symbols.Symbol {
	// An enumeration literal is of its enumeration however its value is written.
	if sym, ok := ec.ctx.model.resolver.ResolveTarget(ec.scope, operand); ok && sym != nil {
		if canonical, aliased := ec.ctx.model.resolver.ResolveAliasTarget(sym); aliased {
			sym = canonical
		}
		if enum := semantics.EnumerationOwning(sym); enum != nil {
			return []*symbols.Symbol{enum}
		}
	}
	var declared []*symbols.Symbol
	for _, typ := range ec.ctx.model.semantics.ExprResultTypes(ec.scope, operand) {
		// A feature typed Anything states nothing about the values it holds.
		if typ != nil && !semantics.IsAnything(typ) {
			declared = append(declared, typ)
		}
	}
	return declared
}

// castValue keeps the values of value that target classifies: element-wise and in
// order for a collection, the value itself or the empty sequence for one value.
func (ec *EvalContext) castValue(
	value Value, target *symbols.Symbol, declared []*symbols.Symbol,
) (Value, error) {
	switch value.Kind {
	case ValNull, ValInvalid:
		return sequenceOf(nil), nil
	case ValSequence, ValSet:
		elements := elementsOf(value)
		kept := make([]Value, 0, len(elements))
		for _, element := range elements {
			keep, err := ec.castKeeps(element, target, declared)
			if err != nil {
				return Value{}, err
			}
			if keep {
				kept = append(kept, element)
			}
		}
		if value.Kind == ValSet {
			set := NewSetIn(ec.ctx)
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
	keep, err := ec.castKeeps(value, target, declared)
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
func (ec *EvalContext) castKeeps(
	value Value, target *symbols.Symbol, declared []*symbols.Symbol,
) (bool, error) {
	return ec.castKeepsReading(value, target, declared, nil)
}

// castKeepsReading answers castKeeps; reading holds the composed targets being
// read, so a composition naming itself does not recur.
func (ec *EvalContext) castKeepsReading(
	value Value, target *symbols.Symbol, declared []*symbols.Symbol, reading map[*symbols.Symbol]bool,
) (bool, error) {
	types, err := ec.castTypes(value)
	if err != nil && len(declared) == 0 {
		return false, err
	}
	known := append(append([]*symbols.Symbol{}, declared...), types...)
	switch ec.ctx.model.semantics.ClassifiesTypes(known, target) {
	case semantics.ClassifiesAll:
		return true, nil
	case semantics.ClassifiesNone:
		return false, nil
	}
	if keep, composed, err := ec.castComposedKeeps(value, target, declared, reading); composed {
		return keep, err
	}
	return ec.castNarrowerKeeps(value, target)
}

// castComposedKeeps decides a value by a composed target's operands — any of a
// union, all of an intersection, the first of a difference and none of the rest —
// and reports second whether the target is composed at all.
func (ec *EvalContext) castComposedKeeps(
	value Value, target *symbols.Symbol, declared []*symbols.Symbol, reading map[*symbols.Symbol]bool,
) (bool, bool, error) {
	unions := ec.ctx.model.semantics.UnioningTypes(target)
	intersects := ec.ctx.model.semantics.IntersectingTypes(target)
	differences := ec.ctx.model.semantics.DifferencingTypes(target)
	if len(unions)+len(intersects)+len(differences) == 0 || reading[target] {
		return false, false, nil
	}
	if reading == nil {
		reading = make(map[*symbols.Symbol]bool)
	}
	reading[target] = true
	defer delete(reading, target)

	keeps := func(operand *symbols.Symbol) (bool, error) {
		return ec.castKeepsReading(value, operand, declared, reading)
	}
	// An operand no type of the value settles leaves the cast undecided, but only
	// where no other operand excludes the value outright.
	var undecided error
	if len(unions) > 0 {
		kept, err := anyKeeps(unions, keeps)
		switch {
		case err != nil:
			undecided = err
		case !kept:
			return false, true, nil
		}
	}
	for _, operand := range intersects {
		kept, err := keeps(operand)
		switch {
		case err != nil:
			undecided = err
		case !kept:
			return false, true, nil
		}
	}
	for i, operand := range differences {
		kept, err := keeps(operand)
		switch {
		case err != nil:
			undecided = err
		case kept != (i == 0):
			return false, true, nil
		}
	}
	if undecided != nil {
		return false, true, undecided
	}
	return true, true, nil
}

// anyKeeps reports whether any operand keeps the value, reporting an operand's
// error only when no other one keeps it.
func anyKeeps(operands []*symbols.Symbol, keeps func(*symbols.Symbol) (bool, error)) (bool, error) {
	var undecided error
	for _, operand := range operands {
		kept, err := keeps(operand)
		if err != nil {
			undecided = err
			continue
		}
		if kept {
			return true, nil
		}
	}
	return false, undecided
}

// castTypes names the types a value is of for a cast: a quantity value is the
// quantity type it is, whose dimension castNarrowerKeeps then judges, and every
// other value is of the types a classification reads it as.
func (ec *EvalContext) castTypes(value Value) ([]*symbols.Symbol, error) {
	// A deferred expression is of the evaluation type the model reads it as, in
	// the scope it closes over.
	if value.Kind == ValExpr {
		if typ := ec.ctx.model.semantics.ExprResultType(value.exprEnv(ec).scope, value.Expr()); typ != nil {
			return []*symbols.Symbol{typ}, nil
		}
		evaluation, err := ec.ctx.loadedLibraryType(evaluationTypeFQN)
		if err != nil {
			return nil, err
		}
		return []*symbols.Symbol{evaluation}, nil
	}
	if value.Kind == ValQuantity {
		quantity, err := ec.ctx.loadedLibraryType(scalarQuantityTypeFQN)
		if err != nil {
			return nil, err
		}
		return []*symbols.Symbol{quantity}, nil
	}
	// A scalar is of its ScalarValues type whatever a declaration of that name in
	// the reading scope says, so the library symbol answers ahead of a lookup.
	if scalar := ec.scalarLibraryType(value); scalar != nil {
		return []*symbols.Symbol{scalar}, nil
	}
	return ec.ctx.directValueTypes(ec.scope, value)
}

// scalarLibraryType is the ScalarValues type a literal value is of, independent of
// what the reading scope imports or declares under that type's name.
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
	return ec.ctx.model.semantics.ScalarSymbol(prim)
}

// positiveValue reports whether a numeric value is greater than zero; a complex
// value off the real axis is not on the ordering Positive bounds.
func positiveValue(value Value) bool {
	if value.Kind == ValComplex {
		return imag(value.Complex()) == 0 && real(value.Complex()) > 0
	}
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
		prim, ok := ec.ctx.model.semantics.ScalarLatticeElement(target)
		got := valuePrimType(&value)
		if !ok || got == semantics.PrimUnknown {
			return false, ec.undecidedCast(value, target)
		}
		if !semantics.PrimConforms(got, prim) {
			return false, nil
		}
		// Positive shares Natural's lattice element but not its zero.
		return !ec.ctx.model.semantics.PositiveScalar(target) || positiveValue(value), nil
	case ValQuantity:
		return ec.quantityCastKeeps(value, target)
	case ValArray, ValVector, ValVectorQuantity, ValTensorQuantity,
		ValMeasurementRef, ValCoordinateFrame, ValCoordinateTransformation:
		// A structured value's own shape, units and frame decide it, as they do
		// for a value written to a feature of the target type.
		keep, _, err := ec.ctx.valueConforms(ec.scope, &value, target, admitWritten)
		return keep, err
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

// quantityCastKeeps judges a quantity against a narrower target by dimension: an
// incommensurable target measures none of its values, and a target stating its own
// measurement reference measures every value of that dimension. Anything else a
// magnitude and a unit do not state, so it is undecided.
func (ec *EvalContext) quantityCastKeeps(value Value, target *symbols.Symbol) (bool, error) {
	want, ok := ec.ctx.model.semantics.DimensionOfType(target)
	if !ok || value.Quantity() == nil {
		return false, ec.undecidedCast(value, target)
	}
	got, ok := ec.ctx.model.semantics.DimensionOfUnit(value.Quantity().Unit.Term)
	if !ok {
		return false, ec.undecidedCast(value, target)
	}
	if !want.Term.Commensurable(got.Term) {
		return false, nil
	}
	if !ec.ctx.model.semantics.FixesMeasurementReference(target) {
		return false, ec.undecidedCast(value, target)
	}
	return true, nil
}

// undecidedCast reports a cast whose verdict the value does not settle, so the
// cast fails rather than dropping a value that may well be one of the target's.
func (ec *EvalContext) undecidedCast(value Value, target *symbols.Symbol) error {
	return fmt.Errorf("%w: whether %s (%s) is a value of %s is not stated by the value",
		ErrUndecidedClassification, FormatValue(value), describeValue(value), symbolText(target))
}
