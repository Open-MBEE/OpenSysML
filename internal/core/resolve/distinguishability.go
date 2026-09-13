package resolve

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// checkDistinguishability reports the member names of one namespace that are not
// distinguishable: an owned name repeating another owned name, and — for a type
// — an owned name repeating one the type inherits (KerML 7.2.2, SysML 7.6.1).
// Both are warnings, as in the reference implementation.
func (r *Resolver) checkDistinguishability(scope *symbols.Scope) {
	if scope == nil {
		return
	}
	r.checkOwnedNames(scope)
	r.checkInheritedNames(scope)
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
		if len(members) < 2 {
			continue
		}
		dups := r.duplicatesOf(members[0], members[1:])
		if len(dups) == 0 {
			continue
		}
		r.duplicateInherited(owner, name, members)
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
			if r.inheritsFrom(ownerOf(other), ownerOf(sym), model) {
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

// ownerOf is the namespace a member belongs to.
func ownerOf(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// hasUnresolvedRedefinition reports whether sym declares a redefinition whose
// target we could not resolve: the name it reuses is then not evidence of a
// duplicate. See docs/project/spec-compliance.md for the resolution gaps.
func (r *Resolver) hasUnresolvedRedefinition(sym *symbols.Symbol) bool {
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
	for _, imp := range r.importsOf(sup.Scope.Node()) {
		if imp.Visibility == ast.VisibilityPrivate {
			continue
		}
		for _, sym := range r.ImportedElementsInto(owner.Scope, sup.Scope, imp) {
			if sym != nil && sym.Name != "" && !r.idx.Library(sym) && contributesName(sym) && r.BindsName(sym) {
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
// every one naming a different element.
func (r *Resolver) duplicatesOf(sym *symbols.Symbol, others []*symbols.Symbol) []*symbols.Symbol {
	var out []*symbols.Symbol
	for _, other := range others {
		if r.sameElement(sym, other) {
			continue
		}
		out = append(out, other)
	}
	return out
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
// of the rule in internal/core/passes.
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
	for owner := ownerOf(sym); owner != nil; owner = ownerOf(owner) {
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
