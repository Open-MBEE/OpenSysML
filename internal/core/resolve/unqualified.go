package resolve

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// walkUnqualified searches the scope and its ancestors for a local match or an
// imported match, then falls back to the document root and finally the global index.
func (r *Resolver) walkUnqualified(scope *symbols.Scope, name string) resolution {
	return r.walkUnqualifiedHiding(scope, name, nil)
}

// LookupName resolves an unqualified reference from scope the way a written one
// resolves — the enclosing scope chain, inherited members, imports, then the
// global index — but records no diagnostic when it finds nothing.
//
// It is what an evaluator asks: the names it looks up are not all references the
// source wrote down (a value bound at run time is looked up the same way), and a
// miss is for the evaluator to report against the value it was reading.
func (r *Resolver) LookupName(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	res := r.walkUnqualified(scope, name)
	return res.sym, res.ok
}

// walkUnqualifiedHiding is walkUnqualified with the bindings hide covers made
// invisible. Only those bindings are hidden: each scope's inherited members and
// imports are still consulted, so a perform statement's borrowed name does not
// hide the action its owner inherits from its type.
func (r *Resolver) walkUnqualifiedHiding(scope *symbols.Scope, name string, hide *refFilter) resolution {
	for s := scope; s != nil; s = s.Parent() {
		if sym, ok := r.localBinding(s, name, hide); ok {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.acceptPayload(s, name); ok && !hide.hides(sym) {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.implicitlyNamedMember(s, name, hide); ok {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.visibleMember(r.scopeOwner(s), name, hide); ok {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.lookupInheritedImports(s, name); ok && !hide.hides(sym) {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.enclosingLocal(s.Parent(), name, hide); ok {
			return resolution{sym: sym, ok: true}
		}

		if sym, ok := r.lookupImports(s, name); ok && !hide.hides(sym) {
			return resolution{sym: sym, ok: true}
		}
	}
	if root := rootOf(scope); root != nil {
		if sym, ok := r.localBinding(root, name, hide); ok {
			return resolution{sym: sym, ok: true}
		}
	}
	if sym, ok := r.nestedInRedefined(scope, name, hide); ok {
		return resolution{sym: sym, ok: true}
	}
	// Final fallback: check global index (cross-document top-level names)
	if sym := r.lookupGlobalTop(scope, name); sym != nil && !hide.hides(sym) {
		return resolution{sym: sym, ok: true}
	}
	return resolution{}
}

func (r *Resolver) enclosingLocal(scope *symbols.Scope, name string, hide *refFilter) (*symbols.Symbol, bool) {
	for ; scope != nil; scope = scope.Parent() {
		if sym, ok := r.localBinding(scope, name, hide); ok {
			return sym, true
		}
	}
	return nil, false
}

// localBinding is lookupLocal less the members that bind no name of their own.
func (r *Resolver) localBinding(scope *symbols.Scope, name string, hide *refFilter) (*symbols.Symbol, bool) {
	sym, ok := hide.lookupLocal(scope, name)
	if !ok || !r.BindsName(sym) {
		return nil, false
	}
	return sym, true
}

// LocalBindings returns the members scope declares under name that bind it,
// declared names first.
func (r *Resolver) LocalBindings(scope *symbols.Scope, name string) []*symbols.Symbol {
	if scope == nil {
		return nil
	}
	all := symbols.PreferDeclared(scope.LookupLocalAll(name))
	out := make([]*symbols.Symbol, 0, len(all))
	for _, sym := range all {
		if r.BindsName(sym) {
			out = append(out, sym)
		}
	}
	return out
}

// LocalBinding returns the first of LocalBindings.
func (r *Resolver) LocalBinding(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	if all := r.LocalBindings(scope, name); len(all) > 0 {
		return all[0], true
	}
	return nil, false
}

// BindsName reports whether sym binds the name it is registered under. A
// feature with no declared name takes the name of the feature it redefines or
// references, so it binds none when that target resolves to nothing, or to no
// feature (KerML 7.3.4.5).
func (r *Resolver) BindsName(sym *symbols.Symbol) bool {
	if r == nil || sym == nil || sym.Naming == symbols.NamedByDeclaration {
		return true
	}
	if named, done := r.effNames[sym]; done {
		return named
	}
	if r.naming[sym] {
		return true
	}
	r.naming[sym] = true
	defer delete(r.naming, sym)
	r.Enter()
	named := true
	r.aside(func() {
		if sym.Naming == symbols.NamedByReference {
			named = r.referencesFeature(sym.OwnerScope, sym.Decl, sym.NamingTarget)
		} else {
			named = r.namesVisibleFeature(sym.OwnerScope, sym.Decl, sym.NamingTarget)
		}
	})
	// A lookup cut short by a guard answered for an enclosing one, not for good.
	if r.Leave() {
		r.effNames[sym] = named
	}
	return named
}

// referencesFeature reports whether the reference subsetting target owned by
// decl names a feature: a definition or nothing at all names no feature.
func (r *Resolver) referencesFeature(scope *symbols.Scope, decl ast.Node, target ast.Node) bool {
	if target == nil {
		return true
	}
	found, ok := r.resolveTarget(scope, target, referenceFilter(decl, target))
	if !ok {
		return false
	}
	if alias, isAlias := r.ResolveAliasTarget(found); isAlias {
		found = alias
	}
	return found.IsFeature()
}

// namesVisibleFeature reports whether a redefinition target owned by decl names
// a feature the redefinition can see, chain segments included (KerML 8.2.3.5).
func (r *Resolver) namesVisibleFeature(scope *symbols.Scope, decl ast.Node, target ast.Node) bool {
	hide := &refFilter{decl: decl, skipBorrowedName: true, redefining: true}
	chain, ok := target.(*ast.FeatureChainExpr)
	if !ok {
		if qn := ast.AsQualifiedName(target); qn != nil {
			_, resolved := r.resolveQualified(scope, qn, hide)
			return resolved
		}
		return true
	}
	cur, ok := r.resolveTarget(scope, chain.Operand, hide.forPrefix())
	if !ok || chain.Member == nil {
		return false
	}
	for _, part := range chain.Member.Parts {
		if cur, ok = r.chainMember(cur, part.Text, nil); !ok {
			return false
		}
	}
	return true
}

// visibleMember resolves name as a member of sym, skipping what hide covers, so
// that a feature which borrowed a name does not mask the one it took it from.
func (r *Resolver) visibleMember(sym *symbols.Symbol, name string, hide *refFilter) (*symbols.Symbol, bool) {
	// What a feature of sym redefines, sym does not inherit, so no name of it
	// resolves here (KerML 8.3.3.3.6); only a redefinition of it is exempt.
	admits := func(found *symbols.Symbol) (*symbols.Symbol, bool) {
		if !visibleAsInheritedMember(sym, found) {
			return nil, false
		}
		return r.inheritedAsFrom(sym, found, hide)
	}
	if hide.contributedOnly() {
		// The owner's own declarations are the local bindings already filtered
		// by the caller, so only contributed ones remain.
		found, ok := r.lookupContributedMember(sym, name, hide)
		if !ok {
			return nil, false
		}
		return admits(found)
	}
	found, ok := r.lookupMember(sym, name, hide)
	if !ok {
		return nil, false
	}
	if found, ok = admits(found); !ok {
		return nil, false
	}
	if !hide.hides(found) {
		return found, true
	}
	found, ok = r.lookupContributedMember(sym, name, hide)
	if !ok {
		return nil, false
	}
	return admits(found)
}

// implicitlyNamedMember returns the anonymous member of scope that binds name by
// implicitly redefining a parameter so called (KerML 7.3.4.5, SysML 7.6.5):
// `action shoot : Shoot { in item; }` binds `image`. That target is known only
// to the semantic model, hence the binding here and not when scopes are built.
func (r *Resolver) implicitlyNamedMember(scope *symbols.Scope, name string, hide *refFilter) (*symbols.Symbol, bool) {
	if scope == nil || name == "" || !scope.HasAnonymousMembers() {
		return nil, false
	}
	model, ok := r.model.(supertypeProvider)
	if !ok {
		return nil, false
	}
	for _, sym := range scope.AnonymousMembers() {
		if r.naming[sym] || hide.hides(sym) || !impliesNamingFeature(sym) {
			continue
		}
		r.naming[sym] = true
		var redefined *symbols.Symbol
		count := 0
		for _, sup := range model.DirectSupertypes(sym) {
			if isParameter(sup) {
				redefined = sup
				count++
			}
		}
		delete(r.naming, sym)
		// One redefinition names the feature; several need not agree on a
		// name, so the feature stays anonymous (KerML 7.3.4.5).
		if count == 1 && simpleName(redefined) == name {
			return sym, true
		}
	}
	return nil, false
}

// impliesNamingFeature reports whether sym is a nameless parameter whose naming
// feature is implicit: any declared relationship other than a typing would have
// named it, or ruled it out, when scopes were built.
func impliesNamingFeature(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || !isParameter(sym) {
		return false
	}
	for _, rel := range usage.Relationships {
		if rel != nil && rel.Kind != ast.RelTyping {
			return false
		}
	}
	return true
}

// isParameter reports whether sym is declared as a directed feature or a
// result, the features an implicit redefinition matches (SysML 7.6.5).
func isParameter(sym *symbols.Symbol) bool {
	usage, ok := sym.Decl.(*ast.Usage)
	return ok && (usage.Direction != ast.DirNone || usage.IsResult)
}

// simpleName is sym's own name without the qualification an indexed symbol
// carries.
func simpleName(sym *symbols.Symbol) string {
	if i := strings.LastIndex(sym.Name, "::"); i >= 0 {
		return sym.Name[i+2:]
	}
	return sym.Name
}

// lookupImports checks every import declared directly in scope for a member
// matching name.
func (r *Resolver) lookupImports(scope *symbols.Scope, name string) (*symbols.Symbol, bool) {
	node := scope.Node()
	for _, imp := range r.importsOf(node) {
		if r.resolvingImports[imp] {
			continue
		}
		if !r.importPrefixAvailable(scope, imp, name) {
			continue
		}
		if sym, ok := r.matchImport(scope, imp, name); ok {
			return sym, true
		}
	}
	return nil, false
}

// lookupImportedMember resolves a segment surfaced by the namespace being
// traversed, including a public membership import.
func (r *Resolver) lookupImportedMember(target *symbols.Symbol, targetScope, from *symbols.Scope, name string) (*symbols.Symbol, bool) {
	for _, imp := range r.importsOf(targetScope.Node()) {
		if r.importStack[imp] {
			continue
		}
		if !r.importPrefixAvailable(targetScope, imp, name) {
			continue
		}
		if !r.importVisibleFrom(target, from, imp) {
			continue
		}
		// An expose inherited by specialization is judged by the inheriting view too.
		into := targetScope
		if imp.IsExpose && from != nil && r.specializes(from.Owner(), target) {
			into = from
		}
		r.importStack[imp] = true
		if sym, ok := r.matchImportInto(into, targetScope, imp, name); ok {
			delete(r.importStack, imp)
			return sym, true
		}
		delete(r.importStack, imp)
	}
	return nil, false
}

func (r *Resolver) importVisibleFrom(target *symbols.Symbol, from *symbols.Scope, imp *ast.Import) bool {
	if imp.Visibility == ast.VisibilityProtected {
		var owner *symbols.Symbol
		if from != nil {
			owner = from.Owner()
		}
		return inheritedThroughSpecialization(imp) && r.specializes(owner, target)
	}
	if imp.Visibility != ast.VisibilityPrivate && !imp.IsExpose {
		return true
	}
	targetFQN := r.registeredFQN(target)
	fromFQN := r.ReferringNamespaceFQN(from)
	return targetFQN != "" && (fromFQN == targetFQN || strings.HasPrefix(fromFQN, targetFQN+"::"))
}

// importsOf is importsOf memoized: the tree is immutable once parsed, and every
// reference in a namespace looks its name up against that namespace's imports.
func (r *Resolver) importsOf(node ast.Node) []*ast.Import {
	if node == nil {
		return nil
	}
	if imports, ok := r.imports[node]; ok {
		return imports
	}
	imports := importsOf(node)
	journalNew(r, r.imports, node, node)
	r.imports[node] = imports
	return imports
}

// importsOf returns the *ast.Import declarations directly in a namespace-bearing
// node. In KerML an Import is a Relationship owned by any Namespace, and a
// definition or usage body is itself a Namespace, so imports declared inside a
// definition/usage body apply to that body's scope (and, through the ordinary
// parent-scope walk, to every scope nested within it).
func importsOf(node ast.Node) []*ast.Import {
	var members []ast.Node
	switch n := node.(type) {
	case *ast.Package:
		members = n.Members
	case *ast.Namespace:
		members = n.Members
	case *ast.RootNamespace:
		members = n.Members
	case *ast.Definition:
		members = n.Members
	case *ast.Usage:
		members = n.Members
	default:
		return nil
	}
	var out []*ast.Import
	for _, m := range members {
		if imp, ok := m.(*ast.Import); ok {
			out = append(out, imp)
		}
	}
	return out
}

// matchImport tries to satisfy name through a single import declaration.
//
// An element the import's filter clause rejects, or one rejected by a `filter`
// member of the namespace declaring the import, is not brought in at all — so a
// name only such an element bears does not resolve through this import
// (KerML 8.2.4, SysML v2 7.4.4). A rejected element does not end the search
// either: another element of the same name elsewhere in an imported subtree may
// be admitted.
func (r *Resolver) matchImport(scope *symbols.Scope, imp *ast.Import, name string) (*symbols.Symbol, bool) {
	return r.matchImportInto(scope, scope, imp, name)
}

// matchImportInto is matchImport for an import inherited into the namespace
// owning into, whose conditions an inherited expose has to satisfy as well.
func (r *Resolver) matchImportInto(into, scope *symbols.Scope, imp *ast.Import, name string) (*symbols.Symbol, bool) {
	var found *symbols.Symbol
	r.eachImportMatch(into, scope, imp, name, func(sym *symbols.Symbol) bool {
		found = sym
		return false
	})
	return found, found != nil
}

// importMatchesAll is matchImport collecting every element the import surfaces
// under name, in the order matchImport would find them, without duplicates.
func (r *Resolver) importMatchesAll(scope *symbols.Scope, imp *ast.Import, name string) []*symbols.Symbol {
	return r.importMatchesAllInto(scope, scope, imp, name)
}

// importMatchesAllInto is importMatchesAll for an import inherited into the
// namespace owning into (see matchImportInto).
func (r *Resolver) importMatchesAllInto(into, scope *symbols.Scope, imp *ast.Import, name string) []*symbols.Symbol {
	var out []*symbols.Symbol
	r.eachImportMatch(into, scope, imp, name, func(sym *symbols.Symbol) bool {
		out = appendSymbol(out, sym)
		return true
	})
	return out
}

// eachImportMatch calls yield with each element imp surfaces under name until
// yield returns false.
func (r *Resolver) eachImportMatch(into, scope *symbols.Scope, imp *ast.Import, name string, yield func(*symbols.Symbol) bool) {
	if imp.Imported == nil || len(imp.Imported.Parts) == 0 {
		return
	}
	if r.resolvingImports[imp] {
		return
	}
	r.resolvingImports[imp] = true
	// Resolved aside: a miss here may only mean sibling imports were suspended
	// for cycle safety, so it must not be memoized or reported as unresolved.
	var target *symbols.Symbol
	var ok bool
	r.aside(func() { target, ok = r.resolveImportTarget(scope, imp) })
	delete(r.resolvingImports, imp)
	if !ok {
		return
	}
	admit := r.importAdmitsInto(into, scope, imp)
	if imp.Kind == ast.ImportMembership {
		// A membership import names a membership: `import P::Car` where Car is an
		// alias imports that name, so the alias is what it surfaces.
		if alias, isAlias := r.PartAlias(imp.Imported, len(imp.Imported.Parts)-1); isAlias {
			target = alias
		}
		// The imported member itself (last segment) is visible by its own name.
		// For FQN-indexed symbols (stdlib), target.Name may be the full FQN, so extract last segment.
		targetName := target.Name
		if idx := strings.LastIndex(targetName, "::"); idx >= 0 {
			targetName = targetName[idx+2:]
		}
		if (targetName == name || (target.ShortName != "" && target.ShortName == name)) && admit(target) {
			if !yield(target) {
				return
			}
		}
		if imp.IsRecursive {
			r.eachSubtreeMatch(scope, target, name, imp, admit, yield)
		}
		return
	}
	// Namespace import: visible members of the target's scope are surfaced.
	// Check scope first if available
	if target.Scope != nil {
		for _, sym := range symbols.PreferDeclared(target.Scope.LookupLocalAll(name)) {
			if visibleThroughImport(imp, sym) && admit(sym) && !yield(sym) {
				return
			}
		}
		if sym, ok := r.lookupImportedMember(target, target.Scope, scope, name); ok && symbols.VisibleOutside(sym.Visibility) && admit(sym) {
			if !yield(sym) {
				return
			}
		}
	}
	// Also check FQN index for re-exported symbols (wildcard imports populate index, not scope)
	if r.idx != nil {
		// A name only a private import surfaced under the target is a member of
		// it but not a visible one, so a wildcard import does not re-export it
		// (KerML 8.2.3.3) — unless this is an `import all`, which takes the
		// target's private memberships too.
		targetFQN := r.registeredFQN(target)
		var children []*symbols.Symbol
		if importAllowsPrivate(imp) {
			children = r.idx.LookupDirectChildrenNamed(targetFQN, name)
		} else {
			children = r.idx.LookupDirectChildrenNamedFrom(targetFQN, r.ReferringNamespaceFQN(scope), name)
		}
		// The import names one namespace, so a same-named other namespace's
		// members registered under the same path are not what it surfaces.
		children = notConflatedWith(target, children)
		for _, sym := range children {
			// The target may itself have surfaced the name through an import of
			// its own, filtered by its `filter` members: what it re-exports
			// onward is what those select.
			if !r.admitsUnderName("", r.ReferringNamespaceFQN(scope), targetFQN+"::"+name, sym) {
				continue
			}
			if r.importSurfaces(imp, targetFQN, sym) && admit(sym) {
				if !yield(sym) {
					return
				}
			}
		}
	}
	if imp.IsRecursive {
		r.eachSubtreeMatch(scope, target, name, imp, admit, yield)
	}
}

func (r *Resolver) importPrefixAvailable(scope *symbols.Scope, imp *ast.Import, name string) bool {
	if len(r.resolvingImports) == 0 || imp.Imported == nil || len(imp.Imported.Parts) == 0 {
		return true
	}
	// During recursive import resolution, avoid re-entering a path with no independent prefix binding.
	prefix := imp.Imported.Parts[0].Text
	if prefix == name {
		return true
	}
	for s := scope; s != nil; s = s.Parent() {
		if _, ok := s.LookupLocal(prefix); ok {
			return true
		}
	}
	if r.idx != nil && len(r.idx.LookupQualified(prefix)) > 0 {
		return true
	}
	return false
}

// eachSubtreeMatch yields each element named name in the target namespace and its visible
// descendants, nested re-exports included, until yield returns false.
func (r *Resolver) eachSubtreeMatch(scope *symbols.Scope, target *symbols.Symbol, name string, imp *ast.Import, admit func(*symbols.Symbol) bool, yield func(*symbols.Symbol) bool) {
	out := newElementList()
	r.appendSubtree(out, scope, target, imp, admit, map[symbols.ElementKey]bool{})
	for _, sym := range out.elems {
		if (localNameOf(sym) == name || sym.ShortName == name) && !yield(sym) {
			return
		}
	}
}

// LookupNameExcluding resolves name from scope as LookupName does, with decl's
// own binding hidden: what the name resolves to where decl does not declare it.
func (r *Resolver) LookupNameExcluding(scope *symbols.Scope, name string, decl ast.Node) (*symbols.Symbol, bool) {
	res := r.walkUnqualifiedHiding(scope, name, &refFilter{decl: decl})
	return res.sym, res.ok
}
