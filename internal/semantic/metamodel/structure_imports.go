package metamodel

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// importedEntry is one membership the imports of a namespace bring into it, with
// the visibility of the import that brought it first.
type importedEntry struct {
	membership Value
	visibility string
}

// structureImportedEntry memoizes the imported memberships of one namespace.
type structureImportedEntry struct {
	entries []importedEntry
	ok      bool
}

// importWalk is one derivation of imported memberships over a structure. excluded is
// the `excluded` argument of the KerML operations (8.3.2.4.2 Import::importedMemberships):
// the namespaces on the path from the namespace the derivation started at. cut records
// that an import of an excluded namespace was skipped, so what was computed since holds
// for this path alone.
type importWalk struct {
	excluded map[ElementKey]bool
	cut      bool
}

func newImportWalk() *importWalk {
	return &importWalk{excluded: make(map[ElementKey]bool)}
}

// structureGraphImportedMemberships is Namespace::importedMembership (KerML 8.3.2.4.5
// deriveNamespaceImportedMembership) for a namespace only the structure describes.
func (e *Evaluator) structureGraphImportedMemberships(element Element) (Value, bool, bool) {
	entries, ok := e.structureImportedEntries(element, newImportWalk())
	if !ok {
		return Value{}, true, false
	}
	values := make([]Value, 0, len(entries))
	for _, entry := range entries {
		values = append(values, entry.membership)
	}
	return sequence(values...), true, true
}

// structureImportedEntries is Namespace::importedMemberships(excluded) (KerML 8.3.2.4.5)
// with the import visibilities kept: what every owned import brings, less what a Package's
// filter conditions reject (8.3.4.13.4) and what a name collision hides (7.2.5.4). A result
// no excluded namespace shaped is the namespace's own and is memoized.
func (e *Evaluator) structureImportedEntries(namespace Element, walk *importWalk) ([]importedEntry, bool) {
	key := e.structureFeatureKey(namespace)
	e.mu.Lock()
	entry, done := e.structureImported[key]
	e.mu.Unlock()
	if done {
		return entry.entries, entry.ok
	}
	cutBefore := walk.cut
	walk.cut = false
	outer := walk.excluded[key]
	walk.excluded[key] = true
	entries, ok := e.computeStructureImportedEntries(namespace, walk)
	if !outer {
		delete(walk.excluded, key)
	}
	if !walk.cut {
		e.mu.Lock()
		e.structureImported[key] = structureImportedEntry{entries: slices.Clip(entries), ok: ok}
		e.mu.Unlock()
	}
	walk.cut = walk.cut || cutBefore
	return entries, ok
}

func (e *Evaluator) computeStructureImportedEntries(namespace Element, walk *importWalk) ([]importedEntry, bool) {
	if base, ok := e.structureLibraryBase(namespace); ok {
		return e.libraryImportedEntries(base.Symbol)
	}
	imports, handled, ok := e.structureOwnedFilter(namespace, "Import", false)
	if !handled || !ok {
		return nil, false
	}
	if len(imports.Values) == 0 {
		return nil, true
	}
	var entries []importedEntry
	var seen seenValues
	for _, imported := range imports.Values {
		if imported.Kind != ElementValue {
			return nil, false
		}
		visibility, ok := e.structureVisibilityName(imported.Element)
		if !ok {
			return nil, false
		}
		memberships, ok := e.structureImportMemberships(imported.Element, walk)
		if !ok {
			return nil, false
		}
		for _, membership := range memberships {
			if membership.Kind != MembershipValue {
				return nil, false
			}
			if !seen.repeats(membership) {
				entries = append(entries, importedEntry{membership: membership, visibility: visibility})
			}
		}
	}
	entries, ok = e.structureFilterImported(namespace, entries)
	if !ok {
		return nil, false
	}
	return e.structureHideImportCollisions(namespace, entries)
}

// structureVisibilityName is the visibility a membership or import states.
func (e *Evaluator) structureVisibilityName(relationship Element) (string, bool) {
	value, ok := e.options.Structure.Attribute(relationship, "visibility")
	if !ok {
		return "", false
	}
	switch value.Kind {
	case EnumValue:
		return value.Enum, true
	case StringValue:
		return value.String, true
	}
	return "", false
}

func (e *Evaluator) structureBooleanAttribute(element Element, property string) (bool, bool) {
	value, ok := e.options.Structure.Attribute(element, property)
	if !ok || value.Kind != BooleanValue {
		return false, false
	}
	return value.Boolean, true
}

// structureImportMemberships is Import::importedMemberships(excluded) for an import the
// structure states: NamespaceImport (KerML 8.3.2.4.6) or MembershipImport (8.3.2.4.4).
func (e *Evaluator) structureImportMemberships(imported Element, walk *importWalk) ([]Value, bool) {
	recursive, ok := e.structureBooleanAttribute(imported, "isRecursive")
	if !ok {
		return nil, false
	}
	all, ok := e.structureBooleanAttribute(imported, "isImportAll")
	if !ok {
		return nil, false
	}
	isNamespaceImport, known := e.isMetaclass(imported, "NamespaceImport")
	if !known {
		return nil, false
	}
	if isNamespaceImport {
		target, ok := e.structureSingleEndpoint(imported, "importedNamespace")
		if !ok {
			return nil, false
		}
		if walk.excluded[e.structureFeatureKey(target)] {
			walk.cut = true
			return nil, true
		}
		owner, owned, ok := e.options.Structure.OwningRelationship(target)
		if ok && owned && e.structureFeatureKey(owner) == e.structureFeatureKey(imported) {
			return e.structureFilterPackageMemberships(target, walk)
		}
		return e.structureVisibleMemberships(target, walk, recursive, all)
	}
	isMembershipImport, known := e.isMetaclass(imported, "MembershipImport")
	if !known || !isMembershipImport {
		return nil, false
	}
	membership, ok := e.structureImportedMembershipValue(imported)
	if !ok {
		return nil, false
	}
	values := []Value{membership}
	if !recursive {
		return values, true
	}
	members, ok := e.structureMembershipMemberElements(Element{Membership: membership.Membership, IsMembership: true})
	if !ok || len(members) != 1 {
		return nil, false
	}
	isNamespace, known := e.isMetaclass(members[0], "Namespace")
	if !known {
		return nil, false
	}
	if !isNamespace {
		return values, true
	}
	if walk.excluded[e.structureFeatureKey(members[0])] {
		walk.cut = true
		return values, true
	}
	visible, ok := e.structureVisibleMemberships(members[0], walk, true, all)
	if !ok {
		return nil, false
	}
	return append(values, visible...), true
}

// structureFilterPackageMemberships is what a filtered import brings (KerML 7.2.5.4): the
// memberships the filter package's own import selects that meet its conditions. The
// notation gives that inner import no visibility (8.2.3.4.2), so none is asked of it.
func (e *Evaluator) structureFilterPackageMemberships(filterPackage Element, walk *importWalk) ([]Value, bool) {
	key := e.structureFeatureKey(filterPackage)
	if walk.excluded[key] {
		walk.cut = true
		return nil, true
	}
	entries, ok := e.structureImportedEntries(filterPackage, walk)
	if !ok {
		return nil, false
	}
	values := make([]Value, 0, len(entries))
	for _, entry := range entries {
		values = append(values, entry.membership)
	}
	return values, true
}

func (e *Evaluator) structureSingleEndpoint(relationship Element, property string) (Element, bool) {
	endpoints, ok := e.relationshipEndpoints(relationship, property)
	if !ok || len(endpoints) != 1 || endpoints[0].Kind != ElementValue {
		return Element{}, false
	}
	return endpoints[0].Element, true
}

// structureImportedMembershipValue is MembershipImport::importedMembership: a membership
// the structure states, or the library's when the structure only references it.
func (e *Evaluator) structureImportedMembershipValue(imported Element) (Value, bool) {
	endpoints, ok := e.relationshipEndpoints(imported, "importedMembership")
	if !ok || len(endpoints) != 1 {
		return Value{}, false
	}
	switch endpoint := endpoints[0]; endpoint.Kind {
	case MembershipValue:
		return endpoint, true
	case ElementValue:
		if endpoint.Element.IsMembership {
			return membershipValue(endpoint.Element.Membership), true
		}
	}
	return Value{}, false
}

// structureVisibleMemberships is Namespace::visibleMemberships(excluded, isRecursive,
// includeAll) (KerML 8.3.2.4.5) with Type's redefinition (8.3.3.1.10), which adds the
// inherited memberships and, under a recursive import, leaves implied specializations out.
func (e *Evaluator) structureVisibleMemberships(namespace Element, walk *importWalk, recursive, all bool) ([]Value, bool) {
	if base, ok := e.structureLibraryBase(namespace); ok {
		return e.libraryVisibleMemberships(base.Symbol, recursive, all, make(map[*symbols.Symbol]bool))
	}
	owned, handled, ok := e.structureOwnedFilter(namespace, "Membership", true)
	if !handled || !ok {
		return nil, false
	}
	var visible []Value
	var nested []Element
	for _, value := range owned.Values {
		if value.Kind != MembershipValue {
			return nil, false
		}
		handle := Element{Membership: value.Membership, IsMembership: true}
		if !all {
			visibility, ok := e.structureVisibilityName(handle)
			if !ok {
				return nil, false
			}
			if visibility != "public" {
				continue
			}
		}
		visible = append(visible, value)
		if !recursive {
			continue
		}
		owning, known := e.isMetaclass(handle, "OwningMembership")
		if !known {
			return nil, false
		}
		if !owning {
			continue
		}
		members, ok := e.structureMembershipMemberElements(handle)
		if !ok {
			return nil, false
		}
		for _, member := range members {
			isNamespace, known := e.isMetaclass(member, "Namespace")
			if !known {
				return nil, false
			}
			if isNamespace {
				nested = append(nested, member)
			}
		}
	}
	entries, ok := e.structureImportedEntries(namespace, walk)
	if !ok {
		return nil, false
	}
	for _, entry := range entries {
		if all || entry.visibility == "public" {
			visible = append(visible, entry.membership)
		}
	}
	key := e.structureFeatureKey(namespace)
	outer := walk.excluded[key]
	walk.excluded[key] = true
	defer func() {
		if !outer {
			delete(walk.excluded, key)
		}
	}()
	for _, member := range nested {
		memberships, ok := e.structureVisibleMemberships(member, walk, true, all)
		if !ok {
			return nil, false
		}
		visible = append(visible, memberships...)
	}
	isType, known := e.isMetaclass(namespace, "Type")
	if !known {
		return nil, false
	}
	if isType {
		inherited, ok := e.structureVisibleInherited(namespace, walk, recursive, all)
		if !ok {
			return nil, false
		}
		visible = append(visible, inherited...)
	}
	return uniqueOwnedStructureValues(visible, e.options.Structure), true
}

// structureVisibleInherited is the inherited part of Type::visibleMemberships (KerML
// 8.3.3.1.10): inheritedMemberships(excluded, {}, isRecursive) selected by visibility.
func (e *Evaluator) structureVisibleInherited(element Element, walk *importWalk, excludeImplied, all bool) ([]Value, bool) {
	inherited, ok := e.structureWalkInheritedMemberships(element, walk, map[ElementKey]bool{}, excludeImplied)
	if !ok || all {
		return inherited, ok
	}
	visible := make([]Value, 0, len(inherited))
	for _, value := range inherited {
		public, ok := e.structureMembershipIsPublic(value)
		if !ok {
			return nil, false
		}
		if public {
			visible = append(visible, value)
		}
	}
	return visible, true
}

// structureMembershipIsPublic reads a membership's visibility from the structure, or from
// the member symbol when the structure does not state the membership.
func (e *Evaluator) structureMembershipIsPublic(value Value) (bool, bool) {
	if value.Kind != MembershipValue {
		return false, false
	}
	handle := Element{Membership: value.Membership, IsMembership: true}
	if e.structureKnows(handle) {
		visibility, ok := e.structureVisibilityName(handle)
		return visibility == "public", ok
	}
	member := value.Membership.Member
	if member == nil {
		return false, false
	}
	return member.Visibility == ast.VisibilityDefault || member.Visibility == ast.VisibilityPublic, true
}

// structureWalkInheritedMemberships is Type::inheritedMemberships(excludedNamespaces,
// excludedTypes, excludeImplied) (KerML 8.3.3.1.10) inside an import derivation: the
// walk's excluded namespaces reach the imports of every supertype, and with excludeImplied
// only the specializations the model states are followed, at every level (7.2.5.4, second
// limitation on recursive imports).
func (e *Evaluator) structureWalkInheritedMemberships(
	element Element,
	walk *importWalk,
	visited map[ElementKey]bool,
	excludeImplied bool,
) ([]Value, bool) {
	key := e.structureFeatureKey(element)
	if visited[key] {
		return nil, true
	}
	visited[key] = true
	defer delete(visited, key)
	var bases []Element
	var ok bool
	if excludeImplied {
		bases, ok = e.structureStatedSupertypes(element)
	} else {
		bases, ok = e.structureOwnedSupertypes(element)
	}
	if !ok {
		return nil, false
	}
	var inheritable []Value
	for _, base := range bases {
		if visited[e.structureFeatureKey(base)] {
			continue
		}
		if library, ok := e.structureLibraryBase(base); ok {
			var members []Value
			if excludeImplied {
				members, ok = e.libraryStatedNonPrivate(library.Symbol)
			} else {
				members, ok = e.structureLibraryTypeMembers(library, "membership")
			}
			if !ok {
				return nil, false
			}
			inheritable = append(inheritable, members...)
			continue
		}
		owned, handled, ok := e.structureOwnedFilter(base, "Membership", true)
		if !handled || !ok {
			return nil, false
		}
		var public, protected []Value
		for _, value := range owned.Values {
			if value.Kind != MembershipValue {
				return nil, false
			}
			visibility, ok := e.structureVisibilityName(Element{Membership: value.Membership, IsMembership: true})
			if !ok {
				return nil, false
			}
			switch visibility {
			case "public":
				public = append(public, value)
			case "protected":
				protected = append(protected, value)
			}
		}
		imported, ok := e.structureImportedEntries(base, walk)
		if !ok {
			return nil, false
		}
		for _, entry := range imported {
			switch entry.visibility {
			case "public":
				public = append(public, entry.membership)
			case "protected":
				protected = append(protected, entry.membership)
			}
		}
		inherited, ok := e.structureWalkInheritedMemberships(base, walk, visited, excludeImplied)
		if !ok {
			return nil, false
		}
		inheritable = append(append(append(inheritable, public...), protected...), inherited...)
	}
	inheritable = uniqueOwnedStructureValues(inheritable, e.options.Structure)
	return e.structureRemoveRedefinedMemberships(element, inheritable)
}

// structureStatedSupertypes is Type::supertypes(true) (KerML 8.3.3.1.10): the original
// type of a conjugated type, else the generals of the specializations that are not implied.
func (e *Evaluator) structureStatedSupertypes(element Element) ([]Element, bool) {
	conjugators, handled, ok := e.structureOwnedFilter(element, "Conjugation", false)
	if !handled || !ok {
		return nil, false
	}
	if len(conjugators.Values) != 0 {
		return e.structureOwnedSupertypes(element)
	}
	specializations, handled, ok := e.structureOwnedFilter(element, "Specialization", false)
	if !handled || !ok {
		return nil, false
	}
	var bases []Element
	for _, specialization := range specializations.Values {
		if specialization.Kind != ElementValue {
			return nil, false
		}
		implied, ok := e.structureBooleanAttribute(specialization.Element, "isImplied")
		if !ok {
			return nil, false
		}
		if implied {
			continue
		}
		generals, ok := e.relationshipEndpoints(specialization.Element, "general")
		if !ok {
			return nil, false
		}
		for _, general := range generals {
			if general.Kind != ElementValue {
				return nil, false
			}
			isType, known := e.isMetaclass(general.Element, "Type")
			if !known {
				return nil, false
			}
			if isType {
				bases = append(bases, general.Element)
			}
		}
	}
	return uniqueStructureElements(bases, e.options.Structure), true
}

// nonPrivateImported holds the memberships a type's public and protected imports bring.
type nonPrivateImported struct {
	public, protected []Value
}

// structureNonPrivateImported is the imported part of Type::nonPrivateMemberships (KerML
// 8.3.3.1.10): membershipsOfVisibility(public) and (protected) over the type's imports.
func (e *Evaluator) structureNonPrivateImported(element Element) ([]nonPrivateImported, bool) {
	entries, ok := e.structureImportedEntries(element, newImportWalk())
	if !ok {
		return nil, false
	}
	if len(entries) == 0 {
		return nil, true
	}
	var imported nonPrivateImported
	for _, entry := range entries {
		switch entry.visibility {
		case "public":
			imported.public = append(imported.public, entry.membership)
		case "protected":
			imported.protected = append(imported.protected, entry.membership)
		}
	}
	return []nonPrivateImported{imported}, true
}

// libraryImportedEntries is structureImportedEntries for a library namespace the structure
// only references: its imports are the declarations', read through the resolver.
func (e *Evaluator) libraryImportedEntries(namespace *symbols.Symbol) ([]importedEntry, bool) {
	if namespace == nil || namespace.Scope == nil {
		return nil, true
	}
	imports := namespace.Scope.Imports()
	if len(imports) == 0 {
		return nil, true
	}
	if e.resolver == nil {
		return nil, false
	}
	var entries []importedEntry
	seen := make(map[MembershipKey]bool)
	for _, imp := range imports {
		target, ok := e.resolver.ImportTarget(namespace.Scope, imp)
		if !ok || target == nil || !e.importTreeComplete(target, imp, make(map[*symbols.Scope]bool)) {
			return nil, false
		}
		visibility := "private"
		switch imp.Visibility {
		case ast.VisibilityPublic:
			visibility = "public"
		case ast.VisibilityProtected:
			visibility = "protected"
		}
		for _, symbol := range e.resolver.ImportedElements(namespace.Scope, imp) {
			if symbol == nil {
				return nil, false
			}
			if symbol == target && (imp.Kind == ast.ImportNamespace || imp.IsRecursive) {
				continue
			}
			membership := ElementOf(symbol).Membership
			if !hasMembership(membership) {
				return nil, false
			}
			if seen[membership.Key()] {
				continue
			}
			seen[membership.Key()] = true
			entries = append(entries, importedEntry{membership: membershipValue(membership), visibility: visibility})
		}
	}
	return entries, true
}

// libraryVisibleMemberships is structureVisibleMemberships for a library namespace.
func (e *Evaluator) libraryVisibleMemberships(namespace *symbols.Symbol, recursive, all bool, visited map[*symbols.Symbol]bool) ([]Value, bool) {
	if namespace == nil || visited[namespace] {
		return nil, true
	}
	visited[namespace] = true
	if e.model == nil {
		return nil, false
	}
	var visible []Value
	var nested []*symbols.Symbol
	var members []*symbols.Symbol
	if namespace.Scope != nil {
		members = namespace.Scope.Members()
	}
	for _, member := range members {
		if member == nil {
			continue
		}
		if !all && member.Visibility != ast.VisibilityDefault && member.Visibility != ast.VisibilityPublic {
			continue
		}
		membership := ElementOf(member).Membership
		if !hasMembership(membership) {
			continue
		}
		visible = append(visible, membershipValue(membership))
		if recursive && member.Kind != symbols.SymbolAlias && member.Scope != nil {
			nested = append(nested, member)
		}
	}
	entries, ok := e.libraryImportedEntries(namespace)
	if !ok {
		return nil, false
	}
	for _, entry := range entries {
		if all || entry.visibility == "public" {
			visible = append(visible, entry.membership)
		}
	}
	for _, member := range nested {
		memberships, ok := e.libraryVisibleMemberships(member, true, all, visited)
		if !ok {
			return nil, false
		}
		visible = append(visible, memberships...)
	}
	if isType(namespace) {
		var inherited []Value
		if recursive {
			inherited, ok = e.libraryStatedInherited(namespace)
		} else {
			inherited, ok = e.libraryInherited(namespace)
		}
		if !ok {
			return nil, false
		}
		for _, value := range inherited {
			public, ok := e.structureMembershipIsPublic(value)
			if !ok {
				return nil, false
			}
			if all || public {
				visible = append(visible, value)
			}
		}
	}
	return uniqueOwnedStructureValues(visible, e.options.Structure), true
}

// libraryInherited lists the memberships a library type inherits (KerML 8.3.3.1.10).
func (e *Evaluator) libraryInherited(base *symbols.Symbol) ([]Value, bool) {
	members, ok := e.structureLibraryTypeMembers(ElementOf(base), "membership")
	if !ok {
		return nil, false
	}
	inherited := make([]Value, 0, len(members))
	for _, value := range members {
		if member := value.Membership.Member; member != nil && member.Owner() != base {
			inherited = append(inherited, value)
		}
	}
	return inherited, true
}

// libraryStatedInherited is libraryInherited with implied specializations left out at
// every level: the memberships a stated supertype of base owns.
func (e *Evaluator) libraryStatedInherited(base *symbols.Symbol) ([]Value, bool) {
	inherited, ok := e.libraryInherited(base)
	if !ok {
		return nil, false
	}
	stated := e.model.AllDeclaredSupertypes(base)
	kept := inherited[:0:0]
	for _, value := range inherited {
		if slices.Contains(stated, value.Membership.Member.Owner()) {
			kept = append(kept, value)
		}
	}
	return kept, true
}

// libraryStatedNonPrivate is Type::nonPrivateMemberships with excludeImplied for a
// library type: its own non-private memberships and those its stated supertypes give it.
func (e *Evaluator) libraryStatedNonPrivate(base *symbols.Symbol) ([]Value, bool) {
	members, ok := e.structureLibraryTypeMembers(ElementOf(base), "membership")
	if !ok {
		return nil, false
	}
	stated := e.model.AllDeclaredSupertypes(base)
	kept := members[:0:0]
	for _, value := range members {
		member := value.Membership.Member
		if member == nil {
			return nil, false
		}
		if owner := member.Owner(); owner == base || slices.Contains(stated, owner) {
			kept = append(kept, value)
		}
	}
	return kept, true
}
