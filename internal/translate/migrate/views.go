package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// viewBody writes a view usage: its doc, the viewpoints it conforms to by
// generalization or tag, its members, and what its dependencies place in it.
func (m *migration) viewBody(e *sysmlv1.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	m.conformGeneralizations(e)
	m.taggedViewpoints(e)
	m.members(e)
	m.classifierBehavior(e)
	m.stereotypeComments(e)
	m.scope = saved
}

// conforms reports whether a generalization is a v1 Conform: stereotyped so,
// or generalizing a viewpoint, as the profile spells it since SysML 1.4.
func (m *migration) conforms(g *sysmlv1.Element) bool {
	if has(g, "Conform") {
		return true
	}
	t := m.model.Ref(g, "general")
	if t == nil {
		return false
	}
	cat, _ := m.classify(t)
	return cat == catViewpoint
}

// conformGeneralizations writes a view's Conform generalizations as satisfy
// members, each with a report entry of its own.
func (m *migration) conformGeneralizations(e *sysmlv1.Element) {
	for _, g := range e.Owned("generalization") {
		if !m.conforms(g) {
			continue
		}
		vp := m.model.Ref(g, "general")
		if note := m.viewpointNote(vp, e); note != "" {
			m.unmapped(g, note)
			continue
		}
		m.satisfyViewpoint(e, vp)
		m.add(g, Mapped, m.v2Name(e), "")
	}
}

// satisfyViewpoint writes the satisfy member of view for vp, once however many
// relationships name it.
func (m *migration) satisfyViewpoint(view, vp *sysmlv1.Element) {
	if m.satisfied[view] == nil {
		m.satisfied[view] = map[*sysmlv1.Element]bool{}
	}
	if m.satisfied[view][vp] {
		return
	}
	m.satisfied[view][vp] = true
	m.w.line("satisfy " + m.ref(vp, view) + ";")
}

// viewpointNote says why vp cannot be satisfied: it is absent, external, not
// migrated or not a viewpoint; "" when a view can satisfy it.
func (m *migration) viewpointNote(vp, view *sysmlv1.Element) string {
	switch {
	case vp == nil:
		return "the viewpoint is not in the document"
	case vp.IsProxy():
		return "the viewpoint " + qualifiedName(vp) + " lives outside the document and is not written"
	case !m.written(vp):
		_, why := m.classify(vp)
		return joinNotes("the viewpoint "+qualifiedName(vp)+" is not migrated", why)
	}
	if cat, _ := m.classify(vp); cat != catViewpoint {
		return qualifiedName(vp) + " becomes a " + cat.keyword() + ", which a view cannot satisfy"
	}
	return m.featuredNote(vp, view)
}

// featuredNote says why a usage cannot be named from scope: a feature of a
// definition is accessible to that definition's members alone, not by
// qualified name from elsewhere. "" when the usage is accessible.
func (m *migration) featuredNote(u, scope *sysmlv1.Element) string {
	def := m.featuringDef(u)
	if def == nil {
		return ""
	}
	for cur := scope; cur != nil; cur = cur.Parent {
		if cur == def {
			return ""
		}
		if !m.isUsage(cur) {
			break
		}
	}
	cat, _ := m.classify(u)
	defCat, _ := m.classify(def)
	return "the " + cat.keyword() + " " + qualifiedName(u) + " is a feature of the " + defCat.keyword() + " " + qualifiedName(def) + ", which only its members can name"
}

// featuringDef is the innermost definition u is a feature of, through any
// usages between them; nil when u is featured by packages alone.
func (m *migration) featuringDef(u *sysmlv1.Element) *sysmlv1.Element {
	for cur := u.Parent; cur != nil; cur = cur.Parent {
		if cur.Parent == nil && cur.Type == "Model" {
			return nil
		}
		if m.isUsage(cur) {
			continue
		}
		if cat, _ := m.classify(cur); cat != catPackage {
			return cur
		}
	}
	return nil
}

// taggedViewpoints satisfies the viewpoints the «View» stereotype's viewpoint
// tag names, in either spelling the profile has used, when a Conform does not.
func (m *migration) taggedViewpoints(e *sysmlv1.Element) {
	v := stereo(e, "View")
	if v == nil {
		return
	}
	for _, tag := range viewpointTags {
		for _, id := range v.IDs(tag) {
			vp := m.model.Lookup(id)
			if vp == nil {
				m.downgrade(e, "the "+tag+" tag names "+id+", which is not in the document")
				continue
			}
			if note := m.viewpointNote(vp, e); note != "" {
				m.downgrade(e, "the "+tag+" tag is not written: "+note)
				continue
			}
			m.satisfyViewpoint(e, vp)
		}
	}
}

// viewpointTags are the «View» tags naming the viewpoint a view conforms to.
var viewpointTags = []string{"viewpoint", "viewPoint"}

// placeConform registers a «Conform» dependency as a satisfy member in the body
// of each view it has as client; the report entry is written where it stands.
func (m *migration) placeConform(d *sysmlv1.Element) {
	pl := &placement{}
	m.unplaced[d] = pl
	pairs, failed, note := m.dependencyPairs(d)
	pl.failed = failed
	if note != "" {
		pl.notes = append(pl.notes, note)
	}
	for _, p := range pairs {
		if cat, _ := m.classify(p.client); cat != catView {
			pl.failed++
			pl.notes = append(pl.notes, "the client "+qualifiedName(p.client)+" is not a view: v2 lets a view alone satisfy a viewpoint")
			continue
		}
		if n := m.viewpointNote(p.supplier, p.client); n != "" {
			pl.failed++
			pl.notes = append(pl.notes, n)
			continue
		}
		pl.written++
		pl.target = m.v2Name(p.client)
		view, vp := p.client, p.supplier
		m.extras[view] = append(m.extras[view], func() { m.satisfyViewpoint(view, vp) })
	}
}

// placeExpose registers an «Expose» dependency as expose members in the body
// of each view it has as client, and accounts for every pair none can carry.
func (m *migration) placeExpose(d *sysmlv1.Element) {
	pl := &placement{}
	m.unplaced[d] = pl
	clients, suppliers := m.model.Refs(d, "client"), m.model.Refs(d, "supplier")
	lostClients, lostSuppliers := m.model.Unresolved(d, "client"), m.model.Unresolved(d, "supplier")
	total := (len(clients) + len(lostClients)) * (len(suppliers) + len(lostSuppliers))
	if total == 0 {
		pl.failed = 1
		pl.notes = append(pl.notes, "the dependency's client or supplier is not in the document")
		return
	}
	for _, id := range lostClients {
		pl.notes = append(pl.notes, "the exposing element "+id+" is not in the document")
	}
	var diagrams []*sysmlv1.Diagram
	for _, id := range lostSuppliers {
		if d := m.model.Diagram(id); d != nil {
			if note := m.diagramViewNote(d); note != "" {
				pl.notes = append(pl.notes, note)
				continue
			}
			diagrams = append(diagrams, d)
			continue
		}
		pl.notes = append(pl.notes, m.absentNote(id))
	}
	for _, c := range clients {
		if cat, _ := m.classify(c); cat != catView {
			pl.notes = append(pl.notes, "the client "+qualifiedName(c)+" is not a view: v2 exposes elements from a view alone")
			continue
		}
		for _, s := range suppliers {
			if note := m.exposeNote(s); note != "" {
				pl.notes = append(pl.notes, note)
				continue
			}
			pl.written++
			pl.target = m.v2Name(c)
			view, exposed := c, s
			m.extras[view] = append(m.extras[view], func() { m.w.line("expose " + m.exposeRef(view, exposed) + ";") })
		}
		for _, d := range diagrams {
			pl.written++
			pl.target = m.v2Name(c)
			view, shown := c, d
			m.extras[view] = append(m.extras[view], func() { m.w.line("expose " + m.viewRef(m.viewOf[shown], view) + ";") })
		}
	}
	pl.failed = total - pl.written
}

// exposeNote says why a view cannot expose s: it is external or not migrated;
// "" when an expose member can name it.
func (m *migration) exposeNote(s *sysmlv1.Element) string {
	switch {
	case s.IsProxy():
		return "the exposed " + qualifiedName(s) + " lives outside the document and is not written"
	case !m.written(s):
		_, why := m.classify(s)
		return joinNotes("the exposed "+kindOf(s)+" "+describe(s)+" is not migrated and is not written", why)
	}
	return ""
}

// exposeRef writes what a view exposes: a namespace with its contents, as v1
// exposes a package's members with it, any other element by itself.
func (m *migration) exposeRef(view, s *sysmlv1.Element) string {
	ref := m.memberRef(s, view)
	switch s.Type {
	case "Package", "Model":
		return ref + "::**"
	}
	return ref
}

// absentNote says what an unresolved reference to id was: notation the tool
// keeps outside the model, or nothing in the document.
func (m *migration) absentNote(id string) string {
	for _, ext := range m.model.Extensions {
		for _, el := range ext.Elements {
			if el.ID != id {
				continue
			}
			name := el.Name
			if name == "" {
				name = id
			}
			return "the exposed " + strings.TrimPrefix(el.Type, "uml:") + " '" + name + "' is notation the tool keeps outside the model, which v2 does not carry"
		}
	}
	return "the exposed element " + id + " is not in the document"
}

// viewpointBody writes a viewpoint usage: its doc, the tags v2 has a slot for,
// its members and its stereotype tags.
func (m *migration) viewpointBody(e *sysmlv1.Element) {
	saved := m.scope
	m.scope = e
	concerns := m.framedComments(e)
	m.comments(e)
	m.viewpointTags(e, concerns)
	m.members(e)
	m.classifierBehavior(e)
	m.stereotypeComments(e)
	m.scope = saved
}

// viewpointTags writes the «Viewpoint» tags: the purpose as doc, joined by the
// language, method and presentation v2 has no slot for; the stakeholders as
// stakeholder usages; the concerns, as text or comments, as framed concerns.
func (m *migration) viewpointTags(e *sysmlv1.Element, concerns []*sysmlv1.Element) {
	vp := stereo(e, "Viewpoint")
	if vp == nil {
		return
	}
	var doc []string
	if p := vp.Tag("purpose"); p != "" {
		doc = append(doc, p)
	}
	for _, tag := range []string{"language", "method", "presentation"} {
		if vs := vp.Tags[tag]; len(vs) > 0 {
			doc = append(doc, tag+": "+strings.Join(m.tagValues(vs), ", "))
		}
	}
	if len(doc) > 0 {
		m.w.lines(prefixFirst("doc ", commentLines(strings.Join(doc, "\n"))))
	}
	subject := false
	for _, id := range vp.IDs("stakeholder") {
		if !subject && m.stakeholderWritable(e, id) {
			m.w.line("subject;")
			m.add(e, Mapped, "", subjectNote("stakeholders"))
			subject = true
		}
		m.stakeholder(e, id)
	}
	for _, concern := range vp.Tags["concern"] {
		m.frameConcern(concern)
	}
	for _, c := range concerns {
		text := commentBody(c)
		if text == "" {
			m.add(c, Skipped, "", "empty comment")
			continue
		}
		m.frameConcern(text)
		m.add(c, Mapped, "", "")
	}
}

// framedComments resolves the comments a viewpoint's concernList tag names and
// marks them framed, so they are written as concerns rather than comments.
func (m *migration) framedComments(e *sysmlv1.Element) []*sysmlv1.Element {
	vp := stereo(e, "Viewpoint")
	if vp == nil {
		return nil
	}
	var concerns []*sysmlv1.Element
	for _, id := range vp.IDs("concernList") {
		c := m.model.Lookup(id)
		if c == nil {
			m.downgrade(e, "the concernList tag names "+id+", which is not in the document")
			continue
		}
		if c.Type != "Comment" {
			m.downgrade(e, "the concernList tag names the "+kindOf(c)+" "+qualifiedName(c)+", which is not a comment and frames no concern")
			continue
		}
		m.framed[c] = true
		concerns = append(concerns, c)
	}
	return concerns
}

// subjectNote explains the anonymous subject written before a case's or
// viewpoint's other parameters, which v2 places after the subject.
func subjectNote(params string) string {
	return "v2 places the subject before the " + params + ", so an anonymous subject is declared"
}

// stakeholderWritable reports whether the stakeholder tag id becomes a usage.
func (m *migration) stakeholderWritable(vp *sysmlv1.Element, id string) bool {
	s := m.model.Lookup(id)
	if s == nil || s.IsProxy() || !m.written(s) {
		return false
	}
	cat, _ := m.classify(s)
	return cat == catPartDef
}

// stakeholder writes one stakeholder usage of a viewpoint, typed by the part
// def the stakeholder class becomes.
func (m *migration) stakeholder(vp *sysmlv1.Element, id string) {
	s := m.model.Lookup(id)
	switch {
	case s == nil:
		m.downgrade(vp, "the stakeholder tag names "+id+", which is not in the document")
		return
	case s.IsProxy():
		m.downgrade(vp, "the stakeholder "+qualifiedName(s)+" lives outside the document and is not written")
		return
	case !m.written(s):
		m.downgrade(vp, "the stakeholder "+qualifiedName(s)+" is not migrated and is not written")
		return
	}
	if cat, _ := m.classify(s); cat != catPartDef {
		m.downgrade(vp, "the stakeholder "+qualifiedName(s)+" becomes a "+cat.keyword()+", which cannot type a stakeholder")
		return
	}
	name := m.freshName(vp, lowerFirst(m.nameFor(s)))
	m.w.line("stakeholder " + writeName(name) + " : " + m.ref(s, vp) + ";")
}

// frameConcern writes a concern a viewpoint frames, documented by its text.
func (m *migration) frameConcern(text string) {
	m.w.block("frame concern", func() {
		m.w.lines(prefixFirst("doc ", commentLines(text)))
	})
}
