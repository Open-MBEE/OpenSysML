package migrate

import (
	"sort"

	"github.com/Open-MBEE/OpenSysML/internal/core/xmi"
)

// A swimlane (an ActivityPartition) says who performs the nodes it holds: the
// object its `represents` names, a property of the activity's context or a
// classifier. Names in the bodies and guards of those nodes resolve against
// that object first, written as the feature chain the runtime reads it by.

// lane is one partition of an activity with the object it represents resolved.
type lane struct {
	g          *xmi.Element // the ActivityPartition
	parent     *lane        // the partition it is nested in, nil at the top
	represents *xmi.Element // the property or classifier it represents, nil when unset
	typ        *xmi.Element // the classifier whose features names inside resolve against
	expr       string       // how the activity reads the represented object; "" when it cannot
	note       string       // why expr is "", or how it was found
	used       bool         // whether a name or a call resolved through the lane
}

// lanes indexes the partitions of one activity by the nodes and edges they hold.
type lanes struct {
	all []*lane
	of  map[*xmi.Element]*lane // node or edge → the innermost partition holding it
}

// lanesOf indexes the partitions of an activity once, before its nodes are written.
func (m *migration) lanesOf(act *xmi.Element) *lanes {
	if ls, ok := m.lanes[act]; ok {
		return ls
	}
	ls := &lanes{of: map[*xmi.Element]*lane{}}
	m.lanes[act] = ls
	ctx := m.contextClassifier(act)
	var index func(g *xmi.Element, parent *lane)
	index = func(g *xmi.Element, parent *lane) {
		l := &lane{g: g, parent: parent}
		m.resolveLane(l, ctx)
		ls.all = append(ls.all, l)
		for _, n := range m.model.Refs(g, "node") {
			ls.of[n] = l
		}
		for _, e := range m.model.Refs(g, "edge") {
			ls.of[e] = l
		}
		for _, sub := range g.Owned("subpartition") {
			index(sub, l)
		}
	}
	for _, g := range act.Owned("group") {
		if g.Type == "ActivityPartition" {
			index(g, nil)
		}
	}
	// A node may name its partitions itself; the innermost one holds it.
	for _, n := range append(act.Owned("node"), act.Owned("edge")...) {
		if ls.of[n] != nil {
			continue
		}
		for _, g := range m.model.Refs(n, "inPartition") {
			if l := ls.find(g); l != nil && (ls.of[n] == nil || l.depth() > ls.of[n].depth()) {
				ls.of[n] = l
			}
		}
	}
	return ls
}

// find returns the lane of a partition element.
func (ls *lanes) find(g *xmi.Element) *lane {
	for _, l := range ls.all {
		if l.g == g {
			return l
		}
	}
	return nil
}

// depth is how many partitions enclose the lane.
func (l *lane) depth() int {
	d := 0
	for p := l.parent; p != nil; p = p.parent {
		d++
	}
	return d
}

// laneOf returns the innermost partition holding a node or edge of the
// activity; an edge in none is held by its source's partition, then its target's.
func (ls *lanes) laneOf(m *migration, e *xmi.Element) *lane {
	if ls == nil {
		return nil
	}
	if l := ls.of[e]; l != nil {
		return l
	}
	if e.Type == "ControlFlow" || e.Type == "ObjectFlow" {
		for _, role := range []string{"source", "target"} {
			if n := m.model.Ref(e, role); n != nil {
				if l := ls.of[ownerNode(n)]; l != nil {
					return l
				}
			}
		}
	}
	return nil
}

// resolveLane finds how the activity, whose context object is a ctx, reads
// the object the lane represents: `this` itself, one of its parts, or a part
// of the enclosing lane's object.
func (m *migration) resolveLane(l *lane, ctx *xmi.Element) {
	r := m.model.Ref(l.g, "represents")
	if r == nil {
		if m.dangling(l.g, "represents") != "" {
			l.note = "the partition represents an element outside the document"
		} else {
			l.note = "the partition represents nothing"
		}
		return
	}
	l.represents = r
	if r.Type == "Property" || r.Type == "Port" {
		owner := r.Parent
		l.typ = m.model.Ref(r, "type")
		name := writeName(m.nameOf(r))
		switch {
		case !m.written(r) || m.nameOf(r) == "":
			l.note = "the property " + qualifiedName(r) + " it represents has no v2 declaration"
		case l.parent != nil && l.parent.expr != "" && l.parent.typ != nil && m.hasFeature(l.parent.typ, r):
			l.expr = l.parent.expr + "." + name
			l.note = "the partition represents " + m.nameOf(r) + " of the enclosing partition's object, read as " + l.expr
		case ctx == nil:
			l.note = "the activity is in no classifier whose object could hold " + qualifiedName(r)
		case m.hasFeature(ctx, r):
			l.expr = "this." + name
			l.note = "the partition represents the context's " + m.nameOf(r) + ", read as " + l.expr
		default:
			if path := m.partPath(ctx, owner, 3); path != "" {
				l.expr = "this." + path + "." + name
				l.note = "the partition represents " + qualifiedName(r) + ", read as " + l.expr
			} else {
				l.note = "no part of " + qualifiedName(ctx) + " is a " + qualifiedName(owner) + ", which holds the represented " + m.nameOf(r)
			}
		}
		if l.typ == nil && l.expr != "" {
			l.note += "; the property has no type, so no name resolves through it"
		}
		return
	}
	l.typ = r
	switch {
	case !m.written(r):
		l.note = "the classifier " + qualifiedName(r) + " it represents has no v2 declaration"
	case l.parent != nil && l.parent.expr != "" && l.parent.typ != nil && (l.parent.typ == r || m.inherits(l.parent.typ, r)):
		l.expr = l.parent.expr
		l.note = "the partition represents the enclosing partition's object, a " + qualifiedName(r)
	case ctx == nil:
		l.note = "the activity is in no classifier whose object could be the represented " + qualifiedName(r)
	case ctx == r || m.inherits(ctx, r):
		l.expr = "this"
		l.note = "the partition represents the context object itself, a " + qualifiedName(r)
	default:
		if path := m.partPath(ctx, r, 3); path != "" {
			l.expr = "this." + path
			l.note = "the partition represents the context's part " + path + ", a " + qualifiedName(r)
		} else {
			l.note = "no part of " + qualifiedName(ctx) + " is a " + qualifiedName(r) + ", which the partition represents"
		}
	}
}

// partPath finds the one chain of parts from classifier c to an object of
// classifier target, at most depth parts long; "" when none or several exist.
func (m *migration) partPath(c, target *xmi.Element, depth int) string {
	var found []string
	seen := map[*xmi.Element]bool{c: true}
	var walk func(t *xmi.Element, prefix string, left int)
	walk = func(t *xmi.Element, prefix string, left int) {
		if left == 0 || len(found) > 1 {
			return
		}
		visible, _ := m.membersOf(t, memberAny)
		names := make([]string, 0, len(visible))
		for name := range visible {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			p := visible[name]
			if p.Type != "Property" || p.Attrs["aggregation"] != "composite" {
				continue
			}
			pt := m.model.Ref(p, "type")
			if pt == nil || pt.IsProxy() {
				continue
			}
			path := prefix + writeName(name)
			if pt == target || m.inherits(pt, target) {
				found = append(found, path)
				continue
			}
			if !seen[pt] {
				seen[pt] = true
				walk(pt, path+".", left-1)
				delete(seen, pt)
			}
		}
	}
	walk(c, "", depth)
	if len(found) == 1 {
		return found[0]
	}
	return ""
}

// contextClassifier is the classifier whose object is `this` inside e: the
// nearest enclosing one that is not a behavior, nil inside a package.
func (m *migration) contextClassifier(e *xmi.Element) *xmi.Element {
	for cur := e; cur != nil; cur = cur.Parent {
		switch cur.Type {
		case "Package", "Model", "Profile":
			return nil
		case "Class", "Component", "Actor", "Node", "Device", "ExecutionEnvironment", "Interface",
			"Signal", "DataType", "PrimitiveType", "Enumeration", "AssociationClass", "InformationItem", "Stereotype":
			return cur
		}
	}
	return nil
}

// laneFeature returns the feature named n of the lane's object, nil when it has none.
func (m *migration) laneFeature(l *lane, n string) *xmi.Element {
	if l == nil || l.expr == "" || l.typ == nil {
		return nil
	}
	visible, _ := m.membersOf(l.typ, memberAny)
	f := visible[n]
	if f == nil || (f.Type != "Property" && f.Type != "Port") {
		return nil
	}
	return f
}

// lanePerformer is the object, read from the activity's context, that performs
// the behavior a call behavior action n calls: the object n's swimlane represents,
// when it is of the classifier owning the behavior and not the context itself; else "".
func (m *migration) lanePerformer(n *xmi.Element) (obj string, b *xmi.Element) {
	b = m.model.Ref(n, "behavior")
	if b == nil || m.methodOf[b] != nil || !m.written(b) {
		return "", nil
	}
	if cat, _ := m.classify(b); cat != catActionDef {
		return "", nil
	}
	l := m.laneAt(n)
	if l == nil || l.expr == "" || l.expr == "this" || l.typ == nil {
		return "", nil
	}
	owner := classifierOf(b)
	if owner == nil || (l.typ != owner && !m.inherits(l.typ, owner)) {
		return "", nil
	}
	l.used = true
	return l.expr, b
}

// behaviorUsage names the action usage that makes an activity a feature of the
// classifier owning it, as an operation's usage does, so a lane's object can
// perform it; the usage is written when the owner's body is, unless the activity
// is the owner's classifier behavior, whose performance is that usage.
func (m *migration) behaviorUsage(b *xmi.Element) string {
	if name, ok := m.usageOf[b]; ok {
		return name
	}
	owner := classifierOf(b)
	name := m.freshName(owner, lowerFirst(m.nameFor(b)))
	m.usageOf[b] = name
	if m.model.Ref(owner, "classifierBehavior") != b || b.Parent != owner {
		m.extras[owner] = append(m.extras[owner], func() {
			m.w.line("action " + writeName(name) + " : " + m.ref(b, owner) + ";")
		})
	}
	return name
}

// prepareLanes indexes an activity's partitions before anything is written and
// reserves a usage on the owner of every behavior a lane's object performs.
func (m *migration) prepareLanes(act *xmi.Element) {
	m.lanesOf(act)
	for _, n := range act.Owned("node") {
		if n.Type != "CallBehaviorAction" {
			continue
		}
		if _, b := m.lanePerformer(n); b != nil {
			m.behaviorUsage(b)
		}
	}
}
