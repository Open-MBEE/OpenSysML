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

// stateMachineBody writes a state machine's regions as the body of its state def.

// The note fragments the writer repeats.
const (
	doAction    = "do action"
	exitAction  = "exit action"
	notRun      = " is not run: "
	performsIts = "a state performs its "
)

func (m *migration) stateMachineBody(sm *sysmlv1.Element) {
	m.parameters(sm, sm)
	for _, c := range sm.Children {
		switch c.Role {
		case "ownedBehavior", "nestedClassifier", "ownedAttribute", "ownedOperation", "ownedRule":
			m.member(c)
		}
	}
	used := m.nameMachine(sm)
	m.instants(sm, used)
	m.carriers(sm, used)
	for _, cp := range sm.Owned("connectionPoint") {
		m.connectionPoint(cp)
	}
	m.regions(sm, m.populatedRegions(sm), false, func() { /* no extra nesting to write */ })
}

// nameMachine names every vertex of a machine down through its nested regions ahead of writing,
// so a transition can target another region or a submachine; it returns the names in the state def's body.
func (m *migration) nameMachine(sm *sysmlv1.Element) map[string]bool {
	if used, ok := m.regionUsed[sm]; ok {
		return used
	}
	used := inheritedStateNamesSet()
	m.regionUsed[sm] = used
	m.indexTransitions(sm)
	for _, cp := range sm.Owned("connectionPoint") {
		m.nameVertex(cp, sm, used)
	}
	m.nameRegions(m.populatedRegions(sm), sm, used)
	return used
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
				rname = "region"
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
			inner := inheritedStateNamesSet()
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
		return pointForm{defaultEntry: true, note: note}
	}
	for _, t := range out {
		if why := m.entryBranchWhy(t, owner); why != "" {
			return pointForm{why: describe(t) + " leads from the entry point " + why}
		}
	}
	regions := m.regionsCrossed(out, owner, "target")
	if len(out) < 2 || len(regions) < 2 {
		return pointForm{kw: "junction", note: "written as a junction of its state; a transition entering through it runs the state's entry behavior, then the transition leaving the junction"}
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
		return pointForm{kw: "junction", note: "written as a junction of its state; a transition leaving through it runs the transition into the junction, the state's exit behavior, then the transition leaving it"}
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
		name = m.freshMember(owner, used, base)
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
		if without := regionsWithoutInitial(regions); len(without) == 0 {
			m.w.line(entryThen(entered, writeName(name)))
		} else {
			m.w.lines(commentLines("no default entry: the " + pluralRegion(len(without)) + " " + strings.Join(without, ", ") +
				" have no initial pseudostate, so only a fork or a transition naming a nested state enters the regions"))
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
				m.add(r, Mapped, rname, "an orthogonal region is written as a sub-state of the parallel state "+name)
			}
		})
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

func pluralRegion(n int) string {
	if n == 1 {
		return "region"
	}
	return "regions"
}

// region prepares to write region r of the machine that nameMachine named.
func (m *migration) region(r *sysmlv1.Element) *stateRegion {
	return &stateRegion{m: m, r: r, used: m.regionUsed[r], machine: machineOf(r)}
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
// empty one unless entered says the body wrote its own.
func entryThen(entered bool, to string) string {
	if entered {
		return "then " + to + ";"
	}
	return "entry; then " + to + ";"
}

// stateRegion writes one region: its entry, then its states and transitions.
type stateRegion struct {
	m       *migration
	r       *sysmlv1.Element
	used    map[string]bool
	machine *sysmlv1.Element
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
	to, ok := s.target(t, tgt)
	if !ok {
		s.m.unmapped(init, "the initial transition's target has no v2 form here")
		if tgt == nil {
			s.m.unmapped(t, joinNotes(s.m.dangling(t, "target"), "the transition lacks a target"))
		} else {
			s.m.unmapped(t, "the target "+describe(tgt)+isA+kindOf(tgt)+outsideRegion)
		}
		return
	}
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
		s.m.w.line(entryThen(entered, to))
	case entered:
		s.m.w.lines(commentLines("the effect " + describe(eff) + " of the initial transition is dropped: the state's entry behavior is its entry action"))
		s.m.unmapped(eff, "the effect of an initial transition is the entry action of the body, which the state's entry behavior is")
		s.m.w.line(entryThen(true, to))
	default:
		s.m.unbound(eff, "an initial transition accepts no signal")
		s.m.w.line(entryThen(s.m.inlineBehavior("entry action", eff, t), to))
	}
	s.m.add(init, Mapped, "", "written as the entry of the region")
	s.m.add(t, verdictFor(note), "", note)
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
			s.m.w.line(pseudoKind(v) + " " + name + ";")
			s.m.add(v, Mapped, name, "written as a "+pseudoKind(v)+" pseudostate, whose guarded transitions the runtime reads when it is reached; an unguarded one is its else branch")
		case "fork", "join":
			name := writeName(s.name(v))
			s.m.w.line(pseudoKind(v) + " " + name + ";")
			s.m.add(v, Mapped, name, "written as a "+pseudoKind(v)+" pseudostate, whose segments the runtime fires together")
		case "shallowHistory":
			name := writeName(s.name(v))
			s.m.w.line("history " + name + ";")
			s.m.add(v, Mapped, name, "written as a shallow history, which re-enters the substate active when its state was last left")
		case "deepHistory":
			name := writeName(s.name(v))
			s.m.w.line("deep history " + name + ";")
			s.m.add(v, Mapped, name, "written as a deep history, which re-enters the innermost states active when its state was last left")
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
			m.w.line(f.kw + " " + name + ";")
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
	defers := s.deferrals(v)
	head := stateKw + name
	if sub := s.m.model.Ref(v, "submachine"); sub != nil {
		if s.m.written(sub) {
			head += " : " + s.m.ref(sub, s.r)
			s.m.add(v, Mapped, name, "")
		} else {
			for _, c := range v.Owned("connection") {
				s.m.unmapped(c, "the submachine "+qualifiedName(sub)+" has no v2 declaration, so its connection points are not written")
			}
			s.m.add(v, Approximated, name, "its submachine "+qualifiedName(sub)+" has no v2 declaration; the state is written simple")
		}
	} else {
		for _, c := range v.Owned("connection") {
			s.m.unmapped(c, "the state has no submachine whose connection point the reference could name")
		}
		s.m.add(v, Mapped, name, "")
	}
	regions := s.m.populatedRegions(v)
	entry, do, exit := s.m.behaviorIn(v, "entry"), s.m.behaviorIn(v, "doActivity"), s.m.behaviorIn(v, "exit")
	inv := firstOwned(v, "stateInvariant")
	points := s.m.connectionPoints(v)
	pointRegs := pointRegions(v)
	if entry == nil && do == nil && exit == nil && inv == nil && len(regions) == 0 && len(defers) == 0 && s.m.writtenPoints(v) == 0 && !hasTransitions(pointRegs) {
		s.m.w.line(head + ";")
		s.m.statePoints(v)
		return
	}
	s.m.w.block(head, func() {
		s.m.writeComments(v, false)
		s.m.w.lines(defers)
		if inv != nil {
			s.m.invariant(inv)
		}
		entered := entry != nil && s.m.inlineBehavior("entry action", entry, v)
		between := func() {
			if do != nil {
				s.m.inlineBehavior(doAction, do, v)
			}
			if exit != nil {
				s.m.inlineBehavior(exitAction, exit, v)
			}
			if len(points) > 0 {
				s.m.statePoints(v)
			}
		}
		if len(regions) == 0 {
			between()
		} else {
			s.m.regions(v, regions, entered, between)
		}
		for _, pr := range pointRegs {
			// Named from the state's one region when it has one, so its vertices need no path.
			host := pr
			if len(regions) == 1 {
				host = regions[0]
			}
			for _, t := range pr.Owned("transition") {
				s.m.region(host).transition(t)
			}
		}
	})
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

// deferrals writes a state's deferrable triggers as `defer Sig;` lines: v2
// defers the signal a transition would accept, so no other event kind can be.
func (s *stateRegion) deferrals(v *sysmlv1.Element) []string {
	var lines []string
	for _, d := range v.Owned("deferrableTrigger") {
		ev := s.m.model.Ref(d, "event")
		if ev == nil {
			s.m.add(d, Unmapped, "", joinNotes(s.m.dangling(d, "event"), "the deferred trigger names no event"))
			continue
		}
		if ev.Type != "SignalEvent" {
			note := "only a signal event can be deferred, not a " + ev.Type
			s.m.add(d, Unmapped, "", note)
			continue
		}
		sig := s.m.model.Ref(ev, "signal")
		if note, ok := s.m.signalOf(ev); !ok {
			s.m.add(d, Unmapped, "", note)
			s.m.add(ev, Unmapped, "", note)
			continue
		}
		clause := "defer " + s.m.ref(sig, v)
		lines = append(lines, clause+";")
		note := "written as " + clause + ", an OpenSysML extension of the notation that the runtime executes"
		s.m.add(d, Approximated, "", note)
		s.m.add(ev, Approximated, "", "written where a trigger refers to it, as "+clause+", an OpenSysML extension of the notation")
	}
	return lines
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

// inlineBehavior writes a behavior a state or transition owns as the action
// kw of the current body, and reports whether anything was written.
func (m *migration) inlineBehavior(kw string, b, owner *sysmlv1.Element) bool {
	if b.Parent != owner {
		return m.referencedBehavior(kw, b, owner)
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
	header := kw
	if name := m.nameOf(b); name != "" {
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
	m.w.lines(commentLines(kw + " " + describe(b) + isA + b.Type + ", which has no action form"))
	m.add(b, Unmapped, "", "a "+b.Type+" has no action form")
	return false
}

// referencedBehavior writes `kw : ref` for a behavior written elsewhere, binding
// its context and signal-carried parameters where it can and reporting why not.
func (m *migration) referencedBehavior(kw string, b, owner *sysmlv1.Element) bool {
	if !m.written(b) {
		m.w.lines(commentLines(kw + " " + qualifiedName(b) + " has no v2 declaration"))
		m.add(b, Unmapped, "", "the behavior is not written; "+describe(owner)+" names it as its "+kw)
		m.add(owner, Approximated, "", "its "+kw+" "+qualifiedName(b)+notRun+"it has no v2 declaration")
		return false
	}
	if cat, _ := m.classify(b); cat != catActionDef {
		m.w.lines(commentLines(kw + " " + qualifiedName(b) + " is written as a " + cat.keyword() + ", which no state runs"))
		m.downgrade(b, describe(owner)+" names it as its "+kw+", which a "+cat.keyword()+" cannot be")
		m.add(owner, Approximated, "", "its "+kw+" "+qualifiedName(b)+notRun+"it is written as a "+cat.keyword())
		return false
	}
	var ins []string
	note := "also run as the " + kw + " of " + describe(owner)
	if c := m.contextOf(b); c != nil {
		expr, cnote := m.contextBinding(c, classifierOf(owner), "this")
		if expr == "" {
			m.w.lines(commentLines(kw + " " + qualifiedName(b) + notRun + cnote))
			m.downgrade(b, "not run as the "+kw+" of "+describe(owner)+": "+cnote)
			m.add(owner, Approximated, "", "its "+kw+" "+qualifiedName(b)+notRun+cnote)
			return false
		}
		ins = append(ins, "in "+writeName(c.name)+" = "+expr)
		note = joinNotes(note, cnote)
	}
	if params := inParameters(b); len(params) > 0 && owner.Type != "Transition" {
		var ok bool
		ins, note, ok = m.parameterIns(kw, b, owner, params, ins, note)
		if !ok {
			return false
		}
	}
	line := kw + " : " + m.ref(b, owner)
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
			ins = append(ins, m.parameterBinding(p, m.nameFor(p), bound[p]))
		}
		return ins, joinNotes(note, "its parameters take the attributes of the signal the transitions into the state accept"), true
	case slices.IndexFunc(params, requiresValue) >= 0:
		p := params[slices.IndexFunc(params, requiresValue)]
		why = "its parameter " + m.nameFor(p) + " must hold a value that nothing supplies: " + why
		m.w.lines(commentLines(kw + " " + qualifiedName(b) + notRun + why))
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
	if owner := memberOwner(v); owner == s.r || owner == s.machine {
		return writeName(name), true
	}
	segs := s.m.segments(v)
	return s.m.qualified(segs[len(s.m.segments(s.machine)):]), true
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
		case "choice", "junction", "fork", "join", "shallowHistory", "deepHistory":
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
		case "choice", "junction", "fork", "join", "shallowHistory", "deepHistory":
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
// reference stands for, through the submachine state: `sub::point`. The
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
	target := base + "::" + writeName(pname)
	s.m.add(v, Mapped, target, "written as the "+role+" point's state in the submachine state, "+target)
	return target, true
}

// transition writes a transition: one per trigger, since a v2 transition
// accepts one, sharing the guard and effect.
func (s *stateRegion) transition(t *sysmlv1.Element) {
	src, tgt, internal, ok := s.transitionEnds(t)
	if !ok {
		return
	}
	if transient(src) && (pseudoKind(tgt) == "shallowHistory" || pseudoKind(tgt) == "deepHistory") {
		s.m.unmapped(t, "the runtime does not follow a transition from a "+pseudoKind(src)+" pseudostate on into the history pseudostate "+describe(tgt))
		return
	}
	from, ok := s.source(t, src)
	if !ok {
		s.m.unmapped(t, "the source "+describe(src)+s.noForm(src))
		return
	}
	to := from
	if !internal {
		if to, ok = s.target(t, tgt); !ok {
			s.m.unmapped(t, "the target "+describe(tgt)+s.noForm(tgt))
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
		s.m.w.lines(commentLines("transition " + describe(t) + " from " + from + " to " + to + " not migrated — " + strings.Join(notes, "; ")))
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
	tname := ""
	if s.m.nameOf(t) != "" {
		tname = freshIn(s.used, s.m.nameOf(t))
	}
	s.writeAccepts(t, accepts, tname, guard, eff, from, to)
	note := strings.Join(notes, "; ")
	s.m.add(t, verdictFor(note), tname, joinNotes(note, strings.Join(info, "; ")))
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

// writeAccepts writes one transition line per acceptance, each with the guard,
// an effect body when there is one, and the target.
func (s *stateRegion) writeAccepts(t *sysmlv1.Element, accepts []acceptance, tname, guard string, eff *sysmlv1.Element, from, to string) {
	for i, accept := range accepts {
		line := "transition "
		if tname != "" {
			n := tname
			if i > 0 {
				n = freshIn(s.used, tname)
			}
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
		via.clause = a.clause + " via " + writeName(s.m.nameFor(p))
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
	s.m.w.line(line)
	s.m.w.indented(func() {
		if eff == nil {
			s.m.w.braced(doAction, func() { s.m.w.line(accept.keeping) })
		} else {
			saved, savedKeep := s.m.bound, s.m.keeping
			s.m.bound, s.m.keeping = accept.bound, accept.keeping
			s.m.inlineBehavior(doAction, eff, t)
			s.m.bound, s.m.keeping = saved, savedKeep
		}
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
