package resolve

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// importAllowsPrivate reports whether an import re-exports private members.
// Only `import all` widens visibility to include private members.
func importAllowsPrivate(imp *ast.Import) bool {
	return imp.IsAll
}

// visibleThroughImport reports whether imp surfaces sym: an import takes its
// target's visible memberships, and only `import all` takes the rest.
func visibleThroughImport(imp *ast.Import, sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	return importAllowsPrivate(imp) || symbols.VisibleOutside(sym.Visibility)
}

// namedThroughNamespace reports whether sym may be named as a member of the
// namespace a qualified name or feature chain reached: visible resolution reads
// only that namespace's public memberships, whatever the referring namespace
// specializes, except under an `import all` (KerML 8.2.3.5.3).
func (r *Resolver) namedThroughNamespace(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	return r.allVisible > 0 || symbols.VisibleOutside(sym.Visibility)
}

func (r *Resolver) namedThroughNamespaces(cands []*symbols.Symbol) []*symbols.Symbol {
	kept := make([]*symbols.Symbol, 0, len(cands))
	for _, sym := range cands {
		if r.namedThroughNamespace(sym) {
			kept = append(kept, sym)
		}
	}
	return kept
}

// visibleAsInheritedMember reports whether found, reached as a member of owner,
// is visible inside owner: what owner declares always is, what it inherits is
// unless private (KerML 8.2.3.5.2).
func visibleAsInheritedMember(owner, found *symbols.Symbol) bool {
	if found == nil {
		return false
	}
	if owner != nil && owner.Scope != nil && found.OwnerScope == owner.Scope {
		return true
	}
	return symbols.VisibleAs(found.Visibility, false, true)
}

// inheritedThroughSpecialization reports whether imp reaches the bodies that
// specialize the definition or usage declaring it: a protected or public one
// does (SysML v2 7.5.3), a private membership does not (KerML 8.2.3.3).
func inheritedThroughSpecialization(imp *ast.Import) bool {
	return imp.Visibility != ast.VisibilityPrivate
}

func (r *Resolver) specializes(from, target *symbols.Symbol) bool {
	if from == nil || target == nil {
		return false
	}
	if from == target {
		return true
	}
	for _, sup := range r.specializationChain(from) {
		if sup == target {
			return true
		}
	}
	return false
}

func (r *Resolver) specializationChain(from *symbols.Symbol) []*symbols.Symbol {
	if from == nil {
		return nil
	}
	model, ok := r.model.(supertypeProvider)
	if !ok {
		return nil
	}
	seen := map[*symbols.Symbol]bool{from: true}
	queue := model.DirectSupertypes(from)
	var chain []*symbols.Symbol
	for len(queue) > 0 {
		sup := queue[0]
		queue = queue[1:]
		if sup == nil || seen[sup] {
			continue
		}
		seen[sup] = true
		chain = append(chain, sup)
		queue = append(queue, model.DirectSupertypes(sup)...)
	}
	return chain
}

// lookupInheritedImports resolves name through the imports declared by what
// scope's owner specializes, walking the specialization graph upward: inside
// `part def Sub :> Base` it consults Base's protected imports. A feature typing
// is a generalization edge (KerML 8.3.4.6), so `part p : Base` reaches them too.
func (r *Resolver) lookupInheritedImports(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	var found *symbols.Symbol
	r.eachInheritedImport(scope, name, func(from *symbols.Scope, imp *ast.Import) bool {
		found, _ = r.matchImportInto(scope, from, imp, name)
		return found == nil
	})
	return found, found != nil
}

// inheritedImportCandidates is lookupInheritedImports collecting every declaration the
// inherited imports surface under name, in the order lookupInheritedImports searches.
func (r *Resolver) inheritedImportCandidates(scope *symbols.Scope, name string) []*symbols.Symbol {
	var out []*symbols.Symbol
	r.eachInheritedImport(scope, name, func(from *symbols.Scope, imp *ast.Import) bool {
		for _, sym := range r.importMatchesAllInto(scope, from, imp, name) {
			out = appendSymbol(out, sym)
		}
		return true
	})
	return out
}

// eachInheritedImport calls yield with each import that may surface name to scope
// through what its owner specializes, until yield returns false.
func (r *Resolver) eachInheritedImport(scope *symbols.Scope, name string, yield func(*symbols.Scope, *ast.Import) bool) {
	owner := scope.Owner()
	if owner == nil || r.inheritedImports[owner] {
		return
	}
	if _, ok := r.model.(supertypeProvider); !ok {
		return
	}
	// Resolving an inherited import's target resolves names again, which may
	// arrive back here: one visit per owner per lookup.
	r.inheritedImports[owner] = true
	defer delete(r.inheritedImports, owner)

	for _, sup := range r.specializationChain(owner) {
		if sup.Scope == nil {
			continue
		}
		for _, imp := range importsOf(sup.Scope.Node()) {
			if !inheritedThroughSpecialization(imp) || !r.importPrefixAvailable(sup.Scope, imp, name) {
				continue
			}
			if !yield(sup.Scope, imp) {
				return
			}
		}
	}
}

// ImportTarget is the namespace or membership an import names, as the import
// itself reaches it.
func (r *Resolver) ImportTarget(scope *symbols.Scope, imp *ast.Import) (*symbols.Symbol, bool) {
	return r.resolveImportTarget(scope, imp)
}

// resolveImportTarget resolves what an import names; an `import all` (an expose
// is one) reaches a membership its target would otherwise hide.
func (r *Resolver) resolveImportTarget(scope *symbols.Scope, imp *ast.Import) (*symbols.Symbol, bool) {
	if imp == nil || imp.Imported == nil {
		return nil, false
	}
	return r.resolveImportName(scope, imp.Imported, imp)
}

// resolveImportName resolves qn as the target of imp, written in scope. The
// import does not surface members while its own target is looked up.
func (r *Resolver) resolveImportName(scope *symbols.Scope, qn *ast.QualifiedName, imp *ast.Import) (*symbols.Symbol, bool) {
	if !r.resolvingImports[imp] {
		r.resolvingImports[imp] = true
		defer delete(r.resolvingImports, imp)
	}
	var sym *symbols.Symbol
	var ok bool
	if !importAllowsPrivate(imp) {
		// A plain import keeps the boundary even when it is consulted while an
		// enclosing `import all` resolves its own target.
		r.outsideAllVisible(func() { sym, ok = r.ResolveQualified(scope, qn) })
		return sym, ok
	}
	r.inAllVisible(func() { sym, ok = r.ResolveQualified(scope, qn) })
	return sym, ok
}

// importSurfaces reports whether imp takes sym from the namespace named
// targetFQN: its visible memberships, plus, under `import all`, the ones it
// declares privately — but not a hidden one it merely imported itself.
func (r *Resolver) importSurfaces(imp *ast.Import, targetFQN string, sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	if symbols.VisibleOutside(sym.Visibility) {
		return true
	}
	return importAllowsPrivate(imp) && r.declaresMember(targetFQN, sym)
}

// declaresMember reports whether the namespace named fqn is where sym is
// declared, as opposed to a namespace an import surfaced it in.
func (r *Resolver) declaresMember(fqn string, sym *symbols.Symbol) bool {
	if r.idx == nil || fqn == "" {
		return false
	}
	declared := r.idx.GetFQN(sym)
	i := strings.LastIndex(declared, "::")
	return i > 0 && declared[:i] == fqn
}
