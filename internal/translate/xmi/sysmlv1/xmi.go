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
	"unicode"

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
	// Name is the stereotype's local name, such as "Block" or "Requirement": the
	// model name of a resolved Definition, else the XML name the tool wrote.
	Name string
	// Namespace is the XML namespace the profile was serialized under.
	Namespace string
	// BaseID is the value of the base_* attribute; Base resolves it.
	BaseID string
	Base   *Element
	// Definition is the uml:Stereotype the application instantiates, when a
	// document read defines it and the application can be traced to it: through
	// the tool's stereotypesHREFS table, or by name within the profile the
	// application's XML namespace denotes. Nil otherwise.
	Definition *Element
	// Generals are every stereotype Definition specializes, transitively:
	// Model.Ancestors of the definition, so proxies stand for those defined
	// outside the documents read. Empty without a Definition.
	Generals []*Element
	// Tags holds the tagged values: attributes other than xmi:* and base_*, and
	// child elements as their text or idref, keyed by tag name. A multi-valued
	// tag lists each value.
	Tags map[string][]string
	// attrValues counts, per tag, the leading Tags values that came from an
	// attribute rather than a child element.
	attrValues map[string]int
}

// Tag returns the first value of a tag, or "".
func (s *Stereotype) Tag(name string) string {
	if v := s.Tags[name]; len(v) > 0 {
		return v[0]
	}
	return ""
}

// IDs returns the ids a reference-valued tag lists: an IDREFS attribute split
// on whitespace, a child element's idref or href kept whole, since an href
// into another archive entry may contain spaces.
func (s *Stereotype) IDs(name string) []string {
	var ids []string
	for i, v := range s.Tags[name] {
		if i < s.attrValues[name] {
			ids = append(ids, strings.Fields(v)...)
		} else {
			ids = append(ids, v)
		}
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
	// Tables are the table, matrix and relation map definitions read out of
	// the tool profile applications, in document order.
	Tables []*Table
	// Documents are the MDK DocGen documents, in document order.
	Documents       []*DocGenDocument
	byID            map[string]*Element
	proxies         map[string]*Element
	fragments       map[string]*Element
	stereotypeNames map[string]stereotypeName
	// moduleStereotypes are the stereotypes the archive's module snapshots
	// declare, by id, for resolving ids the stereotype table does not name.
	moduleStereotypes map[string]moduleStereotype
	clients           map[*Element][]*Element
	// stereotypeHrefs are the definitions a tool's stereotypesHREFS table
	// names for applied stereotypes, by namespace and name.
	stereotypeHrefs map[stereotypeKey]string
	ancestors       map[*Element][]*Element
}

// stereotypeKey identifies an applied stereotype by the namespace it is
// serialized under and its local name.
type stereotypeKey struct{ namespace, name string }

// Extension records one skipped xmi:Extension: who wrote it and what it held,
// less the ElementValue operands read into the model (see adoptValues).
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

// Generals returns the classifiers e directly specializes through its
// generalizations: elements of the documents read, or proxies for others.
func (m *Model) Generals(e *Element) []*Element {
	var out []*Element
	for _, g := range e.Owned("generalization") {
		out = append(out, m.Refs(g, "general")...)
	}
	return out
}

// UnresolvedGenerals lists the ids of generals of e no document read defines.
func (m *Model) UnresolvedGenerals(e *Element) []string {
	var out []string
	for _, g := range e.Owned("generalization") {
		out = append(out, m.Unresolved(g, "general")...)
	}
	return out
}

// Ancestors returns every classifier e specializes, transitively, nearest
// first and each once, in-document elements and proxies alike. e itself is
// among them exactly when its generalizations cycle back to it.
func (m *Model) Ancestors(e *Element) []*Element {
	if a, ok := m.ancestors[e]; ok {
		return a
	}
	seen := map[*Element]bool{}
	var out []*Element
	queue := m.Generals(e)
	for len(queue) > 0 {
		g := queue[0]
		queue = queue[1:]
		if seen[g] {
			continue
		}
		seen[g] = true
		out = append(out, g)
		queue = append(queue, m.Generals(g)...)
	}
	m.ancestors[e] = out
	return out
}

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
	var project, modules, documents []*zip.File
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
		switch {
		case projectEntry(f.Name):
			project = append(project, f)
		case moduleEntry(f.Name):
			modules = append(modules, f)
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
		for _, f := range modules {
			content, err := readEntry(f)
			if err != nil {
				return nil, err
			}
			m.indexModule(content)
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

// readEntry reads one archive entry within the size bound.
func readEntry(f *zip.File) ([]byte, error) {
	if f.UncompressedSize64 > maxEntrySize {
		return nil, fmt.Errorf("archive entry %s: %d bytes exceeds the %d byte limit", f.Name, f.UncompressedSize64, maxEntrySize)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("archive entry %s: %w", f.Name, err)
	}
	content, err := io.ReadAll(io.LimitReader(rc, maxEntrySize+1))
	if cerr := rc.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, fmt.Errorf("archive entry %s: %w", f.Name, err)
	}
	if len(content) > maxEntrySize {
		return nil, fmt.Errorf("archive entry %s: exceeds the %d byte limit", f.Name, maxEntrySize)
	}
	return content, nil
}

// parseEntry reads one archive entry as an XMI document.
func (m *Model) parseEntry(f *zip.File) error {
	content, err := readEntry(f)
	if err != nil {
		return err
	}
	if err := m.parseDocument(content); err != nil {
		return fmt.Errorf("archive entry %s: %w", f.Name, err)
	}
	return nil
}

func newModel() *Model {
	return &Model{
		byID: map[string]*Element{}, proxies: map[string]*Element{}, fragments: map[string]*Element{},
		stereotypeHrefs: map[stereotypeKey]string{}, ancestors: map[*Element][]*Element{},
	}
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
		adopted := m.adoptValues(raw, owner, ref)
		m.extensionContent(raw, &ext, ref, adopted)
		m.Extensions = append(m.Extensions, ext)
	}
}

// extensionContent records what an extension block holds, in document order:
// a diagram as a Diagram, any other typed element not adopted into the model,
// a diagram's own included, as skipped.
func (m *Model) extensionContent(raw *xmi.Element, ext *Extension, ref *Element, adopted map[*xmi.Element]bool) {
	for _, child := range raw.Children {
		if adopted[child] {
			continue
		}
		if child.Tag == "stereotypesHREFS" {
			m.indexStereotypes(child)
			continue
		}
		if isDiagram(child) {
			m.diagram(child, ext)
			m.extensionContent(child, ext, ref, adopted)
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
		m.extensionContent(child, ext, ref, adopted)
	}
}

// adoptValues reads the ElementValue operands a tool keeps in an extension block, under any
// wrappers but not inside a diagram or a reference, as the owner's; it returns those adopted.
func (m *Model) adoptValues(raw *xmi.Element, owner, ref *Element) map[*xmi.Element]bool {
	adopted := map[*xmi.Element]bool{}
	if owner == nil || ref != nil {
		return adopted
	}
	var walk func(*xmi.Element)
	walk = func(block *xmi.Element) {
		for _, child := range block.Children {
			switch {
			case isDiagram(child) || child.Tag == "referenceExtension":
			case local(child.Type) == "ElementValue":
				e := m.newElement(child, owner)
				owner.Children = append(owner.Children, e)
				m.children(child, e, nil)
				adopted[child] = true
				for _, d := range child.Descendants() {
					adopted[d] = true
				}
			default:
				walk(child)
			}
		}
	}
	walk(raw)
	return adopted
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
		ID: raw.ID, Name: raw.Tag, Namespace: raw.Space, Tags: map[string][]string{}, attrValues: map[string]int{},
	}
	for name, value := range raw.Attrs {
		switch {
		case strings.HasPrefix(name, "base_"):
			s.BaseID = value
		default:
			s.Tags[name] = append(s.Tags[name], value)
			s.attrValues[name]++
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
		if frag := href[i+1:]; m.fragments[frag] == nil {
			m.fragments[frag] = p
		}
	}
	m.proxies[href] = p
	return p
}

// fragmentName reads the element name an href fragment spells, or "" when the
// fragment is a generated id: PrimitiveTypes.xmi#Real names Real, and so does
// SysML.xmi#SysML_dataType.Real, whose dotted path ends in the name, and
// Papyrus's SysML.profile.uml#SysML.package_packagedElement_Blocks.stereotype_packagedElement_Block,
// whose last path step ends in the name after its metaclass and role.
func fragmentName(frag string) string {
	if looksLikeName(frag) {
		return frag
	}
	if strings.HasPrefix(frag, "_") {
		return ""
	}
	last := frag
	if i := strings.LastIndexByte(frag, '.'); i >= 0 {
		last = frag[i+1:]
		if looksLikeName(last) {
			return last
		}
	}
	if i := strings.LastIndexByte(last, '_'); i >= 0 && strings.Contains(frag, ".") {
		if name := last[i+1:]; looksLikeName(name) {
			return name
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

// link resolves each stereotype application to its base element and to the
// stereotype defining it. An application of an element outside the documents
// read (a proxy) is kept unresolved, since nothing in the model is extended by it.
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
	m.bindModuleProfiles()
	m.linkDiagrams()
	defs := m.stereotypeDefinitions()
	for _, s := range m.Stereotypes {
		if s.Definition = m.definitionOf(s, defs); s.Definition != nil {
			s.Generals = m.Ancestors(s.Definition)
			if s.Definition.Name != "" {
				s.Name = s.Definition.Name
			}
		}
	}
	m.readTables()
	m.readDocuments()
}

// stereotypeDefinitions lists every uml:Stereotype the documents read define.
func (m *Model) stereotypeDefinitions() []*Element {
	var defs []*Element
	var walk func(*Element)
	walk = func(e *Element) {
		if e.Type == "Stereotype" {
			defs = append(defs, e)
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	for _, r := range m.Roots {
		walk(r)
	}
	return defs
}

// definitionOf finds the stereotype an application instantiates: the one the
// tool's stereotypesHREFS table names when it has one, else the single
// definition of the same name whose owning profile the application's XML
// namespace denotes. Nothing is guessed when neither settles it.
func (m *Model) definitionOf(s *Stereotype, defs []*Element) *Element {
	if href, ok := m.stereotypeHrefs[stereotypeKey{s.Namespace, s.Name}]; ok {
		if i := strings.LastIndexByte(href, '#'); i >= 0 {
			if d := m.byID[href[i+1:]]; d != nil && d.Type == "Stereotype" {
				return d
			}
		}
		return nil
	}
	var found *Element
	for _, d := range defs {
		if !SameName(d.Name, s.Name) || !denotes(s.Namespace, d) {
			continue
		}
		if found != nil {
			return nil
		}
		found = d
	}
	return found
}

// denotes reports whether the XML namespace an application is serialized
// under names a package owning definition d: by the package's URI, by the
// nsURI of an Ecore annotation on it, or by the namespace's document name
// spelling the package's name as a tool derives one from the other.
func denotes(ns string, d *Element) bool {
	doc := namespaceDocument(ns)
	for p := d.Parent; p != nil; p = p.Parent {
		switch p.Type {
		case "Profile", "Package", "Model":
		default:
			continue
		}
		if p.Attrs["URI"] == ns || annotatedNamespace(p, ns) || (doc != "" && SameName(p.Name, doc)) {
			return true
		}
	}
	return false
}

// annotatedNamespace reports whether an Ecore annotation under package p
// (Papyrus writes one per profile definition) declares the nsURI ns.
func annotatedNamespace(p *Element, ns string) bool {
	var found bool
	var walk func(*Element)
	walk = func(e *Element) {
		if found || e.Attrs["nsURI"] == ns {
			found = true
			return
		}
		for _, c := range e.Children {
			walk(c)
		}
	}
	for _, a := range p.Owned("eAnnotations") {
		walk(a)
	}
	return found
}

// namespaceDocument is the last path segment of a namespace URI without its
// model extension: MagicDraw derives it from the profile's name.
func namespaceDocument(ns string) string {
	doc := ns
	if i := strings.IndexAny(doc, "#?"); i >= 0 {
		doc = doc[:i]
	}
	doc = strings.TrimRight(doc, "/")
	if i := strings.LastIndexByte(doc, '/'); i >= 0 {
		doc = doc[i+1:]
	}
	for _, ext := range []string{".xmi", ".uml", ".xml"} {
		if len(doc) > len(ext) && strings.EqualFold(doc[len(doc)-len(ext):], ext) {
			return doc[:len(doc)-len(ext)]
		}
	}
	return doc
}

// SameName compares names up to the characters a tool replaces to make an
// XML name of a model name: case and everything but letters and digits.
func SameName(a, b string) bool {
	return foldName(a) == foldName(b)
}

func foldName(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}
