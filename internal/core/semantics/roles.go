package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

type caseRole uint8

const (
	noCaseRole caseRole = iota
	subjectRole
	objectiveRole
	// actorRole is an actor or stakeholder: a parameter after the subject (SysML v2 §8.3.20
	// ActorMembership, StakeholderMembership), redefined by position like any parameter.
	actorRole
)

// viewRenderingFQN is the library feature every `render` member redefines
// (SysML v2 8.3.26 RenderingUsage).
const viewRenderingFQN = "Views::View::viewRendering"

// ImplicitRoleRedefinitions returns the features sym redefines by its role: the library
// `viewRendering` for a view's `render` member, and the same-role features of the owner's
// generals that sym does not redefine by name: every one for a subject, each general's
// first for a first objective. An analysis case may state several objectives, and each
// one redefines the general's effective objective at the same position. An actor or
// stakeholder redefines each general's effective actor at its position (KerML §7.4.7.3).
func (m *Model) ImplicitRoleRedefinitions(sym *symbols.Symbol) []*symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	if isViewRendering(sym.Decl) {
		return m.viewRenderingRedefinition(sym)
	}
	role := roleOf(sym)
	if role == noCaseRole {
		return nil
	}
	owner := sym.OwnerScope.Owner()
	if owner == nil || !behaviorLike(owner) {
		return nil
	}
	if role == actorRole {
		return m.positionalActorRedefinitions(owner, sym)
	}
	if role == objectiveRole {
		position := rolePosition(owner, role, sym)
		if position < 0 || (position > 0 && !analysisCase(owner)) {
			return nil
		}
	}
	var out []*symbols.Symbol
	seenCases := map[*symbols.Symbol]bool{}
	walk := newRoleWalk()
	seenRoles := m.explicitRedefinitions(sym)
	for _, sup := range m.roleSources(owner) {
		if !behaviorLike(sup) {
			continue
		}
		var inherited []*symbols.Symbol
		if role == objectiveRole {
			inherited, _ = m.effectiveObjectives(sup, walk)
			if f := m.positionalObjective(owner, sym, inherited); f != nil {
				inherited = []*symbols.Symbol{f}
			} else {
				inherited = nil
			}
		} else {
			inherited = m.effectiveRoles(sup, role, seenCases)
		}
		for _, f := range inherited {
			if !seenRoles[f] {
				seenRoles[f] = true
				out = append(out, f)
			}
		}
	}
	return out
}

// viewRenderingRedefinition is the library `viewRendering` a `render` member
// redefines, unless a clause of its own already names it.
func (m *Model) viewRenderingRedefinition(sym *symbols.Symbol) []*symbols.Symbol {
	if m.resolver == nil || m.resolver.Index() == nil {
		return nil
	}
	explicit := m.explicitRedefinitions(sym)
	for _, lib := range m.resolver.Index().LookupQualified(viewRenderingFQN) {
		if lib != nil && lib != sym && !explicit[lib] {
			return []*symbols.Symbol{lib}
		}
	}
	return nil
}

func isViewRendering(node ast.Node) bool {
	if wrapper, ok := node.(*ast.Membership); ok {
		node = wrapper.Member
	}
	usage, ok := node.(*ast.Usage)
	return ok && usage.Kind == ast.UsageViewRendering
}

// positionalActorRedefinitions is the effective actor at sym's position in each general of
// owner; a `:>>` clause of sym's own governs instead, as for any parameter.
func (m *Model) positionalActorRedefinitions(owner, sym *symbols.Symbol) []*symbols.Symbol {
	if redefinesExplicitly(sym) {
		return nil
	}
	position := rolePosition(owner, actorRole, sym)
	if position < 0 {
		return nil
	}
	var out []*symbols.Symbol
	walk := newRoleWalk()
	placed := map[*symbols.Symbol]bool{sym: true}
	replaced := replacements{}
	for _, sup := range m.roleSources(owner) {
		if !behaviorLike(sup) {
			continue
		}
		inherited, inheritedReplaced := m.effectiveActors(sup, walk)
		for f, by := range inheritedReplaced {
			for b := range by {
				replaced.add(f, b)
			}
		}
		if position < len(inherited) {
			out = placeRole(out, placed, inherited[position], replaced)
		}
	}
	return out
}

// effectiveActors lists sym's actor parameters: its own, then each general's that none of
// its own redefines by clause or position (KerML §7.4.7.2), restatements merged as for objectives.
func (m *Model) effectiveActors(sym *symbols.Symbol, walk *roleWalk) ([]*symbols.Symbol, replacements) {
	if sym == nil {
		return nil, nil
	}
	if walk.visiting[sym] {
		walk.cuts++
		return nil, nil
	}
	if done, ok := walk.done[sym]; ok {
		return done.roles, done.replaced
	}
	walk.visiting[sym] = true
	defer delete(walk.visiting, sym)
	cuts := walk.cuts
	replaced := replacements{}
	owned := ownedRoles(sym, actorRole)
	explicit := make([]map[*symbols.Symbol]bool, len(owned))
	for i, o := range owned {
		explicit[i] = m.explicitRedefinitions(o)
	}
	out := append([]*symbols.Symbol(nil), owned...)
	placed := map[*symbols.Symbol]bool{}
	for _, o := range owned {
		placed[o] = true
	}
	for _, sup := range m.roleSources(sym) {
		if !behaviorLike(sup) {
			continue
		}
		inherited, inheritedReplaced := m.effectiveActors(sup, walk)
		for f, by := range inheritedReplaced {
			for b := range by {
				replaced.add(f, b)
			}
		}
		for i, f := range inherited {
			for j, o := range owned {
				if explicit[j][f] || (i == j && !redefinesExplicitly(o)) {
					replaced.add(f, o)
				}
			}
			out = placeRole(out, placed, f, replaced)
		}
	}
	if walk.cuts == cuts {
		walk.done[sym] = effectiveRoleSet{roles: out, replaced: replaced}
	}
	return out, replaced
}

// redefinesExplicitly reports whether sym's declaration carries a `:>>` clause.
func redefinesExplicitly(sym *symbols.Symbol) bool {
	for _, rel := range RelationshipsOf(sym) {
		if rel != nil && rel.Kind == ast.RelRedefines {
			return true
		}
	}
	return false
}

// roleSources are the cases whose subjects and objectives sym inherits: its generals and
// the usage it reference-subsets, a subsetting too (KerML 8.3.3.3.9) but kept out of
// DirectSupertypes.
func (m *Model) roleSources(sym *symbols.Symbol) []*symbols.Symbol {
	out := m.DirectSupertypes(sym)
	if ref := m.ReferencedFeature(sym); ref != nil && !slices.Contains(out, ref) {
		out = append(out[:len(out):len(out)], ref)
	}
	return out
}

// positionalObjective is the general's effective objective at sym's position in owner,
// nil when there is none or sym is a later objective outside an analysis case.
func (m *Model) positionalObjective(owner, sym *symbols.Symbol, inherited []*symbols.Symbol) *symbols.Symbol {
	position := rolePosition(owner, objectiveRole, sym)
	if position < 0 || (position > 0 && !analysisCase(owner)) || position >= len(inherited) {
		return nil
	}
	return inherited[position]
}

// replacements records which objectives restate which: replaced objective to its restatements.
type replacements map[*symbols.Symbol]map[*symbols.Symbol]bool

func (r replacements) add(replaced, by *symbols.Symbol) {
	if r[replaced] == nil {
		r[replaced] = map[*symbols.Symbol]bool{}
	}
	r[replaced][by] = true
}

// restates reports whether by restates sym, directly or through a chain of restatements.
func (r replacements) restates(by, sym *symbols.Symbol) bool {
	seen := map[*symbols.Symbol]bool{}
	var walk func(*symbols.Symbol) bool
	walk = func(s *symbols.Symbol) bool {
		if seen[s] {
			return false
		}
		seen[s] = true
		for next := range r[s] {
			if next == by || walk(next) {
				return true
			}
		}
		return false
	}
	return walk(sym)
}

// roleWalk is the state of one effectiveObjectives or effectiveActors query: the cases on
// the current path, which cut a cycle, and the finished ones, read once however many paths reach them.
type roleWalk struct {
	visiting map[*symbols.Symbol]bool
	done     map[*symbols.Symbol]effectiveRoleSet
	cuts     int // cycles cut so far; a result computed across a cut is path-bound
}

// effectiveRoleSet is a finished answer of a roleWalk; readers must not mutate it.
type effectiveRoleSet struct {
	roles    []*symbols.Symbol
	replaced replacements
}

func newRoleWalk() *roleWalk {
	return &roleWalk{visiting: map[*symbols.Symbol]bool{}, done: map[*symbols.Symbol]effectiveRoleSet{}}
}

// effectiveObjectives lists sym's objectives by position: each general's, replaced by the
// owned one redefining it by clause or position, then the owned ones redefining none. A
// restatement met through one general stands for the objective it restates met through
// another. Every general is read in full, and once per walk when no cycle cuts it short.
func (m *Model) effectiveObjectives(sym *symbols.Symbol, walk *roleWalk) ([]*symbols.Symbol, replacements) {
	if sym == nil {
		return nil, nil
	}
	if walk.visiting[sym] {
		walk.cuts++
		return nil, nil
	}
	if done, ok := walk.done[sym]; ok {
		return done.roles, done.replaced
	}
	walk.visiting[sym] = true
	defer delete(walk.visiting, sym)
	cuts := walk.cuts
	replaced := replacements{}
	owned := ownedRoles(sym, objectiveRole)
	explicit := make([]map[*symbols.Symbol]bool, len(owned))
	for i, o := range owned {
		explicit[i] = m.explicitRedefinitions(o)
	}
	var out []*symbols.Symbol
	placed := map[*symbols.Symbol]bool{}
	for _, sup := range m.roleSources(sym) {
		if !behaviorLike(sup) {
			continue
		}
		inherited, inheritedReplaced := m.effectiveObjectives(sup, walk)
		for f, by := range inheritedReplaced {
			for b := range by {
				replaced.add(f, b)
			}
		}
		for _, f := range inherited {
			for i, o := range owned {
				if explicit[i][f] || m.positionalObjective(sym, o, inherited) == f {
					replaced.add(f, o)
					f = o
					break
				}
			}
			out = placeRole(out, placed, f, replaced)
		}
	}
	for _, o := range owned {
		if !placed[o] {
			placed[o] = true
			out = append(out, o)
		}
	}
	if walk.cuts == cuts {
		walk.done[sym] = effectiveRoleSet{roles: out, replaced: replaced}
	}
	return out, replaced
}

// placeRole adds f to out: in place of a role it restates, nowhere when one already
// placed restates it, and at the end otherwise.
func placeRole(out []*symbols.Symbol, placed map[*symbols.Symbol]bool, f *symbols.Symbol, replaced replacements) []*symbols.Symbol {
	if placed[f] {
		return out
	}
	placed[f] = true
	for i, prev := range out {
		if replaced.restates(prev, f) {
			return out
		}
		if replaced.restates(f, prev) {
			out[i] = f
			return out
		}
	}
	return append(out, f)
}

// ObjectivesOf returns the objectives a case owns and the inherited ones that no
// owned or inherited objective redefines, by clause or by role.
func (m *Model) ObjectivesOf(sym *symbols.Symbol) (owned, inherited []*symbols.Symbol) {
	return m.visibleRoles(sym, objectiveRole)
}

// SubjectsOf is ObjectivesOf for the subjects of a requirement or case.
func (m *Model) SubjectsOf(sym *symbols.Symbol) (owned, inherited []*symbols.Symbol) {
	return m.visibleRoles(sym, subjectRole)
}

func (m *Model) visibleRoles(sym *symbols.Symbol, role caseRole) (owned, inherited []*symbols.Symbol) {
	if m == nil || sym == nil {
		return nil, nil
	}
	owned = ownedRoles(sym, role)
	var reachable []*symbols.Symbol
	seen := map[*symbols.Symbol]bool{sym: true}
	for _, sup := range m.roleSources(sym) {
		reachable = m.collectRoles(sup, role, seen, reachable)
	}
	masked := map[*symbols.Symbol]bool{}
	for _, o := range append(append([]*symbols.Symbol{}, owned...), reachable...) {
		for target := range m.explicitRedefinitions(o) {
			masked[target] = true
		}
		for _, target := range m.ImplicitRoleRedefinitions(o) {
			masked[target] = true
		}
	}
	for _, f := range reachable {
		if !masked[f] {
			inherited = append(inherited, f)
		}
	}
	return owned, inherited
}

// collectRoles appends the role features sym owns or inherits, each once.
func (m *Model) collectRoles(sym *symbols.Symbol, role caseRole, seen map[*symbols.Symbol]bool, out []*symbols.Symbol) []*symbols.Symbol {
	if sym == nil || seen[sym] || !behaviorLike(sym) {
		return out
	}
	seen[sym] = true
	out = append(out, ownedRoles(sym, role)...)
	for _, sup := range m.roleSources(sym) {
		out = m.collectRoles(sup, role, seen, out)
	}
	return out
}

// SubjectParameterOf returns the subject parameter of a requirement or case: its own,
// else the inherited one no other visible subject redefines, or nil when it has none.
func (m *Model) SubjectParameterOf(sym *symbols.Symbol) *symbols.Symbol {
	owned, inherited := m.SubjectsOf(sym)
	if len(owned) > 0 {
		return owned[0]
	}
	if len(inherited) > 0 {
		return inherited[0]
	}
	return nil
}

func (m *Model) effectiveRoles(sym *symbols.Symbol, role caseRole, seen map[*symbols.Symbol]bool) []*symbols.Symbol {
	if sym == nil || seen[sym] {
		return nil
	}
	seen[sym] = true
	if owned := ownedRoles(sym, role); len(owned) > 0 {
		return owned
	}
	var out []*symbols.Symbol
	for _, sup := range m.roleSources(sym) {
		if behaviorLike(sup) {
			out = append(out, m.effectiveRoles(sup, role, seen)...)
		}
	}
	return out
}

func ownedRoles(sym *symbols.Symbol, role caseRole) []*symbols.Symbol {
	if sym == nil || sym.Scope == nil {
		return nil
	}
	var out []*symbols.Symbol
	for _, member := range declMembers(sym) {
		if roleOfNode(member) != role {
			continue
		}
		node := member
		if wrapper, ok := member.(*ast.Membership); ok {
			node = wrapper.Member
		}
		if found := memberSymbol(sym.Scope, node); found != nil {
			out = append(out, found)
		}
	}
	return out
}

// rolePosition is sym's index among the role features owner declares, -1 when
// it is not one of them.
func rolePosition(owner *symbols.Symbol, role caseRole, sym *symbols.Symbol) int {
	for i, owned := range ownedRoles(owner, role) {
		if owned == sym {
			return i
		}
	}
	return -1
}

func analysisCase(sym *symbols.Symbol) bool {
	if sym == nil {
		return false
	}
	switch d := sym.Decl.(type) {
	case *ast.Definition:
		return d.Kind == ast.DefAnalysisCase
	case *ast.Usage:
		return d.Kind == ast.UsageAnalysisCase
	}
	return false
}

func roleOf(sym *symbols.Symbol) caseRole {
	if sym == nil {
		return noCaseRole
	}
	return roleOfNode(sym.Decl)
}

func roleOfNode(node ast.Node) caseRole {
	if wrapper, ok := node.(*ast.Membership); ok {
		node = wrapper.Member
	}
	switch d := node.(type) {
	case *ast.SubjectMember:
		return subjectRole
	case *ast.Usage:
		switch d.Kind {
		case ast.UsageSubject:
			return subjectRole
		case ast.UsageObjective:
			return objectiveRole
		case ast.UsageActor, ast.UsageStakeholder:
			return actorRole
		}
	}
	return noCaseRole
}

// explicitRedefinitions returns the features sym's own `:>>` clauses resolve to.
func (m *Model) explicitRedefinitions(sym *symbols.Symbol) map[*symbols.Symbol]bool {
	out := map[*symbols.Symbol]bool{}
	for _, rel := range RelationshipsOf(sym) {
		if rel == nil || rel.Kind != ast.RelRedefines || rel.Target == nil {
			continue
		}
		if target := m.relationshipTarget(sym, rel); target != nil {
			out[target] = true
		}
	}
	return out
}
