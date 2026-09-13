package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// endRedefinitionLookup is the part of the semantic model that reports the ends
// a connector or association end implicitly redefines, which a specializing
// association's ends never spell out (KerML 8.4.4.6).
type endRedefinitionLookup interface {
	ImplicitEndRedefinitions(sym *symbols.Symbol) []*symbols.Symbol
	ImplicitRoleRedefinitions(sym *symbols.Symbol) []*symbols.Symbol
}

// nestedInRedefined resolves name among the features nested, at any depth, under
// a feature that an enclosing feature redefines (KerML 8.3.4.4). Reached only
// once ordinary lookup fails, so it shadows nothing.
func (r *Resolver) nestedInRedefined(scope *symbols.Scope, name string, hide *refFilter) (*symbols.Symbol, bool) {
	if name == "" {
		return nil, false
	}
	for s := scope; s != nil; s = s.Parent() {
		owner := s.Owner()
		if owner == nil || !isFeatureDecl(owner.Decl) {
			continue
		}
		for _, redefined := range r.redefinedFeatures(owner) {
			if sym, ok := r.nestedMember(redefined, name, hide); ok {
				return sym, true
			}
		}
	}
	return nil, false
}

// redefinedFeatures returns the features sym redefines, explicitly, implicitly
// in a metadata body, or as an association or connector end.
func (r *Resolver) redefinedFeatures(sym *symbols.Symbol) []*symbols.Symbol {
	if cached, done := r.redefined[sym]; done {
		return cached
	}
	r.redefined[sym] = nil
	out := r.explicitRedefinitions(sym)
	if model, ok := r.model.(endRedefinitionLookup); ok {
		for _, end := range model.ImplicitEndRedefinitions(sym) {
			if end != nil && end != sym {
				out = append(out, end)
			}
		}
		for _, role := range model.ImplicitRoleRedefinitions(sym) {
			if role != nil && role != sym {
				out = append(out, role)
			}
		}
	}
	if usage, ok := sym.Decl.(*ast.Usage); ok {
		if owner := r.MetadataBodyOwner(sym.OwnerScope); owner != nil {
			if target := symbols.MetadataBodyTarget(r.model, owner, usage.Ident); target != nil &&
				target != sym {
				out = append(out, target)
			}
		}
	}
	r.redefined[sym] = out
	return out
}

// explicitRedefinitions returns the features the `:>>` clauses of sym's
// declaration resolve to.
func (r *Resolver) explicitRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, rel := range redefinesRelationships(sym.Decl) {
		if found, ok := r.ResolveRedefinitionTarget(sym.OwnerScope, sym.Decl, rel.Target); ok && found != sym {
			out = append(out, found)
		}
	}
	return out
}

// nestedMember resolves name among the features nested under sym, at any depth,
// skipping what hide covers.
func (r *Resolver) nestedMember(sym *symbols.Symbol, name string, hide *refFilter) (*symbols.Symbol, bool) {
	seen := map[*symbols.Symbol]bool{}
	queue := []*symbols.Symbol{sym}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == nil || seen[cur] {
			continue
		}
		seen[cur] = true
		if member, ok := r.lookupMember(cur, name, hide); ok && !hide.hides(member) {
			return member, true
		}
		var members []*symbols.Symbol
		if cur.Scope != nil {
			members = append(members, cur.Scope.Members()...)
		}
		if r.idx != nil {
			members = append(members, r.idx.LookupDirectChildren(r.indexedNameOf(cur))...)
		}
		for _, member := range members {
			if member.Name == name && !hide.hides(member) {
				return member, true
			}
			queue = append(queue, member)
		}
	}
	return nil, false
}

// redefinesRelationships returns decl's explicit redefinitions.
func redefinesRelationships(decl ast.Node) []*ast.Relationship {
	var rels []*ast.Relationship
	if oc, ok := ast.OwnedConstraintOf(decl); ok {
		rels = oc.Relationships
	} else {
		switch d := decl.(type) {
		case *ast.Usage:
			rels = d.Relationships
		case *ast.Definition:
			rels = d.Relationships
		case *ast.CrossFeatureMember:
			rels = d.Relationships
		case *ast.SubjectMember:
			rels = d.Relationships
		default:
			return nil
		}
	}
	var out []*ast.Relationship
	for _, rel := range rels {
		if rel != nil && rel.Kind == ast.RelRedefines {
			out = append(out, rel)
		}
	}
	return out
}

// isFeatureDecl reports whether decl declares a feature rather than a type.
func isFeatureDecl(decl ast.Node) bool {
	if _, ok := ast.OwnedConstraintOf(decl); ok {
		return true
	}
	switch decl.(type) {
	case *ast.Usage, *ast.SubjectMember, *ast.CrossFeatureMember:
		return true
	}
	return false
}
