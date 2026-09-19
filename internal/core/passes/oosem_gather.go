package passes

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/core/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// oosemFacts is what the OOSEM audit gathers from one document: the kinds it
// declares and the derivation, satisfaction and allocation relationships it states.
type oosemFacts struct {
	present map[oosemKind]bool
	// derived holds each requirement at a `#derive` end with a kind of an
	// `#original` end of the same `#derivation`.
	derived   map[oosemDerivation]bool
	satisfied map[symbols.ElementKey]bool
	allocated map[symbols.ElementKey]bool
	// satisfies holds true when the document states any satisfy.
	satisfies map[bool]bool
}

// oosemDerivation is one requirement derived from a requirement of one kind.
type oosemDerivation struct {
	requirement symbols.ElementKey
	from        oosemKind
}

func newOOSEMFacts() *oosemFacts {
	return &oosemFacts{
		present:   map[oosemKind]bool{},
		derived:   map[oosemDerivation]bool{},
		satisfied: map[symbols.ElementKey]bool{},
		allocated: map[symbols.ElementKey]bool{},
		satisfies: map[bool]bool{},
	}
}

// oosemUnion is the OOSEM facts of every workspace document, counted per
// document so a document's regather moves only what it alone stated. Judgments
// read it by name (see resolve.Resolver.ReadName), so a regather that flips an
// answer drops exactly the documents that read it.
type oosemUnion struct {
	perDoc    map[string]*oosemFacts
	present   kit.CountSet[oosemKind]
	derived   kit.CountSet[oosemDerivation]
	satisfied kit.CountSet[symbols.ElementKey]
	allocated kit.CountSet[symbols.ElementKey]
	satisfies kit.CountSet[bool]
}

func newOOSEMUnion() *oosemUnion {
	return &oosemUnion{
		perDoc:    map[string]*oosemFacts{},
		present:   kit.CountSet[oosemKind]{},
		derived:   kit.CountSet[oosemDerivation]{},
		satisfied: kit.CountSet[symbols.ElementKey]{},
		allocated: kit.CountSet[symbols.ElementKey]{},
		satisfies: kit.CountSet[bool]{},
	}
}

func oosemPresentName(k oosemKind) string { return "\x00oosem/present/" + strconv.Itoa(int(k)) }
func oosemDerivedName(d oosemDerivation) string {
	return "\x00oosem/derived/" + d.requirement.String()
}
func oosemSatisfiedName(k symbols.ElementKey) string { return "\x00oosem/satisfied/" + k.String() }
func oosemAllocatedName(k symbols.ElementKey) string { return "\x00oosem/allocated/" + k.String() }
func oosemSatisfiesName(bool) string                 { return "\x00oosem/satisfies" }

// regather replaces doc's facts with a fresh gather — none when doc is no
// workspace document — naming what the union now answers differently.
func (u *oosemUnion) Regather(ctx *Context, g *Gathers, doc string, changed map[string]bool) {
	if a := newOOSEMAudit(ctx); a != nil {
		u.regather(ctx, g, a, doc, changed)
	}
}

func (u *oosemUnion) regather(ctx *Context, g *Gathers, a *oosemAudit, doc string, changed map[string]bool) {
	old := u.perDoc[doc]
	var cur *oosemFacts
	if g.Gathered(doc) {
		cur = newOOSEMFacts()
		a.facts = cur
		g.Gather(ctx, doc, a.gather)
		a.facts = nil
	}
	kit.Move(u.present, old.presentSet(), cur.presentSet(), oosemPresentName, changed)
	kit.Move(u.derived, old.derivedSet(), cur.derivedSet(), oosemDerivedName, changed)
	kit.Move(u.satisfied, old.satisfiedSet(), cur.satisfiedSet(), oosemSatisfiedName, changed)
	kit.Move(u.allocated, old.allocatedSet(), cur.allocatedSet(), oosemAllocatedName, changed)
	kit.Move(u.satisfies, old.satisfiesSet(), cur.satisfiesSet(), oosemSatisfiesName, changed)
	if cur == nil {
		delete(u.perDoc, doc)
	} else {
		u.perDoc[doc] = cur
	}
}

func (f *oosemFacts) presentSet() map[oosemKind]bool {
	if f == nil {
		return nil
	}
	return f.present
}

func (f *oosemFacts) derivedSet() map[oosemDerivation]bool {
	if f == nil {
		return nil
	}
	return f.derived
}

func (f *oosemFacts) satisfiedSet() map[symbols.ElementKey]bool {
	if f == nil {
		return nil
	}
	return f.satisfied
}

func (f *oosemFacts) allocatedSet() map[symbols.ElementKey]bool {
	if f == nil {
		return nil
	}
	return f.allocated
}

func (f *oosemFacts) satisfiesSet() map[bool]bool {
	if f == nil {
		return nil
	}
	return f.satisfies
}

// The reads a judgment makes: the union by name, so a regather that flips the
// answer drops the reader, plus the facts of a root gathered for this run alone.

func (a *oosemAudit) hasKind(k oosemKind) bool {
	a.ctx.Resolver().ReadName(oosemPresentName(k))
	return a.union.present.Has(k) || a.local.presentSet()[k]
}

func (a *oosemAudit) derivedFrom(requirement symbols.ElementKey, from oosemKind) bool {
	d := oosemDerivation{requirement, from}
	a.ctx.Resolver().ReadName(oosemDerivedName(d))
	return a.union.derived.Has(d) || a.local.derivedSet()[d]
}

func (a *oosemAudit) isSatisfied(k symbols.ElementKey) bool {
	a.ctx.Resolver().ReadName(oosemSatisfiedName(k))
	return a.union.satisfied.Has(k) || a.local.satisfiedSet()[k]
}

func (a *oosemAudit) isAllocated(k symbols.ElementKey) bool {
	a.ctx.Resolver().ReadName(oosemAllocatedName(k))
	return a.union.allocated.Has(k) || a.local.allocatedSet()[k]
}

func (a *oosemAudit) statesSatisfaction() bool {
	a.ctx.Resolver().ReadName(oosemSatisfiesName(true))
	return a.union.satisfies.Has(true) || a.local.satisfiesSet()[true]
}
