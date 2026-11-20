package metamodel

// importedMembershipNames are the names a membership binds (KerML 8.3.2.4.3 memberName and
// memberShortName), read from the structure or, for a library membership, from its symbol.
func (e *Evaluator) importedMembershipNames(membership Membership) ([]string, bool) {
	handle := Element{Membership: membership, IsMembership: true}
	var names []string
	if !e.structureKnows(handle) {
		member := membership.Member
		if member == nil {
			return nil, false
		}
		if member.Named() {
			names = append(names, member.Name)
		}
		if member.HasShortName() {
			names = append(names, member.ShortName)
		}
		return names, true
	}
	for _, property := range [][2]string{{"memberName", "name"}, {"memberShortName", "shortName"}} {
		value, handled, ok := e.structureMemberName(handle, property[0], property[1])
		if !handled || !ok {
			return nil, false
		}
		if value.Kind == StringValue && value.String != "" {
			names = append(names, value.String)
		}
	}
	return names, true
}

// importedMember is the one element a membership names.
func (e *Evaluator) importedMember(membership Membership) (Element, bool) {
	members, ok := e.structureMembershipMemberElements(Element{Membership: membership, IsMembership: true})
	if !ok || len(members) != 1 {
		return Element{}, false
	}
	return members[0], true
}

func (e *Evaluator) importedMetaclassName(element Element) (string, bool) {
	if name, ok := e.options.Structure.Metaclass(element); ok {
		return name, true
	}
	if symbol := e.structureElementSymbol(element); symbol != nil && e.model != nil {
		if metaclass := e.model.MetaclassOf(symbol); metaclass != nil {
			return metaclass.Name, true
		}
	}
	return "", false
}

// importedDistinguishableByMetaclass is the metaclass clause of Membership::isDistinguishableFrom
// (KerML 8.3.2.4.3): neither element's metaclass conforms to the other's.
func (e *Evaluator) importedDistinguishableByMetaclass(a, b Element) (bool, bool) {
	nameA, ok := e.importedMetaclassName(a)
	if !ok {
		return false, false
	}
	nameB, ok := e.importedMetaclassName(b)
	if !ok {
		return false, false
	}
	aIsB, known := e.isMetaclass(a, nameB)
	if !known {
		return false, false
	}
	bIsA, known := e.isMetaclass(b, nameA)
	if !known {
		return false, false
	}
	return !aIsB && !bIsA, true
}

// namedMembership is a membership with what Membership::isDistinguishableFrom reads of it.
type namedMembership struct {
	names  []string
	member Element
}

func (e *Evaluator) namedMembership(membership Membership) (namedMembership, bool) {
	names, ok := e.importedMembershipNames(membership)
	if !ok || len(names) == 0 {
		return namedMembership{}, ok
	}
	member, ok := e.importedMember(membership)
	return namedMembership{names: names, member: member}, ok
}

// structureHideImportCollisions drops the imports a name collision hides (KerML 7.2.5.4,
// 8.3.2.4.3): an owned or inherited membership stays, two colliding imports both go.
func (e *Evaluator) structureHideImportCollisions(
	namespace Element,
	entries []importedEntry,
	inherited []Value,
) ([]importedEntry, bool) {
	if len(entries) == 0 {
		return entries, true
	}
	owned, handled, ok := e.structureOwnedFilter(namespace, "Membership", true)
	if !handled || !ok {
		return nil, false
	}
	hiders := make(map[string][]Element)
	for _, value := range append(slicesClone(owned.Values), inherited...) {
		if value.Kind != MembershipValue {
			return nil, false
		}
		named, ok := e.namedMembership(value.Membership)
		if !ok {
			return nil, false
		}
		for _, name := range named.names {
			hiders[name] = append(hiders[name], named.member)
		}
	}
	hidden := make([]bool, len(entries))
	members := make([]Element, len(entries))
	byName := make(map[string][]int)
	var order []string
	for i, entry := range entries {
		named, ok := e.namedMembership(entry.membership.Membership)
		if !ok {
			return nil, false
		}
		members[i] = named.member
		for _, name := range named.names {
			for _, hider := range hiders[name] {
				distinguishable, ok := e.importedDistinguishableByMetaclass(named.member, hider)
				if !ok {
					return nil, false
				}
				hidden[i] = hidden[i] || !distinguishable
			}
			if _, seen := byName[name]; !seen {
				order = append(order, name)
			}
			byName[name] = append(byName[name], i)
		}
	}
	for _, name := range order {
		sharing := byName[name]
		for x, i := range sharing {
			for _, j := range sharing[x+1:] {
				if i == j {
					continue
				}
				distinguishable, ok := e.importedDistinguishableByMetaclass(members[i], members[j])
				if !ok {
					return nil, false
				}
				if !distinguishable {
					hidden[i], hidden[j] = true, true
				}
			}
		}
	}
	kept := entries[:0:0]
	for i, entry := range entries {
		if !hidden[i] {
			kept = append(kept, entry)
		}
	}
	return kept, true
}

// structureFilterImported is Package::importedMemberships (KerML 8.3.4.13.4): the imported
// memberships whose member element meets every filterCondition (includeAsMember).
func (e *Evaluator) structureFilterImported(namespace Element, entries []importedEntry) ([]importedEntry, bool) {
	if len(entries) == 0 {
		return entries, true
	}
	isPackage, known := e.isMetaclass(namespace, "Package")
	if !known {
		return nil, false
	}
	if !isPackage {
		return entries, true
	}
	filters, handled, ok := e.structureOwnedFilter(namespace, "ElementFilterMembership", true)
	if !handled || !ok {
		return nil, false
	}
	if len(filters.Values) == 0 {
		return entries, true
	}
	var conditions []Element
	for _, filter := range filters.Values {
		members, ok := e.structureMembershipMembers(Element{Membership: filter.Membership, IsMembership: true})
		if !ok || len(members) != 1 {
			return nil, false
		}
		conditions = append(conditions, members[0])
	}
	kept := entries[:0:0]
	for _, entry := range entries {
		member, ok := e.importedMember(entry.membership.Membership)
		if !ok {
			return nil, false
		}
		included, ok := e.structureIncludeAsMember(member, conditions)
		if !ok {
			e.recordUnsupportedReason(namespace, "Namespace", "importedMembership", Reason{
				Category: ReasonIncompleteStructure,
				Detail:   "Package::filterCondition is not evaluable over the structure",
			})
			return nil, false
		}
		if included {
			kept = append(kept, entry)
		}
	}
	return kept, true
}

// structureIncludeAsMember is Package::includeAsMember (KerML 8.3.4.13.4): every
// condition holds for element given the metadata annotating it.
func (e *Evaluator) structureIncludeAsMember(element Element, conditions []Element) (bool, bool) {
	metadata, ok := e.structureMetadataFeatures(element)
	if !ok {
		return false, false
	}
	for _, condition := range conditions {
		value, ok := e.structureFilterBoolean(condition, element, metadata)
		if !ok {
			return false, false
		}
		if value.kind != filterValueBoolean || !value.boolean {
			return false, true
		}
	}
	return true, true
}

// structureMetadataFeatures lists the metadata features annotating element (KerML
// 8.3.2.3.2): those it owns that state no annotation, and those annotations make about it.
func (e *Evaluator) structureMetadataFeatures(element Element) ([]Element, bool) {
	var candidates []Element
	if symbol := e.structureLibrarySymbol(element); symbol != nil {
		// A library element's own metadata is not in the structure; only an annotation
		// the model states `about` it is.
		if e.model == nil || len(e.model.AnnotatingMetadataOf(symbol)) != 0 {
			return nil, false
		}
		return e.structureAnnotatingMetadata(element, candidates)
	}
	if !e.structureKnows(element) {
		return nil, false
	}
	owned, handled, ok := e.structureOwnedFilter(element, "OwningMembership", true)
	if !handled || !ok {
		return nil, false
	}
	key := e.structureFeatureKey(element)
	for _, membership := range owned.Values {
		members, ok := e.structureMembershipMembers(Element{Membership: membership.Membership, IsMembership: true})
		if !ok {
			return nil, false
		}
		for _, member := range members {
			isMetadata, known := e.isMetaclass(member, "MetadataFeature")
			if !known {
				return nil, false
			}
			if !isMetadata {
				continue
			}
			annotated, ok := e.Property(member, "AnnotatingElement", "annotatedElement")
			if !ok {
				return nil, false
			}
			for _, value := range annotated.Values {
				if value.Kind == ElementValue && e.structureFeatureKey(value.Element) == key {
					candidates = append(candidates, member)
				}
			}
		}
	}
	return e.structureAnnotatingMetadata(element, candidates)
}

// structureAnnotatingMetadata adds to candidates the metadata features annotating element
// through an Annotation it does not own.
func (e *Evaluator) structureAnnotatingMetadata(element Element, candidates []Element) ([]Element, bool) {
	annotating, ok := e.options.Structure.(AnnotationStructure)
	if !ok {
		return nil, false
	}
	annotations, ok := annotating.Annotations(element)
	if !ok {
		return nil, false
	}
	for _, annotation := range annotations {
		sources, ok := e.relationshipEndpoints(annotation, "annotatingElement")
		if !ok || len(sources) != 1 || sources[0].Kind != ElementValue {
			return nil, false
		}
		isMetadata, known := e.isMetaclass(sources[0].Element, "MetadataFeature")
		if !known {
			return nil, false
		}
		if isMetadata {
			candidates = append(candidates, sources[0].Element)
		}
	}
	return uniqueStructureElements(candidates, e.options.Structure), true
}

// structureOperandExpressions lists the expressions an operator expression binds its
// input parameters to, in parameter order, read from the stated parameter feature values.
func (e *Evaluator) structureOperandExpressions(expression Element) ([]Element, bool) {
	parameters, handled, ok := e.structureOwnedFilter(expression, "ParameterMembership", true)
	if !handled || !ok {
		return nil, false
	}
	var operands []Element
	for _, parameter := range parameters.Values {
		handle := Element{Membership: parameter.Membership, IsMembership: true}
		if result, known := e.isMetaclass(handle, "ReturnParameterMembership"); !known {
			return nil, false
		} else if result {
			continue
		}
		features, ok := e.structureMembershipMembers(handle)
		if !ok || len(features) != 1 {
			return nil, false
		}
		values, handled, ok := e.structureOwnedFilter(features[0], "FeatureValue", true)
		if !handled || !ok || len(values.Values) > 1 {
			return nil, false
		}
		if len(values.Values) == 0 {
			continue
		}
		bound, ok := e.structureMembershipMembers(Element{Membership: values.Values[0].Membership, IsMembership: true})
		if !ok || len(bound) != 1 {
			return nil, false
		}
		operands = append(operands, bound[0])
	}
	return operands, true
}

// structureMetadataClassified evaluates `@T` for element: metadata annotating it is typed
// by T or a specialization, or T is a metaclass of the abstract syntax element instantiates.
func (e *Evaluator) structureMetadataClassified(test, element Element, metadata []Element) (bool, bool) {
	tested, ok := e.structureTypeArgument(test)
	if !ok {
		return false, false
	}
	if symbol := e.reflectiveMetaclass(tested); symbol != nil {
		return e.isMetaclass(element, symbol.Name)
	}
	target := e.structureFeatureKey(tested)
	for _, feature := range metadata {
		conforms, ok := e.structureMetadataConforms(feature, target)
		if !ok || conforms {
			return conforms, ok
		}
	}
	return false, true
}

// structureMetadataConforms reports whether a type of the metadata feature is target or
// specializes it.
func (e *Evaluator) structureMetadataConforms(metadata Element, target ElementKey) (bool, bool) {
	types, handled, ok := e.structureFeatureTypes(metadata)
	if !handled || !ok {
		return false, false
	}
	for _, value := range types.Values {
		if value.Kind != ElementValue {
			return false, false
		}
		conforms, ok := e.structureConformsTo(value.Element, target, map[ElementKey]bool{})
		if !ok || conforms {
			return conforms, ok
		}
	}
	return false, true
}

// structureConformsTo reports whether element is target or specializes it, directly or not.
func (e *Evaluator) structureConformsTo(element Element, target ElementKey, visited map[ElementKey]bool) (bool, bool) {
	key := e.structureFeatureKey(element)
	if key == target {
		return true, true
	}
	if visited[key] {
		return false, true
	}
	visited[key] = true
	if e.structureLibrarySymbol(element) != nil {
		return false, true
	}
	bases, ok := e.structureOwnedSupertypes(element)
	if !ok {
		return false, false
	}
	for _, base := range bases {
		conforms, ok := e.structureConformsTo(base, target, visited)
		if !ok || conforms {
			return conforms, ok
		}
	}
	return false, true
}
