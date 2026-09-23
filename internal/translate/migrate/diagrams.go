package migrate

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// viewsPrefix qualifies a rendering of the standard Views package; a member
// named Views on the way out of scope hides the library, and globalViewsPrefix
// then names it from the global namespace.
const (
	viewsPrefix       = "Views::"
	globalViewsPrefix = "$::" + viewsPrefix
)

// renderings pairs the words that name a UML diagram type family with the
// Views library rendering the family is drawn as; the first pair whose word
// the diagram's kind carries wins, and a kind matching none is rendered as text.
var renderings = []struct {
	words     []string
	rendering string
}{
	{[]string{"table", "matrix"}, "asElementTable"},
	{[]string{"internal block", "parametric", "composite structure", "interconnection"}, interconnectionRendering},
	{[]string{"block definition", "class", "package", "object", "component", "deployment", "profile", "structure"}, "asTreeDiagram"},
}

const (
	textualRendering         = "asTextualNotation"
	interconnectionRendering = "asInterconnectionDiagram"
)

// viewDefinitionsPrefix qualifies a standard view definition, like viewsPrefix
// for renderings.
const (
	viewDefinitionsPrefix       = "StandardViewDefinitions::"
	globalViewDefinitionsPrefix = "$::" + viewDefinitionsPrefix
)

// diagramLayoutPrefix qualifies a DiagramLayout annotation, like viewsPrefix
// for renderings; a member named DiagramLayout shadows the library.
const (
	diagramLayoutPrefix       = "DiagramLayout::"
	globalDiagramLayoutPrefix = "$::" + diagramLayoutPrefix
)

// rendering picks the rendering of a diagram from its tool type and the UML
// type it derives from.
func rendering(d *sysmlv1.Diagram) string {
	kind := strings.ToLower(d.Kind + " / " + d.UMLKind)
	for _, r := range renderings {
		for _, w := range r.words {
			if strings.Contains(kind, w) {
				return r.rendering
			}
		}
	}
	return textualRendering
}

// view is one diagram planned as a view usage: where it is written, under
// what name, and how the report accounts for it once it is.
type view struct {
	d *sysmlv1.Diagram
	// host is the element whose body holds the view; nil for the top level.
	host *sysmlv1.Element
	name string
	// note says what of the view's placement is approximated; "" when nothing.
	note string
	// entry is the report row, filled when the view is written.
	entry *Entry
}

// planViews assigns every diagram the body its view is written in and reserves
// its name there, ahead of writing, so references account for the new member;
// and it names every anonymous element a view exposes, so its declaration
// carries the name the expose refers to.
func (m *migration) planViews() {
	for i := range m.model.Diagrams {
		d := &m.model.Diagrams[i]
		v := &view{d: d}
		m.viewOf[d] = v
		var placed bool
		v.host, placed, v.note = m.viewHost(d)
		if !placed {
			v.entry = m.diagramEntry(d, Unmapped, "", v.note)
			continue
		}
		for _, shown := range d.Shown {
			if x := m.exposable(shown.Element); x != nil {
				m.segments(x)
			}
		}
		name := d.Name
		if name == "" {
			name = "diagram"
			v.note = joinNotes(v.note, "the anonymous diagram is named "+name)
		}
		v.name = m.viewName(v.host, name)
		if v.name != name && d.Name != "" {
			v.note = joinNotes(v.note, "written as "+v.name+" since a member of its owner is also named "+name)
		}
		where := "view '" + v.name + "'"
		if d.Owner != nil {
			where += " in " + qualifiedName(d.Owner)
		}
		for _, shown := range d.Shown {
			if x := m.exposable(shown.Element); x != nil && x.Parent != v.host {
				m.expose(x, where+" exposes it")
			}
		}
		m.hosted[v.host] = append(m.hosted[v.host], v)
	}
}

// viewName reserves the name a view takes in host's body, or at the top level
// for a nil host: name, or name with a number when a member of the body has it
// — the vertices a state def writes from its regions and the members of an
// operation's method included.
func (m *migration) viewName(host *sysmlv1.Element, name string) string {
	var used map[string]bool
	var method *sysmlv1.Element
	if host != nil {
		if host.Type == "StateMachine" {
			used = m.nameMachine(host)
		}
		method = m.bodyMethod(host)
	}
	taken := func(name string) bool {
		return used[name] || m.nameTaken(host, name) || method != nil && m.nameTaken(method, name)
	}
	base := name
	for i := 2; taken(name); i++ {
		name = fmt.Sprintf("%s %d", base, i)
	}
	if used != nil {
		used[name] = true
	}
	m.take(host, name)
	return name
}

// viewHost finds the element whose body a diagram's view is written in: its
// owner when that hosts views — the operation, for a behavior written as the
// operation's method — else the owner's nearest ancestor that does; for a
// diagram naming no owner the element whose extension holds it, and failing
// that the document's top level. The top level is a nil host; the root model,
// whose members are written there, is never one. note says why the host is not
// the owner; placed is false when nothing written can hold the view.
func (m *migration) viewHost(d *sysmlv1.Diagram) (host *sysmlv1.Element, placed bool, note string) {
	from := m.viewOwner(d)
	switch {
	case d.Owner != nil:
	case d.OwnerID != "":
		note = "ownerOfDiagram " + d.OwnerID + " resolves to no element"
		from = d.Holder
	default:
		note = "the diagram names no owner"
		from = d.Holder
	}
	if d.Owner == nil && from == nil {
		return nil, true, joinNotes(note, m.writtenIn(nil))
	}
	for cur := from; cur != nil; cur = cur.Parent {
		host := m.bodyOf(cur)
		if !m.hostsViews(host) {
			continue
		}
		if m.flattened(host) {
			host = nil
		}
		switch {
		case cur == from && d.Owner != nil:
		case d.Owner != nil:
			note = "its owner " + kindOf(d.Owner) + " " + qualifiedName(d.Owner) + " has no v2 body; " + m.writtenIn(host)
		default:
			note = joinNotes(note, m.writtenIn(host)+", which holds it")
		}
		return host, true, note
	}
	if d.Owner != nil {
		return nil, false, "neither its owner " + kindOf(d.Owner) + " " + qualifiedName(d.Owner) + " nor any ancestor of it is written"
	}
	return nil, false, joinNotes(note, "and neither "+kindOf(from)+" "+qualifiedName(from)+", which holds it, nor any ancestor of it is written")
}

// viewOwner is the element whose v2 body stands for a diagram's owner.
func (m *migration) viewOwner(d *sysmlv1.Diagram) *sysmlv1.Element {
	return m.bodyOf(d.Owner)
}

// bodyOf is the element whose v2 body stands for e: the operation a method
// behavior is written as the body of, else e itself.
func (m *migration) bodyOf(e *sysmlv1.Element) *sysmlv1.Element {
	if op := m.methodOf[e]; op != nil {
		return op
	}
	return e
}

// hostsViews reports whether e is written with a body a view can be a member of: the
// root model, or a declared package or definition (not an enum def, node, vertex or region).
func (m *migration) hostsViews(e *sysmlv1.Element) bool {
	if m.flattened(e) {
		return true
	}
	switch e.Type {
	case "Property", "Port", "Parameter", "EnumerationLiteral", "Region":
		return false
	}
	if m.methodOf[e] != nil || isActionNode(e) || vertexBase(e) != "" || !m.written(e) {
		return false
	}
	cat, _ := m.classify(e)
	return cat != catEnumDef
}

// hostName describes a host for a note: its v2 declaration, or the top level.
func (m *migration) hostName(host *sysmlv1.Element) string {
	if host == nil {
		return "the top level"
	}
	cat, _ := m.classify(host)
	return cat.keyword() + " " + m.v2Name(host)
}

// writtenIn says where a view is written, for a note.
func (m *migration) writtenIn(host *sysmlv1.Element) string {
	if host == nil {
		return "written at the top level"
	}
	return "written in " + m.hostName(host)
}

// isTopLevel reports whether e is the root model, whose members are the
// document's top level.
func isTopLevel(e *sysmlv1.Element) bool {
	return e.Parent == nil && e.Type == "Model"
}

// views leaves room for the views of the diagrams host owns (nil: the top level's), written
// once the whole model is, so they can expose the edge members the writers name along the way.
func (m *migration) views(host *sysmlv1.Element) {
	for _, v := range m.hosted[host] {
		m.w.hole(func() { m.writeView(v) })
	}
}

// diagramViewNote says why an «Expose» of a diagram cannot name its view: no
// body takes the view; "" when the view is written.
func (m *migration) diagramViewNote(d *sysmlv1.Diagram) string {
	if _, placed, note := m.viewHost(d); !placed {
		return "the exposed Diagram '" + d.Name + "' is not written as a view: " + note
	}
	return ""
}

// viewRef writes a reference to a diagram's view from inside scope's body: its
// name alone where that resolves to it, else its name qualified under its
// host's, as memberRef writes the host.
func (m *migration) viewRef(v *view, scope *sysmlv1.Element) string {
	name := writeName(v.name)
	chain := scopeChain(scope)
	host := v.host
	for i, s := range chain {
		if s == host {
			if !m.shadows(chain[:i], v.name) {
				return name
			}
			break
		}
	}
	if host == nil {
		if m.shadows(chain, v.name) {
			return "$::" + name
		}
		return name
	}
	return m.memberRef(host, scope) + "::" + name
}

// shadows reports whether one of scopes declares a member named name.
func (m *migration) shadows(scopes []*sysmlv1.Element, name string) bool {
	for _, s := range scopes {
		if m.nameTaken(s, name) {
			return true
		}
	}
	return false
}

// exposures sorts what a diagram shows into the elements its view exposes,
// deduplicated in shown order, and the counts of what it cannot.
type exposures struct {
	refs []string
	// drawn counts shown members of the form's subject, drawn by its graph.
	drawn int
	// unwritten counts shown elements nothing written stands for.
	unwritten int
	// dangling counts shown ids that resolve to no element.
	dangling int
}

// exposed reports whether the view exposes what ref names.
func (x exposures) exposed(ref string) bool {
	for _, r := range x.refs {
		if r == ref {
			return true
		}
	}
	return false
}

// inGraph reports whether el is part of the graph a view of form f draws: within
// the form's subject, and not the subject itself.
func inGraph(f viewForm, el *sysmlv1.Element) bool {
	return f.subject != nil && el != f.subject && within(el, f.subject)
}

// graphPins reports whether the graph a view of form f draws shows el as a node or
// edge of its own, which a Layout or Route pins: one of the form's subject, drawn.
func (m *migration) graphPins(f viewForm, el *sysmlv1.Element) bool {
	return inGraph(f, el) && (m.drawsNode(el, f) || m.drawsAsEdge(f, el))
}

// graphDraws reports whether the graph a view of form f draws shows el for the
// view: a node or edge of the subject it pins, or an action a state's node lists.
func (m *migration) graphDraws(f viewForm, el *sysmlv1.Element) bool {
	return m.graphPins(f, el) || inGraph(f, el) && m.drawsInNode(el, f)
}

// draws reports whether a view of form f exposing x shows el, which ref names, where
// geometry can pin it: an element it exposes, or a node or edge its subject's graph draws.
func (m *migration) draws(x exposures, f viewForm, el *sysmlv1.Element, ref string) bool {
	return x.exposed(ref) || m.graphPins(f, el)
}

// drawsAsEdge reports whether the rendering of form f draws el as an edge, which no
// Layout positions: Cameo places an internal transition as text in its state's box.
func (m *migration) drawsAsEdge(f viewForm, el *sysmlv1.Element) bool {
	em, ok := m.edgeMembers[el]
	return ok && em.name != "" && f.drawsEdge(em.keyword)
}

// places reports whether a view of form f exposing x shows el, which ref names, as a
// node a Layout positions: an element it exposes or a node of its subject's graph.
func (m *migration) places(x exposures, f viewForm, el *sysmlv1.Element, ref string) bool {
	return !m.drawsAsEdge(f, el) && (x.exposed(ref) || inGraph(f, el) && m.drawsNode(el, f))
}

// writeView writes a diagram as a view usage exposing each shown element the
// document writes, rendered by the diagram's kind, and records its report row.
func (m *migration) writeView(v *view) {
	d, host := v.d, v.host
	form := m.formOf(d)
	x := m.exposures(d, host, form)
	render := form.rendering
	kind := diagramKind(d)
	note := article(strings.ToLower(kind)) + kind + " written as a view rendered " + render
	decl := "view " + writeName(v.name)
	if form.definition != "" {
		note += " of " + form.definition
		prefix := viewDefinitionsPrefix
		if m.shadowsLibrary("StandardViewDefinitions", host) {
			prefix = globalViewDefinitionsPrefix
		}
		decl += " : " + prefix + form.definition
	}
	if host != d.Owner && host == m.viewOwner(d) {
		note = joinNotes(note, "its owner "+kindOf(d.Owner)+" "+qualifiedName(d.Owner)+" is written as the body of "+m.hostName(host)+", whose method it is")
	}
	if form.subject != nil {
		subject := "the view exposes " + m.hostName(m.bodyOf(form.subject)) + ", whose graph the rendering draws"
		if x.drawn > 0 {
			subject += fmt.Sprintf(" with the %d shown nodes and edges of it", x.drawn)
		}
		note = joinNotes(note, subject)
	}
	note = joinNotes(note, v.note)
	prefix := viewsPrefix
	if m.shadowsLibrary("Views", host) {
		prefix = globalViewsPrefix
	}
	shown := len(d.Shown)
	untyped := d.Kind == "" && d.UMLKind == ""
	switch {
	case !d.Represented():
		note = joinNotes(note, "no diagram representation is serialized: what the diagram is and shows is unknown, and the view exposes nothing")
	case shown == 0:
		note = joinNotes(note, "the diagram shows nothing; the view exposes nothing")
	case len(x.refs) == 0:
		note = joinNotes(note, "none of the "+strconv.Itoa(shown)+" shown elements is written; the view exposes nothing")
	}
	if untyped && d.Represented() {
		note = joinNotes(note, "the representation names no diagram type")
	}
	if x.unwritten > 0 {
		note = joinNotes(note, fmt.Sprintf("%d of %d shown elements are not written and not exposed", x.unwritten, shown))
	}
	if x.dangling > 0 {
		note = joinNotes(note, fmt.Sprintf("%d of %d shown ids resolve to no element", x.dangling, shown))
	}
	geo := m.viewGeometry(v, x, form)
	if geo.note != "" {
		note = joinNotes(note, geo.note)
	}
	m.w.block(decl, func() {
		for _, ref := range x.refs {
			m.w.line("expose " + ref + ";")
		}
		for _, line := range geo.lines {
			m.w.line(line)
		}
		m.w.line("render " + prefix + render + ";")
	})
	verdict := Mapped
	if v.note != "" || untyped || shown == 0 || len(x.refs) == 0 || x.unwritten+x.dangling > 0 {
		verdict = Approximated
	}
	v.entry = m.diagramEntry(d, verdict, m.qualified(append(m.segments(host), v.name)), note)
}

// exposures resolves what a diagram shows into expose references, in the
// scope of the view's host. A view of a form drawing a graph exposes the
// behavior whose graph it is in place of the shown nodes and edges it draws.
func (m *migration) exposures(d *sysmlv1.Diagram, host *sysmlv1.Element, form viewForm) exposures {
	var x exposures
	seen := map[string]bool{}
	add := func(ref string) {
		if !seen[ref] {
			seen[ref] = true
			x.refs = append(x.refs, ref)
		}
	}
	if form.subject != nil {
		add(m.exposure(form.subject, host))
	}
	for _, shown := range d.Shown {
		if shown.Element == nil {
			x.dangling++
			continue
		}
		ref := m.exposure(shown.Element, host)
		switch {
		case m.graphDraws(form, shown.Element):
			x.drawn++
		case ref == "":
			x.unwritten++
		default:
			add(ref)
		}
	}
	return x
}

// exposure names, from scope, what stands for e: the library type a primitive
// maps to, or the declaration exposable names. It is "" when nothing does.
func (m *migration) exposure(e, scope *sysmlv1.Element) string {
	if sv := m.scalarValue(e); sv != "" {
		if m.shadowsLibrary("ScalarValues", scope) {
			return "$::" + scalarValuesPrefix + sv
		}
		return scalarValuesPrefix + sv
	}
	if link := m.actorLinkOf(e); link != nil {
		return m.memberRef(link.useCase, scope) + "::" + writeName(link.name)
	}
	if ref := m.edgeRef(e, scope); ref != "" {
		return ref
	}
	if x := m.exposable(e); x != nil {
		return m.memberRef(x, scope)
	}
	return ""
}

// actorLinkOf is the actor usage that stands for e: an anonymous association to
// an actor, or its end at the actor; nil when e is written by itself.
func (m *migration) actorLinkOf(e *sysmlv1.Element) *actorLink {
	if e == nil {
		return nil
	}
	assoc := e
	if e.Type == "Property" && e.Parent != nil && e.Parent.Type == "Association" {
		assoc = e.Parent
	}
	link := m.actors[assoc]
	if link == nil || assoc.Name != "" || assoc != e && link.end != e {
		return nil
	}
	return link
}

// exposable is the declaration written for e that a view can expose: its own, the
// operation a behavior is the method of, or a state's named inline action; nil for none.
func (m *migration) exposable(e *sysmlv1.Element) *sysmlv1.Element {
	if e == nil || m.scalarValue(e) != "" {
		return nil
	}
	if op := m.methodOf[e]; op != nil {
		e = op
	}
	if op := m.realizes[e]; op != nil {
		e = op
	}
	if !m.written(e) && !m.inlineWritten(e) {
		return nil
	}
	return e
}

// shadowsLibrary reports whether a member named like a standard library
// package hides it from scope: one of a scope on the chain, or a top-level one.
func (m *migration) shadowsLibrary(lib string, scope *sysmlv1.Element) bool {
	return m.shadows(scopeChain(scope), lib) || m.nameTaken(nil, lib)
}

// diagramEntry builds the report row of a diagram: named under its owner, as
// the elements are, and kept aside until the diagrams are reported together.
func (m *migration) diagramEntry(d *sysmlv1.Diagram, v Verdict, target, note string) *Entry {
	name := d.Name
	if name == "" {
		name = "<Diagram>"
	}
	if d.Owner != nil && d.Owner.Parent != nil {
		name = qualifiedName(d.Owner) + "::" + name
	}
	return &Entry{ID: d.ID, Kind: "Diagram", Name: name, Target: target, Verdict: v, Note: note}
}

// diagramKind names what kind of diagram d is, as the tool typed it.
func diagramKind(d *sysmlv1.Diagram) string {
	switch {
	case d.Kind != "":
		return d.Kind
	case d.UMLKind != "":
		return d.UMLKind
	}
	return "diagram of unknown kind"
}

// diagrams reports every diagram in document order once the model is written:
// as its view, or as unmapped, with a comment at the top level, when no body
// took its view.
func (m *migration) diagrams() {
	for i := range m.model.Diagrams {
		d := &m.model.Diagrams[i]
		v := m.viewOf[d]
		if v.entry == nil {
			v.entry = m.diagramEntry(d, Unmapped, "", m.hostName(v.host)+" is written without a body its view could be a member of")
		}
		if v.entry.Verdict == Unmapped {
			name := d.Name
			if name == "" {
				name = "(" + d.ID + ")"
			} else {
				name = "'" + name + "'"
			}
			m.w.lines(commentLines("not migrated: " + v.entry.Kind + " " + name + " — " + v.entry.Note))
		}
		m.report.Entries = append(m.report.Entries, *v.entry)
	}
}

// viewGeometry is the layout a matched MTIP diagram record writes into a view:
// the annotation lines and the report clause describing them.
type viewGeometry struct {
	lines []string
	note  string
}

// viewGeometry plans the DiagramLayout annotations of the diagram record the
// export holds for v's diagram: a Canvas sized by the bounding box of what is
// written, a Layout per shown element the view exposes, and a Route per
// connector whose element does, tallied by the connector's v1 kind and why it
// is or is not pinned. nil geometry when there is no layout export.
func (m *migration) viewGeometry(v *view, x exposures, form viewForm) viewGeometry {
	if m.layout == nil {
		return viewGeometry{}
	}
	rec := m.layoutByID[v.d.ID]
	s := m.layoutSummary
	if rec == nil {
		s.ViewsWithoutLayout++
		return viewGeometry{}
	}
	s.DiagramsJoined++
	m.layoutJoined[rec.ID] = true
	prefix := diagramLayoutPrefix
	if m.shadowsLibrary("DiagramLayout", v.host) {
		prefix = globalDiagramLayoutPrefix
	}
	var placements, routes []string
	var maxX, maxY float64
	var size bool
	grow := func(w, h float64) {
		size = true
		if w > maxX {
			maxX = w
		}
		if h > maxY {
			maxY = h
		}
	}
	var written, unexposed, dangling int
	seen := map[string]bool{}
	for _, p := range rec.Placements {
		s.Placements++
		el := m.model.Lookup(p.ID)
		if el == nil {
			s.PlacementsDangling++
			dangling++
			continue
		}
		ref := m.exposure(el, v.host)
		if ref == "" || !m.places(x, form, el, ref) {
			s.PlacementsUnexposed++
			unexposed++
			continue
		}
		if seen[ref] {
			continue
		}
		seen[ref] = true
		s.PlacementsWritten++
		written++
		placements = append(placements, fmt.Sprintf("metadata %sLayout about %s { x = %s; y = %s; width = %s; height = %s; }",
			prefix, ref, layoutNumber(p.X), layoutNumber(p.Y), layoutNumber(p.Width), layoutNumber(p.Height)))
		grow(p.X+p.Width, p.Y+p.Height)
	}
	reasons := map[string]int{}
	pinned := map[string]bool{}
	for _, c := range rec.Connectors {
		s.Routes++
		el := m.model.Lookup(c.ID)
		if el == nil {
			s.RoutesDangling++
			reasons[routeDangling]++
			m.routeKinds.add(routeKindName(c.Type, nil), routeDangling)
			continue
		}
		kind := routeKindName(c.Type, el)
		ref, why := m.routeTarget(el, v.host, form)
		switch {
		case why != "":
		case !m.draws(x, form, el, ref):
			why = routeNotExposed
		case pinned[ref]:
			why = routeDuplicate
		}
		if why != "" {
			s.RoutesUnexposed++
			reasons[why]++
			m.routeKinds.add(kind, why)
			continue
		}
		pinned[ref] = true
		s.RoutesWritten++
		reasons[routeWritten]++
		m.routeKinds.add(kind, routeWritten)
		var b strings.Builder
		b.WriteString("metadata " + prefix + "Route about " + ref + " { points = (")
		for i, n := range c.Points {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(layoutNumber(n))
			if i%2 == 0 {
				grow(n, maxY)
			} else {
				grow(maxX, n)
			}
		}
		b.WriteString("); }")
		routes = append(routes, b.String())
	}
	for tag, n := range rec.Unsupported {
		s.Unsupported[tag] += n
	}
	var geo viewGeometry
	if size {
		geo.lines = append(geo.lines, fmt.Sprintf("@%sCanvas { unit = \"px\"; width = %s; height = %s; }",
			prefix, layoutNumber(maxX), layoutNumber(maxY)))
	}
	geo.lines = append(geo.lines, placements...)
	geo.lines = append(geo.lines, routes...)
	if len(rec.Placements)+len(rec.Connectors) > 0 {
		geo.note = fmt.Sprintf("laid out from %s: %s", m.layoutSource, layoutClause(written, unexposed, dangling, len(rec.Placements))+", "+routeClause(reasons, len(rec.Connectors)))
	}
	return geo
}

// layoutClause words the placements of the layout note: how many of the
// record's shown elements were positioned, and why the rest were not.
func layoutClause(written, unexposed, dangling, total int) string {
	clause := fmt.Sprintf("%d of %d shown elements positioned", written, total)
	var parts []string
	if unexposed > 0 {
		parts = append(parts, fmt.Sprintf("%d not exposed", unexposed))
	}
	if dangling > 0 {
		parts = append(parts, fmt.Sprintf("%d resolving to no element", dangling))
	}
	if len(parts) > 0 {
		clause += " (" + strings.Join(parts, ", ") + ")"
	}
	return clause
}

// routeClause words the routes of the layout note: how many of the record's
// connectors were routed, and why the rest were not, a reason at a time.
func routeClause(reasons map[string]int, total int) string {
	clause := fmt.Sprintf("%d of %d connectors routed", reasons[routeWritten], total)
	var parts []string
	for _, why := range routeReasons {
		n := reasons[why]
		switch {
		case n == 0 || why == routeWritten:
		case why == routeDangling:
			parts = append(parts, fmt.Sprintf("%d resolving to no element", n))
		default:
			parts = append(parts, fmt.Sprintf("%d %s", n, why))
		}
	}
	if len(parts) > 0 {
		clause += " (" + strings.Join(parts, ", ") + ")"
	}
	return clause
}

// layoutNumber writes a coordinate in its shortest exact form.
func layoutNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// layoutReport finishes the report's layout account: a row per export record
// that matched no written view — no diagram of the model, or one the migration
// does not write a view for — and per malformed record, then the summary itself.
func (m *migration) layoutReport() {
	if m.layout == nil {
		return
	}
	s := m.layoutSummary
	for i := range m.layout.Diagrams {
		rec := &m.layout.Diagrams[i]
		switch {
		case !m.diagramIDs[rec.ID]:
			s.DiagramsUnmatched++
			m.report.Entries = append(m.report.Entries, Entry{ID: rec.ID, Kind: "Layout", Name: rec.Name, Verdict: Unmapped, Note: "matches no diagram of the model"})
		case !m.layoutJoined[rec.ID]:
			s.DiagramsUnmatched++
			m.report.Entries = append(m.report.Entries, Entry{ID: rec.ID, Kind: "Layout", Name: rec.Name, Verdict: Unmapped, Note: "matches a diagram the migration does not write as a view"})
		}
		for _, problem := range rec.Malformed {
			m.report.Entries = append(m.report.Entries, Entry{ID: rec.ID, Kind: "Layout", Name: rec.Name, Verdict: Unmapped, Note: problem})
		}
	}
	s.RoutesByKind = m.routeKinds.sorted()
	m.report.Layout = s
}
