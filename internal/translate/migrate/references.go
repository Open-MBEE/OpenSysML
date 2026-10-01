package migrate

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// A cross-reference in a comment body resolves through the model's index: the
// View Editor's element ids and MagicDraw's hyperlink ids are the xmi:ids of
// the export. Resolved, a reference to an element's name is written as a
// reference to the element where the comment becomes a Paragraph, and as the
// element's name where the comment is plain text; a reference to a value or
// to documentation is the text it has at migration time. A reference whose id
// the export does not contain prints what the tool cached for it — nothing,
// for the View Editor's placeholder — and is noted on the comment.

// docRun is one run of a migrated Paragraph: text, or a reference to an
// element or a diagram of the export.
type docRun struct {
	text    string
	elem    *sysmlv1.Element
	diagram *sysmlv1.Diagram
	// named marks a reference the author made to the target's name, which
	// a Ref then shows in place of the label of the block it links.
	named bool
}

// isRef reports whether the run references an element or a diagram.
func (r docRun) isRef() bool { return r.elem != nil || r.diagram != nil }

// resolveProse resolves the cross-references of a body's runs. In a
// Paragraph (inline), a reference to a name becomes a reference run and a
// dangling placeholder is left out; in plain text, each is the text it prints.
// The notes say what the ledger records of the references.
func (m *migration) resolveProse(runs []proseRun, inline bool) (out []docRun, notes []string) {
	for _, r := range runs {
		if !r.isRef() {
			out = append(out, docRun{text: r.text})
			continue
		}
		run, note := m.resolveRef(r, inline)
		out = append(out, run)
		if note != "" && !contains(notes, note) {
			notes = append(notes, note)
		}
	}
	return out, notes
}

// resolveRef resolves one cross-reference, see resolveProse.
func (m *migration) resolveRef(r proseRun, inline bool) (docRun, string) {
	e, d := m.lookupRef(r.id)
	if e == nil && d == nil {
		return m.danglingRef(r, inline)
	}
	switch r.cf {
	case cfValue, cfDoc:
		if m.resolving[r.id] {
			return docRun{}, "a cross-reference to the " + cfWhat(r.cf) + " of " + refTarget(e, d) + " refers back to the text being written, so nothing stands for it"
		}
		m.resolving[r.id] = true
		defer delete(m.resolving, r.id)
		var text, note string
		if r.cf == cfValue {
			text, note = m.valueText(e, d)
		} else {
			text, note = m.documentationText(e, d)
		}
		return docRun{text: text}, note
	}
	name := refName(e, d)
	var note string
	if cached := cachedText(r); cached != "" && strings.Join(strings.Fields(cached), " ") != strings.Join(strings.Fields(name), " ") {
		note = refSubject(r) + " reads '" + cached + "', not the current name '" + name + "' of " + refTarget(e, d) + ", which is written"
	}
	if inline {
		return docRun{elem: e, diagram: d, named: r.cf == cfName}, note
	}
	return docRun{text: name}, note
}

// danglingRef is what a reference to an id outside the export prints: a
// hyperlink's or view link's text; a View Editor reference's fallback as
// plain text, and nothing in a Paragraph, as the tool printed.
func (m *migration) danglingRef(r proseRun, inline bool) (docRun, string) {
	note := refSubject(r) + " names an element the export does not contain: " + r.id
	text := cachedText(r)
	linked := r.link || r.cf == cfView
	if !linked && inline {
		text = ""
	}
	switch {
	case text == "":
		note += "; nothing stands for it"
	case linked:
		note += "; its text stands"
	default:
		note += "; its cached text '" + text + "' stands"
	}
	return docRun{text: text}, note
}

// lookupRef finds the element or diagram an id names in the export.
func (m *migration) lookupRef(id string) (*sysmlv1.Element, *sysmlv1.Diagram) {
	if e := m.model.Lookup(id); e != nil && !e.IsProxy() {
		return e, nil
	}
	if d := m.model.Diagram(id); d != nil {
		return nil, d
	}
	return nil, nil
}

// refName is the name of what a reference resolves to.
func refName(e *sysmlv1.Element, d *sysmlv1.Diagram) string {
	if d != nil {
		return d.Name
	}
	return e.Name
}

// refTarget describes what a reference resolves to, for a note.
func refTarget(e *sysmlv1.Element, d *sysmlv1.Diagram) string {
	if d != nil {
		return "the diagram '" + d.Name + "'"
	}
	return describe(e)
}

// refSubject names a reference in a note.
func refSubject(r proseRun) string {
	switch {
	case r.link:
		return "the hyperlink '" + r.text + "'"
	case r.cf == cfView:
		return "a link to a view"
	}
	return "a cross-reference to the " + cfWhat(r.cf) + " of an element"
}

// cfWhat says what a View Editor reference shows of its element.
func cfWhat(cf string) string {
	switch cf {
	case cfValue:
		return "value"
	case cfDoc:
		return "documentation"
	}
	return "name"
}

// valueText is the text of a reference to the value of a slot or a property:
// the literal as written, an instance's name, else the value's v2 expression.
// A target of another kind, or a value without text, leaves nothing, noted.
func (m *migration) valueText(e *sysmlv1.Element, d *sysmlv1.Diagram) (string, string) {
	const subject = "a cross-reference to the value of "
	if d != nil {
		return "", subject + refTarget(e, d) + " has no text: a diagram has no value"
	}
	var values []*sysmlv1.Element
	var scope, feature *sysmlv1.Element
	switch e.Type {
	case "Slot":
		values, scope, feature = e.Owned("value"), e.Parent, m.model.Ref(e, "definingFeature")
	case "Property", "Port", "Parameter":
		values, scope, feature = e.Owned("defaultValue"), e.Parent, e
	default:
		return "", subject + describe(e) + " has no text: " + article(e.Type) + e.Type + " has no value"
	}
	// A slot whose defining feature the export lacks is described as itself.
	target := feature
	if target == nil {
		target = e
	}
	var texts, notes []string
	for _, v := range values {
		text, nested, ok := m.literalText(v, feature, scope)
		if !ok {
			return "", subject + refTarget(target, nil) + " has no text: its value " + v.Type + " has no literal form"
		}
		texts = append(texts, text)
		notes = append(notes, nested...)
	}
	if len(texts) == 0 {
		return "", subject + refTarget(target, nil) + " has no text: it holds no value"
	}
	note := subject + refTarget(target, nil) + " is written as the text of its value at migration time"
	for _, n := range notes {
		note = joinNotes(note, n)
	}
	return strings.Join(texts, ", "), note
}

// literalText is a value's text: a literal as written, its cross-references
// resolved and what the ledger records of them returned; an instance's name;
// or the v2 expression of a value of another form.
func (m *migration) literalText(v, f, scope *sysmlv1.Element) (string, []string, bool) {
	switch v.Type {
	case "LiteralString", "LiteralInteger", "LiteralReal", "LiteralBoolean", "LiteralUnlimitedNatural":
		if text, ok := v.Attrs["value"]; ok {
			text, notes := m.proseNoted(text)
			return text, notes, true
		}
	case "InstanceValue":
		if inst := m.model.Ref(v, "instance"); inst != nil && inst.Name != "" {
			return inst.Name, nil, true
		}
	}
	if f == nil {
		return "", nil, false
	}
	expr, ok, _ := m.featureValue(v, f, scope)
	return expr, nil, ok
}

// documentationText is the text of a reference to an element's
// documentation: its doc comment's text — the first comment of its own with
// any, as docComment finds it — or a comment's own body. The reference's id
// is marked resolving by the caller; the comment is checked before it is
// read, since a reference to the element it documents reaches it while it
// is being written.
func (m *migration) documentationText(e *sysmlv1.Element, d *sysmlv1.Diagram) (string, string) {
	const subject = "a cross-reference to the documentation of "
	if d != nil {
		text := m.proseText(d.Documentation, nil)
		if text == "" {
			return "", subject + refTarget(e, d) + " has no text: the diagram has no documentation"
		}
		return text, subject + refTarget(e, d) + " is written as its text at migration time"
	}
	comments := []*sysmlv1.Element{e}
	if e.Type != "Comment" {
		comments = comments[:0]
		for _, c := range e.Owned("ownedComment") {
			if !m.framed[c] && !m.annotatesOthers(c, e) {
				comments = append(comments, c)
			}
		}
	}
	var text string
	for _, c := range comments {
		if m.resolving[c.ID] {
			return "", subject + describe(e) + " refers back to the text being written, so nothing stands for it"
		}
		if text = m.commentBody(c); text != "" {
			break
		}
	}
	if text == "" {
		return "", subject + describe(e) + " has no text: the element has no documentation"
	}
	return text, subject + describe(e) + " is written as its text at migration time"
}

// proseText renders a body as plain text, its cross-references resolved; what
// the ledger records of them is noted on owner, when there is one. The owner
// is marked resolving meanwhile, so a reference that reaches back to it ends.
func (m *migration) proseText(body string, owner *sysmlv1.Element) string {
	if owner != nil {
		m.resolving[owner.ID] = true
		defer delete(m.resolving, owner.ID)
	}
	runs, notes := m.resolveProse(parseProse(body), false)
	if owner != nil {
		for _, n := range notes {
			m.annotate(owner, n, true)
		}
	}
	return runsText(runs)
}

// proseNoted renders a body as plain text, its cross-references resolved,
// and returns what the ledger records of them.
func (m *migration) proseNoted(body string) (string, []string) {
	runs, notes := m.resolveProse(parseProse(body), false)
	return runsText(runs), notes
}

// runsText flattens resolved runs to text, a reference as its name.
func runsText(runs []docRun) string {
	var b strings.Builder
	refs := false
	for _, r := range runs {
		switch {
		case r.diagram != nil:
			b.WriteString(r.diagram.Name)
			refs = true
		case r.elem != nil:
			b.WriteString(r.elem.Name)
			refs = true
		default:
			b.WriteString(r.text)
		}
	}
	if !refs && len(runs) == 1 {
		return runs[0].text
	}
	return strings.TrimSpace(tidyText(b.String(), true))
}

// commentBody reads a comment's text, from its body attribute or child
// element, its cross-references resolved and noted on the comment.
func (m *migration) commentBody(c *sysmlv1.Element) string {
	return m.proseText(commentRawBody(c), c)
}

// paragraphProse plans a Paragraph's body: its text, and its runs when a
// cross-reference in it is written as a reference.
func (m *migration) paragraphProse(cp *contentPlan, body string) {
	runs, notes := m.resolveProse(parseProse(body), true)
	runs, glued := wordRefs(mergeText(runs))
	cp.text = runsText(runs)
	cp.notes = append(cp.notes, notes...)
	cp.notes = append(cp.notes, glued...)
	for _, r := range runs {
		if r.isRef() {
			cp.runs = runs
			return
		}
	}
}

// wordRefs writes as text each reference that runs into the text beside it
// with no space or binding punctuation between — "pre<a>fix</a>ed",
// "<a>Mount</a>'s" — since a Paragraph's runs are joined by spaces, which
// would split the word; the notes say which. The runs are merged.
func wordRefs(runs []docRun) ([]docRun, []string) {
	var notes []string
	demoted := false
	for i, r := range runs {
		if !r.isRef() {
			continue
		}
		glued := false
		if i > 0 {
			prev := runs[i-1]
			glued = prev.isRef() || (prev.text != "" && !endsSpaced(prev.text) && !docplan.BindsRight(prev.text))
		}
		if i+1 < len(runs) && !glued {
			next := runs[i+1]
			glued = next.isRef() || (next.text != "" && !startsSpaced(next.text) && !docplan.BindsLeft(next.text))
		}
		if !glued {
			continue
		}
		name := refName(r.elem, r.diagram)
		runs[i] = docRun{text: name}
		demoted = true
		note := "the reference to " + refTarget(r.elem, r.diagram) + " runs into the word around it, so its name is written as text"
		if !contains(notes, note) {
			notes = append(notes, note)
		}
	}
	if demoted {
		runs = mergeText(runs)
	}
	return runs, notes
}

// startsSpaced and endsSpaced report whether text opens, or closes, with
// whitespace.
func startsSpaced(text string) bool { return strings.TrimLeft(text, " \t\n") != text }
func endsSpaced(text string) bool   { return strings.TrimRight(text, " \t\n") != text }

// mergeText joins neighbouring text runs into one and drops empty ones, so
// the text between two references is a single run and a placeholder left out
// leaves no gap.
func mergeText(runs []docRun) []docRun {
	var out []docRun
	for _, r := range runs {
		switch {
		case r.isRef():
			out = append(out, r)
		case r.text == "":
		case len(out) > 0 && !out[len(out)-1].isRef():
			out[len(out)-1].text = joinText(out[len(out)-1].text, r.text)
		default:
			out = append(out, r)
		}
	}
	return out
}

// writeRuns writes a Paragraph's runs as its parts: each text run as a Span
// and each reference as a Ref to the block of the document standing for its
// target — the section of a view, the figure of a diagram — else to the
// migrated element itself. A reference to an element the migration does not
// write is its name, in a Span, noted on the paragraph.
func (m *migration) writeRuns(dp *docPlan, cp *contentPlan) {
	names := columnNames{}
	for _, r := range cp.runs {
		text, target := r.text, ""
		if r.isRef() {
			var note string
			target, note = m.runTarget(dp, r)
			if note != "" && !contains(cp.notes, note) {
				cp.notes = append(cp.notes, note)
			}
			text = refName(r.elem, r.diagram)
		} else {
			text = tidyText(strings.Trim(text, " \t"), false)
		}
		if target != "" {
			m.blockPart(dp.host, names.claim("ref"), "Ref", nil, func() {
				m.w.line("ref redefines target = " + target + ";")
				if r.named {
					m.w.line("attribute redefines text = " + stringLiteral(text) + ";")
				}
			})
			continue
		}
		if text == "" {
			continue
		}
		m.blockPart(dp.host, names.claim("span"), "Span", nil, func() {
			m.w.line("attribute redefines text = " + stringLiteral(text) + ";")
		})
	}
}

// runTarget is the reference written for a resolved run from inside dp's
// document: the block of the document standing for the target, else the view
// of the diagram or the element itself, through its metadata — the one form
// that names any element as a value, a feature nested in a definition
// included, where a name alone must be an accessible feature; "" with a note
// when nothing written stands for it.
func (m *migration) runTarget(dp *docPlan, r docRun) (string, string) {
	if path := blockPath(dp.root, dp.target, "::", r); path != "" {
		return path, ""
	}
	if r.diagram != nil {
		if v := m.viewOf[r.diagram]; v != nil && v.placed {
			return m.viewRef(v, dp.host) + ".metadata", ""
		}
		return "", refTarget(nil, r.diagram) + " it references is not written, so its name stands"
	}
	if !m.written(r.elem) {
		return "", refTarget(r.elem, nil) + " it references is not written, so its name stands"
	}
	ref := m.ref(r.elem, dp.host)
	if ref == "" {
		return "", refTarget(r.elem, nil) + " it references has no name to refer to, so its name stands"
	}
	return ref + ".metadata", ""
}

// blockPath names the block under sec, itself at path, that stands for the
// run's target: the section of its view, or the figure, table or list made of
// its node; "" when none does. The document's own members are named with sep,
// `::` from the document, and the blocks under them by dot notation, the way
// a nested feature is reached.
func blockPath(sec *sectionPlan, path, sep string, r docRun) string {
	for _, cp := range sec.content {
		if cp.refused != "" {
			continue
		}
		own := path + sep + writeName(cp.name)
		if r.diagram != nil && cp.diagram == r.diagram && (cp.kind == "Diagram" || cp.kind == "Image") {
			return own
		}
		if r.elem != nil && cp.node == r.elem && !cp.documentation {
			return own
		}
		if cp.section != nil {
			if r.elem != nil && cp.section.v.Class == r.elem {
				return own
			}
			if found := blockPath(cp.section, own, ".", r); found != "" {
				return found
			}
		}
	}
	for _, child := range sec.children {
		own := path + sep + writeName(child.name)
		if r.elem != nil && child.v.Class == r.elem {
			return own
		}
		if found := blockPath(child, own, ".", r); found != "" {
			return found
		}
	}
	return ""
}
