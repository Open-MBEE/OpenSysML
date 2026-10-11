package resolve

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// checkDistinguishability reports the member names of one namespace that are not
// distinguishable: an owned name repeating another owned name, — for a type —
// an owned name repeating one the type inherits, and an imported name repeating
// another imported one (KerML 7.2.2, SysML 7.6.1). All are warnings, as the
// reference implementation reports the first two.
func (r *Resolver) checkDistinguishability(scope *symbols.Scope) {
	if scope == nil {
		return
	}
	r.checkOwnedNames(scope)
	r.checkInheritedNames(scope)
	r.checkImportedNames(scope)
}

// importedMember is one membership an import surfaces into a namespace, with
// the import that brought it, or one the namespace inherits (imp nil).
type importedMember struct {
	sym *symbols.Symbol
	imp *ast.Import
}

// ownedName is a name among one namespace's own members. A repeat of it there
// — two documents declaring the same package and member — is that namespace's
// duplicate, reported where it is declared, not at every importer.
type ownedName struct {
	owner ownerKey
	name  string
}

// ownerKey identifies a namespace across documents: by qualified name when it
// and every namespace enclosing it is a package, since one package declared in
// two documents is one namespace, and by scope otherwise — two types of one
// name are two namespaces.
type ownerKey struct {
	scope *symbols.Scope
	fqn   string
}

func ownerKeyOf(scope *symbols.Scope) ownerKey {
	if scope == nil || scope.Owner() == nil {
		return ownerKey{scope: scope}
	}
	for s := scope; s != nil && s.Owner() != nil; s = s.Owner().OwnerScope {
		owner := s.Owner()
		if owner.Name == "" || (owner.Kind != symbols.SymbolPackage && owner.Kind != symbols.SymbolNamespace) {
			return ownerKey{scope: scope}
		}
	}
	return ownerKey{fqn: symbols.FQNOf(scope.Owner())}
}

// boundName is one name one element is reached under: an alias and the element
// it names bind one name to one element, a membership two imports reach too.
type boundName struct {
	key  symbols.ElementKey
	name string
}

// checkImportedNames reports each name an imported membership shares with
// another imported membership or, in a type, with an inherited one: a
// namespace's memberships are its owned, imported and inherited ones together,
// and all of them must be distinguishable (KerML 8.3.2.4.5). A membership two
// imports both reach is one membership and conflicts with nothing; so are two
// memberships of one element, an alias and what it names. An imported name an
// owned member hides takes no part, nor does library content, as the inherited
// pass leaves library supertypes out. Two imported memberships colliding are
// both hidden from the namespace (importedCollisions); an inherited one
// colliding with an imported one hides neither, the type being ill-formed.
func (r *Resolver) checkImportedNames(scope *symbols.Scope) {
	if r.idx.DocumentLibraryTier(r.document).Library() {
		return
	}
	c := r.importedCollisions(scope)
	if len(c.names) == 0 {
		return
	}
	inherited := r.inheritedAgainstImports(scope)
	for _, name := range c.names {
		members := c.byName[name]
		for _, sym := range inherited[name] {
			members = append(members, importedMember{sym: sym})
		}
		if len(members) < 2 {
			continue
		}
		// Keep the members some other member is indistinguishable from; the
		// rest conflict with nothing.
		var kept []importedMember
		imported := 0
		for i, member := range members {
			if len(r.duplicatesOf(member.sym, importedSymbolsExcept(members, i))) > 0 {
				kept = append(kept, member)
				if member.imp != nil {
					imported++
				}
			}
		}
		if len(kept) < 2 || imported == 0 {
			continue
		}
		r.duplicateImported(name, kept)
	}
}

// importCollisions is what the imports of one namespace bring, by name, and
// the memberships KerML 7.2.5.4 hides among them: distinct elements two or
// more imports bring under one name or short name, indistinguishable by
// metaclass, no owned member hiding them. Library content takes no part, as
// in checkImportedNames: the warning and the hiding are one computation.
type importCollisions struct {
	// names are the imported names in import order and byName their
	// memberships, one per element reached, less those an owned member hides.
	names  []string
	byName map[string][]importedMember
	// hidden holds every name a colliding membership binds; collided lists,
	// per colliding name, the memberships hidden under it.
	hidden   map[boundName]bool
	collided map[string][]importedMember
}

var noImportCollisions = &importCollisions{}

// importedCollisions is the importCollisions of scope, memoized once the resolver is
// settled and kept provisionally meanwhile, so an import cycle computes each namespace once.
func (r *Resolver) importedCollisions(scope *symbols.Scope) *importCollisions {
	if scope == nil || r.idx == nil {
		return noImportCollisions
	}
	if r.colliding[scope] {
		r.collisionCut = true
		return noImportCollisions
	}
	if c, done := r.collisions[scope]; done {
		return c
	}
	if r.collisionsSettled() {
		r.provisional = nil
	} else if c, ok := r.provisional[scope]; ok {
		r.collisionCut = true
		return c
	}
	imports := r.scopeImports(scope)
	if len(imports) == 0 || r.idx.DocumentLibraryTier(symbols.DocNameOf(scope)).Library() {
		return noImportCollisions
	}
	r.colliding[scope] = true
	cut := r.collisionCut
	r.collisionCut = false
	c, complete := r.collectImported(scope, imports)
	delete(r.colliding, scope)
	// A cut met while this computation was the outermost is a cycle back to
	// scope, which excluding it settles; a cut within another's is not.
	complete = complete && (!r.collisionCut || len(r.colliding) == 0)
	r.collisionCut = cut || r.collisionCut
	switch {
	case complete:
		journalNew(r, r.collisions, scope, imports[0])
		r.collisions[scope] = c
	case !r.collisionsSettled():
		if r.provisional == nil {
			r.provisional = map[*symbols.Scope]*importCollisions{}
		}
		r.provisional[scope] = c
	}
	return c
}

// collisionsSettled reports whether what every namespace's imports bring can
// be known now: no import target or filter condition is being resolved and no
// namespace's collisions are being computed.
func (r *Resolver) collisionsSettled() bool {
	return r.inCondition == 0 && len(r.resolvingImports) == 0 && len(r.colliding) == 0
}

// collectImported gathers what imports bring into scope and decides the
// collisions among them; complete is false when an import's target could not
// be settled yet.
func (r *Resolver) collectImported(scope *symbols.Scope, imports []*ast.Import) (*importCollisions, bool) {
	complete := r.inCondition == 0
	owned := map[string]bool{}
	members, aliases := r.DistinguishableMembers(scope)
	for _, sym := range append(members, aliases...) {
		for _, name := range memberNames(sym) {
			owned[name] = true
		}
	}
	c := &importCollisions{
		byName:   map[string][]importedMember{},
		hidden:   map[boundName]bool{},
		collided: map[string][]importedMember{},
	}
	seen := map[boundName]bool{}
	owners := map[ownedName]bool{}
	for _, imp := range imports {
		if r.resolvingImports[imp] {
			complete = false
			continue
		}
		target, ok := r.importTargetOf(scope, imp)
		if !ok {
			complete = complete && len(r.resolvingImports) == 0
			continue
		}
		if target == nil || r.idx.Library(target) {
			continue
		}
		for _, sym := range r.importedMembersInto(scope, scope, imp, false) {
			if sym.Name == "" || r.idx.Library(sym) || !contributesName(sym) || !r.BindsName(sym) {
				continue
			}
			key := symbols.KeyOf(r.aliasTarget(sym))
			for _, name := range memberNames(sym) {
				bound := boundName{key: key, name: name}
				owner := ownedName{owner: ownerKeyOf(sym.OwnerScope), name: name}
				if owned[name] || seen[bound] || owners[owner] {
					continue
				}
				seen[bound] = true
				owners[owner] = true
				if _, ok := c.byName[name]; !ok {
					c.names = append(c.names, name)
				}
				c.byName[name] = append(c.byName[name], importedMember{sym: sym, imp: imp})
			}
		}
	}
	for _, name := range c.names {
		members := c.byName[name]
		if len(members) < 2 {
			continue
		}
		var kept []importedMember
		for i, member := range members {
			if len(r.duplicatesOf(member.sym, importedSymbolsExcept(members, i))) > 0 {
				kept = append(kept, member)
			}
		}
		if len(kept) < 2 {
			continue
		}
		c.collided[name] = kept
		for _, member := range kept {
			key := symbols.KeyOf(r.aliasTarget(member.sym))
			for _, bound := range memberNames(member.sym) {
				c.hidden[boundName{key: key, name: bound}] = true
			}
		}
	}
	return c, complete
}

// hiddenImport reports whether sym, reached under name through an import of
// scope, is a membership scope hides (KerML 7.2.5.4). An invocation name is
// looked up as an overload set, every membership imported under it a
// candidate, so nothing is hidden from it (docs/project/spec-compliance.md,
// "Invocation overload selection").
func (r *Resolver) hiddenImport(scope *symbols.Scope, name string, sym *symbols.Symbol) bool {
	if sym == nil || r.overloading > 0 {
		return false
	}
	c := r.importedCollisions(scope)
	return len(c.hidden) > 0 && c.hidden[boundName{key: symbols.KeyOf(r.aliasTarget(sym)), name: name}]
}

// hiddenImportMember is hiddenImport under any name sym binds.
func (r *Resolver) hiddenImportMember(scope *symbols.Scope, sym *symbols.Symbol) bool {
	if sym == nil || r.overloading > 0 {
		return false
	}
	c := r.importedCollisions(scope)
	if len(c.hidden) == 0 {
		return false
	}
	key := symbols.KeyOf(r.aliasTarget(sym))
	for _, name := range memberNames(sym) {
		if c.hidden[boundName{key: key, name: name}] {
			return true
		}
	}
	return false
}

// withoutHiddenImports drops from syms, reached under name, what scope hides
// and what reaches scope only through a namespace that hides it: a membership
// hidden there is no member of that namespace, so no import brings it on from
// there, though another import of scope may still bring the element itself.
func (r *Resolver) withoutHiddenImports(scope *symbols.Scope, name string, syms []*symbols.Symbol) []*symbols.Symbol {
	if scope == nil || r.overloading > 0 || len(syms) == 0 {
		return syms
	}
	// A route read through a cycle of imports still being computed is undecided
	// (collisionCut): what it brings is kept, as the cycle's closure, not dropped.
	cut := r.collisionCut
	r.collisionCut = false
	defer func() { r.collisionCut = cut || r.collisionCut }()
	through := r.collidedThrough(scope, name, map[*symbols.Scope]bool{})
	if len(through) == 0 && len(r.importedCollisions(scope).hidden) == 0 {
		return syms
	}
	out := syms[:0:0]
	for _, sym := range syms {
		if r.hiddenImport(scope, name, sym) {
			continue
		}
		key := symbols.KeyOf(r.aliasTarget(sym))
		hidden := false
		for _, member := range through {
			if symbols.KeyOf(r.aliasTarget(member.sym)) == key {
				hidden = true
				break
			}
		}
		if !hidden || r.importBrings(scope, name, sym) || r.collisionCut {
			out = append(out, sym)
		}
	}
	return out
}

// importBrings reports whether an owned membership or an import of scope brings
// sym under name: what an import surfaces is decided by eachImportMatch, so a
// route counts only where the element is actually a visible, admitted member
// along it, hidden nowhere on the way. A recursive import brings only what its
// subtree holds, and a namespace import nothing of a non-namespace target.
func (r *Resolver) importBrings(scope *symbols.Scope, name string, sym *symbols.Symbol) bool {
	if scope == nil || r.hiddenImport(scope, name, sym) {
		return false
	}
	if asked, ok := r.bringing[scope]; ok {
		// A namespace asked before brings nothing new; one still being asked is a
		// cycle back, undecided here (collisionCut).
		r.collisionCut = r.collisionCut || asked
		return false
	}
	key := symbols.KeyOf(r.aliasTarget(sym))
	for _, owned := range r.LocalBindings(scope, name) {
		if symbols.KeyOf(r.aliasTarget(owned)) == key {
			return true
		}
	}
	// One search asks each namespace once: one found to bring nothing brings
	// nothing by a longer route either, so a cycle of re-exports stays linear.
	if len(r.bringing) == 0 {
		defer clear(r.bringing)
	}
	r.bringing[scope] = true
	defer func() { r.bringing[scope] = false }()
	for _, imp := range r.scopeImports(scope) {
		for _, found := range r.importMatchesAll(scope, imp, name) {
			if symbols.KeyOf(r.aliasTarget(found)) == key {
				return true
			}
		}
	}
	return false
}

// withoutHiddenReexports is withoutHiddenImports over the index entries under
// prefix that are re-exports; what the namespace declares is kept as it is.
func (r *Resolver) withoutHiddenReexports(scope *symbols.Scope, prefix, name string, syms []*symbols.Symbol) []*symbols.Symbol {
	var reexported []*symbols.Symbol
	for _, sym := range syms {
		if r.reexportedUnder(prefix, sym) {
			reexported = append(reexported, sym)
		}
	}
	if len(reexported) == 0 {
		return syms
	}
	kept := map[*symbols.Symbol]bool{}
	for _, sym := range r.withoutHiddenImports(scope, name, reexported) {
		kept[sym] = true
	}
	out := syms[:0:0]
	for _, sym := range syms {
		if !r.reexportedUnder(prefix, sym) || kept[sym] {
			out = append(out, sym)
		}
	}
	return out
}

// hiddenOnRoute reports whether sym, an index entry registered under scope's
// namespace, reaches scope only through a namespace that hides it, under any
// name sym binds (see withoutHiddenImports).
func (r *Resolver) hiddenOnRoute(scope *symbols.Scope, sym *symbols.Symbol) bool {
	if scope == nil || sym == nil || r.overloading > 0 {
		return false
	}
	for _, name := range memberNames(sym) {
		if len(r.withoutHiddenImports(scope, name, []*symbols.Symbol{sym})) == 0 {
			return true
		}
	}
	return false
}

// collidedThrough is what hides name in scope, or in a namespace an import of
// scope re-exports from: a hidden membership is no member of that namespace,
// so the import brings none under the name.
func (r *Resolver) collidedThrough(scope *symbols.Scope, name string, seen map[*symbols.Scope]bool) []importedMember {
	if scope == nil || seen[scope] || r.idx == nil || r.idx.DocumentLibraryTier(symbols.DocNameOf(scope)).Library() {
		// A library namespace hides nothing (importedCollisions), nor do the
		// libraries it imports.
		return nil
	}
	seen[scope] = true
	if hidden := r.importedCollisions(scope).collided[name]; len(hidden) > 0 {
		return hidden
	}
	for _, imp := range r.scopeImports(scope) {
		if imp.Kind != ast.ImportNamespace || r.resolvingImports[imp] {
			continue
		}
		if target, ok := r.importTargetOf(scope, imp); ok && target != nil && target.Scope != nil {
			if hidden := r.collidedThrough(target.Scope, name, seen); len(hidden) > 0 {
				return hidden
			}
		}
	}
	return nil
}

// hiddenImportsOf lists, for an unqualified name written in scope that
// resolves to nothing, the colliding memberships that hide it on the way out:
// in scope, its enclosing namespaces, and the generals of the types among them.
func (r *Resolver) hiddenImportsOf(scope *symbols.Scope, name string) []importedMember {
	for s := scope; s != nil; s = s.Parent() {
		if hidden := r.collidedThrough(s, name, map[*symbols.Scope]bool{}); len(hidden) > 0 {
			return hidden
		}
		if owner := s.Owner(); owner != nil && r.model != nil {
			if _, ok := r.model.(supertypeProvider); ok {
				for _, sup := range r.specializationChain(owner) {
					if sup.Scope == nil {
						continue
					}
					if hidden := r.importedCollisions(sup.Scope).collided[name]; len(hidden) > 0 {
						return hidden
					}
				}
			}
		}
	}
	return nil
}

// inheritedAgainstImports is what a type inherits, by name, for its imported
// memberships to be told apart from: nothing for a namespace that is no type,
// and nothing while a redefinition the type declares is unresolved, as that
// may be the one hiding the inherited name (as checkInheritedAmbiguity holds).
func (r *Resolver) inheritedAgainstImports(scope *symbols.Scope) map[string][]*symbols.Symbol {
	owner := scope.Owner()
	if r.model == nil || owner == nil || ParameterizedByName(owner) {
		return nil
	}
	model, ok := r.model.(supertypeProvider)
	if !ok || len(model.DirectSupertypes(owner)) == 0 || r.hasUnresolvedRedefinitions(scope) {
		return nil
	}
	return r.inheritedMembers(owner, model)
}

// memberNames are the names a membership binds: the member's name and, when it
// declares one, its short name (KerML 8.3.2.4.3 compares both).
func memberNames(sym *symbols.Symbol) []string {
	names := []string{sym.Name}
	if sym.ShortName != "" && sym.ShortName != sym.Name {
		names = append(names, sym.ShortName)
	}
	return names
}

// importedSymbolsExcept is the symbols of members other than the i-th.
func importedSymbolsExcept(members []importedMember, i int) []*symbols.Symbol {
	out := make([]*symbols.Symbol, 0, len(members)-1)
	for j, member := range members {
		if j != i {
			out = append(out, member.sym)
		}
	}
	return out
}

// duplicateImported reports one name several memberships share, at the import
// that brought the last imported one, naming every colliding member and the
// import each came through — or that it is inherited — so a reader can rename
// one, narrow an import or import the member wanted explicitly.
func (r *Resolver) duplicateImported(name string, members []importedMember) {
	parts := make([]string, 0, len(members))
	var last *ast.Import
	for _, member := range members {
		origin := "inherited"
		if member.imp != nil {
			origin = importText(member.imp)
			last = member.imp
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", source.QualifiedNameOf(symbols.NameChain(member.sym)), origin))
	}
	span := last.Span()
	if last.Imported != nil {
		span = last.Imported.Span()
	}
	r.reportDuplicate(span, fmt.Sprintf("Duplicate of imported member name '%s': %s", name, strings.Join(parts, ", ")))
}

// importText spells an import as its declaration does, visibility aside:
// `import A::*`, `import A::*::**`, `import A::x` or `import A::x::**`.
func importText(imp *ast.Import) string {
	text := "import "
	if imp.IsAll {
		text += "all "
	}
	names := make([]string, 0, len(imp.Imported.Parts))
	for _, part := range imp.Imported.Parts {
		names = append(names, part.Text)
	}
	if imp.Imported.Global {
		text += "$::"
	}
	text += source.QualifiedNameOf(names)
	if imp.Kind == ast.ImportNamespace {
		text += "::*"
	}
	if imp.IsRecursive {
		text += "::**"
	}
	return text
}

// checkOwnedNames reports each name a namespace declares twice. Aliases are a
// separate namespace of their own: an alias collides with an owned name and with
// another alias, each under its own wording.
func (r *Resolver) checkOwnedNames(scope *symbols.Scope) {
	owned, aliases := r.DistinguishableMembers(scope)
	ownedByName := byName(owned)
	aliasByName := byName(aliases)
	for _, sym := range owned {
		if len(r.duplicatesOf(sym, ownedByName[sym.Name])) > 0 {
			r.duplicateName(sym, "Duplicate of other owned member name", nil)
		}
	}
	for _, sym := range aliases {
		if len(r.duplicatesOf(sym, ownedByName[sym.Name])) > 0 {
			r.duplicateName(sym, "Duplicate of owned member name", nil)
		}
		if len(r.duplicatesOf(sym, aliasByName[sym.Name])) > 0 {
			r.duplicateName(sym, "Duplicate of other alias name", nil)
		}
	}
}

// checkInheritedNames reports each member a type declares whose name is already
// the name of a member the type inherits. A feature the type redefines is not
// inherited any more, which is how the name is legitimately reused.
func (r *Resolver) checkInheritedNames(scope *symbols.Scope) {
	owner := scope.Owner()
	if r.model == nil || owner == nil || ParameterizedByName(owner) {
		return
	}
	// Nothing is inherited without a supertype, and collecting the inherited
	// members of every scope is not free.
	model, ok := r.model.(supertypeProvider)
	if !ok || len(model.DirectSupertypes(owner)) == 0 {
		return
	}
	inherited := r.inheritedMembers(owner, model)
	if len(inherited) == 0 {
		return
	}
	owned, aliases := r.DistinguishableMembers(scope)
	declared := map[string]bool{}
	for _, sym := range append(owned, aliases...) {
		declared[sym.Name] = true
		if ImplicitlyRedefined(sym) || r.hasUnresolvedRedefinition(sym) {
			continue
		}
		dups := r.duplicatesOf(sym, inherited[sym.Name])
		if len(dups) == 0 {
			continue
		}
		r.duplicateName(sym, "Duplicate of inherited member name", dups)
	}
	r.checkInheritedAmbiguity(owner, inherited, declared, model)
}

// checkInheritedAmbiguity reports a name the type inherits from two different
// supertypes at once: the type declares nothing at fault, so the reference
// reports it on the type itself. A name the type redeclares is reported there.
// A member whose redefinition we could not resolve may be the one resolving the
// ambiguity, so nothing is claimed for such a type.
func (r *Resolver) checkInheritedAmbiguity(
	owner *symbols.Symbol,
	inherited map[string][]*symbols.Symbol,
	declared map[string]bool,
	model supertypeProvider,
) {
	if r.hasUnresolvedRedefinitions(owner.Scope) {
		return
	}
	names := make([]string, 0, len(inherited))
	for name := range inherited {
		if name != "" && !declared[name] && len(inherited[name]) > 1 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		members := r.withoutImplicitlyRedefined(inherited[name], model)
		// Keep the members some other member is indistinguishable from; the
		// rest conflict with nothing and belong to neither warning nor `from`.
		var kept []*symbols.Symbol
		for _, member := range members {
			if len(r.duplicatesOf(member, othersOf(members, member))) > 0 {
				kept = append(kept, member)
			}
		}
		if len(kept) < 2 {
			continue
		}
		r.duplicateInherited(owner, name, kept)
	}
}

// withoutImplicitlyRedefined drops the members of one inherited name that a
// same-named member implicitly redefines: a parameter redefines the one its
// owner's own supertype declares under that name (KerML 8.4.4.6).
func (r *Resolver) withoutImplicitlyRedefined(
	members []*symbols.Symbol,
	model supertypeProvider,
) []*symbols.Symbol {
	out := make([]*symbols.Symbol, 0, len(members))
	for _, sym := range members {
		hidden := false
		for _, other := range members {
			if other == sym || !ImplicitlyRedefined(other) {
				continue
			}
			if r.inheritsFrom(other.Owner(), sym.Owner(), model) {
				hidden = true
				break
			}
		}
		if !hidden {
			out = append(out, sym)
		}
	}
	return out
}

// inheritsFrom reports whether sub reaches sup through its supertypes.
func (r *Resolver) inheritsFrom(sub, sup *symbols.Symbol, model supertypeProvider) bool {
	if sub == nil || sup == nil || sub == sup {
		return false
	}
	seen := map[*symbols.Symbol]bool{sub: true}
	queue := model.DirectSupertypes(sub)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == nil || seen[cur] {
			continue
		}
		if cur == sup {
			return true
		}
		seen[cur] = true
		queue = append(queue, model.DirectSupertypes(cur)...)
	}
	return false
}

// hasUnresolvedRedefinition reports whether sym declares a redefinition whose
// target we could not resolve: the name it reuses is then not evidence of a
// duplicate. See docs/project/spec-compliance.md for the resolution gaps.
func (r *Resolver) hasUnresolvedRedefinition(sym *symbols.Symbol) bool {
	if sym.Recorded() {
		return sym.Facts.Modifiers.Has(symbols.ModUnresolvedRedefinition)
	}
	for _, rel := range redefinesRelationships(sym.Decl) {
		if _, ok := r.ResolveRedefinitionTarget(sym.OwnerScope, sym.Decl, rel.Target); !ok {
			return true
		}
	}
	return false
}

// hasUnresolvedRedefinitions reports whether any member of scope declares a
// redefinition whose target we could not resolve.
func (r *Resolver) hasUnresolvedRedefinitions(scope *symbols.Scope) bool {
	if scope == nil {
		return false
	}
	found := false
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		if r.hasUnresolvedRedefinition(sym) {
			found = true
			return false
		}
		return true
	})
	return found
}

// inheritedMembers collects the members owner inherits, keyed by name: what each
// supertype contributes, less the ones owner's own members redefine. Library
// supertypes are not walked — see docs/project/spec-compliance.md.
func (r *Resolver) inheritedMembers(owner *symbols.Symbol, model supertypeProvider) map[string][]*symbols.Symbol {
	var candidates []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{owner: true}
	for _, sup := range model.DirectSupertypes(owner) {
		candidates = append(candidates, r.inheritableMembers(owner, sup, model, seen)...)
	}
	if len(candidates) == 0 {
		return nil
	}
	out := map[string][]*symbols.Symbol{}
	for _, sym := range r.removeRedefinedFeatures(owner, candidates) {
		out[sym.Name] = append(out[sym.Name], sym)
	}
	return out
}

// inheritableMembers is what a supertype contributes to its subtypes: its own
// non-private members plus what it inherits itself, redefined ones removed at
// each level, so a redefinition anywhere up the chain hides its target. What an
// inherited expose contributes is admitted against owner's conditions too.
func (r *Resolver) inheritableMembers(owner, sup *symbols.Symbol, model supertypeProvider, seen map[*symbols.Symbol]bool) []*symbols.Symbol {
	if sup == nil || seen[sup] || r.idx.Library(sup) {
		return nil
	}
	seen[sup] = true
	var out []*symbols.Symbol
	for _, next := range model.DirectSupertypes(sup) {
		out = append(out, r.inheritableMembers(owner, next, model, seen)...)
	}
	if sup.Scope != nil {
		owned, aliases := r.DistinguishableMembers(sup.Scope)
		for _, sym := range append(owned, aliases...) {
			if sym.Visibility != ast.VisibilityPrivate {
				out = append(out, sym)
			}
		}
		out = append(out, r.importedMembers(owner, sup)...)
	}
	return r.removeRedefinedFeatures(sup, out)
}

// importedMembers is what a namespace's non-private imports contribute to it: a
// membership is inherited whether the namespace owns it or imported it
// (KerML 8.4.3.2). Library elements are left out, as library supertypes are.
func (r *Resolver) importedMembers(owner, sup *symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, imp := range r.scopeImports(sup.Scope) {
		if imp.Visibility == ast.VisibilityPrivate {
			continue
		}
		for _, sym := range r.importedMembersInto(owner.Scope, sup.Scope, imp, false) {
			if sym != nil && sym.Name != "" && !r.idx.Library(sym) && contributesName(sym) && r.BindsName(sym) &&
				!r.hiddenImportMember(sup.Scope, sym) {
				out = append(out, sym)
			}
		}
	}
	return out
}

// removeRedefinedFeatures drops the inherited members that are no longer
// inherited: one whose redefinitions reach a feature an owned member redefines,
// and one another inherited member redefines (KerML 7.4.3). An alias goes with
// the element it names, being another membership of that same element.
func (r *Resolver) removeRedefinedFeatures(owner *symbols.Symbol, inherited []*symbols.Symbol) []*symbols.Symbol {
	byOwner := r.redefinedByMembers(owner.Scope)
	var byInherited map[*symbols.Symbol]bool
	kept := make([]*symbols.Symbol, 0, len(inherited))
	for _, sym := range inherited {
		element := r.aliasTarget(sym)
		if len(r.redefinedFeatures(element)) == 0 {
			// Nothing redefined: the closure is element alone, so no map is built.
			if redefinerOtherThan(byOwner[element], sym) {
				continue
			}
			kept = append(kept, sym)
			continue
		}
		// What a dropped member redefines still stops being inherited.
		redefines := r.redefinedClosure(element)
		for target := range redefines {
			if target != element {
				if byInherited == nil {
					byInherited = map[*symbols.Symbol]bool{}
				}
				byInherited[target] = true
			}
		}
		if !redefinedByOther(redefines, byOwner, sym) {
			kept = append(kept, sym)
		}
	}
	out := make([]*symbols.Symbol, 0, len(kept))
	for _, sym := range kept {
		if !byInherited[r.aliasTarget(sym)] {
			out = append(out, sym)
		}
	}
	return out
}

// redefinedByMembers maps each feature the members of scope redefine to the
// members redefining it.
func (r *Resolver) redefinedByMembers(scope *symbols.Scope) map[*symbols.Symbol][]*symbols.Symbol {
	var out map[*symbols.Symbol][]*symbols.Symbol
	if scope == nil {
		return out
	}
	// An unnamed member redefines just as a named one does, and one whose
	// effective name only the semantic model knows is anonymous here.
	scope.ForEachMember(func(sym *symbols.Symbol) bool {
		for _, target := range r.redefinedFeatures(sym) {
			if out == nil {
				out = map[*symbols.Symbol][]*symbols.Symbol{}
			}
			out[target] = append(out[target], sym)
		}
		return true
	})
	return out
}

// redefinedClosure is sym together with every feature it redefines, directly or
// through a chain of redefinitions.
func (r *Resolver) redefinedClosure(sym *symbols.Symbol) map[*symbols.Symbol]bool {
	out := map[*symbols.Symbol]bool{}
	queue := []*symbols.Symbol{sym}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == nil || out[cur] {
			continue
		}
		out[cur] = true
		queue = append(queue, r.redefinedFeatures(cur)...)
	}
	return out
}

// redefinedByOther reports whether a member other than sym redefines one of the
// features sym's redefinition closure reaches: sym's own redefinition of an
// inherited feature must not remove sym from what its owner contributes.
func redefinedByOther(
	closure map[*symbols.Symbol]bool,
	byOwner map[*symbols.Symbol][]*symbols.Symbol,
	sym *symbols.Symbol,
) bool {
	for target := range closure {
		if redefinerOtherThan(byOwner[target], sym) {
			return true
		}
	}
	return false
}

// redefinerOtherThan reports whether redefiners holds a member other than sym.
func redefinerOtherThan(redefiners []*symbols.Symbol, sym *symbols.Symbol) bool {
	for _, redefiner := range redefiners {
		if redefiner != sym {
			return true
		}
	}
	return false
}

// duplicatesOf returns the members of others that make sym's name ambiguous:
// every one naming a different element of a conforming metaclass.
func (r *Resolver) duplicatesOf(sym *symbols.Symbol, others []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, other := range others {
		if r.sameElement(sym, other) || r.DistinguishableByMetaclass(sym, other) {
			continue
		}
		out = append(out, other)
	}
	return out
}

// othersOf is members without member itself.
func othersOf(members []*symbols.Symbol, member *symbols.Symbol) []*symbols.Symbol {
	out := make([]*symbols.Symbol, 0, len(members)-1)
	for _, m := range members {
		if m != member {
			out = append(out, m)
		}
	}
	return out
}

// DistinguishableByMetaclass reports whether neither member's element has a
// metaclass conforming to the other's (KerML 8.3.2.4.3); unknown is never distinguishable.
func (r *Resolver) DistinguishableByMetaclass(a, b *symbols.Symbol) bool {
	model, ok := r.model.(metaclassProvider)
	if !ok {
		return false
	}
	ea, eb := a, b
	if a.Kind == symbols.SymbolAlias {
		target, ok := r.ResolveAliasTarget(a)
		if !ok || target == nil {
			return false
		}
		ea = target
	}
	if b.Kind == symbols.SymbolAlias {
		target, ok := r.ResolveAliasTarget(b)
		if !ok || target == nil {
			return false
		}
		eb = target
	}
	ma, mb := model.MetaclassOf(ea), model.MetaclassOf(eb)
	if ma == nil || mb == nil {
		return false
	}
	return !model.Conforms(ma, mb) && !model.Conforms(mb, ma)
}

// sameElement reports whether two members name the same element, which no
// distinguishability rule objects to: an alias for what it sits beside is the
// same element under two memberships.
func (r *Resolver) sameElement(a, b *symbols.Symbol) bool {
	if a == b {
		return true
	}
	return r.aliasTarget(a) == b || r.aliasTarget(b) == a
}

// aliasTarget is what an alias names, or the symbol itself.
func (r *Resolver) aliasTarget(sym *symbols.Symbol) *symbols.Symbol {
	if target, ok := r.ResolveAliasTarget(sym); ok {
		return target
	}
	return sym
}

// duplicateName reports one indistinguishable name at the member declaring it,
// or at the whole member when the name is derived rather than declared. from
// names the namespaces a duplicate comes from, which the reference's wording
// carries for a name it did not find in the namespace itself.
func (r *Resolver) duplicateName(sym *symbols.Symbol, message string, from []*symbols.Symbol) {
	span := sym.NameSpan
	if span == (source.Span{}) || sym.Naming != symbols.NamedByDeclaration {
		span = sym.DeclSpan
	}
	if names := ownerNames(sym, from); len(names) > 0 {
		message = fmt.Sprintf("%s '%s' from %s", message, sym.Name, strings.Join(names, ", "))
	}
	r.reportDuplicate(span, message)
}

// duplicateInherited reports a name owner inherits twice, at owner's own
// declaration: no member of owner is at fault for it.
func (r *Resolver) duplicateInherited(owner *symbols.Symbol, name string, from []*symbols.Symbol) {
	message := "Duplicate of inherited member name"
	if names := ownerNames(nil, from); len(names) > 0 {
		message = fmt.Sprintf("%s '%s' from %s", message, name, strings.Join(names, ", "))
	}
	r.reportDuplicate(owner.DeclSpan, message)
}

func (r *Resolver) reportDuplicate(span source.Span, message string) {
	r.report(Diagnostic{
		Span:    span,
		Message: message,
		Code:    CodeNameConflict,
		Warning: true,
	})
}

// ownerNames are the distinct names of the namespaces the duplicates belong to,
// sorted, skipping the namespace the reported member is in.
func ownerNames(sym *symbols.Symbol, dups []*symbols.Symbol) []string {
	var own *symbols.Scope
	if sym != nil {
		own = sym.OwnerScope
	}
	seen := map[string]bool{}
	var out []string
	for _, dup := range dups {
		if dup.OwnerScope == nil || dup.OwnerScope == own {
			continue
		}
		owner := dup.OwnerScope.Owner()
		if owner == nil || seen[owner.Name] {
			continue
		}
		seen[owner.Name] = true
		out = append(out, owner.Name)
	}
	sort.Strings(out)
	return out
}

// DistinguishableMembers splits the members of scope whose names the
// distinguishability rules compare into owned members and aliases: those that
// bind a name, as LocalBinding finds them. Exported for the library-base half
// of the rule in internal/check/passes.
func (r *Resolver) DistinguishableMembers(scope *symbols.Scope) (owned, aliases []*symbols.Symbol) {
	for _, name := range scope.MemberNames() {
		for _, sym := range scope.LookupLocalAll(name) {
			// A member declaring both a short and a primary name is registered
			// under both keys; it is compared once, under its own name.
			if sym.Name != name || !contributesName(sym) || !r.BindsName(sym) {
				continue
			}
			if sym.Kind == symbols.SymbolAlias {
				aliases = append(aliases, sym)
				continue
			}
			owned = append(owned, sym)
		}
	}
	return owned, aliases
}

// contributesName reports whether a member contributes a name to its namespace,
// as Membership::memberName does in the reference: a member naming an existing
// feature rather than declaring one contributes none.
func contributesName(sym *symbols.Symbol) bool {
	if sym.Recorded() {
		return sym.Facts.Modifiers.Has(symbols.ModContributesName)
	}
	switch decl := sym.Decl.(type) {
	case *ast.Usage:
		if decl.Ident.Name == "" && decl.Ident.ShortName == "" {
			// An unnamed feature contributes the name its naming feature
			// gives it, if any (KerML 7.3.4.5).
			return sym.EffectiveName()
		}
		// A metadata usage without a typing is malformed (SysML.xtext
		// MetadataUsageDeclaration requires one) and contributes no name.
		if decl.Kind == ast.UsageMetadata && !hasTypingRelationship(decl) {
			return false
		}
		return true
	case *ast.Definition:
		return decl.Ident.Name != "" || decl.Ident.ShortName != ""
	}
	// A `first x` node borrows the name of the node it sequences.
	return sym.NameSpan != (source.Span{})
}

func hasTypingRelationship(decl *ast.Usage) bool {
	for _, rel := range decl.Relationships {
		if rel != nil && rel.Kind == ast.RelTyping {
			return true
		}
	}
	return false
}

// ImplicitlyRedefined reports whether a member takes an inherited feature's name
// by implicitly redefining it: a behavior parameter matched by position, and the
// subject, actors, stakeholders and objective of a requirement or case
// (KerML 7.3.4.5, SysML 7.18.4).
func ImplicitlyRedefined(sym *symbols.Symbol) bool {
	if sym.Recorded() {
		return sym.Facts.Modifiers.Has(symbols.ModImplicitlyRedefined)
	}
	if isParameter(sym) || inMetadataUsageBody(sym) {
		return true
	}
	switch decl := sym.Decl.(type) {
	case *ast.SubjectMember:
		return true
	case *ast.ConnectorEnd:
		// A connect-clause end redefines the end of the typing connector
		// definition at the same position (SysML 7.13.2).
		return true
	case *ast.Usage:
		// An end of a specializing association or connector redefines the
		// corresponding end by position (KerML 8.4.4.6).
		if decl.IsEnd {
			return true
		}
		switch decl.Kind {
		case ast.UsageSubject, ast.UsageActor, ast.UsageStakeholder, ast.UsageObjective:
			return true
		}
	}
	return false
}

// inMetadataUsageBody reports whether sym is a member of a metadata usage body,
// where the name is always an owned redefinition (SysML.xtext MetadataBodyUsage).
func inMetadataUsageBody(sym *symbols.Symbol) bool {
	for owner := sym.Owner(); owner != nil; owner = owner.Owner() {
		usage, ok := owner.Decl.(*ast.Usage)
		if !ok {
			return false
		}
		if usage.Kind == ast.UsageMetadata {
			return true
		}
	}
	return false
}

func byName(syms []*symbols.Symbol) map[string][]*symbols.Symbol {
	out := map[string][]*symbols.Symbol{}
	for _, sym := range syms {
		out[sym.Name] = append(out[sym.Name], sym)
	}
	return out
}
