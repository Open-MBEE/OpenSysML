package metamodel

// structureFilterConditions derives Package::filterCondition: the conditions of the
// Package's owned ElementFilterMemberships (KerML 8.3.4.13.4 derivePackageFilterCondition).
func (e *Evaluator) structureFilterConditions(element Element) (Value, bool, bool) {
	memberships, handled, ok := e.structureOwnedFilter(element, "ElementFilterMembership", true)
	if !handled || !ok {
		return Value{}, true, false
	}
	var conditions []Value
	for _, membership := range memberships.Values {
		if membership.Kind != MembershipValue {
			return Value{}, true, false
		}
		members, ok := e.structureMembershipMembers(MembershipElement(membership.Membership))
		if !ok {
			return Value{}, true, false
		}
		for _, member := range members {
			conditions = append(conditions, elementValue(member))
		}
	}
	return sequence(conditions...), true, true
}

// structurePredicate derives BooleanExpression::predicate, which redefines
// Expression::function: the function when it is a Predicate (KerML 8.3.4.7.2).
func (e *Evaluator) structurePredicate(element Element) (Value, bool, bool) {
	function, ok := e.Property(element, "Expression", "function")
	if !ok {
		return Value{}, true, false
	}
	return e.structureValueOfKind(function, "Predicate")
}

// structureMetaclass derives MetadataFeature::metaclass: the first of its types that is
// a Metaclass (KerML deriveMetadataFeatureMetaclass).
func (e *Evaluator) structureMetaclass(element Element) (Value, bool, bool) {
	types, ok := e.Property(element, "Feature", "type")
	if !ok || types.Kind != SequenceValue {
		return Value{}, true, false
	}
	for _, typ := range types.Values {
		value, _, ok := e.structureValueOfKind(typ, "Metaclass")
		if !ok || value.Kind == ElementValue {
			return value, true, ok
		}
	}
	return nullValue(), true, true
}

// structureReferencedTarget derives the usage a referencing usage stands for: itself with no
// owned ReferenceSubsetting, else the featureTarget of the referenced feature, which must be
// of kind (SysML 8.3.9.2 deriveEventOccurrenceUsageEventOccurrence, 8.3.17.14, 8.3.18.2, 8.3.20.2).
func (e *Evaluator) structureReferencedTarget(element Element, class, property, kind string) (Value, bool, bool) {
	target, handled, ok := e.structurePerformedAction(element)
	if !handled || !ok {
		return Value{}, true, false
	}
	target, _, ok = e.structureValueOfKind(target, kind)
	if !ok {
		return Value{}, true, false
	}
	if target.Kind != ElementValue {
		// The property is required, and the model references something that cannot be its value.
		e.recordUnsupportedReason(element, class, property, Reason{
			Category: ReasonNotApplicable,
			Detail:   class + "::" + property + " references a feature that is not a " + kind,
		})
		return Value{}, true, false
	}
	return target, true, true
}

// structureValueOfKind is value when it is an element of metaclass kind, else null.
func (e *Evaluator) structureValueOfKind(value Value, kind string) (Value, bool, bool) {
	if value.Kind != ElementValue {
		return nullValue(), true, true
	}
	matches, known := e.isMetaclass(value.Element, kind)
	if !known {
		return Value{}, true, false
	}
	if !matches {
		return nullValue(), true, true
	}
	return value, true, true
}
