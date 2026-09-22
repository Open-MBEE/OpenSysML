// Package sysmlv1 interprets the element tree of internal/translate/xmi as the UML
// model with stereotype applications that SysML v1 tools export.
//
// The reader is deliberately tolerant of the serialization's dialect: the UML,
// XMI and profile namespaces differ between the OMG normative XMI and the
// Eclipse UML2 serialization Papyrus writes, so elements are classified by the
// local part of their xmi:type and stereotype applications by the local part
// of their element name. A zip archive holding the model, such as a MagicDraw
// .mdzip project, is opened in place and its model entries read as one document.
//
// The tree carries no UML semantics of its own; internal/translate/migrate
// interprets it as a SysML v1 model.
package sysmlv1

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi"
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
	// QualifiedName is the name a tool records beside an href for the element it
	// points to (MagicDraw's referentPath); "" when the document gives none.
	QualifiedName string
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

// IDs returns the ids a reference-valued tag lists, one per value when the
// tool wrote child elements and split on whitespace when it wrote an IDREFS
// attribute.
func (s *Stereotype) IDs(name string) []string {
	var ids []string
	for _, v := range s.Tags[name] {
		ids = append(ids, strings.Fields(v)...)
	}
	return ids
}

// Model is one document, or one archive's documents read as one.
type Model struct {
	// Roots are the top-level UML elements, in document order.
	Roots []*Element
	// Stereotypes are every stereotype application, in document order.
	Stereotypes []*Stereotype
	// Exporter records the xmi:Documentation exporter, when the document names one.
	Exporter string
	// Extensions are the tool-private xmi:Extension blocks that were skipped.
	Extensions []Extension
	// Diagrams are the diagrams read out of those blocks, in document order.
	Diagrams []Diagram
	byID     map[string]*Element
	proxies  map[string]*Element
}

// Extension records one skipped xmi:Extension: who wrote it and what it held.
type Extension struct {
	// Extender is the tool named by the block's extender attribute.
	Extender string
	// Owner is the element the block sits in; nil at the document root.
	Owner *Element
	// Elements are the xmi:type and name of every typed element inside, the
	// diagrams excepted, which Model.Diagrams holds.
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
	ids := e.RefIDs(role)
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
	for _, id := range e.RefIDs(role) {
		if m.Lookup(id) == nil {
			out = append(out, id)
		}
	}
	return out
}

// RefIDs lists the raw ids a role of e refers to, attribute ids first: xmi:ids
// within the document, or the hrefs of elements other documents hold, kept as
// written even once such an href resolves to a bundled copy of its element.
func (e *Element) RefIDs(role string) []string {
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

// Parse reads an XMI document, or a zip archive (such as a MagicDraw .mdzip)
// holding one or more, into a Model.
func Parse(data []byte) (*Model, error) {
	// The central directory, not a leading local-file header, makes a zip: an
	// empty or stub-prefixed archive is one, a truncated one is still not XMI.
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err == nil {
		return parseArchive(zr)
	}
	if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
		return nil, fmt.Errorf("reading archive: %w", err)
	}
	m := newModel()
	if err := m.parseDocument(data); err != nil {
		return nil, err
	}
	return m.finish()
}

// errNoModel reports an XMI document holding no model element.
var errNoModel = errors.New("the XMI document holds no model: expected a uml:Model or uml:Package under the xmi:XMI root")

// finish links the read documents and checks a model was read at all.
func (m *Model) finish() (*Model, error) {
	if len(m.Roots) == 0 {
		return nil, errNoModel
	}
	m.link()
	return m, nil
}

// maxEntrySize bounds an archive entry's uncompressed size, so a compressed
// archive cannot expand without limit while being read.
const maxEntrySize = 512 << 20

// projectEntry reports whether an archive entry is a MagicDraw project model
// entry, which is read unconditionally.
func projectEntry(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, "uml_model.model") || strings.HasSuffix(lower, "uml_model.shared_model")
}

// documentEntry reports whether an archive entry may hold an XMI document
// when the archive has no project model entries.
func documentEntry(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".xmi") || strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".uml")
}

// parseArchive reads the MagicDraw project model entries of an archive, or,
// in an archive that has none, every XMI document among its .xmi/.xml/.uml
// files; other XML there is metadata and is left alone.
func parseArchive(zr *zip.Reader) (*Model, error) {
	var project, documents []*zip.File
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
		switch {
		case projectEntry(f.Name):
			project = append(project, f)
		case documentEntry(f.Name):
			documents = append(documents, f)
		}
	}
	m := newModel()
	read := 0
	if len(project) > 0 {
		for _, f := range project {
			if err := m.parseEntry(f); err != nil {
				return nil, err
			}
			read++
		}
	} else {
		for _, f := range documents {
			err := m.parseEntry(f)
			if errors.Is(err, errNotXMI) {
				continue
			}
			if err != nil {
				return nil, err
			}
			read++
		}
	}
	if read == 0 {
		sort.Strings(names)
		return nil, fmt.Errorf("archive holds no model document (expected a MagicDraw uml_model.model entry or an .xmi file); entries: %s", strings.Join(names, ", "))
	}
	model, err := m.finish()
	if err != nil {
		return nil, fmt.Errorf("archive: %w", err)
	}
	return model, nil
}

// parseEntry reads one archive entry as an XMI document.
func (m *Model) parseEntry(f *zip.File) error {
	if f.UncompressedSize64 > maxEntrySize {
		return fmt.Errorf("archive entry %s: %d bytes exceeds the %d byte limit", f.Name, f.UncompressedSize64, maxEntrySize)
	}
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("archive entry %s: %w", f.Name, err)
	}
	content, err := io.ReadAll(io.LimitReader(rc, maxEntrySize+1))
	if cerr := rc.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("archive entry %s: %w", f.Name, err)
	}
	if len(content) > maxEntrySize {
		return fmt.Errorf("archive entry %s: exceeds the %d byte limit", f.Name, maxEntrySize)
	}
	if err := m.parseDocument(content); err != nil {
		return fmt.Errorf("archive entry %s: %w", f.Name, err)
	}
	return nil
}

func newModel() *Model {
	return &Model{byID: map[string]*Element{}, proxies: map[string]*Element{}}
}

// local returns the local part of an "prefix:name" value.
func local(s string) string {
	if i := strings.LastIndexByte(s, ':'); i >= 0 {
		return s[i+1:]
	}
	return s
}

// rootOf returns a document's root start element without reading past it, so a
// foreign document is recognized before its content is parsed.
func rootOf(data []byte) (xml.StartElement, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			return xml.StartElement{}, err
		}
		if start, ok := tok.(xml.StartElement); ok {
			return start, nil
		}
	}
}

// errNotXMI reports a document whose root is not xmi:XMI or a UML element.
var errNotXMI = errors.New("not an XMI document: expected an xmi:XMI or uml:Model root element")

// parseDocument reads one document's elements and stereotype applications.
func (m *Model) parseDocument(data []byte) error {
	start, err := rootOf(data)
	if err != nil {
		return fmt.Errorf("parsing XMI: %w", err)
	}
	typed := false
	for _, attr := range start.Attr {
		if xmi.IsXMINamespace(attr.Name.Space) && attr.Name.Local == "type" && local(attr.Value) != "" {
			typed = true
			break
		}
	}
	if !(xmi.IsXMINamespace(start.Name.Space) && start.Name.Local == "XMI") &&
		!xmi.IsUMLNamespace(start.Name.Space) && !typed {
		return errNotXMI
	}
	doc, err := xmi.Parse(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("parsing XMI: %w", err)
	}
	root := doc.Root
	if xmi.IsXMINamespace(root.Space) && root.Tag == "XMI" {
		for _, child := range root.Children {
			m.topLevel(child)
		}
		return nil
	}
	m.topLevel(root)
	return nil
}

func (m *Model) topLevel(raw *xmi.Element) {
	if xmi.IsXMINamespace(raw.Space) {
		m.special(raw, nil, nil)
		return
	}
	if xmi.IsUMLNamespace(raw.Space) || raw.Type != "" {
		e := m.newElement(raw, nil)
		m.Roots = append(m.Roots, e)
		m.children(raw, e, nil)
		return
	}
	m.Stereotypes = append(m.Stereotypes, m.newStereotype(raw))
}

func (m *Model) children(raw *xmi.Element, parent *Element, ref *Element) {
	for _, child := range raw.Children {
		if xmi.IsXMINamespace(child.Space) {
			m.special(child, parent, ref)
			continue
		}
		if ref != nil {
			continue
		}
		if id := child.Attr("idref"); id != "" {
			parent.addRef(child.Tag, id)
			continue
		}
		if href := child.Href(); href != "" {
			proxy := m.proxy(href)
			if typ := local(child.Type); typ != "" && proxy.Type == "" {
				proxy.Type = typ
			}
			parent.addRef(child.Tag, href)
			m.children(child, parent, proxy)
			continue
		}
		e := m.newElement(child, parent)
		parent.Children = append(parent.Children, e)
		m.children(child, e, nil)
	}
}

func (m *Model) special(raw *xmi.Element, owner, ref *Element) {
	switch raw.Tag {
	case "Documentation":
		if exporter := raw.Attr("exporter"); exporter != "" {
			m.Exporter = exporter
		}
	case "Extension":
		ext := Extension{Extender: raw.Attr("extender"), Owner: owner}
		m.extensionContent(raw, &ext, ref)
		m.Extensions = append(m.Extensions, ext)
	}
}

// extensionContent records what an extension block holds, in document order:
// a diagram as a Diagram, any other typed element, a diagram's own included, as skipped.
func (m *Model) extensionContent(raw *xmi.Element, ext *Extension, ref *Element) {
	for _, child := range raw.Children {
		if isDiagram(child) {
			m.diagram(child, ext)
			m.extensionContent(child, ext, ref)
			continue
		}
		if child.Type != "" {
			ext.Elements = append(ext.Elements, ExtensionElement{
				ID: child.ID, Type: child.Type, Name: child.Name(),
			})
		}
		if ref != nil && child.Tag == "referenceExtension" {
			m.describeReference(ref, child)
		}
		m.extensionContent(child, ext, ref)
	}
}

func (m *Model) describeReference(ref *Element, raw *xmi.Element) {
	if path := raw.Attr("referentPath"); path != "" {
		ref.QualifiedName = path
		if ref.Name == "" {
			if i := strings.LastIndex(path, "::"); i >= 0 {
				ref.Name = path[i+2:]
			} else {
				ref.Name = path
			}
		}
	}
	if typ := raw.Attr("referentType"); typ != "" && ref.Type == "" {
		ref.Type = typ
	}
}

func stereotypeText(raw *xmi.Element) string {
	var text strings.Builder
	raw.Walk(func(e *xmi.Element) bool {
		text.WriteString(e.Text)
		return true
	})
	return strings.TrimSpace(text.String())
}

func (m *Model) newStereotype(raw *xmi.Element) *Stereotype {
	s := &Stereotype{
		ID: raw.ID, Name: raw.Tag, Namespace: raw.Space, Tags: map[string][]string{},
	}
	for name, value := range raw.Attrs {
		switch {
		case strings.HasPrefix(name, "base_"):
			s.BaseID = value
		default:
			s.Tags[name] = append(s.Tags[name], value)
		}
	}
	for _, child := range raw.Children {
		if xmi.IsXMINamespace(child.Space) {
			m.special(child, nil, nil)
			continue
		}
		if value := child.Attr("idref"); value != "" {
			s.Tags[child.Tag] = append(s.Tags[child.Tag], value)
		} else if value := child.Href(); value != "" {
			s.Tags[child.Tag] = append(s.Tags[child.Tag], value)
		} else if value := stereotypeText(child); value != "" {
			s.Tags[child.Tag] = append(s.Tags[child.Tag], value)
		}
	}
	return s
}

func (m *Model) newElement(raw *xmi.Element, parent *Element) *Element {
	e := &Element{
		ID: raw.ID, Type: local(raw.Type), Role: raw.Tag, Name: raw.Name(),
		Text: raw.Text, Parent: parent, Attrs: map[string]string{}, refs: map[string][]string{},
	}
	if parent == nil {
		e.Role = ""
	}
	for name, value := range raw.Attrs {
		e.Attrs[name] = value
	}
	for name, value := range raw.XMIAttrs {
		e.Attrs[name] = value
	}
	if e.Type == "" && parent == nil {
		e.Type = raw.Tag
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
		p.Name = fragmentName(href[i+1:])
	}
	m.proxies[href] = p
	return p
}

// fragmentName reads the element name an href fragment spells, or "" when the
// fragment is a generated id: PrimitiveTypes.xmi#Real names Real, and so does
// SysML.xmi#SysML_dataType.Real, whose dotted path ends in the name.
func fragmentName(frag string) string {
	if looksLikeName(frag) {
		return frag
	}
	if i := strings.LastIndexByte(frag, '.'); i >= 0 && !strings.HasPrefix(frag, "_") {
		if last := frag[i+1:]; looksLikeName(last) {
			return last
		}
	}
	return ""
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
	// An href whose fragment is an id this document, or another entry of the
	// same archive, defines is that element.
	for href := range m.proxies {
		if i := strings.LastIndexByte(href, '#'); i >= 0 {
			if e, ok := m.byID[href[i+1:]]; ok {
				m.proxies[href] = e
			}
		}
	}
	m.linkDiagrams()
}
