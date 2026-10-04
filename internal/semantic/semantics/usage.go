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
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || sym.OwnerScope == nil {
		return false
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil || !m.conformsByName(owner, "Occurrences::Occurrence") ||
		usage.IsPortion || usage.Portion != ast.PortionNone ||
		m.conformsByName(sym, "Links::SelfLink") ||
		m.conformsByName(sym, "Occurrences::HappensLink") {
		return false
	}
	return !usageIsComposite(usage) ||
		(usage.Kind != ast.UsageAction && !m.conformsByName(sym, "Actions::Action"))
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

// UsageIsComposite reports whether sym declares a composite usage.
func UsageIsComposite(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && usageIsComposite(usage)
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
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind == ast.RelReferences {
			return false
		}
	}
	return true
}
