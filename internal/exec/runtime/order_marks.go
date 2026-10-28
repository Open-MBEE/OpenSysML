package runtime

import (
	"slices"
	"sync/atomic"

	"github.com/Open-MBEE/OpenSysML/internal/ir/lower"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A value is order-dependent when an unresolved choice point may have left it
// different, directly or through what was computed from it. The run keeps, per
// place written, the choice points its value now stands on: a choice seeds the
// places its conflicting alternatives touch (classifyChoice), and every later
// move writes its targets with what its reads stand on — the union over the
// places its footprint reads, with the control it runs under (a guard read, a
// branch drawn), everything when the footprint is dynamic — so a write that
// reads nothing order-dependent clears the places it rewrites. The mark follows
// the data dependence KerML leaves open: successions (Performances.kerml) and
// send-before-accept (Transfers.kerml) order moves, and only what no such
// HappensBefore (Occurrences.kerml) orders stands open.

// choiceSerials numbers the choice points a process classifies, so a mark names
// a choice independently of where the run's notes keep it.
var choiceSerials atomic.Int64

func nextChoiceSerial() int64 {
	return choiceSerials.Add(1)
}

// markSeed is what a choice point left when it was noted: the instant drawn at,
// whether the clock orders its alternatives against later moves, and the names
// it touched.
type markSeed struct {
	at      float64
	clocked bool
	touched []lower.Place
}

// orderMark is one place and the choice points its value stands on.
type orderMark struct {
	place lower.Place
	by    []int64
}

// orderMarks is what the run's values stand on, by place written.
type orderMarks struct {
	entries []orderMark
	seeds   map[int64]markSeed
}

// clone is the marks by value, for a capture to restore.
func (m orderMarks) clone() orderMarks {
	out := orderMarks{entries: make([]orderMark, len(m.entries))}
	for i, e := range m.entries {
		out.entries[i] = orderMark{place: e.place, by: slices.Clone(e.by)}
	}
	if m.seeds != nil {
		out.seeds = make(map[int64]markSeed, len(m.seeds))
		for s, seed := range m.seeds {
			out.seeds[s] = seed
		}
	}
	return out
}

// clockedChoice reports whether the kind's alternatives are moves of the shared
// clock's instant: the run orders none of that instant's moves against them, so
// what they touched stands until the clock advances, whatever else writes it there.
func clockedChoice(kind ChoiceKind) bool {
	switch kind {
	case ChoiceTransition, ChoiceRegionOrder, ChoiceDueOrder, choiceDispatchOrder,
		choiceEntryOrder, choiceExitOrder, choiceStepOrder, choiceEntryStep:
		return true
	}
	return false
}

// seed marks what the choice touched as standing on it, at the instant now.
func (m *orderMarks) seed(c ChoicePoint, now float64) {
	if c.Independent || c.serial == 0 || len(c.touched) == 0 {
		return
	}
	if m.seeds == nil {
		m.seeds = make(map[int64]markSeed)
	}
	m.seeds[c.serial] = markSeed{at: now, clocked: clockedChoice(c.Kind), touched: c.touched}
	for _, p := range c.touched {
		m.add(p, []int64{c.serial})
	}
}

// add joins the serials to what the place stands on.
func (m *orderMarks) add(p lower.Place, by []int64) {
	if len(by) == 0 {
		return
	}
	for i := range m.entries {
		if m.entries[i].place == p {
			m.entries[i].by = unionSerials(m.entries[i].by, by)
			return
		}
	}
	m.entries = append(m.entries, orderMark{place: p, by: slices.Clone(by)})
}

// of is what a read of the place stands on: every entry the place may name.
func (m *orderMarks) of(p lower.Place) []int64 {
	var out []int64
	for _, e := range m.entries {
		if e.place.Conflicts(p) {
			out = unionSerials(out, e.by)
		}
	}
	return out
}

// at is what the one declaration the place names stands on, the marks a rewrite
// of the place replaces; a read, which may reach same-named places, uses of.
func (m *orderMarks) at(p lower.Place) []int64 {
	var out []int64
	for _, e := range m.entries {
		if samePlace(e.place, p) {
			out = unionSerials(out, e.by)
		}
	}
	return out
}

// all is every choice point some value stands on, what a dynamic read may reach.
func (m *orderMarks) all() []int64 {
	var out []int64
	for _, e := range m.entries {
		out = unionSerials(out, e.by)
	}
	return out
}

// rewrite sets what a write of the place leaves it standing on: by, and what a
// choice drawn at this instant of the clock left on it, which the write is not
// ordered after. Only the place itself is rewritten: a same-named place of
// another declaration may hold a different value, which the write leaves as it was.
func (m *orderMarks) rewrite(p lower.Place, by []int64, now float64) {
	kept := slices.Clone(by)
	entries := m.entries[:0]
	for _, e := range m.entries {
		if !samePlace(e.place, p) {
			entries = append(entries, e)
			continue
		}
		for _, s := range e.by {
			if seed, ok := m.seeds[s]; ok && seed.clocked && seed.at == now && touchesPlace(seed.touched, p) {
				kept = unionSerials(kept, []int64{s})
			}
		}
	}
	// A place written stays listed, bare: it witnesses that its node performed.
	m.entries = append(entries, orderMark{place: p, by: kept})
}

// performed reports whether a node of the name has written a place of its own:
// a qualified name reports the node's pin precisely only then, else what the
// enclosing feature of the name stands on, which an unperformed node's pin reads.
func (m *orderMarks) performed(node string) bool {
	for _, e := range m.entries {
		if e.place.Node != nil && ActionNodeName(e.place.Node) == node {
			return true
		}
	}
	return false
}

// samePlace reports whether two places certainly name one value: the same
// declaration, or the same name where neither is resolved to one. A place that may
// merely conflict (Place.Conflicts) is read conservatively, never rewritten.
func samePlace(p, q lower.Place) bool {
	if p.Node != nil && q.Node != nil && p.Node != q.Node {
		return false
	}
	if p.State != nil || q.State != nil {
		return p.State == q.State
	}
	if p.Sym != nil || q.Sym != nil {
		return p.Sym == q.Sym && lower.HeldAlike(p, q)
	}
	return p.Name == q.Name
}

// standingOn is the choice points a value stands on, among those live: the marks
// on every place named reports as the value's, a state's activity aside.
func (m *orderMarks) standingOn(live map[int64]bool, named func(lower.Place) bool) int {
	var found []int64
	for _, e := range m.entries {
		if e.place.State != nil || !named(e.place) {
			continue
		}
		for _, s := range e.by {
			if live[s] {
				found = unionSerials(found, []int64{s})
			}
		}
	}
	return len(found)
}

func touchesPlace(touched []lower.Place, p lower.Place) bool {
	for _, t := range touched {
		if t.Conflicts(p) {
			return true
		}
	}
	return false
}

// unionSerials is a sorted union without duplicates.
func unionSerials(a, b []int64) []int64 {
	if len(b) == 0 {
		return a
	}
	out := slices.Clone(a)
	for _, s := range b {
		if i, found := slices.BinarySearch(out, s); !found {
			out = slices.Insert(out, i, s)
		}
	}
	return out
}

// underControl runs what follows under the choice points the serials name — a
// guard read standing on them, a branch drawn among alternatives — until
// releaseControl of what it returns; every write made meanwhile stands on them too.
func (ctx *Context) underControl(serials []int64) int {
	n := len(ctx.controls)
	ctx.controls = append(ctx.controls, serials...)
	return n
}

// releaseControl ends the control underControl returned n for.
func (ctx *Context) releaseControl(n int) { ctx.controls = ctx.controls[:n] }

// standingOf is what a move with the footprint reads stands on: the control it
// runs under and the marks of every place it reads, every mark where the
// declarations do not decide the places and dynamicAll holds.
func (ctx *Context) standingOf(fp lower.Footprint, dynamicAll bool) []int64 {
	return ctx.readStanding(slices.Clone(ctx.controls), fp, dynamicAll)
}

// readStanding unions onto deps the marks of every place the footprint reads,
// every mark where the declarations do not decide the places and dynamicAll holds.
func (ctx *Context) readStanding(deps []int64, fp lower.Footprint, dynamicAll bool) []int64 {
	marks := &ctx.run.marks
	if fp.Dynamic && dynamicAll {
		return unionSerials(deps, marks.all())
	}
	for _, r := range fp.Reads {
		deps = unionSerials(deps, marks.of(ctx.placeOf(r)))
	}
	return deps
}

// deriveMarks records what a feature valued by an expression stands on as its
// value is derived: the marks of the places the expression reads, as a write
// computed from them would. Deriving is a read, not a move, so it stands on no
// control; a derivation reading nothing order-dependent clears the mark, as a
// clean write does.
// For a feature of an object held via a feature, the expression's bare names
// are that object's own, held the same way.
func (ctx *Context) deriveMarks(place lower.Place, scope *symbols.Scope, value ast.Node) {
	if place.Name == "" || value == nil {
		return
	}
	fp := ctx.model.expressionFootprint(scope, value)
	if place.Via != nil {
		fp.Reads = slices.Clone(fp.Reads)
		for i := range fp.Reads {
			if fp.Reads[i].Via == nil && fp.Reads[i].Node == nil && !fp.Reads[i].Local {
				fp.Reads[i].Via = place.Via
			}
		}
	}
	ctx.writeMarks([]lower.Place{place}, ctx.readStanding(nil, fp, true))
}

// placeOf is the place as the run holds it: a parameter of the action performed,
// which its footprint leaves to whichever node performs the action, is that node's pin.
func (ctx *Context) placeOf(p lower.Place) lower.Place {
	if p.Node == nil && ctx.performing != nil && ctx.performingParams[p.Sym] {
		p.Node, p.Local = ctx.performing, true
	}
	return p
}

// performingNode runs what follows as the performance of node, nil for a run of
// its own, until the function returned is called; params are the performed
// action's own parameters, the node's pins.
func (ctx *Context) performingNode(node ast.Node, params map[*symbols.Symbol]bool) func() {
	savedNode, savedParams := ctx.performing, ctx.performingParams
	ctx.performing, ctx.performingParams = node, params
	return func() { ctx.performing, ctx.performingParams = savedNode, savedParams }
}

// propagateMarks records a move with the footprint made: what it wrote stands on
// what it read. A probe's preview is not a run.
func (ctx *Context) propagateMarks(fp lower.Footprint, dynamicAll bool) {
	if ctx.probes > 0 || len(fp.Writes) == 0 {
		return
	}
	ctx.writeMarks(fp.Writes, ctx.standingOf(fp, dynamicAll))
}

// writeMarks records writes of the places standing on the serials.
func (ctx *Context) writeMarks(places []lower.Place, by []int64) {
	if ctx.probes > 0 {
		return
	}
	for _, w := range places {
		ctx.run.marks.rewrite(ctx.placeOf(w), by, ctx.clock.now)
	}
}

// acceptMarks records the places an accept bound from a message: they stand on
// what the message's payload stood on where it was sent, and on the control here.
func (ctx *Context) acceptMarks(places []lower.Place, dependence []int64) {
	ctx.writeMarks(places, unionSerials(slices.Clone(ctx.controls), dependence))
}

// liveChoices is the serials of the run's unresolved choice points.
func (ctx *Context) liveChoices() map[int64]bool {
	live := make(map[int64]bool)
	for _, n := range ctx.run.notes {
		if c, ok := n.(ChoicePoint); ok && !c.Independent && c.serial != 0 {
			live[c.serial] = true
		}
	}
	return live
}

// commonReach is the nodes every one of the successions reaches: where a token
// stands past them, which succession it took no longer decides its path.
func commonReach(graph *lower.ActionGraph, successors []lower.ActionEdge) map[ast.Node]bool {
	var common map[ast.Node]bool
	for _, edge := range successors {
		reached := make(map[ast.Node]bool)
		work := []ast.Node{edge.Target}
		for len(work) > 0 {
			n := work[len(work)-1]
			work = work[:len(work)-1]
			if reached[n] {
				continue
			}
			reached[n] = true
			for _, out := range graph.Edges[n] {
				work = append(work, out.Target)
			}
		}
		if common == nil {
			common = reached
			continue
		}
		for n := range common {
			if !reached[n] {
				delete(common, n)
			}
		}
	}
	return common
}
