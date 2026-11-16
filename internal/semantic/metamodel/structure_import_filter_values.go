package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// filterValueKind is the kind of value a filter condition's subexpression has for one
// candidate element.
type filterValueKind uint8

const (
	// filterValueEmpty is no value at all: a feature read from an annotation the
	// candidate does not carry, which an operator over it propagates.
	filterValueEmpty filterValueKind = iota
	filterValueBoolean
	filterValueInteger
	filterValueReal
	filterValueString
	// filterValueElement is an element compared by identity, an enumeration literal.
	filterValueElement
)

// filterValue is the value of a model-level evaluable expression in a filter condition
// (KerML 8.4.4.9) for one candidate.
type filterValue struct {
	kind    filterValueKind
	boolean bool
	integer int64
	real    float64
	text    string
	element ElementKey
}

func filterBoolean(value bool) filterValue {
	return filterValue{kind: filterValueBoolean, boolean: value}
}

// structureFilterBoolean evaluates an operand a condition or Boolean operator needs: a
// Boolean, or nothing at all.
func (e *Evaluator) structureFilterBoolean(condition, element Element, metadata []Element) (filterValue, bool) {
	value, ok := e.structureFilterValue(condition, element, metadata)
	if !ok || (value.kind != filterValueBoolean && value.kind != filterValueEmpty) {
		return filterValue{}, false
	}
	return value, true
}

// structureFilterValue evaluates condition for element (KerML 8.3.4.12.2 includeAsMember).
// A form it does not cover is unknown, not false, so the filtered import is withheld.
func (e *Evaluator) structureFilterValue(condition, element Element, metadata []Element) (filterValue, bool) {
	class, known := e.options.Structure.Metaclass(condition)
	if !known {
		return filterValue{}, false
	}
	switch class {
	case "LiteralBoolean", "LiteralInteger", "LiteralRational", "LiteralString":
		return e.structureLiteralValue(condition)
	case "FeatureReferenceExpression":
		referent, handled, ok := e.structureFeatureReferenceReferent(condition)
		if !handled || !ok || referent.Kind != ElementValue {
			return filterValue{}, false
		}
		return e.structureFilterReference(referent.Element, element, metadata)
	case "FeatureChainExpression":
		return e.structureFilterCastRead(condition, element, metadata)
	case "OperatorExpression":
		return e.structureFilterOperator(condition, element, metadata)
	}
	return filterValue{}, false
}

// structureLiteralValue is the value a literal expression states.
func (e *Evaluator) structureLiteralValue(literal Element) (filterValue, bool) {
	value, ok := e.options.Structure.Attribute(literal, "value")
	if !ok {
		return filterValue{}, false
	}
	switch value.Kind {
	case BooleanValue:
		return filterBoolean(value.Boolean), true
	case IntegerValue:
		return filterValue{kind: filterValueInteger, integer: value.Integer}, true
	case RealValue:
		return filterValue{kind: filterValueReal, real: value.Real}, true
	case StringValue:
		return filterValue{kind: filterValueString, text: value.String}, true
	}
	return filterValue{}, false
}

// structureFilterOperator evaluates the classification operator, a Boolean operator or
// a comparison.
func (e *Evaluator) structureFilterOperator(condition, element Element, metadata []Element) (filterValue, bool) {
	operator, ok := e.options.Structure.Attribute(condition, "operator")
	if !ok || operator.Kind != StringValue {
		return filterValue{}, false
	}
	arguments, ok := e.structureOperandExpressions(condition)
	if !ok {
		return filterValue{}, false
	}
	switch operator.String {
	case "@":
		if len(arguments) != 0 {
			return filterValue{}, false
		}
		classified, ok := e.structureMetadataClassified(condition, element, metadata)
		return filterBoolean(classified), ok
	case "not":
		if len(arguments) != 1 {
			return filterValue{}, false
		}
		value, ok := e.structureFilterBoolean(arguments[0], element, metadata)
		if !ok || value.kind == filterValueEmpty {
			return value, ok
		}
		return filterBoolean(!value.boolean), true
	}
	if len(arguments) != 2 {
		return filterValue{}, false
	}
	switch operator.String {
	case "and", "&", "or", "|", "xor", "implies":
		return e.structureFilterConnective(operator.String, arguments, element, metadata)
	case "==", "!=", "<", "<=", ">", ">=":
		left, ok := e.structureFilterValue(arguments[0], element, metadata)
		if !ok {
			return filterValue{}, false
		}
		right, ok := e.structureFilterValue(arguments[1], element, metadata)
		if !ok {
			return filterValue{}, false
		}
		return compareFilterValues(operator.String, left, right)
	}
	return filterValue{}, false
}

// structureFilterConnective applies a binary Boolean operator, settled by its left operand
// where that decides it; an operand with no value leaves the result with none.
func (e *Evaluator) structureFilterConnective(operator string, arguments []Element, element Element, metadata []Element) (filterValue, bool) {
	left, ok := e.structureFilterBoolean(arguments[0], element, metadata)
	if !ok || left.kind == filterValueEmpty {
		return left, ok
	}
	switch {
	case (operator == "and" || operator == "&") && !left.boolean:
		return filterBoolean(false), true
	case (operator == "or" || operator == "|") && left.boolean:
		return filterBoolean(true), true
	case operator == "implies" && !left.boolean:
		return filterBoolean(true), true
	}
	right, ok := e.structureFilterBoolean(arguments[1], element, metadata)
	if !ok || right.kind == filterValueEmpty {
		return right, ok
	}
	if operator == "xor" {
		return filterBoolean(left.boolean != right.boolean), true
	}
	return right, true
}

// compareFilterValues orders numbers and equates values of one kind. Nothing equals only
// nothing, and an ordering over nothing has no value.
func compareFilterValues(operator string, left, right filterValue) (filterValue, bool) {
	equality := operator == "==" || operator == "!="
	if left.kind == filterValueEmpty || right.kind == filterValueEmpty {
		if !equality {
			return filterValue{}, true
		}
		return filterBoolean((left.kind == right.kind) == (operator == "==")), true
	}
	numeric := func(value filterValue) bool {
		return value.kind == filterValueInteger || value.kind == filterValueReal
	}
	if !numeric(left) || !numeric(right) {
		if !equality || left.kind != right.kind {
			return filterValue{}, false
		}
		return filterBoolean((left == right) == (operator == "==")), true
	}
	order := 0
	if left.kind == filterValueInteger && right.kind == filterValueInteger {
		switch {
		case left.integer < right.integer:
			order = -1
		case left.integer > right.integer:
			order = 1
		}
	} else {
		l, r := left.real, right.real
		if left.kind == filterValueInteger {
			l = float64(left.integer)
		}
		if right.kind == filterValueInteger {
			r = float64(right.integer)
		}
		switch {
		case l < r:
			order = -1
		case l > r:
			order = 1
		case l != r: // a NaN orders with nothing
			return filterValue{}, false
		}
	}
	switch operator {
	case "==":
		return filterBoolean(order == 0), true
	case "!=":
		return filterBoolean(order != 0), true
	case "<":
		return filterBoolean(order < 0), true
	case "<=":
		return filterBoolean(order <= 0), true
	case ">":
		return filterBoolean(order > 0), true
	}
	return filterBoolean(order >= 0), true
}

// structureTypeArgument is the one type a classification or cast operator names, which
// the element form states as the typing of the operator's parameter.
func (e *Evaluator) structureTypeArgument(operator Element) (Element, bool) {
	parameters, handled, ok := e.structureOwnedFeatures(operator)
	if !handled || !ok {
		return Element{}, false
	}
	var tested []Element
	for _, parameter := range parameters.Values {
		if parameter.Kind != ElementValue {
			return Element{}, false
		}
		types, ok := e.structureSpecializedGeneralTypes(parameter.Element)
		if !ok {
			return Element{}, false
		}
		tested = append(tested, types...)
	}
	if len(tested) != 1 {
		return Element{}, false
	}
	return tested[0], true
}

// reflectiveMetaclass is the library symbol of element when it is a metaclass of the
// abstract syntax, which the standard library declares under KerML and SysML.
func (e *Evaluator) reflectiveMetaclass(element Element) *symbols.Symbol {
	symbol := e.structureLibrarySymbol(element)
	if symbol == nil {
		return nil
	}
	root := symbol
	for owner := root.Owner(); owner != nil; owner = root.Owner() {
		root = owner
	}
	if root == symbol || (root.Name != "KerML" && root.Name != "SysML") {
		return nil
	}
	return symbol
}

// structureFilterOwningType is the type that owns feature, in the structure or in the
// library.
func (e *Evaluator) structureFilterOwningType(feature Element) (Element, bool) {
	if symbol := e.structureLibrarySymbol(feature); symbol != nil {
		owner := symbol.Owner()
		if owner == nil {
			return Element{}, false
		}
		return ElementOf(owner), true
	}
	if !e.structureKnows(feature) {
		return Element{}, false
	}
	owner, handled, ok := e.structureOwningType(feature)
	if !handled || !ok || owner.Kind != ElementValue {
		return Element{}, false
	}
	return owner.Element, true
}

// structureFilterReference evaluates a name used as a value: an enumeration literal, by
// identity, or a feature of a metadata type or metaclass, read from the candidate.
func (e *Evaluator) structureFilterReference(referent, element Element, metadata []Element) (filterValue, bool) {
	if isLiteral, known := e.isMetaclass(referent, "EnumerationUsage"); !known {
		return filterValue{}, false
	} else if isLiteral {
		return filterValue{kind: filterValueElement, element: e.structureFeatureKey(referent)}, true
	}
	owner, ok := e.structureFilterOwningType(referent)
	if !ok {
		return filterValue{}, false
	}
	if isMetadataType, known := e.isMetaclass(owner, "Metaclass"); !known || !isMetadataType {
		return filterValue{}, false
	}
	return e.structureFilterRead(owner, referent, element, metadata)
}

// structureFilterCastRead evaluates `(as T).f`: f read from the candidate's metadata of
// type T, the form KerML 7.4.9.2 gives for reading an annotation's feature.
func (e *Evaluator) structureFilterCastRead(chain, element Element, metadata []Element) (filterValue, bool) {
	operands, ok := e.structureOperandExpressions(chain)
	if !ok || len(operands) != 1 {
		return filterValue{}, false
	}
	cast := operands[0]
	if class, known := e.options.Structure.Metaclass(cast); !known || class != "OperatorExpression" {
		return filterValue{}, false
	}
	operator, ok := e.options.Structure.Attribute(cast, "operator")
	if !ok || operator.Kind != StringValue || operator.String != "as" {
		return filterValue{}, false
	}
	if castOperands, ok := e.structureOperandExpressions(cast); !ok || len(castOperands) != 0 {
		return filterValue{}, false
	}
	typ, ok := e.structureTypeArgument(cast)
	if !ok {
		return filterValue{}, false
	}
	target, handled, ok := e.structureFeatureReferenceReferent(chain)
	if !handled || !ok || target.Kind != ElementValue {
		return filterValue{}, false
	}
	if e.structureLibrarySymbol(target.Element) == nil {
		// A target the expression owns is a feature chain, which reads further than
		// the annotation's own feature.
		chainings, handled, ok := e.structureOwnedFilter(target.Element, "FeatureChaining", false)
		if !handled || !ok || len(chainings.Values) != 0 {
			return filterValue{}, false
		}
	}
	return e.structureFilterRead(typ, target.Element, element, metadata)
}

// structureFilterRead reads feature of typ from the candidate's metadata typed by typ, else
// from the candidate itself when typ is a metaclass it instantiates; else it has no value.
func (e *Evaluator) structureFilterRead(typ, feature, element Element, metadata []Element) (filterValue, bool) {
	value, ok := e.structureAnnotationValue(typ, feature, metadata)
	if !ok || value.kind != filterValueEmpty {
		return value, ok
	}
	metaclass := e.reflectiveMetaclass(typ)
	if metaclass == nil {
		return filterValue{}, true
	}
	if instance, known := e.isMetaclass(element, metaclass.Name); !known {
		return filterValue{}, false
	} else if !instance {
		return filterValue{}, true
	}
	symbol := e.structureLibrarySymbol(feature)
	if symbol == nil || symbol.Owner() == nil || e.reflectiveMetaclass(ElementOf(symbol.Owner())) == nil {
		return filterValue{}, false
	}
	// The metaclass states the feature as an attribute of the element or derives it.
	if e.structureKnows(element) {
		if attribute, ok := e.options.Structure.Attribute(element, symbol.Name); ok {
			return propertyFilterValue(attribute)
		}
	}
	property, ok := e.Property(element, symbol.Owner().Name, symbol.Name)
	if !ok {
		return filterValue{}, false
	}
	return propertyFilterValue(property)
}

// propertyFilterValue is a metaclass property's value as a filter condition compares it.
func propertyFilterValue(property Value) (filterValue, bool) {
	switch property.Kind {
	case NullValue:
		return filterValue{}, true
	case BooleanValue:
		return filterBoolean(property.Boolean), true
	case IntegerValue:
		return filterValue{kind: filterValueInteger, integer: property.Integer}, true
	case RealValue:
		return filterValue{kind: filterValueReal, real: property.Real}, true
	case StringValue:
		return filterValue{kind: filterValueString, text: property.String}, true
	case SequenceValue:
		switch len(property.Values) {
		case 0:
			return filterValue{}, true
		case 1:
			return propertyFilterValue(property.Values[0])
		}
	}
	return filterValue{}, false
}

// structureAnnotationValue reads feature from the first metadata conforming to typ that
// gives it a value: the one its redefinition is bound to, else its declared default.
func (e *Evaluator) structureAnnotationValue(typ, feature Element, metadata []Element) (filterValue, bool) {
	typeKey := e.structureFeatureKey(typ)
	target := e.structureFeatureKey(feature)
	for _, annotation := range metadata {
		conforms, ok := e.structureMetadataConforms(annotation, typeKey)
		if !ok {
			return filterValue{}, false
		}
		if !conforms {
			continue
		}
		owned, handled, ok := e.structureOwnedFeatures(annotation)
		if !handled || !ok {
			return filterValue{}, false
		}
		bound := feature
		for _, candidate := range owned.Values {
			if candidate.Kind != ElementValue {
				return filterValue{}, false
			}
			redefines, ok := e.structureRedefines(candidate.Element, target, map[ElementKey]bool{})
			if !ok {
				return filterValue{}, false
			}
			if redefines {
				bound = candidate.Element
				break
			}
		}
		if e.structureFeatureKey(bound) == target {
			if bound, ok = e.structureInheritedBinding(annotation, feature, target); !ok {
				return filterValue{}, false
			}
		}
		value, found, ok := e.structureBoundValue(bound)
		if !ok {
			return filterValue{}, false
		}
		if found {
			return value, true
		}
	}
	return filterValue{}, true
}

// structureInheritedBinding is the feature the types of a metadata annotation bind feature
// through: the redefinition of it they inherit that no other one redefines, else feature itself.
func (e *Evaluator) structureInheritedBinding(annotation, feature Element, target ElementKey) (Element, bool) {
	types, handled, ok := e.structureFeatureTypes(annotation)
	if !handled || !ok {
		return Element{}, false
	}
	var pending []Element
	for _, value := range types.Values {
		if value.Kind != ElementValue {
			return Element{}, false
		}
		pending = append(pending, value.Element)
	}
	var candidates []Element
	visited := map[ElementKey]bool{}
	for len(pending) > 0 {
		current := pending[0]
		pending = pending[1:]
		key := e.structureFeatureKey(current)
		if visited[key] || e.structureLibrarySymbol(current) != nil {
			continue
		}
		visited[key] = true
		owned, handled, ok := e.structureOwnedFeatures(current)
		if !handled || !ok {
			return Element{}, false
		}
		for _, candidate := range owned.Values {
			if candidate.Kind != ElementValue {
				return Element{}, false
			}
			redefines, ok := e.structureRedefines(candidate.Element, target, map[ElementKey]bool{})
			if !ok {
				return Element{}, false
			}
			if redefines {
				candidates = append(candidates, candidate.Element)
			}
		}
		bases, ok := e.structureOwnedSupertypes(current)
		if !ok {
			return Element{}, false
		}
		pending = append(pending, bases...)
	}
	bound, found := feature, false
	for at, candidate := range candidates {
		key := e.structureFeatureKey(candidate)
		redefined := false
		for other, overriding := range candidates {
			if other == at {
				continue
			}
			var ok bool
			if redefined, ok = e.structureRedefines(overriding, key, map[ElementKey]bool{}); !ok {
				return Element{}, false
			}
			if redefined {
				break
			}
		}
		if redefined {
			continue
		}
		if found {
			// Two unrelated redefinitions reach the annotation: the model does not say which binds.
			return Element{}, false
		}
		bound, found = candidate, true
	}
	return bound, true
}

// structureRedefines reports whether feature redefines target, directly or through the
// features it redefines.
func (e *Evaluator) structureRedefines(feature Element, target ElementKey, visited map[ElementKey]bool) (bool, bool) {
	key := e.structureFeatureKey(feature)
	if visited[key] || e.structureLibrarySymbol(feature) != nil {
		return false, true
	}
	visited[key] = true
	redefinitions, handled, ok := e.structureOwnedFilter(feature, "Redefinition", false)
	if !handled || !ok {
		return false, false
	}
	for _, redefinition := range redefinitions.Values {
		redefined, ok := e.relationshipEndpoints(redefinition.Element, "redefinedFeature")
		if !ok {
			return false, false
		}
		for _, value := range redefined {
			if value.Kind != ElementValue {
				return false, false
			}
			if e.structureFeatureKey(value.Element) == target {
				return true, true
			}
			redefines, ok := e.structureRedefines(value.Element, target, visited)
			if !ok || redefines {
				return redefines, ok
			}
		}
	}
	return false, true
}

// structureBoundValue is the constant feature is bound to, and whether it is bound: a
// literal or an enumeration literal. A library feature's value is outside the structure.
func (e *Evaluator) structureBoundValue(feature Element) (filterValue, bool, bool) {
	if symbol := e.structureLibrarySymbol(feature); symbol != nil {
		usage, ok := symbol.Decl.(*ast.Usage)
		return filterValue{}, false, ok && usage.Value == nil
	}
	values, handled, ok := e.structureOwnedFilter(feature, "FeatureValue", true)
	if !handled || !ok || len(values.Values) > 1 {
		return filterValue{}, false, false
	}
	if len(values.Values) == 0 {
		return filterValue{}, false, true
	}
	expressions, ok := e.structureMembershipMembers(Element{Membership: values.Values[0].Membership, IsMembership: true})
	if !ok || len(expressions) != 1 {
		return filterValue{}, false, false
	}
	class, known := e.options.Structure.Metaclass(expressions[0])
	if !known {
		return filterValue{}, false, false
	}
	switch class {
	case "LiteralBoolean", "LiteralInteger", "LiteralRational", "LiteralString":
		value, ok := e.structureLiteralValue(expressions[0])
		return value, ok, ok
	case "FeatureReferenceExpression":
		referent, handled, ok := e.structureFeatureReferenceReferent(expressions[0])
		if !handled || !ok || referent.Kind != ElementValue {
			return filterValue{}, false, false
		}
		if isLiteral, known := e.isMetaclass(referent.Element, "EnumerationUsage"); !known || !isLiteral {
			return filterValue{}, false, false
		}
		return filterValue{kind: filterValueElement, element: e.structureFeatureKey(referent.Element)}, true, true
	}
	return filterValue{}, false, false
}
