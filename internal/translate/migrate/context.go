package migrate

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// behaviorContext is the object an activity acts on when its owner is not it: the
// classifier whose ports its actions go through, taken as a reference parameter.
type behaviorContext struct {
	name       string
	classifier *sysmlv1.Element
}

// contextVisit is an activity the search settling contexts has reached: its
// place in the visit order, the earliest place a chain of calls from it reaches,
// and the classifiers whose ports it names itself or through settled behaviors.
type contextVisit struct {
	index, low int
	owners     []*sysmlv1.Element
}

// contextOf settles, once, the context an activity needs: the one classifier whose
// ports it or the behaviors it calls name. A classifier's own behavior acts on its
// object unless nothing it needs is one: v1 runs a called behavior on the caller's
// object, whoever owns it, so such a behavior takes the object it acts on instead.
// Activities calling each other in a cycle name the same ports through the cycle,
// so the search settles a cycle at once, with everything its members name.
func (m *migration) contextOf(b *sysmlv1.Element) *behaviorContext {
	if b == nil || b.Type != "Activity" {
		return nil
	}
	if c, settled := m.contexts[b]; settled {
		return c
	}
	if m.visiting[b] != nil {
		// Reached again before settling: the cycle it closes settles it.
		return nil
	}
	m.visitContext(b)
	return m.contexts[b]
}

// visitContext reaches b and, through it, the behaviors it calls; once every call
// from b leads back no earlier than b, b and the cycle it heads are settled.
func (m *migration) visitContext(b *sysmlv1.Element) {
	v := &contextVisit{index: len(m.visits), low: len(m.visits), owners: m.namedPortOwners(b)}
	m.visiting[b] = v
	m.visits = append(m.visits, b)
	for _, c := range m.calledActivities(b) {
		if ctx, settled := m.contexts[c]; settled {
			if ctx != nil {
				v.owners = addOwner(v.owners, ctx.classifier)
			}
			continue
		}
		if w := m.visiting[c]; w != nil {
			v.low = min(v.low, w.index)
			continue
		}
		m.visitContext(c)
		if ctx, settled := m.contexts[c]; settled {
			if ctx != nil {
				v.owners = addOwner(v.owners, ctx.classifier)
			}
		} else {
			v.low = min(v.low, m.visiting[c].low)
		}
	}
	if v.low != v.index {
		return
	}
	members := slices.Clone(m.visits[v.index:])
	m.visits = m.visits[:v.index]
	var owners []*sysmlv1.Element
	for _, e := range members {
		for _, o := range m.visiting[e].owners {
			owners = addOwner(owners, o)
		}
		delete(m.visiting, e)
		// Settled as none while the members decide, so a cycle contributes nothing to itself.
		m.contexts[e] = nil
	}
	decided := make([]*behaviorContext, len(members))
	for i, e := range members {
		decided[i] = m.decideContext(e, owners)
	}
	for i, e := range members {
		m.contexts[e] = decided[i]
	}
}

func addOwner(owners []*sysmlv1.Element, c *sysmlv1.Element) []*sysmlv1.Element {
	if c != nil && !slices.Contains(owners, c) {
		owners = append(owners, c)
	}
	return owners
}

// decideContext is the context b takes given the classifiers whose ports it or
// the behaviors it calls name; the note why none is written is kept for the report.
func (m *migration) decideContext(b *sysmlv1.Element, owners []*sysmlv1.Element) *behaviorContext {
	how := "its actions go through ports of "
	switch owner := classifierOf(b); {
	case owner != nil && b.Parent != owner:
		// A state's or transition's behavior runs on the machine's object.
		return nil
	case owner != nil:
		if len(owners) == 0 || m.providesAny(owner, owners) || m.usesFeaturesOf(b, owner) {
			return nil
		}
	case len(owners) == 0:
		owners = m.invokerOwners(b)
		how = "its accepts take signals arriving at the ports of "
	}
	c := m.mostSpecific(owners)
	if c == nil {
		if len(owners) > 1 {
			names := make([]string, len(owners))
			for i, o := range owners {
				names[i] = qualifiedName(o)
			}
			m.contextNotes[b] = how + strings.Join(names, " and ") +
				", none of which is a special of the others, so no one object is written for them to act on"
		}
		return nil
	}
	return &behaviorContext{name: m.freshName(b, "context"), classifier: c}
}

// providesAny reports whether an object of owner is, or holds one part that is,
// one of the classifiers cs: then a behavior it owns acts on it.
func (m *migration) providesAny(owner *sysmlv1.Element, cs []*sysmlv1.Element) bool {
	for _, c := range cs {
		if expr, _ := m.contextBinding(&behaviorContext{classifier: c}, owner, "this"); expr != "" {
			return true
		}
	}
	return false
}

// usesFeaturesOf reports whether b reads itself or a structural feature of c, so
// that it plainly acts on an object of c.
func (m *migration) usesFeaturesOf(b, c *sysmlv1.Element) bool {
	uses := false
	m.walkActions(b, func(e *sysmlv1.Element) {
		switch e.Type {
		case "ReadSelfAction":
			uses = true
		case "ReadStructuralFeatureAction", "AddStructuralFeatureValueAction", "RemoveStructuralFeatureValueAction", "ClearStructuralFeatureAction":
			if f := m.model.Ref(e, "structuralFeature"); f != nil && m.hasFeature(c, f) {
				uses = true
			}
		}
	})
	return uses
}

// portOwners lists the classifiers whose ports the actions of b, or the
// behaviors it calls, name.
func (m *migration) portOwners(b *sysmlv1.Element) []*sysmlv1.Element {
	owners := m.namedPortOwners(b)
	for _, c := range m.calledActivities(b) {
		if ctx := m.contextOf(c); ctx != nil {
			owners = addOwner(owners, ctx.classifier)
		}
	}
	return owners
}

// namedPortOwners lists the classifiers whose ports the actions of b itself name.
func (m *migration) namedPortOwners(b *sysmlv1.Element) []*sysmlv1.Element {
	var owners []*sysmlv1.Element
	port := func(p *sysmlv1.Element) {
		if p != nil && p.Parent != nil && m.written(p) && m.written(p.Parent) {
			owners = addOwner(owners, p.Parent)
		}
	}
	m.walkActions(b, func(e *sysmlv1.Element) {
		switch e.Type {
		case "SendSignalAction", "SendObjectAction", "CallOperationAction":
			port(m.model.Ref(e, "onPort"))
		case "Trigger":
			for _, p := range m.model.Refs(e, "port") {
				port(p)
			}
		}
	})
	return owners
}

// calledActivities lists the activities the call actions of b name, once each.
func (m *migration) calledActivities(b *sysmlv1.Element) []*sysmlv1.Element {
	var called []*sysmlv1.Element
	m.walkActions(b, func(e *sysmlv1.Element) {
		if e.Type != "CallBehaviorAction" {
			return
		}
		if c := m.model.Ref(e, "behavior"); c != nil && c.Type == "Activity" && !slices.Contains(called, c) {
			called = append(called, c)
		}
	})
	return called
}

// invokerOwners lists the classifiers whose behaviors run b, when a signal an
// accept of b names no port for arrives at a port of theirs: v1 hands the
// object's accepts what its ports receive, which v2 takes only via the port.
func (m *migration) invokerOwners(b *sysmlv1.Element) []*sysmlv1.Element {
	sigs := m.unportedSignals(b)
	if len(sigs) == 0 {
		return nil
	}
	var owners []*sysmlv1.Element
	for _, c := range m.runningClassifiers(b, map[*sysmlv1.Element]bool{b: true}) {
		if slices.Contains(owners, c) || !m.written(c) {
			continue
		}
		for _, sig := range sigs {
			if len(m.arrivalPorts(c, sig)) > 0 {
				owners = append(owners, c)
				break
			}
		}
	}
	return owners
}

// unportedSignals lists the signals the accept actions of b name no port for.
func (m *migration) unportedSignals(b *sysmlv1.Element) []*sysmlv1.Element {
	var sigs []*sysmlv1.Element
	m.walkActions(b, func(e *sysmlv1.Element) {
		if e.Type != "Trigger" || e.Parent == nil || e.Parent.Type != "AcceptEventAction" || len(m.model.Refs(e, "port")) > 0 {
			return
		}
		ev := m.model.Ref(e, "event")
		if ev == nil || ev.Type != "SignalEvent" {
			return
		}
		if sig := m.model.Ref(ev, "signal"); sig != nil && !slices.Contains(sigs, sig) {
			sigs = append(sigs, sig)
		}
	})
	return sigs
}

// runningClassifiers lists the classifiers whose behaviors run b: the owners of
// what invokes it, followed up through activities no classifier owns.
func (m *migration) runningClassifiers(b *sysmlv1.Element, seen map[*sysmlv1.Element]bool) []*sysmlv1.Element {
	var out []*sysmlv1.Element
	add := func(c *sysmlv1.Element) {
		if c != nil && !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	for _, e := range m.invokers[b] {
		host := enclosingActivity(e)
		switch {
		case e.Type != "CallBehaviorAction":
			if behaviorScope(e) {
				add(classifierOf(e))
			} else {
				add(e)
			}
		case host == nil:
		case classifierOf(host) != nil && m.contextOf(host) == nil:
			add(classifierOf(host))
		case classifierOf(host) != nil:
			add(m.contextOf(host).classifier)
		case seen[host]:
		default:
			seen[host] = true
			if c := m.mostSpecific(m.portOwners(host)); c != nil {
				add(c)
				continue
			}
			for _, c := range m.runningClassifiers(host, seen) {
				add(c)
			}
		}
	}
	return out
}

// invoke indexes e as an invoker of each behavior a role names without owning it.
func (m *migration) invoke(e *sysmlv1.Element, roles ...string) {
	for _, role := range roles {
		for _, b := range m.model.Refs(e, role) {
			if b.Parent != e && isBehavior(b) {
				m.invokers[b] = append(m.invokers[b], e)
			}
		}
	}
}

// walkActions visits every element of an activity's node graph, structured
// nodes included; a behavior nested in it is left to its own walk.
func (m *migration) walkActions(b *sysmlv1.Element, visit func(*sysmlv1.Element)) {
	var walk func(*sysmlv1.Element)
	walk = func(e *sysmlv1.Element) {
		for _, c := range e.Children {
			if isBehavior(c) {
				continue
			}
			visit(c)
			walk(c)
		}
	}
	walk(b)
}

// mostSpecific is the classifier of the list every other one generalizes, nil
// when the list is empty or none is.
func (m *migration) mostSpecific(cs []*sysmlv1.Element) *sysmlv1.Element {
	for _, c := range cs {
		special := true
		for _, o := range cs {
			if o != c && !m.inherits(c, o) {
				special = false
				break
			}
		}
		if special {
			return c
		}
	}
	return nil
}

// contextParameter declares the reference parameter an activity takes for the
// object whose ports its actions go through, or reports why none is written.
func (m *migration) contextParameter(b *sysmlv1.Element) {
	c := m.contextOf(b)
	if c == nil {
		if note := m.contextNotes[b]; note != "" {
			m.add(b, Approximated, "", note)
		}
		return
	}
	m.w.line("in ref " + writeName(c.name) + " : " + m.ref(c.classifier, b) + ";")
	note := "acts on a " + qualifiedName(c.classifier) + " through its ports, which it takes as its parameter " + c.name
	if owner := classifierOf(b); owner != nil {
		m.add(b, Approximated, "", note+" rather than its owner "+qualifiedName(owner)+", which is no such object and holds no one part that is: v1 ran it on whichever object called it")
		return
	}
	m.add(b, Mapped, "", note)
}

// enclosingActivity is the activity whose object the nodes written for e act
// on: e itself or the one its structured node belongs to; nil for an operation's body.
func enclosingActivity(e *sysmlv1.Element) *sysmlv1.Element {
	for cur := e; cur != nil && cur.Type != "Operation"; cur = cur.Parent {
		if cur.Type == "Activity" {
			return cur
		}
	}
	return nil
}

// self writes the object the activity's actions act on: this, or the context parameter.
func (a *activity) self() string {
	if a.ctx != nil {
		return writeName(a.ctx.name)
	}
	return "this"
}

// selfType is the classifier of the object the activity acts on, nil when none is known.
func (a *activity) selfType() *sysmlv1.Element {
	if a.ctx != nil {
		return a.ctx.classifier
	}
	return classifierOf(a.act)
}

// hasPort reports whether port is a port of the object the activity acts on.
func (a *activity) hasPort(port *sysmlv1.Element) bool {
	t := a.selfType()
	return t != nil && a.m.written(port) && a.m.hasFeature(t, port)
}

// on writes a member of the object obj holds as a perform or an accept names it:
// bare for a member of this, else under the object's path.
func (a *activity) on(obj, member string) string {
	if obj == "this" {
		return member
	}
	return strings.TrimPrefix(obj, "this.") + "." + member
}

// viaPrefix is what an accept's via path starts with to name a port of the object
// the activity acts on: nothing for this, whose ports the action's scope sees.
func (a *activity) viaPrefix() string {
	if a.ctx != nil {
		return writeName(a.ctx.name) + "."
	}
	return ""
}

// contextArgument writes the object bound to a called behavior's context parameter:
// the caller's object when it is one, else its one part that is.
func (a *activity) contextArgument(c *behaviorContext) (expr, note string) {
	return a.m.contextBinding(c, a.selfType(), a.self())
}

// contextBinding writes the object bound to a run behavior's context parameter,
// where the runner's object is a self of type selfType: that object when it is
// one, else its one part that is.
func (m *migration) contextBinding(c *behaviorContext, selfType *sysmlv1.Element, self string) (expr, note string) {
	kind := qualifiedName(c.classifier)
	switch {
	case selfType == nil:
		return "", "the behavior acts on a " + kind + " through its parameter " + c.name + ", which is left unbound: the caller acts on no object"
	case selfType == c.classifier || m.inherits(selfType, c.classifier):
		return self, "the behavior acts on a " + kind + " through its parameter " + c.name + ", which is bound to " + self
	}
	var parts []*sysmlv1.Element
	for _, f := range m.attributesOf(selfType) {
		if f.Type != "Property" || !m.written(f) {
			continue
		}
		if t := m.model.Ref(f, "type"); t != nil && (t == c.classifier || m.inherits(t, c.classifier)) {
			parts = append(parts, f)
		}
	}
	if len(parts) == 1 {
		part := self + "." + writeName(m.nameFor(parts[0]))
		return part, "the behavior acts on a " + kind + " through its parameter " + c.name + ", which is bound to " + part + ", the caller's one part that is one"
	}
	why := "has no part that is one"
	if len(parts) > 1 {
		why = "has " + strconv.Itoa(len(parts)) + " parts that are one, so no one of them is chosen"
	}
	return "", "the behavior acts on a " + kind + " through its parameter " + c.name + ", which is left unbound: the caller is a " + qualifiedName(selfType) + ", which is no " + kind + " and " + why
}
