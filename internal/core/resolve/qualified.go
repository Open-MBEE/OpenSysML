package resolve

import (
	"fmt"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/quickfix"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// walkQualified resolves a qualified name segment-by-segment, storing each
// segment's resolved symbol in the resolver's side table.
// hide, when set, makes the bindings of a reference subsetting's own borrowed
// name invisible to the first segment's lookup.
func (r *Resolver) walkQualified(scope *symbols.Scope, qn *ast.QualifiedName, hide *refFilter) resolution {
	if len(qn.Parts) == 0 {
		return resolution{nil, false}
	}

	// Single-segment qualified names (like type references) should use full
	// unqualified lookup (including imports), not just outward scope lookup.
	// Fall back to normal qualified lookup if scope is nil.
	if len(qn.Parts) == 1 && !qn.Global && scope != nil {
		res := r.walkUnqualifiedHiding(scope, qn.Parts[0].Text, hide)
		if res.ok {
			res.sym = r.resolvedPart(qn, 0, res.sym)
		}
		if !res.ok || r.AliasNamesNothing(res.sym) {
			r.unresolved(scope, qn)
			return resolution{nil, false}
		}
		return res
	}

	// Resolve the first segment. A non-global name first searches the enclosing
	// scope chain; a global ($::) name starts at the document root. When the
	// local scope tree has no match, fall back to the global qualified-name
	// index so top-level names declared in other documents resolve.
	//
	// For non-global names, use import-aware unqualified lookup so that
	// multi-segment names like TrafficLightColor::green can resolve the first
	// segment via wildcard imports (e.g., import Def1::*).
	first := qn.Parts[0].Text
	var cur *symbols.Symbol
	if qn.Global {
		cur = r.lookupInRoot(scope, first)
	} else {
		// Use import-aware lookup for first segment of multi-part names
		res := r.walkUnqualifiedHiding(scope, first, hide.forLeadingSegment())
		cur = res.sym
	}
	if cur == nil {
		cur = r.lookupGlobalTop(scope, first)
	}
	if cur == nil {
		r.unresolvedNamespace(qn, first)
		return resolution{nil, false}
	}
	cur = r.resolvedPart(qn, 0, cur)

	return r.walkQualifiedTail(scope, qn, cur, 1, hide)
}

// walkQualifiedTail resolves qn's segments from start beneath cur, each hiding
// what hide covers among the members it reaches. Several members under the
// last segment are ambiguous unless an invocation calls them.
func (r *Resolver) walkQualifiedTail(scope *symbols.Scope, qn *ast.QualifiedName, cur *symbols.Symbol, start int, hide *refFilter) resolution {
	hide = hide.forTail()
	last := len(qn.Parts) - 1
	for i := start; i <= last; i++ {
		all, ok := r.qualifiedSegment(scope, qn, cur, i, hide)
		if !ok {
			return resolution{nil, false}
		}
		if len(all) > 1 && !(i == last && r.invocationNames[qn]) {
			r.ambiguous(qn, len(all))
			return resolution{nil, false}
		}
		cur = r.resolvedPart(qn, i, all[0])
		if r.AliasNamesNothing(cur) {
			r.unresolved(scope, qn)
			return resolution{nil, false}
		}
	}
	return resolution{cur, true}
}

// qualifiedSegment returns the members of cur that qn's segment i names, in
// lookup order, or reports the name unresolved when the segment reaches none.
func (r *Resolver) qualifiedSegment(scope *symbols.Scope, qn *ast.QualifiedName, cur *symbols.Symbol, i int, hide *refFilter) ([]*symbols.Symbol, bool) {
	all := r.membersNamed(scope, cur, qn.Parts[i].Text, qn.Global, hide)
	if len(all) == 0 {
		r.unresolvedMember(scope, qn, cur, i)
		return nil, false
	}
	return all, true
}

// membersNamed returns the members of cur a segment spelled name reaches from
// scope (global: a `$::`-rooted name), less those hide covers, in lookup
// order; it records nothing.
func (r *Resolver) membersNamed(scope *symbols.Scope, cur *symbols.Symbol, name string, global bool, hide *refFilter) []*symbols.Symbol {
	from := r.ReferringNamespaceFQN(scope)
	var all []*symbols.Symbol

	// Try local scope lookup first if available. A segment names a member of
	// the namespace the walk has reached, so it reaches only the visible ones.
	if cur.Scope != nil {
		all = hide.without(r.namedThroughNamespaces(r.LocalBindings(cur.Scope, name)))
	}

	// A member cur inherits hides one its imports surface (KerML 8.3.3.1.4);
	// what cur's features redefine is not inherited (KerML 8.3.3.3.6).
	if len(all) == 0 {
		if sym, ok := r.lookupContributedMember(cur, name, hide); ok &&
			visibleAsInheritedMember(cur, sym) && r.namedThroughNamespace(sym) {
			if sym, ok = r.inheritedAsFrom(cur, sym, hide); ok {
				all = []*symbols.Symbol{sym}
			}
		}
	}

	if len(all) == 0 && cur.Scope != nil {
		if sym, ok := r.lookupImportedMember(cur, cur.Scope, scope, name); ok &&
			r.namedThroughNamespace(sym) && !hide.hides(sym) {
			all = []*symbols.Symbol{sym}
		}
	}

	// If local lookup fails (or no scope), look the segment up under the FQN
	// walked so far. This handles cases like ScalarValues::Real where
	// ScalarValues is a package from stdlib that was indexed with full FQNs
	// but doesn't have a populated Scope, at any nesting depth.
	memberFQN := r.registeredFQN(cur) + "::" + name
	if len(all) == 0 && r.idx != nil {
		found := r.idx.LookupQualifiedFrom(memberFQN, from)
		if !global {
			found = notConflatedWith(cur, found)
		}
		candidates := r.namedThroughNamespaces(
			r.admittedUnder(r.documentOf(scope), from, memberFQN, found))
		switch visible := hide.without(candidates); {
		case len(visible) > 0:
			return visible
		case len(candidates) > 0:
			// Only what the reference must not see is declared here; a general may
			// still contribute the name.
		case len(found) > 0:
			// Every candidate the name reaches is filtered out, so it is not a
			// member of the namespace it appears under (KerML 8.2.4) and no
			// other route may recover it.
			return nil
		}
	}

	// The name exists under cur but only because a private import surfaced it
	// there: it is invisible from here (KerML 8.2.3.3), and the member search
	// below reaches cached symbols by a route that does not know that.
	if len(all) == 0 && r.idx != nil && r.idx.HiddenFrom(memberFQN, from) {
		return nil
	}

	// A segment may name a member the current symbol inherits rather than
	// declares: `engine::'4cylEngine'` reaches the variants of the type
	// `engine` is typed by.
	if len(all) == 0 {
		if sym, ok := r.lookupMember(cur, name, hide); ok && r.namedThroughNamespace(sym) && !hide.hides(sym) {
			if sym, ok = r.inheritedAsFrom(cur, sym, hide); ok {
				all = []*symbols.Symbol{sym}
			}
		}
	}
	return all
}

// notConflatedWith drops candidates owned by another namespace that merely
// shares cur's name: the walk resolved the qualification to one namespace, and
// only its members continue it (KerML 8.2.3.5). A `$::`-rooted name is exempt.
func notConflatedWith(cur *symbols.Symbol, cands []*symbols.Symbol) []*symbols.Symbol {
	if cur == nil || cur.Scope == nil {
		return cands
	}
	kept := make([]*symbols.Symbol, 0, len(cands))
	for _, sym := range cands {
		if sym != nil && sym.OwnerScope != nil {
			if owner := sym.OwnerScope.Owner(); owner != nil && owner != cur && owner.Name == cur.Name {
				continue
			}
		}
		kept = append(kept, sym)
	}
	return kept
}

// registeredFQN is the name a symbol's own members are indexed under: the path
// the index walks for a parsed symbol, and the already-qualified name a symbol
// restored from a cache record carries.
func (r *Resolver) registeredFQN(sym *symbols.Symbol) string {
	if r.idx == nil || sym == nil {
		return ""
	}
	if fqn := r.idx.GetFQN(sym); fqn != "" {
		return fqn
	}
	return sym.Name
}

// ReferringNamespaceFQN returns the fully-qualified name of the namespace a
// reference made in scope belongs to, or "" for one made outside any namespace.
// It is the context a qualified lookup is answered in: a name a private wildcard
// import brought into a namespace is a member of it but visible only from
// within (KerML 8.2.3.3), so `Mid::Hidden` resolves inside Mid and nowhere else.
func (r *Resolver) ReferringNamespaceFQN(scope *symbols.Scope) string {
	if r.idx == nil {
		return ""
	}
	for s := scope; s != nil; s = s.Parent() {
		if owner := s.Owner(); owner != nil && owner.Name != "" {
			return r.idx.GetFQN(owner)
		}
	}
	return ""
}

// lookupInRoot finds a name in the document root scope reachable from scope.
func (r *Resolver) lookupInRoot(scope *symbols.Scope, name string) *symbols.Symbol {
	root := rootOf(scope)
	if root == nil {
		return nil
	}
	sym, _ := root.LookupLocal(name)
	return sym
}

// lookupGlobalTop finds a top-level (single-segment FQN) symbol in the global
// index. Resolution there is single-valued (KerML 8.2.3.5), so two root
// namespaces of one name make the first the answer, not an error: only a
// Namespace's own members must be distinguishable, and the global namespace is
// not one.
func (r *Resolver) lookupGlobalTop(scope *symbols.Scope, name string) *symbols.Symbol {
	// A name reached here may be one a filtered import surfaced at a document's
	// root, so the conditions of the routes registering it decide it here too.
	syms := r.globalCandidates(scope, name)
	if len(syms) == 0 {
		return nil
	}
	return syms[0]
}

// rootOf returns the topmost ancestor of scope (the document root), or nil.
func rootOf(scope *symbols.Scope) *symbols.Scope {
	if scope == nil {
		return nil
	}
	for scope.Parent() != nil {
		scope = scope.Parent()
	}
	return scope
}

// unresolved records an unresolved-reference diagnostic, offering the spellings
// a simple name may have meant. A qualified name already says where to look, so
// only an unqualified one is second-guessed.
func (r *Resolver) unresolved(scope *symbols.Scope, qn *ast.QualifiedName) {
	delete(r.ambiguities, qn)
	msg := unresolvedReferencePrefix + qnText(qn)
	var fixes []quickfix.Fix
	if len(qn.Parts) == 1 && !qn.Global {
		name := qn.Parts[0].Text
		msg = r.unresolvedMessage(scope, name, qn)
		fixes = r.unresolvedFixes(scope, name, qn)
	}
	r.reportQualified(qn, Diagnostic{
		Span:    qn.Span(),
		Message: msg,
		Fixes:   fixes,
	})
}

// unresolvedMember records an unresolved-reference diagnostic for a qualified
// name whose segment i names no member of cur, offering the members it is the
// unquoted start of: `T::SA` may mean `T::'SA-506'`.
func (r *Resolver) unresolvedMember(scope *symbols.Scope, qn *ast.QualifiedName, cur *symbols.Symbol, i int) {
	delete(r.ambiguities, qn)
	r.reportQualified(qn, Diagnostic{
		Span:    qn.Span(),
		Message: unresolvedReferencePrefix + r.UnresolvedMember(scope, qn, cur, i),
	})
}

// unresolvedNamespace records an unresolved-reference diagnostic for a
// qualified name whose qualifying namespace ns is not loaded at all, naming
// elements of the same simple name found elsewhere — which is what a reference
// into a library the workspace does not have looks like.
func (r *Resolver) unresolvedNamespace(qn *ast.QualifiedName, ns string) {
	msg := "unresolved reference: " + qnText(qn)
	if r.idx != nil && len(qn.Parts) > 1 {
		last := qn.Parts[len(qn.Parts)-1].Text
		if cands := r.idx.FQNsEndingIn(last, 3); len(cands) > 0 {
			msg += fmt.Sprintf(" (no namespace %q is loaded; %q is declared as %s)",
				ns, last, strings.Join(cands, ", "))
		}
	}
	r.reportQualified(qn, Diagnostic{Span: qn.Span(), Message: msg})
}

// ambiguous records an ambiguity diagnostic reporting the number of matches.
func (r *Resolver) ambiguous(qn *ast.QualifiedName, n int) {
	journalNew(r, r.ambiguities, qn, qn)
	r.ambiguities[qn] = n
	r.reportQualified(qn, Diagnostic{
		Span:    qn.Span(),
		Message: fmt.Sprintf("ambiguous reference: %s (%d candidates)", qnText(qn), n),
	})
}

func (r *Resolver) reportQualified(qn *ast.QualifiedName, d Diagnostic) {
	if r.quiet == 0 {
		if r.reportedQualified[qn] {
			return
		}
		journalNew(r, r.reportedQualified, qn, qn)
		r.reportedQualified[qn] = true
	}
	r.report(d)
}
