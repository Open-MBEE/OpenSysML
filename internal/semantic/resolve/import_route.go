package resolve

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// The imported memberships of a namespace are computed with the namespaces
// already being read excluded (KerML 8.3.2.4.5 Namespace::importedMemberships,
// 8.3.2.4.6 NamespaceImport::importedMemberships): an import of an excluded
// namespace brings nothing. What a namespace's imports bring therefore depends
// on the route that reached it, and so does every answer kept along the way.

// routeDeps is what an import read depended on of the route that made it: the
// excluded namespaces it was cut at, and the namespaces it entered.
type routeDeps struct {
	cut, entered []*symbols.Scope
}

// routeMark is a point in the record of cuts and entries (see routeDepsSince).
type routeMark struct {
	hits, seen int
}

// reading runs f with the namespace owning scope excluded, as the computation
// of its imported memberships excludes it.
func (r *Resolver) reading(scope *symbols.Scope, f func()) {
	if scope == nil || slices.Contains(r.importPath, scope) {
		f()
		return
	}
	r.importPath = append(r.importPath, scope)
	defer func() {
		r.importPath = r.importPath[:len(r.importPath)-1]
		r.forgetRoute()
	}()
	f()
}

// entersNamespace reports whether a namespace import may read the namespace
// owning scope, which it may not while that namespace is excluded.
func (r *Resolver) entersNamespace(scope *symbols.Scope) bool {
	if scope == nil {
		return true
	}
	if slices.Contains(r.importPath, scope) {
		r.importHits = append(r.importHits, scope)
		return false
	}
	r.importSeen = append(r.importSeen, scope)
	return true
}

// freshImportRoute runs f as the read of a namespace in its own right: no
// namespace is excluded and no import is in progress, whatever route asked.
// The caller guards against asking again for what it is computing.
func (r *Resolver) freshImportRoute(f func()) {
	stack := r.importStack
	r.importStack = map[*ast.Import]bool{}
	defer func() { r.importStack = stack }()
	r.freshImportVisits(f)
}

// excludedNamespaces names the namespaces being read, as the index names them.
func (r *Resolver) excludedNamespaces() []string {
	var out []string
	for _, scope := range r.importPath {
		if fqn := r.namespaceFQNOf(scope); fqn != "" {
			out = append(out, fqn)
		}
	}
	return out
}

// markRoute starts recording what the reads that follow depend on; each mark
// is closed by one routeDepsSince.
func (r *Resolver) markRoute() routeMark {
	r.importMarks++
	return routeMark{hits: len(r.importHits), seen: len(r.importSeen)}
}

// routeDepsSince closes mark: the excluded namespaces still being read that the
// reads since were cut at, self apart, and the namespaces they entered.
func (r *Resolver) routeDepsSince(mark routeMark, self *symbols.Scope) routeDeps {
	var deps routeDeps
	for _, scope := range r.importHits[mark.hits:] {
		if scope != self && slices.Contains(r.importPath, scope) && !slices.Contains(deps.cut, scope) {
			deps.cut = append(deps.cut, scope)
		}
	}
	deps.entered = distinctScopes(r.importSeen[mark.seen:])
	r.importMarks--
	r.forgetRoute()
	return deps
}

// forgetRoute drops the record once no read is in progress to depend on it.
func (r *Resolver) forgetRoute() {
	if r.importMarks == 0 && len(r.importPath) == 0 {
		r.importHits, r.importSeen = r.importHits[:0], r.importSeen[:0]
	}
}

// routeHolds reports whether an answer with these dependencies is the answer on
// the route in progress: one that found nothing stays empty wherever its cuts
// recur, and a set of members also needs every namespace it entered readable.
func (r *Resolver) routeHolds(deps routeDeps, members bool) bool {
	for _, scope := range deps.cut {
		if !slices.Contains(r.importPath, scope) {
			return false
		}
	}
	if members {
		for _, scope := range r.importPath {
			if slices.Contains(deps.entered, scope) {
				return false
			}
		}
	}
	return true
}

// replayRoute makes the reads in progress depend on what a reused answer did.
func (r *Resolver) replayRoute(deps routeDeps) {
	r.importHits = append(r.importHits, deps.cut...)
	r.importSeen = append(r.importSeen, deps.entered...)
	r.forgetRoute()
}

func distinctScopes(scopes []*symbols.Scope) []*symbols.Scope {
	if len(scopes) == 0 {
		return nil
	}
	seen := make(map[*symbols.Scope]struct{}, len(scopes))
	out := make([]*symbols.Scope, 0, len(scopes))
	for _, scope := range scopes {
		if _, dup := seen[scope]; !dup {
			seen[scope] = struct{}{}
			out = append(out, scope)
		}
	}
	return out
}
