package passes

import (
	"strconv"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes/kit"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// mosaFacts is what the MOSA audit gathers from one document: the kinds and
// annotations it declares and the conformances and satisfactions it states.
type mosaFacts struct {
	present map[mosaKind]bool
	marks   map[mosaMark]bool
	// conformant: elements at a #conformant end; satisfiers: elements a satisfy traces.
	conformant map[symbols.ElementKey]bool
	satisfiers map[symbols.ElementKey]bool
}

// mosaMark is one of the MOSA annotation families an element may carry.
type mosaMark int

const (
	mosaMarkDataRights mosaMark = iota
	mosaMarkProprietary
	mosaMarkInterfaceControl
)

func newMOSAFacts() *mosaFacts {
	return &mosaFacts{
		present:    map[mosaKind]bool{},
		marks:      map[mosaMark]bool{},
		conformant: map[symbols.ElementKey]bool{},
		satisfiers: map[symbols.ElementKey]bool{},
	}
}

// note records the marks an element carries among those stated anywhere.
func (f *mosaFacts) note(m mosaMarks) {
	if m.dataRights {
		f.marks[mosaMarkDataRights] = true
	}
	if m.proprietary {
		f.marks[mosaMarkProprietary] = true
	}
	if m.interfaceControl {
		f.marks[mosaMarkInterfaceControl] = true
	}
}

// mosaUnion is the MOSA facts of every workspace document, counted per document
// and read by name, as oosemUnion is.
type mosaUnion struct {
	perDoc     map[string]*mosaFacts
	present    kit.CountSet[mosaKind]
	marks      kit.CountSet[mosaMark]
	conformant kit.CountSet[symbols.ElementKey]
	satisfiers kit.CountSet[symbols.ElementKey]
}

func newMOSAUnion() *mosaUnion {
	return &mosaUnion{
		perDoc:     map[string]*mosaFacts{},
		present:    kit.CountSet[mosaKind]{},
		marks:      kit.CountSet[mosaMark]{},
		conformant: kit.CountSet[symbols.ElementKey]{},
		satisfiers: kit.CountSet[symbols.ElementKey]{},
	}
}

func mosaPresentName(k mosaKind) string { return "\x00mosa/present/" + strconv.Itoa(int(k)) }
func mosaMarkName(m mosaMark) string    { return "\x00mosa/marks/" + strconv.Itoa(int(m)) }
func mosaConformantName(k symbols.ElementKey) string {
	return "\x00mosa/conformant/" + k.String()
}
func mosaSatisfierName(k symbols.ElementKey) string { return "\x00mosa/satisfier/" + k.String() }

// regather replaces doc's facts with a fresh gather — none when doc is no
// workspace document — naming what the union now answers differently.
func (u *mosaUnion) Regather(ctx *Context, g *Gathers, doc string, changed map[string]bool) {
	if a := newMOSAAudit(ctx); a != nil {
		u.regather(ctx, g, a, doc, changed)
	}
}

func (u *mosaUnion) regather(ctx *Context, g *Gathers, a *mosaAudit, doc string, changed map[string]bool) {
	old := u.perDoc[doc]
	var cur *mosaFacts
	if g.Gathered(doc) {
		cur = newMOSAFacts()
		a.facts = cur
		g.Gather(ctx, doc, a.gather)
		a.facts = nil
	}
	kit.Move(u.present, old.presentSet(), cur.presentSet(), mosaPresentName, changed)
	kit.Move(u.marks, old.markSet(), cur.markSet(), mosaMarkName, changed)
	kit.Move(u.conformant, old.conformantSet(), cur.conformantSet(), mosaConformantName, changed)
	kit.Move(u.satisfiers, old.satisfierSet(), cur.satisfierSet(), mosaSatisfierName, changed)
	if cur == nil {
		delete(u.perDoc, doc)
	} else {
		u.perDoc[doc] = cur
	}
}

func (f *mosaFacts) presentSet() map[mosaKind]bool {
	if f == nil {
		return nil
	}
	return f.present
}

func (f *mosaFacts) markSet() map[mosaMark]bool {
	if f == nil {
		return nil
	}
	return f.marks
}

func (f *mosaFacts) conformantSet() map[symbols.ElementKey]bool {
	if f == nil {
		return nil
	}
	return f.conformant
}

func (f *mosaFacts) satisfierSet() map[symbols.ElementKey]bool {
	if f == nil {
		return nil
	}
	return f.satisfiers
}

// The reads a judgment makes: the union by name, so a regather that flips the
// answer drops the reader, plus the facts of a root gathered for this run alone.

func (a *mosaAudit) hasKind(k mosaKind) bool {
	a.ctx.Resolver().ReadName(mosaPresentName(k))
	return a.union.present.Has(k) || a.local.presentSet()[k]
}

func (a *mosaAudit) anyMarked(m mosaMark) bool {
	a.ctx.Resolver().ReadName(mosaMarkName(m))
	return a.union.marks.Has(m) || a.local.markSet()[m]
}

func (a *mosaAudit) isConformant(k symbols.ElementKey) bool {
	a.ctx.Resolver().ReadName(mosaConformantName(k))
	return a.union.conformant.Has(k) || a.local.conformantSet()[k]
}

func (a *mosaAudit) isSatisfier(k symbols.ElementKey) bool {
	a.ctx.Resolver().ReadName(mosaSatisfierName(k))
	return a.union.satisfiers.Has(k) || a.local.satisfierSet()[k]
}
