package migrate

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// stateMachineBody writes a state machine's regions as the body of its state def.
func (m *migration) stateMachineBody(sm *sysmlv1.Element) {
	m.parameters(sm, sm)
	for _, c := range sm.Children {
		switch c.Role {
		case "ownedBehavior", "nestedClassifier", "ownedAttribute", "ownedOperation", "ownedRule":
			m.member(c)
		}
	}
	for _, cp := range sm.Owned("connectionPoint") {
		m.connectionPoint(cp)
	}
	m.regions(sm, sm.Owned("region"), inheritedStateNamesSet(), false, func() {})
}

// regions writes the regions of a state machine or composite state: one inline,
// several as the sub-states of one parallel state. The entry succession follows
// the body's entry action when entered says one was written; between writes
// the members that come after it and before the states.
func (m *migration) regions(owner *sysmlv1.Element, regions []*sysmlv1.Element, used map[string]bool, entered bool, between func()) {
	switch len(regions) {
	case 0:
		between()
		m.w.lines(commentLines("the " + kindOf(owner) + " has no region"))
	case 1:
		st := &stateRegion{m: m, r: regions[0], used: used, names: map[*sysmlv1.Element]string{}}
		st.enter(entered)
		between()
		st.write()
	default:
		name := freshIn(used, "regions")
		m.w.line(entryThen(entered, writeName(name)))
		between()
		m.w.block("state "+writeName(name)+" parallel", func() {
			inner := inheritedStateNamesSet()
			for _, r := range regions {
				rname := m.nameOf(r)
				if rname == "" {
					rname = "region"
				}
				rname = freshIn(inner, rname)
				m.names[r] = rname
				m.parallel[r] = name
				m.w.block("state "+writeName(rname), func() {
					st := &stateRegion{m: m, r: r, used: inheritedStateNamesSet(), names: map[*sysmlv1.Element]string{}}
					st.enter(false)
					st.write()
				})
				m.add(r, Mapped, rname, "an orthogonal region is written as a sub-state of the parallel state "+name)
			}
		})
		m.w.line("transition first " + writeName(name) + " then done;")
	}
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
	m     *migration
	r     *sysmlv1.Element
	used  map[string]bool
	names map[*sysmlv1.Element]string
}

// enter names the region's vertices and writes its entry succession.
func (s *stateRegion) enter(entered bool) {
	for _, v := range s.r.Owned("subvertex") {
		switch {
		case v.Type == "State", v.Type == "Pseudostate" && pseudoKind(v) == "choice", v.Type == "Pseudostate" && pseudoKind(v) == "junction":
			s.name(v)
		}
	}
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
	s.m.writeComments(s.r, false)
	s.m.add(s.r, Mapped, "", "the one region is written as the body of its owner")
}

// name returns the v2 name of a vertex in the body written.
func (s *stateRegion) name(v *sysmlv1.Element) string {
	if n, ok := s.names[v]; ok {
		return n
	}
	name := s.m.nameOf(v)
	if name == "" {
		switch pseudoKind(v) {
		case "choice", "junction":
			name = pseudoKind(v)
		default:
			name = "state"
		}
	}
	if s.used[name] {
		name = freshIn(s.used, name)
	}
	s.used[name] = true
	s.names[v] = name
	if name != s.m.nameOf(v) {
		s.m.names[v] = name
	}
	return name
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
			s.m.unmapped(t, "the target "+describe(tgt)+" is a "+kindOf(tgt)+" outside the region, or one with no v2 form")
		}
		return
	}
	note := ""
	for _, tr := range t.Owned("trigger") {
		note = "an initial transition takes no trigger; its triggers are dropped"
		s.m.add(tr, Unmapped, "", "a trigger of an initial transition, which takes none, is dropped")
	}
	if g := firstOwned(t, "guard"); g != nil {
		text := "the guard " + describe(g)
		if spec := firstOwned(g, "specification"); spec != nil {
			text = "[" + describeValue(spec) + "]"
		}
		note = joinNotes(note, "an initial transition takes no guard; "+text+" is dropped")
		s.m.add(g, Unmapped, "", "a guard of an initial transition, which takes none, is dropped")
	}
	switch eff := firstOwned(t, "effect"); {
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
			s.m.w.line("state " + name + ";")
			s.m.add(v, Approximated, name, "a "+pseudoKind(v)+" pseudostate is written as a state its guarded transitions leave at once")
		case "terminate":
			s.m.add(v, Approximated, "done", "a terminate pseudostate ends the machine; a transition to it is written to done, which ends its region")
		case "entryPoint", "exitPoint":
			s.m.connectionPoint(v)
		default:
			s.m.unmapped(v, "no v2 form for a "+pseudoKind(v)+" pseudostate")
		}
	case "ConnectionPointReference":
		s.m.unmapped(v, "a connection point reference has no v2 form; transitions through it are written to and from the submachine state")
	default:
		s.m.unmapped(v, "no v2 form for a UML "+v.Type)
	}
}

// connectionPoint reports an entry or exit point, which v2 has no form for.
func (m *migration) connectionPoint(v *sysmlv1.Element) {
	switch pseudoKind(v) {
	case "exitPoint":
		m.add(v, Approximated, "done", "a transition into the exit point is written to done, which completes the state def as a whole")
	default:
		m.add(v, Unmapped, "", "an entry point has no v2 form; a state def enters at its initial state")
	}
}

// state writes a state, typed by its submachine's state def when it has one,
// with its entry, do and exit actions, deferrals and regions.
func (s *stateRegion) state(v *sysmlv1.Element) {
	name := writeName(s.name(v))
	defers := s.deferrals(v)
	head := "state " + name
	if sub := s.m.model.Ref(v, "submachine"); sub != nil {
		for _, c := range v.Owned("connection") {
			s.m.unmapped(c, "a connection point reference has no v2 form; transitions through it are written to and from the submachine state")
		}
		if s.m.written(sub) {
			head += " : " + s.m.ref(sub, s.r)
			s.m.add(v, Mapped, name, "")
		} else {
			s.m.add(v, Approximated, name, "its submachine "+qualifiedName(sub)+" has no v2 declaration; the state is written simple")
		}
	} else {
		s.m.add(v, Mapped, name, "")
	}
	regions := v.Owned("region")
	entry, do, exit := firstOwned(v, "entry"), firstOwned(v, "doActivity"), firstOwned(v, "exit")
	inv := firstOwned(v, "stateInvariant")
	if entry == nil && do == nil && exit == nil && inv == nil && len(regions) == 0 && len(defers) == 0 {
		s.m.w.line(head + ";")
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
				s.m.inlineBehavior("do action", do, v)
			}
			if exit != nil {
				s.m.inlineBehavior("exit action", exit, v)
			}
		}
		if len(regions) == 0 {
			between()
			return
		}
		s.m.regions(v, regions, inheritedStateNamesSet(), entered, between)
	})
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
			s.m.add(ev, Unmapped, "", "a deferred trigger refers to it; "+note)
			continue
		}
		sig := s.m.model.Ref(ev, "signal")
		if sig == nil || !s.m.written(sig) {
			note := "the signal event names no migrated signal"
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
		if !m.written(b) {
			m.w.lines(commentLines(kw + " " + qualifiedName(b) + " has no v2 declaration"))
			m.add(b, Unmapped, "", "the behavior is not written; "+describe(owner)+" names it as its "+kw)
			return false
		}
		if cat, _ := m.classify(b); cat != catActionDef {
			m.w.lines(commentLines(kw + " " + qualifiedName(b) + " is written as a " + cat.keyword() + ", which no state runs"))
			m.downgrade(b, describe(owner)+" names it as its "+kw+", which a "+cat.keyword()+" cannot be")
			return false
		}
		m.w.line(kw + " : " + m.ref(b, owner) + ";")
		m.downgrade(b, "also run as the "+kw+" of "+describe(owner))
		return true
	}
	saved := m.scope
	m.scope = b
	defer func() { m.scope = saved }()
	header := kw
	if name := m.nameOf(b); name != "" {
		if inheritedStateNames[name] {
			name = freshIn(map[string]bool{name: true}, name)
			m.names[b] = name
		}
		header += " " + writeName(name)
	}
	switch b.Type {
	case "Activity":
		m.w.block(header, func() {
			m.comments(b)
			m.parameters(b, b)
			m.activityBody(b, b)
		})
		m.add(b, Mapped, m.v2Name(b), "written as the "+kw+" of "+describe(owner))
		return true
	case "OpaqueBehavior", "FunctionBehavior":
		body, lang := opaqueBody(b)
		lines, ok, note := m.statements(body, lang, b)
		m.w.block(header, func() {
			m.comments(b)
			m.parameters(b, b)
			if ok {
				m.w.lines(lines)
				return
			}
			m.opaqueComment(body, lang, note)
		})
		if ok {
			m.add(b, Approximated, m.v2Name(b), "the "+langName(lang)+" body is written as v2 assignments")
		} else {
			m.add(b, Approximated, m.v2Name(b), "the body is kept as a comment: "+note)
		}
		return true
	}
	m.w.lines(commentLines(kw + " " + describe(b) + " is a " + b.Type + ", which has no action form"))
	m.add(b, Unmapped, "", "a "+b.Type+" has no action form")
	return false
}

// target names what a transition leads to: a state of the region, done for a final
// or terminate state, or the state a connection point reference belongs to.
func (s *stateRegion) target(t, v *sysmlv1.Element) (string, bool) {
	if v == nil {
		return "", false
	}
	switch v.Type {
	case "FinalState":
		return "done", true
	case "State":
		if v.Parent != s.r {
			return "", false
		}
		return writeName(s.name(v)), true
	case "Pseudostate":
		switch pseudoKind(v) {
		case "terminate", "exitPoint":
			return "done", true
		case "choice", "junction":
			if v.Parent != s.r {
				return "", false
			}
			return writeName(s.name(v)), true
		}
	case "ConnectionPointReference":
		if v.Parent != nil && v.Parent.Type == "State" && v.Parent.Parent == s.r {
			s.m.add(t, Approximated, "", "the transition through the connection point "+describe(v)+" is written to the submachine state, which enters at its initial state")
			return writeName(s.name(v.Parent)), true
		}
	}
	return "", false
}

// source names the state a transition leaves.
func (s *stateRegion) source(t, v *sysmlv1.Element) (string, bool) {
	if v == nil {
		return "", false
	}
	switch v.Type {
	case "State":
		if v.Parent != s.r {
			return "", false
		}
		return writeName(s.name(v)), true
	case "Pseudostate":
		switch pseudoKind(v) {
		case "choice", "junction":
			if v.Parent != s.r {
				return "", false
			}
			return writeName(s.name(v)), true
		}
	case "ConnectionPointReference":
		if v.Parent != nil && v.Parent.Type == "State" && v.Parent.Parent == s.r {
			s.m.add(t, Approximated, "", "the transition from the connection point "+describe(v)+" is written from the submachine state, firing when it completes")
			return writeName(s.name(v.Parent)), true
		}
	}
	return "", false
}

// transition writes a transition: one per trigger, since a v2 transition
// accepts one, sharing the guard and effect.
func (s *stateRegion) transition(t *sysmlv1.Element) {
	src, tgt := s.m.model.Ref(t, "source"), s.m.model.Ref(t, "target")
	if src == nil || tgt == nil {
		s.m.unmapped(t, joinNotes(s.m.dangling(t, "source", "target"), "the transition lacks an end"))
		return
	}
	if pseudoKind(src) == "initial" {
		// Written as the region's entry.
		return
	}
	from, ok := s.source(t, src)
	if !ok {
		s.m.unmapped(t, "the source "+describe(src)+" is a "+kindOf(src)+" outside the region, or one with no v2 form")
		return
	}
	to, ok := s.target(t, tgt)
	if !ok {
		s.m.unmapped(t, "the target "+describe(tgt)+" is a "+kindOf(tgt)+" outside the region, or one with no v2 form")
		return
	}
	if t.Attrs["kind"] == "internal" {
		s.m.unmapped(t, "an internal transition has no v2 form")
		return
	}
	guard, gnote := s.guard(t)
	eff := firstOwned(t, "effect")
	var accepts []acceptance
	var notes []string
	if gnote != "" {
		notes = append(notes, gnote)
	}
	triggers := t.Owned("trigger")
	for _, tr := range triggers {
		ev := s.m.model.Ref(tr, "event")
		if ev == nil {
			note := joinNotes(s.m.dangling(tr, "event"), "the trigger names no event")
			s.m.add(tr, Unmapped, "", note)
			notes = append(notes, "a trigger is dropped: "+note)
			continue
		}
		var a acceptance
		if eff != nil {
			a = s.payload(t, eff, ev)
		}
		clause, note, ok := s.m.triggerClause(ev, t, a.payload)
		if !ok {
			s.m.add(tr, Unmapped, "", note)
			notes = append(notes, "a trigger is dropped: "+note)
			continue
		}
		s.m.add(tr, verdictFor(note), "", note)
		a.clause = " " + clause
		accepts = append(accepts, a)
	}
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
	} else if len(accepts) > 1 {
		notes = append(notes, "written as "+strconv.Itoa(len(accepts))+" transitions, one per trigger")
	}
	tname := ""
	if s.m.nameOf(t) != "" {
		tname = freshIn(s.used, s.m.nameOf(t))
	}
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
		if eff != nil {
			if i > 0 {
				s.m.downgrade(eff, "run by each of the transitions written for its triggers")
			}
			s.m.w.line(line)
			s.m.w.indented(func() {
				saved := s.m.bound
				s.m.bound = accept.bound
				s.m.inlineBehavior("do action", eff, t)
				s.m.bound = saved
				s.m.w.line("then " + to + ";")
			})
			continue
		}
		s.m.w.line(line + " then " + to + ";")
	}
	note := strings.Join(notes, "; ")
	s.m.add(t, verdictFor(note), tname, note)
}

// acceptance is one written trigger: its accept clause, the name the clause
// gives the accepted signal, and the effect parameters bound to that name.
type acceptance struct {
	clause  string
	payload string
	bound   map[*sysmlv1.Element]string
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
	used := map[string]bool{}
	for _, p := range eff.Owned("ownedParameter") {
		used[s.m.nameFor(p)] = true
	}
	name := freshIn(used, lowerFirst(s.m.nameFor(sig)))
	a := acceptance{payload: writeName(name), bound: map[*sysmlv1.Element]string{}}
	for _, p := range params {
		typ := s.m.model.Ref(p, "type")
		switch {
		case typ == sig || typ != nil && s.m.inherits(sig, typ), typ == nil && len(params) == 1:
			a.bound[p] = a.payload
		case typ == nil:
			s.m.add(p, Approximated, "", "the parameter takes no value: it is untyped, and the transition passes only the accepted "+s.m.nameFor(sig))
		default:
			s.m.add(p, Approximated, "", "the parameter takes no value: the transition passes only the accepted "+s.m.nameFor(sig)+", which is no "+s.m.nameFor(typ))
		}
	}
	return a
}

// unbound notes on each in parameter of an effect why it takes no value.
func (m *migration) unbound(eff *sysmlv1.Element, why string) {
	for _, p := range inParameters(eff) {
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
// comment when it is not a v2 expression the state machine's owner resolves.
func (s *stateRegion) guard(t *sysmlv1.Element) (string, string) {
	g := firstOwned(t, "guard")
	if g == nil {
		return "", ""
	}
	spec := firstOwned(g, "specification")
	if spec == nil {
		s.m.add(g, Unmapped, "", "the guard has no specification")
		return "", "the guard " + describe(g) + " has no specification and is dropped"
	}
	if spec.Type == "LiteralBoolean" && (spec.Attrs["value"] == "true" || spec.Attrs["value"] == "") {
		s.m.add(g, Mapped, "", "a true guard is not written")
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
