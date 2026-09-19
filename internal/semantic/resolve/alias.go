package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// AliasedElement is the element a name bound to sym reaches: an alias names an
// existing element rather than being one (KerML 8.2.3.2). An alias whose target
// is unresolvable stays itself, so its own diagnostic still fires.
func (r *Resolver) AliasedElement(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.Kind != symbols.SymbolAlias {
		return sym
	}
	if target, ok := r.ResolveAliasTarget(sym); ok && target != nil {
		return target
	}
	return sym
}

// AliasNamesNothing reports whether the name an alias binds reaches no element,
// because the alias's own target reference is unresolved (KerML 8.2.3.2). A
// cyclic alias is excluded: its target reference does name an element.
func (r *Resolver) AliasNamesNothing(sym *symbols.Symbol) bool {
	if sym == nil || sym.Kind != symbols.SymbolAlias {
		return false
	}
	al, ok := sym.Decl.(*ast.Alias)
	if !ok || al.For == nil {
		return false
	}
	res, done := r.memo[al.For]
	return done && !res.ok
}

// ResolveAliasTarget follows an alias symbol to its ultimate non-alias target.
// Non-alias symbols resolve to themselves. Cycles yield (nil, false).
func (r *Resolver) ResolveAliasTarget(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	if sym == nil {
		return nil, false
	}
	r.EnterDoc(sym.DocName)
	defer r.LeaveDoc()
	if cached, ok := r.aliasTargets[sym]; ok {
		return cached.sym, cached.ok
	}
	if r.resolvingAlias[sym] {
		return nil, false
	}
	r.resolvingAlias[sym] = true
	defer delete(r.resolvingAlias, sym)

	target, ok := r.resolveAliasTarget(sym)
	journalNew(r, r.aliasTargets, sym, sym.Decl)
	r.aliasTargets[sym] = resolution{sym: target, ok: ok}
	return target, ok
}

func (r *Resolver) resolveAliasTarget(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	seen := map[*symbols.Symbol]bool{}
	cur := sym
	for cur != nil {
		if cur.Kind != symbols.SymbolAlias {
			return cur, true
		}
		if seen[cur] {
			return nil, false
		}
		seen[cur] = true

		next, ok := r.aliasStep(cur)
		if !ok {
			return nil, false
		}
		cur = next
	}
	return nil, false
}

// aliasStep resolves one alias's target, named by a qualified name resolved from
// the alias's own scope.
func (r *Resolver) aliasStep(sym *symbols.Symbol) (*symbols.Symbol, bool) {
	al, ok := sym.Decl.(*ast.Alias)
	if !ok || al.For == nil {
		return nil, false
	}
	return r.ResolveQualified(aliasScope(sym), al.For)
}

// aliasScope returns the scope in which an alias's target should be resolved:
// the alias symbol's enclosing scope (where it was declared). Leaf symbols
// (such as aliases) carry their enclosing scope in OwnerScope; Scope is the
// child scope a declaration owns and is nil for leaves.
func aliasScope(sym *symbols.Symbol) *symbols.Scope {
	return sym.OwnerScope
}
