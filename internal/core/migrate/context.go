package migrate

import (
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// behaviorContext is the object an activity no classifier owns acts on: the
// classifier whose ports its actions go through, taken as a reference parameter.
type behaviorContext struct {
	name       string
	classifier *xmi.Element
}

// contextOf settles, once, the context an activity needs: none when a classifier owns
// it, else the one classifier whose ports it or the behaviors it calls name.
func (m *migration) contextOf(b *xmi.Element) *behaviorContext {
	if b == nil || b.Type != "Activity" || classifierOf(b) != nil {
		return nil
	}
	if c, settled := m.contexts[b]; settled {
		return c
	}
	// A cycle of calls contributes nothing to itself.
	m.contexts[b] = nil
	var owners []*xmi.Element
	add := func(c *xmi.Element) {
		if c != nil && !slices.Contains(owners, c) {
			owners = append(owners, c)
		}
	}
	port := func(p *xmi.Element) {
		if p != nil && p.Parent != nil && m.written(p) && m.written(p.Parent) {
			add(p.Parent)
		}
	}
	m.walkActions(b, func(e *xmi.Element) {
		switch e.Type {
		case "SendSignalAction", "SendObjectAction", "CallOperationAction":
			port(m.model.Ref(e, "onPort"))
		case "Trigger":
			for _, p := range m.model.Refs(e, "port") {
				port(p)
			}
		case "CallBehaviorAction":
			if c := m.contextOf(m.model.Ref(e, "behavior")); c != nil {
				add(c.classifier)
			}
		}
	})
	c := m.mostSpecific(owners)
	if c == nil {
		if len(owners) > 1 {
			names := make([]string, len(owners))
			for i, o := range owners {
				names[i] = qualifiedName(o)
			}
			m.contextNotes[b] = "its actions go through ports of " + strings.Join(names, " and ") +
				", none of which is a special of the others, so no one object is written for them to act on"
		}
		return nil
	}
	ctx := &behaviorContext{name: m.freshName(b, "context"), classifier: c}
	m.contexts[b] = ctx
	return ctx
}

// walkActions visits every element of an activity's node graph, structured
// nodes included; a behavior nested in it is left to its own walk.
func (m *migration) walkActions(b *xmi.Element, visit func(*xmi.Element)) {
	var walk func(*xmi.Element)
	walk = func(e *xmi.Element) {
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
func (m *migration) mostSpecific(cs []*xmi.Element) *xmi.Element {
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
func (m *migration) contextParameter(b *xmi.Element) {
	c := m.contextOf(b)
	if c == nil {
		if note := m.contextNotes[b]; note != "" {
			m.add(b, Approximated, "", note)
		}
		return
	}
	m.w.line("in ref " + writeName(c.name) + " : " + m.ref(c.classifier, b) + ";")
	m.add(b, Mapped, "", "acts on a "+qualifiedName(c.classifier)+" through its ports, which it takes as its parameter "+c.name)
}

// enclosingActivity is the activity whose object the nodes written for e act
// on: e itself or the one its structured node belongs to; nil for an operation's body.
func enclosingActivity(e *xmi.Element) *xmi.Element {
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
func (a *activity) selfType() *xmi.Element {
	if a.ctx != nil {
		return a.ctx.classifier
	}
	return classifierOf(a.act)
}

// hasPort reports whether port is a port of the object the activity acts on.
func (a *activity) hasPort(port *xmi.Element) bool {
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
	self := a.selfType()
	kind := qualifiedName(c.classifier)
	switch {
	case self == nil:
		return "", "the behavior acts on a " + kind + " through its parameter " + c.name + ", which is left unbound: the caller acts on no object"
	case self == c.classifier || a.m.inherits(self, c.classifier):
		return a.self(), "the behavior acts on a " + kind + " through its parameter " + c.name + ", which is bound to " + a.self()
	}
	var parts []*xmi.Element
	for _, f := range a.m.attributesOf(self) {
		if f.Type != "Property" || !a.m.written(f) {
			continue
		}
		if t := a.m.model.Ref(f, "type"); t != nil && (t == c.classifier || a.m.inherits(t, c.classifier)) {
			parts = append(parts, f)
		}
	}
	if len(parts) == 1 {
		part := a.self() + "." + writeName(a.m.nameFor(parts[0]))
		return part, "the behavior acts on a " + kind + " through its parameter " + c.name + ", which is bound to " + part + ", the caller's one part that is one"
	}
	why := "has no part that is one"
	if len(parts) > 1 {
		why = "has " + strconv.Itoa(len(parts)) + " parts that are one, so no one of them is chosen"
	}
	return "", "the behavior acts on a " + kind + " through its parameter " + c.name + ", which is left unbound: the caller is a " + qualifiedName(self) + ", which is no " + kind + " and " + why
}
