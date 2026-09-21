package migrate

import (
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// A swimlane (an ActivityPartition) says who performs the nodes it holds: the
// object its `represents` names, a property of the activity's context or a
// classifier. Names in the bodies and guards of those nodes resolve against
// that object first, written as the feature chain the runtime reads it by.

// lane is one partition of an activity with the object it represents resolved.
type lane struct {
	g          *sysmlv1.Element // the ActivityPartition
	parent     *lane            // the partition it is nested in, nil at the top
	represents *sysmlv1.Element // the property or classifier it represents, nil when unset
	typ        *sysmlv1.Element // the classifier whose features names inside resolve against
	expr       string           // how the activity reads the represented object; "" when it cannot
	plural     bool             // whether expr reads a collection of objects rather than one
	note       string           // why expr is "", or how it was found
	used       bool             // whether a name or a call resolved through the lane
}

// use records that a name or a call at e resolved through lane l, and so
// through every other partition holding e that represents the same object.

// The note fragments the writer repeats.
const (
	partRepresents = "the partition represents "
	butNote        = ", but "
)

func (ls *lanes) use(e *sysmlv1.Element, l *lane) {
	l.used = true
	for _, o := range ls.of[e] {
		if o.expr == l.expr {
			o.used = true
		}
	}
}

// lanes indexes the partitions of one activity by the nodes and edges they hold.
type lanes struct {
	all    []*lane
	of     map[*sysmlv1.Element][]*lane // node or edge → every partition holding it
	picked map[*sysmlv1.Element]*lane   // node or edge → the partition its names resolve against
	clash  map[*sysmlv1.Element][]*lane // node or edge → partitions of different dimensions naming different objects
}

// lanesOf indexes the partitions of an activity once, before its nodes are written.
func (m *migration) lanesOf(act *sysmlv1.Element) *lanes {
	if ls, ok := m.lanes[act]; ok {
		return ls
	}
	ls := &lanes{of: map[*sysmlv1.Element][]*lane{}, picked: map[*sysmlv1.Element]*lane{}, clash: map[*sysmlv1.Element][]*lane{}}
	m.lanes[act] = ls
	ctx := m.contextClassifier(act)
	var index func(g *sysmlv1.Element, parent *lane)
	index = func(g *sysmlv1.Element, parent *lane) {
		l := &lane{g: g, parent: parent}
		m.resolveLane(l, ctx)
		ls.all = append(ls.all, l)
		for _, n := range append(m.model.Refs(g, "node"), m.model.Refs(g, "edge")...) {
			ls.hold(n, l)
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
	// A node may name its partitions itself, the same membership seen from its side.
	for _, n := range append(act.Owned("node"), act.Owned("edge")...) {
		for _, g := range m.model.Refs(n, "inPartition") {
			if l := ls.find(g); l != nil {
				ls.hold(n, l)
			}
		}
	}
	return ls
}

// hold records that partition l holds e, once.
func (ls *lanes) hold(e *sysmlv1.Element, l *lane) {
	if !slices.Contains(ls.of[e], l) {
		ls.of[e] = append(ls.of[e], l)
	}
}

// pick settles which of the partitions holding e its names resolve against:
// the innermost when they nest in one chain, else the one naming an object;
// several naming different objects are a clash, and nothing is picked.
func (ls *lanes) pick(e *sysmlv1.Element) *lane {
	if l, ok := ls.picked[e]; ok {
		return l
	}
	held := ls.of[e]
	var inner, named []*lane
	for _, l := range held {
		if !slices.ContainsFunc(held, func(o *lane) bool { return o != l && o.within(l) }) {
			inner = append(inner, l)
		}
	}
	for _, l := range inner {
		if l.expr != "" && !slices.ContainsFunc(named, func(o *lane) bool { return o.expr == l.expr }) {
			named = append(named, l)
		}
	}
	var l *lane
	switch {
	case len(inner) == 1:
		l = inner[0]
	case len(named) == 1:
		l = named[0]
	case len(named) > 1:
		ls.clash[e] = named
	}
	ls.picked[e] = l
	return l
}

// within reports whether l is nested in o, at any depth.
func (l *lane) within(o *lane) bool {
	for p := l.parent; p != nil; p = p.parent {
		if p == o {
			return true
		}
	}
	return false
}

// clashNote says why names in e resolve against no partition: subject, e or
// the node an edge leaves, is in partitions of different dimensions that name
// different objects; "" when not.
func (ls *lanes) clashNote(e *sysmlv1.Element, subject string) string {
	named := ls.clash[e]
	if len(named) == 0 {
		return ""
	}
	parts := make([]string, len(named))
	for i, l := range named {
		parts[i] = describe(l.g) + " (" + l.expr + ")"
	}
	return subject + " is in the partitions " + strings.Join(parts, " and ") + ", which represent different objects, so names resolve through no partition"
}

// clashing lists, for the report of partition l, the nodes it holds together
// with another partition naming a different object.
func (ls *lanes) clashing(l *lane) []string {
	var nodes []string
	for e, named := range ls.clash {
		if slices.Contains(named, l) {
			nodes = append(nodes, describe(e))
		}
	}
	sort.Strings(nodes)
	return nodes
}

// find returns the lane of a partition element.
func (ls *lanes) find(g *sysmlv1.Element) *lane {
	for _, l := range ls.all {
		if l.g == g {
			return l
		}
	}
	return nil
}

// laneOf returns the partition a node or edge of the activity resolves names
// against: the one holding it, else the one holding the node an edge leaves.
func (ls *lanes) laneOf(m *migration, e *sysmlv1.Element) *lane {
	if ls == nil {
		return nil
	}
	h, _ := ls.holder(m, e)
	return ls.pick(h)
}

// holder is the element whose partitions decide e's: e itself when partitions
// hold it or it is a node; an edge in none, the node of its source end when
// partitions hold that, else of its target end; with the end's role, "" for e.
func (ls *lanes) holder(m *migration, e *sysmlv1.Element) (*sysmlv1.Element, string) {
	if len(ls.of[e]) > 0 || (e.Type != "ControlFlow" && e.Type != "ObjectFlow") {
		return e, ""
	}
	for _, role := range []string{"source", "target"} {
		if n := m.model.Ref(e, role); n != nil && len(ls.of[ownerNode(n)]) > 0 {
			return ownerNode(n), role
		}
	}
	return e, ""
}

// resolveLane finds how the activity, whose context object is a ctx, reads
// the object the lane represents: `this` itself, one of its parts, or a part
// of the enclosing lane's object.
func (m *migration) resolveLane(l *lane, ctx *sysmlv1.Element) {
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
		m.laneFeatureObject(l, r, ctx)
		return
	}
	l.typ = r
	switch {
	case !m.written(r):
		l.note = "the classifier " + qualifiedName(r) + " it represents has no v2 declaration"
	case l.parent != nil && l.parent.expr != "" && l.parent.typ != nil && (l.parent.typ == r || m.inherits(l.parent.typ, r)):
		l.expr, l.plural = l.parent.expr, l.parent.plural
		l.note = "the partition represents the enclosing partition's object, a " + qualifiedName(r)
	case ctx == nil:
		l.note = "the activity is in no classifier whose object could be the represented " + qualifiedName(r)
	case ctx == r || m.inherits(ctx, r):
		l.expr = "this"
		l.note = "the partition represents the context object itself, a " + qualifiedName(r)
	default:
		if path, plural, unread := m.partPath(ctx, r); unread != nil {
			l.note = "the partition represents a " + qualifiedName(r) + " through the part " + qualifiedName(unread) + butNote + boundsNote(unread)
		} else if path != "" {
			l.expr, l.plural = "this."+path, plural
			l.note = "the partition represents the context's part " + path + ", a " + qualifiedName(r)
		} else {
			l.note = "no part of " + qualifiedName(ctx) + " is a " + qualifiedName(r) + ", which the partition represents"
		}
	}
	if l.plural {
		l.note += "; it is a collection, so names read through it are collections and are not assigned"
	}
}

// laneFeatureObject resolves a lane representing a property or port: the read
// expression, its type, and a note saying how it was found or why not.
func (m *migration) laneFeatureObject(l *lane, r, ctx *sysmlv1.Element) {
	owner := r.Parent
	l.typ = m.model.Ref(r, "type")
	name := writeName(m.nameOf(r))
	switch {
	case !m.written(r) || m.nameOf(r) == "":
		l.note = "the property " + qualifiedName(r) + " it represents has no v2 declaration"
	case unreadableBounds(r):
		l.note = partRepresents + qualifiedName(r) + butNote + boundsNote(r)
	case l.parent != nil && l.parent.expr != "" && l.parent.typ != nil && m.hasFeature(l.parent.typ, r):
		l.expr = l.parent.expr + "." + name
		l.plural = l.parent.plural || manyValued(r)
		l.note = partRepresents + m.nameOf(r) + " of the enclosing partition's object, read as " + l.expr
	case ctx == nil:
		l.note = "the activity is in no classifier whose object could hold " + qualifiedName(r)
	case m.hasFeature(ctx, r):
		l.expr = "this." + name
		l.plural = manyValued(r)
		l.note = "the partition represents the context's " + m.nameOf(r) + ", read as " + l.expr
	default:
		if path, plural, unread := m.partPath(ctx, owner); unread != nil {
			l.note = partRepresents + qualifiedName(r) + " through the part " + qualifiedName(unread) + butNote + boundsNote(unread)
		} else if path != "" {
			l.expr = "this." + path + "." + name
			l.plural = plural || manyValued(r)
			l.note = partRepresents + qualifiedName(r) + ", read as " + l.expr
		} else {
			l.note = "no part of " + qualifiedName(ctx) + " is a " + qualifiedName(owner) + ", which holds the represented " + m.nameOf(r)
		}
	}
	if l.typ == nil && l.expr != "" {
		l.note += "; the property has no type, so no name resolves through it"
	}
	if l.plural {
		l.note += "; it is a collection, so names read through it are collections and are not assigned"
	}
}

// partPath finds the one chain of composite parts, of any length, from
// classifier c to an object of classifier target; "" when none or several
// exist. plural reports whether any part on the chain holds several objects;
// unread is the first part on it whose multiplicity is not written in numbers.
func (m *migration) partPath(c, target *sysmlv1.Element) (path string, plural bool, unread *sysmlv1.Element) {
	r := m.partRoutes(c, target, map[*sysmlv1.Element]bool{})
	if r.count == 1 {
		return r.path, r.plural, r.unread
	}
	return "", false, nil
}

// partRoute counts the chains of composite parts from one classifier to a
// target, capped at two, and keeps the chain when there is exactly one.
type partRoute struct {
	count  int
	path   string
	plural bool
	unread *sysmlv1.Element // the first part on the chain whose multiplicity is not written in numbers
	cut    bool             // a chain was cut at a classifier already on the walk, so count depends on the walk
}

func (r *partRoute) add(path string, plural bool, unread *sysmlv1.Element) {
	if r.count == 0 {
		r.path, r.plural, r.unread = path, plural, unread
	}
	r.count = min(r.count+1, 2)
}

// partRoutes walks the composite parts of t, memoizing each classifier's
// routes to target once its count cannot depend on how it was reached.
func (m *migration) partRoutes(t, target *sysmlv1.Element, onWalk map[*sysmlv1.Element]bool) partRoute {
	key := [2]*sysmlv1.Element{t, target}
	if r, ok := m.routes[key]; ok {
		return r
	}
	var r partRoute
	onWalk[t] = true
	visible, _ := m.membersOf(t, memberAny)
	names := make([]string, 0, len(visible))
	for name := range visible {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if r.count == 2 {
			break
		}
		p := visible[name]
		if p.Type != "Property" || p.Attrs["aggregation"] != "composite" {
			continue
		}
		pt := m.model.Ref(p, "type")
		if pt == nil || pt.IsProxy() {
			continue
		}
		step, many := writeName(name), manyValued(p)
		var unread *sysmlv1.Element
		if unreadableBounds(p) {
			unread = p
		}
		switch {
		case pt == target || m.inherits(pt, target):
			r.add(step, many, unread)
		case onWalk[pt]:
			r.cut = true
		default:
			sub := m.partRoutes(pt, target, onWalk)
			r.cut = r.cut || sub.cut
			switch sub.count {
			case 1:
				if unread == nil {
					unread = sub.unread
				}
				r.add(step+"."+sub.path, many || sub.plural, unread)
			case 2:
				r.count = 2
			}
		}
	}
	delete(onWalk, t)
	if r.count == 2 {
		r.cut = false
	}
	if !r.cut {
		m.routes[key] = r
	}
	return r
}

// contextClassifier is the classifier whose object is `this` inside e: the
// nearest enclosing one that is not a behavior, nil inside a package.
func (m *migration) contextClassifier(e *sysmlv1.Element) *sysmlv1.Element {
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
func (m *migration) laneFeature(l *lane, n string) *sysmlv1.Element {
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
// when it is of the classifier owning the behavior and not the context itself;
// else "", with why when the lane's object could have performed it but is a collection.
func (m *migration) lanePerformer(n *sysmlv1.Element) (performer *lane, b *sysmlv1.Element, why string) {
	b = m.model.Ref(n, "behavior")
	if b == nil || m.methodOf[b] != nil || !m.written(b) {
		return nil, nil, ""
	}
	if cat, _ := m.classify(b); cat != catActionDef {
		return nil, nil, ""
	}
	l, _ := m.laneAt(n)
	if l == nil || l.expr == "" || l.expr == "this" || l.typ == nil {
		return nil, nil, ""
	}
	owner := classifierOf(b)
	if owner == nil || (l.typ != owner && !m.inherits(l.typ, owner)) {
		return nil, nil, ""
	}
	if l.plural {
		return nil, nil, "its swimlane represents " + l.expr + ", a collection of objects, so none of them performs the call, which runs in the caller's context"
	}
	m.useLane(n, l)
	return l, b, ""
}

// behaviorUsage names the action usage that makes an activity a feature of the
// classifier owning it, as an operation's usage does, so a lane's object can
// perform it; the usage is written when the owner's body is, unless the activity
// is the owner's classifier behavior, whose performance is that usage.
func (m *migration) behaviorUsage(b *sysmlv1.Element) string {
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
func (m *migration) prepareLanes(act *sysmlv1.Element) {
	m.lanesOf(act)
	for _, n := range act.Owned("node") {
		if n.Type != "CallBehaviorAction" {
			continue
		}
		if _, b, _ := m.lanePerformer(n); b != nil {
			m.behaviorUsage(b)
		}
	}
}
