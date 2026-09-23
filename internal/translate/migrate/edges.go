package migrate

import (
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// edgeMember locates the member an edge was written as: the element whose body
// declares it and its name there, "" when the edge was written anonymously.
type edgeMember struct {
	edgePlace
	// keyword is the usage kind the member is declared as: succession, flow,
	// binding, transition, connection, dependency, include, action, and so on.
	keyword string
	// also locates the further members the edge was written as: a transition
	// once per trigger, a dependency once per client–supplier pair.
	also []edgePlace
	// none marks an edge written into its ends' declarations, not as a member.
	none bool
}

// edgePlace is where one member an edge was written as is declared.
type edgePlace struct {
	owner *sysmlv1.Element
	// nest names the anonymous actions the member is declared within, in
	// owner's body, when it is not a direct member.
	nest []string
	name string
}

func (p edgePlace) equal(q edgePlace) bool {
	return p.owner == q.owner && p.name == q.name && slices.Equal(p.nest, q.nest)
}

// places lists where every member the edge was written as is declared, the first first.
func (em edgeMember) places() []edgePlace {
	return append([]edgePlace{em.edgePlace}, em.also...)
}

// nameableEdge reports whether e is a relationship the migrator writes as a
// member of its own, which a view can expose once the member is named.
func nameableEdge(e *sysmlv1.Element) bool {
	switch e.Type {
	case "ControlFlow", "ObjectFlow", "Transition", "Connector", "Dependency", "Abstraction",
		"Realization", "Usage", "Include", "Extend", "Message":
		return true
	}
	return false
}

// planEdges marks the nameable edges some diagram draws. Their writers name the
// members, so the diagram's view can expose them; edges no diagram draws stay as written.
func (m *migration) planEdges() {
	for _, d := range m.model.Diagrams {
		for _, s := range d.Shown {
			if s.Element != nil && nameableEdge(s.Element) {
				m.shownEdges[s.Element] = true
			}
		}
	}
}

// edgeName is the name a shown edge's member is declared under, before it is made
// distinct: the v1 name, else base (spelled from its ends); "" for an edge no diagram draws.
func (m *migration) edgeName(e *sysmlv1.Element, base string) string {
	if !m.shownEdges[e] {
		return ""
	}
	if n := m.nameOf(e); n != "" {
		return n
	}
	return base
}

// spoken is how written text reads when spelled into an edge's name: quotes
// and escapes read, and each qualified name cut to its last segment.
func spoken(written string) string {
	var b strings.Builder
	segment := -1 // where in b the current name's segment starts; -1 outside a name
	for i := 0; i < len(written); i++ {
		switch c := written[i]; {
		case c == '\'':
			if segment < 0 {
				segment = b.Len()
			}
			for i++; i < len(written) && written[i] != '\''; i++ {
				if written[i] == '\\' && i+1 < len(written) {
					i++
				}
				b.WriteByte(written[i])
			}
		case c == ':' && i+1 < len(written) && written[i+1] == ':':
			if segment >= 0 {
				kept := b.String()[:segment]
				b.Reset()
				b.WriteString(kept)
			}
			i++
		case nameByte(c):
			if segment < 0 {
				segment = b.Len()
			}
			b.WriteByte(c)
		default:
			segment = -1
			b.WriteByte(c)
		}
	}
	return b.String()
}

// nameByte reports whether c continues an unquoted name, `$` included as the
// root namespace a qualified name may start from.
func nameByte(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// wroteEdge records the member edge e was written as: a keyword usage in owner's body
// under name, anonymous when name is "". A lone region's members are its owner's.
func (m *migration) wroteEdge(e, owner *sysmlv1.Element, keyword, name string) {
	m.wroteNestedEdge(e, owner, keyword, nil, name)
}

// wroteNestedEdge records the member edge e was written as within the actions
// nest names, declared in owner's body; see wroteEdge.
func (m *migration) wroteNestedEdge(e, owner *sysmlv1.Element, keyword string, nest []string, name string) {
	if _, ok := m.edgeMembers[e]; ok {
		return
	}
	m.edgeMembers[e] = edgeMember{edgePlace: m.edgePlaceIn(owner, nest, name), keyword: keyword}
}

// edgePlaceIn locates a member declared in owner's body within the actions nest
// names; a lone region's members are its owner's, a method's its operation's.
func (m *migration) edgePlaceIn(owner *sysmlv1.Element, nest []string, name string) edgePlace {
	for owner != nil && owner.Type == "Region" && owner.Role == "region" {
		if _, parallel := m.parallel[owner]; parallel {
			break
		}
		owner = owner.Parent
	}
	if op := m.methodOf[owner]; op != nil {
		owner = op
	}
	return edgePlace{owner: owner, nest: nest, name: name}
}

// wroteEdgeAlso records one more member edge e was written as, the first when none
// is recorded yet: a transition per trigger, a dependency per client–supplier pair.
func (m *migration) wroteEdgeAlso(e, owner *sysmlv1.Element, keyword string, nest []string, name string) {
	em, ok := m.edgeMembers[e]
	if !ok {
		m.wroteNestedEdge(e, owner, keyword, nest, name)
		return
	}
	p := m.edgePlaceIn(owner, nest, name)
	if em.none || em.name == "" || name == "" || slices.ContainsFunc(em.places(), p.equal) {
		return
	}
	em.also = append(em.also, p)
	m.edgeMembers[e] = em
}

// wroteNoMember records edge e as written without a member of its own, as an
// initial transition is written as its region's entry.
func (m *migration) wroteNoMember(e *sysmlv1.Element) {
	if _, ok := m.edgeMembers[e]; !ok {
		m.edgeMembers[e] = edgeMember{none: true}
	}
}

// wroteSameEdge records edge e as standing for the member edge first was written as.
func (m *migration) wroteSameEdge(e, first *sysmlv1.Element) {
	if em, ok := m.edgeMembers[first]; ok {
		m.edgeMembers[e] = em
	}
}

// reaches reports whether a qualified name reaches the members of owner's body: the
// top level, a declaration that is written, or a state's or transition's named inline action.
func (m *migration) reaches(owner *sysmlv1.Element) bool {
	return owner == nil || owner.Parent == nil && owner.Type == "Model" || m.written(owner) || m.inlineWritten(owner)
}

// edgePath is the qualified-name segments of the member declared at p.
func (m *migration) edgePath(p edgePlace) []segment {
	path := m.path(p.owner)
	for _, n := range p.nest {
		path = append(path, segment{name: n, feature: true})
	}
	return append(path, segment{name: p.name, feature: true})
}

// edgeTarget is the qualified name a report entry records for edge e's member;
// "" when the edge was written anonymously.
func (m *migration) edgeTarget(e *sysmlv1.Element) string {
	em, ok := m.edgeMembers[e]
	if !ok || em.name == "" {
		return ""
	}
	path := m.edgePath(em.edgePlace)
	segs := make([]string, len(path))
	for i, s := range path {
		segs[i] = s.name
	}
	return m.qualified(segs)
}

// edgeRef writes a reference to edge e's member from inside scope's body as an
// expose names it; "" when the edge has no named member a name reaches.
func (m *migration) edgeRef(e, scope *sysmlv1.Element) string {
	if refs := m.edgeRefs(e, scope); len(refs) > 0 {
		return refs[0]
	}
	return ""
}

// edgeRefs writes a reference to each member edge e was written as that a name
// reaches, from inside scope's body; nil when the edge has no named member one does.
func (m *migration) edgeRefs(e, scope *sysmlv1.Element) []string {
	em, ok := m.edgeMembers[e]
	if !ok || !m.written(e) {
		return nil
	}
	var refs []string
	for _, p := range em.places() {
		if m.reaches(p.owner) {
			refs = append(refs, m.refEdge(p, scope))
		}
	}
	return refs
}

// refEdge writes a reference from inside scope's body to the member declared at p.
func (m *migration) refEdge(p edgePlace, scope *sysmlv1.Element) string {
	path := m.edgePath(p)
	if len(p.nest) == 0 {
		return m.refMember(p.owner, p.name, path, scope, false)
	}
	// The outermost nesting action is owner's member; the rest qualify from it.
	member := len(path) - len(p.nest) - 1
	ref := m.refMember(p.owner, p.nest[0], path[:member+1], scope, false)
	for _, s := range path[member+1:] {
		ref += "::" + writeName(s.name)
	}
	return ref
}

// Reasons a connector route is pinned to a member of the view, or is not.
const (
	routeWritten    = "written"      // pinned to a member the form draws
	routeNotDrawn   = "not drawn"    // the form draws no edge of the member's kind
	routeNotExposed = "not exposed"  // no qualified name reaches the member
	routeUnnamed    = "unnamed"      // the edge was written anonymously
	routeNotWritten = "not written"  // the edge is not migrated
	routeNoMember   = "no v2 member" // the kind is realized by no member of its own
	routeDuplicate  = "duplicate"    // another record's route took the member
	routeDangling   = "dangling"     // the record resolves to no element
)

// routeReasons orders the reasons as a note lists them.
var routeReasons = []string{
	routeWritten, routeNotDrawn, routeNotExposed, routeUnnamed,
	routeNotWritten, routeNoMember, routeDuplicate, routeDangling,
}

// RouteKind counts the routes of one v1 connector kind that share a reason: written,
// not drawn, not exposed, unnamed, not written, no v2 member, duplicate or dangling.
type RouteKind struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// routeKinds tallies routes by v1 kind and reason.
type routeKinds map[RouteKind]int

func (r routeKinds) add(kind, reason string) {
	r[RouteKind{Kind: kind, Reason: reason}]++
}

// sorted lists the tallies by kind, then reason.
func (r routeKinds) sorted() []RouteKind {
	out := make([]RouteKind, 0, len(r))
	for k, n := range r {
		k.Count = n
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Reason < out[j].Reason
	})
	return out
}

// routeKindName is the v1 kind a connector record is tallied under: the type
// the export names, past its namespace, else the element's.
func routeKindName(exported string, el *sysmlv1.Element) string {
	if i := strings.LastIndex(exported, "."); i >= 0 {
		exported = exported[i+1:]
	}
	switch {
	case exported != "":
		return exported
	case el != nil:
		return el.Type
	}
	return "unknown"
}

// routeTarget is every member a route of el is pinned to, as scope's view of
// form f exposes them; or nil and the reason there is none.
func (m *migration) routeTarget(el, scope *sysmlv1.Element, f viewForm) ([]string, string) {
	if nameableEdge(el) {
		em, ok := m.edgeMembers[el]
		switch {
		case !ok:
			return nil, routeNotWritten
		case em.none:
			return nil, routeNoMember
		case em.name == "":
			return nil, routeUnnamed
		case !f.drawsEdge(em.keyword):
			return nil, routeNotDrawn
		case !m.reaches(em.owner):
			return nil, routeNotExposed
		}
		return m.edgeRefs(el, scope), ""
	}
	if m.exposure(el, scope) != "" {
		return nil, routeNotDrawn
	}
	return nil, routeNoMember
}

// viewForm is how a diagram's view is drawn: its Views rendering, the standard view
// definition it specializes ("" for none) and, for a graph, the behavior it draws.
type viewForm struct {
	rendering  string
	definition string
	subject    *sysmlv1.Element
}

// graphDefinitions pairs the words naming a UML behavior diagram family with the
// standard view definition whose graph the family draws.
var graphDefinitions = []struct {
	words      []string
	definition string
}{
	{[]string{"activity diagram"}, "ActionFlowView"},
	{[]string{"state machine diagram", "statechart diagram"}, "StateTransitionView"},
}

// formOf picks how a diagram is drawn from its tool and UML types: a Views rendering when one
// applies, else a behavior diagram's graph when its owner is written as that graph's kind and
// the diagram shows some node or edge of it; a diagram showing none of the graph stays textual.
func (m *migration) formOf(d *sysmlv1.Diagram) viewForm {
	f, _ := m.form(d)
	return f
}

// form is formOf with, for a diagram drawn as textual notation although its kind
// has a graph or sequence rendering, why that rendering does not draw it.
func (m *migration) form(d *sysmlv1.Diagram) (viewForm, string) {
	f := viewForm{rendering: rendering(d)}
	if f.rendering != textualRendering {
		return f, ""
	}
	kind := strings.ToLower(d.Kind + " / " + d.UMLKind)
	for _, g := range graphDefinitions {
		for _, w := range g.words {
			if !strings.Contains(kind, w) {
				continue
			}
			graph := viewForm{rendering: interconnectionRendering, definition: g.definition}
			graph.subject = m.graphSubject(d.Owner, graph)
			view, kind := graphNames(g.definition)
			switch {
			case graph.subject == nil && d.Owner == nil:
				return f, "it has no owner, and " + view + " draws only the graph of " + kind
			case graph.subject == nil:
				return f, "its owner " + kindOf(d.Owner) + " " + qualifiedName(d.Owner) + " is not written as " + kind + ", whose graph " + view + " draws"
			case !m.showsGraph(d, graph):
				return f, "it shows no node or edge of " + m.hostName(m.bodyOf(graph.subject)) + ", whose graph " + view + " draws"
			}
			return graph, ""
		}
	}
	if strings.Contains(kind, "sequence diagram") {
		return f, m.sequenceNote(d.Owner)
	}
	return f, ""
}

// sequenceNote says why a sequence diagram is not drawn as a SequenceView, which draws
// the message flows between the events of occurrence parts: a migrated interaction is a
// scenario of send, accept and call steps, which declares none, or has no v2 form at all.
func (m *migration) sequenceNote(owner *sysmlv1.Element) string {
	const draws = "a SequenceView draws the message flows between the events of occurrence parts"
	for cur := owner; cur != nil; cur = cur.Parent {
		if cur.Type != "Interaction" {
			continue
		}
		if note := m.interactionNote(cur); note != "" {
			return "its Interaction " + qualifiedName(cur) + " has no v2 form: " + note
		}
		return "its Interaction " + qualifiedName(cur) + " is written as a scenario of send, accept and call steps, and " + draws + ", which a scenario does not declare"
	}
	return draws + ", and no Interaction owns the diagram"
}

// graphNames names, article included, a standard view definition and the
// definition kind whose graph it draws.
func graphNames(definition string) (view, kind string) {
	if definition == "StateTransitionView" {
		return "a StateTransitionView", "a state def"
	}
	return "an ActionFlowView", "an action def"
}

// drawsEdge reports whether the form draws a member declared by keyword as an
// edge, so a route pinned to it shapes the drawing.
func (f viewForm) drawsEdge(keyword string) bool {
	switch f.definition {
	case "ActionFlowView":
		return keyword == "succession" || keyword == "flow"
	case "StateTransitionView":
		return keyword == "transition"
	}
	return f.rendering == interconnectionRendering && (keyword == "connection" || keyword == "binding")
}

// showsGraph reports whether d shows a node or edge the graph of form f draws.
func (m *migration) showsGraph(d *sysmlv1.Diagram, f viewForm) bool {
	return slices.ContainsFunc(d.Shown, func(s sysmlv1.ElementRef) bool {
		return s.Element != nil && m.graphDraws(f, s.Element)
	})
}

// graphSubject is the behavior whose graph a view of form f draws: the diagram owner's
// nearest enclosing behavior, when it is written as the definition kind f presents.
func (m *migration) graphSubject(owner *sysmlv1.Element, f viewForm) *sysmlv1.Element {
	want := catActionDef
	if f.definition == "StateTransitionView" {
		want = catStateDef
	}
	for cur := owner; cur != nil; cur = cur.Parent {
		if !isBehavior(cur) {
			continue
		}
		decl := m.exposable(cur)
		if decl == nil {
			return nil
		}
		if cat, _ := m.classify(decl); cat == want {
			return cur
		}
		return nil
	}
	return nil
}

// drawsNode reports whether the graph of form f draws el as a node: an activity node
// other than a pin or parameter node, or a vertex or the state standing for orthogonal regions.
func (m *migration) drawsNode(el *sysmlv1.Element, f viewForm) bool {
	switch f.definition {
	case "ActionFlowView":
		k := nodeKind(el)
		return el.Role == "node" && k != nodePin && k != nodeParam
	case "StateTransitionView":
		return el.Role == "subvertex" || el.Type == "Region" && m.parallel[el] != ""
	}
	return false
}

// drawsInNode reports whether the graph of form f lists el in a node rather than as one:
// a state's inline entry, do or exit action, which the state's node names and no Layout places.
func (m *migration) drawsInNode(el *sysmlv1.Element, f viewForm) bool {
	return f.definition == "StateTransitionView" && el.Parent != nil && el.Parent.Type == "State" && m.inlineWritten(el)
}

// within reports whether e is owner or is owned under it.
func within(e, owner *sysmlv1.Element) bool {
	for cur := e; cur != nil; cur = cur.Parent {
		if cur == owner {
			return true
		}
	}
	return false
}
