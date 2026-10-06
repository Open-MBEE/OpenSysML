package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// featuringProvider is the part of the semantic model that reports the types a
// feature is declared `featured by` and whether one type conforms to another.
// *semantics.Model implements it.
type featuringProvider interface {
	FeaturingTypes(sym *symbols.Symbol) []*symbols.Symbol
	Conforms(a, b *symbols.Symbol) bool
}

// featuredMember reads a chain member no member of sym declares as a feature
// visible where the chain is written (so imported) that is `featured by` a type
// sym conforms to: an extension library's StateActivity::isActive.
func (r *Resolver) featuredMember(scope *symbols.Scope, sym *symbols.Symbol, name string, chain ast.Node) (*symbols.Symbol, bool) {
	if scope == nil || sym == nil || chain == nil {
		return nil, false
	}
	model, ok := r.model.(featuringProvider)
	if !ok {
		return nil, false
	}
	key := featuredKey{scope: scope, name: name}
	if depth := r.featuring[key]; depth != 0 {
		r.CutShort(depth)
		return nil, false
	}
	if r.featuring == nil {
		r.featuring = make(map[featuredKey]int)
	}
	r.featuring[key] = r.Enter()
	var found *symbols.Symbol
	r.aside(func() { found, ok = r.ResolveName(scope, name, nil) })
	delete(r.featuring, key)
	r.Leave()
	if !ok || found == nil || !r.namedThroughNamespace(found) || !r.featuredBy(model, sym, found) {
		return nil, false
	}
	return found, true
}

// featuredKey identifies a featured-member lookup: the name sought as visible
// in a scope.
type featuredKey struct {
	scope *symbols.Scope
	name  string
}

// featuredBy reports whether feature is declared `featured by` a type sym conforms to.
func (r *Resolver) featuredBy(model featuringProvider, sym, feature *symbols.Symbol) bool {
	for _, typ := range model.FeaturingTypes(feature) {
		if typ != nil && model.Conforms(sym, typ) {
			return true
		}
	}
	return false
}

// FeaturedMembersOf lists the features visible in scope that a feature chain
// written there reads as members of sym through their `featured by` (see
// featuredMember), for completion on the chain.
func (r *Resolver) FeaturedMembersOf(scope *symbols.Scope, sym *symbols.Symbol) []*symbols.Symbol {
	model, ok := r.model.(featuringProvider)
	if !ok || scope == nil || sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{}
	consider := func(cand *symbols.Symbol) {
		if cand == nil || seen[cand] {
			return
		}
		seen[cand] = true
		if r.namedThroughNamespace(cand) && r.featuredBy(model, sym, cand) {
			out = append(out, cand)
		}
	}
	for s := scope; s != nil; s = s.Parent() {
		for _, member := range s.Members() {
			consider(member)
		}
		for _, imp := range r.scopeImports(s) {
			for _, cand := range r.importedSymbols(s, imp) {
				consider(cand)
			}
		}
	}
	return out
}

// importedSymbols are the elements imp brings into scope: the member a
// membership import names, or the visible members of the namespace a namespace
// import names.
func (r *Resolver) importedSymbols(scope *symbols.Scope, imp *ast.Import) []*symbols.Symbol {
	if imp == nil || imp.Imported == nil || r.idx == nil {
		return nil
	}
	var target *symbols.Symbol
	var ok bool
	r.aside(func() { target, ok = r.ResolveQualified(scope, imp.Imported) })
	if !ok || target == nil {
		return nil
	}
	if imp.Kind == ast.ImportMembership {
		return []*symbols.Symbol{target}
	}
	fqn := r.idx.GetFQN(target)
	return r.AdmittedChildrenOf(scope, fqn, r.idx.LookupDirectChildrenFrom(fqn, r.ReferringNamespaceFQN(scope)))
}
