package metamodel

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// structureExpressionModelLevelEvaluable derives Expression::isModelLevelEvaluable as
// modelLevelEvaluable(Set{}) over the element's structure (KerML 8.3.4.7.3).
func (e *Evaluator) structureExpressionModelLevelEvaluable(element Element) (Value, bool, bool) {
	evaluable, ok := e.structureModelLevelEvaluable(element, &modelLevelWalk{visited: map[ElementKey]bool{}, settled: map[ElementKey]bool{}})
	if !ok {
		e.recordUnsupportedReason(element, "Expression", "isModelLevelEvaluable", Reason{
			Category: ReasonIncompleteStructure,
			Detail:   "Expression::isModelLevelEvaluable reads an argument, referent, function or parameter the structure does not settle",
		})
		return Value{}, true, false
	}
	return booleanValue(evaluable), true, true
}

// structureFunctionModelLevelEvaluable derives Function::isModelLevelEvaluable: true for the
// Kernel Function Library functions a model-level evaluation may call (KerML 8.3.4.7.4).
func (e *Evaluator) structureFunctionModelLevelEvaluable(element Element) (Value, bool, bool) {
	// A library function is read through its symbol, as the expressions that call it hold it.
	if symbol := element.Symbol; symbol != nil && e.model != nil && e.resolver != nil &&
		e.resolver.Index() != nil && e.resolver.Index().Library(symbol) {
		return booleanValue(semantics.ModelLevelFunction(e.model.QualifiedNameOf(symbol))), true, true
	}
	library, ok := e.Property(element, "Element", "isLibraryElement")
	if !ok || library.Kind != BooleanValue {
		return Value{}, true, false
	}
	if !library.Boolean {
		return booleanValue(false), true, true
	}
	name, ok := e.Property(element, "Element", "qualifiedName")
	if !ok || name.Kind != StringValue {
		return Value{}, true, false
	}
	return booleanValue(semantics.ModelLevelFunction(name.String)), true, true
}

// modelLevelWalk is one evaluation of modelLevelEvaluable: the features on the current path,
// and the answers of the referents already walked, so a shared referent is walked once.
type modelLevelWalk struct {
	visited map[ElementKey]bool
	settled map[ElementKey]bool
	cycles  int
}

// structureModelLevelEvaluable is Expression::modelLevelEvaluable(visited) and its
// redefinitions; it reports false when a value the operation reads is not settled.
func (e *Evaluator) structureModelLevelEvaluable(expression Element, walk *modelLevelWalk) (evaluable, ok bool) {
	is := func(metaclass string) bool {
		matches, known := e.isMetaclass(expression, metaclass)
		ok = ok && known
		return matches
	}
	ok = true
	switch {
	case is("LiteralExpression"), is("NullExpression"), is("MetadataAccessExpression"):
		return true, ok
	// SysML 8.3.19.3 CalculationUsage and 8.3.20.4 ConstraintUsage redefine the operation as false.
	case is("CalculationUsage"), is("ConstraintUsage"):
		return false, ok
	case is("FeatureReferenceExpression"):
		if !ok {
			return false, false
		}
		return e.structureReferenceModelLevelEvaluable(expression, walk)
	case is("ConstructorExpression"):
		if !ok {
			return false, false
		}
		return e.structureArgumentsModelLevelEvaluable(expression, walk)
	case is("InvocationExpression"):
		if !ok {
			return false, false
		}
		evaluable, ok = e.structureArgumentsModelLevelEvaluable(expression, walk)
		if !ok || !evaluable {
			return false, ok
		}
		function, ok := e.Property(expression, "Expression", "function")
		if !ok || function.Kind != ElementValue {
			return false, false
		}
		// Read directly: the structure does not hold a library function for Property to dispatch on.
		functionEvaluable, _, ok := e.structureFunctionModelLevelEvaluable(function.Element)
		if !ok || functionEvaluable.Kind != BooleanValue {
			return false, false
		}
		return functionEvaluable.Boolean, true
	}
	if !ok {
		return false, false
	}
	return e.structureBodyModelLevelEvaluable(expression, walk)
}

func (e *Evaluator) structureArgumentsModelLevelEvaluable(expression Element, walk *modelLevelWalk) (evaluable, ok bool) {
	arguments, ok := e.Property(expression, "InstantiationExpression", "argument")
	if !ok || arguments.Kind != SequenceValue {
		return false, false
	}
	for _, argument := range arguments.Values {
		if argument.Kind != ElementValue {
			return false, false
		}
		if evaluable, ok := e.structureModelLevelEvaluable(argument.Element, walk); !ok || !evaluable {
			return false, ok
		}
	}
	return true, true
}

// structureReferenceModelLevelEvaluable is FeatureReferenceExpression::modelLevelEvaluable (KerML 8.3.4.8.5).
func (e *Evaluator) structureReferenceModelLevelEvaluable(expression Element, walk *modelLevelWalk) (evaluable, ok bool) {
	referent, ok := e.Property(expression, "FeatureReferenceExpression", "referent")
	if !ok || referent.Kind != ElementValue {
		return false, false
	}
	feature := referent.Element
	self, ok := e.structureConformsToSelf(feature, map[ElementKey]bool{})
	if !ok {
		return false, false
	}
	key := e.structureFeatureKey(feature)
	if self {
		return true, true
	}
	if walk.visited[key] {
		walk.cycles++
		return false, true
	}
	if evaluable, settled := walk.settled[key]; settled {
		return evaluable, true
	}
	walk.visited[key] = true
	cycles := walk.cycles
	evaluable, ok = e.structureReferentModelLevelEvaluable(feature, walk)
	delete(walk.visited, key)
	// A false that met a feature on the path holds for this path only.
	if ok && (evaluable || walk.cycles == cycles) {
		walk.settled[key] = evaluable
	}
	return evaluable, ok
}

// structureReferentModelLevelEvaluable is the part of that operation that reads the referent.
func (e *Evaluator) structureReferentModelLevelEvaluable(feature Element, walk *modelLevelWalk) (evaluable, ok bool) {
	isExpression, known := e.isMetaclass(feature, "Expression")
	if !known {
		return false, false
	}
	if isExpression {
		if evaluable, ok := e.structureModelLevelEvaluable(feature, walk); !ok || evaluable {
			return evaluable, ok
		}
	}
	owningType, ok := e.Property(feature, "Feature", "owningType")
	if !ok {
		return false, false
	}
	if owningType.Kind == ElementValue {
		for _, metaclass := range []string{"Metaclass", "MetadataFeature"} {
			matches, known := e.isMetaclass(owningType.Element, metaclass)
			if !known {
				return false, false
			}
			if matches {
				return true, true
			}
		}
	}
	featuringTypes, ok := e.Property(feature, "Feature", "featuringType")
	if !ok || featuringTypes.Kind != SequenceValue {
		return false, false
	}
	if len(featuringTypes.Values) != 0 {
		return false, true
	}
	values, ok := e.structureFeatureValueExpressions(feature)
	if !ok {
		return false, false
	}
	for _, value := range values {
		if value.Kind != ElementValue {
			return false, false
		}
		if evaluable, ok := e.structureModelLevelEvaluable(value.Element, walk); !ok || !evaluable {
			return false, ok
		}
	}
	return true, true
}

// structureConformsToSelf reports whether feature is, or specializes, Base::Anything::self.
// A library element is read through its symbol, whose supertypes the structure does not hold.
func (e *Evaluator) structureConformsToSelf(feature Element, visited map[ElementKey]bool) (conforms, ok bool) {
	if symbol := e.structureElementSymbol(feature); symbol != nil {
		self := e.librarySymbol("Base::Anything::self")
		return self != nil && e.model != nil && e.model.Conforms(symbol, self), true
	}
	key := e.structureFeatureKey(feature)
	if visited[key] {
		return false, true
	}
	visited[key] = true
	generals, ok := e.structureSpecializedGeneralTypes(feature)
	if !ok {
		return false, false
	}
	for _, general := range generals {
		if conforms, ok := e.structureConformsToSelf(general, visited); !ok || conforms {
			return conforms, ok
		}
	}
	return false, true
}

// structureBodyModelLevelEvaluable is Expression::modelLevelEvaluable for an Expression no
// redefinition covers: no stated specialization, and only bare in or result parameters and
// a model-level evaluable result Expression as owned features (KerML 8.3.4.7.3).
func (e *Evaluator) structureBodyModelLevelEvaluable(expression Element, walk *modelLevelWalk) (evaluable, ok bool) {
	specializations, handled, ok := e.structureOwnedFilter(expression, "Specialization", false)
	if !handled || !ok {
		return false, false
	}
	for _, specialization := range specializations.Values {
		if specialization.Kind != ElementValue {
			return false, false
		}
		implied, ok := e.options.Structure.Attribute(specialization.Element, "isImplied")
		if !ok || implied.Kind != BooleanValue {
			return false, false
		}
		if !implied.Boolean {
			return false, true
		}
	}
	result, handled, ok := e.structureResultFeatures(expression, "Expression", "result")
	if !handled || !ok {
		return false, false
	}
	memberships, handled, ok := e.structureOwnedFilter(expression, "FeatureMembership", true)
	if !handled || !ok {
		return false, false
	}
	for _, membership := range memberships.Values {
		if membership.Kind != MembershipValue {
			return false, false
		}
		handle := MembershipElement(membership.Membership)
		resultExpression, known := e.isMetaclass(handle, "ResultExpressionMembership")
		if !known {
			return false, false
		}
		members, ok := e.structureMembershipMembers(handle)
		if !ok {
			return false, false
		}
		for _, feature := range members {
			if resultExpression {
				if evaluable, ok := e.structureModelLevelEvaluable(feature, walk); !ok || !evaluable {
					return false, ok
				}
				continue
			}
			if bare, ok := e.structureBareParameter(expression, feature, result); !ok || !bare {
				return false, ok
			}
		}
	}
	return true, true
}

// structureBareParameter reports whether feature is an in parameter or the result of
// expression, with neither an owned feature nor a FeatureValue.
func (e *Evaluator) structureBareParameter(expression, feature Element, result Value) (bare, ok bool) {
	isResult := result.Kind == ElementValue && sameStructureElement(feature, result.Element, e.options.Structure)
	if !isResult {
		direction, known := e.structureEffectiveDirection(expression, feature)
		if !known || direction != ast.DirIn {
			return false, true
		}
	}
	owned, handled, ok := e.structureOwnedFeatures(feature)
	if !handled || !ok {
		return false, false
	}
	values, ok := e.structureFeatureValueExpressions(feature)
	if !ok {
		return false, false
	}
	return len(owned.Values) == 0 && len(values) == 0, true
}
