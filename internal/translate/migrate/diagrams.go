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
	{[]string{"internal block", "parametric", "composite structure", "interconnection"}, "asInterconnectionDiagram"},
	{[]string{"block definition", "class", "package", "object", "component", "deployment", "profile", "structure"}, "asTreeDiagram"},
}

const textualRendering = "asTextualNotation"

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
	d    *sysmlv1.Diagram
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
		v.host, v.note = m.viewHost(d)
		if v.host == nil {
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
		m.hosted[v.host] = append(m.hosted[v.host], v)
	}
}

// viewName reserves the name a view takes in host's body: name, or name with a
// number when a member of the body has it — the vertices a state def writes
// from its regions included, which are named ahead of it.
func (m *migration) viewName(host *sysmlv1.Element, name string) string {
	var used map[string]bool
	if host.Type == "StateMachine" {
		used = m.nameMachine(host)
	}
	base := name
	for i := 2; used[name] || m.nameTaken(host, name); i++ {
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
// that the document's top level. note says why the host is not the owner;
// host is nil when nothing written can hold the view.
func (m *migration) viewHost(d *sysmlv1.Diagram) (host *sysmlv1.Element, note string) {
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
		for _, r := range m.model.Roots {
			if m.hostsViews(r) {
				return r, joinNotes(note, m.writtenIn(r))
			}
		}
		return nil, joinNotes(note, "and nothing written holds it")
	}
	for cur := from; cur != nil; cur = cur.Parent {
		if !m.hostsViews(cur) {
			continue
		}
		switch {
		case cur == from && d.Owner != nil:
		case d.Owner != nil:
			note = "its owner " + kindOf(d.Owner) + " " + qualifiedName(d.Owner) + " has no v2 body; " + m.writtenIn(cur)
		default:
			note = joinNotes(note, m.writtenIn(cur)+", which holds it")
		}
		return cur, note
	}
	if d.Owner != nil {
		return nil, "neither its owner " + kindOf(d.Owner) + " " + qualifiedName(d.Owner) + " nor any ancestor of it is written"
	}
	return nil, joinNotes(note, "and neither "+kindOf(from)+" "+qualifiedName(from)+", which holds it, nor any ancestor of it is written")
}

// viewOwner is the element whose v2 body stands for a diagram's owner: the
// operation a method behavior is written as the body of, else the owner itself.
func (m *migration) viewOwner(d *sysmlv1.Diagram) *sysmlv1.Element {
	if op := m.methodOf[d.Owner]; op != nil {
		return op
	}
	return d.Owner
}

// hostsViews reports whether e is written with a body a view can be a member
// of: the top level of the root model, or a declared package or definition
// other than an enum def, whose members are its values.
func (m *migration) hostsViews(e *sysmlv1.Element) bool {
	if isTopLevel(e) {
		return !m.isLibrary(e)
	}
	switch e.Type {
	case "Property", "Port", "Parameter", "EnumerationLiteral":
		return false
	}
	if m.methodOf[e] != nil || !m.written(e) {
		return false
	}
	cat, _ := m.classify(e)
	return cat != catEnumDef
}

// hostName describes a host for a note: its v2 declaration, or the top level.
func (m *migration) hostName(host *sysmlv1.Element) string {
	if isTopLevel(host) {
		return "the top level"
	}
	cat, _ := m.classify(host)
	return cat.keyword() + " " + m.v2Name(host)
}

// writtenIn says where a view is written, for a note.
func (m *migration) writtenIn(host *sysmlv1.Element) string {
	if isTopLevel(host) {
		return "written at the top level"
	}
	return "written in " + m.hostName(host)
}

// isTopLevel reports whether e is the root model, whose members are the
// document's top level.
func isTopLevel(e *sysmlv1.Element) bool {
	return e.Parent == nil && e.Type == "Model"
}

// views writes the views of the diagrams host owns, in source order.
func (m *migration) views(host *sysmlv1.Element) {
	for _, v := range m.hosted[host] {
		m.writeView(v)
	}
}

// diagramViewNote says why an «Expose» of a diagram cannot name its view: no
// body takes the view; "" when the view is written.
func (m *migration) diagramViewNote(d *sysmlv1.Diagram) string {
	if host, note := m.viewHost(d); host == nil {
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
	if isTopLevel(host) {
		host = nil
	}
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
	// unwritten counts shown elements nothing written stands for.
	unwritten int
	// dangling counts shown ids that resolve to no element.
	dangling int
}

// writeView writes a diagram as a view usage exposing each shown element the
// document writes, rendered by the diagram's kind, and records its report row.
func (m *migration) writeView(v *view) {
	d, host := v.d, v.host
	x := m.exposures(d, host)
	render := rendering(d)
	kind := diagramKind(d)
	note := article(strings.ToLower(kind)) + kind + " written as a view rendered " + render
	if host != d.Owner && host == m.viewOwner(d) {
		note = joinNotes(note, "its owner "+kindOf(d.Owner)+" "+qualifiedName(d.Owner)+" is written as the body of "+m.hostName(host)+", whose method it is")
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
	m.w.block("view "+writeName(v.name), func() {
		for _, ref := range x.refs {
			m.w.line("expose " + ref + ";")
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
// scope of the view's host.
func (m *migration) exposures(d *sysmlv1.Diagram, host *sysmlv1.Element) exposures {
	var x exposures
	seen := map[string]bool{}
	for _, shown := range d.Shown {
		if shown.Element == nil {
			x.dangling++
			continue
		}
		ref := m.exposure(shown.Element, host)
		if ref == "" {
			x.unwritten++
			continue
		}
		if !seen[ref] {
			seen[ref] = true
			x.refs = append(x.refs, ref)
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

// exposable is the declaration written for e that a view can expose: its own,
// or the operation a behavior is the method of, or that one's parameter. It is
// nil when nothing written stands for e: library and tool content, and what a
// body leaves out.
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
	if !m.written(e) {
		return nil
	}
	return e
}

// shadowsLibrary reports whether a member named like a standard library
// package hides it from scope: one of a scope on the chain, or a top-level one.
func (m *migration) shadowsLibrary(lib string, scope *sysmlv1.Element) bool {
	if m.shadows(scopeChain(scope), lib) {
		return true
	}
	for _, r := range m.model.Roots {
		if r.Type == "Model" {
			if m.nameTaken(r, lib) {
				return true
			}
		} else if m.nameOf(r) == lib {
			return true
		}
	}
	return false
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
