package resolve

import (
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// MetadataBodyOwner returns the type whose features a metadata body implicitly
// redefines (KerML 7.4.7): the metadata type of `@M { … }` / `metadata m : M { … }`,
// or the redefined feature for a body nested in one; nil when it does not resolve.
func (r *Resolver) MetadataBodyOwner(scope *symbols.Scope) *symbols.Symbol {
	if scope == nil {
		return nil
	}
	if scope.BodyLocal() {
		if _, ok := scope.Node().(*ast.PrefixMetadata); !ok {
			return nil
		}
		return r.scopeOwner(scope)
	}
	owner := scope.Owner()
	if owner == nil || owner.OwnerScope == nil {
		return nil
	}
	usage, ok := owner.Decl.(*ast.Usage)
	if !ok {
		return nil
	}
	r.EnterDoc(symbols.DocNameOf(scope))
	defer r.LeaveDoc()
	if def, done := r.bodyOwners[scope]; done {
		return def
	}
	journalNew(r, r.bodyOwners, scope, scope.Node())
	r.bodyOwners[scope] = nil
	var resolved *symbols.Symbol
	if owner.Kind == symbols.SymbolMetadataUsage {
		r.aside(func() {
			if types := r.findTypingTargets(owner.OwnerScope, usage.Relationships); len(types) == 1 {
				resolved = types[0]
			}
		})
	} else {
		resolved = r.metadataBodyRedefined(owner, usage)
	}
	r.bodyOwners[scope] = resolved
	return resolved
}

// metadataBodyRedefined returns the feature a declaration in a metadata body
// redefines: its `:>>` target when it writes one, else the owner's feature of its name.
func (r *Resolver) metadataBodyRedefined(sym *symbols.Symbol, usage *ast.Usage) *symbols.Symbol {
	owner := r.MetadataBodyOwner(sym.OwnerScope)
	if owner == nil {
		return nil
	}
	if len(redefinesRelationships(usage)) > 0 {
		if explicit := r.explicitRedefinitions(sym); len(explicit) > 0 {
			return explicit[0]
		}
		return nil
	}
	if target := symbols.MetadataBodyTarget(r.model, owner, usage.Ident); target != sym {
		return target
	}
	return nil
}

// scopeOwner returns the symbol whose members scope contributes to unqualified
// lookup. A metadata annotation body not yet stamped by document resolution has
// its owner — the metadata definition the annotation names — resolved on
// demand, quietly, and memoized per resolver so shared scopes stay unmutated.
func (r *Resolver) scopeOwner(scope *symbols.Scope) *symbols.Symbol {
	if scope == nil {
		return nil
	}
	if owner := scope.Owner(); owner != nil {
		return owner
	}
	prefix, ok := scope.Node().(*ast.PrefixMetadata)
	if !ok || !scope.BodyLocal() {
		return nil
	}
	r.EnterDoc(symbols.DocNameOf(scope))
	defer r.LeaveDoc()
	if owner, done := r.bodyOwners[scope]; done {
		return owner
	}
	// Break cycles while the metaclass itself resolves.
	journalNew(r, r.bodyOwners, scope, scope.Node())
	r.bodyOwners[scope] = nil
	var resolved *symbols.Symbol
	r.aside(func() {
		owner, ok := r.ResolveQualified(scope.Parent(), prefix.Type)
		if !ok || owner == nil {
			return
		}
		if target, aliasOK := r.ResolveAliasTarget(owner); aliasOK {
			owner = target
		}
		resolved = owner
	})
	r.bodyOwners[scope] = resolved
	return resolved
}
