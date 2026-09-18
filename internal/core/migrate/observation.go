package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// stamp is an action written beside a node that reads the clock into an
// attribute of the action def: before the node, or after it and before its successions.
type stamp struct {
	name  string
	lines []string
}

// timing is what a duration observation between two nodes of an activity
// becomes: a [0..1] attribute holding the elapsed clock, stamped at each end
// and left without a value, not zero, by a run that does not reach both.
type timing struct {
	o              *xmi.Element
	from, to       *xmi.Element
	fromEnd, toEnd bool
	name, start    string
}

// timings reads the activity's duration observations whose events are nodes
// of it into attributes of the action def, and reports the rest with the reason.
func (a *activity) timings() {
	for _, o := range a.act.Owned("observation") {
		if o.Type != "DurationObservation" {
			a.m.add(o, Unmapped, "", "an observation records what a run measures; a UML "+o.Type+" has no v2 form")
			continue
		}
		t, why := a.timingOf(o)
		if t == nil {
			a.m.add(o, Unmapped, "", why)
			continue
		}
		t.name = a.fresh(a.m.nameFor(o))
		t.start = a.fresh(t.name + " start")
		a.timed = append(a.timed, t)
		a.stampAt(t.from, t.fromEnd, "assign "+writeName(t.start)+" := "+clockRead+";")
		a.stampAt(t.to, t.toEnd,
			"if "+writeName(t.start)+"->SequenceFunctions::notEmpty() {",
			"    assign "+writeName(t.name)+" := "+clockRead+" - "+writeName(t.start)+";",
			"}")
	}
}

// timingOf reads which nodes an observation spans and at which end of each, or
// says why it spans none: it observes no events, or events that are not nodes here.
// An initial or flow final node is a point a token reaches, so it has a start but no end.
func (a *activity) timingOf(o *xmi.Element) (*timing, string) {
	if why := a.m.dangling(o, "event"); why != "" {
		return nil, "the observation's events resolve to nothing: " + why
	}
	events := a.m.model.Refs(o, "event")
	if len(events) == 0 {
		if by := a.m.observers(o); len(by) > 0 {
			return nil, "the observation observes no event; the duration " + strings.Join(by, ", ") + " that refers to it is written from its own value"
		}
		return nil, "the observation observes no event, so there is nothing to measure between"
	}
	if len(events) > 2 {
		return nil, "the observation names more than two events"
	}
	for _, e := range events {
		if k := nodeKind(e); e.Parent != a.act || k != nodeAction && k != nodeControl && k != nodeBuffer && k != nodeFinal && k != nodeInitial && k != nodeFlowFinal {
			return nil, "the observation's event " + describe(e) + " is not a node of the activity, so no elapsed clock can be read between its nodes"
		}
	}
	ends := strings.Fields(o.Attrs["firstEvent"])
	for _, c := range o.Children {
		if c.Role == "firstEvent" {
			ends = append(ends, strings.TrimSpace(c.Text))
		}
	}
	t := &timing{o: o, from: events[0], to: events[len(events)-1]}
	at := func(i int, dflt bool) bool {
		if i < len(ends) {
			return ends[i] == "false"
		}
		return dflt
	}
	if len(events) == 1 {
		// UML 2.5.1 DurationObservation: one event observes the duration of that
		// element's own execution, from its entering to its exiting.
		t.fromEnd, t.toEnd = false, true
	} else {
		// firstEvent is [0..2] with no default in UML; omitted, the span covers both nodes whole.
		t.fromEnd, t.toEnd = at(0, false), at(1, true)
	}
	for _, end := range []struct {
		n   *xmi.Element
		end bool
	}{{t.from, t.fromEnd}, {t.to, t.toEnd}} {
		if end.end && nodeKind(end.n) != nodeAction {
			return nil, "the observation reads the clock at the end of " + describe(end.n) + ", which is no action and so has no end of its own"
		}
	}
	return t, ""
}

// stampAt schedules a clock read at one end of node n: before it starts, or
// after it ends and before anything it leads to. An initial node's start is
// the activity's, so its stamp follows start (see startSuccessions).
func (a *activity) stampAt(n *xmi.Element, end bool, lines ...string) {
	stamps := a.before
	if end {
		stamps = a.after
	}
	s := stamps[n]
	if s == nil {
		s = &stamp{name: writeName(a.fresh("stamp"))}
		stamps[n] = s
	}
	s.lines = append(s.lines, lines...)
}

// flowFinal writes a flow final node: done, led into through the stamp, wait
// or merge written before it when a timing or duration bounds the node.
func (a *activity) flowFinal(n *xmi.Element) {
	if _, ok := a.entry[n]; ok {
		a.leadIn(n, "done")
	}
	a.m.add(n, Mapped, "done", "a flow final ends the token, as done does")
}

// timingAttributes declares the attributes the timings read the clock into.
func (a *activity) timingAttributes() {
	for _, t := range a.timed {
		a.m.w.line("attribute " + writeName(t.start) + " : ScalarValues::Real [0..1];")
		a.m.w.line("attribute " + writeName(t.name) + " : ScalarValues::Real [0..1];")
		note := "the elapsed clock from the " + endName(t.fromEnd) + " of " + describe(t.from) + " to the " + endName(t.toEnd) + " of " + describe(t.to) + " is assigned to the attribute " + t.name + ", in seconds, and left without a value by a run that does not reach both"
		a.m.add(t.o, Mapped, a.m.v2Name(a.def)+"."+t.name, note)
	}
}

func endName(end bool) string {
	if end {
		return "end"
	}
	return "start"
}

// strayObservation says why an observation owned outside any activity has no v2
// form: no nodes of an activity bound it, so there is no clock to read between.
func (m *migration) strayObservation(o *xmi.Element) string {
	note := "the observation is owned by " + qualifiedName(o.Parent) + ", not an activity, so no nodes bound what it measures"
	if by := m.observers(o); len(by) > 0 {
		note += "; the duration " + strings.Join(by, ", ") + " that refers to it is written from its own value"
	}
	return note
}

// observers names the durations that refer to observation o, which the
// document links only from the duration's side.
func (m *migration) observers(o *xmi.Element) []string {
	if m.observed == nil {
		m.observed = map[*xmi.Element][]*xmi.Element{}
		var walk func(e *xmi.Element)
		walk = func(e *xmi.Element) {
			if e.Type == "Duration" || e.Type == "TimeExpression" {
				for _, obs := range m.model.Refs(e, "observation") {
					m.observed[obs] = append(m.observed[obs], e)
				}
			}
			for _, c := range e.Children {
				walk(c)
			}
		}
		for _, r := range m.model.Roots {
			walk(r)
		}
	}
	var names []string
	for _, d := range m.observed[o] {
		if v := firstOwned(d, "expr"); v != nil {
			names = append(names, describeValue(v))
		} else {
			names = append(names, describe(d))
		}
	}
	return names
}

// leafStep writes a call behavior action that calls nothing and has no pins
// as the step its duration constraint stands for; false when it has pins or a
// behavior reference the document does not resolve, which are lost.
func (a *activity) leafStep(n *xmi.Element, name string) bool {
	if len(a.m.model.Unresolved(n, "behavior")) > 0 || len(inputPins(n)) > 0 || len(n.Owned("result")) > 0 || len(n.Owned("outputValue")) > 0 {
		return false
	}
	a.m.w.line("action " + name + ";")
	if _, ok := a.waits[n]; ok {
		a.m.add(n, Mapped, name, "a step with a duration and no further behavior")
	} else {
		a.m.add(n, Approximated, name, "a step with no behavior and no duration; it passes the token on")
	}
	return true
}
