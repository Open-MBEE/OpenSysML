package resolve

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// routeDeps is what an import read depended on of its route (KerML 8.3.2.4.5,
// 8.3.2.4.6): the excluded namespaces it was cut at, and the ones it entered.
type routeDeps struct {
	cut     []*symbols.Scope
	entered []readNamespace
}

// readNamespace is a namespace an import read: all of its memberships or the visible
// ones, with its owned namespaces or not (KerML 8.3.2.4.5 visibleMemberships).
type readNamespace struct {
	scope          *symbols.Scope
	all, recursive bool
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
	gen := r.routeGen
	r.routeSeq++
	r.routeGen = r.routeSeq
	defer func() {
		r.routeGen = gen
		r.importPath = r.importPath[:len(r.importPath)-1]
		r.importNames = r.importNames[:min(len(r.importNames), len(r.importPath))]
		r.forgetRoute()
	}()
	f()
}

// entersNamespace reports whether imp may read the namespace owning scope,
// which it may not while that namespace is excluded.
func (r *Resolver) entersNamespace(scope *symbols.Scope, imp *ast.Import) bool {
	if scope == nil {
		return true
	}
	if slices.Contains(r.importPath, scope) {
		r.importHits = append(r.importHits, scope)
		return false
	}
	r.readsNamespace(scope, imp)
	return true
}

// readsNamespace records that imp reads the namespace owning scope.
func (r *Resolver) readsNamespace(scope *symbols.Scope, imp *ast.Import) {
	if scope == nil {
		return
	}
	read := readNamespace{scope: scope, all: importAllowsPrivate(imp), recursive: imp.IsRecursive}
	r.importSeen = append(r.importSeen, read)
	if fix := r.fixpoint(); fix != nil {
		fix.enter(read)
	}
}

// freshImportRoute runs f as the read of a namespace in its own right: no
// namespace is excluded and no import is in progress, whatever route asked.
func (r *Resolver) freshImportRoute(f func()) {
	stack := r.importStack
	r.importStack = map[*ast.Import]bool{}
	defer func() { r.importStack = stack }()
	r.freshImportVisits(f)
}

// noNamespace stands in the excluded namespaces for a scope the index does
// not name: no namespace has it for a name, so it excludes none.
const noNamespace = "\x00"

// excludedNamespaces names the namespaces being read, as the index names them.
// The result is valid until the read that asked for it returns.
func (r *Resolver) excludedNamespaces() []string {
	for _, scope := range r.importPath[len(r.importNames):] {
		fqn := r.namespaceFQNOf(scope)
		if fqn == "" {
			fqn = noNamespace
		}
		r.importNames = append(r.importNames, fqn)
	}
	return r.importNames
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
	deps.entered = distinct(r.importSeen[mark.seen:])
	// The enclosing reads depend on the same namespaces, each recorded once.
	r.importSeen = append(r.importSeen[:mark.seen], deps.entered...)
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

// routeHolds reports whether an answer is the route in progress's too: its cuts
// recur and, for a set of members, every namespace it entered is readable.
func (r *Resolver) routeHolds(deps routeDeps, members bool) bool {
	for _, scope := range deps.cut {
		if !slices.Contains(r.importPath, scope) {
			return false
		}
	}
	if members {
		for _, read := range deps.entered {
			if slices.Contains(r.importPath, read.scope) {
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
	if fix := r.fixpoint(); fix != nil {
		for _, read := range deps.entered {
			fix.enter(read)
		}
	}
	r.forgetRoute()
}

func distinct[T comparable](scopes []T) []T {
	if len(scopes) == 0 {
		return nil
	}
	out := make([]T, 0, len(scopes))
	if len(scopes) <= 8 {
		for _, scope := range scopes {
			if !slices.Contains(out, scope) {
				out = append(out, scope)
			}
		}
		return out
	}
	seen := make(map[T]struct{}, len(scopes))
	for _, scope := range scopes {
		if _, dup := seen[scope]; !dup {
			seen[scope] = struct{}{}
			out = append(out, scope)
		}
	}
	return out
}
