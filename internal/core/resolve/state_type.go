package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// TypeDecl returns the declaration a type name reaches and the scope of that
// declaration's body, which is what a usage typed by it inherits its content
// from. It reports nothing itself: the tier reported the name already.
func (r *Resolver) TypeDecl(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, *symbols.Scope, bool) {
	var sym *symbols.Symbol
	var ok bool
	r.aside(func() { sym, ok = r.ResolveQualified(scope, qn) })
	if !ok || sym == nil || sym.Scope == nil {
		return nil, nil, false
	}
	return sym.Decl, sym.Scope, true
}

// RedefinitionTarget returns the feature a redefinition owned by decl, declared in
// scope, targets — an alias followed to what it names. It reports nothing itself.
func (r *Resolver) RedefinitionTarget(scope *symbols.Scope, decl ast.Node, target ast.Node) (*symbols.Symbol, bool) {
	var sym *symbols.Symbol
	var ok bool
	r.aside(func() {
		sym, ok = r.ResolveRedefinitionTarget(scope, decl, target)
		if alias, aliasOK := r.ResolveAliasTarget(sym); ok && aliasOK {
			sym = alias
		}
	})
	return sym, ok && sym != nil
}

// LibraryFeature reports whether sym is declared by bundled library content, so a
// model's own declaration under a library name is not mistaken for the library's.
func (r *Resolver) LibraryFeature(sym *symbols.Symbol) bool {
	return r.idx != nil && r.idx.Library(sym)
}

// TypeDeclInScope is TypeDecl from the scope tree alone, for a machine lowered
// without a resolver over its document.
func TypeDeclInScope(scope *symbols.Scope, qn *ast.QualifiedName) (ast.Node, *symbols.Scope, bool) {
	sym, ok := lookupScopeQualified(scope, qn)
	if !ok || sym == nil || sym.Scope == nil {
		return nil, nil, false
	}
	return sym.Decl, sym.Scope, true
}

// inheritedVertex finds the vertex a body inherits under name from the types it
// is typed by or specializes, for a name its own scope does not hold.
func inheritedVertex(body *symbols.Scope, name string, seen map[*symbols.Symbol]bool) (*symbols.Symbol, bool) {
	if body == nil {
		return nil, false
	}
	var rels []*ast.Relationship
	switch n := body.Node().(type) {
	case *ast.Definition:
		rels = n.Relationships
	case *ast.Usage:
		rels = n.Relationships
	default:
		return nil, false
	}
	for _, rel := range rels {
		if rel == nil || (rel.Kind != ast.RelSpecializes && rel.Kind != ast.RelTyping &&
			rel.Kind != ast.RelSubsets && rel.Kind != ast.RelRedefines) {
			continue
		}
		target := rel.Target
		if fr, ok := target.(*ast.FeatureReference); ok {
			target = fr.Name
		}
		qn, ok := target.(*ast.QualifiedName)
		if !ok {
			continue
		}
		super, ok := lookupScopeQualified(body.Parent(), qn)
		if !ok || super == nil || seen[super] {
			continue
		}
		seen[super] = true
		superBody := super.Scope
		if superBody == nil {
			continue
		}
		if member, found := superBody.LookupLocal(name); found && IsVertex(member.Decl) {
			return member, true
		}
		if member, found := inheritedVertex(superBody, name, seen); found {
			return member, true
		}
	}
	return nil, false
}

// inheritedVertexInScope finds the vertex an endpoint names among the members a
// declaration in scope inherits: `nested.i1` where the state `nested` is typed
// by a definition declaring `i1`, or a plain `i1` inherited by the body the
// endpoint is written in.
func inheritedVertexInScope(scope *symbols.Scope, parts []string) (*symbols.Symbol, bool) {
	if scope == nil || len(parts) == 0 {
		return nil, false
	}
	name := parts[len(parts)-1]
	if len(parts) > 1 {
		qualifier, ok := lookupScopePartsText(scope, parts[:len(parts)-1])
		if !ok || qualifier == nil {
			return nil, false
		}
		return inheritedVertex(qualifier.Scope, name, make(map[*symbols.Symbol]bool))
	}
	machine := machineScope(scope)
	for s := scope; s != nil; s = s.Parent() {
		if sym, ok := inheritedVertex(s, name, make(map[*symbols.Symbol]bool)); ok {
			return sym, true
		}
		if s == machine {
			break
		}
	}
	return nil, false
}
