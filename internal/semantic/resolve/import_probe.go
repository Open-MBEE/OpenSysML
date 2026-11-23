package resolve

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// importProbe is one walk over the imports reachable from a namespace, each
// read once, for whether any namespace among them could hold a name at all.
type importProbe struct {
	seen map[importVisit]bool
	held bool
}

// importsHold reports whether any namespace targetScope's imports reach holds a member under visit's
// name, before any filter or hiding: a route reads a part of those namespaces, so finds no more.
func (r *Resolver) importsHold(target *symbols.Symbol, targetScope, from *symbols.Scope, visit importVisit) bool {
	probe := &importProbe{seen: map[importVisit]bool{}}
	r.holdProbe = probe
	defer func() { r.holdProbe = nil }()
	r.probeImports(target, targetScope, from, visit)
	return probe.held
}

// probeImports is lookupImportedMember for a probe: the imports a search would
// follow, whichever of them are in progress.
func (r *Resolver) probeImports(target *symbols.Symbol, targetScope, from *symbols.Scope, visit importVisit) {
	probe := r.holdProbe
	if probe.held || probe.seen[visit] {
		return
	}
	probe.seen[visit] = true
	for _, imp := range r.scopeImports(targetScope) {
		if !r.importPrefixAvailable(targetScope, imp, visit.name) {
			continue
		}
		if !visit.all && !r.importVisibleFrom(target, from, imp) {
			continue
		}
		r.probeImport(targetScope, imp, visit.name)
		if probe.held {
			return
		}
	}
}

// probeImport is eachMatchOfImport for a probe: it notes whether imp, declared
// in scope, could surface a member under name, without admitting or hiding any.
func (r *Resolver) probeImport(scope *symbols.Scope, imp *ast.Import, name string) {
	probe := r.holdProbe
	if imp.Imported == nil || len(imp.Imported.Parts) == 0 || r.resolvingImports[imp] {
		return
	}
	target, ok := r.importTargetOf(scope, imp)
	if !ok {
		return
	}
	if imp.Kind == ast.ImportMembership {
		if alias, isAlias := r.PartAlias(imp.Imported, len(imp.Imported.Parts)-1); isAlias {
			target = alias
		}
		targetName := target.Name
		if idx := strings.LastIndex(targetName, "::"); idx >= 0 {
			targetName = targetName[idx+2:]
		}
		// What a recursive import's subtree holds is left to the search.
		probe.held = imp.IsRecursive || targetName == name || r.shortNameOf(target) == name
		return
	}
	if imp.IsRecursive {
		probe.held = true
		return
	}
	if !r.entersNamespace(target.Scope, imp) {
		return
	}
	if target.Scope != nil {
		if len(target.Scope.LookupLocalAll(name)) > 0 || len(r.shortNamedMembers(target.Scope, name, nil)) > 0 {
			probe.held = true
			return
		}
		r.probeImports(target, target.Scope, importReader(imp, scope), importVisit{target: target.Scope, from: importReader(imp, scope), name: name, all: importAllowsPrivate(imp)})
		if probe.held {
			return
		}
	}
	if r.idx != nil {
		targetFQN := r.registeredFQN(target)
		probe.held = len(r.idx.LookupDirectChildrenNamedFrom(targetFQN, importOrigin(imp, targetFQN), name)) > 0
	}
}
