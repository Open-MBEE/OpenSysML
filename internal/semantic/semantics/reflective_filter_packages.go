package semantics

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// filterPackageParts are the import a filter package owns, as written, and the
// ElementFilterMembership owning its condition.
type filterPackageParts struct {
	inner, membership *symbols.Symbol
}

// filterPackageOf is the Package an import with a filter, `import X::*[c]`, imports and
// owns (KerML 8.2.5.4 Import; SysML.xtext FilterPackage): it owns the import as written
// and a private ElementFilterMembership whose condition is c. Nil for any other import.
func (m *Model) filterPackageOf(imp *symbols.Symbol) *symbols.Symbol {
	if m == nil || imp == nil || imp.Implicit == nil || imp.Implicit.Form != symbols.ImplicitImport {
		return nil
	}
	decl, _ := imp.Implicit.Decl.(*ast.Import)
	namespace := imp.Implicit.Owner
	if decl == nil || decl.FilterExpr == nil || namespace == nil || IsFilterPackage(namespace) {
		return nil
	}
	if cached, ok := m.filterPackages[imp]; ok {
		return cached
	}
	pkg := &symbols.Symbol{
		Kind:       symbols.SymbolPackage,
		DocName:    imp.DocName,
		Decl:       decl,
		DeclSpan:   imp.DeclSpan,
		Visibility: ast.VisibilityPrivate,
		Expr:       &symbols.ExpressionElement{Owner: namespace, Role: symbols.ExpressionFilterPackage},
	}
	if m.filterPackages == nil {
		m.filterPackages = make(map[*symbols.Symbol]*symbols.Symbol)
		m.filterPackageImports = make(map[*symbols.Symbol]*symbols.Symbol)
	}
	m.filterPackages[imp] = pkg
	m.filterPackageImports[pkg] = imp
	return pkg
}

// IsFilterPackage reports whether sym is the Package an import with a filter imports.
func IsFilterPackage(sym *symbols.Symbol) bool {
	return sym != nil && sym.Expr != nil && sym.Expr.Role == symbols.ExpressionFilterPackage
}

// FilterPackageImport is the Import that imports and owns the filter package pkg.
func (m *Model) FilterPackageImport(pkg *symbols.Symbol) *symbols.Symbol {
	if m == nil || !IsFilterPackage(pkg) {
		return nil
	}
	return m.filterPackageImports[pkg]
}

// filterPackagePartsOf synthesizes a filter package's import and filter membership once:
// the import keeps the declaration's kind and recursion, the membership is private and
// owns the condition, which resolves its names where the import is written.
func (m *Model) filterPackagePartsOf(pkg *symbols.Symbol) filterPackageParts {
	imp := m.FilterPackageImport(pkg)
	if imp == nil {
		return filterPackageParts{}
	}
	if cached, ok := m.filterPackageMembers[pkg]; ok {
		return cached
	}
	decl := imp.Implicit.Decl.(*ast.Import)
	parts := filterPackageParts{
		inner: &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    pkg.DocName,
			DeclSpan:   imp.DeclSpan,
			Visibility: decl.Visibility,
			Implicit:   &symbols.ImplicitRelationship{Owner: pkg, Form: symbols.ImplicitImport, Decl: decl},
		},
		membership: &symbols.Symbol{
			Kind:       symbols.SymbolRelationship,
			DocName:    pkg.DocName,
			DeclSpan:   decl.FilterExpr.Span(),
			Visibility: ast.VisibilityPrivate,
			Implicit:   &symbols.ImplicitRelationship{Owner: pkg, Form: symbols.ImplicitMembership, Decl: decl},
		},
	}
	parts.membership.Implicit.Member = &symbols.Symbol{
		Kind:       symbols.SymbolExpression,
		Decl:       decl.FilterExpr,
		DeclSpan:   decl.FilterExpr.Span(),
		DocName:    pkg.DocName,
		Visibility: ast.VisibilityPrivate,
		OwnerScope: pkg.Expr.Owner.Scope,
		Expr:       &symbols.ExpressionElement{Owner: parts.membership, Role: symbols.ExpressionOperand, Slot: "filter"},
	}
	if m.filterPackageMembers == nil {
		m.filterPackageMembers = make(map[*symbols.Symbol]filterPackageParts)
	}
	m.filterPackageMembers[pkg] = parts
	return parts
}
