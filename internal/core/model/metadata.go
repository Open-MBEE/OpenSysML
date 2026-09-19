package model

import (
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// MetadataBodyRedefines returns the metadata-definition feature a metadata
// annotation body declaration implicitly redefines (KerML 7.4.7), with the
// feature's qualified name. ok is false when sym is not such a declaration or
// the annotation's metaclass does not resolve.
func (w *Workspace) MetadataBodyRedefines(sym *symbols.Symbol) (*symbols.Symbol, string, bool) {
	if sym == nil || sym.OwnerScope == nil {
		return nil, "", false
	}
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok {
		return nil, "", false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	var target *symbols.Symbol
	w.queryLocked(sym.DocName, func(resolver *resolve.Resolver, sem *semantics.Model) {
		if owner := resolver.MetadataBodyOwner(sym.OwnerScope); owner != nil {
			target = symbols.MetadataBodyTarget(sem, owner, usage.Ident)
		}
	})
	if target == nil {
		return nil, "", false
	}
	return target, symbols.FQNOf(target), true
}

// EnclosingMetadataBody returns the nearest scope, from scope outward, whose
// declarations redefine the features of a metadata type; nil outside one.
func (w *Workspace) EnclosingMetadataBody(scope *symbols.Scope) *symbols.Scope {
	w.mu.Lock()
	defer w.mu.Unlock()
	var body *symbols.Scope
	w.queryLocked(symbols.DocNameOf(scope), func(resolver *resolve.Resolver, _ *semantics.Model) {
		for ; scope != nil; scope = scope.Parent() {
			if resolver.MetadataBodyOwner(scope) != nil {
				body = scope
				return
			}
		}
	})
	return body
}

// MetadataBodyMembers returns the members of the metadata definition an
// annotation body's declarations redefine, as seen from the body scope, or nil
// when scope is not a metadata annotation body or its metaclass does not
// resolve.
func (w *Workspace) MetadataBodyMembers(scope *symbols.Scope) []*symbols.Symbol {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []*symbols.Symbol
	w.queryLocked(symbols.DocNameOf(scope), func(resolver *resolve.Resolver, sem *semantics.Model) {
		if owner := resolver.MetadataBodyOwner(scope); owner != nil {
			out = w.memberSymbolsLocked(resolver, sem, scope, owner)
		}
	})
	return out
}
