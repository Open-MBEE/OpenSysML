package runtime

import (
	"errors"
	"fmt"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// ErrNoMetaclass is returned when an element's declaration is classified by no
// reflective metaclass the loaded libraries declare, so it has no metaobject.
var ErrNoMetaclass = errors.New("no reflective metaclass classifies the element")

// ErrReflectiveFeatureUnsupported is returned when a metaclass declares the
// feature read from a metaobject but the runtime derives no value for it.
var ErrReflectiveFeatureUnsupported = errors.New("reflective feature is not derived")

// evalMetaCast evaluates `x meta T`, the shorthand for `x.metadata as T` (KerML
// 1.0 §7.4.9.2): of the element x names, the metadata annotations whose type
// conforms to T in model order, then its reflective metaobject when its own
// metaclass conforms to T (§8.3.4.8.15); `()` when nothing does.
func (ec *EvalContext) evalMetaCast(n *ast.OperatorExpr) (Value, error) {
	if len(n.Operands) != 1 || n.TypeRef == nil {
		return Value{}, fmt.Errorf("%w: '%s' requires one element and one type",
			ErrTypeMismatch, n.Operator)
	}
	target, ok := ec.resolveClassificationType(n.TypeRef)
	if !ok {
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedType, qualifiedNameToString(n.TypeRef))
	}
	sym, err := ec.metaCastSubject(n)
	if err != nil {
		return Value{}, err
	}
	sem := ec.ctx.model.semantics
	if sem == nil {
		return Value{}, fmt.Errorf("%w: no model holds the metadata of %s",
			ErrTypeMismatch, ec.ctx.qualifiedSymbolName(sym))
	}
	commit, rollback := ec.ctx.beginJournal()
	var values []Value
	for i, annotation := range sem.ElementMetadataOf(sym) {
		if !sem.Conforms(annotation.Type, target) {
			continue
		}
		val, err := ec.metadataInstance(metadataAnnotation{element: sym, index: i}, annotation)
		if err != nil {
			rollback()
			return Value{}, err
		}
		values = append(values, val)
	}
	if sem.MetaclassConforms(sym, ec.ctx.qualifiedSymbolName(target)) {
		meta, err := ec.reflectiveMetaobject(sym)
		if err != nil {
			rollback()
			return Value{}, err
		}
		values = append(values, meta)
	}
	seq, err := ec.newSequence(values)
	if err != nil {
		rollback()
		return Value{}, err
	}
	commit()
	return seq, nil
}

// metaCastSubject is the element `x meta T` reflects on: the element x names
// (through any alias), else the element x's value denotes.
func (ec *EvalContext) metaCastSubject(n *ast.OperatorExpr) (*symbols.Symbol, error) {
	sym, err := ec.classifiedElement(n)
	if err != nil {
		return nil, err
	}
	if resolved, aliased := ec.ctx.model.resolver.ResolveAliasTarget(sym); aliased {
		sym = resolved
	}
	if sym.Decl == nil {
		return nil, fmt.Errorf("%w: '%s' requires an element, but %s declares none",
			ErrTypeMismatch, n.Operator, ec.ctx.qualifiedSymbolName(sym))
	}
	return sym, nil
}

// reflectiveMetaobject is the metaobject of sym's own metaclass, the one
// `x.metadata` ends with (KerML 1.0 §8.3.4.8.15).
func (ec *EvalContext) reflectiveMetaobject(sym *symbols.Symbol) (Value, error) {
	metaclass := ec.ctx.model.semantics.MetaclassOf(sym)
	if metaclass == nil {
		return Value{}, fmt.Errorf("%w: %s", ErrNoMetaclass, ec.ctx.qualifiedSymbolName(sym))
	}
	return NewMetaobject(sym, metaclass), nil
}

// metaobjectFeature reads a feature of a metaobject: the metaclass's feature of
// that name, derived for the denoted element from the semantic model's tables.
func (ec *EvalContext) metaobjectFeature(meta Value, name string) (Value, error) {
	element, metaclass := meta.MetaobjectElement(), meta.MetaobjectClass()
	sem := ec.ctx.model.semantics
	if element == nil || metaclass == nil || sem == nil {
		return Value{}, fmt.Errorf("%w: %s has no feature %s", ErrTypeMismatch, describeValue(meta), name)
	}
	feature, ok := ec.metaclassFeature(metaclass, name)
	if !ok {
		return Value{}, fmt.Errorf("%w: %s declares no feature %s", ErrNoSuchFeature,
			ec.ctx.qualifiedSymbolName(metaclass), name)
	}
	for _, property := range ec.reflectivePropertyNames(metaclass, feature) {
		if property == "direction" {
			return ec.reflectiveDirection(element, feature)
		}
		if elements, ok := sem.ReflectiveElements(element, property); ok {
			values := make([]Value, 0, len(elements))
			for _, elem := range elements {
				val, err := ec.reflectiveMetaobject(elem)
				if err != nil {
					return Value{}, err
				}
				values = append(values, val)
			}
			return ec.reflectiveResult(element, feature, values)
		}
		if filterValues, ok := sem.ReflectiveFeatureValues(element, property); ok {
			values := make([]Value, 0, len(filterValues))
			for _, fv := range filterValues {
				val, err := ec.valueOfFilterValue(fv)
				if err != nil {
					return Value{}, err
				}
				values = append(values, val)
			}
			return ec.reflectiveResult(element, feature, values)
		}
	}
	return Value{}, fmt.Errorf("%w: %s::%s for %s", ErrReflectiveFeatureUnsupported,
		ec.ctx.qualifiedSymbolName(metaclass), name, ec.ctx.qualifiedSymbolName(element))
}

// reflectiveResult shapes the values derived for feature as a read of any
// feature is: one value under a multiplicity admitting at most one, else a sequence.
func (ec *EvalContext) reflectiveResult(element, feature *symbols.Symbol, values []Value) (Value, error) {
	if !ec.ctx.model.semantics.EffectiveMultiplicityOf(feature).AtMostOne() {
		return ec.newSequence(values)
	}
	switch len(values) {
	case 0:
		return ec.newSequence(nil)
	case 1:
		return values[0], nil
	}
	return Value{}, fmt.Errorf("%w: %s derives %d values for %s, which admits at most one",
		ErrTypeMismatch, ec.ctx.qualifiedSymbolName(element), len(values), ec.ctx.qualifiedSymbolName(feature))
}

// metaclassFeature is the feature named name that metaclass has: declared or
// inherited, or inherited under a redefinition that masks the name (a metaobject
// cast to a supertype is still read by the supertype's names, KerML 8.2.4).
func (ec *EvalContext) metaclassFeature(metaclass *symbols.Symbol, name string) (*symbols.Symbol, bool) {
	sem := ec.ctx.model.semantics
	if feature, ok := sem.LookupMember(metaclass, name); ok && feature.IsFeature() {
		return feature, true
	}
	for _, member := range sem.MembersOfIncludingRedefined(metaclass) {
		if member.IsFeature() && semantics.SimpleSymbolName(member) == name {
			return member, true
		}
	}
	return nil, false
}

// reflectivePropertyNames is the names the semantic model may derive feature's
// value under: its own, then those of the features it redefines and of the
// feature redefining it in metaclass, since a redefinition shares its target's
// value.
func (ec *EvalContext) reflectivePropertyNames(metaclass, feature *symbols.Symbol) []string {
	sem := ec.ctx.model.semantics
	names := []string{semantics.SimpleSymbolName(feature)}
	for _, redefined := range sem.AllRedefinedFeatures(feature) {
		names = append(names, semantics.SimpleSymbolName(redefined))
	}
	for _, member := range sem.MembersOf(metaclass) {
		if member.IsFeature() && slices.Contains(sem.AllRedefinedFeatures(member), feature) {
			names = append(names, semantics.SimpleSymbolName(member))
		}
	}
	return slices.Compact(names)
}

// reflectiveDirection is Feature::direction of element: the FeatureDirectionKind
// member its declaration states, `()` for a feature stating none.
func (ec *EvalContext) reflectiveDirection(element, feature *symbols.Symbol) (Value, error) {
	direction, ok := semantics.ReflectiveDirection(element)
	if !ok {
		return Value{}, fmt.Errorf("%w: %s declares no feature to have a direction",
			ErrTypeMismatch, ec.ctx.qualifiedSymbolName(element))
	}
	if direction == ast.DirNone {
		return ec.newSequence(nil)
	}
	sem := ec.ctx.model.semantics
	for _, kind := range sem.FeatureTypeSet(feature) {
		if literal, ok := sem.LookupMember(kind, direction.String()); ok {
			return NewEnumLiteral(literal), nil
		}
	}
	return Value{}, fmt.Errorf("%w: FeatureDirectionKind declares no member %s",
		ErrUnresolvedReference, direction)
}

// valueOfFilterValue is the runtime value of a constant the semantic model derives.
func (ec *EvalContext) valueOfFilterValue(fv symbols.FilterValue) (Value, error) {
	switch fv.Kind {
	case symbols.FilterValueBool:
		return boolValue(fv.Bool), nil
	case symbols.FilterValueInt:
		return Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: fv.Int}}, nil
	case symbols.FilterValueReal:
		return realConst(fv.Real), nil
	case symbols.FilterValueString:
		return NewStringValue(fv.Str), nil
	case symbols.FilterValueEmpty:
		return ec.newSequence(nil)
	case symbols.FilterValueRef:
		if ref := ec.ctx.resolveType(ec.scope, fv.RefFQN); ref != nil {
			return NewEnumLiteral(ref), nil
		}
		return Value{}, fmt.Errorf("%w: %s", ErrUnresolvedReference, fv.RefFQN)
	}
	return Value{}, fmt.Errorf("%w: a %v constant has no runtime value", ErrTypeMismatch, fv.Kind)
}
