package semantics

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// OwnedMemberships returns the Membership objects through which the namespace
// sym owns its members, in declaration order (KerML 8.3.2.5 Namespace::
// ownedMembership): a synthesized OwningMembership — or the SysML membership
// kind the member's declaration implies — per owned member, and an alias as
// itself, since `alias` declares a Membership. An element owned through an
// Annotation (prefix metadata, a relationship body's annotating element) has no
// membership (KerML 8.2.3.3.1 OwnedAnnotation).
func (m *Model) OwnedMemberships(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Scope == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.memberships[sym]; ok {
		return cached
	}
	journal(m, m.memberships, sym, sym.Decl)
	var out []*symbols.Symbol
	seen := make(map[*symbols.Symbol]bool)
	ordinal := 0
	sym.Scope.ForEachMember(func(member *symbols.Symbol) bool {
		if member == nil || seen[member] || member.Chain != nil || ownedThroughAnnotation(sym, member) {
			return true
		}
		seen[member] = true
		if member.Kind == symbols.SymbolAlias {
			out = append(out, member)
			return true
		}
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    sym.DocName,
			DeclSpan:   member.DeclSpan,
			Visibility: member.Visibility,
			Implicit: &symbols.ImplicitRelationship{
				Owner: sym, Ordinal: ordinal, Form: symbols.ImplicitMembership, Member: member,
			},
		})
		ordinal++
		return true
	})
	m.memberships[sym] = out
	return out
}

// ownedThroughAnnotation reports whether owner owns member through an
// Annotation rather than a Membership: a `#M` prefix metadata feature, or an
// annotating element in the body of a relationship or alias (KerML 8.2.3.3.1).
func ownedThroughAnnotation(owner, member *symbols.Symbol) bool {
	if isPrefixMetadataSymbol(member) {
		return true
	}
	return relationshipBodyOwner(owner) && isAnnotatingSymbol(member) && !symbols.AnnotatesOthers(member.Decl)
}

// isPrefixMetadataSymbol reports whether sym is the metadata feature a `#M`
// prefix declares on the element it annotates.
func isPrefixMetadataSymbol(sym *symbols.Symbol) bool {
	if sym.Recorded() {
		return sym.Facts.Node == symbols.NodePrefixMetadata
	}
	_, ok := sym.Decl.(*ast.PrefixMetadata)
	return ok
}

// isAnnotatingSymbol reports whether sym declares a KerML AnnotatingElement:
// a comment, documentation, textual representation or metadata feature.
func isAnnotatingSymbol(sym *symbols.Symbol) bool {
	switch sym.Kind {
	case symbols.SymbolComment, symbols.SymbolDocumentation, symbols.SymbolTextualRepresentation, symbols.SymbolMetadataUsage:
		return true
	}
	return false
}

// relationshipBodyOwner reports whether sym's body is a relationship body
// (KerML 8.2.3.1 RelationshipBody), whose annotating elements the relationship
// owns through Annotations: a dependency, an alias, or a keyword-first
// relationship member.
func relationshipBodyOwner(sym *symbols.Symbol) bool {
	if sym.Kind == symbols.SymbolDependency || sym.Kind == symbols.SymbolAlias {
		return true
	}
	_, ok := sym.RelationshipDecl()
	return ok
}

// OwningMembership returns the Membership through which sym's owner owns it, or
// nil for an element owned through no membership: a root element, an alias or
// other relationship owned directly, a prefix metadata feature, or a reflected
// relationship object.
func (m *Model) OwningMembership(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Chain != nil || sym.Kind == symbols.SymbolAlias {
		return nil
	}
	owner := m.ownerOf(sym)
	if owner == nil {
		return nil
	}
	for _, membership := range m.OwnedMemberships(owner) {
		if membership.Implicit != nil && membership.Implicit.Member == sym {
			return membership
		}
	}
	return nil
}

// OwningRelationship returns the relationship that owns sym (KerML 8.3.2.1
// Element::owningRelationship): its owning membership, the Annotation that owns
// a prefix metadata feature or a relationship body's annotating element, or
// the relationship whose chain target it denotes; nil for an element owned
// directly, such as a membership or alias, and for a root element.
func (m *Model) OwningRelationship(sym *symbols.Symbol) *symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil {
		return nil
	}
	if sym.Chain != nil {
		return sym.Chain.Relationship
	}
	if membership := m.OwningMembership(sym); membership != nil {
		return membership
	}
	return m.owningAnnotatingRelationship(sym)
}

// OwnedImports returns the Import objects the namespace sym declares, in
// declaration order (KerML 8.3.2.5 Namespace::ownedImport).
func (m *Model) OwnedImports(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil || sym.Implicit != nil || sym.Scope == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.ownedImports[sym]; ok {
		return cached
	}
	journal(m, m.ownedImports, sym, sym.Decl)
	var out []*symbols.Symbol
	for ordinal, imp := range scopeImportDecls(sym.Scope) {
		out = append(out, &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    sym.DocName,
			DeclSpan:   imp.Span(),
			Visibility: imp.Visibility,
			Implicit: &symbols.ImplicitRelationship{
				Owner: sym, Ordinal: ordinal, Form: symbols.ImplicitImport, Decl: imp,
			},
		})
	}
	m.ownedImports[sym] = out
	return out
}

// scopeImportDecls returns the import declarations directly in a scope's body:
// a recorded scope holds them itself, a parsed one reads its node.
func scopeImportDecls(scope *symbols.Scope) []*ast.Import {
	if scope.Recorded() {
		return scope.Imports()
	}
	var out []*ast.Import
	for _, member := range ast.NodeBodyMembers(scope.Node()) {
		if imp, ok := member.(*ast.Import); ok {
			out = append(out, imp)
		}
	}
	return out
}

// OwnedRelationships returns the relationships sym owns (KerML 8.3.2.1
// Element::ownedRelationship): the Annotations of its prefix metadata, the
// relationship clauses written on it, and its body's imports, memberships and
// `about` annotations in source order.
func (m *Model) OwnedRelationships(sym *symbols.Symbol) []*symbols.Symbol {
	if m == nil || sym == nil {
		return nil
	}
	if sym.Implicit != nil {
		return m.implicitOwnedRelationships(sym)
	}
	var out []*symbols.Symbol
	for _, ann := range m.OwnedAnnotations(sym) {
		if ann.Implicit.Member != nil && isPrefixMetadataSymbol(ann.Implicit.Member) {
			out = append(out, ann)
		}
	}
	out = append(out, m.ImplicitRelationships(sym)...)
	var body []*symbols.Symbol
	for _, ann := range m.OwnedAnnotations(sym) {
		if ann.Implicit.Member == nil || !isPrefixMetadataSymbol(ann.Implicit.Member) {
			body = append(body, ann)
		}
	}
	body = append(body, m.OwnedImports(sym)...)
	body = append(body, m.OwnedMemberships(sym)...)
	body = append(body, m.AnnotatingRelationships(sym)...)
	sort.SliceStable(body, func(i, j int) bool { return body[i].DeclSpan.Offset < body[j].DeclSpan.Offset })
	return append(out, body...)
}

// implicitOwnedRelationships is OwnedRelationships for a reflected relationship
// object: an owned annotation owns nothing but its annotating element, which is
// no relationship, and a membership, import or written clause owns nothing.
func (m *Model) implicitOwnedRelationships(sym *symbols.Symbol) []*symbols.Symbol {
	return []*symbols.Symbol{}
}

// membershipMetaclassName is the metaclass of the membership through which
// owner owns member: a FeatureMembership for a feature of a type, narrowed to
// the SysML membership the member's declaration implies (SysML 8.3), else an
// OwningMembership.
func (m *Model) membershipMetaclassName(owner, member *symbols.Symbol) string {
	if name := m.sysmlMembershipMetaclassName(owner, member); name != "" {
		return name
	}
	if !member.IsFeature() || !m.reflectiveMetaclassConforms(owner, "Type") {
		return "OwningMembership"
	}
	if member.Kind == symbols.SymbolConnectorEnd || usageFlag(member, func(u *ast.Usage) bool { return u.IsEnd }) {
		return "EndFeatureMembership"
	}
	if usageFlag(member, func(u *ast.Usage) bool { return u.IsResult }) {
		return "ReturnParameterMembership"
	}
	if behaviorLike(owner) || m.reflectiveMetaclassConforms(owner, "Step") {
		if direction, ok := ReflectiveDirection(member); ok && direction != ast.DirNone {
			return "ParameterMembership"
		}
	}
	return "FeatureMembership"
}

// sysmlMembershipMetaclassName is the SysML membership kind member's
// declaration implies, or "" where the KerML kinds decide.
func (m *Model) sysmlMembershipMetaclassName(owner, member *symbols.Symbol) string {
	if m.isKerMLDoc(member) {
		return ""
	}
	if isMetadataBodyMember(member) {
		return ""
	}
	if _, ok := requirementConstraintKind(member); ok {
		if kind, _ := member.UsageKind(); kind == ast.UsageFramedConcern {
			return "FramedConcernMembership"
		}
		return "RequirementConstraintMembership"
	}
	if isVerifiedRequirementMember(member) {
		return "RequirementVerificationMembership"
	}
	if member.Recorded() && member.Facts.Node == symbols.NodeSubject {
		return "SubjectMembership"
	}
	if _, ok := member.Decl.(*ast.SubjectMember); ok {
		return "SubjectMembership"
	}
	if IsVariant(member) {
		return "VariantMembership"
	}
	kind, ok := member.UsageKind()
	if !ok {
		return ""
	}
	switch kind {
	case ast.UsageSubject:
		return "SubjectMembership"
	case ast.UsageObjective:
		return "ObjectiveMembership"
	case ast.UsageActor:
		return "ActorMembership"
	case ast.UsageStakeholder:
		return "StakeholderMembership"
	case ast.UsageViewRendering:
		return "ViewRenderingMembership"
	case ast.UsageMetadata:
		// A metadata usage in a body annotates the element owning the body
		// through an OwningMembership (SysML.xtext AnnotatingMember).
		return "OwningMembership"
	}
	if usageFlag(member, func(u *ast.Usage) bool { return u.IsStateAction() }) {
		return "StateSubactionMembership"
	}
	return ""
}

// isMetadataBodyMember reports whether member is a feature of a metadata
// annotation's body (`@M { x = 1; }`), which binds the annotation's feature
// rather than declaring a member of the annotated element.
func isMetadataBodyMember(member *symbols.Symbol) bool {
	return member.OwnerScope != nil && member.OwnerScope.Annotated() != nil
}

// usageFlag reports pred of member's usage declaration; false for any other
// declaration or a record.
func usageFlag(member *symbols.Symbol, pred func(*ast.Usage) bool) bool {
	u, ok := member.Decl.(*ast.Usage)
	return ok && pred(u)
}

// requirementConstraintKind is the RequirementConstraintKind of the membership
// through which a requirement owns member: `assumption` for an `assume`,
// `requirement` for a `require` or a framed concern; ok is false for any other
// member (SysML 8.3.19 RequirementConstraintMembership, FramedConcernMembership).
func requirementConstraintKind(member *symbols.Symbol) (kind string, ok bool) {
	if member.Recorded() {
		switch member.Facts.Node {
		case symbols.NodeAssume:
			return "assumption", true
		case symbols.NodeRequire:
			return "requirement", true
		case symbols.NodeUsage:
			if member.Facts.UsageKind == ast.UsageFramedConcern {
				return "requirement", true
			}
			switch member.Facts.Keyword {
			case "assume":
				return "assumption", true
			case "require":
				return "requirement", true
			}
		}
		return "", false
	}
	switch d := member.Decl.(type) {
	case *ast.AssumeMember:
		return "assumption", true
	case *ast.RequireMember:
		return "requirement", true
	case *ast.Usage:
		if d.Kind == ast.UsageFramedConcern {
			return "requirement", true
		}
		if !d.IsRequirementConstraint() {
			return "", false
		}
		if d.Keyword == "assume" || d.PrefixKeyword == "assume" {
			return "assumption", true
		}
		return "requirement", true
	}
	return "", false
}

// isVerifiedRequirementMember reports whether member is an objective's
// `verify r;` (SysML 8.3.19 RequirementVerificationMembership).
func isVerifiedRequirementMember(member *symbols.Symbol) bool {
	if member.Recorded() {
		return member.Facts.Node == symbols.NodeUsage && member.Facts.UsageKind == ast.UsageSatisfy &&
			member.Facts.Keyword == "verify"
	}
	return usageFlag(member, func(u *ast.Usage) bool { return u.IsVerifiedRequirement() })
}

// membershipElements is ReflectiveElements for a reflected Membership object.
func (m *Model) membershipElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	rel := sym.Implicit
	owner := []*symbols.Symbol{rel.Owner}
	member := []*symbols.Symbol{rel.Member}
	empty := []*symbols.Symbol{}
	switch feature {
	case "source", "membershipOwningNamespace", "owningRelatedElement", "owner", "owningType", "type":
		if feature == "owningType" || feature == "type" {
			if !m.reflectiveMetaclassConforms(sym, "FeatureMembership") {
				return nil, false
			}
		}
		return owner, true
	case "target", "memberElement", "ownedMemberElement", "ownedRelatedElement":
		return member, true
	case "relatedElement":
		return []*symbols.Symbol{rel.Owner, rel.Member}, true
	case "ownedMemberFeature", "feature", "ownedFeature":
		if !m.reflectiveMetaclassConforms(sym, "FeatureMembership") {
			return nil, false
		}
		return member, true
	case "ownedMemberParameter":
		if !m.reflectiveMetaclassConforms(sym, "ParameterMembership") {
			return nil, false
		}
		return member, true
	case "ownedResultExpression":
		if !m.reflectiveMetaclassConforms(sym, "ResultExpressionMembership") {
			return nil, false
		}
		return member, true
	case "ownedConstraint", "ownedRequirement", "ownedConcern", "ownedSubjectParameter",
		"ownedObjectiveRequirement", "ownedActorParameter", "ownedStakeholderParameter",
		"ownedVariantUsage", "ownedRendering", "action":
		if !m.reflectiveMetaclassConforms(sym, sysmlMembershipOwnedEnd[feature]) {
			return nil, false
		}
		return member, true
	case "referencedConstraint", "referencedConcern", "verifiedRequirement", "referencedRendering":
		if !m.reflectiveMetaclassConforms(sym, sysmlMembershipReferencedEnd[feature]) {
			return nil, false
		}
		return m.referencedConstraintOf(rel.Member, feature), true
	case "ownedRelationship", "ownedElement", "ownedAnnotation", "owningRelationship", "owningMembership",
		"owningNamespace", "documentation", "textualRepresentation", "ownedImport", "ownedMembership":
		// A membership owns its member through no relationship of its own, so
		// Element::ownedElement (ownedRelationship.ownedRelatedElement) is empty.
		return empty, true
	}
	return nil, false
}

// sysmlMembershipOwnedEnd is the metaclass declaring each SysML membership's
// owned-member end, each a redefinition of OwningMembership::ownedMemberElement.
var sysmlMembershipOwnedEnd = map[string]string{
	"ownedConstraint":           "RequirementConstraintMembership",
	"ownedRequirement":          "RequirementVerificationMembership",
	"ownedConcern":              "FramedConcernMembership",
	"ownedSubjectParameter":     "SubjectMembership",
	"ownedObjectiveRequirement": "ObjectiveMembership",
	"ownedActorParameter":       "ActorMembership",
	"ownedStakeholderParameter": "StakeholderMembership",
	"ownedVariantUsage":         "VariantMembership",
	"ownedRendering":            "ViewRenderingMembership",
	"action":                    "StateSubactionMembership",
}

// sysmlMembershipReferencedEnd is the metaclass declaring each SysML
// membership's referenced end: the feature the owned member's reference
// subsetting targets, else the owned member itself.
var sysmlMembershipReferencedEnd = map[string]string{
	"referencedConstraint": "RequirementConstraintMembership",
	"referencedConcern":    "FramedConcernMembership",
	"verifiedRequirement":  "RequirementVerificationMembership",
	"referencedRendering":  "ViewRenderingMembership",
}

// referencedConstraintOf is the element a membership's owned member stands for:
// the target of its reference subsetting when it has one, else the member
// itself (SysML 8.3.19 deriveRequirementConstraintMembershipReferencedConstraint).
func (m *Model) referencedConstraintOf(member *symbols.Symbol, feature string) []*symbols.Symbol {
	if target := m.referenceSubsettingTarget(member); target != nil {
		return []*symbols.Symbol{target}
	}
	return []*symbols.Symbol{member}
}

// referenceSubsettingTarget is the feature member's owned reference subsetting
// targets (`verify r;`, `require r;`, `satisfy r;`), or nil for a member
// declaring its own constraint or whose reference does not resolve.
func (m *Model) referenceSubsettingTarget(member *symbols.Symbol) *symbols.Symbol {
	if member.Recorded() {
		if target := m.recordedElement(member.Facts.References); target != nil {
			return target
		}
		for _, kind := range []ast.RelationshipKind{ast.RelReferences, ast.RelSubsets} {
			if targets := m.RecordedRelationshipTargets(member, kind); len(targets) > 0 {
				return targets[0]
			}
		}
		return nil
	}
	switch d := member.Decl.(type) {
	case *ast.Usage:
		if rel := d.ReferenceSubsetting(); rel != nil {
			return m.RelationshipTarget(member, rel)
		}
	case *ast.AssumeMember:
		if d.Reference != nil {
			target, ok := m.resolver.ResolveQualified(member.OwnerScope, d.Reference)
			if ok {
				return target
			}
		}
	case *ast.RequireMember:
		if d.Reference != nil {
			target, ok := m.resolver.ResolveQualified(member.OwnerScope, d.Reference)
			if ok {
				return target
			}
		}
	}
	return nil
}

// membershipValue is reflectiveFeatureValue for a reflected Membership object.
func (m *Model) membershipValue(sym *symbols.Symbol, feature string) (symbols.FilterValue, bool) {
	rel := sym.Implicit
	switch feature {
	case "memberName", "ownedMemberName":
		return stringOrEmpty(simpleSymbolName(rel.Member)), true
	case "memberShortName", "ownedMemberShortName":
		return stringOrEmpty(m.EffectiveShortNameOf(rel.Member)), true
	case "visibility":
		return m.visibilityValue(rel.Member.Visibility), true
	case "kind":
		if m.reflectiveMetaclassConforms(sym, "RequirementConstraintMembership") {
			kind, _ := requirementConstraintKind(rel.Member)
			return m.enumLiteralValue(m.sysmlMetaclass("RequirementConstraintKind"), kind)
		}
		if m.reflectiveMetaclassConforms(sym, "StateSubactionMembership") {
			return m.enumLiteralValue(m.sysmlMetaclass("StateSubactionKind"), stateSubactionKind(rel.Member))
		}
	}
	return symbols.FilterValue{}, false
}

// stateSubactionKind is the `entry`, `do` or `exit` a state subaction is written with.
func stateSubactionKind(member *symbols.Symbol) string {
	u, ok := member.Decl.(*ast.Usage)
	if !ok {
		return ""
	}
	switch u.Keyword {
	case "entry", "do", "exit":
		return u.Keyword
	}
	return u.PrefixKeyword
}

// visibilityValue is the VisibilityKind literal of a declared visibility,
// `public` where none is written (KerML 8.2.3.5.1).
func (m *Model) visibilityValue(vis ast.Visibility) symbols.FilterValue {
	name := "public"
	switch vis {
	case ast.VisibilityPrivate:
		name = "private"
	case ast.VisibilityProtected:
		name = "protected"
	}
	value, _ := m.enumLiteralValue(m.kermlMetaclass("VisibilityKind"), name)
	return value
}

// enumLiteralValue is the reference value naming the literal of kind called
// name; ok is false where the library declares no such literal.
func (m *Model) enumLiteralValue(kind *symbols.Symbol, name string) (symbols.FilterValue, bool) {
	if kind == nil || name == "" {
		return symbols.FilterValue{}, false
	}
	literal, ok := m.LookupMember(kind, name)
	if !ok || literal == nil {
		return symbols.FilterValue{}, false
	}
	fqn := m.fqnOf(literal)
	if fqn == "" {
		return symbols.FilterValue{}, false
	}
	return symbols.FilterValue{Kind: symbols.FilterValueRef, RefFQN: fqn}, true
}

// importElements is ReflectiveElements for a reflected Import object.
func (m *Model) importElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	rel := sym.Implicit
	imp, _ := rel.Decl.(*ast.Import)
	owner := []*symbols.Symbol{rel.Owner}
	empty := []*symbols.Symbol{}
	switch feature {
	case "source", "importOwningNamespace", "owningRelatedElement", "owner":
		return owner, true
	case "ownedRelatedElement", "ownedElement", "ownedRelationship", "ownedAnnotation",
		"owningRelationship", "owningMembership", "owningNamespace", "documentation", "textualRepresentation":
		return empty, true
	}
	target := m.importTarget(rel.Owner, imp)
	switch feature {
	case "importedElement", "importedNamespace", "target":
		if feature == "importedNamespace" && imp.Kind != ast.ImportNamespace {
			return nil, false
		}
		if target == nil {
			return nil, false
		}
		return []*symbols.Symbol{target}, true
	case "importedMembership":
		if imp.Kind != ast.ImportMembership || target == nil {
			return nil, false
		}
		if target.Kind == symbols.SymbolAlias {
			return []*symbols.Symbol{target}, true
		}
		if membership := m.OwningMembership(target); membership != nil {
			return []*symbols.Symbol{membership}, true
		}
		return nil, false
	case "relatedElement":
		if target == nil {
			return nil, false
		}
		return []*symbols.Symbol{rel.Owner, target}, true
	}
	return nil, false
}

// importTarget resolves the element an import names, in the importing namespace.
func (m *Model) importTarget(owner *symbols.Symbol, imp *ast.Import) *symbols.Symbol {
	if imp == nil || imp.Imported == nil || owner.Scope == nil {
		return nil
	}
	target, ok := m.resolver.ResolveQualified(owner.Scope, imp.Imported)
	if !ok || target == nil {
		return nil
	}
	if imp.Kind == ast.ImportMembership {
		// A membership import names the membership, which an alias is itself.
		return target
	}
	if resolved, ok := m.resolver.ResolveAliasTarget(target); ok && resolved != nil {
		return resolved
	}
	return target
}

// importValue is reflectiveFeatureValue for a reflected Import object.
func (m *Model) importValue(sym *symbols.Symbol, feature string) (symbols.FilterValue, bool) {
	imp, _ := sym.Implicit.Decl.(*ast.Import)
	if imp == nil {
		return symbols.FilterValue{}, false
	}
	switch feature {
	case "isRecursive":
		return boolValue(imp.IsRecursive), true
	case "isImportAll":
		return boolValue(imp.IsAll), true
	case "visibility":
		return m.visibilityValue(imp.Visibility), true
	}
	return symbols.FilterValue{}, false
}

// importMetaclassName is the metaclass of the import a declaration states:
// a NamespaceImport or MembershipImport, or SysML's Expose kinds for `expose`.
func (m *Model) importMetaclassName(sym *symbols.Symbol) string {
	imp, _ := sym.Implicit.Decl.(*ast.Import)
	if imp == nil {
		return ""
	}
	expose := imp.IsExpose && !m.isKerMLDoc(sym.Implicit.Owner)
	if imp.Kind == ast.ImportNamespace {
		if expose {
			return "NamespaceExpose"
		}
		return "NamespaceImport"
	}
	if expose {
		return "MembershipExpose"
	}
	return "MembershipImport"
}

// inheritedMemberships returns the memberships of the members sym inherits
// (KerML 8.3.3.1 Type::inheritedMembership), each the owning membership of an
// inherited member not masked by a redefinition, in inheritance order.
func (m *Model) inheritedMemberships(sym *symbols.Symbol) []*symbols.Symbol {
	own := make(map[*symbols.Symbol]bool)
	for _, member := range ownedMembersOf(sym) {
		own[member] = true
	}
	var out []*symbols.Symbol
	for _, member := range m.membersIncludingAnonymous(sym) {
		if own[member] || m.ownerOf(member) == sym {
			continue
		}
		if membership := m.OwningMembership(member); membership != nil {
			out = append(out, membership)
		} else if member.Kind == symbols.SymbolAlias {
			out = append(out, member)
		}
	}
	return out
}

// featureMembershipsOf filters memberships to those classifying as FeatureMembership.
func (m *Model) featureMembershipsOf(memberships []*symbols.Symbol) []*symbols.Symbol {
	out := []*symbols.Symbol{}
	for _, membership := range memberships {
		if m.reflectiveMetaclassConforms(membership, "FeatureMembership") {
			out = append(out, membership)
		}
	}
	return out
}

// namespaceRelationshipElements is ReflectiveElements for the relationship-valued
// features of Element, Namespace and Type that the implied Membership, Import
// and Annotation objects answer; ok is false for any other feature.
func (m *Model) namespaceRelationshipElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	switch feature {
	case "ownedRelationship":
		return m.OwnedRelationships(sym), true
	case "ownedAnnotation":
		out := []*symbols.Symbol{}
		return append(out, m.OwnedAnnotations(sym)...), true
	case "owningRelationship":
		if rel := m.OwningRelationship(sym); rel != nil {
			return []*symbols.Symbol{rel}, true
		}
		return []*symbols.Symbol{}, true
	case "owningMembership":
		if membership := m.OwningMembership(sym); membership != nil {
			return []*symbols.Symbol{membership}, true
		}
		return []*symbols.Symbol{}, true
	case "ownedMembership":
		if !m.reflectiveMetaclassConforms(sym, "Namespace") {
			return nil, false
		}
		return m.OwnedMemberships(sym), true
	case "ownedImport":
		if !m.reflectiveMetaclassConforms(sym, "Namespace") {
			return nil, false
		}
		return m.OwnedImports(sym), true
	case "ownedFeatureMembership":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		return m.featureMembershipsOf(m.OwnedMemberships(sym)), true
	case "inheritedMembership":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		return m.inheritedMemberships(sym), true
	case "featureMembership":
		if !m.reflectiveMetaclassConforms(sym, "Type") {
			return nil, false
		}
		owned := m.featureMembershipsOf(m.OwnedMemberships(sym))
		return append(owned, m.featureMembershipsOf(m.inheritedMemberships(sym))...), true
	case "textualRepresentation":
		var reps []*symbols.Symbol
		for _, elem := range m.ownedElementsOf(sym) {
			if elem.Kind == symbols.SymbolTextualRepresentation {
				reps = append(reps, elem)
			}
		}
		return reps, true
	}
	return nil, false
}

// aliasMembershipElements is ReflectiveElements for an alias, the Membership
// it declares (KerML 8.2.3.5.2): its member is the aliased element, owned by
// the namespace declaring the alias.
func (m *Model) aliasMembershipElements(sym *symbols.Symbol, feature string) ([]*symbols.Symbol, bool) {
	owner := m.ownerOf(sym)
	owners := []*symbols.Symbol{}
	if owner != nil {
		owners = append(owners, owner)
	}
	empty := []*symbols.Symbol{}
	switch feature {
	case "source", "membershipOwningNamespace", "owningRelatedElement", "owner":
		return owners, true
	case "target", "memberElement":
		if target, ok := m.resolver.ResolveAliasTarget(sym); ok && target != nil {
			return []*symbols.Symbol{target}, true
		}
		return nil, false
	case "relatedElement":
		target, ok := m.resolver.ResolveAliasTarget(sym)
		if !ok || target == nil {
			return nil, false
		}
		return append(owners, target), true
	case "ownedRelatedElement", "ownedElement", "owningRelationship", "owningMembership", "owningNamespace":
		return empty, true
	}
	return nil, false
}

// aliasMembershipValue is reflectiveFeatureValue for an alias: the member name
// and short name it declares, and its visibility.
func (m *Model) aliasMembershipValue(sym *symbols.Symbol, feature string) (symbols.FilterValue, bool) {
	switch feature {
	case "memberName":
		return stringOrEmpty(simpleSymbolName(sym)), true
	case "memberShortName":
		return stringOrEmpty(sym.ShortName), true
	case "visibility":
		return m.visibilityValue(sym.Visibility), true
	}
	return symbols.FilterValue{}, false
}
