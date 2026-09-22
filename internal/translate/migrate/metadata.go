package migrate

import (
	"math/big"
	"slices"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// A user profile's stereotypes are written as metadata defs in the profile's
// package, and their applications as metadata usages in the body of the element
// applied to. The standard profiles' stereotypes classify what an element
// becomes instead, and the modeling tool's own configure the tool, not the model.

// enclosingProfile returns the nearest profile e sits in, or nil.
func enclosingProfile(e *sysmlv1.Element) *sysmlv1.Element {
	for cur := e.Parent; cur != nil; cur = cur.Parent {
		if cur.Type == "Profile" {
			return cur
		}
	}
	return nil
}

// userStereotype reports whether d is a stereotype defined in the document
// outside the standard profiles and libraries, which is written as a metadata def.
func (m *migration) userStereotype(d *sysmlv1.Element) bool {
	return d != nil && d.Type == "Stereotype" && !d.IsProxy() && !m.isLibrary(d)
}

// metadataGenerals writes the metadata defs a user stereotype specializes: the
// user stereotypes among its generals, less one closing a cycle. A standard general
// classifies the applications instead, and a general outside the document is noted.
func (m *migration) metadataGenerals(e *sysmlv1.Element) (string, string) {
	m.defsWritten[e] = true
	var refs, notes []string
	for _, g := range e.Owned("generalization") {
		target := m.model.Ref(g, "general")
		switch {
		case target == nil:
			notes = append(notes, "a generalization refers to nothing in the document")
		case isStandardDefinition(target):
			// Carried by the applications, which take the standard stereotype's v2 form.
		case m.userStereotype(target) && m.defsWritten[target] && slices.Contains(m.model.Ancestors(target), e):
			notes = append(notes, "generalization of "+qualifiedName(target)+" is not written: it closes a cycle of generalizations")
		case m.userStereotype(target):
			refs = append(refs, m.ref(target, m.scope))
		default:
			notes = append(notes, "generalization of "+qualifiedName(target)+" is not written: it is not a user stereotype defined in the document")
		}
	}
	return strings.Join(refs, ", "), strings.Join(notes, "; ")
}

// metadataBody writes the members of a metadata def: its tag definitions as
// attributes or references, and whatever else the stereotype owns.
func (m *migration) metadataBody(e *sysmlv1.Element) {
	saved := m.scope
	m.scope = e
	m.comments(e)
	for _, c := range e.Children {
		if c.Role == "ownedAttribute" {
			m.metadataAttribute(c)
			continue
		}
		m.member(c)
	}
	for _, extra := range m.extras[e] {
		extra()
	}
	m.stereotypeAnnotations(e)
	m.scope = saved
}

// extensionEnd reports whether p is the end of an extension, naming the
// metaclass its stereotype extends, which v2 metadata does not restrict.
func extensionEnd(p *sysmlv1.Element) bool {
	return strings.HasPrefix(p.Name, "base_")
}

// metadataAttribute writes one tag definition of a metadata def: an attribute
// typed by a value type, or a ref to the elements the tag refers to.
func (m *migration) metadataAttribute(p *sysmlv1.Element) {
	if extensionEnd(p) {
		m.add(p, Skipped, "", "an extension end names the metaclass the stereotype extends; v2 metadata applies to any element")
		return
	}
	kw, typ, note := m.metadataFeature(p)
	name := m.nameOf(p)
	if name == "" {
		name = m.nameFor(p)
		note = joinNotes(note, "the anonymous property is named "+name)
	}
	decl := kw + " " + writeName(name)
	if typ != "" {
		decl += " : " + typ
	}
	mult, mnote := m.declaredMultiplicity(p)
	note = joinNotes(note, mnote)
	decl += mult + collection(p)
	m.add(p, verdictFor(note), m.v2Name(p), note)
	m.w.block(decl, func() {
		saved := m.scope
		m.scope = p
		m.comments(p)
		m.stereotypeAnnotations(p)
		m.scope = saved
	})
}

// metadataFeature decides how a tag definition is written: as an attribute
// when its type is a value type v2 has, as a ref to elements otherwise.
func (m *migration) metadataFeature(p *sysmlv1.Element) (kw, typ, note string) {
	t := m.model.Ref(p, "type")
	if t == nil {
		if len(m.model.Unresolved(p, "type")) > 0 {
			return "ref", "", "the type refers to nothing in the document"
		}
		return "attribute", "", ""
	}
	if sv := m.scalarValue(t); sv != "" {
		typ, note = m.typeRef(t, m.scope)
		return "attribute", typ, note
	}
	if t.IsProxy() {
		if isStandardHref(t.Href) || libraryReference(t) {
			return "ref", "", ""
		}
		return "ref", "", "type " + qualifiedName(t) + " lives outside the document and is not written"
	}
	switch cat, _ := m.classify(t); cat {
	case catAttributeDef, catEnumDef:
		return "attribute", m.ref(t, m.scope), ""
	case catLibrary, catUnmapped, catNone:
		return "ref", "", "type " + qualifiedName(t) + " is not migrated and is not written"
	}
	return "ref", m.ref(t, m.scope), ""
}

// tagDefinition finds the property of a user stereotype, or of a user
// stereotype it specializes, that defines the tag a tool spelled as name.
func (m *migration) tagDefinition(def *sysmlv1.Element, name string) *sysmlv1.Element {
	chain := append([]*sysmlv1.Element{def}, m.model.Ancestors(def)...)
	for _, d := range chain {
		if !m.userStereotype(d) {
			continue
		}
		for _, p := range d.Owned("ownedAttribute") {
			if !extensionEnd(p) && sysmlv1.SameName(p.Name, name) {
				return p
			}
		}
	}
	return nil
}

// metadataUsage writes the application s of a user stereotype as a metadata
// usage in the body of e, with the tags not consumed by a standard stereotype
// it specializes as the usage's values.
func (m *migration) metadataUsage(e *sysmlv1.Element, s *sysmlv1.Stereotype, consumed map[string]bool) {
	def := s.Definition
	var values, notes []string
	keys := make([]string, 0, len(s.Tags))
	for k := range s.Tags {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if consumed[k] {
			continue
		}
		p := m.tagDefinition(def, k)
		if p == nil {
			notes = append(notes, "«"+s.Name+"» tag "+k+" is not defined by the stereotype and is kept as a comment")
			values = append(values, commentLines(k+" = "+strings.Join(m.tagValues(s.Tags[k]), ", "))...)
			continue
		}
		value, note := m.tagValue(p, s.Tags[k])
		if note != "" {
			notes = append(notes, "«"+s.Name+"» tag "+k+": "+note)
		}
		if value == "" {
			values = append(values, commentLines(m.nameOf(p)+" = "+strings.Join(m.tagValues(s.Tags[k]), ", "))...)
			continue
		}
		values = append(values, writeName(m.nameOf(p))+" = "+value+";")
	}
	m.w.block("@"+m.ref(def, m.scope), func() { m.w.lines(values) })
	for _, n := range notes {
		m.downgrade(e, n)
	}
}

// tagValue writes the values of a tag as the expression its definition p types:
// a literal for a value type, an enumeration literal by name, a reference to
// the element written; "" with a note when a value has no such form.
func (m *migration) tagValue(p *sysmlv1.Element, raw []string) (string, string) {
	kw, _, _ := m.metadataFeature(p)
	t := m.model.Ref(p, "type")
	var exprs []string
	for _, v := range raw {
		if kw == "ref" {
			for _, id := range strings.Fields(v) {
				expr, note := m.tagReference(id)
				if note != "" {
					return "", note
				}
				exprs = append(exprs, expr)
			}
			continue
		}
		expr, note := m.tagLiteral(t, v)
		if note != "" {
			return "", note
		}
		exprs = append(exprs, expr)
	}
	switch len(exprs) {
	case 0:
		return "", ""
	case 1:
		return exprs[0], ""
	}
	return "(" + strings.Join(exprs, ", ") + ")", ""
}

// tagReference writes a reference-valued tag's value: the written element it
// names, by the shortest name resolving in the current scope.
func (m *migration) tagReference(id string) (string, string) {
	target := m.model.Lookup(id)
	switch {
	case target == nil:
		return "", "refers to " + id + ", which is not in the document"
	case !m.written(target):
		return "", "refers to " + qualifiedName(target) + ", which is not written"
	}
	return m.ref(target, m.scope), ""
}

// tagLiteral writes one value of a tag typed by t: a string, number or boolean
// literal, an enumeration literal, or a string when the tag is untyped.
func (m *migration) tagLiteral(t *sysmlv1.Element, v string) (string, string) {
	if t != nil && t.Type == "Enumeration" && !t.IsProxy() {
		for _, lit := range t.Owned("ownedLiteral") {
			if lit.ID == v || lit.Name == v || sysmlv1.SameName(lit.Name, v) {
				return m.ref(lit, m.scope), ""
			}
		}
		return "", "the value " + v + " is not a literal of " + qualifiedName(t)
	}
	sv := m.scalarValue(t)
	text := strings.TrimSpace(v)
	switch sv {
	case "", "String":
		return source.StringText(commentText(v)), ""
	case "Boolean":
		switch text {
		case "true", "false":
			return text, ""
		}
		return "", "the value " + v + " is not a boolean"
	case "Integer", "Natural":
		n, ok := new(big.Int).SetString(text, 10)
		if !ok || (sv == "Natural" && n.Sign() < 0) {
			return "", "the value " + v + " is not an integer"
		}
		return n.String(), ""
	}
	if !decimal(text) {
		return "", "the value " + v + " is not a number"
	}
	if _, ok := new(big.Rat).SetString(text); !ok {
		return "", "the value " + v + " is not a number"
	}
	if !strings.ContainsAny(text, ".eE") {
		text += ".0"
	}
	return text, ""
}
