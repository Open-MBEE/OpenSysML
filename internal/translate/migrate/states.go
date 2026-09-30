package migrate

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// stateKw opens a state usage; isA, outsideRegion and outsideMachine are the
// note fragments the unmappable-vertex diagnostics share.
const (
	stateKw        = "state "
	isA            = " is a "
	outsideRegion  = " outside the region, or one with no v2 form"
	outsideMachine = " outside the machine, or one with no v2 form"
)

const (
	transitionKeyword = "transition "
	actionKeyword     = "action "
)

func theTransition(t *sysmlv1.Element) string { return "the transition " + describe(t) }

func deferredBy(v *sysmlv1.Element) string { return "deferred by " + describe(v) }

// pseudostateLine spells the member a pseudostate of kind kw writes as: a
// StateMachines metadata usage for choice/junction/history — qualified so a
// member named like the metadata cannot shadow it — the standard literal for
// fork/join.
func pseudostateLine(prefix, kw, name string) string {
	switch kw {
	case "choice", "junction", "shallowHistory", "deepHistory":
		return "#" + prefix + "StateMachines::" + kw + " state " + name + ";"
	}
	return kw + " " + name + ";"
}

// stateMachinesPrefix is "$::" when a member written as `StateMachines` hides
// the library package from the scope a clause is written in, so the metadata
// reference names it from the root.
func (m *migration) stateMachinesPrefix(scope *sysmlv1.Element) string {
	if m.shadowsLibrary("StateMachines", scope) {
		return "$::"
	}
	return ""
}

// useStateMachines adds `private import StateMachines::*;` to the body of the
// innermost package enclosing the current scope, once, for the StateMachines
// metadata member being written.
func (m *migration) useStateMachines() {
	for e := m.scope; e != nil; e = e.Parent {
		switch e.Type {
		case "Package", "Model", "Profile":
			if m.stateMachinesUsed[e] {
				return
			}
			m.stateMachinesUsed[e] = true
			prefix := m.stateMachinesPrefix(e)
			m.extras[e] = append(m.extras[e], func() { m.w.line("private import " + prefix + "StateMachines::*;") })
			return
		}
	}
}

// stateMachineBody writes a state machine's regions as the body of its state def.

// The note fragments the writer repeats.
const (
	doAction    = "do action"
	exitAction  = "exit action"
	notRun      = " is not run: "
	performsIts = "a state performs its "
)

func (m *migration) stateMachineBody(sm *sysmlv1.Element) {
	m.unwrittenMembers(sm, "observation")
	m.parameters(sm, sm)
	for _, c := range sm.Children {
		switch c.Role {
		case "ownedBehavior", "nestedClassifier", "ownedAttribute", "ownedOperation", "ownedRule":
			m.member(c)
		}
	}
	m.bodyWithContext(sm, func() {
		used := m.nameMachine(sm)
		m.instants(sm, used)
		m.carriers(sm, used)
		for _, cp := range sm.Owned("connectionPoint") {
			m.connectionPoint(cp)
		}
		m.regions(sm, m.populatedRegions(sm), false, func() { /* no extra nesting to write */ })
		m.writeRelocatedTransitions(sm)
	})
}

// nameMachine names every vertex of a machine down through its nested regions ahead of writing,
// so a transition can target another region or a submachine; it returns the names in the state def's body.
func (m *migration) nameMachine(sm *sysmlv1.Element) map[string]bool {
	if used, ok := m.regionUsed[sm]; ok {
		return used
	}
	used := inheritedStateNamesSet()
	for name := range inheritedStateNames {
		m.take(sm, name)
	}
	m.regionUsed[sm] = used
	m.indexTransitions(sm)
	for _, cp := range sm.Owned("connectionPoint") {
		m.nameVertex(cp, sm, used)
	}
	m.nameRegions(m.populatedRegions(sm), sm, used)
	m.prepareRelocatedTransitions(sm)
	return used
}

// prepareRelocatedTransitions indexes transitions under the state scope that writes them.
func (m *migration) prepareRelocatedTransitions(sm *sysmlv1.Element) {
	var walk func(*sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		for _, child := range e.Children {
			if child.Type == "StateMachine" {
				continue
			}
			if child.Role == "transition" && child.Parent != nil && child.Parent.Type == "Region" {
				src, tgt := m.model.Ref(child, "source"), m.model.Ref(child, "target")
				if src != nil && tgt != nil && pseudoKind(src) != "initial" {
					s := m.region(child.Parent)
					if host := s.relocationHost(src, tgt); host != nil {
						m.relocated[host] = append(m.relocated[host], child)
						m.relocatedTo[child] = host
					}
				}
			}
			walk(child)
		}
	}
	walk(sm)
}

// indexTransitions lists the transitions into and out of every vertex of a machine, so a
// connection point's shape can be read before it is written; a machine nested in it indexes its own.
func (m *migration) indexTransitions(sm *sysmlv1.Element) {
	var walk func(e *sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		for _, c := range e.Children {
			if c.Type == "StateMachine" {
				continue
			}
			if c.Role == "transition" {
				if src := m.model.Ref(c, "source"); src != nil {
					m.outgoing[src] = append(m.outgoing[src], c)
				}
				if tgt := m.model.Ref(c, "target"); tgt != nil {
					m.incoming[tgt] = append(m.incoming[tgt], c)
				}
			}
			walk(c)
		}
	}
	walk(sm)
}

// nameRegions names the vertices of the regions of a state machine or state:
// one region shares its owner's body, several become the sub-states of one
// parallel state, each with a body of its own; owner is the element whose body
// used names, nil for a body no v1 element owns.
func (m *migration) nameRegions(regions []*sysmlv1.Element, owner *sysmlv1.Element, used map[string]bool) {
	if len(regions) > 1 {
		name := m.freshMember(owner, used, "regions")
		inner := inheritedStateNamesSet()
		for _, r := range regions {
			rname := m.nameOf(r)
			if rname == "" {
				rname, m.synthesized[r] = "region", true
			}
			m.names[r] = freshIn(inner, rname)
			m.parallel[r] = name
			m.nameRegion(r, nil, inheritedStateNamesSet())
		}
		return
	}
	for _, r := range regions {
		m.nameRegion(r, owner, used)
	}
}

func (m *migration) nameRegion(r, owner *sysmlv1.Element, used map[string]bool) {
	m.regionUsed[r] = used
	for _, v := range r.Owned("subvertex") {
		if v.Type == "State" {
			m.nameVertex(v, owner, used)
			for name := range inheritedStateNames {
				m.take(v, name)
			}
			inner := inheritedStateNamesSet()
			m.stateUsed[v] = inner
			m.namePoints(v, inner)
			m.nameRegions(m.populatedRegions(v), v, inner)
			for _, pr := range pointRegions(v) {
				m.regionUsed[pr] = inner
			}
			continue
		}
		if pointOwner(v) != nil && pointOwner(v).Type == "State" {
			// A tool that lists a state's connection point among its region's vertices.
			continue
		}
		m.nameVertex(v, owner, used)
	}
}

// pointRegions lists the regions of state v holding nothing but connection points
// of v, which a tool listed there: their transitions are written in v's body.
func pointRegions(v *sysmlv1.Element) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	for _, r := range v.Owned("region") {
		if len(r.Owned("subvertex")) > 0 && pointsOnly(r, v) {
			out = append(out, r)
		}
	}
	return out
}

// pointsOnly reports whether every vertex region r lists is a connection point of
// the state owner, written in the state's body rather than the region's.
func pointsOnly(r, owner *sysmlv1.Element) bool {
	if owner.Type != "State" {
		return false
	}
	for _, v := range r.Owned("subvertex") {
		if pointOwner(v) != owner {
			return false
		}
	}
	return true
}

// namePoints settles how each connection point of composite state v is written
// and names those written as members of its body, whose names used lists.
func (m *migration) namePoints(v *sysmlv1.Element, used map[string]bool) {
	// seen holds the names of the body's other members; a written point that bears one
	// yields to it, taking a name no member and no synthesized name of the body has.
	seen := map[string]bool{}
	for _, c := range namespaceMembers(v) {
		if name := m.nameOf(c); name != "" {
			seen[name] = true
		}
	}
	synthesized := func(n string) bool { return m.taken[v][n] }
	for _, cp := range m.connectionPoints(v) {
		f := m.statePointForm(cp, v)
		m.points[cp] = f
		if f.kw == "" {
			continue
		}
		if name := m.nameOf(cp); name != "" && seen[name] {
			m.names[cp] = distinct(seen, synthesized, name)
		} else if name != "" {
			seen[name] = true
		}
		m.nameVertex(cp, v, used)
	}
}

// connectionPoints lists the entry and exit points of composite state v: those
// it owns, and those a tool listed among the vertices of its regions.
func (m *migration) connectionPoints(v *sysmlv1.Element) []*sysmlv1.Element {
	points := v.Owned("connectionPoint")
	for _, r := range v.Owned("region") {
		for _, sv := range r.Owned("subvertex") {
			if k := pseudoKind(sv); k == "entryPoint" || k == "exitPoint" {
				points = append(points, sv)
			}
		}
	}
	return points
}

// pointOwner is the state or state machine an entry or exit point belongs to,
// through the region a tool may have listed it in; nil for another vertex.
func pointOwner(v *sysmlv1.Element) *sysmlv1.Element {
	if k := pseudoKind(v); k != "entryPoint" && k != "exitPoint" {
		return nil
	}
	owner := v.Parent
	if owner != nil && owner.Type == "Region" {
		owner = owner.Parent
	}
	return owner
}

// memberOwner is the element whose body vertex v is written in: the state whose
// connection point it is, however a tool listed it, else v's parent.
func memberOwner(v *sysmlv1.Element) *sysmlv1.Element {
	if owner := pointOwner(v); owner != nil && owner.Type == "State" {
		return owner
	}
	return v.Parent
}

// pointForm says how a composite state's entry or exit point is written: as
// the pseudostate kw of the state's body, or not at all.
type pointForm struct {
	// kw is junction, fork or join; "" when the point is written as no member.
	kw string
	// note is what the report says of a point that is written.
	note string
	// defaultEntry marks an entry point no transition leaves: a transition to it
	// enters its state by the state's default entry, so it is written to the state.
	defaultEntry bool
	// why says why a point has no v2 form; "" when it has one.
	why string
}

// statePointForm settles how connection point v of composite state owner is written from the
// transitions through it: a junction, a fork starting several regions, or a join they leave by.
func (m *migration) statePointForm(v, owner *sysmlv1.Element) pointForm {
	if pseudoKind(v) == "entryPoint" {
		return m.entryPointForm(v, owner)
	}
	return m.exitPointForm(v, owner)
}

func (m *migration) entryPointForm(v, owner *sysmlv1.Element) pointForm {
	out := m.outgoing[v]
	if len(out) == 0 {
		note := "no transition leaves the entry point, so entering through it enters " + describe(owner) + " by its default entry; a transition to it is written to the state"
		if without := regionsWithoutInitial(m.populatedRegions(owner)); len(without) > 0 {
			note += "; no initial pseudostate starts the " + pluralRegion(len(without)) + " " + strings.Join(without, ", ") + ", which v1 too leaves inactive on entering the state"
		}
		if without := m.regionsWithoutOrthogonalInitialEntry(m.populatedRegions(owner)); len(without) > 0 {
			note += "; the initial target in the " + pluralRegion(len(without)) + " " + strings.Join(without, ", ") + " is in an orthogonal region, so no entry is written for it"
		}
		return pointForm{defaultEntry: true, note: note}
	}
	for _, t := range out {
		if why := m.entryBranchWhy(t, owner); why != "" {
			return pointForm{why: describe(t) + " leads from the entry point " + why}
		}
	}
	regions := m.regionsCrossed(out, owner, "target")
	if len(out) < 2 || len(regions) < 2 {
		return pointForm{kw: "junction", note: "written as a `#StateMachines::junction state` of its state; a transition entering through it runs the state's entry behavior, then the transition leaving the junction"}
	}
	if len(regions) < len(out) {
		return pointForm{why: "the entry point starts several regions of " + describe(owner) + ", as a fork does, but two of its outgoing transitions enter the same region"}
	}
	for _, t := range out {
		if why := m.forkBranchWhy(t, m.model.Ref(t, "target")); why != "" {
			return pointForm{why: "the entry point starts several regions of " + describe(owner) + ", as a fork does, but " + describe(t) + " " + why}
		}
	}
	return pointForm{kw: "fork", note: "written as a fork of its state, whose branches start its regions; a transition entering through it runs the state's entry behavior, then the branches"}
}

// entryBranchWhy says why transition t, leaving an entry point of owner, keeps the point from being
// written: it has a trigger, or its target is missing, owner itself or outside it, a history, or an exit point of owner.
func (m *migration) entryBranchWhy(t, owner *sysmlv1.Element) string {
	tgt := m.model.Ref(t, "target")
	switch {
	case len(t.Owned("trigger")) > 0:
		return "with a trigger, which no transition out of a pseudostate takes; the runtime would follow it without waiting for the event"
	case tgt == nil:
		return "to no target"
	case tgt == owner:
		return "back to the state itself, which v1 enters by its default entry while the runtime would leave and re-enter it"
	case pseudoKind(tgt) == "exitPoint" && pointOwner(tgt) == owner:
		return "straight to the exit point " + describe(tgt) + " of the same state, crossing it without settling in it; the runtime would then run neither its entry nor its exit behavior"
	case pseudoKind(tgt) == "shallowHistory" || pseudoKind(tgt) == "deepHistory":
		return "on into the history pseudostate " + describe(tgt) + ", which the runtime does not follow from a junction"
	case regionWithin(tgt, owner) == nil:
		return "out of the state, to " + describe(tgt)
	}
	return ""
}

func (m *migration) exitPointForm(v, owner *sysmlv1.Element) pointForm {
	out := m.outgoing[v]
	if len(out) == 0 {
		return pointForm{why: "no transition leaves the exit point, so a transition into it leaves " + describe(owner) + " for nowhere; the runtime would halt at the junction"}
	}
	for _, t := range out {
		if why := m.exitBranchWhy(t, owner); why != "" {
			return pointForm{why: describe(t) + " leads from the exit point " + why}
		}
	}
	// in holds the transitions from within the state that shape the point; one from the
	// state itself is its local transition, one from its entry point is refused with that.
	var in, local []*sysmlv1.Element
	for _, t := range m.incoming[v] {
		src := m.model.Ref(t, "source")
		switch {
		case src == nil:
			return pointForm{why: describe(t) + " leads to the exit point from no source"}
		case src == owner:
			local = append(local, t)
		case pseudoKind(src) == "entryPoint" && pointOwner(src) == owner:
		case regionWithin(src, owner) == nil:
			return pointForm{why: describe(t) + " leads to the exit point from outside the state, from " + describe(src) + "; v1 never enters the state, while the runtime would enter and leave it, running its entry and exit behaviors"}
		default:
			in = append(in, t)
		}
	}
	regions := m.regionsCrossed(in, owner, "source")
	if len(in) < 2 || len(regions) < 2 {
		return pointForm{kw: "junction", note: "written as a `#StateMachines::junction state` of its state; a transition leaving through it runs the transition into the junction, the state's exit behavior, then the transition leaving it"}
	}
	joinBut := "several regions of " + describe(owner) + " leave through the exit point, as through a join, but "
	if len(regions) < len(in) {
		return pointForm{why: joinBut + "two of its incoming transitions leave the same region"}
	}
	if len(local) > 0 {
		return pointForm{why: joinBut + describe(local[0]) + " leaves the state itself, which a join does not wait for"}
	}
	for _, t := range in {
		if src := m.model.Ref(t, "source"); src.Type != "State" {
			return pointForm{why: joinBut + describe(t) + " leaves " + describe(src) + ", a " + kindOf(src) + " rather than a state"}
		}
	}
	return pointForm{kw: "join", note: "written as a join of its state, which its regions leave through together; the transitions into the join run, then the state's exit behavior, then the transition leaving it"}
}

// exitBranchWhy says why transition t, leaving an exit point of owner, keeps the point from
// being written: it has a trigger, or its target is missing, a history, or owner or a vertex within it.
func (m *migration) exitBranchWhy(t, owner *sysmlv1.Element) string {
	tgt := m.model.Ref(t, "target")
	switch {
	case len(t.Owned("trigger")) > 0:
		return "with a trigger, which no transition out of a pseudostate takes; the runtime would follow it without waiting for the event"
	case tgt == nil:
		return "to no target"
	case tgt == owner:
		return "back to the state itself, which v1 leaves and re-enters by its default entry while the runtime, moving from a member of the state to the state, would stay in it, running neither its exit nor its entry"
	case pseudoKind(tgt) == "shallowHistory" || pseudoKind(tgt) == "deepHistory":
		return "on into the history pseudostate " + describe(tgt) + ", which the runtime does not follow from a junction"
	case regionWithin(tgt, owner) != nil || pointOwner(tgt) == owner:
		return "back into the state, to " + describe(tgt) + ", which v1 leaves and re-enters while the runtime, moving between members of the state, would stay in it"
	}
	return ""
}

// regionsCrossed lists the distinct regions of owner the given ends of the
// transitions lie in; an end outside owner counts as none.
func (m *migration) regionsCrossed(transitions []*sysmlv1.Element, owner *sysmlv1.Element, role string) []*sysmlv1.Element {
	var regions []*sysmlv1.Element
	for _, t := range transitions {
		if r := regionWithin(m.model.Ref(t, role), owner); r != nil && !slices.Contains(regions, r) {
			regions = append(regions, r)
		}
	}
	return regions
}

// regionWithin is the region of owner that holds vertex v, however deep; nil
// when v lies outside owner.
func regionWithin(v, owner *sysmlv1.Element) *sysmlv1.Element {
	for cur := v; cur != nil && cur.Parent != nil; cur = cur.Parent {
		if cur.Parent == owner && cur.Type == "Region" {
			return cur
		}
	}
	return nil
}

// forkBranchWhy says why transition t into tgt, a route entryBranchWhy passed, cannot be a
// fork's branch, which enters a state with no guard of its own; "" when it can.
func (m *migration) forkBranchWhy(t, tgt *sysmlv1.Element) string {
	g := m.guardOf(t)
	switch {
	case tgt.Type != "State":
		return "enters " + describe(tgt) + ", a " + kindOf(tgt) + " rather than a state"
	case g != nil && !trueLiteral(firstOwned(g, "specification")):
		return "has a guard"
	}
	return ""
}

// guardOf returns transition t's guard: its guard child, or the owned rule its guard
// reference names, which is how some exporters serialize the composite property.
func (m *migration) guardOf(t *sysmlv1.Element) *sysmlv1.Element {
	if g := firstOwned(t, "guard"); g != nil {
		return g
	}
	if g := m.model.Ref(t, "guard"); g != nil && g.Type == "Constraint" {
		return g
	}
	return nil
}

// trueLiteral reports whether spec is a LiteralBoolean holding true; a value the
// serialization omits is the UML default, false.
func trueLiteral(spec *sysmlv1.Element) bool {
	return spec != nil && spec.Type == "LiteralBoolean" && (spec.Attrs["value"] == "true" || spec.Attrs["value"] == "1")
}

// populatedRegions returns writtenRegions and reports the regions it leaves out: an
// empty one, or one listing only its state's connection points, has nothing to
// enter, so it is skipped rather than written as a sub-state no entry starts.
func (m *migration) populatedRegions(owner *sysmlv1.Element) []*sysmlv1.Element {
	for _, r := range owner.Owned("region") {
		switch {
		case len(r.Owned("subvertex")) == 0:
			m.add(r, Skipped, "", unreferencedNote+": the region holds no vertex, so nothing enters it and no state is written for it")
		case pointsOnly(r, owner):
			m.add(r, Skipped, "", unreferencedNote+": the region lists only connection points of its state, which are written in the state's body, so nothing enters it and no state is written for it")
		}
	}
	return writtenRegions(owner)
}

// writtenRegions lists the regions of a machine or state a body is written for:
// those holding a vertex of their own.
func writtenRegions(owner *sysmlv1.Element) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	for _, r := range owner.Owned("region") {
		if len(r.Owned("subvertex")) > 0 && !pointsOnly(r, owner) {
			out = append(out, r)
		}
	}
	return out
}

// vertexWritten reports whether vertex v is written as a member of its state def's
// body: its machine is written and nameMachine named it, which skips what no region writes.
func (m *migration) vertexWritten(v *sysmlv1.Element) bool {
	sm := machineOf(v)
	if sm == nil || !m.written(sm) {
		return false
	}
	m.nameMachine(sm)
	_, ok := m.vertexNames[v]
	return ok
}

// regionWritten reports whether region r is written as a member of its own: a sub-state
// of the parallel state an orthogonal state's regions become, in a machine that is written.
func (m *migration) regionWritten(r *sysmlv1.Element) bool {
	sm := machineOf(r)
	if sm == nil || !m.written(sm) {
		return false
	}
	m.nameMachine(sm)
	return m.parallel[r] != ""
}

// nameVertex gives a vertex that is written as a member its name in the body
// used lists, after its own when it has one and no sibling took it; a name
// synthesized from its kind also keeps clear of owner's other members.
func (m *migration) nameVertex(v, owner *sysmlv1.Element, used map[string]bool) {
	base := vertexBase(v)
	if base == "" {
		return
	}
	name := m.nameOf(v)
	switch {
	case name == "":
		name, m.synthesized[v] = m.freshMember(owner, used, base), true
	case used[name]:
		name = freshIn(used, name)
	}
	used[name] = true
	m.vertexNames[v] = name
	if name != m.nameOf(v) {
		m.names[v] = name
	}
}

// freshMember is freshIn for a synthesized member of owner's body: the name
// must also differ from the members owner writes, an attribute called fork say.
func (m *migration) freshMember(owner *sysmlv1.Element, used map[string]bool, base string) string {
	name := base
	for i := 2; used[name] || m.nameTaken(owner, name); i++ {
		name = base + strconv.Itoa(i)
	}
	used[name] = true
	return name
}

// vertexBase is the name a vertex of a kind that is written as a member takes
// when anonymous; "" for a kind that is not.
func vertexBase(v *sysmlv1.Element) string {
	switch v.Type {
	case "State":
		return "state"
	case "Pseudostate":
		switch k := pseudoKind(v); k {
		case "choice", "junction", "fork", "join", "entryPoint", "exitPoint":
			return k
		case "shallowHistory", "deepHistory":
			return "history"
		}
	}
	return ""
}

// regions writes the regions of a state machine or composite state: one inline,
// several as the sub-states of one parallel state. The entry succession follows
// the body's entry action when entered says one was written; between writes
// the members that come after it and before the states.
func (m *migration) regions(owner *sysmlv1.Element, regions []*sysmlv1.Element, entered bool, between func()) {
	switch len(regions) {
	case 0:
		between()
		if len(owner.Owned("region")) == 0 {
			m.w.lines(commentLines("the " + kindOf(owner) + " has no region"))
		} else {
			m.w.lines(commentLines("the " + kindOf(owner) + "'s regions hold no vertex: nothing enters them"))
		}
	case 1:
		st := m.region(regions[0])
		st.enter(entered)
		between()
		st.write()
	default:
		name := m.parallel[regions[0]]
		withoutInitial := regionsWithoutInitial(regions)
		withoutOrthogonal := m.regionsWithoutOrthogonalInitialEntry(regions)
		withoutEntry := slices.Concat(withoutInitial, withoutOrthogonal)
		if len(withoutEntry) == 0 {
			m.w.line(entryThen(entered, "", writeName(name)))
		} else if len(withoutOrthogonal) == 0 {
			m.w.lines(commentLines("no default entry: the " + pluralRegion(len(withoutInitial)) + " " + strings.Join(withoutInitial, ", ") +
				" have no initial pseudostate, so only a fork or a transition naming a nested state enters the regions"))
		} else {
			m.w.lines(commentLines("no default entry: these regions have no written entry: " + strings.Join(withoutEntry, ", ") +
				"; only a fork or a transition naming a nested state enters them"))
		}
		between()
		m.w.block(stateKw+writeName(name)+" parallel", func() {
			for _, r := range regions {
				rname := m.names[r]
				m.w.block(stateKw+writeName(rname), func() {
					st := m.region(r)
					st.enter(false)
					st.write()
				})
				m.madeUp(r, writeName(rname))
				m.add(r, Mapped, rname, "an orthogonal region is written as a sub-state of the parallel state "+name)
			}
		})
		m.w.madeUp(writeName(name))
		m.w.line("transition first " + writeName(name) + " then done;")
	}
}

// regionsWithoutInitial names the regions no initial pseudostate starts.
func regionsWithoutInitial(regions []*sysmlv1.Element) []string {
	var out []string
	for _, r := range regions {
		has := false
		for _, v := range r.Owned("subvertex") {
			if pseudoKind(v) == "initial" {
				has = true
			}
		}
		if !has {
			out = append(out, describe(r))
		}
	}
	return out
}

// regionsWithoutOrthogonalInitialEntry names regions whose initial targets an orthogonal region.
func (m *migration) regionsWithoutOrthogonalInitialEntry(regions []*sysmlv1.Element) []string {
	var out []string
	for _, r := range regions {
		var initial *sysmlv1.Element
		for _, v := range r.Owned("subvertex") {
			if pseudoKind(v) == "initial" {
				initial = v
				break
			}
		}
		if initial == nil {
			continue
		}
		for _, t := range r.Owned("transition") {
			if m.model.Ref(t, "source") == initial {
				if isOrthogonalRegionTarget(r, m.model.Ref(t, "target")) {
					out = append(out, describe(r))
				}
				break
			}
		}
	}
	return out
}

func pluralRegion(n int) string {
	if n == 1 {
		return "region"
	}
	return "regions"
}

// region prepares to write region r of the machine that nameMachine named.
func (m *migration) region(r *sysmlv1.Element) *stateRegion {
	s := &stateRegion{m: m, r: r, owner: r.Parent, used: m.regionUsed[r], machine: machineOf(r)}
	if _, parallel := m.parallel[r]; parallel {
		s.owner = nil
	}
	return s
}

// machineOf returns the state machine an element of one belongs to.
func machineOf(e *sysmlv1.Element) *sysmlv1.Element {
	for cur := e; cur != nil; cur = cur.Parent {
		if cur.Type == "StateMachine" {
			return cur
		}
	}
	return nil
}

// freshIn returns base, or base with a number, not yet in used, and takes it.
func freshIn(used map[string]bool, base string) string {
	name := base
	for i := 2; used[name]; i++ {
		name = base + strconv.Itoa(i)
	}
	used[name] = true
	return name
}

// entryThen writes the succession into to from the body's entry action, an
// empty one unless entered says the body wrote its own; a named one is a
// transition out of `start`, which a view can route.
func entryThen(entered bool, name, to string) string {
	if name != "" {
		return transitionKeyword + writeName(name) + " first start then " + to + ";"
	}
	if entered {
		return "then " + to + ";"
	}
	return "entry; then " + to + ";"
}

// stateRegion writes one region: its entry, then its states and transitions,
// named against owner's body as its vertices were (nil for a parallel region's).
type stateRegion struct {
	m          *migration
	r          *sysmlv1.Element
	owner      *sysmlv1.Element
	used       map[string]bool
	machine    *sysmlv1.Element
	ownerScope bool
}

// enter writes the region's entry succession.
func (s *stateRegion) enter(entered bool) {
	s.initial(s.r.Owned("subvertex"), s.r.Owned("transition"), entered)
}

// write writes the region's states and transitions; enter comes first.
func (s *stateRegion) write() {
	vertices := s.r.Owned("subvertex")
	transitions := s.r.Owned("transition")
	for _, v := range vertices {
		s.vertex(v)
	}
	for _, t := range transitions {
		s.transition(t)
	}
	for _, v := range vertices {
		for _, c := range v.Owned("connection") {
			if !s.m.reported(c) {
				s.m.add(c, Unmapped, "", "no transition of the machine passes through the connection point reference, so nothing enters or leaves the submachine state by it")
			}
		}
	}
	s.m.writeComments(s.r, false)
	s.m.add(s.r, Mapped, "", "the one region is written as the body of its owner")
}

// name returns the v2 name nameMachine gave a vertex.
func (s *stateRegion) name(v *sysmlv1.Element) string {
	return s.m.vertexNames[v]
}

func pseudoKind(v *sysmlv1.Element) string {
	if v.Type != "Pseudostate" {
		return ""
	}
	if k := v.Attrs["kind"]; k != "" {
		return k
	}
	return "initial"
}

// initial writes the region's entry: `entry; then s` from its initial
// pseudostate, with the initial transition's effect as the entry action
// unless the body wrote its own, which entered says.
func (s *stateRegion) initial(vertices, transitions []*sysmlv1.Element, entered bool) {
	var init *sysmlv1.Element
	for _, v := range vertices {
		if pseudoKind(v) == "initial" {
			if init != nil {
				s.m.unmapped(v, "a region has one initial pseudostate; "+describe(init)+" is written as it")
				continue
			}
			init = v
		}
	}
	if init == nil {
		s.m.w.lines(commentLines("the region has no initial pseudostate: nothing enters it"))
		return
	}
	var out []*sysmlv1.Element
	for _, t := range transitions {
		if s.m.model.Ref(t, "source") == init {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		s.m.unmapped(init, "no transition leaves the initial pseudostate")
		return
	}
	t := out[0]
	for _, other := range out[1:] {
		s.m.unmapped(other, "an initial pseudostate has one outgoing transition; "+describe(t)+" is written as it")
	}
	tgt := s.m.model.Ref(t, "target")
	orthogonalTarget := isOrthogonalRegionTarget(s.r, tgt)
	var to string
	var ok bool
	if !orthogonalTarget {
		to, ok = s.target(t, tgt)
	}
	if !ok {
		s.m.unmapped(init, "the initial transition's target has no v2 form here")
		if tgt == nil {
			s.m.unmapped(t, joinNotes(s.m.dangling(t, "target"), "the transition lacks a target"))
		} else {
			why := "the target " + describe(tgt) + isA + kindOf(tgt) + outsideRegion
			if orthogonalTarget {
				why = "its target lies in an orthogonal region"
			}
			s.m.unmapped(t, why)
			if s.m.strict {
				s.refusedParts(t, why)
			}
		}
		return
	}
	name := s.initialName(t, to)
	note := ""
	for _, tr := range t.Owned("trigger") {
		note = "an initial transition takes no trigger; its triggers are dropped"
		s.m.add(tr, Unmapped, "", "a trigger of an initial transition, which takes none, is dropped")
	}
	if g := s.m.guardOf(t); g != nil {
		text := "the guard " + describe(g)
		if spec := firstOwned(g, "specification"); spec != nil {
			text = "[" + describeValue(spec) + "]"
		}
		note = joinNotes(note, "an initial transition takes no guard; "+text+" is dropped")
		s.m.add(g, Unmapped, "", "a guard of an initial transition, which takes none, is dropped")
	}
	switch eff := s.m.behaviorIn(t, "effect"); {
	case eff == nil:
		s.m.w.line(entryThen(entered, name, to))
	case entered:
		s.m.w.lines(commentLines("the effect " + describe(eff) + " of the initial transition is dropped: the state's entry behavior is its entry action"))
		s.m.unmapped(eff, "the effect of an initial transition is the entry action of the body, which the state's entry behavior is")
		s.m.w.line(entryThen(true, name, to))
	default:
		s.m.unbound(eff, "an initial transition accepts no signal")
		s.m.w.line(entryThen(s.m.inlineBehavior("entry action", eff, t), name, to))
	}
	s.m.add(init, Mapped, "", "written as the entry of the region")
	s.m.add(t, verdictFor(note), s.m.edgeTarget(t), note)
}

// isOrthogonalRegionTarget reports whether target belongs to a region unrelated to region.
func isOrthogonalRegionTarget(region, target *sysmlv1.Element) bool {
	var targetRegion *sysmlv1.Element
	for cur := target; cur != nil; cur = cur.Parent {
		if cur.Type == "Region" {
			targetRegion = cur
			break
		}
	}
	if targetRegion == nil || targetRegion == region {
		return false
	}
	for cur := targetRegion.Parent; cur != nil; cur = cur.Parent {
		if cur == region {
			return false
		}
	}
	for cur := region.Parent; cur != nil; cur = cur.Parent {
		if cur == targetRegion {
			return false
		}
	}
	return true
}

// initialName is the name the initial transition t is declared under when a
// diagram draws it, so the view can route it: the v1 name, else `start then s`
// made fresh; "" when it is written as the bare entry `entry; then s`.
func (s *stateRegion) initialName(t *sysmlv1.Element, to string) string {
	name := s.m.edgeName(t, spoken("start then "+to))
	if name == "" {
		s.m.wroteNoMember(t)
		return ""
	}
	name = s.m.freshMember(s.owner, s.used, name)
	s.m.wroteEdge(t, s.r, "transition", name)
	s.m.madeUp(t, writeName(name))
	return name
}

// vertex writes one vertex's declaration.
func (s *stateRegion) vertex(v *sysmlv1.Element) {
	switch v.Type {
	case "State":
		s.state(v)
	case "FinalState":
		s.m.add(v, Mapped, "done", "")
	case "Pseudostate":
		switch pseudoKind(v) {
		case "initial":
		case "choice", "junction":
			name := writeName(s.name(v))
			s.m.useStateMachines()
			s.m.w.line(pseudostateLine(s.m.stateMachinesPrefix(v.Parent), pseudoKind(v), name))
			s.m.madeUp(v, name)
			s.m.add(v, Mapped, name, "written as a #StateMachines::"+pseudoKind(v)+" state pseudostate, whose guarded transitions the runtime reads when it is reached; an unguarded one is its else branch")
		case "fork", "join":
			name := writeName(s.name(v))
			s.m.w.line(pseudoKind(v) + " " + name + ";")
			s.m.madeUp(v, name)
			s.m.add(v, Mapped, name, "written as a "+pseudoKind(v)+" pseudostate, whose segments the runtime fires together")
		case "shallowHistory":
			name := writeName(s.name(v))
			s.m.useStateMachines()
			s.m.w.line(pseudostateLine(s.m.stateMachinesPrefix(v.Parent), "shallowHistory", name))
			s.m.madeUp(v, name)
			s.m.add(v, Mapped, name, "written as a `#StateMachines::shallowHistory state` shallow history, which re-enters the substate active when its state was last left")
		case "deepHistory":
			name := writeName(s.name(v))
			s.m.useStateMachines()
			s.m.w.line(pseudostateLine(s.m.stateMachinesPrefix(v.Parent), "deepHistory", name))
			s.m.madeUp(v, name)
			s.m.add(v, Mapped, name, "written as a `#StateMachines::deepHistory state` deep history, which re-enters the innermost states active when its state was last left")
		case "terminate":
			s.m.add(v, Approximated, "done", "a terminate pseudostate ends the machine; a transition to it is written to done, which ends its region")
		case "entryPoint", "exitPoint":
			if pointOwner(v).Type == "State" {
				// Written in the body of the state it belongs to.
				return
			}
			s.m.connectionPoint(v)
		default:
			s.m.unmapped(v, "no v2 form for a "+pseudoKind(v)+" pseudostate")
		}
	case "ConnectionPointReference":
		// Written where a transition passes through it.
	default:
		s.m.unmapped(v, "no v2 form for a UML "+v.Type)
	}
}

// connectionPoint writes an entry or exit point of a state machine as a state of the state def: entered
// at the entry point's state, and leaving through the exit point's state completes the submachine state's transition.
func (m *migration) connectionPoint(v *sysmlv1.Element) {
	name, ok := m.vertexNames[v]
	if !ok {
		m.unmapped(v, "a "+pseudoKind(v)+" owned by a "+v.Parent.Type+" is not written; only a state machine's or a state's connection points are")
		return
	}
	m.w.line(stateKw + writeName(name) + ";")
	m.madeUp(v, writeName(name))
	if pseudoKind(v) == "exitPoint" {
		m.add(v, Mapped, name, "written as a state; a transition leaving a submachine state through the exit point leaves this state")
		return
	}
	m.add(v, Mapped, name, "written as a state; a transition entering a submachine state through the entry point enters this state")
}

// statePoints writes the connection points of composite state v in its body, in
// the form namePoints settled on; it reports how many members were written.
func (m *migration) statePoints(v *sysmlv1.Element) int {
	written := 0
	for _, cp := range m.connectionPoints(v) {
		f := m.points[cp]
		switch {
		case f.why != "":
			m.unmapped(cp, f.why)
		case f.defaultEntry:
			m.add(cp, Mapped, m.vertexNames[v], f.note)
		default:
			name := writeName(m.vertexNames[cp])
			if f.kw == "junction" {
				m.useStateMachines()
			}
			m.w.line(pseudostateLine(m.stateMachinesPrefix(m.scope), f.kw, name))
			m.madeUp(cp, name)
			m.add(cp, Mapped, name, f.note)
			written++
		}
	}
	return written
}

// writtenPoints counts the connection points of v written as members of its body.
func (m *migration) writtenPoints(v *sysmlv1.Element) int {
	n := 0
	for _, cp := range m.connectionPoints(v) {
		if m.points[cp].kw != "" {
			n++
		}
	}
	return n
}

// state writes a state, typed by its submachine's state def when it has one,
// with its entry, do and exit actions, deferrals and regions.
func (s *stateRegion) state(v *sysmlv1.Element) {
	name := writeName(s.name(v))
	s.m.madeUp(v, name)
	entry, do, exit := s.m.behaviorIn(v, "entry"), s.m.behaviorIn(v, "doActivity"), s.m.behaviorIn(v, "exit")
	defers := s.deferrals(v, do, exit)
	head := s.stateHead(v, name)
	regions := s.m.populatedRegions(v)
	inv := firstOwned(v, "stateInvariant")
	points := s.m.connectionPoints(v)
	pointRegs := pointRegions(v)
	ins := ""
	sub := s.m.model.Ref(v, "submachine")
	if sub != nil && s.m.written(sub) {
		ins, _ = s.m.contextIns(s.m.contextOf(sub), v)
	}
	if entry == nil && do == nil && exit == nil && inv == nil && len(regions) == 0 && len(defers.declared) == 0 && len(defers.lines) == 0 && s.m.writtenPoints(v) == 0 && !hasTransitions(pointRegs) {
		if ins != "" {
			s.m.w.line(head + " { " + strings.Join(s.m.contextBody(sub, ins), "; ") + "; }")
		} else {
			s.m.w.line(head + ";")
		}
		s.m.statePoints(v)
		return
	}
	s.m.w.block(head, func() {
		s.m.writeComments(v, false)
		if ins != "" {
			for _, member := range s.m.contextBody(sub, ins) {
				s.m.w.line(member + ";")
			}
		}
		s.deferredHead(v, defers)
		if inv != nil {
			s.m.invariant(inv)
		}
		entered := entry != nil && s.m.inlineBehavior("entry action", entry, v)
		between := s.betweenActions(v, do, exit, points, defers)
		if len(regions) == 0 {
			between()
		} else {
			s.m.regions(v, regions, entered, between)
		}
		s.pointTransitions(regions, pointRegs)
		s.m.writeRelocatedTransitions(v)
	})
}

// betweenActions is a state's block body after its entry action: its do and
// exit actions and its connection points.
func (s *stateRegion) betweenActions(v, do, exit *sysmlv1.Element, points []*sysmlv1.Element, defers *deferrals) func() {
	return func() {
		switch {
		case defers.encoded():
			s.deferredDo(v, do, defers)
			s.deferredExit(v, exit, defers)
		default:
			if do != nil {
				s.m.inlineBehavior(doAction, do, v)
			}
			if exit != nil {
				s.m.inlineBehavior(exitAction, exit, v)
			}
		}
		if len(points) > 0 {
			s.m.statePoints(v)
		}
	}
}

// stateHead writes a state's head: its name, typed by its submachine's state
// def when one is written; the connection points an unwritten submachine or a
// state without one leaves behind are marked unmapped.
func (s *stateRegion) stateHead(v *sysmlv1.Element, name string) string {
	head := stateKw + name
	if sub := s.m.model.Ref(v, "submachine"); sub != nil {
		if s.m.written(sub) {
			s.m.add(v, Mapped, name, "")
			return head + " : " + s.m.ref(sub, s.r)
		}
		for _, c := range v.Owned("connection") {
			s.m.unmapped(c, "the submachine "+qualifiedName(sub)+" has no v2 declaration, so its connection points are not written")
		}
		s.m.add(v, Approximated, name, "its submachine "+qualifiedName(sub)+" has no v2 declaration; the state is written simple")
		return head
	}
	for _, c := range v.Owned("connection") {
		s.m.unmapped(c, "the state has no submachine whose connection point the reference could name")
	}
	s.m.add(v, Mapped, name, "")
	return head
}

// pointTransitions writes the transitions each point region owns; a state with
// one region names them from it, so its vertices need no path.
func (s *stateRegion) pointTransitions(regions, pointRegs []*sysmlv1.Element) {
	for _, pr := range pointRegs {
		host := pr
		if len(regions) == 1 {
			host = regions[0]
		}
		for _, t := range pr.Owned("transition") {
			if _, relocated := s.m.relocatedTo[t]; relocated {
				continue
			}
			s.m.region(host).transition(t)
		}
	}
}

// hasTransitions reports whether any of the regions owns a transition.
func hasTransitions(regions []*sysmlv1.Element) bool {
	for _, r := range regions {
		if len(r.Owned("transition")) > 0 {
			return true
		}
	}
	return false
}

// invariant keeps a state invariant, which v2 has no form for, as a comment.
func (m *migration) invariant(inv *sysmlv1.Element) {
	note := "a state invariant has no v2 form"
	if spec := firstOwned(inv, "specification"); spec != nil {
		note += "; [" + describeValue(spec) + "] is kept as a comment"
	}
	m.unmapped(inv, note)
}

// deferral is a deferrable trigger a state keeps, with the members the
// migration encodes it through: its buffer item and its accept loop.
type deferral struct {
	trigger, event, sig *sysmlv1.Element
	// loops accept the signal by every route it reaches the object through;
	// item is the flush loop's variable.
	loops               []*deferralLoop
	buffer, item, clear string
	// contested is a transition out of the state, or out of a substate, that
	// takes the signal from the deferral only while its guard holds or its
	// substate is active.
	contested *sysmlv1.Element
	// completion is a guarded completion transition out of the state, which
	// the accept loop keeping the signal never lets the do action end for.
	completion *sysmlv1.Element
	// info and note say which routes the loops accept by, and which they skip.
	info, note string
}

// deferralLoop is one accept loop of a deferral: from the object itself when
// port is nil, via port otherwise.
type deferralLoop struct {
	port                   *sysmlv1.Element
	receive, keep, payload string
}

// deferrals is what a state defers and how it is written: the members of the
// buffering do and flushing exit actions that keep each deferred signal.
type deferrals struct {
	// declared is every signal the state's triggers defer, which its
	// annotations name whether or not the state keeps the signal.
	declared []*sysmlv1.Element
	kept     []*deferral
	// lines are the comments the state's body opens with, one per deferral
	// dropped.
	lines []string
	// split forks the accept loops beside the state's own do behavior, run;
	// exitRun is the state's own exit behavior inside flush.
	buffer, split, run, flush, exitRun string
}

// encoded reports whether the state's body carries the encoding: it keeps a signal.
func (d *deferrals) encoded() bool {
	return len(d.kept) > 0
}

// deferredEventFQN names the library metadata marking a deferred signal.
const deferredEventFQN = "MigrationMetadata::DeferredEvent"

// deferrals reads a state's deferrable triggers: v2 defers the signal a
// transition would accept, so no other event kind can be, and a signal a
// transition out of the state accepts is never deferred, the transition
// winning. Only a transition the output writes counts, so in a migration that
// writes a pseudostate a transition into it takes the signal, while in one
// that refuses the pseudostate it does not.
func (s *stateRegion) deferrals(v, do, exit *sysmlv1.Element) *deferrals {
	out := &deferrals{}
	for _, d := range v.Owned("deferrableTrigger") {
		s.readDeferral(v, d, out)
	}
	s.foldDeferrals(v, out)
	if len(out.kept) == 0 {
		return out
	}
	s.nameDeferrals(v, do, exit, out)
	for _, k := range out.kept {
		s.m.add(k.trigger, Approximated, "", s.deferralNote(v, k, out))
		s.m.add(k.event, Approximated, "", deferredBy(v)+" through the standard SysML v2 encoding, an accept loop keeping the signal while the state is active and an exit action sending it to self")
	}
	return out
}

// readDeferral reads one deferrable trigger, keeping the deferral of a signal
// no transition out of the state takes and no completion transition preempts.
func (s *stateRegion) readDeferral(v, d *sysmlv1.Element, out *deferrals) {
	ev := s.m.model.Ref(d, "event")
	if ev == nil {
		s.m.add(d, Unmapped, "", joinNotes(s.m.dangling(d, "event"), "the deferred trigger names no event"))
		return
	}
	if ev.Type != "SignalEvent" {
		s.m.add(d, Unmapped, "", "only a signal event can be deferred, not a "+ev.Type)
		return
	}
	sig := s.m.model.Ref(ev, "signal")
	if note, ok := s.m.signalOf(ev); !ok {
		s.m.add(d, Unmapped, "", note)
		s.m.add(ev, Unmapped, "", note)
		return
	}
	k := &deferral{trigger: d, event: ev, sig: sig}
	out.declared = append(out.declared, sig)
	s.routeDeferral(k)
	always, contested := s.acceptsOutOf(v, sig)
	if t, taken := s.takeRoutes(k, always); len(k.loops) == 0 {
		note := theTransition(t) + " out of the state accepts the signal, which in v1 takes precedence over deferring it, so the state does not keep it; its @" + deferredEventFQN + " annotation records the deferral"
		s.m.add(d, Approximated, "", note)
		s.m.add(ev, Approximated, "", deferredBy(v)+", which the state's @"+deferredEventFQN+" annotation records; the state does not keep the signal, the transition "+describe(t)+" accepting it")
		return
	} else if taken != "" {
		k.info = joinNotes(k.info, taken)
	}
	k.contested = contested
	alwaysDone, guardedDone := s.completionOutOf(v)
	if alwaysDone != nil {
		note := "the completion transition " + describe(alwaysDone) + " leaves the state once its do action ends, which the accept loop that would keep " + qualifiedName(sig) + " never lets it, so the deferral is dropped"
		out.lines = append(out.lines, commentLines("not migrated: deferrableTrigger "+describe(d)+" on "+qualifiedName(sig)+" — "+note)...)
		s.m.add(d, Unmapped, "", note)
		s.m.add(ev, Unmapped, "", note)
		return
	}
	k.completion = guardedDone
	out.kept = append(out.kept, k)
}

// deferralNote explains how a kept deferral is encoded and where the encoding
// departs from v1's precedence.
func (s *stateRegion) deferralNote(v *sysmlv1.Element, k *deferral, out *deferrals) string {
	note := "kept in the item " + k.buffer + " by the accept loop of the do action " + out.buffer + " while the state is active, and sent to self by the exit action " + out.flush + ": the standard SysML v2 encoding of a deferred signal, which the state's @" + deferredEventFQN + " annotation records"
	note = joinNotes(joinNotes(note, k.info), k.note)
	if len(out.kept) > 1 {
		note = joinNotes(note, "the state's deferred signals are flushed one signal at a time, each in arrival order, so the order between occurrences of different signals is not kept; UML leaves the order of the event pool open, so that is a permitted approximation")
	}
	if t := k.contested; t != nil {
		note = joinNotes(note, s.contestedNote(v, t, k.sig))
	}
	if t, enclosing := s.outrankedBy(v, k.sig); t != nil {
		if enclosing {
			note = joinNotes(note, theTransition(t)+" out of the enclosing state "+describe(s.m.model.Ref(t, "source"))+" accepts the signal too, which in v1 the deferral takes precedence over while the state is active; the standard encoding cannot hold a signal back from a transition of an enclosing state, so that transition takes each occurrence it can fire on and the accept loop keeps the rest")
		} else {
			note = joinNotes(note, theTransition(t)+" out of "+describe(s.m.model.Ref(t, "source"))+", in a region beside the state's, accepts the signal too, which in v1 the deferral takes precedence over while the state is active; the standard encoding cannot hold a signal back from a transition of another region, so that transition fires on each occurrence it can, which the accept loop keeps as well")
		}
	}
	if t := k.completion; t != nil {
		note = joinNotes(note, "the guarded completion transition "+describe(t)+" leaves the state once its do action ends, which the accept loop never lets it: the state keeps the signal, and leaves only by a transition a trigger fires")
	}
	if t := s.internalOutOf(v); t != nil {
		note = joinNotes(note, "the internal transition "+describe(t)+" is written as a self transition, which exits and re-enters the state where v1 stayed in it: the exit action sends the kept occurrences to self, and the accept loop, started again, keeps them again unless a transition then accepts them")
	}
	return note
}

// contestedNote explains a transition that contests the deferral of sig.
func (s *stateRegion) contestedNote(v, t, sig *sysmlv1.Element) string {
	switch {
	case !s.m.acceptsGeneralOf(t, sig):
		return theTransition(t) + " out of the state accepts a specialization of the signal, which in v1 takes precedence over deferring those occurrences; the standard leaves open which of the transition and the accept loop takes them, which the runtime settles for the transition when it can fire and the loop otherwise"
	case s.m.model.Ref(t, "source") != v:
		return theTransition(t) + " out of a substate accepts the signal too, which in v1 takes precedence over deferring it only while that substate is active; the standard leaves open which of the transition and the accept loop takes the signal, which the runtime settles for the transition when it can fire and the loop otherwise"
	}
	return theTransition(t) + " out of the state accepts the signal too, which in v1 takes precedence over deferring it only while its guard holds; the standard leaves open which of the transition and the accept loop takes the signal, which the runtime settles for the transition when its guard holds and the loop otherwise"
}

// foldDeferrals drops from a deferral every loop another deferral of the
// state, on the same signal or a general of it, accepts by too: the general's
// loop keeps those occurrences already, and two loops would keep them twice.
// A deferral left with no loop is reported as kept by the deferral its loops
// went to that itself keeps a loop, through any folded away in between.
func (s *stateRegion) foldDeferrals(v *sysmlv1.Element, d *deferrals) {
	by := map[*deferral]*deferral{}
	all := d.kept
	var kept []*deferral
	stays := map[*deferral]bool{}
	for i, k := range all {
		for j, g := range all {
			if i == j || !s.m.signalConforms(k.sig, g.sig) || (g.sig == k.sig && j > i) {
				continue
			}
			loops := k.loops[:0]
			for _, l := range k.loops {
				if g.hasRoute(l.port) {
					by[k] = g
					continue
				}
				loops = append(loops, l)
			}
			k.loops = loops
		}
		if len(k.loops) > 0 {
			kept = append(kept, k)
			stays[k] = true
		}
	}
	for _, k := range all {
		if stays[k] {
			continue
		}
		g := by[k]
		for !stays[g] {
			g = by[g]
		}
		note := "kept by the accept loop of the deferral of " + qualifiedName(g.sig) + " in the same state, which accepts every occurrence of the signal too; a loop of its own would keep each occurrence twice"
		s.m.add(k.trigger, Approximated, "", note)
		s.m.add(k.event, Approximated, "", deferredBy(v)+" through the deferral of "+qualifiedName(g.sig)+", whose accept loop keeps every occurrence of the signal")
	}
	d.kept = kept
}

// hasRoute reports whether the deferral's loops accept by the route of port,
// the object itself when nil.
func (k *deferral) hasRoute(port *sysmlv1.Element) bool {
	for _, l := range k.loops {
		if l.port == port {
			return true
		}
	}
	return false
}

// nameDeferrals names the members the standard encoding adds to the state's
// body and to its do action, clear of the members the state has of its own.
func (s *stateRegion) nameDeferrals(v, do, exit *sysmlv1.Element, d *deferrals) {
	used, ok := s.m.stateUsed[v]
	if !ok {
		used = inheritedStateNamesSet()
		s.m.stateUsed[v] = used
	}
	inner := inheritedActionNamesSet()
	outer := inheritedActionNamesSet()
	s.reserveDeferralRefs(d, used, inner, outer)
	for _, k := range d.kept {
		base := "deferred"
		if len(d.kept) > 1 {
			base += identifierSuffix(s.m.nameFor(k.sig))
		}
		k.buffer = s.m.freshMember(v, used, base)
	}
	d.buffer = s.m.freshMember(v, used, "buffer")
	d.flush = s.m.freshMember(v, used, "flush")
	if do != nil {
		d.run = s.m.nestedBehaviorName(do, v, inner, "run", d.buffer)
	}
	if exit != nil {
		d.exitRun = s.m.nestedBehaviorName(exit, v, outer, "run", d.flush)
	}
	if do != nil || d.loopCount() > 1 {
		d.split = freshIn(inner, "split")
	}
	for _, k := range d.kept {
		suffix := ""
		if len(d.kept) > 1 {
			suffix = identifierSuffix(s.m.nameFor(k.sig))
		}
		for _, l := range k.loops {
			via := suffix
			if l.port != nil {
				via += " via " + s.m.nameFor(l.port)
			}
			l.receive = freshIn(inner, "receive"+via)
			l.keep = freshIn(inner, "keep"+via)
			l.payload = freshIn(inner, deferredPayload+via)
		}
		k.item = freshIn(outer, deferredPayload+suffix)
		k.clear = freshIn(outer, "clear"+suffix)
	}
}

// reserveDeferralRefs takes, in every scope the encoding names members in, the
// names the encoding refers to from there — a signal and the root of its
// qualified name, a port, the library packages — so no generated member
// shadows them.
func (s *stateRegion) reserveDeferralRefs(d *deferrals, scopes ...map[string]bool) {
	names := []string{"SequenceFunctions", "MigrationMetadata"}
	for _, k := range d.kept {
		names = append(names, s.m.nameOf(k.sig))
		if p := s.m.path(k.sig); len(p) > 0 {
			names = append(names, p[0].name)
		}
		for _, l := range k.loops {
			if l.port != nil {
				names = append(names, s.m.nameFor(l.port))
			}
		}
	}
	for _, name := range names {
		for _, used := range scopes {
			used[name] = true
		}
	}
}

// routeDeferral gives a deferral its accept loops, one per route the signal
// reaches the object by: the ports the trigger names, or else the object
// itself and every port a connector delivers the signal to, as a transition's
// trigger would accept it.
func (s *stateRegion) routeDeferral(k *deferral) {
	ports, direct, info, note := s.m.portRoutes(k.trigger, classifierOf(k.trigger), k.sig)
	if direct {
		k.loops = append(k.loops, &deferralLoop{})
	}
	for _, p := range ports {
		k.loops = append(k.loops, &deferralLoop{port: p})
	}
	if len(ports) > 0 {
		info = joinNotes(info, "an occurrence that arrived at a port is sent back to the object itself, which a trigger naming no port accepts")
	}
	k.info, k.note = info, note
}

// takeRoutes drops from a deferral's loops every route one of the transitions
// always, which leave the state on the signal under no guard, accepts by: the
// transition takes the signal there, so the state need not keep it. It returns
// the first transition that took a route, and a note naming what it took when
// loops remain.
func (s *stateRegion) takeRoutes(k *deferral, always []*sysmlv1.Element) (taker *sysmlv1.Element, note string) {
	var taken []string
	for _, t := range always {
		for _, tr := range t.Owned("trigger") {
			ev := s.m.model.Ref(tr, "event")
			if ev == nil || !s.m.signalConforms(k.sig, s.m.model.Ref(ev, "signal")) {
				continue
			}
			ports, direct, _, _ := s.m.portRoutes(tr, classifierOf(tr), s.m.model.Ref(ev, "signal"))
			kept := k.loops[:0]
			for _, l := range k.loops {
				switch {
				case l.port == nil && direct:
					taken = append(taken, "from the object itself")
				case l.port != nil && containsElement(ports, l.port):
					taken = append(taken, "via "+s.m.nameFor(l.port))
				default:
					kept = append(kept, l)
					continue
				}
				if taker == nil {
					taker = t
				}
			}
			k.loops = kept
		}
	}
	if taker == nil || len(k.loops) == 0 {
		return taker, ""
	}
	return taker, theTransition(taker) + " out of the state accepts the signal " + strings.Join(taken, " and ") + ", which in v1 takes precedence over deferring it, so no loop keeps it there"
}

// containsElement reports whether es holds e.
func containsElement(es []*sysmlv1.Element, e *sysmlv1.Element) bool {
	for _, x := range es {
		if x == e {
			return true
		}
	}
	return false
}

// loopCount is how many accept loops the encoding writes in all.
func (d *deferrals) loopCount() int {
	n := 0
	for _, k := range d.kept {
		n += len(k.loops)
	}
	return n
}

// nestedBehaviorName names the action a state's behavior becomes in the body
// of the generated action it is nested in: the behavior's own name when the
// state owns it and no inherited member takes it, base otherwise. A behavior
// declared elsewhere keeps its own name there; the nested action is fresh.
// An owned behavior's qualified name gains the segment within.
func (m *migration) nestedBehaviorName(b, owner *sysmlv1.Element, used map[string]bool, base, within string) string {
	name := m.nameOf(b)
	if b.Parent != owner {
		if name != "" && !used[name] {
			used[name] = true
			return name
		}
		return freshIn(used, base)
	}
	m.nestedIn[b] = within
	if name == "" || used[name] {
		name = freshIn(used, base)
		m.names[b], m.synthesized[b] = name, true
		return name
	}
	used[name] = true
	return name
}

// acceptsOutOf returns the transitions out of state v, or out of a vertex
// within it, that a trigger referring to signal sig fires: always holds those
// that always take the signal from the deferral by the routes their triggers
// accept by, out of v itself under no guard or a true one; contested is the
// first other, which takes the signal only while its guard holds or while the
// vertex it leaves is active, so the state keeps the signal the rest of the
// time. nil when no transition accepts.
func (s *stateRegion) acceptsOutOf(v, sig *sysmlv1.Element) (always []*sysmlv1.Element, contested *sysmlv1.Element) {
	m := s.m
	var walk func(e *sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		for _, t := range m.outgoing[e] {
			if !s.transitionWritten(t) {
				continue
			}
			for _, tr := range t.Owned("trigger") {
				if ev := m.model.Ref(tr, "event"); ev != nil && ev.Type == "SignalEvent" && m.signalsMeet(sig, m.model.Ref(ev, "signal")) {
					g := m.guardOf(t)
					if e == v && m.signalConforms(sig, m.model.Ref(ev, "signal")) && (g == nil || trueLiteral(firstOwned(g, "specification"))) {
						always = append(always, t)
					} else if contested == nil {
						contested = t
					}
					break
				}
			}
		}
		for _, r := range e.Owned("region") {
			for _, sub := range r.Owned("subvertex") {
				walk(sub)
			}
		}
	}
	walk(v)
	return always, contested
}

// outrankedBy returns a transition a trigger referring to signal sig fires
// out of a state enclosing v, or out of a vertex in a region beside the one
// holding v, and whether it leaves an enclosing state. In v1 the deferral
// takes precedence over either while v is active; the standard encoding holds
// the signal back from neither. nil when none.
func (s *stateRegion) outrankedBy(v, sig *sysmlv1.Element) (t *sysmlv1.Element, enclosing bool) {
	for cur := v; cur.Parent != nil && cur.Parent.Type == "Region"; cur = cur.Parent.Parent {
		region := cur.Parent
		owner := region.Parent
		if owner == nil {
			break
		}
		if owner.Type == "State" {
			if t := s.firesOutOf(owner, sig); t != nil {
				return t, true
			}
		}
		for _, r := range owner.Owned("region") {
			if r == region {
				continue
			}
			for _, sub := range r.Owned("subvertex") {
				if t := s.firesWithin(sub, sig); t != nil {
					return t, false
				}
			}
		}
		if owner.Type != "State" {
			break
		}
	}
	return nil, false
}

// firesOutOf is a written transition out of e that a trigger referring to sig fires.
func (s *stateRegion) firesOutOf(e, sig *sysmlv1.Element) *sysmlv1.Element {
	for _, t := range s.m.outgoing[e] {
		if s.acceptsSignal(t, sig) {
			return t
		}
	}
	return nil
}

// firesWithin is a transition sig fires out of e or any vertex nested in it.
func (s *stateRegion) firesWithin(e, sig *sysmlv1.Element) *sysmlv1.Element {
	if t := s.firesOutOf(e, sig); t != nil {
		return t
	}
	for _, r := range e.Owned("region") {
		for _, sub := range r.Owned("subvertex") {
			if t := s.firesWithin(sub, sig); t != nil {
				return t
			}
		}
	}
	return nil
}

// acceptsSignal reports whether a written transition has a trigger referring to sig.
func (s *stateRegion) acceptsSignal(t, sig *sysmlv1.Element) bool {
	if !s.transitionWritten(t) {
		return false
	}
	m := s.m
	for _, tr := range t.Owned("trigger") {
		if ev := m.model.Ref(tr, "event"); ev != nil && ev.Type == "SignalEvent" && m.signalsMeet(sig, m.model.Ref(ev, "signal")) {
			return true
		}
	}
	return false
}

// transitionWritten reports whether transition t is one the output writes, so
// it accepts its trigger or completes its source: its ends resolve as
// transitionEnds requires, and its target is one target names — a state of the
// machine, a final or terminate state of the transition's own region, a
// pseudostate with a v2 form, or a connection point reference into a
// submachine state. It decides as target does, noting nothing.
func (s *stateRegion) transitionWritten(t *sysmlv1.Element) bool {
	src, tgt := s.m.model.Ref(t, "source"), s.m.model.Ref(t, "target")
	internal := t.Attrs["kind"] == "internal"
	if internal && tgt == nil && len(s.m.model.Unresolved(t, "target")) == 0 {
		tgt = src
	}
	if src == nil || tgt == nil || tgt.IsProxy() || pseudoKind(src) == "initial" {
		return false
	}
	if internal && (src.Type != "State" || tgt != src) {
		return false
	}
	named := func(v *sysmlv1.Element) bool {
		_, ok := s.path(v)
		return ok
	}
	switch tgt.Type {
	case "FinalState":
		return tgt.Parent == t.Parent
	case "State":
		return named(tgt)
	case "Pseudostate":
		switch pseudoKind(tgt) {
		case "terminate":
			return true
		case "exitPoint":
			return s.m.points[tgt].why == "" && (named(tgt) || pointOwner(tgt).Type != "State")
		case "entryPoint":
			f := s.m.points[tgt]
			if f.why != "" {
				return false
			}
			if f.defaultEntry {
				return named(pointOwner(tgt))
			}
			return named(tgt)
		case "choice", "junction", "shallowHistory", "deepHistory", "fork", "join":
			return named(tgt)
		}
	case "ConnectionPointReference":
		return s.connectionWritten(tgt, "entry")
	}
	return false
}

// connectionWritten reports whether a transition through connection point
// reference v, as role, is one connection writes: the reference lies in a
// submachine state that is written, and names a connection point of the
// submachine as its role.
func (s *stateRegion) connectionWritten(v *sysmlv1.Element, role string) bool {
	st := v.Parent
	if st == nil || st.Type != "State" {
		return false
	}
	sub := s.m.model.Ref(st, "submachine")
	if sub == nil || !s.m.written(sub) {
		return false
	}
	points := s.m.model.Refs(v, role)
	if len(points) == 0 {
		return false
	}
	s.m.nameMachine(sub)
	if _, ok := s.m.vertexNames[points[0]]; !ok || machineOf(points[0]) != sub {
		return false
	}
	_, ok := s.path(st)
	return ok
}

// signalConforms reports whether every occurrence of signal sig is one of
// signal general: sig is general or specializes it, so a trigger referring to
// general accepts sig, as a v2 accept typed by general does.
func (m *migration) signalConforms(sig, general *sysmlv1.Element) bool {
	if sig == nil || general == nil {
		return false
	}
	seen := map[*sysmlv1.Element]bool{}
	var walk func(cur *sysmlv1.Element) bool
	walk = func(cur *sysmlv1.Element) bool {
		if cur == nil || seen[cur] {
			return false
		}
		seen[cur] = true
		if cur == general {
			return true
		}
		for _, g := range cur.Owned("generalization") {
			if walk(m.model.Ref(g, "general")) {
				return true
			}
		}
		return false
	}
	return walk(sig)
}

// signalsMeet reports whether some occurrence is of both signals: one
// specializes the other, so a trigger on either can accept it.
func (m *migration) signalsMeet(a, b *sysmlv1.Element) bool {
	return m.signalConforms(a, b) || m.signalConforms(b, a)
}

// acceptsGeneralOf reports whether a trigger of transition t refers to sig or
// a general of it, accepting every occurrence of sig.
func (m *migration) acceptsGeneralOf(t, sig *sysmlv1.Element) bool {
	for _, tr := range t.Owned("trigger") {
		if ev := m.model.Ref(tr, "event"); ev != nil && m.signalConforms(sig, m.model.Ref(ev, "signal")) {
			return true
		}
	}
	return false
}

// internalOutOf returns the first internal transition of state v the output
// writes, as a self transition that exits and re-enters the state. nil when none.
func (s *stateRegion) internalOutOf(v *sysmlv1.Element) *sysmlv1.Element {
	for _, t := range s.m.outgoing[v] {
		if t.Attrs["kind"] == "internal" && len(t.Owned("trigger")) > 0 && s.transitionWritten(t) {
			return t
		}
	}
	return nil
}

// completionOutOf returns the transitions out of state v that no trigger
// fires, which v1 takes when the state's do activity completes: always is one
// under no guard or a true one, which then leaves for certain; guarded is one
// under any other guard, which may leave the state active. A transition the
// output does not write is none. nil when none.
func (s *stateRegion) completionOutOf(v *sysmlv1.Element) (always, guarded *sysmlv1.Element) {
	m := s.m
	for _, t := range m.outgoing[v] {
		if len(t.Owned("trigger")) > 0 || t.Attrs["kind"] == "internal" || !s.transitionWritten(t) {
			continue
		}
		if g := m.guardOf(t); g == nil || trueLiteral(firstOwned(g, "specification")) {
			if always == nil {
				always = t
			}
		} else if guarded == nil {
			guarded = t
		}
	}
	return always, guarded
}

// deferredHead writes what a state's body opens with for its deferrals: the
// comment for each dropped, the annotation naming each deferred signal, and
// the item buffering each kept.
func (s *stateRegion) deferredHead(v *sysmlv1.Element, d *deferrals) {
	s.m.w.lines(d.lines)
	prefix := ""
	if s.m.shadowsLibrary("MigrationMetadata", v) {
		prefix = "$::"
	}
	for _, sig := range d.declared {
		s.m.w.line("@" + prefix + deferredEventFQN + " { ref :>> signal : " + s.m.ref(sig, v) + "; }")
	}
	if !d.encoded() {
		return
	}
	for _, k := range d.kept {
		s.m.w.line("item " + writeName(k.buffer) + " : " + s.m.ref(k.sig, v) + "[*] ordered;")
		s.m.w.madeUp(k.buffer)
	}
}

// deferredPayload is the base name of a generated accept's parameter and of a
// generated flush loop's variable.
const deferredPayload = "kept"

// deferredDo writes the do action that keeps each deferred signal while the
// state is active: one endless accept loop per signal, run beside the state's
// own do behavior when it has one.
func (s *stateRegion) deferredDo(v, do *sysmlv1.Element, d *deferrals) {
	s.m.w.block(doAction+" "+writeName(d.buffer), func() {
		// The state's own do behavior is rendered first: only one that is
		// written as an action gets a branch of the fork.
		run, ran := "", false
		if do != nil {
			run = s.m.w.aside(func() {
				ran = s.m.inlineBehaviorHeaded(doAction, actionKeyword+writeName(d.run), do, v)
			})
		}
		if ran || d.loopCount() > 1 {
			s.m.w.line("first start then " + d.split + ";")
			s.m.w.line("fork " + d.split + ";")
			s.m.w.madeUp(d.split)
			if ran {
				s.m.w.line("then " + writeName(d.run) + ";")
			}
			for _, k := range d.kept {
				for _, l := range k.loops {
					s.m.w.line("then " + writeName(l.receive) + ";")
				}
			}
		} else {
			s.m.w.line("first start then " + writeName(d.kept[0].loops[0].receive) + ";")
		}
		_, _ = s.m.w.buf().WriteString(run)
		if ran {
			s.m.madeUp(do, d.run)
		}
		including := "SequenceFunctions::including"
		if s.m.shadowsLibrary("SequenceFunctions", v) {
			including = "$::" + including
		}
		for _, k := range d.kept {
			for _, l := range k.loops {
				via := ""
				if l.port != nil {
					via = " via " + s.m.ownerPrefix(v) + writeName(s.m.nameFor(l.port))
				}
				receive, keep, payload := writeName(l.receive), writeName(l.keep), writeName(l.payload)
				s.m.w.line(actionKeyword + receive + " accept " + payload + " : " + s.m.acceptSignalRef(k.sig, v, payload) + via + ";")
				s.m.w.line("then action " + keep + " { assign " + writeName(k.buffer) + " := " + including + "(" + writeName(k.buffer) + ", " + receive + "." + payload + "); }")
				s.m.w.line("then " + receive + ";")
				s.m.w.madeUp(receive)
				s.m.w.madeUp(keep)
			}
		}
	})
	s.m.w.madeUp(d.buffer)
}

// deferredExit writes the exit action that sends each kept occurrence to the
// state's own object once the state exits, after the state's own exit behavior,
// and empties each buffer so a later visit replays only what it kept.
func (s *stateRegion) deferredExit(v, exit *sysmlv1.Element, d *deferrals) {
	s.m.w.block(exitAction+" "+writeName(d.flush), func() {
		prefix := ""
		if exit != nil && s.m.inlineBehaviorHeaded(exitAction, actionKeyword+writeName(d.exitRun), exit, v) {
			s.m.madeUp(exit, d.exitRun)
			prefix = "then "
		}
		for _, k := range d.kept {
			s.m.w.line(prefix + "for " + writeName(k.item) + " in " + writeName(k.buffer) + " { send " + writeName(k.item) + " to self; }")
			s.m.w.line("then action " + k.clear + " { assign " + writeName(k.buffer) + " := (); }")
			s.m.w.madeUp(k.clear)
			prefix = "then "
		}
	})
	s.m.w.madeUp(d.flush)
}

// inheritedActionNamesSet is a fresh used-name set seeded with the members
// every action usage inherits, for an action body that hands out names.
func inheritedActionNamesSet() map[string]bool {
	return map[string]bool{"start": true, "done": true, "self": true}
}

// inheritedStateNames are the members every state usage inherits, which the
// actions written inside one must not be called.
var inheritedStateNames = map[string]bool{
	"start": true, "done": true, "self": true,
	"entryAction": true, "doAction": true, "exitAction": true,
}

// inheritedStateNamesSet is a fresh used-name set seeded with the inherited
// state members, for a body that hands out names.
func inheritedStateNamesSet() map[string]bool {
	used := map[string]bool{}
	for n := range inheritedStateNames {
		used[n] = true
	}
	return used
}

// aside writes a note where the grammar admits a comment: in place, or before
// the declaration being written when one has claimed the notes.
func (m *migration) aside(text string) {
	if m.asides != nil {
		*m.asides = append(*m.asides, commentLines(text)...)
		return
	}
	m.w.lines(commentLines(text))
}

// inlineBehavior writes a behavior a state or transition owns as the action
// kw of the current body, and reports whether anything was written.
func (m *migration) inlineBehavior(kw string, b, owner *sysmlv1.Element) bool {
	return m.inlineBehaviorHeaded(kw, kw, b, owner)
}

// inlineBehaviorHeaded is inlineBehavior declaring the behavior as head, a
// keyword and name, where it is nested; kw still says what it is to the state.
func (m *migration) inlineBehaviorHeaded(kw, head string, b, owner *sysmlv1.Element) bool {
	if b.Parent != owner {
		return m.referencedBehavior(kw, head, b, owner)
	}
	saved := m.scope
	m.scope = b
	defer func() { m.scope = saved }()
	if owner.Type != "Transition" {
		bound := m.carrierBindings(owner, b)
		switch {
		case bound != nil && kw != exitAction:
			savedBound, savedNote := m.bound, m.boundNote
			m.bound, m.boundNote = bound, "an attribute of the signal the transitions into the state accept"
			defer func() { m.bound, m.boundNote = savedBound, savedNote }()
		case owner.Type == "State":
			m.unbound(b, joinNotes(performsIts+kw+" with no arguments", m.carrierWhy(owner, kw)))
		default:
			m.unbound(b, performsIts+kw+" with no arguments; only a transition's effect receives the accepted signal")
		}
	}
	header := head
	if name := m.nameOf(b); name != "" && head == kw {
		if inheritedStateNames[name] {
			name = freshIn(map[string]bool{name: true}, name)
			m.names[b] = name
		}
		header += " " + writeName(name)
	}
	return m.writeInlineBody(kw, b, owner, header)
}

// effectClause reports whether kw is a transition's effect, which its target
// follows on the next line, so no `;` may end it; a state's action stands alone.
func effectClause(kw string, owner *sysmlv1.Element) bool {
	return owner.Type == "Transition" && kw == doAction
}

// writeInlineBody writes an inline behavior's block by its kind: an activity's
// full body, an opaque body's statements or a comment, nothing otherwise.
func (m *migration) writeInlineBody(kw string, b, owner *sysmlv1.Element, header string) bool {
	enclose := m.w.block
	if effectClause(kw, owner) {
		enclose = m.w.braced
	}
	switch b.Type {
	case "Activity":
		enclose(header, func() {
			m.comments(b)
			m.parameters(b, b)
			m.activityBody(b, b)
		})
		note := "written as the " + kw + " of " + describe(owner)
		switch refused, total := m.nodesRefused(b); {
		case len(b.Owned("node")) == 0:
			note = joinNotes(note, "the activity has no nodes, so the action is empty")
		case refused > 0 && refused == total:
			m.add(b, Approximated, m.v2Name(b), joinNotes(note, "none of its "+count(total, "action")+" is migrated, so it runs nothing"))
			return true
		}
		m.add(b, Mapped, m.v2Name(b), note)
		return true
	case "OpaqueBehavior", "FunctionBehavior":
		body, lang := opaqueBody(b)
		lines, ok, note := m.statements(body, lang, b)
		enclose(header, func() {
			m.comments(b)
			m.parameters(b, b)
			if ok {
				m.w.lines(lines)
			} else {
				m.opaqueComment(body, lang, note)
			}
			if m.keeping != "" {
				m.w.line(m.keeping)
			}
		})
		if ok {
			m.add(b, Approximated, m.v2Name(b), "the "+langName(lang)+" body is written as v2 assignments")
		} else {
			m.add(b, Approximated, m.v2Name(b), "the body is kept as a comment: "+note)
		}
		return true
	}
	m.aside(kw + " " + describe(b) + isA + b.Type + ", which has no action form")
	m.add(b, Unmapped, "", "a "+b.Type+" has no action form")
	return false
}

// referencedBehavior writes `kw : ref` for a behavior written elsewhere, binding
// its context and signal-carried parameters where it can and reporting why not.
func (m *migration) referencedBehavior(kw, head string, b, owner *sysmlv1.Element) bool {
	if !m.written(b) {
		m.aside(kw + " " + qualifiedName(b) + " has no v2 declaration")
		m.add(b, Unmapped, "", "the behavior is not written; "+describe(owner)+" names it as its "+kw)
		m.add(owner, Approximated, "", "its "+kw+" "+qualifiedName(b)+notRun+"it has no v2 declaration")
		return false
	}
	if cat, _ := m.classify(b); cat != catActionDef {
		m.aside(kw + " " + qualifiedName(b) + " is written as a " + cat.keyword() + ", which no state runs")
		m.downgrade(b, describe(owner)+" names it as its "+kw+", which a "+cat.keyword()+" cannot be")
		m.add(owner, Approximated, "", "its "+kw+" "+qualifiedName(b)+notRun+"it is written as a "+cat.keyword())
		return false
	}
	var ins []string
	note := "also run as the " + kw + " of " + describe(owner)
	var contextIn string
	if c := m.contextOf(b); c != nil {
		in, cnote := m.contextIns(c, owner)
		if in == "" && cnote != "" {
			m.aside(kw + " " + qualifiedName(b) + notRun + cnote)
			m.downgrade(b, "not run as the "+kw+" of "+describe(owner)+": "+cnote)
			m.add(owner, Approximated, "", "its "+kw+" "+qualifiedName(b)+notRun+cnote)
			return false
		}
		contextIn = in
		note = joinNotes(note, cnote)
	}
	if params := inParameters(b); len(params) > 0 && owner.Type != "Transition" {
		var ok bool
		ins, note, ok = m.parameterIns(kw, b, owner, params, ins, note)
		if !ok {
			return false
		}
	}
	if contextIn != "" {
		// The context redefinition follows the parameter bindings.
		ins = append(ins, contextIn)
	}
	line := head + " : " + m.ref(b, owner)
	switch {
	case len(ins) > 0:
		line += " { " + strings.Join(ins, "; ") + "; }"
	case !effectClause(kw, owner):
		line += ";"
	}
	m.w.line(line)
	m.downgrade(b, note)
	return true
}

// parameterIns binds a referenced behavior's parameters: the carrier's signal
// attributes when it has one, otherwise it reports what nothing supplies.
func (m *migration) parameterIns(kw string, b, owner *sysmlv1.Element, params []*sysmlv1.Element, ins []string, note string) ([]string, string, bool) {
	why := performsIts + kw + " with no arguments; " + m.carrierWhy(owner, kw)
	bound := m.carrierBindings(owner, b)
	switch {
	case bound != nil && kw != exitAction:
		for _, p := range params {
			ins = append(ins, m.parameterBinding(p, m.nameFor(p), m.callBodyExpr(bound[p], owner)))
		}
		return ins, joinNotes(note, "its parameters take the attributes of the signal the transitions into the state accept"), true
	case slices.IndexFunc(params, requiresValue) >= 0:
		p := params[slices.IndexFunc(params, requiresValue)]
		why = "its parameter " + m.nameFor(p) + " must hold a value that nothing supplies: " + why
		m.aside(kw + " " + qualifiedName(b) + notRun + why)
		m.downgrade(b, "not run as the "+kw+" of "+describe(owner)+": "+why)
		m.add(owner, Approximated, "", "its "+kw+" "+qualifiedName(b)+notRun+why)
		return ins, note, false
	default:
		return ins, joinNotes(note, "its parameters take no value: "+why), true
	}
}

// path names vertex v from region s.r: its name when the region holds it, else
// its name qualified from the state def down, which resolves from any region
// of the machine; false for a vertex of another machine or one not written.
func (s *stateRegion) path(v *sysmlv1.Element) (string, bool) {
	name, ok := s.m.vertexNames[v]
	if !ok || machineOf(v) != s.machine {
		return "", false
	}
	if owner := memberOwner(v); owner == s.machine || owner == s.r && !s.ownerScope {
		return writeName(name), true
	}
	segs := s.m.segments(v)
	base := s.owner
	if base == nil {
		base = s.machine
	}
	return qualifiedStatePathWithSeparator(segs[len(s.m.segments(base)):], "."), true
}

// qualifiedStatePathWithSeparator joins state path segments with the requested separator.
func qualifiedStatePathWithSeparator(segs []string, separator string) string {
	parts := make([]string, len(segs))
	for i, seg := range segs {
		parts[i] = writeName(seg)
	}
	return strings.Join(parts, separator)
}

// endpoint names an end of a transition that is a state or a pseudostate
// written as a member, noting on t when it lies outside the region.
func (s *stateRegion) endpoint(t, v *sysmlv1.Element, role string) (string, bool) {
	p, ok := s.path(v)
	if !ok {
		return "", false
	}
	if owner := memberOwner(v); owner != s.r && owner != s.machine {
		s.m.add(t, Mapped, "", "the "+role+" "+describe(v)+" lies in another region and is named by its path "+p)
	}
	return p, true
}

// target names what a transition leads to: a state or pseudostate of the machine
// by its path, done for a final or terminate state of the region, or the entry
// state of a submachine state a connection point reference enters.
func (s *stateRegion) target(t, v *sysmlv1.Element) (string, bool) {
	if v == nil {
		return "", false
	}
	switch v.Type {
	case "FinalState":
		if v.Parent != s.r {
			s.m.add(t, Unmapped, "", "the final state "+describe(v)+" ends another region than the transition's, which done would not")
			return "", false
		}
		return "done", true
	case "State":
		return s.endpoint(t, v, "target")
	case "Pseudostate":
		switch pseudoKind(v) {
		case "terminate":
			return "done", true
		case "exitPoint":
			if s.m.points[v].why != "" {
				return "", false
			}
			if p, ok := s.endpoint(t, v, "target"); ok {
				return p, true
			}
			if pointOwner(v).Type == "State" {
				return "", false
			}
			s.m.add(t, Approximated, "", "the exit point "+describe(v)+" belongs to another machine; the transition is written to done")
			return "done", true
		case "entryPoint":
			f := s.m.points[v]
			if f.why != "" {
				return "", false
			}
			if f.defaultEntry {
				p, ok := s.endpoint(t, pointOwner(v), "target")
				if ok {
					s.m.add(t, Mapped, "", "written to "+p+": no transition leaves the entry point "+describe(v)+", so entering through it enters the state by its default entry")
				}
				return p, ok
			}
			return s.endpoint(t, v, "target")
		case "choice", "junction", "shallowHistory", "deepHistory":
			return s.endpoint(t, v, "target")
		case "fork", "join":
			return s.endpoint(t, v, "target")
		}
	case "ConnectionPointReference":
		return s.connection(t, v, "entry")
	}
	return "", false
}

// source names the state or pseudostate a transition leaves, or the exit state
// of a submachine state a connection point reference leaves through.
func (s *stateRegion) source(t, v *sysmlv1.Element) (string, bool) {
	if v == nil {
		return "", false
	}
	switch v.Type {
	case "State":
		return s.endpoint(t, v, "source")
	case "Pseudostate":
		switch pseudoKind(v) {
		case "entryPoint", "exitPoint":
			if s.m.points[v].why != "" {
				return "", false
			}
			return s.endpoint(t, v, "source")
		case "choice", "junction", "shallowHistory", "deepHistory":
			return s.endpoint(t, v, "source")
		case "fork", "join":
			return s.endpoint(t, v, "source")
		}
	case "ConnectionPointReference":
		return s.connection(t, v, "exit")
	}
	return "", false
}

// noForm says why vertex v is no end for a transition: its connection point's refusal,
// else that it lies outside the machine or has no v2 form.
func (s *stateRegion) noForm(v *sysmlv1.Element) string {
	if why := s.m.points[v].why; why != "" {
		return " has no v2 form: " + why
	}
	return isA + kindOf(v) + outsideMachine
}

// transient reports a pseudostate a compound transition passes through in one
// step, whose outgoing transitions the runtime follows on reaching it.
func transient(v *sysmlv1.Element) bool {
	switch pseudoKind(v) {
	case "junction", "choice", "entryPoint", "exitPoint":
		return true
	}
	return false
}

// connection names the state of a submachine's state def a connection point
// reference stands for, through the submachine state: `sub.point`. The
// reference is reported once, where the first transition passes through it.
func (s *stateRegion) connection(t, v *sysmlv1.Element, role string) (string, bool) {
	st := v.Parent
	if st == nil || st.Type != "State" {
		return "", false
	}
	sub := s.m.model.Ref(st, "submachine")
	if sub == nil || !s.m.written(sub) {
		return "", false
	}
	points := s.m.model.Refs(v, role)
	if len(points) == 0 {
		other := "exit"
		if role == "exit" {
			other = "entry"
		}
		if len(s.m.model.Refs(v, other)) > 0 {
			s.m.add(t, Unmapped, "", "the transition passes through "+describe(v)+" as an "+role+", but the reference names only "+other+" points")
		} else {
			s.m.add(t, Unmapped, "", joinNotes(s.m.dangling(v, role), "the connection point reference "+describe(v)+" names no "+role+" point"))
		}
		return "", false
	}
	for _, p := range points[1:] {
		s.m.add(v, Approximated, "", "the reference names several "+role+" points; only "+describe(points[0])+" is passed through, not "+describe(p))
	}
	point := points[0]
	s.m.nameMachine(sub)
	pname, ok := s.m.vertexNames[point]
	if !ok || machineOf(point) != sub {
		s.m.add(t, Unmapped, "", "the "+role+" point "+describe(point)+" of "+describe(v)+" is not a connection point of the submachine "+qualifiedName(sub))
		return "", false
	}
	base, ok := s.endpoint(t, st, role)
	if !ok {
		return "", false
	}
	target := base + "." + writeName(pname)
	s.m.add(v, Mapped, target, "written as the "+role+" point's state in the submachine state, "+target)
	return target, true
}

// transition writes a transition: one per trigger, since a v2 transition
// accepts one, sharing the guard and effect.
func (s *stateRegion) transition(t *sysmlv1.Element) {
	if host, relocated := s.m.relocatedTo[t]; relocated && s.owner != host {
		return
	}
	src, tgt, internal, ok := s.transitionEnds(t)
	if !ok {
		return
	}
	if host := s.relocationHost(src, tgt); host != nil {
		s.m.relocated[host] = append(s.m.relocated[host], t)
		s.m.relocatedTo[t] = host
		return
	}
	if transient(src) && (pseudoKind(tgt) == "shallowHistory" || pseudoKind(tgt) == "deepHistory") {
		s.m.unmapped(t, "the runtime does not follow a transition from a "+pseudoKind(src)+" pseudostate on into the history pseudostate "+describe(tgt))
		return
	}
	from, ok := s.source(t, src)
	if !ok {
		why := "the source " + describe(src) + s.noForm(src)
		s.m.unmapped(t, why)
		if s.m.strict {
			s.refusedParts(t, why)
		}
		return
	}
	to := from
	if !internal {
		if to, ok = s.target(t, tgt); !ok {
			why := "the target " + describe(tgt) + s.noForm(tgt)
			s.m.unmapped(t, why)
			if s.m.strict {
				s.refusedParts(t, why)
			}
			return
		}
	}
	triggers := t.Owned("trigger")
	notes, done := s.kindNotes(t, src, internal, len(triggers), from)
	if done {
		return
	}
	guard, gnote := s.guard(t, src)
	eff := s.m.behaviorIn(t, "effect")
	if gnote != "" {
		notes = append(notes, gnote)
	}
	accepts, notes, info, written := s.transitionAccepts(t, triggers, eff, tgt, notes)
	if len(triggers) > 0 && len(accepts) == 0 {
		s.m.w.lines(commentLines(transitionKeyword + describe(t) + " from " + from + " to " + to + " not migrated — " + strings.Join(notes, "; ")))
		s.m.add(t, Unmapped, "", "every trigger is dropped, so the transition would fire at once: "+strings.Join(notes, "; "))
		return
	}
	if len(accepts) == 0 {
		accepts = []acceptance{{}}
		if eff != nil {
			s.m.unbound(eff, "the transition accepts no signal")
		}
	} else if written > 1 {
		notes = append(notes, "written as "+strconv.Itoa(written)+" transitions, one per trigger")
	}
	tname := s.m.nameOf(t)
	if tname == "" {
		tname = s.m.edgeName(t, transitionBase(from, accepts[0], guard, to))
	}
	if tname != "" {
		tname = s.m.freshMember(s.owner, s.used, tname)
	}
	s.writeAccepts(t, accepts, tname, guard, eff, from, to)
	note := strings.Join(notes, "; ")
	s.m.add(t, verdictFor(note), s.m.edgeTarget(t), joinNotes(note, strings.Join(info, "; ")))
}

// relocationHost returns the common state scope where a transition's endpoints can be named.
func (s *stateRegion) relocationHost(src, tgt *sysmlv1.Element) *sysmlv1.Element {
	if memberOwner(src) == s.r && memberOwner(tgt) == s.r {
		return nil
	}
	host := commonStateScope(src, tgt)
	if host == nil || host == s.owner {
		return nil
	}
	return host
}

// commonStateScope returns the nearest state or machine scope shared by both vertices.
func commonStateScope(src, tgt *sysmlv1.Element) *sysmlv1.Element {
	scope := func(vertex *sysmlv1.Element) *sysmlv1.Element {
		for cur := memberOwner(vertex); cur != nil; cur = cur.Parent {
			if cur.Type == "State" || cur.Type == "StateMachine" {
				return cur
			}
		}
		return nil
	}
	left, right := scope(src), scope(tgt)
	if left == nil || right == nil || machineOf(left) != machineOf(right) {
		return nil
	}
	ancestors := map[*sysmlv1.Element]bool{}
	for cur := left; cur != nil; cur = cur.Parent {
		if cur.Type == "State" || cur.Type == "StateMachine" {
			ancestors[cur] = true
		}
	}
	for cur := right; cur != nil; cur = cur.Parent {
		if (cur.Type == "State" || cur.Type == "StateMachine") && ancestors[cur] {
			return cur
		}
	}
	return nil
}

// writeRelocatedTransitions writes transitions deferred to their common state scope.
func (m *migration) writeRelocatedTransitions(host *sysmlv1.Element) {
	transitions := m.relocated[host]
	if len(transitions) == 0 {
		return
	}
	delete(m.relocated, host)
	regions := m.populatedRegions(host)
	if len(regions) == 0 {
		return
	}
	s := m.region(regions[0])
	s.owner = host
	s.ownerScope = true
	if host.Type == "State" {
		s.used = m.stateUsed[host]
	}
	for _, transition := range transitions {
		s.transition(transition)
	}
}

// refusedParts accounts for the children of a transition a strict migration
// refuses because an endpoint is an extension pseudostate: its triggers and
// the events they name, its guard and its effect go unmapped with it, so the
// report loses none of them.
func (s *stateRegion) refusedParts(t *sysmlv1.Element, why string) {
	note := "its transition is not written: " + why
	for _, tr := range t.Owned("trigger") {
		s.m.add(tr, Unmapped, "", note)
		if ev := s.m.model.Ref(tr, "event"); ev != nil {
			s.m.triggered[ev] = true
			if !s.m.reported(ev) {
				s.m.add(ev, Unmapped, "", note)
			}
		}
	}
	if g := s.m.guardOf(t); g != nil {
		gnote := note
		if spec := firstOwned(g, "specification"); spec != nil {
			gnote += "; the guard [" + describeValue(spec) + "] is dropped with it"
		}
		s.m.add(g, Unmapped, "", gnote)
	}
	if eff := s.m.behaviorIn(t, "effect"); eff != nil && !s.m.reported(eff) {
		s.m.add(eff, Unmapped, "", note)
	}
}

// transitionBase spells the name a shown anonymous transition is declared
// under, from what it is written between: `Wait accept Sig then Retrieve`.
func transitionBase(from string, a acceptance, guard, to string) string {
	clause := a.clause
	if a.payload != "" {
		clause = strings.Replace(clause, writeName(a.payload)+" : ", "", 1)
	}
	return spoken(from + clause + guard + " then " + to)
}

// kindNotes notes how an internal or local transition's semantics change in
// v2; done reports an internal transition with no trigger, which is unmapped.
func (s *stateRegion) kindNotes(t, src *sysmlv1.Element, internal bool, triggers int, from string) (notes []string, done bool) {
	switch {
	case internal:
		if triggers == 0 {
			s.m.unmapped(t, "an internal transition without a trigger has no v2 form: a self transition would fire again on every re-entry")
			return nil, true
		}
		if !s.m.reentryObservable(src) {
			s.m.add(t, Mapped, "", "an internal transition is written as a self transition; "+from+" has no entry, exit or do behavior and no substates, so re-entering it is not observable")
			return nil, false
		}
		return []string{"an internal transition is written as a self transition, which exits and re-enters " + from + " where v1 stayed in it, running its exit and entry behaviors"}, false
	case t.Attrs["kind"] == "local":
		return []string{"a local transition is written external: the composite state " + from + " exits and re-enters where v1 stayed in it, running its exit and entry behaviors"}, false
	}
	return nil, false
}

// transitionEnds resolves a transition's source and target, checking the ends
// exist, an initial source is skipped, and an internal one stays in its source.
// False means the transition is done or unmapped.
func (s *stateRegion) transitionEnds(t *sysmlv1.Element) (src, tgt *sysmlv1.Element, internal, ok bool) {
	src, tgt = s.m.model.Ref(t, "source"), s.m.model.Ref(t, "target")
	internal = t.Attrs["kind"] == "internal"
	if internal && tgt == nil && len(s.m.model.Unresolved(t, "target")) == 0 {
		// An internal transition stays in its source; some tools write it with no target.
		tgt = src
	}
	if src == nil || tgt == nil {
		s.m.unmapped(t, joinNotes(s.m.dangling(t, "source", "target"), "the transition lacks an end"))
		return nil, nil, internal, false
	}
	if pseudoKind(src) == "initial" {
		// Written as the region's entry.
		return nil, nil, internal, false
	}
	if internal {
		if src.Type != "State" {
			s.m.unmapped(t, "the source "+describe(src)+isA+kindOf(src)+", and only a state has an internal transition")
			return nil, nil, internal, false
		}
		if tgt != src {
			s.m.unmapped(t, "an internal transition targets "+describe(tgt)+", not its source "+describe(src)+"; whether it stays or moves cannot be told")
			return nil, nil, internal, false
		}
	}
	return src, tgt, internal, true
}

// transitionAccepts writes one acceptance per usable trigger, noting the ones
// dropped; returns the acceptances, updated notes and infos, and how many were written.
func (s *stateRegion) transitionAccepts(t *sysmlv1.Element, triggers []*sysmlv1.Element, eff, tgt *sysmlv1.Element, notes []string) ([]acceptance, []string, []string, int) {
	var accepts []acceptance
	var info []string
	written := 0
	for _, tr := range triggers {
		a, note, ok := s.triggerAccept(t, tr, eff, tgt)
		if !ok {
			notes = append(notes, note)
			continue
		}
		written++
		routes, rinfo, rnote := s.routes(tr, a)
		if rinfo != "" {
			info = append(info, rinfo)
		}
		note = joinNotes(note, rnote)
		s.m.add(tr, verdictFor(note), "", joinNotes(note, rinfo))
		accepts = append(accepts, routes...)
	}
	return accepts, notes, info, written
}

// writeAccepts writes one transition line per acceptance, each with the guard, an
// effect body when there is one, and the target, recording each as a member of t.
func (s *stateRegion) writeAccepts(t *sysmlv1.Element, accepts []acceptance, tname, guard string, eff *sysmlv1.Element, from, to string) {
	s.m.wroteEdge(t, s.r, "transition", tname)
	for i, accept := range accepts {
		line := transitionKeyword
		if tname != "" {
			n := tname
			if i > 0 {
				n = s.m.freshMember(s.owner, s.used, s.furtherName(t, tname, accept, guard, from, to))
				s.m.wroteEdgeAlso(t, s.r, "transition", nil, n)
			}
			s.m.madeUp(t, writeName(n))
			line += writeName(n) + " "
		}
		line += "first " + from + accept.clause + guard
		if eff != nil || accept.keeping != "" {
			s.writeTransitionEffect(t, eff, accept, line, to, i)
			continue
		}
		s.m.w.line(line + " then " + to + ";")
	}
}

// furtherName is the base name of a further transition written for t: the v1
// name when t has one, else spelled from the acceptance the line is written for.
func (s *stateRegion) furtherName(t *sysmlv1.Element, tname string, a acceptance, guard, from, to string) string {
	if s.m.nameOf(t) != "" {
		return tname
	}
	return transitionBase(from, a, guard, to)
}

// routes writes the acceptances a trigger stands for: as read when taken from the object itself,
// and via each port the trigger names or the signal arrives at. info says what was added, note what was dropped.
func (s *stateRegion) routes(tr *sysmlv1.Element, a acceptance) (routes []acceptance, info, note string) {
	ev := s.m.model.Ref(tr, "event")
	sig := s.m.model.Ref(ev, "signal")
	if ev.Type != "SignalEvent" || sig == nil {
		return []acceptance{a}, "", ""
	}
	ports, direct, info, note := s.m.portRoutes(tr, classifierOf(tr), sig)
	if direct {
		routes = append(routes, a)
	}
	for _, p := range ports {
		via := a
		via.clause = a.clause + " via " + s.m.ownerPrefix(tr) + writeName(s.m.nameFor(p))
		routes = append(routes, via)
	}
	return routes, info, note
}

// reentryObservable reports whether leaving and re-entering state v runs a
// behavior or resets a substate, which is what a self transition adds to an
// internal one.
func (m *migration) reentryObservable(v *sysmlv1.Element) bool {
	if v.Type != "State" {
		return true
	}
	if m.behaviorIn(v, "entry") != nil || m.behaviorIn(v, "exit") != nil || m.behaviorIn(v, "doActivity") != nil {
		return true
	}
	return len(v.Owned("region")) > 0 || m.model.Ref(v, "submachine") != nil || len(v.Owned("deferrableTrigger")) > 0
}

// behaviorIn gives the behavior a state or transition runs in a role, whether
// it owns it or refers to one owned elsewhere.
func (m *migration) behaviorIn(v *sysmlv1.Element, role string) *sysmlv1.Element {
	if b := firstOwned(v, role); b != nil {
		return b
	}
	return m.model.Ref(v, role)
}

// triggerAccept resolves one trigger into the acceptance written for it, keeping
// the signal for a target that carries it; ok is false when the trigger is
// dropped, note saying why, and on success note is what triggerClause noted.
func (s *stateRegion) triggerAccept(t, tr, eff, tgt *sysmlv1.Element) (a acceptance, note string, ok bool) {
	ev := s.m.model.Ref(tr, "event")
	if ev == nil {
		note := joinNotes(s.m.dangling(tr, "event"), "the trigger names no event")
		s.m.add(tr, Unmapped, "", note)
		return a, "a trigger is dropped: " + note, false
	}
	if eff != nil {
		a = s.payload(t, eff, ev)
	}
	if c := s.m.carrierOf[tgt]; c != nil && ev.Type == "SignalEvent" && s.m.model.Ref(ev, "signal") == c.sig {
		if a.payload == "" {
			a.payload = s.payloadName(eff, c.sig)
		}
		a.keeping = s.m.keep(tgt, c.sig, a.payload)
	}
	clause, tnote, tok := s.m.triggerClause(ev, t, a.payload)
	if !tok {
		s.m.add(tr, Unmapped, "", tnote)
		return a, "a trigger is dropped: " + tnote, false
	}
	a.clause = " " + clause
	return a, tnote, true
}

// writeTransitionEffect writes a transition's effect, or the statement keeping
// its signal, as its do action, then its target.
func (s *stateRegion) writeTransitionEffect(t, eff *sysmlv1.Element, accept acceptance, line, to string, i int) {
	if i > 0 && eff != nil {
		s.m.downgrade(eff, "run by each of the transitions written for it")
	}
	var notes []string
	effect := s.m.w.capture(func() {
		if eff == nil {
			s.m.w.braced(doAction, func() { s.m.w.line(accept.keeping) })
			return
		}
		saved, savedKeep, savedAsides := s.m.bound, s.m.keeping, s.m.asides
		s.m.bound, s.m.keeping, s.m.asides = accept.bound, accept.keeping, &notes
		s.m.inlineBehavior(doAction, eff, t)
		s.m.bound, s.m.keeping, s.m.asides = saved, savedKeep, savedAsides
	})
	s.m.w.lines(notes)
	s.m.w.line(line)
	s.m.w.indented(func() {
		_, _ = s.m.w.buf().WriteString(effect) // already rendered one level deeper by capture
		s.m.w.line("then " + to + ";")
	})
}

// acceptance is one written trigger: its accept clause, the name the clause
// gives the accepted signal, the effect parameters bound to that name, and the
// statement keeping the signal for the state entered.
type acceptance struct {
	clause  string
	payload string
	bound   map[*sysmlv1.Element]string
	keeping string
}

// payload binds the effect's parameters to the signal a trigger accepts, as a
// transition passes the signal instance to its effect: to each parameter typed
// by the signal or a general of it, or to the sole untyped in parameter.
func (s *stateRegion) payload(t, eff, ev *sysmlv1.Element) acceptance {
	params := inParameters(eff)
	if ev.Type != "SignalEvent" {
		s.m.unbound(eff, "the transition accepts a "+ev.Type+", which carries no signal")
		return acceptance{}
	}
	sig := s.m.model.Ref(ev, "signal")
	if sig == nil || len(params) == 0 {
		return acceptance{}
	}
	if eff.Parent != t {
		s.m.unbound(eff, "the effect is written once, as its own action def, so only a transition owning it can pass the accepted "+s.m.nameFor(sig))
		return acceptance{}
	}
	a := acceptance{payload: s.payloadName(eff, sig), bound: map[*sysmlv1.Element]string{}}
	for _, p := range params {
		typ := s.m.model.Ref(p, "type")
		switch {
		case typ == sig || typ != nil && s.m.inherits(sig, typ), typ == nil && len(params) == 1:
			a.bound[p] = writeName(a.payload)
		case typ == nil:
			s.m.unvalued[p] = true
			s.m.add(p, Approximated, "", "the parameter takes no value: it is untyped, and the transition passes only the accepted "+s.m.nameFor(sig))
		default:
			s.m.unvalued[p] = true
			s.m.add(p, Approximated, "", "the parameter takes no value: the transition passes only the accepted "+s.m.nameFor(sig)+", which is no "+s.m.nameFor(typ))
		}
	}
	return a
}

// payloadName names the signal an accept clause binds, clear of the effect's
// parameters.
func (s *stateRegion) payloadName(eff, sig *sysmlv1.Element) string {
	used := map[string]bool{}
	for _, c := range s.m.carrierOf {
		used[c.holder] = true
	}
	if eff != nil {
		for _, p := range eff.Owned("ownedParameter") {
			used[s.m.nameFor(p)] = true
		}
	}
	return freshIn(used, lowerFirst(s.m.nameFor(sig)))
}

// unbound notes on each in parameter of an effect why it takes no value.
func (m *migration) unbound(eff *sysmlv1.Element, why string) {
	for _, p := range inParameters(eff) {
		m.unvalued[p] = true
		m.add(p, Approximated, "", "the parameter takes no value: "+why)
	}
}

// inParameters lists the parameters of a behavior that take a value.
func inParameters(b *sysmlv1.Element) []*sysmlv1.Element {
	var params []*sysmlv1.Element
	for _, p := range b.Owned("ownedParameter") {
		switch p.Attrs["direction"] {
		case "out", "return":
		default:
			params = append(params, p)
		}
	}
	return params
}

// guard writes a transition's guard as ` if <expr>`, or keeps its text in a
// comment when it is not a v2 expression the state machine's owner resolves;
// an else guard out of a choice or junction is the unguarded transition.
func (s *stateRegion) guard(t, src *sysmlv1.Element) (string, string) {
	g := s.m.guardOf(t)
	if g == nil {
		return "", ""
	}
	spec := firstOwned(g, "specification")
	if spec == nil {
		s.m.add(g, Unmapped, "", "the guard has no specification")
		return "", "the guard " + describe(g) + " has no specification and is dropped"
	}
	if trueLiteral(spec) {
		s.m.add(g, Mapped, "", "a true guard is not written")
		return "", ""
	}
	if k := pseudoKind(src); (k == "choice" || k == "junction") && isElseGuard(spec) {
		s.m.add(g, Mapped, "", "an else guard is written as the unguarded transition out of the "+k+", which is taken when no guarded one holds")
		return "", ""
	}
	expr, ok, note := s.m.behaviorValue(spec, t)
	if !ok {
		s.m.w.lines(commentLines("guard not migrated: [" + describeValue(spec) + "] — " + note))
		s.m.add(g, Approximated, "", "the guard is kept as a comment and the transition written unguarded: "+note)
		return "", "the guard [" + describeValue(spec) + "] is kept as a comment and the transition written unguarded: " + note
	}
	s.m.add(g, verdictFor(note), "", note)
	return " if " + expr, note
}

// isElseGuard reports whether a guard's specification is the word else, which
// a v1 tool writes on the branch a choice takes when no other guard holds.
func isElseGuard(spec *sysmlv1.Element) bool {
	var text string
	switch spec.Type {
	case "OpaqueExpression":
		text, _ = opaqueBody(spec)
	case "LiteralString":
		text = spec.Attrs["value"]
	default:
		return false
	}
	return strings.EqualFold(strings.TrimSpace(text), "else")
}
