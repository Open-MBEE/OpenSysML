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
		return !m.isKerMLDoc(sym) && sym.Facts.Node == symbols.NodeUsage &&
			sym.Facts.Modifiers.Has(symbols.ModMayTimeVary)
	}
	if _, ok := sym.Decl.(*ast.Usage); !ok || sym.OwnerScope == nil {
		return false
	}
	return m.UsageMayTimeVaryForOwningType(sym, sym.OwnerScope.Owner())
}

// UsageMayTimeVaryForOwningType derives Usage::mayTimeVary for the owning type
// stated by Structure, or nil when the usage has no owning FeatureMembership.
func (m *Model) UsageMayTimeVaryForOwningType(sym, owner *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return false
	}
	if owner == nil || !m.conformsByName(owner, "Occurrences::Occurrence") ||
		usage.IsPortion || usage.Portion != ast.PortionNone ||
		m.conformsByName(sym, "Links::SelfLink") ||
		m.conformsByName(sym, "Occurrences::HappensLink") {
		return false
	}
	return !m.UsageIsComposite(sym) || !m.conformsByName(sym, "Actions::Action")
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

// UsageIsComposite reports the syntax-derived composite status without the
// model constraints that require resolver access.
func UsageIsComposite(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if sym.Recorded() {
		return sym.Facts.Modifiers.Has(symbols.ModComposite) ||
			!recordedUsageIsReferential(sym)
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usageIsComposite(usage)
}

func (m *Model) UsageIsReferential(sym *symbols.Symbol) bool {
	return sym != nil && !m.UsageIsComposite(sym)
}

// UsageIsComposite reports whether sym declares a composite usage.
func (m *Model) UsageIsComposite(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if isControlNode(sym) {
		return true
	}
	if _, ok := sym.Decl.(*ast.SubstateMember); ok {
		return true
	}
	if sym.Recorded() {
		if m.metaclassConforms(sym, sysmlMetaclassPrefix+"ControlNode") {
			return true
		}
		if sym.Kind.IsAttributeLike() || sym.Kind == symbols.SymbolReferenceUsage ||
			sym.Facts.UsageKind == ast.UsageEnumeration ||
			sym.Facts.Modifiers.Has(symbols.ModEnd|symbols.ModEvent) ||
			sym.Facts.Direction != ast.DirNone {
			return false
		}
		if m.reflectiveAttributeFeatureOwner(sym.Owner()) {
			return false
		}
		if sym.Facts.UsageKind == ast.UsagePort && !m.portOwningType(sym) {
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
			m.reflectiveAttributeFeatureOwner(sym.Owner()) {
			return false
		}
		if m.metaclassConforms(sym, sysmlMetaclassPrefix+"PortUsage") && !m.portOwningType(sym) {
			return false
		}
		return defaultUsageComposite(sym.Kind)
	}
	if m.metaclassConforms(sym, sysmlMetaclassPrefix+"AttributeUsage") ||
		m.metaclassConforms(sym, sysmlMetaclassPrefix+"ReferenceUsage") ||
		m.metaclassConforms(sym, sysmlMetaclassPrefix+"EventOccurrenceUsage") {
		return false
	}
	if m.reflectiveAttributeFeatureOwner(sym.Owner()) {
		return false
	}
	if usage.Kind == ast.UsagePort && !m.portOwningType(sym) {
		return false
	}
	return usageIsComposite(usage)
}

// UsageDeclIsComposite derives SysML Usage::isComposite (SysML v2 §7.6.2) for a usage declaration.
func UsageDeclIsComposite(usage *ast.Usage) bool {
	return usageIsComposite(usage)
}

func usageIsComposite(usage *ast.Usage) bool {
	if usage == nil || usage.IsReference || usage.Direction != ast.DirNone ||
		usage.IsEnd || usage.IsEvent || usage.Kind == ast.UsageAttribute ||
		usage.Kind == ast.UsageEnumeration || usage.IsVariantReference() {
		return false
	}
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelReferences {
			return false
		}
	}
	return true
}

func (m *Model) portOwningType(sym *symbols.Symbol) bool {
	owner := sym.Owner()
	if owner == nil {
		return false
	}
	if owner.Kind == symbols.SymbolPortDef || owner.Kind == symbols.SymbolPortUsage {
		return true
	}
	return m.metaclassConforms(owner, sysmlMetaclassPrefix+"PortDefinition") ||
		m.metaclassConforms(owner, sysmlMetaclassPrefix+"PortUsage")
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
