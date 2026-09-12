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
	return ec.castValue(soleElement(value), target, ec.declaredOperandTypes(n.Operands[0]))
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
	out, err := ec.castValue(value, target, ec.declaredOperandTypes(entry))
	if err != nil {
		return nil, nil, err
	}
	return elementsOf(out), []Value{value}, nil
}

// declaredOperandTypes names every type an operand is declared with, which classifies
// its values where their own content does not state their type (KerML 1.0 §7.3.4.1).
func (ec *EvalContext) declaredOperandTypes(operand ast.Node) []*symbols.Symbol {
	// An enumeration literal is of its enumeration however its value is written.
	if sym, ok := ec.ctx.resolveTarget(ec.scope, operand); ok && sym != nil {
		if canonical, aliased := ec.ctx.resolveAliasTarget(sym); aliased {
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
			element, keep, err := ec.castKept(element, target, declared)
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
	kept, keep, err := ec.castKept(value, target, declared)
	if err != nil {
		return Value{}, err
	}
	if !keep {
		return ec.sequenceFrom(nil, value)
	}
	return kept, nil
}

// castKept is the value a cast keeps: the value itself, or the enumerated value it
// equals when target is an enumeration (`3 as Level` is `Level::high`).
func (ec *EvalContext) castKept(
	value Value, target *symbols.Symbol, declared []*symbols.Symbol,
) (Value, bool, error) {
	keep, err := ec.castKeeps(value, target, declared)
	if err != nil || !keep {
		return value, keep, err
	}
	enumerated, found, err := ec.ctx.asEnumerated(value, target)
	if err != nil || !found {
		return value, true, err
	}
	return enumerated, true, nil
}

// castKeeps reports whether target classifies one value, by the shared classification;
// a verdict neither the value's types nor its content settle fails the cast (undecidedCast).
func (ec *EvalContext) castKeeps(
	value Value, target *symbols.Symbol, declared []*symbols.Symbol,
) (bool, error) {
	verdict, err := ec.ctx.classifyValue(ec.scope, value, target, declared, byAnyType)
	if err != nil {
		return false, err
	}
	switch verdict {
	case semantics.ClassifiesAll:
		return true, nil
	case semantics.ClassifiesNone:
		return false, nil
	}
	return false, ec.undecidedCast(value, target)
}

// undecidedCast reports a cast whose verdict the value does not settle, so the
// cast fails rather than dropping a value that may well be one of the target's.
func (ec *EvalContext) undecidedCast(value Value, target *symbols.Symbol) error {
	return fmt.Errorf("%w: whether %s (%s) is a value of %s is not stated by the value",
		ErrUndecidedClassification, FormatValue(value), describeValue(value), symbolText(target))
}
