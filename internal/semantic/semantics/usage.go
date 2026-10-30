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
	var portion, actionKind bool
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		portion = d.IsPortion || d.Portion != ast.PortionNone
		actionKind = d.Kind == ast.UsageAction
	case *ast.RequireMember, *ast.AssumeMember:
		// A required or assumed constraint is the ConstraintUsage its member declares.
	default:
		return false
	}
	if sym.OwnerScope == nil {
		return false
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil || !m.conformsByName(owner, "Occurrences::Occurrence") || portion ||
		m.conformsByName(sym, "Links::SelfLink") ||
		m.conformsByName(sym, "Occurrences::HappensLink") {
		return false
	}
	return !m.EffectiveIsComposite(sym) ||
		(!actionKind && !m.conformsByName(sym, "Actions::Action"))
}

// UsageMayTimeVaryForOwningType derives Usage::mayTimeVary for the owning type
// stated by Structure, or nil when the usage has no owning FeatureMembership.
func (m *Model) UsageMayTimeVaryForOwningType(sym, owner *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	isPortion := false
	if sym.Recorded() {
		isPortion = sym.Facts.Modifiers.Has(symbols.ModPortion)
	} else if usage, ok := sym.Decl.(*ast.Usage); ok {
		isPortion = usage.IsPortion || usage.Portion != ast.PortionNone
	} else {
		return false
	}
	if owner == nil || !m.conformsByName(owner, "Occurrences::Occurrence") ||
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
	case *ast.RequireMember, *ast.AssumeMember:
		return !m.isKerMLDoc(sym) && m.UsageMayTimeVary(sym)
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
	if usage == nil || usage.IsReference || usage.Direction != ast.DirNone || usage.IsBodyParameter ||
		usage.IsEnd || usage.IsEvent || usage.Kind == ast.UsageEnumeration || usage.IsVariantReference() {
		return false
	}
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelReferences {
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
