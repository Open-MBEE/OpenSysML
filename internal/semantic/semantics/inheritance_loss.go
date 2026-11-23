package semantics

import (
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// What a type does not inherit (KerML 1.1 8.3.3.1.10): Type::inheritedMemberships
// is what its supertypes pass on (inheritableMemberships) less removeRedefinedFeatures.

// ownRedefinitions is what the features a type declares redefine.
type ownRedefinitions struct {
	owner *symbols.Symbol
	// direct holds the features redefined by clause, position or role; nil while empty, as is closure.
	direct map[*symbols.Symbol]bool
	// closure adds what those features redefine in turn.
	closure map[*symbols.Symbol]bool
}

// masks reports whether a feature of the type redefines candidate, or directly
// redefines a feature candidate redefines; a member it declares is never lost.
func (o *ownRedefinitions) masks(m *Model, candidate *symbols.Symbol) bool {
	if o == nil || candidate == nil || candidate.OwnerScope == o.owner.Scope {
		return false
	}
	return o.closure[candidate] || m.redefinesAny(candidate, o.direct)
}

// inheritanceLoss is what a type does not inherit of what its supertypes hold.
type inheritanceLoss struct {
	// redefiners are the redefining features the type's supertypes pass on to it.
	redefiners []*symbols.Symbol
	// unoffered holds what no supertype passes on.
	unoffered map[*symbols.Symbol]bool
	// inherited adds what a passed-on feature redefines: the loss a redefinition
	// written in the type sees.
	inherited map[*symbols.Symbol]bool
	// full adds what the type's own features remove; nil until they are read.
	full map[*symbols.Symbol]bool
	// passed memoizes passedOn once full is known.
	passed []*symbols.Symbol
	// settled is set once no guard cut the derivation short for a lookup outside it.
	settled bool
}

// redefinitionReach returns what feature's redefinitions remove from a type
// inheriting it, walked exactly where the memoized closure is cyclic.
func (m *Model) redefinitionReach(feature *symbols.Symbol) map[*symbols.Symbol]bool {
	closure, cyclic := m.redefinitionClosure(feature)
	if !cyclic {
		return closure
	}
	out := make(map[*symbols.Symbol]bool)
	seen := map[*symbols.Symbol]bool{feature: true}
	pending := []*symbols.Symbol{feature}
	for i := 0; i < len(pending); i++ {
		redefining := pending[i]
		for _, target := range m.maskingRedefinedFeatures(redefining) {
			if target == nil {
				continue
			}
			if !redefinesSibling(redefining, target) {
				out[target] = true
			}
			if !seen[target] {
				seen[target] = true
				pending = append(pending, target)
			}
		}
	}
	return out
}

// redefinesAny reports whether feature redefines, directly or indirectly, one of targets.
func (m *Model) redefinesAny(feature *symbols.Symbol, targets map[*symbols.Symbol]bool) bool {
	if len(targets) == 0 {
		return false
	}
	for target := range m.redefinitionReach(feature) {
		if targets[target] {
			return true
		}
	}
	return false
}

// ownRedefinitions returns what the features sym declares redefine, less exclude,
// a declaration being written (KerML 8.3.3.3.6). Memoized for a nil exclude.
func (m *Model) ownRedefinitions(sym, exclude *symbols.Symbol) *ownRedefinitions {
	if m == nil || sym == nil || sym.Scope == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	memoize := exclude == nil
	if memoize {
		if cached, ok := m.redefOwn[sym]; ok {
			return cached
		}
		journal(m, m.redefOwn, sym, sym.Decl)
		m.redefOwn[sym] = nil // re-entrancy guard: a nested query sees no redefinition
	}
	m.resolver.Enter()
	own := &ownRedefinitions{owner: sym}
	sym.Scope.ForEachMember(func(feature *symbols.Symbol) bool {
		if feature == exclude {
			return true
		}
		for _, target := range m.maskingRedefinedFeatures(feature) {
			if target != nil && !redefinesSibling(feature, target) {
				if own.direct == nil {
					own.direct = make(map[*symbols.Symbol]bool)
				}
				own.direct[target] = true
			}
		}
		for target := range m.redefinitionReach(feature) {
			if own.closure == nil {
				own.closure = make(map[*symbols.Symbol]bool)
			}
			own.closure[target] = true
		}
		return true
	})
	keepOwnMembers(sym, own.direct, own.closure)
	// A redefinition mid-resolution contributes nothing yet, so the answer is not final.
	settled := m.resolver.Leave() && m.computingRedefinedFeatures == 0
	if memoize {
		if settled {
			m.redefOwn[sym] = own
		} else {
			delete(m.redefOwn, sym)
		}
	}
	return own
}

// keepOwnMembers drops sym and the members it declares from sets: a local
// declaration is present whatever redefines it.
func keepOwnMembers(sym *symbols.Symbol, sets ...map[*symbols.Symbol]bool) {
	for _, set := range sets {
		delete(set, sym)
	}
	if sym.Scope == nil {
		return
	}
	sym.Scope.ForEachMember(func(local *symbols.Symbol) bool {
		for _, set := range sets {
			delete(set, local)
		}
		return true
	})
}

// inheritanceLoss returns what sym does not inherit, computing its supertypes'
// answers on the way and memoizing each one whose own derivation has settled.
func (m *Model) inheritanceLoss(sym *symbols.Symbol) *inheritanceLoss {
	if m == nil || sym == nil {
		return nil
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.redefLoss[sym]; ok {
		if cached == nil {
			m.cutShortLoss(sym)
		}
		return cached
	}
	m.resolver.Enter()
	walked := make(map[*symbols.Symbol]*inheritanceLoss)
	loss := m.walkInheritanceLoss(sym, walked)
	// A redefinition mid-resolution contributes nothing yet, so the answers are not final.
	final := m.computingRedefinedFeatures == 0
	settled := m.resolver.Leave() && final
	for walkedSym, walkedLoss := range walked {
		// A supertype is kept when the cut that unsettled the walk was not in its own derivation.
		keep := settled || (final && walkedLoss.settled)
		if _, stable := m.contributed[walkedSym]; !keep || !stable {
			delete(m.redefLoss, walkedSym)
		}
	}
	return loss
}

// walkInheritanceLoss derives sym's loss from those of the types contributing to it; walked holds
// the answers of this walk, a type met again answering as it stands and cutting its reader short.
func (m *Model) walkInheritanceLoss(sym *symbols.Symbol, walked map[*symbols.Symbol]*inheritanceLoss) *inheritanceLoss {
	if loss, ok := walked[sym]; ok {
		m.cutShortLoss(sym)
		return loss
	}
	defer m.own(sym).LeaveDoc()
	if cached, ok := m.redefLoss[sym]; ok {
		if cached.lost() == nil {
			m.cutShortLoss(sym)
		}
		return cached
	}
	loss := &inheritanceLoss{}
	walked[sym] = loss
	journal(m, m.redefLoss, sym, sym.Decl)
	m.redefLoss[sym] = nil // re-entrancy guard: a nested query sees no loss
	m.walkingLoss[sym] = m.resolver.Enter()
	defer func() {
		delete(m.walkingLoss, sym)
		loss.settled = m.resolver.Leave()
	}()

	contributors := m.DirectMemberSources(sym)
	losses := make([]*inheritanceLoss, len(contributors))
	for i, contributor := range contributors {
		losses[i] = m.walkInheritanceLoss(contributor, walked)
	}
	unoffered := lossSet{from: m.unofferedBy(contributors, losses)}
	// Condition 1: a feature another inheritable membership redefines is removed.
	offered := make(map[*symbols.Symbol]bool)
	var reached []map[*symbols.Symbol]bool
	offer := func(feature *symbols.Symbol) {
		if offered[feature] {
			return
		}
		offered[feature] = true
		reach := m.redefinitionReach(feature)
		if len(reach) == 0 {
			return
		}
		loss.redefiners = append(loss.redefiners, feature)
		reached = append(reached, reach)
	}
	for i, contributor := range contributors {
		if contributor.Scope != nil {
			contributor.Scope.ForEachMember(func(feature *symbols.Symbol) bool {
				if Inheritable(feature) {
					offer(feature)
				}
				return true
			})
		}
		for _, feature := range losses[i].passedOn() {
			offer(feature)
		}
	}
	unoffered.dropOwnMembers(sym)
	inherited := lossSet{from: unoffered.set()}
	for _, reach := range reached {
		for target := range reach {
			inherited.add(target)
		}
	}
	inherited.dropOwnMembers(sym)
	loss.unoffered, loss.inherited = unoffered.set(), inherited.set()
	// What sym inherits is known before its own features are read: resolving
	// the features they redefine asks for it.
	m.redefLoss[sym] = loss
	loss.full = m.lossWithOwn(sym, loss, m.ownRedefinitions(sym, nil))
	return loss
}

// cutShortLoss records that a reader was answered by a loss still being walked.
func (m *Model) cutShortLoss(sym *symbols.Symbol) {
	if depth := m.walkingLoss[sym]; depth != 0 {
		m.resolver.CutShort(depth)
	}
}

// lossWithOwn adds to loss's inherited part what own removes (condition 2): what
// sym's features redefine, and the passed-on features redefining what they directly redefine.
func (m *Model) lossWithOwn(sym *symbols.Symbol, loss *inheritanceLoss, own *ownRedefinitions) map[*symbols.Symbol]bool {
	full := lossSet{from: loss.inherited}
	if own != nil {
		for target := range own.closure {
			full.add(target)
		}
		for _, feature := range loss.redefiners {
			if m.redefinesAny(feature, own.direct) {
				full.add(feature)
			}
		}
	}
	full.dropOwnMembers(sym)
	return full.set()
}

// unofferedBy returns what no contributor passes on: a member a contributor has lost and
// no other still offers. One contributor's whole loss is answered as the set it already holds.
func (m *Model) unofferedBy(contributors []*symbols.Symbol, losses []*inheritanceLoss) map[*symbols.Symbol]bool {
	var unoffered lossSet
	for i := range contributors {
		lost := losses[i].lost()
		if len(lost) == 0 {
			continue
		}
		whole := unoffered.from == nil && unoffered.own == nil
		for member := range lost {
			if m.offeredByAnother(contributors, losses, i, member) {
				whole = false
				break
			}
		}
		if whole {
			unoffered.from = lost
			continue
		}
		for member := range lost {
			if !m.offeredByAnother(contributors, losses, i, member) {
				unoffered.add(member)
			}
		}
	}
	return unoffered.set()
}

// lossSet is a set of lost members that starts as from, a set another loss holds and
// so never written to, and is copied on the first change.
type lossSet struct {
	from map[*symbols.Symbol]bool
	own  map[*symbols.Symbol]bool
}

func (s *lossSet) add(member *symbols.Symbol) {
	if s.own == nil {
		if s.from[member] {
			return
		}
		s.own = make(map[*symbols.Symbol]bool, len(s.from)+1)
		for held := range s.from {
			s.own[held] = true
		}
	}
	s.own[member] = true
}

func (s *lossSet) remove(member *symbols.Symbol) {
	if s.own == nil {
		if !s.from[member] {
			return
		}
		s.own = make(map[*symbols.Symbol]bool, len(s.from))
		for held := range s.from {
			s.own[held] = true
		}
	}
	delete(s.own, member)
}

// dropOwnMembers removes sym and the members it declares: a local declaration is
// present whatever redefines it.
func (s *lossSet) dropOwnMembers(sym *symbols.Symbol) {
	s.remove(sym)
	if sym.Scope == nil {
		return
	}
	sym.Scope.ForEachMember(func(local *symbols.Symbol) bool {
		s.remove(local)
		return true
	})
}

// set is the set as it stands, never nil.
func (s *lossSet) set() map[*symbols.Symbol]bool {
	switch {
	case s.own != nil:
		return s.own
	case s.from != nil:
		return s.from
	}
	return map[*symbols.Symbol]bool{}
}

// lost is everything the type does not inherit; nil for a type still being walked.
func (l *inheritanceLoss) lost() map[*symbols.Symbol]bool {
	if l == nil {
		return nil
	}
	return l.full
}

// passedOn returns the redefining features the type inherits and so passes on.
func (l *inheritanceLoss) passedOn() []*symbols.Symbol {
	if l == nil {
		return nil
	}
	if l.passed != nil {
		return l.passed
	}
	var out []*symbols.Symbol
	for _, feature := range l.redefiners {
		if !l.full[feature] {
			out = append(out, feature)
		}
	}
	if l.full != nil {
		l.passed = out
	}
	return out
}

// offeredByAnother reports whether a contributor other than the i-th holds lost
// and does not lose it itself, so the type still inherits it along that path.
func (m *Model) offeredByAnother(contributors []*symbols.Symbol, losses []*inheritanceLoss, i int, lost *symbols.Symbol) bool {
	if lost.OwnerScope == nil {
		return false
	}
	owner := lost.OwnerScope.Owner()
	if owner == nil {
		return false
	}
	for j, contributor := range contributors {
		if j == i || losses[j].lost()[lost] {
			continue
		}
		if contributor == owner || slices.Contains(m.MemberSources(contributor), owner) {
			return true
		}
	}
	return false
}

// redefinitionMask returns the elements sym does not inherit (KerML 8.3.3.1.10);
// declared false leaves out what its own features remove (KerML 8.3.3.3.6).
func (m *Model) redefinitionMask(sym *symbols.Symbol, declared bool) map[*symbols.Symbol]bool {
	loss := m.inheritanceLoss(sym)
	switch {
	case loss == nil:
		return nil
	case declared && loss.full != nil:
		return loss.full
	case declared:
		m.cutShortLoss(sym)
	}
	return loss.inherited
}

// redefinitionMaskExcluding is the mask on sym less what exclude, a declaration
// being written in it, removes (KerML 8.3.3.3.6). Not memoized: asked per edit.
func (m *Model) redefinitionMaskExcluding(sym, exclude *symbols.Symbol) map[*symbols.Symbol]bool {
	loss := m.inheritanceLoss(sym)
	if loss == nil {
		return nil
	}
	return m.lossWithOwn(sym, loss, m.ownRedefinitions(sym, exclude))
}

// passedOnTo reports whether a supertype of sym passes member on to it: no type
// on every way from member's owner to sym has lost it.
func (m *Model) passedOnTo(sym, member *symbols.Symbol) bool {
	loss := m.inheritanceLoss(sym)
	return loss == nil || !m.maskedBy(loss.unoffered, member)
}
