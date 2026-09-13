package resolve

import (
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/suggest"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// suggestKey identifies a suggestion by the name that did not resolve, the scope
// it was written in, which is what decides how a candidate is reached, and
// whether an invocation called it, which lets several declarations answer.
type suggestKey struct {
	scope *symbols.Scope
	name  string
	call  bool
}

// suggestion is what an unresolvable name may have meant, as registered: near
// spellings, and the declared names it is the unquoted start of ('SA-506' for SA).
type suggestion struct {
	spellings []string
	unquoted  []string
}

// suggestionFor returns the suggestion for an unresolvable unqualified name written
// in scope, memoized per scope and name for as long as at is (see Scratch).
func (r *Resolver) suggestionFor(scope *symbols.Scope, name string, at ast.Node) suggestion {
	if r.idx == nil || name == "" {
		return suggestion{}
	}
	key := suggestKey{scope: scope, name: name, call: r.called(at)}
	if s, ok := r.suggestions[key]; ok {
		return s
	}
	// Scoring resolves names, which may report one unresolved in turn: another
	// name is still worth suggesting for, this one is not.
	if r.suggesting[key] {
		return suggestion{}
	}
	r.suggesting[key] = true
	defer delete(r.suggesting, key)

	table := r.suggestTable()
	var cands []suggest.Candidate
	for _, fqn := range table.Qualified(name) {
		if r.namesSomething(fqn) {
			cands = append(cands, suggest.Candidate{Spelling: fqn, Library: r.libraryFQN(fqn)})
		}
	}
	for _, near := range table.Neighbours(name) {
		if c, ok := r.candidateFor(scope, near); ok {
			cands = append(cands, c)
		}
	}
	out := suggestion{spellings: suggest.Rank(cands), unquoted: r.unquotedFor(scope, name, key.call)}
	journalNew(r, r.suggestions, key, at)
	r.suggestions[key] = out
	return out
}

// called reports whether at is a name an invocation calls (see ResolveInvocationName).
func (r *Resolver) called(at ast.Node) bool {
	qn, ok := at.(*ast.QualifiedName)
	return ok && r.invocationNames[qn]
}

// namesSomething reports whether the declaration registered under fqn is a spelling
// of anything: an alias naming nothing, or a member binding no name, is not.
func (r *Resolver) namesSomething(fqn string) bool {
	decl := r.idx.Declaring(fqn)
	return !r.AliasNamesNothing(decl) && r.BindsName(decl)
}

// unquotedFor returns the declared names name is the unquoted start of, as typed
// from scope: bare when they resolve there, else by the path declaring them,
// when that path resolves from scope too — a private member's does not.
func (r *Resolver) unquotedFor(scope *symbols.Scope, name string, call bool) []string {
	table := r.suggestTable()
	var cands []suggest.Candidate
	for _, full := range table.Unquoted(name) {
		if res := r.walkUnqualified(scope, full); res.ok {
			cands = append(cands, suggest.Candidate{Spelling: suggest.Name(full), InScope: true, Library: r.idx.Library(res.sym)})
			continue
		}
		for _, fqn := range table.Declared(full) {
			if r.importable(fqn) && r.namesSomething(fqn) && r.pathReaches(scope, fqn, full, call) {
				cands = append(cands, suggest.Candidate{Spelling: suggest.Spelled(fqn, full), Library: r.libraryFQN(fqn)})
				break
			}
		}
	}
	return suggest.Rank(cands)
}

// pathReaches reports whether the registered fqn, ending in the declared name, resolves
// from scope as a reference there would: each qualifier to one namespace, the name as denoted.
func (r *Resolver) pathReaches(scope *symbols.Scope, fqn, name string, call bool) bool {
	path, ok := strings.CutSuffix(fqn, "::"+name)
	if !ok {
		return false
	}
	segs := strings.Split(path, "::")
	cur := r.walkUnqualified(scope, segs[0]).sym
	if cur == nil {
		cur = r.lookupGlobalTop(scope, segs[0])
	}
	for _, seg := range segs[1:] {
		if cur == nil || r.AliasNamesNothing(cur) {
			return false
		}
		all := r.membersNamed(scope, r.AliasedElement(cur), seg, false, nil)
		if len(all) != 1 {
			return false
		}
		cur = all[0]
	}
	if cur == nil || r.AliasNamesNothing(cur) {
		return false
	}
	return r.denoted(r.membersNamed(scope, r.AliasedElement(cur), name, false, nil), call)
}

// denoted reports whether the members a last segment names denote something:
// one that is not an alias of nothing, or several only where a call selects among them.
func (r *Resolver) denoted(all []*symbols.Symbol, call bool) bool {
	if len(all) > 1 && !call {
		return false
	}
	for _, sym := range all {
		if !r.AliasNamesNothing(sym) {
			return true
		}
	}
	return false
}

// memberKey identifies a member hint by the namespace and the segment written
// under it; probing one resolves names, which may want the same hint in turn.
type memberKey struct {
	owner *symbols.Symbol
	name  string
}

// unquotedMembers returns the members of owner, qualified by prefix, that the
// segment written under it is the unquoted start of and that resolve from scope.
func (r *Resolver) unquotedMembers(scope *symbols.Scope, owner *symbols.Symbol, prefix, segment string, global, call bool) []string {
	if owner == nil {
		return nil
	}
	key := memberKey{owner: owner, name: segment}
	if r.probing[key] {
		return nil
	}
	r.probing[key] = true
	defer delete(r.probing, key)

	seen := map[string]bool{}
	var names []string
	if owner.Scope != nil {
		for _, name := range owner.Scope.MemberNames() {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	if r.idx != nil {
		// Every registered name: an inherited or imported member is a candidate too.
		for _, name := range r.suggestTable().Unquoted(segment) {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	var out []string
	for _, name := range suggest.Unquoted(segment, names) {
		if !r.denoted(r.membersNamed(scope, owner, name, global, nil), call) {
			continue
		}
		out = append(out, prefix+"::"+suggest.Name(name))
		if len(out) == suggest.Limit {
			break
		}
	}
	return out
}

// candidateFor scores one near spelling: what the user would have to write to
// reach it from scope — the name itself when it resolves there, else the
// qualified path that declares it — and whether that is one of their own
// declarations or a bundled library name. A misspelling is not offered a path
// the user cannot import: fixing the spelling and qualifying a name nested in
// some other element is two corrections, which is past coincidence.
func (r *Resolver) candidateFor(scope *symbols.Scope, near suggest.Neighbour) (suggest.Candidate, bool) {
	if res := r.walkUnqualified(scope, near.Name); res.ok {
		return suggest.Candidate{
			Spelling: near.Name,
			Distance: near.Distance,
			InScope:  true,
			Library:  r.idx.Library(res.sym),
		}, true
	}
	for _, fqn := range r.suggestTable().Qualified(near.Name) {
		if r.importable(fqn) {
			return suggest.Candidate{
				Spelling: fqn,
				Distance: near.Distance,
				Library:  r.libraryFQN(fqn),
			}, true
		}
	}
	return suggest.Candidate{}, false
}

// importable reports whether fqn is declared in a namespace the user can name
// an import of: a package or namespace, rather than the body of some element.
func (r *Resolver) importable(fqn string) bool {
	i := strings.LastIndex(fqn, "::")
	if i < 0 {
		return true
	}
	owner := r.idx.Declaring(fqn[:i])
	if owner == nil {
		return false
	}
	return owner.Kind == symbols.SymbolPackage || owner.Kind == symbols.SymbolNamespace
}

// libraryFQN reports whether fqn names a bundled library declaration.
func (r *Resolver) libraryFQN(fqn string) bool {
	return r.idx.Library(r.idx.Declaring(fqn))
}

// suggestTable indexes the names the index registers, swept once per resolver:
// a suggestion is a lookup, not a scan of the whole library per unresolved name.
func (r *Resolver) suggestTable() *suggest.Table {
	if r.names == nil {
		r.names = suggest.NewTable(r.idx)
	}
	return r.names
}

// unresolvedMessage is what an unresolved unqualified reference written in scope
// reports. The hint belongs to the diagnostic, not to one renderer, so the CLI,
// the REPL and the LSP all show it.
func (r *Resolver) unresolvedMessage(scope *symbols.Scope, name string, at ast.Node) string {
	return unresolvedReferencePrefix + r.UnresolvedName(scope, name, at)
}

// UnresolvedName is the text after "unresolved reference: " for an unqualified
// name written at in scope that resolves to nothing: the name and the spellings
// it may mean.
func (r *Resolver) UnresolvedName(scope *symbols.Scope, name string, at ast.Node) string {
	s := r.suggestionFor(scope, name, at)
	spellings := make([]string, len(s.spellings))
	for i, spelling := range s.spellings {
		spellings[i] = suggest.Notation(spelling)
	}
	return suggest.Hint(name, name, spellings, s.unquoted)
}

// UnresolvedMember is the text after "unresolved reference: " for a qualified
// name read from scope whose segment i names no member of owner: as written,
// plus `T::'SA-506'` for `T::SA` when that spelling would resolve from scope.
func (r *Resolver) UnresolvedMember(scope *symbols.Scope, qn *ast.QualifiedName, owner *symbols.Symbol, i int) string {
	written := qnText(qn)
	if i <= 0 || i >= len(qn.Parts) {
		return written
	}
	prefix := qnText(&ast.QualifiedName{Global: qn.Global, Parts: qn.Parts[:i]})
	return suggest.Hint(written, written, nil,
		r.unquotedMembers(scope, owner, prefix, qn.Parts[i].Text, qn.Global, r.invocationNames[qn]))
}

// unresolvedReferencePrefix is how a reference that resolves to nothing reads.
const unresolvedReferencePrefix = "unresolved reference: "
