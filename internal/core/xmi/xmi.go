// Package xmi reads UML 2.x models serialized as XMI, as SysML v1 tools export
// them, into a generic element tree with the stereotypes applied to it.
//
// The reader is deliberately tolerant of the serialization's dialect: the UML,
// XMI and profile namespaces differ between the OMG normative XMI and the
// Eclipse UML2 serialization Papyrus writes, so elements are classified by the
// local part of their xmi:type and stereotype applications by the local part
// of their element name.
//
// The tree carries no UML semantics of its own; internal/core/migrate
// interprets it as a SysML v1 model.
package xmi

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Element is one node of the model: a UML element with an xmi:id, or a proxy
// for an element in another document that an href points at.
type Element struct {
	// ID is the xmi:id, or the href for a proxy of an external element.
	ID string
	// Type is the local name of the xmi:type, such as "Class" or "Property".
	// A proxy carries the type its reference declared, or "".
	Type string
	// Role is the XML element name that owns this element in its parent, such
	// as "packagedElement" or "ownedAttribute"; empty for a root.
	Role string
	// Name is the element's name attribute; a proxy takes the href fragment
	// when it reads as a name.
	Name string
	// Href is set on a proxy for an element of another document.
	Href string
	// Attrs holds the XML attributes other than xmi:id and xmi:type, keyed by
	// local name; references appear as their raw id text.
	Attrs map[string]string
	// Text is the element's character content, which stereotype tag values
	// and opaque expression bodies are written as.
	Text     string
	Parent   *Element
	Children []*Element
	// Stereotypes are the stereotype applications whose base is this element.
	Stereotypes []*Stereotype
	// refs are child reference elements (xmi:idref or href) by role.
	refs map[string][]string
}

// Stereotype is one stereotype application: an element outside the UML model
// whose base_* attribute names the element it extends.
type Stereotype struct {
	ID string
	// Name is the stereotype's local name, such as "Block" or "Requirement".
	Name string
	// Namespace is the XML namespace the profile was serialized under.
	Namespace string
	// BaseID is the value of the base_* attribute; Base resolves it.
	BaseID string
	Base   *Element
	// Tags holds the tagged values: attributes other than xmi:* and base_*, and
	// child elements as their text or idref, keyed by tag name. A multi-valued
	// tag lists each value.
	Tags map[string][]string
}

// Tag returns the first value of a tag, or "".
func (s *Stereotype) Tag(name string) string {
	if v := s.Tags[name]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// Model is one XMI document.
type Model struct {
	// Roots are the top-level UML elements, in document order.
	Roots []*Element
	// Stereotypes are every stereotype application, in document order.
	Stereotypes []*Stereotype
	// Exporter records the xmi:Documentation exporter, when the document names one.
	Exporter string
	// Extensions are the tool-private xmi:Extension blocks that were skipped.
	Extensions []Extension
	byID       map[string]*Element
	proxies    map[string]*Element
}

// Extension records one skipped xmi:Extension: who wrote it and what it held.
type Extension struct {
	// Extender is the tool named by the block's extender attribute.
	Extender string
	// Owner is the element the block sits in; nil at the document root.
	Owner *Element
	// Elements are the xmi:type and name of every typed element inside, e.g. "uml:Diagram Vehicle BDD".
	Elements []ExtensionElement
}

// ExtensionElement is one typed element inside a skipped extension.
type ExtensionElement struct {
	ID, Type, Name string
}

// Lookup resolves an xmi:id, or an href of an element another document holds,
// to its element; nil when the document defines neither.
func (m *Model) Lookup(id string) *Element {
	if e, ok := m.byID[id]; ok {
		return e
	}
	return m.proxies[id]
}

// Refs returns the elements a role of e refers to: the ids in the attribute of
// that name (space-separated, as XMI writes multi-valued references) and the
// child elements of that name carrying xmi:idref or href. Unresolvable ids are
// dropped (Unresolved lists them); an href yields a proxy element.
func (m *Model) Refs(e *Element, role string) []*Element {
	ids := e.refIDs(role)
	out := make([]*Element, 0, len(ids))
	for _, id := range ids {
		if target := m.Lookup(id); target != nil {
			out = append(out, target)
		}
	}
	return out
}

// Unresolved returns the ids a role of e refers to that no read document
// defines, so a caller can tell a complete reference list from a dangling one.
func (m *Model) Unresolved(e *Element, role string) []string {
	var out []string
	for _, id := range e.refIDs(role) {
		if m.Lookup(id) == nil {
			out = append(out, id)
		}
	}
	return out
}

// refIDs lists the raw ids a role of e refers to, attribute ids first.
func (e *Element) refIDs(role string) []string {
	var ids []string
	if v, ok := e.Attrs[role]; ok {
		ids = append(ids, strings.Fields(v)...)
	}
	return append(ids, e.refs[role]...)
}

// Ref returns the first element a role refers to, or nil.
func (m *Model) Ref(e *Element, role string) *Element {
	if refs := m.Refs(e, role); len(refs) > 0 {
		return refs[0]
	}
	return nil
}

// Owned returns the children of e in a role, such as its "ownedAttribute"s.
func (e *Element) Owned(role string) []*Element {
	var out []*Element
	for _, c := range e.Children {
		if c.Role == role {
			out = append(out, c)
		}
	}
	return out
}

// Stereotype returns e's application of the named stereotype, or nil.
func (e *Element) Stereotype(name string) *Stereotype {
	for _, s := range e.Stereotypes {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// HasStereotype reports whether any of the named stereotypes applies to e.
func (e *Element) HasStereotype(names ...string) bool {
	for _, n := range names {
		if e.Stereotype(n) != nil {
			return true
		}
	}
	return false
}

// IsProxy reports whether e stands for an element of another document.
func (e *Element) IsProxy() bool { return e.Href != "" }

// Path returns the names from the root to e, for diagnostics; anonymous
// elements contribute their type in angle brackets.
func (e *Element) Path() []string {
	var names []string
	for cur := e; cur != nil; cur = cur.Parent {
		name := cur.Name
		if name == "" {
			name = "<" + cur.Type + ">"
		}
		names = append([]string{name}, names...)
	}
	return names
}

// Parse reads an XMI document into a Model.
func Parse(data []byte) (*Model, error) {
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return nil, errArchive
	}
	m := &Model{byID: map[string]*Element{}, proxies: map[string]*Element{}}
	if err := m.parseDocument(data); err != nil {
		return nil, err
	}
	if len(m.Roots) == 0 {
		return nil, errNoModel
	}
	m.link()
	return m, nil
}

// errNoModel reports an XMI document holding no model element.
var errNoModel = errors.New("the XMI document holds no model: expected a uml:Model or uml:Package under the xmi:XMI root")

// errArchive reports a zip archive: a tool's project container, not an XMI document.
var errArchive = errors.New("the input is a zip archive, not an XMI document: export the model as XMI 2.5.1 and migrate that file")

// local returns the local part of an "prefix:name" value.
func local(s string) string {
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// errNotXMI reports a document whose root is not xmi:XMI or a UML element.
var errNotXMI = errors.New("not an XMI document: expected an xmi:XMI or uml:Model root element")

// parseDocument reads one document's elements and stereotype applications.
func (m *Model) parseDocument(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	p := &docParser{m: m, dec: dec}
	return p.run()
}

// docParser holds the state of one document's streaming parse.
type docParser struct {
	m   *Model
	dec *xml.Decoder
	// stack is the open element for each open XML element below the root; nil
	// entries are reference or skipped elements that own nothing.
	stack []*Element
	// stereo is the stereotype application being read, when inside one, and
	// stereoDepth the stack depth its own element sits at.
	stereo      *Stereotype
	stereoDepth int
	// tag is the open tag-value element of stereo, and tagText its content.
	tag     string
	tagText strings.Builder
	depth   int
	sawRoot bool
}

func (p *docParser) run() error {
	for {
		tok, err := p.dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("parsing XMI: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if err := p.start(t); err != nil {
				return err
			}
		case xml.CharData:
			p.chars(t)
		case xml.EndElement:
			p.end()
		}
	}
	if !p.sawRoot {
		return errNotXMI
	}
	return nil
}

func (p *docParser) start(t xml.StartElement) error {
	p.depth++
	name := t.Name.Local
	if p.depth == 1 {
		p.sawRoot = true
		if name == "XMI" && isXMINamespace(t.Name.Space) {
			// The document root wraps the model; its children are top-level.
			p.stack = append(p.stack, nil)
			return nil
		}
		if !isUMLNamespace(t.Name.Space) && !hasXMIType(t.Attr) {
			return errNotXMI
		}
	}
	if isXMINamespace(t.Name.Space) {
		if name == "Documentation" {
			for _, a := range t.Attr {
				if a.Name.Local == "exporter" {
					p.m.Exporter = a.Value
				}
			}
		}
		if name == "Extension" {
			return p.skipExtension(t)
		}
		if err := p.dec.Skip(); err != nil {
			return fmt.Errorf("parsing XMI: %w", err)
		}
		p.depth--
		return nil
	}
	if p.stereo != nil {
		// A direct child of a stereotype application is a tag value.
		if len(p.stack) == p.stereoDepth {
			p.tag = name
			p.tagText.Reset()
			for _, a := range t.Attr {
				if a.Name.Local == "idref" || a.Name.Local == "href" {
					p.stereo.Tags[name] = append(p.stereo.Tags[name], a.Value)
				}
			}
		}
		p.stack = append(p.stack, nil)
		return nil
	}
	var parent *Element
	if n := len(p.stack); n > 0 {
		parent = p.stack[n-1]
	}
	if parent == nil {
		// Top level: a UML root, or a stereotype application.
		if isUMLNamespace(t.Name.Space) || hasXMIType(t.Attr) {
			e := p.m.newElement(t, nil)
			p.m.Roots = append(p.m.Roots, e)
			p.stack = append(p.stack, e)
			return nil
		}
		p.stereo = p.m.newStereotype(t)
		p.stack = append(p.stack, nil)
		p.stereoDepth = len(p.stack)
		return nil
	}
	idref, href := "", ""
	for _, a := range t.Attr {
		switch a.Name.Local {
		case "idref":
			idref = a.Value
		case "href":
			href = a.Value
		}
	}
	// An xmi:type on a reference describes its target and does not make it owned.
	switch {
	case idref != "":
		parent.addRef(name, idref)
		p.stack = append(p.stack, nil)
	case href != "":
		px := p.m.proxy(href)
		if typ := xmiType(t.Attr); typ != "" && px.Type == "" {
			px.Type = typ
		}
		parent.addRef(name, href)
		p.stack = append(p.stack, nil)
	default:
		e := p.m.newElement(t, parent)
		parent.Children = append(parent.Children, e)
		p.stack = append(p.stack, e)
	}
	return nil
}

func (p *docParser) chars(t xml.CharData) {
	if p.stereo != nil {
		if p.tag != "" {
			p.tagText.Write(t)
		}
		return
	}
	if n := len(p.stack); n > 0 && p.stack[n-1] != nil {
		p.stack[n-1].Text += string(t)
	}
}

func (p *docParser) end() {
	if p.stereo != nil {
		switch len(p.stack) {
		case p.stereoDepth:
			p.m.Stereotypes = append(p.m.Stereotypes, p.stereo)
			p.stereo = nil
		case p.stereoDepth + 1:
			if text := strings.TrimSpace(p.tagText.String()); text != "" {
				p.stereo.Tags[p.tag] = append(p.stereo.Tags[p.tag], text)
			}
			p.tag = ""
		}
	}
	if n := len(p.stack); n > 0 {
		p.stack = p.stack[:n-1]
	}
	p.depth--
}

// isUMLNamespace recognizes the UML metamodel namespaces of the OMG and Eclipse
// serializations: a "UML" path segment followed only by a version, so a
// profile below it (…/UML/20161101/StandardProfile) is not one.
func isUMLNamespace(ns string) bool {
	segs := strings.Split(strings.TrimRight(ns, "/"), "/")
	for i := len(segs) - 1; i >= 0; i-- {
		if strings.EqualFold(segs[i], "uml") {
			return true
		}
		if !isVersionSegment(segs[i]) {
			return false
		}
	}
	return false
}

func isVersionSegment(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; !(c >= '0' && c <= '9' || c == '.') {
			return false
		}
	}
	return true
}

func isXMINamespace(ns string) bool {
	return strings.Contains(strings.ToLower(ns), "xmi")
}

func hasXMIType(attrs []xml.Attr) bool { return xmiType(attrs) != "" }

// xmiType returns the local part of the xmi:type attribute, or "".
func xmiType(attrs []xml.Attr) string {
	for _, a := range attrs {
		if a.Name.Local == "type" && isXMINamespace(a.Name.Space) {
			return local(a.Value)
		}
	}
	return ""
}

func (m *Model) newElement(t xml.StartElement, parent *Element) *Element {
	e := &Element{Role: t.Name.Local, Parent: parent, Attrs: map[string]string{}, refs: map[string][]string{}}
	if parent == nil {
		e.Role = ""
	}
	for _, a := range t.Attr {
		switch {
		case a.Name.Local == "id" && isXMINamespace(a.Name.Space):
			e.ID = a.Value
		case a.Name.Local == "type" && isXMINamespace(a.Name.Space):
			e.Type = local(a.Value)
		case a.Name.Local == "name":
			e.Name = a.Value
			e.Attrs["name"] = a.Value
		default:
			e.Attrs[a.Name.Local] = a.Value
		}
	}
	if e.Type == "" && parent == nil {
		e.Type = t.Name.Local
	}
	if e.ID != "" {
		m.byID[e.ID] = e
	}
	return e
}

func (e *Element) addRef(role, id string) {
	e.refs[role] = append(e.refs[role], id)
}

// proxy returns the proxy element for an href, creating it on first sight.
func (m *Model) proxy(href string) *Element {
	if p, ok := m.proxies[href]; ok {
		return p
	}
	p := &Element{ID: href, Href: href, Attrs: map[string]string{}, refs: map[string][]string{}}
	if i := strings.LastIndexByte(href, '#'); i >= 0 {
		frag := href[i+1:]
		if looksLikeName(frag) {
			p.Name = frag
		}
	}
	m.proxies[href] = p
	return p
}

// looksLikeName reports whether an href fragment is a readable name rather
// than a generated id: letters only, as PrimitiveTypes.xmi#Real is.
func looksLikeName(s string) bool {
	if s == "" || s[0] == '_' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}

func (m *Model) newStereotype(t xml.StartElement) *Stereotype {
	s := &Stereotype{Name: t.Name.Local, Namespace: t.Name.Space, Tags: map[string][]string{}}
	for _, a := range t.Attr {
		switch {
		case isXMINamespace(a.Name.Space):
			if a.Name.Local == "id" {
				s.ID = a.Value
			}
		case a.Name.Space == "xmlns" || a.Name.Local == "xmlns":
		case strings.HasPrefix(a.Name.Local, "base_"):
			s.BaseID = a.Value
		default:
			s.Tags[a.Name.Local] = append(s.Tags[a.Name.Local], a.Value)
		}
	}
	return s
}

// link resolves each stereotype application to its base element. An
// application of an element outside the documents read (a proxy) is kept
// unresolved, since nothing in the model is extended by it.
func (m *Model) link() {
	for _, s := range m.Stereotypes {
		if base, ok := m.byID[s.BaseID]; ok {
			s.Base = base
			base.Stereotypes = append(base.Stereotypes, s)
		}
	}
	// An href whose fragment is an id this document defines is that element.
	for href := range m.proxies {
		if i := strings.LastIndexByte(href, '#'); i >= 0 {
			if e, ok := m.byID[href[i+1:]]; ok {
				m.proxies[href] = e
			}
		}
	}
}

// skipExtension reads past an xmi:Extension, recording it and the typed
// elements (diagrams, mostly) it holds so a migration can account for them.
func (p *docParser) skipExtension(t xml.StartElement) error {
	ext := Extension{}
	for _, a := range t.Attr {
		if a.Name.Local == "extender" {
			ext.Extender = a.Value
		}
	}
	for i := len(p.stack) - 1; i >= 0; i-- {
		if p.stack[i] != nil {
			ext.Owner = p.stack[i]
			break
		}
	}
	depth := 1
	for depth > 0 {
		tok, err := p.dec.Token()
		if err != nil {
			return fmt.Errorf("parsing XMI: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
			var el ExtensionElement
			for _, a := range tok.Attr {
				switch {
				case a.Name.Local == "type" && isXMINamespace(a.Name.Space):
					el.Type = a.Value
				case a.Name.Local == "id" && isXMINamespace(a.Name.Space):
					el.ID = a.Value
				case a.Name.Local == "name" && a.Name.Space == "":
					el.Name = a.Value
				}
			}
			if el.Type != "" {
				ext.Elements = append(ext.Elements, el)
			}
		case xml.EndElement:
			depth--
		}
	}
	p.m.Extensions = append(p.m.Extensions, ext)
	p.depth--
	return nil
}
