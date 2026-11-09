package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// UsageMayTimeVary derives SysML Usage::mayTimeVary (SysML v2 §8.3.6.4).
func (m *Model) UsageMayTimeVary(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Recorded() {
		// Recorded only by this derivation, on a usage or the constraint a
		// require/assume member declares.
		return !m.isKerMLDoc(sym) && sym.Facts.Modifiers.Has(symbols.ModMayTimeVary)
	}
	if m.isKerMLDoc(sym) {
		// A KerML feature has no Usage::mayTimeVary; its isVariable is declared.
		return false
	}
	var portion, actionKind, bodyParameter bool
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		portion = d.IsPortion || d.Portion != ast.PortionNone
		actionKind = d.Kind == ast.UsageAction
	case *ast.RequireMember, *ast.AssumeMember:
		// A required or assumed constraint is the ConstraintUsage its member declares.
	case *ast.SubjectMember:
		// A subject parameter is the ReferenceUsage its SubjectMembership declares.
	case *ast.Relationship:
		// A satisfy's `by` subject is a parameter too; its owning type is the satisfy usage.
		if d.Kind != ast.RelSubject {
			return false
		}
	case *ast.ConnectorEnd:
		// A connector's end is the usage its EndFeatureMembership declares; its
		// owning type is the connector.
	case *ast.BodyExpr:
		// An expression body's parameter: its owning type is the Expression, a
		// Performance and so an Occurrence (KerML Performances::Performance).
		bodyParameter = true
	case *ast.TimeEvent, *ast.ChangeEvent, *ast.AcceptEvent:
		// A trigger's payload parameter: its owning type is the accept action
		// the trigger synthesizes, an action and so an Occurrence.
		bodyParameter = true
	case *ast.QualifiedName:
		// A flow end's flow feature; its owning type is the flow end, which
		// references the connected occurrence.
		if !IsFlowFeature(sym) {
			return false
		}
	default:
		return false
	}
	owner := synthesizedPartOwner(sym)
	if owner == nil && sym.OwnerScope != nil {
		owner = sym.OwnerScope.Owner()
	}
	if (owner == nil && sym.OwnerScope == nil) || m.isPayloadFeature(sym) || IsVariant(sym) {
		// A variant's VariantMembership is an OwningMembership, no
		// FeatureMembership, so the variant has no owningType.
		return false
	}
	if !bodyParameter && (owner == nil || !m.owningTypeIsOccurrence(owner)) || portion ||
		m.conformsByName(sym, "Links::SelfLink") ||
		m.conformsByName(sym, "Occurrences::HappensLink") {
		return false
	}
	return !m.EffectiveIsComposite(sym) ||
		(!actionKind && !m.conformsByName(sym, "Actions::Action"))
}

// UsageMayTimeVaryUnder derives Usage::mayTimeVary for a referential usage that
// declares nothing of its own — a connector end, a parameter a transition or a
// satisfy's subject implies — whose owning type is owner (deriveUsageMayTimeVary).
func (m *Model) UsageMayTimeVaryUnder(owner *symbols.Symbol) bool {
	return owner != nil && !m.isKerMLDoc(owner) && m.conformsByName(owner, occurrenceFQN)
}

// UsageMayTimeVaryUnderKind is UsageMayTimeVaryUnder for an owning usage declared
// by nothing but its kind, read through the kind's implicit library base.
func (m *Model) UsageMayTimeVaryUnderKind(kind ast.UsageKind) bool {
	base, ok := implicitUsageBases[kind]
	return ok && m.conformsByName(m.librarySymbol(base), occurrenceFQN)
}

// UsageMayTimeVaryUnderLibraryFeature is UsageMayTimeVaryUnder for an owning
// feature the notation fixes as redefining the library feature fqn.
func (m *Model) UsageMayTimeVaryUnderLibraryFeature(fqn string) bool {
	return m.conformsByName(m.librarySymbol(fqn), occurrenceFQN)
}

// isPayloadFeature reports a flow's payload: a PayloadFeature is a KerML Feature,
// not a Usage, so Usage::mayTimeVary does not apply (SysML v2.1 §8.3.16).
func (m *Model) isPayloadFeature(sym *symbols.Symbol) bool {
	meta := m.MetaclassOf(sym)
	return meta != nil && meta.Name == "PayloadFeature"
}

// UsageMayTimeVaryForOwningType derives Usage::mayTimeVary for the owning type
// stated by Structure, or nil when the usage has no owning FeatureMembership.
func (m *Model) UsageMayTimeVaryForOwningType(sym, owner *symbols.Symbol) bool {
	if sym == nil || m.isKerMLDoc(sym) {
		// A KerML feature has no Usage::mayTimeVary (as UsageMayTimeVary).
		return false
	}
	isPortion, underAction := false, false
	if sym.Recorded() {
		if sym.OwnerScope != nil && sym.OwnerScope.Owner() == owner {
			// The recorded fact is this derivation over the symbol's own owner.
			return m.UsageMayTimeVary(sym)
		}
		isPortion = sym.Facts.Modifiers.Has(symbols.ModPortion)
	} else {
		switch d := sym.Decl.(type) {
		case *ast.Usage:
			isPortion = d.IsPortion || d.Portion != ast.PortionNone
		case *ast.RequireMember, *ast.AssumeMember, *ast.SubjectMember, *ast.BodyExpr, *ast.ConnectorEnd:
		case *ast.TimeEvent, *ast.ChangeEvent, *ast.AcceptEvent:
			// A trigger's payload parameter, owned by the accept action.
			underAction = true
		case *ast.QualifiedName:
			// A flow end's flow feature, owned by the flow end.
			if !IsFlowFeature(sym) {
				return false
			}
		case *ast.Relationship:
			// A satisfy's `by` subject parameter.
			if d.Kind != ast.RelSubject {
				return false
			}
		default:
			return false
		}
	}
	if owner == nil || m.isPayloadFeature(sym) || IsVariant(sym) || (!underAction && !m.owningTypeIsOccurrence(owner)) ||
		isPortion ||
		m.conformsByName(sym, "Links::SelfLink") ||
		m.conformsByName(sym, "Occurrences::HappensLink") {
		return false
	}
	return !m.EffectiveIsComposite(sym) || !m.conformsByName(sym, "Actions::Action")
}

// FeatureIsVariable derives KerML Feature::isVariable (KerML 1.1 §8.3.3.1): declared by
// `var` or `const` in KerML, and redefined as Usage::mayTimeVary for a SysML usage.
// A cross feature's membership is no FeatureMembership, so it has no owningType.
func (m *Model) FeatureIsVariable(sym *symbols.Symbol) bool {
	if sym == nil || isKerMLTypeDecl(sym) {
		return false
	}
	// A connector's implied end declares no `var` and is written non-variable (connectorEnd).
	if sym.Expr != nil && sym.Expr.Role == symbols.ExpressionConnectorEnd {
		return false
	}
	if sym.Recorded() {
		mods := sym.Facts.Modifiers
		if m.isKerMLDoc(sym) {
			return mods.Has(symbols.ModVariable) || mods.Has(symbols.ModConstant)
		}
		return mods.Has(symbols.ModVariable) || m.UsageMayTimeVary(sym)
	}
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		if m.isKerMLDoc(sym) {
			return d.IsVariable || d.IsConstant
		}
		return m.UsageMayTimeVary(sym)
	case *ast.CrossFeatureMember:
		return m.isKerMLDoc(sym) && (d.IsVariable || d.IsConstant)
	case *ast.RequireMember, *ast.AssumeMember, *ast.SubjectMember, *ast.BodyExpr, *ast.ConnectorEnd,
		*ast.TimeEvent, *ast.ChangeEvent, *ast.AcceptEvent:
		return !m.isKerMLDoc(sym) && m.UsageMayTimeVary(sym)
	case *ast.QualifiedName:
		// A flow end's flow feature (`from a.x`: the ReferenceUsage for x the end owns).
		return IsFlowFeature(sym) && !m.isKerMLDoc(sym) && m.UsageMayTimeVary(sym)
	case *ast.Relationship:
		// A satisfy's `by` subject parameter.
		return d.Kind == ast.RelSubject && !m.isKerMLDoc(sym) && m.UsageMayTimeVary(sym)
	}
	return false
}

// IsKerMLFeature reports whether sym is a KerML feature declaration.
func (m *Model) IsKerMLFeature(sym *symbols.Symbol) bool {
	return sym != nil && sym.IsFeature() && m.isKerMLDoc(sym)
}

// usageIsReferential derives SysML Usage::isReference: attribute usages are
// always referential; other usages are referential unless composite.
func usageIsReferential(usage *ast.Usage) bool {
	if usage == nil {
		return false
	}
	if usage.Kind == ast.UsageAttribute {
		return true
	}
	return !usageIsComposite(usage)
}

// UsageIsReferential reports whether a usage is a reference rather than composition.
func UsageIsReferential(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usageIsReferential(usage)
}

// UsageIsComposite derives SysML Usage::isComposite (SysML v2 §7.6.2) for the usage sym declares:
// the objects it holds are portions of the object holding it. A symbol declaring no usage is not.
func UsageIsComposite(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usageIsComposite(usage)
}

func (m *Model) UsageIsReferential(sym *symbols.Symbol) bool {
	return sym != nil && !m.UsageIsComposite(sym)
}

func (m *Model) isSysMLUsage(sym *symbols.Symbol) bool {
	if m == nil || sym == nil || m.isKerMLDoc(sym) {
		return false
	}
	if sym.Recorded() {
		return sym.Facts.Node == symbols.NodeUsage ||
			m.metaclassConforms(sym, sysmlMetaclassPrefix+"Usage")
	}
	switch sym.Decl.(type) {
	case *ast.Usage, *ast.SubstateMember:
		return true
	}
	return m.metaclassConforms(sym, sysmlMetaclassPrefix+"Usage")
}

func (m *Model) isPortUsage(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Kind == symbols.SymbolPortUsage ||
		(sym.Recorded() && sym.Facts.UsageKind == ast.UsagePort) {
		return true
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Kind == ast.UsagePort {
		return true
	}
	return m.metaclassConforms(sym, sysmlMetaclassPrefix+"PortUsage")
}

// UsageIsComposite reports whether sym declares a composite usage.
func (m *Model) UsageIsComposite(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if isControlNode(sym) {
		return true
	}
	if symbolIsPortion(sym) && m.isKerMLDoc(sym) {
		return true
	}
	if hasDefaultInputParameterDirection(sym) {
		return false
	}
	if m.usageIsAlwaysReferential(sym) {
		return false
	}
	if _, ok := sym.Decl.(*ast.SubstateMember); ok {
		return true
	}
	if sym.Recorded() {
		if m.metaclassConforms(sym, sysmlMetaclassPrefix+"ControlNode") {
			return true
		}
		if sym.Kind.IsAttributeLike() || sym.Kind == symbols.SymbolReferenceUsage ||
			sym.Facts.Modifiers.Has(symbols.ModEnd|symbols.ModEvent) ||
			sym.Facts.Direction != ast.DirNone {
			return false
		}
		if m.reflectiveUsageHasAttributeFeaturingType(sym) {
			return false
		}
		if sym.Facts.UsageKind == ast.UsagePort && !m.portHasExpectedFeaturingType(sym) {
			return false
		}
		if sym.Facts.Modifiers.Has(symbols.ModComposite) {
			return true
		}
		return !recordedUsageIsReferential(sym)
	}
	if m.metaclassConforms(sym, sysmlMetaclassPrefix+"ControlNode") {
		return true
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		if !m.metaclassConforms(sym, sysmlMetaclassPrefix+"Usage") ||
			m.metaclassConforms(sym, sysmlMetaclassPrefix+"AttributeUsage") ||
			m.metaclassConforms(sym, sysmlMetaclassPrefix+"ReferenceUsage") ||
			m.metaclassConforms(sym, sysmlMetaclassPrefix+"EventOccurrenceUsage") ||
			m.reflectiveUsageHasAttributeFeaturingType(sym) {
			return false
		}
		if m.metaclassConforms(sym, sysmlMetaclassPrefix+"PortUsage") &&
			!m.portHasExpectedFeaturingType(sym) {
			return false
		}
		return defaultUsageComposite(sym.Kind)
	}
	if m.metaclassConforms(sym, sysmlMetaclassPrefix+"AttributeUsage") ||
		m.metaclassConforms(sym, sysmlMetaclassPrefix+"ReferenceUsage") ||
		m.metaclassConforms(sym, sysmlMetaclassPrefix+"EventOccurrenceUsage") ||
		usage.IsPerformedAction() || usage.IsExhibitedState() {
		return false
	}
	if m.reflectiveUsageHasAttributeFeaturingType(sym) {
		return false
	}
	if usage.Kind == ast.UsagePort && !m.portHasExpectedFeaturingType(sym) {
		return false
	}
	return usageIsComposite(usage)
}

// AttributeUsage, EventOccurrenceUsage, PerformActionUsage, ExhibitStateUsage,
// IncludeUseCaseUsage, and ReferenceUsage have explicit SysML reference rules.
// (SysML.uml:8641–8650, 9670–9676, 11964–11973, 12559–12565,
// 14735–14741, and 15601–15610.)
func (m *Model) usageIsAlwaysReferential(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if isBodySuccessionNode(sym.Decl) {
		// A `then` succession is a SuccessionAsUsage, referential as `succession` is.
		return true
	}
	if sym.Recorded() {
		if sym.Kind == symbols.SymbolBindingUsage || sym.Kind == symbols.SymbolSuccessionUsage ||
			sym.Kind.IsAttributeLike() || sym.Kind == symbols.SymbolReferenceUsage ||
			sym.Facts.UsageKind == ast.UsageBinding || sym.Facts.UsageKind == ast.UsageSuccession ||
			sym.Facts.Modifiers.Has(symbols.ModEvent) {
			return true
		}
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		if usage.Kind == ast.UsageAttribute || usage.Kind == ast.UsageBinding ||
			usage.Kind == ast.UsageSuccession ||
			usage.IsEvent || usage.IsPerformedAction() ||
			usage.IsStateAction() || usage.IsExhibitedState() || usage.IsIncludedUseCase() {
			return true
		}
	}
	if m == nil {
		return false
	}
	for _, metaclass := range []string{
		"AttributeUsage",
		"BindingConnectorAsUsage",
		"EventOccurrenceUsage",
		"ExhibitStateUsage",
		"IncludeUseCaseUsage",
		"PerformActionUsage",
		"ReferenceUsage",
		"SuccessionAsUsage",
	} {
		if m.metaclassConforms(sym, sysmlMetaclassPrefix+metaclass) {
			return true
		}
	}
	return false
}

// EffectiveIsComposite reports the modeled Feature::isComposite value.
func (m *Model) EffectiveIsComposite(sym *symbols.Symbol) bool {
	if m == nil || sym == nil {
		return false
	}
	if isControlNode(sym) || m.metaclassConforms(sym, sysmlMetaclassPrefix+"ControlNode") {
		return true
	}
	isUsage := m.isSysMLUsage(sym)
	var composite bool
	if isUsage {
		if usageRequiredCompositeByMembership(sym) {
			return true
		}
		composite = m.UsageIsComposite(sym)
	} else if sym.Recorded() {
		composite = sym.Facts.Modifiers.Has(symbols.ModComposite)
	} else {
		switch declaration := sym.Decl.(type) {
		case *ast.Usage:
			composite = declaration.IsComposite
		case *ast.CrossFeatureMember:
			composite = declaration.IsComposite
		}
	}

	if m.reflectiveUsageHasAttributeFeaturingType(sym) {
		return false
	}
	if !isUsage {
		return composite
	}
	if m.isPortUsage(sym) && !m.portHasExpectedFeaturingType(sym) {
		return false
	}
	isEnd := sym.Recorded() && sym.Facts.Modifiers.Has(symbols.ModEnd)
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		isEnd = usage.IsEnd
	}
	direction, _ := ReflectiveDirection(sym)
	if isEnd || direction != ast.DirNone || !m.reflectiveUsageHasFeaturingType(sym) {
		return false
	}
	return composite
}

func symbolIsPortion(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Recorded() && sym.Facts.Modifiers.Has(symbols.ModPortion) {
		return true
	}
	switch declaration := sym.Decl.(type) {
	case *ast.Usage:
		return declaration.IsPortion || declaration.Portion != ast.PortionNone
	case *ast.CrossFeatureMember:
		return declaration.IsPortion
	default:
		return false
	}
}

// UsageDeclIsComposite derives SysML Usage::isComposite (SysML v2 §7.6.2) for a usage declaration.
func UsageDeclIsComposite(usage *ast.Usage) bool {
	return usageIsComposite(usage)
}

func usageIsComposite(usage *ast.Usage) bool {
	if usage == nil || usage.IsReference || usage.Direction != ast.DirNone ||
		usage.IsEnd || usage.IsEvent || usage.Kind == ast.UsageEnumeration || usage.IsVariantReference() {
		return false
	}
	// A binding or succession written as a usage is referential (SysML 7.6.3;
	// the pilot's usage post-processing), as usageIsAlwaysReferential states.
	if usage.Kind == ast.UsageBinding || usage.Kind == ast.UsageSuccession {
		return false
	}
	// A usage that refers to what it names (`perform a`, `exhibit s`, `include u`,
	// `render r`) is a reference; an asserted constraint (`assert c`, SysML.xtext
	// AssertConstraintUsage) subsets c by reference and stays composite.
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelReferences && usage.Keyword != "assert" {
			return false
		}
	}
	return true
}

func hasDefaultInputParameterDirection(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Recorded() {
		return sym.Facts.Node == symbols.NodeSubject ||
			sym.Facts.UsageKind == ast.UsageSubject ||
			sym.Facts.UsageKind == ast.UsageActor ||
			sym.Facts.UsageKind == ast.UsageStakeholder
	}
	switch declaration := sym.Decl.(type) {
	case *ast.SubjectMember:
		return true
	case *ast.Usage:
		return declaration.Kind == ast.UsageSubject || declaration.Kind == ast.UsageActor ||
			declaration.Kind == ast.UsageStakeholder
	default:
		return false
	}
}

func usageRequiredCompositeByMembership(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Recorded() {
		if sym.Facts.Node == symbols.NodeAssume || sym.Facts.Node == symbols.NodeRequire ||
			sym.Facts.UsageKind == ast.UsageFramedConcern ||
			sym.Facts.UsageKind == ast.UsageObjective {
			return true
		}
	}
	switch declaration := sym.Decl.(type) {
	case *ast.AssumeMember, *ast.RequireMember:
		return true
	case *ast.Usage:
		return declaration.Kind == ast.UsageFramedConcern || declaration.Kind == ast.UsageObjective
	default:
		return false
	}
}

func (m *Model) portHasExpectedFeaturingType(sym *symbols.Symbol) bool {
	expected := m.reflectiveExpectedFeaturingType(sym)
	return expected != nil &&
		(expected.Kind == symbols.SymbolPortDef ||
			expected.Kind == symbols.SymbolPortUsage ||
			m.reflectiveMetaclassConforms(expected, "PortDefinition") ||
			m.reflectiveMetaclassConforms(expected, "PortUsage"))
}

func (m *Model) reflectiveUsageHasAttributeFeaturingType(sym *symbols.Symbol) bool {
	return m.reflectiveAttributeFeatureOwner(m.reflectiveExpectedFeaturingType(sym))
}

func (m *Model) reflectiveExpectedFeaturingType(sym *symbols.Symbol) *symbols.Symbol {
	seen := make(map[*symbols.Symbol]bool)
	for sym != nil && !seen[sym] {
		seen[sym] = true
		if _, isMetadataAnnotation := sym.Decl.(*ast.PrefixMetadata); isMetadataAnnotation {
			return nil
		}
		if usage, ok := sym.Decl.(*ast.Usage); ok &&
			usage.Kind == ast.UsageMetadata && usage.HasBody {
			return nil
		}
		owner := m.ownerOf(sym)
		if IsVariant(sym) {
			if owner == nil || !m.reflectiveMetaclassConforms(owner, "Usage") || !IsVariation(owner) {
				return nil
			}
			sym = owner
			continue
		}
		if owner == nil ||
			(!owner.IsFeature() &&
				!owner.Kind.IsDefinition() &&
				!m.reflectiveMetaclassConforms(owner, "Type")) {
			return nil
		}
		return owner
	}
	return nil
}

func isControlNode(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch sym.Decl.(type) {
	case *ast.ForkNode, *ast.JoinNode, *ast.MergeNode, *ast.DecisionNode:
		return true
	default:
		return false
	}
}

// DefaultsToInputDirection reports a usage whose kind gives it direction `in`
// without writing one: a subject, actor or stakeholder parameter.
func DefaultsToInputDirection(sym *symbols.Symbol) bool {
	return hasDefaultInputParameterDirection(sym)
}

// synthesizedPartOwner is the element a synthesized part (a transition's
// succession and its ends, a trigger's accept and payload) is owned through,
// its owning type; nil for a symbol declared in a scope of its own.
func synthesizedPartOwner(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.Expr == nil {
		return nil
	}
	switch sym.Kind {
	case symbols.SymbolConnectorEnd, symbols.SymbolSuccessionUsage, symbols.SymbolReferenceUsage, symbols.SymbolActionUsage:
		return sym.Expr.Owner
	}
	return nil
}

// owningTypeIsOccurrence reports whether the owning type owner is an Occurrence
// (deriveUsageMayTimeVary). A succession or binding a transition or body
// synthesizes shares its declaration node with what wrote it, so it is read by
// its kind: a succession is a HappensBefore link, no Occurrence.
func (m *Model) owningTypeIsOccurrence(owner *symbols.Symbol) bool {
	switch owner.Kind {
	case symbols.SymbolMetadataUsage:
		// A metadata usage's type is a Metaobject, an Object (KerML Metaobjects).
		return m.UsageMayTimeVaryUnderKind(ast.UsageMetadata)
	case symbols.SymbolExpression:
		// An expression body is a Performance, an Occurrence (KerML Performances).
		return true
	}
	if owner.Expr != nil {
		switch owner.Kind {
		case symbols.SymbolSuccessionUsage:
			return m.UsageMayTimeVaryUnderKind(ast.UsageSuccession)
		case symbols.SymbolBindingUsage:
			return m.UsageMayTimeVaryUnderKind(ast.UsageBinding)
		}
	}
	return m.conformsByName(owner, occurrenceFQN)
}
