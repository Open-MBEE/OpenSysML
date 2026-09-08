package passes

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Pilot KerMLValidator (2026-05) checkNamespace, INVALID_NAMESPACE_DISTINGUISHABILITY_MSG_2:
// an owned or inherited member name a type also inherits from elsewhere.
const msgW9CDuplicateInherited = "Duplicate of inherited member name"

// W9CInheritedNameConflictPass warns when a declaration reuses a member name a
// library definition it conforms to already supplies — `state start;` inside a
// state, against StatePerformances::StateAction::start — and when two such
// definitions each supply a feature of one name: `action a : ABlock` inherits
// `self`, `start` and `done` from both Actions::Action and Parts::Part. The
// resolver's own rule covers only the document's own supertypes.
type W9CInheritedNameConflictPass struct{}

func (W9CInheritedNameConflictPass) Level() PassLevel { return LevelType }

func (W9CInheritedNameConflictPass) Run(ctx *Context, name string, root *ast.RootNamespace) []Diagnostic {
	if ctx == nil || ctx.Index == nil || root == nil {
		return nil
	}
	rootScope := ctx.Index.DocumentRoot(name)
	if rootScope == nil {
		return nil
	}
	c := &w9cConflictChecker{
		model:    ctx.Model(),
		idx:      ctx.Index,
		resolver: ctx.Resolver(),
		members:  map[*symbols.Symbol]w9cContributor{},
		own:      map[*symbols.Symbol]w9cContributor{},
	}
	if c.model == nil {
		return nil
	}
	w8dWalkSymbols(ctx, rootScope, c.check)
	return c.diags
}

type w9cConflictChecker struct {
	model    *semantics.Model
	idx      *symbols.Index
	resolver *resolve.Resolver
	// members memoizes the visible member of each library base, per name.
	members map[*symbols.Symbol]w9cContributor
	// own memoizes the own members of each type passed through, per name.
	own   map[*symbols.Symbol]w9cContributor
	diags []Diagnostic
}

// w9cCandidate is one library base's contribution of a member name, with the
// type in that base's chain that declares it.
type w9cCandidate struct {
	declaredBy *symbols.Symbol
	member     *symbols.Symbol
}

func (c *w9cConflictChecker) check(sym *symbols.Symbol) {
	switch d := sym.Decl.(type) {
	case *ast.Usage:
		// A variant only references a member of its variation; it declares no
		// specialization of its own for a diamond to be drawn through.
		if d.IsVariant {
			return
		}
	case *ast.Definition:
	default:
		return
	}
	if c.idx.Library(sym) {
		return
	}
	reach := c.libraryBases(sym)
	if len(reach.bases) == 0 {
		return
	}
	contribs := c.contributors(reach)
	c.checkOwnedNames(sym, contribs)
	if len(contribs) < 2 {
		// One base contributes each name once, so no name is reached twice.
		return
	}
	var names []string
	for _, name := range sharedNames(contribs) {
		if !c.declares(sym, name) {
			names = append(names, name)
		}
	}
	spans := append([]source.Span{sym.DeclSpan}, c.chainSpans(sym)...)
	for _, name := range names {
		if from := c.conflictingBases(contributionsOf(contribs, name)); len(from) > 1 {
			for _, span := range spans {
				c.report(span, name, from)
			}
		}
	}
}

// w9cContributor is one source of inherited members, by the name it supplies each under.
type w9cContributor map[string]w9cCandidate

// contributors are the memoized member maps of each library base, then of each
// type passed on the way there; they are shared and looked up, never merged.
func (c *w9cConflictChecker) contributors(reach w9cReach) []w9cContributor {
	out := make([]w9cContributor, 0, len(reach.bases)+len(reach.through))
	for _, base := range reach.bases {
		out = append(out, c.baseMembers(base))
	}
	for _, via := range reach.through {
		out = append(out, c.throughMembers(via))
	}
	return out
}

// contributionsOf is each contributor's member of one name, in contributor order.
func contributionsOf(contribs []w9cContributor, name string) []w9cCandidate {
	var out []w9cCandidate
	for _, members := range contribs {
		if cand, ok := members[name]; ok {
			out = append(out, cand)
		}
	}
	return out
}

// sharedNames are the names two or more contributors supply, sorted; the
// largest map is only looked up, since a name it alone supplies is reached once.
func sharedNames(contribs []w9cContributor) []string {
	largest := 0
	for i, members := range contribs {
		if len(members) > len(contribs[largest]) {
			largest = i
		}
	}
	seen := map[string]bool{}
	var names []string
	for i, members := range contribs {
		if i == largest {
			continue
		}
		for name := range members {
			if seen[name] {
				continue
			}
			seen[name] = true
			if len(contributionsOf(contribs, name)) > 1 {
				names = append(names, name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// throughMembers are the own members of a type passed on the way to a library base. Memoized.
func (c *w9cConflictChecker) throughMembers(via *symbols.Symbol) w9cContributor {
	if cached, ok := c.own[via]; ok {
		return cached
	}
	out := w9cContributor{}
	for name, member := range c.ownMembers(via) {
		out[name] = w9cCandidate{declaredBy: via, member: member}
	}
	c.own[via] = out
	return out
}

// ownMembers is the visible member sym declares under each name, its short
// name included (KerML 7.2.2). A document that re-declares a library file
// shares its names, so only sym's copy counts.
func (c *w9cConflictChecker) ownMembers(sym *symbols.Symbol) map[string]*symbols.Symbol {
	out := map[string]*symbols.Symbol{}
	// The index carries a library type's nested names in both cache states;
	// its scope exists only when the library was parsed.
	for _, member := range c.idx.LookupDirectChildren(symbols.FQNOf(sym)) {
		name := leafOf(member.Name)
		if name == "" || member.Visibility == ast.VisibilityPrivate ||
			c.idx.Library(member) != c.idx.Library(sym) {
			continue
		}
		out[name] = member
		if short := shortNameOf(member); short != "" && short != name {
			out[short] = member
		}
	}
	return out
}

// shortNameOf is a symbol's short name, read from its declaration when the
// index did not record one.
func shortNameOf(sym *symbols.Symbol) string {
	if sym.ShortName != "" {
		return sym.ShortName
	}
	id, _ := symbols.DeclIdent(sym.Decl)
	return id.ShortName
}

// checkOwnedNames reports each member sym declares whose name a library base
// already contributes (KerML 8.4.3.2): the two are indistinguishable unless the
// declaration redefines or subsets the inherited feature.
func (c *w9cConflictChecker) checkOwnedNames(sym *symbols.Symbol, contribs []w9cContributor) {
	if sym.Scope == nil || resolve.ParameterizedByName(sym) {
		return
	}
	owned, aliases := c.resolver.DistinguishableMembers(sym.Scope)
	for _, mems := range [2][]*symbols.Symbol{owned, aliases} {
		for _, mem := range mems {
			if resolve.ImplicitlyRedefined(mem) || c.hasUnresolvedRedefinition(mem) {
				continue
			}
			for _, key := range ownedKeysOf(mem) {
				cands := contributionsOf(contribs, key.name)
				if len(cands) == 0 {
					continue
				}
				if from := c.conflictingBases(c.notSpecializedBy(mem, cands)); len(from) > 0 {
					c.report(key.span, key.name, from)
				}
			}
		}
	}
}

// ownedKeysOf is each identifier a member or alias binds, short name included,
// at the span it was written; a member naming itself another way binds its own name.
func ownedKeysOf(mem *symbols.Symbol) []w9cKey {
	id, ok := symbols.DeclIdent(mem.Decl)
	if !ok || (id.Name == "" && id.ShortName == "") {
		return []w9cKey{{name: mem.Name, span: nameSpanOf(mem)}}
	}
	var keys []w9cKey
	if id.ShortName != "" {
		keys = append(keys, w9cKey{name: id.ShortName, span: id.ShortNameSpan})
	}
	if id.Name != "" && id.Name != id.ShortName {
		keys = append(keys, w9cKey{name: id.Name, span: id.NameSpan})
	}
	for i := range keys {
		if keys[i].span == (source.Span{}) {
			keys[i].span = nameSpanOf(mem)
		}
	}
	return keys
}

// notSpecializedBy drops the inherited features mem redefines, subsets or is an
// alias for, which it is then free to reuse the name of.
func (c *w9cConflictChecker) notSpecializedBy(
	mem *symbols.Symbol,
	cands []w9cCandidate,
) []w9cCandidate {
	target := c.aliasTarget(mem)
	out := make([]w9cCandidate, 0, len(cands))
	for _, cand := range cands {
		if cand.member == mem || cand.member == target || c.specializes(mem, cand.member) {
			continue
		}
		out = append(out, cand)
	}
	return out
}

// aliasTarget is the element an alias names, or nil for any other member.
func (c *w9cConflictChecker) aliasTarget(mem *symbols.Symbol) *symbols.Symbol {
	if c.resolver == nil {
		return nil
	}
	if target, ok := c.resolver.ResolveAliasTarget(mem); ok {
		return target
	}
	return nil
}

// hasUnresolvedRedefinition reports whether sym declares a redefinition whose
// target we could not resolve: the reused name is then no evidence of a
// duplicate, since the unresolved target may be the feature it redefines.
func (c *w9cConflictChecker) hasUnresolvedRedefinition(sym *symbols.Symbol) bool {
	if c.resolver == nil {
		return false
	}
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		if _, ok := c.resolver.ResolveTarget(sym.OwnerScope, rel.Target); !ok {
			return true
		}
	}
	return false
}

// nameSpanOf is where a member's name was written, or its declaration.
func nameSpanOf(sym *symbols.Symbol) source.Span {
	if sym.NameSpan != (source.Span{}) {
		return sym.NameSpan
	}
	return sym.DeclSpan
}

// chainSpans are the spans of the feature chains sym references. A chain is an
// owned feature of its own (KerML feature chaining), specialized as the
// referencing usage is, so it repeats that usage's conflicts.
func (c *w9cConflictChecker) chainSpans(sym *symbols.Symbol) []source.Span {
	var out []source.Span
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelReferences || rel.Target == nil {
			continue
		}
		if _, ok := rel.Target.(*ast.FeatureChainExpr); ok {
			out = append(out, rel.Target.Span())
			continue
		}
		qname := ast.AsQualifiedName(rel.Target)
		if qname == nil {
			continue
		}
		for _, part := range qname.Parts {
			if part.Chained {
				out = append(out, rel.Target.Span())
				break
			}
		}
	}
	return out
}

// w9cReach is the nearest library definitions a symbol conforms to and the
// types and features passed through on the way to them.
type w9cReach struct {
	bases   []*symbols.Symbol
	through []*symbols.Symbol
}

// libraryBases are the nearest library definitions sym conforms to, reached
// through the document's own types and through features where those intervene.
func (c *w9cConflictChecker) libraryBases(sym *symbols.Symbol) w9cReach {
	var out w9cReach
	seen := map[*symbols.Symbol]bool{sym: true}
	var walk func(*symbols.Symbol)
	walk = func(cur *symbols.Symbol) {
		if cur != sym {
			out.through = append(out.through, cur)
		}
		sups := c.model.DirectSupertypes(cur)
		for _, sup := range sups {
			if sup == nil || seen[sup] {
				continue
			}
			// A feature's implicit value typing is only added when nothing else
			// classifies it: a definition it is typed by, a subsetting or a
			// redefinition supplies its type, and a feature it is typed by does
			// not, so only then is a diamond drawn through Base::DataValue.
			if symbols.HasFQN(sup, "Base::DataValue") && !isDefKind(cur.Kind) &&
				!c.typedByFeatureOnly(cur) {
				continue
			}
			seen[sup] = true
			// A feature contributes through the type that types it, not as a base.
			if c.idx.Library(sup) && (isDefKind(sup.Kind) || sup.Kind == symbols.SymbolKerMLType) {
				out.bases = append(out.bases, sup)
				continue
			}
			walk(sup)
		}
		// A reference subsetting (`::>`, `perform b.a`) is a subsetting, so the
		// referenced feature's type is a base of the referencing usage too.
		for _, ref := range c.referenceSubsettings(cur) {
			if seen[ref] {
				continue
			}
			seen[ref] = true
			walk(ref)
		}
	}
	walk(sym)
	return out
}

// referenceSubsettings are the features sym references (`::>` or the reference
// form of `perform`/`exhibit`/`include`).
func (c *w9cConflictChecker) referenceSubsettings(sym *symbols.Symbol) []*symbols.Symbol {
	if c.resolver == nil || sym == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelReferences || rel.Target == nil {
			continue
		}
		target, ok := c.resolver.ResolveTarget(sym.OwnerScope, rel.Target)
		if !ok || target == nil || target == sym {
			continue
		}
		out = append(out, target)
	}
	return out
}

// conflictingBases names the types declaring the features one name reaches
// that survive: a feature another candidate redefines is not inherited, and
// two candidates naming the same feature are one member reached twice.
// Conflicts among the document's own members alone are the resolver's.
func (c *w9cConflictChecker) conflictingBases(cands []w9cCandidate) []string {
	seen := map[*symbols.Symbol]bool{}
	names := map[string]bool{}
	library := false
	for _, cand := range cands {
		if seen[cand.member] || c.redefinedByOther(cand.member, cands) ||
			c.declaredBySubtype(cand, cands) {
			continue
		}
		seen[cand.member] = true
		library = library || c.idx.Library(cand.member)
		names[leafOf(symbols.FQNOf(cand.declaredBy))] = true
	}
	if !library {
		return nil
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// declaredBySubtype reports whether another candidate declares the name in a
// subtype of this one's declaring type, which supplies the nearer member.
func (c *w9cConflictChecker) declaredBySubtype(cand w9cCandidate, cands []w9cCandidate) bool {
	for _, other := range cands {
		if other.declaredBy != cand.declaredBy && c.specializes(other.declaredBy, cand.declaredBy) {
			return true
		}
	}
	return false
}

// redefinedByOther reports whether another candidate's feature specializes
// member, directly or transitively: a redefinition hides what it redefines.
func (c *w9cConflictChecker) redefinedByOther(member *symbols.Symbol, cands []w9cCandidate) bool {
	for _, other := range cands {
		if other.member != member && c.specializes(other.member, member) {
			return true
		}
	}
	return false
}

// specializes reports whether sub reaches sup through its supertypes, which for
// a feature includes the features it redefines and subsets.
func (c *w9cConflictChecker) specializes(sub, sup *symbols.Symbol) bool {
	seen := map[*symbols.Symbol]bool{sub: true}
	queue := c.model.DirectSupertypes(sub)
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
		queue = append(queue, c.model.DirectSupertypes(cur)...)
	}
	return false
}

// baseMembers is the member a library base makes visible under each name, the
// closest one when several of its supertypes supply it. Memoized per base.
func (c *w9cConflictChecker) baseMembers(base *symbols.Symbol) w9cContributor {
	if cached, ok := c.members[base]; ok {
		return cached
	}
	out := w9cContributor{}
	c.members[base] = out
	seen := map[*symbols.Symbol]bool{}
	queue := []*symbols.Symbol{base}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur == nil || seen[cur] {
			continue
		}
		seen[cur] = true
		for name, member := range c.ownMembers(cur) {
			if _, taken := out[name]; taken {
				continue
			}
			out[name] = w9cCandidate{declaredBy: cur, member: member}
		}
		queue = append(queue, c.model.DirectSupertypes(cur)...)
	}
	return out
}

// declares reports whether sym binds name itself, which masks what it inherits.
func (c *w9cConflictChecker) declares(sym *symbols.Symbol, name string) bool {
	if sym.Scope == nil {
		return false
	}
	_, ok := sym.Scope.LookupLocal(name)
	return ok
}

func (c *w9cConflictChecker) report(span source.Span, name string, from []string) {
	c.diags = append(c.diags, Diagnostic{
		Severity: SeverityWarning,
		Span:     span,
		Message:  fmt.Sprintf("%s '%s' from %s", msgW9CDuplicateInherited, name, strings.Join(from, ", ")),
		Code:     "name-conflict",
		Source:   "name-resolution",
	})
}

func leafOf(fqn string) string {
	if i := strings.LastIndex(fqn, "::"); i >= 0 {
		return fqn[i+2:]
	}
	return fqn
}

// typedByFeatureOnly reports whether sym is typed by at least one feature and
// classified by nothing else: no definition types it, and it neither subsets
// nor redefines another feature.
func (c *w9cConflictChecker) typedByFeatureOnly(sym *symbols.Symbol) bool {
	typedByFeature := false
	for _, rel := range semantics.RelationshipsOf(sym) {
		if rel == nil {
			continue
		}
		switch rel.Kind {
		case ast.RelSubsets, ast.RelRedefines, ast.RelReferences, ast.RelSpecializes:
			return false
		case ast.RelTyping:
			target, ok := c.resolver.ResolveTarget(sym.OwnerScope, rel.Target)
			if !ok || target == nil || isDefKind(target.Kind) {
				return false
			}
			typedByFeature = true
		}
	}
	return typedByFeature
}
