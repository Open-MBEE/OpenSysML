package migrate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/mtip"
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
	{[]string{"table", "matrix"}, tableRendering},
	{[]string{"internal block", "parametric", "composite structure", "interconnection"}, interconnectionRendering},
	{[]string{"block definition", "class", "package", "object", "component", "deployment", "profile", "structure"}, treeRendering},
}

const (
	textualRendering         = "asTextualNotation"
	interconnectionRendering = "asInterconnectionDiagram"
	treeRendering            = "asTreeDiagram"
	tableRendering           = "asElementTable"
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
	// placed is false when nothing written can hold the view.
	placed bool
	name   string
	// note says what of the view's placement is approximated; "" when nothing.
	note string
	// entry is the report row, filled when the view is written.
	entry *Entry
	// tables are the table definitions the diagram carries, written beside
	// the view in definition order; empty for a diagram that defines none.
	tables []*tableDoc
	// rebuilt lists what the view exposes in place of rows the tool never
	// stored; nil for a diagram whose content the tool serialized.
	rebuilt *reconstruction
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
		v.host, v.placed, v.note = m.viewHost(d)
		if !v.placed {
			v.entry = m.diagramEntry(d, Unmapped, "", v.note)
			continue
		}
		for _, shown := range d.Shown {
			if x := m.exposable(shown.Element); x != nil {
				m.segments(x)
			}
		}
		m.planMonteCarlo(d)
		name := d.Name
		if name == "" {
			name = "diagram"
			v.note = joinNotes(v.note, "the anonymous diagram is named "+name)
		}
		v.name = m.viewName(v.host, name)
		if v.name != name && d.Name != "" {
			v.note = joinNotes(v.note, writtenAs+v.name+" since a member of its owner is also named "+name)
		}
		where := "view '" + v.name + "'"
		if d.Owner != nil {
			where += " in " + qualifiedName(d.Owner)
		}
		form := m.formOf(d)
		for _, shown := range d.Shown {
			if !symbolized(form, shown) {
				continue
			}
			if x := m.exposable(shown.Element); x != nil && x.Parent != v.host {
				m.expose(x, where+" exposes it")
			}
		}
		m.hosted[v.host] = append(m.hosted[v.host], v)
	}
	m.planPrivateLayoutAliasHoles()
	m.planTables()
	m.planReconstructions()
	m.planDocuments()
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
// that the document's top level, a nil host; a root Model hosts views as any
// package does. note says why the host is not
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

// hostsViews reports whether e is written with a body a view can be a member of: a
// declared package or definition (not an enum def, node, vertex or region).
func (m *migration) hostsViews(e *sysmlv1.Element) bool {
	switch e.Type {
	case "Property", "Port", "Parameter", "EnumerationLiteral", "Region":
		return false
	}
	if m.methodOf[e] != nil || m.isActionNode(e) || vertexBase(e) != "" || !m.written(e) {
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
	kw := cat.keyword()
	if m.asUsage[host] {
		kw = "action"
	}
	return kw + " " + m.v2Name(host)
}

// writtenIn says where a view is written, for a note.
func (m *migration) writtenIn(host *sysmlv1.Element) string {
	if host == nil {
		return "written at the top level"
	}
	return "written in " + m.hostName(host)
}

// views leaves room for the views of the diagrams host owns (nil: the top level's), written
// once the whole model is, so they can expose the edge members the writers name along the way.
func (m *migration) views(host *sysmlv1.Element) {
	for _, v := range m.hosted[host] {
		m.w.hole(func() { m.writeView(v) })
	}
	if m.privateAliasOwners[host] && !m.privateAliasHoles[host] {
		m.w.holeLast(func() { m.writePrivateLayoutAliases(host) })
		m.privateAliasHoles[host] = true
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
			if !m.hidden(v.name) && !m.shadows(chain[:i], v.name) {
				m.refer(chain[:i], v.name)
				return name
			}
			break
		}
	}
	if host == nil {
		if m.hidden(v.name) || m.shadows(chain, v.name) {
			return "$::" + name
		}
		m.refer(chain, v.name)
		return name
	}
	return m.memberRef(host, scope) + "::" + name
}

// viewSteps is the feature chain from the namespace above v's outermost usage
// down to v; def is that namespace when a definition, why says why no chain reaches v.
func (m *migration) viewSteps(v *view) (def *sysmlv1.Element, steps []segment, why string) {
	path := m.path(v.host)
	i := len(path)
	for i > 0 && path[i-1].feature {
		i--
	}
	steps = append(append(steps, path[i:]...), segment{name: v.name, feature: true})
	if i == 0 {
		return nil, steps, ""
	}
	ns := path[i-1]
	if ns.elem != nil {
		switch cat, _ := m.classify(ns.elem); {
		case cat == catPackage:
			return nil, steps, ""
		case strings.HasSuffix(cat.keyword(), " def"):
			return ns.elem, steps, ""
		}
	}
	return nil, nil, "the view '" + v.name + "' is a member of " + m.qualified(m.segments(v.host)) + ", which no feature reaches"
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
	refs  []string
	names map[string]bool
	// by lists the refs importing each name, in shown order.
	by map[string][]string
	// drawn counts shown members of the form's subject, drawn by its graph.
	drawn int
	// unwritten counts shown elements nothing written stands for.
	unwritten int
	// dangling counts shown ids that resolve to no element.
	dangling int
	// symbolless counts shown elements with no symbol of their own, drawn by an
	// exposed element's symbol: compartment rows and end labels on lines.
	symbolless int
}

// symbolized reports whether a view of form f exposes shown: a tree exposes only what
// a symbol of its own stands for, not a compartment row or an end label on a line.
func symbolized(f viewForm, shown sysmlv1.ElementRef) bool {
	return f.rendering != treeRendering || !shown.Listed && !shown.OnPath
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

// shadows reports whether a view exposure uses name.
func (x exposures) shadows(name string) bool {
	return x.names[name]
}

// hides lists the exposed names a reference written in the view's body cannot
// use as the host would: all but a name one expose imports by its simple
// spelling, which denotes inside the view what it does outside it.
func (x exposures) hides() columnNames {
	hidden := columnNames{}
	for name := range x.names {
		if refs := x.by[name]; len(refs) == 1 && refs[0] == writeName(name) {
			continue
		}
		hidden[name] = true
	}
	return hidden
}

// diagramLayoutAttribute qualifies an attribute when its name is shadowed in the view.
func diagramLayoutAttribute(prefix, definition, name, value string, x exposures) string {
	return diagramLayoutAttributeText(prefix, definition, name, value, x.shadows(name))
}

// diagramLayoutAttributeForHost also qualifies an attribute shadowed by its containing scopes.
func (m *migration) diagramLayoutAttributeForHost(prefix, definition, name, value string, host *sysmlv1.Element, x exposures) string {
	return diagramLayoutAttributeText(prefix, definition, name, value, x.shadows(name) || m.shadows(scopeChain(host), name))
}

// diagramLayoutFeatureForHost names a feature redefined in an annotation body,
// qualified when the view or its containing scopes shadow its name.
func (m *migration) diagramLayoutFeatureForHost(prefix, definition, name string, host *sysmlv1.Element, x exposures) string {
	if x.shadows(name) || m.shadows(scopeChain(host), name) {
		return prefix + definition + "::" + name
	}
	return name
}

func diagramLayoutAttributeText(prefix, definition, name, value string, shadowed bool) string {
	if shadowed {
		return ":>> " + prefix + definition + "::" + name + " = " + value
	}
	return name + " = " + value
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
	return m.graphPins(f, el) || inGraph(f, el) && (m.drawsInNode(el, f) || m.drawsEntry(f, el))
}

// drawsEntry reports whether the graph of form f draws el as the edge from its start:
// an initial transition or flow written into its region or activity, not as a member.
func (m *migration) drawsEntry(f viewForm, el *sysmlv1.Element) bool {
	em, ok := m.edgeMembers[el]
	if !ok || !em.none {
		return false
	}
	switch f.definition {
	case "StateTransitionView":
		return el.Type == "Transition"
	case "ActionFlowView":
		return el.Role == "edge"
	}
	return false
}

// draws reports whether a view of form f exposing x shows el, which ref names, where
// geometry can pin it: an element it exposes, or a node or edge its subject's graph draws.
func (m *migration) draws(x exposures, f viewForm, host, el *sysmlv1.Element, ref string) bool {
	if prop := m.backedProperty(el); prop != nil {
		// A tree draws an attribute's typing edge between its owner's and its type's
		// boxes; a private attribute is out of the view's reach.
		return !m.writtenHidden(prop) && x.exposed(m.exposure(prop.Parent, host)) && x.exposed(m.exposure(m.model.Ref(prop, "type"), host))
	}
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
	if f.useCases && m.connOf(el) != nil {
		return false
	}
	return !m.drawsAsEdge(f, el) && (x.exposed(ref) || inGraph(f, el) && m.drawsNode(el, f))
}

// emptyView says why a view of form f draws nothing: its diagram shows no
// element the view exposes and carries no pasted image the view draws; ""
// when the rendering has something to draw.
func (m *migration) emptyView(v *view, f viewForm) string {
	d := v.d
	if len(m.exposures(d, v.host, f).refs) > 0 {
		return ""
	}
	pictured := len(m.pastedPictures(d).drawn)
	if pictured > 0 && f.drawsPictures() {
		return ""
	}
	for _, td := range v.tables {
		m.lowerTable(td)
		if td.written() {
			return ""
		}
	}
	switch {
	case !d.Represented():
		return "no diagram representation is serialized, so what it shows is unknown and its view exposes nothing"
	case v.rebuilt != nil:
		return v.rebuilt.note(m, d.Owner, v.host)
	case len(d.Shown) == 0 && pictured > 0:
		return "it shows no model element, only " + plural(pictured, pastedImage) + " a view rendered " + f.rendering + " does not draw, and its view exposes nothing"
	case len(d.Shown) == 0:
		return "it " + showsNothing(d, "shows no model element") + ", and its view exposes nothing"
	}
	return "none of the " + strconv.Itoa(len(d.Shown)) + " elements it shows is written, and its view exposes nothing"
}

// showsNothing predicates what a diagram showing no model element draws, as far
// as its serialization tells: nothing at all, or free symbols only, which stand
// for no element and so no view exposes; unread tells only what the tool listed.
func showsNothing(d *sysmlv1.Diagram, unread string) string {
	switch {
	case !d.Drawn:
		return unread
	case len(d.Free) == 0:
		return "draws nothing at all"
	}
	return "shows no model element, only " + freeSymbols(d.Free) + " standing for none"
}

// freeSymbolNouns names the tool's symbol classes that stand for no element.
var freeSymbolNouns = map[string]string{
	"ImageShape": "image", "TextBox": "text box", "TextBoxWithIcon": "text box",
	"Note": "note", "NoteAnchor": "note anchor", "RectangularShape": "rectangle",
	"Separator": "separator", "Swimlane": "swimlane", "ContainmentLink": "containment line",
	"Tree": "tree line",
}

// freeSymbols lists free symbols by count: "an image and 2 text boxes".
func freeSymbols(free map[string]int) string {
	counts := map[string]int{}
	for class, n := range free {
		noun, ok := freeSymbolNouns[class]
		if !ok {
			noun = class + " symbol"
		}
		counts[noun] += n
	}
	nouns := make([]string, 0, len(counts))
	for noun := range counts {
		nouns = append(nouns, noun)
	}
	sort.Strings(nouns)
	parts := make([]string, 0, len(nouns))
	for _, noun := range nouns {
		n := counts[noun]
		if n == 1 {
			parts = append(parts, article(noun)+noun)
			continue
		}
		if strings.HasSuffix(noun, "x") {
			noun += "e"
		}
		parts = append(parts, strconv.Itoa(n)+" "+noun+"s")
	}
	if len(parts) > 1 {
		parts[len(parts)-2] += " and " + parts[len(parts)-1]
		parts = parts[:len(parts)-1]
	}
	return strings.Join(parts, ", ")
}

// writeView writes a diagram as a view usage exposing each shown element the
// document writes, rendered by the diagram's kind, and records its report row.
func (m *migration) writeView(v *view) {
	hidden := m.viewHiddenNames(v)
	m.inside(hidden, func() { m.writeViewBody(v, m.formOf(v.d)) })
}

// viewHiddenNames lists the names hidden inside v by its exposes or nested views.
func (m *migration) viewHiddenNames(v *view) columnNames {
	x := m.exposures(v.d, v.host, m.formOf(v.d))
	// The exposes import their names into the view, where every reference in
	// its body resolves first: the writer avoids those that hide what it means.
	hidden := x.hides()
	for _, scope := range scopeChain(v.host) {
		for name := range m.viewNames[scope] {
			hidden[name] = true
		}
	}
	return hidden
}

// writeViewBody writes v's declaration and body, with references resolving
// through the names its exposes import.
func (m *migration) writeViewBody(v *view, form viewForm) {
	d, host := v.d, v.host
	x := m.exposures(d, host, form)
	for _, scope := range scopeChain(host) {
		for name := range m.viewNames[scope] {
			x.names[name] = true
		}
	}
	decl, note := m.viewDeclaration(v, form, x)
	prefix := viewsPrefix
	if m.shadowsLibrary("Views", host) {
		prefix = globalViewsPrefix
	}
	shown := len(d.Shown)
	untyped := d.Kind == "" && d.UMLKind == ""
	for _, td := range v.tables {
		m.lowerTable(td)
		if td.written() {
			x.names[td.doc] = true
		}
	}
	geo := m.viewGeometry(v, x, form)
	note = joinNotes(note, m.viewShownNote(v, x, geo, untyped))
	doc, docNotes := m.proseNoted(d.Documentation)
	note = joinNotes(note, strings.Join(docNotes, "; "))
	m.w.block(decl, func() {
		if doc != "" {
			m.w.lines(prefixFirst("doc ", commentLines(doc)))
		}
		for _, ref := range x.refs {
			m.w.line("expose " + ref + ";")
		}
		for _, td := range v.tables {
			if td.written() {
				m.w.line("expose " + writeName(td.doc) + ";")
			}
		}
		for _, line := range geo.lines {
			m.w.line(line)
		}
		m.w.line("render " + prefix + form.rendering + ";")
	})
	for _, td := range v.tables {
		m.writeTable(td)
	}
	verdict := Mapped
	drawsNothing := (shown == 0 || len(x.refs) == 0) && geo.pictures == 0
	if v.note != "" || untyped || drawsNothing || v.rebuilt != nil || x.unwritten+x.dangling > 0 || geo.underlaid+geo.undrawn+geo.lost > 0 || len(docNotes) > 0 {
		verdict = Approximated
	}
	v.entry = m.diagramEntry(d, verdict, m.qualified(append(m.segments(host), v.name)), note)
}

// viewDeclaration is the view usage's declaration and the note on what it is
// written as, of, and for.
func (m *migration) viewDeclaration(v *view, form viewForm, x exposures) (decl, note string) {
	d, host := v.d, v.host
	kind := diagramKind(d)
	note = article(strings.ToLower(kind)) + kind + " written as a view rendered " + form.rendering
	decl = "view " + writeName(v.name)
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
	return decl, joinNotes(note, v.note)
}

// viewShownNote notes what of the diagram's shown elements the view exposes
// and draws, and what it leaves out; for rows the tool never stored, what the
// view exposes in their place.
func (m *migration) viewShownNote(v *view, x exposures, geo viewGeometry, untyped bool) string {
	d := v.d
	shown := len(d.Shown)
	var note string
	switch {
	case !d.Represented():
		note = "no diagram representation is serialized: what the diagram is and shows is unknown, and the view exposes nothing"
	case v.rebuilt != nil:
		note = v.rebuilt.note(m, d.Owner, v.host)
	case shown == 0 && geo.pictures > 0:
		note = "the diagram shows no model element; the view draws the " + plural(geo.pictures, pastedImage) + " it carries and exposes nothing"
	case shown == 0 && geo.undrawn > 0:
		note = "the diagram shows no model element, only " + plural(geo.undrawn, pastedImage) + " the view carries and its rendering does not draw; the view exposes nothing"
	case shown == 0:
		note = "the diagram " + showsNothing(d, "shows nothing") + "; the view exposes nothing"
	case len(x.refs) == 0:
		note = "none of the " + strconv.Itoa(shown) + " shown elements is written; the view exposes nothing"
	}
	if untyped && d.Represented() {
		note = joinNotes(note, "the representation names no diagram type")
	}
	if x.unwritten > 0 {
		note = joinNotes(note, fmt.Sprintf("%d of %d shown elements are not written and not exposed", x.unwritten, shown))
	}
	if x.symbolless > 0 {
		note = joinNotes(note, fmt.Sprintf("%d of %d shown elements are compartment rows or end labels of exposed elements, not exposed", x.symbolless, shown))
	}
	if x.dangling > 0 {
		note = joinNotes(note, fmt.Sprintf("%d of %d shown ids resolve to no element", x.dangling, shown))
	}
	return joinNotes(note, geo.note)
}

// exposures resolves what a diagram shows into expose references, in the
// scope of the view's host. A view of a form drawing a graph exposes the
// behavior whose graph it is in place of the shown nodes and edges it draws.
func (m *migration) exposures(d *sysmlv1.Diagram, host *sysmlv1.Element, form viewForm) exposures {
	x := exposures{names: map[string]bool{}, by: map[string][]string{}}
	seen := map[string]bool{}
	add := func(ref string) {
		if !seen[ref] {
			seen[ref] = true
			x.refs = append(x.refs, ref)
			if name := exposureName(ref); name != "" {
				x.names[name] = true
				x.by[name] = append(x.by[name], ref)
			}
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
		ref := m.shownRef(shown.Element, host, form)
		switch {
		case m.graphDraws(form, shown.Element):
			x.drawn++
		case m.notesShown(d, shown.Element):
			// written as a Note the view's dressing lays out
		case ref == "":
			x.unwritten++
		case !symbolized(form, shown):
			x.symbolless++
		default:
			add(ref)
			if c := m.connOf(shown.Element); form.useCases && c != nil {
				// The connection stands for the edge; its members would shadow the usages.
				for _, also := range m.connRefs(c.of, host) {
					add(also)
				}
				continue
			}
			for _, also := range m.edgeRefs(shown.Element, host) {
				add(also)
			}
		}
	}
	if v := m.viewOf[d]; v != nil && v.rebuilt != nil {
		refs, _ := v.rebuilt.refs(m, host)
		for _, ref := range refs {
			add(ref)
		}
	}
	return x
}

// privateLayoutAlias records a private member's owner-local alias.
type privateLayoutAlias struct {
	owner  *sysmlv1.Element
	target *sysmlv1.Element
	name   string
}

// privateLayoutPlacement identifies a view's layout placement of a private member.
type privateLayoutPlacement struct {
	view   *view
	target *sysmlv1.Element
}

// privateLayoutMember reports whether the element is written as a private feature.
func (m *migration) privateLayoutMember(e *sysmlv1.Element) bool {
	if e == nil || e.Type != "Property" && e.Type != "Port" {
		return false
	}
	prefix, _ := m.featureVisibility(e, false, "")
	return m.written(e) && prefix == privatePrefix
}

// planPrivateLayoutAliasHoles marks owners that may need a deferred alias declaration.
func (m *migration) planPrivateLayoutAliasHoles() {
	for i := range m.model.Diagrams {
		v := m.viewOf[&m.model.Diagrams[i]]
		if v == nil || !v.placed {
			continue
		}
		rec, _ := m.layoutRecord(v)
		if rec == nil {
			continue
		}
		scopes := scopeChain(v.host)
		for _, placement := range rec.Placements {
			if placement.OnPath {
				continue
			}
			target := m.model.Lookup(placement.ID)
			if !m.privateLayoutMember(target) {
				continue
			}
			if owner := target.Parent; containsElement(scopes, owner) {
				m.privateAliasOwners[owner] = true
			}
		}
	}
}

// privateLayoutNameShadowed reports whether a private member's simple name is unusable in a view.
func (m *migration) privateLayoutNameShadowed(owner, target, scope *sysmlv1.Element, hidden bool) bool {
	name := m.nameFor(target)
	chain := scopeChain(scope)
	for i, enclosing := range chain {
		if enclosing != owner {
			continue
		}
		if hidden {
			return true
		}
		for _, inner := range chain[:i] {
			if name != "" && m.nameTaken(inner, name) {
				return true
			}
		}
		return false
	}
	return false
}

// planPrivateLayoutAliases chooses aliases for shadowed private layout placements.
func (m *migration) planPrivateLayoutAliases() {
	for i := range m.model.Diagrams {
		d := &m.model.Diagrams[i]
		v := m.viewOf[d]
		if v == nil || !v.placed {
			continue
		}
		rec, _ := m.layoutRecord(v)
		if rec == nil {
			continue
		}
		hidden := m.viewHiddenNames(v)
		for _, placement := range rec.Placements {
			if placement.OnPath {
				continue
			}
			target := m.model.Lookup(placement.ID)
			if !m.privateLayoutMember(target) {
				continue
			}
			owner := target.Parent
			if !containsElement(scopeChain(v.host), owner) ||
				!m.privateLayoutNameShadowed(owner, target, v.host, hidden[m.nameFor(target)]) {
				continue
			}
			alias := m.privateAliases[target]
			if alias == nil {
				alias = m.planPrivateLayoutAlias(target, owner)
			}
			m.privateAliasPlacements[privateLayoutPlacement{view: v, target: target}] = alias
		}
	}
}

// privateLayoutAlias returns the alias planned for a private layout placement, if any.
func (m *migration) privateLayoutAlias(v *view, target *sysmlv1.Element) *privateLayoutAlias {
	return m.privateAliasPlacements[privateLayoutPlacement{view: v, target: target}]
}

// planPrivateLayoutAlias chooses an owner-local name that resolves in each view of target.
func (m *migration) planPrivateLayoutAlias(target, owner *sysmlv1.Element) *privateLayoutAlias {
	if alias := m.privateAliases[target]; alias != nil {
		return alias
	}
	base := m.nameFor(target) + " (" + m.nameFor(owner) + ")"
	for i := 1; ; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s %d", base, i)
		}
		if m.nameTaken(owner, name) || m.referredPast(owner, name) || !m.privateLayoutAliasAvailable(target, owner, name) {
			continue
		}
		alias := &privateLayoutAlias{owner: owner, target: target, name: name}
		m.take(owner, name)
		m.privateAliases[target] = alias
		m.privateAliasesByOwner[owner] = append(m.privateAliasesByOwner[owner], alias)
		return alias
	}
}

// privateLayoutAliasAvailable reports whether the alias resolves unambiguously in every view that uses it.
func (m *migration) privateLayoutAliasAvailable(target, owner *sysmlv1.Element, name string) bool {
	available := func(v *view) bool {
		if v == nil {
			return true
		}
		chain := scopeChain(v.host)
		ownerIndex := -1
		for i, scope := range chain {
			if scope == owner {
				ownerIndex = i
				break
			}
		}
		if ownerIndex < 0 {
			return true
		}
		x := m.exposures(v.d, v.host, m.formOf(v.d))
		if x.names[name] || m.viewHiddenNames(v)[name] {
			return false
		}
		for _, table := range v.tables {
			if table.doc == name && table.written() {
				return false
			}
		}
		for _, inner := range chain[:ownerIndex] {
			if m.nameTaken(inner, name) {
				return false
			}
		}
		return true
	}
	for i := range m.model.Diagrams {
		v := m.viewOf[&m.model.Diagrams[i]]
		if v == nil {
			continue
		}
		rec, _ := m.layoutRecord(v)
		if rec == nil {
			continue
		}
		shown := false
		for _, placement := range rec.Placements {
			if placement.ID == target.ID {
				shown = true
				break
			}
		}
		if shown && !available(v) {
			return false
		}
	}
	return true
}

// writePrivateLayoutAliases writes the aliases planned for an owner's body.
func (m *migration) writePrivateLayoutAliases(owner *sysmlv1.Element) {
	saved := m.scope
	m.scope = owner
	for _, alias := range m.privateAliasesByOwner[owner] {
		m.w.line("private alias " + writeName(alias.name) + " for " + writeName(m.nameFor(alias.target)) + ";")
		m.w.line(m.synthesizedNames([]string{writeName(alias.name)}))
	}
	m.scope = saved
}

// exposureName returns the unqualified name at the end of an exposure reference.
func exposureName(ref string) string {
	start := 0
	quoted := false
	for i := 0; i < len(ref); i++ {
		switch {
		case quoted && ref[i] == '\\':
			i++
		case ref[i] == '\'':
			quoted = !quoted
		case !quoted && strings.HasPrefix(ref[i:], "::"):
			start = i + 2
			i++
		}
	}
	name := ref[start:]
	if name == "*" || name == "**" {
		return ""
	}
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		name = source.Unescape(name[1 : len(name)-1])
	}
	return name
}

// exposure names, from scope, what stands for e: the library type a primitive
// maps to, the analysis def a statistic of the tool's Monte Carlo pattern is
// written into, or the declaration exposable names. It is "" when nothing does.
func (m *migration) exposure(e, scope *sysmlv1.Element) string {
	if sv := m.scalarValue(e); sv != "" {
		if m.shadowsLibrary("ScalarValues", scope) {
			return "$::" + scalarValuesPrefix + sv
		}
		return scalarValuesPrefix + sv
	}
	if link := m.actorLinkOf(e); link != nil {
		if link.conn == nil {
			return ""
		}
		return m.hostedRef(link.conn.host, link.conn.name, scope)
	}
	if ref := m.monteCarloExposure(e, scope); ref != "" {
		return ref
	}
	if ref := m.edgeRef(e, scope); ref != "" {
		return ref
	}
	if x := m.exposable(e); x != nil {
		return m.memberRef(x, scope)
	}
	return ""
}

// actorLinkOf is the link whose connection stands for e: an anonymous association
// to an actor, or its end at the actor; nil when e is written by itself.
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

// shownRef is what a view of form f exposes for e, from scope: on a use case
// diagram the usage written for a definition, or for an association the
// connection alone (see connRefs); otherwise what exposure names.
func (m *migration) shownRef(e, scope *sysmlv1.Element, f viewForm) string {
	if f.useCases {
		if u := m.usageIn(e); u != nil {
			return m.usageRef(u, scope)
		}
		if c := m.connOf(e); c != nil {
			return m.connRefs(c.of, scope)[0]
		}
	}
	if e.Type == "Property" && m.associationDef(e.Parent) {
		// An end an association's def declares is shown by the def's line or box;
		// exposed by itself it would shadow a same-named end of another def.
		return m.exposure(e.Parent, scope)
	}
	return m.exposure(e, scope)
}

// inheritedNodeRef names the library member a graph draws el as — the start an
// initial node or pseudostate is written as, the done a flow final or final state
// ends at — qualified through the body it is drawn in, as `about Run::start` names
// Run's start: a view is a part with a start and done of its own, which the bare
// name would reach. "" for a node written as a member of its own.
func (m *migration) inheritedNodeRef(el, scope *sysmlv1.Element) string {
	var name string
	switch {
	case el.Type == "InitialNode", pseudoKind(el) == "initial":
		name = "start"
	case el.Type == "FlowFinalNode", el.Type == "FinalState":
		name = "done"
	default:
		return ""
	}
	body := el.Parent
	if pseudoKind(el) == "initial" {
		body = m.entryRegion(el)
	}
	if body == nil {
		return ""
	}
	owner := m.edgePlaceIn(body, nil, name).owner
	if owner == nil || !m.reaches(owner) {
		return ""
	}
	return m.memberRef(owner, scope) + "::" + name
}

// entryRegion is the region whose start the initial pseudostate init is written as:
// its own region when it is that region's entry, or the orthogonal region it is
// donated to; nil for a duplicate, dangling, coinciding, conflicting or foreign
// initial, which no start stands for.
func (m *migration) entryRegion(init *sysmlv1.Element) *sysmlv1.Element {
	r := init.Parent
	if r == nil || r.Type != "Region" {
		return nil
	}
	e := m.regionEntry(r)
	if e.init == init {
		return r
	}
	for _, st := range e.strays {
		if st.init == init && m.regionEntry(st.into).init == init {
			return st.into
		}
	}
	return nil
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
	return m.hidden(lib) || m.shadows(scopeChain(scope), lib) || m.nameTaken(nil, lib)
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
		for _, sym := range d.Symbols {
			if sym.ImageError != nil {
				m.report.Entries = append(m.report.Entries, Entry{
					ID: sym.ID, Kind: sym.Class, Name: v.entry.Name, Verdict: Unmapped, Note: sym.ImageError.Error(),
				})
			}
		}
	}
	m.unplacedTables()
}

// viewGeometry is the layout a diagram writes into a view: the annotation lines, their
// report clause, and the pasted images drawn, underlaid, written but undrawn, and lost.
type viewGeometry struct {
	lines     []string
	note      string
	pictures  int
	underlaid int
	undrawn   int
	lost      int
}

// viewGeometry plans the DiagramLayout annotations of v's layout record, the
// export's or the diagram's own stream's: a Canvas sized by the frame or the
// bounding box of what is written, a Layout per shown element the view exposes,
// a Route per connector whose element does, tallied by the connector's v1 kind
// and why it is or is not pinned, then the Pictures, Styles and Notes the
// stream carries.
func (m *migration) viewGeometry(v *view, x exposures, form viewForm) viewGeometry {
	s := m.layoutSummary
	if s == nil {
		return viewGeometry{}
	}
	rec, src := m.layoutRecord(v)
	if rec == nil {
		s.ViewsWithoutLayout++
		return viewGeometry{}
	}
	source := m.layoutSourceName(src)
	switch {
	case src.export:
		s.DiagramsJoined++
		m.layoutJoined[rec.ID] = true
		if src.stream && !m.strict {
			s.StreamSupplemented++
		}
	default:
		s.StreamDiagrams++
	}
	if m.strict {
		s.GeometryOmitted++
		// Nothing is written, so the stream supplements nothing.
		source = m.layoutSourceName(layoutSources{export: src.export})
		return viewGeometry{note: "layout from " + source + " omitted: a strict migration names no OpenSysML library, and DiagramLayout is one's"}
	}
	prefix := diagramLayoutPrefix
	if m.shadowsLibrary("DiagramLayout", v.host) {
		prefix = globalDiagramLayoutPrefix
	}
	p := &layoutPlan{m: m, v: v, x: x, form: form, prefix: prefix, refOf: map[string]string{}}
	if src.frame {
		f := v.d.Frame
		p.grow(f.X+f.Width, f.Y+f.Height)
	}
	placements := p.placements(rec)
	routes := p.routes(rec)
	for tag, n := range rec.Unsupported {
		s.Unsupported[tag] += n
	}
	dress := m.viewDressing(v, form, prefix, x, func(id string) string {
		if ref, ok := p.refOf[id]; ok {
			return ref
		}
		return m.drawnRef(x, form, v.host, id)
	})
	for _, pic := range m.pastedPictures(v.d).drawn {
		p.grow(pic.sym.Bounds.X+pic.sym.Bounds.Width, pic.sym.Bounds.Y+pic.sym.Bounds.Height)
	}
	geo := viewGeometry{pictures: dress.pictures, underlaid: dress.underlaid, undrawn: dress.undrawn, lost: dress.lost}
	if p.size {
		attrs := []string{
			m.diagramLayoutAttributeForHost(prefix, "Canvas", "unit", `"px"`, v.host, x),
			m.diagramLayoutAttributeForHost(prefix, "Canvas", "width", layoutNumber(p.maxX), v.host, x),
			m.diagramLayoutAttributeForHost(prefix, "Canvas", "height", layoutNumber(p.maxY), v.host, x),
		}
		geo.lines = append(geo.lines, fmt.Sprintf("@%sCanvas { %s; }", prefix, strings.Join(attrs, "; ")))
	}
	geo.lines = append(geo.lines, placements...)
	geo.lines = append(geo.lines, routes...)
	geo.lines = append(geo.lines, dress.lines...)
	geo.note = joinNotes(geo.note, strings.Join(p.notes, "; "))
	if len(rec.Placements)+len(rec.Connectors) > 0 {
		clauses := append([]string{layoutClause(p.written, p.unexposed, p.dangling, p.onEdges, len(rec.Placements)), routeClause(p.reasons, len(rec.Connectors))}, dress.notes...)
		geo.note = joinNotes(geo.note, fmt.Sprintf("laid out from %s: %s", source, strings.Join(clauses, ", ")))
	} else if len(dress.notes) > 0 {
		geo.note = joinNotes(geo.note, fmt.Sprintf("dressed from %s: %s", streamSource, strings.Join(dress.notes, ", ")))
	}
	return geo
}

// layoutPlan places and routes one view's layout record, growing the canvas
// to hold what it writes and keeping the reference each id was written as.
type layoutPlan struct {
	m          *migration
	v          *view
	x          exposures
	form       viewForm
	prefix     string
	maxX, maxY float64
	size       bool
	refOf      map[string]string
	written    int
	unexposed  int
	onEdges    int
	dangling   int
	reasons    map[string]int
	notes      []string
}

func (p *layoutPlan) grow(w, h float64) {
	p.size = true
	if w > p.maxX {
		p.maxX = w
	}
	if h > p.maxY {
		p.maxY = h
	}
}

// placements writes a Layout per shown element the view exposes, once per reference.
func (p *layoutPlan) placements(rec *mtip.Diagram) []string {
	m, v, s := p.m, p.v, p.m.layoutSummary
	var placements []string
	seen := map[string]bool{}
	for _, pl := range rec.Placements {
		s.Placements++
		el := m.model.Lookup(pl.ID)
		if el == nil {
			s.PlacementsDangling++
			p.dangling++
			continue
		}
		if c := m.connOf(el); pl.OnPath || p.form.useCases && c != nil && c.of != el {
			// An end's symbol lies on the connection's route, which carries the end.
			s.PlacementsOnEdges++
			p.onEdges++
			continue
		}
		var privateOwner *sysmlv1.Element
		if m.privateLayoutMember(el) {
			privateOwner = el.Parent
			if !containsElement(scopeChain(v.host), privateOwner) {
				p.notePrivateUnexposed(el, privateOwner)
				s.PlacementsUnexposed++
				p.unexposed++
				continue
			}
		}
		ref := ""
		if vertexBase(el) == "" || m.vertexWritten(el) {
			ref = m.shownRef(el, v.host, p.form)
		}
		if ref == "" {
			ref = m.inheritedNodeRef(el, v.host)
		}
		if ref == "" || !m.places(p.x, p.form, el, ref) {
			s.PlacementsUnexposed++
			p.unexposed++
			continue
		}
		if alias := m.privateLayoutAlias(v, el); alias != nil {
			ref = writeName(alias.name)
		}
		p.refOf[pl.ID] = ref
		if seen[ref] {
			continue
		}
		seen[ref] = true
		s.PlacementsWritten++
		p.written++
		attrs := []string{
			m.diagramLayoutAttributeForHost(p.prefix, "Layout", "x", layoutNumber(pl.X), v.host, p.x),
			m.diagramLayoutAttributeForHost(p.prefix, "Layout", "y", layoutNumber(pl.Y), v.host, p.x),
			m.diagramLayoutAttributeForHost(p.prefix, "Layout", "width", layoutNumber(pl.Width), v.host, p.x),
			m.diagramLayoutAttributeForHost(p.prefix, "Layout", "height", layoutNumber(pl.Height), v.host, p.x),
		}
		for _, kind := range pl.HiddenCompartments {
			attrs = append(attrs, m.diagramLayoutAttributeForHost(p.prefix, "Layout", "show"+kind, "false", v.host, p.x))
		}
		if len(pl.HiddenCompartments) > 0 {
			s.CompartmentsSuppressed += len(pl.HiddenCompartments)
			s.SymbolsSuppressing++
		}
		for _, id := range pl.UnsupportedCompartments {
			s.Unsupported[id]++
		}
		placements = append(placements, fmt.Sprintf("metadata %sLayout about %s { %s; }",
			p.prefix, ref, strings.Join(attrs, "; ")))
		p.grow(pl.X+pl.Width, pl.Y+pl.Height)
	}
	return placements
}

// notePrivateUnexposed records why a private layout target has no visible name in the view.
func (p *layoutPlan) notePrivateUnexposed(target, owner *sysmlv1.Element) {
	note := "the shown element " + writeName(p.m.nameFor(target)) + " is private to " +
		qualifiedName(owner) + " and no name visible in the view denotes it"
	for _, existing := range p.notes {
		if existing == note {
			return
		}
	}
	p.notes = append(p.notes, note)
}

// routes writes a Route per connector whose element the view draws, tallying
// by the connector's v1 kind why each is or is not pinned.
func (p *layoutPlan) routes(rec *mtip.Diagram) []string {
	m, v, s := p.m, p.v, p.m.layoutSummary
	var routes []string
	p.reasons = map[string]int{}
	pinned := map[string]bool{}
	boxes := map[string]mtip.Placement{}
	for _, pl := range rec.Placements {
		boxes[pl.ID] = pl
	}
	for _, c := range rec.Connectors {
		s.Routes++
		el := m.model.Lookup(c.ID)
		if el == nil {
			s.RoutesDangling++
			p.reasons[routeDangling]++
			m.routeKinds.add(routeKindName(c.Type, nil), routeDangling)
			continue
		}
		kind := routeKindName(c.Type, el)
		var refs []string
		var points [][]float64
		general, why := "", ""
		if refs, general, why = m.routeTarget(el, v.host, p.form); why == "" {
			if !m.draws(p.x, p.form, v.host, el, refs[0]) || general != "" && !p.x.exposed(general) {
				why = routeNotExposed
			}
			refs, points = m.routeLines(el, p.form, refs, m.linePoints(el, c, boxes))
		}
		ref := strings.Join(refs, ", ")
		if why == "" && pinned[ref+" :> "+general] {
			p.refOf[c.ID] = ref
			why = routeDuplicate
		}
		if why != "" {
			s.RoutesUnexposed++
			p.reasons[why]++
			m.routeKinds.add(kind, why)
			continue
		}
		pinned[ref+" :> "+general] = true
		p.refOf[c.ID] = ref
		s.RoutesWritten++
		p.reasons[routeWritten]++
		m.routeKinds.add(kind, routeWritten)
		for i, r := range refs {
			routes = append(routes, p.route(r, general, points[i]))
		}
	}
	return routes
}

// route writes a Route pinning points, x then y per point, to the member ref
// names; a general names the element a specialization edge is steered to.
func (p *layoutPlan) route(ref, general string, points []float64) string {
	var b strings.Builder
	b.WriteString("metadata " + p.prefix + "Route about " + ref + " { ")
	b.WriteString(p.m.diagramLayoutAttributeForHost(p.prefix, "Route", "points", "(", p.v.host, p.x))
	for i, n := range points {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(layoutNumber(n))
		if i%2 == 0 {
			p.grow(n, p.maxY)
		} else {
			p.grow(p.maxX, n)
		}
	}
	b.WriteString(");")
	if general != "" {
		b.WriteString(" ref :>> " + p.m.diagramLayoutFeatureForHost(p.prefix, "Route", "general", p.v.host, p.x) + " : " + general + ";")
	}
	b.WriteString(" }")
	return b.String()
}

// linePoints is the line's points as the Route of el lists them, run from the written
// element's first end to its second as a rendering reads them: the record's ends, or
// failing those the boxes it places, say which way the tool drew the line.
func (m *migration) linePoints(el *sysmlv1.Element, c mtip.Connector, boxes map[string]mtip.Placement) []float64 {
	if el.Type == "Connector" {
		return m.connectorLinePoints(el, c)
	}
	first, second := m.lineEnds(el)
	if first == nil || second == nil || first == second {
		return c.Points
	}
	reversed, known := lineReversed(c, first.ID, second.ID, boxes)
	if !known {
		// A flow between pins may be drawn, or its ends placed, at the actions owning them.
		if a, b := ownerNode(first), ownerNode(second); a != b && (a != first || b != second) {
			reversed, _ = lineReversed(c, a.ID, b.ID, boxes)
		}
	}
	if reversed {
		return reversePoints(c.Points)
	}
	return c.Points
}

// lineReversed reports whether c's points run from the element at the line's second
// end to the one at its first, by the ends the record names or, naming none, the
// boxes it places; known is false when neither end is named or placed.
func lineReversed(c mtip.Connector, first, second string, boxes map[string]mtip.Placement) (reversed, known bool) {
	ends := c.Ends
	if ends == [2]string{} {
		ends = endsAt(c.Points, [2]string{first, second}, boxes)
	}
	switch {
	case ends[0] == second, ends[1] == first:
		return true, true
	case ends[0] == first, ends[1] == second:
		return false, true
	}
	return false, false
}

// lineEnds are the elements at the ends of the line el is drawn as, in the order its
// member names them: a flow's or transition's source then target, a connection def's end
// types, or a backed attribute's owner then its type.
func (m *migration) lineEnds(el *sysmlv1.Element) (first, second *sysmlv1.Element) {
	if el.Role == "edge" || el.Type == "Transition" {
		return m.model.Ref(el, "source"), m.model.Ref(el, "target")
	}
	if prop := m.backedProperty(el); prop != nil {
		return prop.Parent, m.model.Ref(prop, "type")
	}
	if el.Type == "Generalization" {
		return el.Parent, m.model.Ref(el, "general")
	}
	if !m.associationDef(el) {
		return nil, nil
	}
	ends := m.model.Refs(el, "memberEnd")
	if len(ends) != 2 {
		return nil, nil
	}
	return m.model.Ref(ends[0], "type"), m.model.Ref(ends[1], "type")
}

// connectorLinePoints is connector el's line run as `bind a = b` names its ends: reversed when
// the stream drew it from the second end's feature; as drawn when it names no end or one feature.
func (m *migration) connectorLinePoints(el *sysmlv1.Element, c mtip.Connector) []float64 {
	segs, note := m.connectorEnds(el, el.Parent)
	if note != "" || c.Ends[0] == "" && c.Ends[1] == "" {
		return c.Points
	}
	first, second := endFeature(segs[0]), endFeature(segs[1])
	if first == second {
		return c.Points
	}
	if c.Ends[0] == second || c.Ends[1] == first {
		return reversePoints(c.Points)
	}
	return c.Points
}

// endFeature is the id of the feature a connector end's path names last, the element
// the symbol at that end of the line stands for.
func endFeature(seg []*sysmlv1.Element) string {
	return seg[len(seg)-1].ID
}

// reversePoints is the flattened point list read from its last point to its first.
func reversePoints(points []float64) []float64 {
	out := make([]float64, 0, len(points))
	for i := len(points) - 2; i >= 0; i -= 2 {
		out = append(out, points[i], points[i+1])
	}
	return out
}

// drawnRef names the element id as the view of form f exposing x draws it, as a
// node or an edge, or "" when the rendering does not draw it.
func (m *migration) drawnRef(x exposures, f viewForm, host *sysmlv1.Element, id string) string {
	el := m.model.Lookup(id)
	if el == nil {
		return ""
	}
	if ref := m.shownRef(el, host, f); ref != "" && m.places(x, f, el, ref) {
		return ref
	}
	if ref := m.inheritedNodeRef(el, host); ref != "" && m.places(x, f, el, ref) {
		return ref
	}
	refs, _, why := m.routeTarget(el, host, f)
	if why != "" || !m.draws(x, f, host, el, refs[0]) {
		return ""
	}
	return strings.Join(refs, ", ")
}

// layoutClause words the placements of the layout note: how many of the
// record's shown elements were positioned, and why the rest were not.
func layoutClause(written, unexposed, dangling, onEdges, total int) string {
	clause := fmt.Sprintf("%d of %d shown elements positioned", written, total)
	var parts []string
	if unexposed > 0 {
		parts = append(parts, fmt.Sprintf("%d not exposed", unexposed))
	}
	if onEdges > 0 {
		parts = append(parts, fmt.Sprintf("%d lying on connections", onEdges))
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
	s := m.layoutSummary
	if s == nil {
		return
	}
	for i := 0; m.layout != nil && i < len(m.layout.Diagrams); i++ {
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
