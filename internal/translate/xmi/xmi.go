// Package xmi parses XMI documents into an immutable element tree indexed by
// xmi:id. The fUML and PSSM test-suite readers in tools/referee and sysmlv1
// interpret that tree for their respective models.
package xmi

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html/charset"
)

// isXMI reports whether the attribute is in an XMI namespace of any version.
func isXMI(a xml.Attr) bool {
	return IsXMINamespace(a.Name.Space)
}

// IsXMINamespace reports whether ns is an XMI namespace.
func IsXMINamespace(ns string) bool {
	return isNamedNamespace(ns, "XMI")
}

// IsUMLNamespace reports whether ns is a UML metamodel namespace.
func IsUMLNamespace(ns string) bool {
	return isNamedNamespace(ns, "UML")
}

func isNamedNamespace(ns, name string) bool {
	segments := strings.Split(ns, "/")
	if len(segments) == 0 {
		return false
	}
	last := segments[len(segments)-1]
	if strings.EqualFold(last, name) {
		return true
	}
	return len(segments) > 1 && isVersionSegment(last) &&
		strings.EqualFold(segments[len(segments)-2], name)
}

func isVersionSegment(s string) bool {
	if s == "" {
		return false
	}
	for _, part := range strings.Split(s, ".") {
		if part == "" || strings.Trim(part, "0123456789") != "" {
			return false
		}
	}
	return true
}

// Element is one XML element of an XMI document: its local tag, its xmi:type
// and xmi:id, its non-XMI and XMI attributes by local name, the namespaces it
// declares, and its children in document order. A parsed document is never
// modified after Parse returns.
type Element struct {
	Tag      string
	Space    string
	Type     string
	ID       string
	Attrs    map[string]string
	XMIAttrs map[string]string
	// Namespaces are the xmlns declarations on this element, prefix to URI;
	// the default namespace is under "".
	Namespaces map[string]string
	Text       string
	Children   []*Element
	Parent     *Element
	Line       int
}

// Namespace resolves a prefix to the URI declared for it on this element or
// the nearest ancestor, or "" when none declares it.
func (e *Element) Namespace(prefix string) string {
	for cur := e; cur != nil; cur = cur.Parent {
		if uri, ok := cur.Namespaces[prefix]; ok {
			return uri
		}
	}
	return ""
}

// Attr returns the attribute named by its local name, or "" when absent.
func (e *Element) Attr(name string) string {
	if e == nil {
		return ""
	}
	if value, ok := e.Attrs[name]; ok {
		return value
	}
	return e.XMIAttrs[name]
}

// Name is the element's name attribute.
func (e *Element) Name() string { return e.Attr("name") }

// Href is the cross-document reference the element carries, or "".
func (e *Element) Href() string { return e.Attr("href") }

// Ref returns the xmi:id a reference names, whether written as an attribute
// (`guard="_x"`) or as a child element (`<guard xmi:idref="_x"/>`), or "".
func (e *Element) Ref(name string) string {
	if v := e.Attr(name); v != "" {
		return v
	}
	if c := e.First(name); c != nil {
		return c.Attr("idref")
	}
	return ""
}

// Refs returns every xmi:id a multi-valued reference names: the attribute's
// space-separated ids and each child element's idref.
func (e *Element) Refs(name string) []string {
	var out []string
	if v := e.Attr(name); v != "" {
		out = append(out, strings.Fields(v)...)
	}
	for _, c := range e.Tagged(name) {
		if id := c.Attr("idref"); id != "" {
			out = append(out, id)
		}
	}
	return out
}

// Tagged returns the children with the given tag, in document order.
func (e *Element) Tagged(tag string) []*Element {
	var out []*Element
	for _, c := range e.Children {
		if c.Tag == tag {
			out = append(out, c)
		}
	}
	return out
}

// First returns the first child with the given tag, or nil.
func (e *Element) First(tag string) *Element {
	for _, c := range e.Children {
		if c.Tag == tag {
			return c
		}
	}
	return nil
}

// Walk visits the element and every descendant in document order until fn
// returns false.
func (e *Element) Walk(fn func(*Element) bool) {
	var walk func(*Element) bool
	walk = func(n *Element) bool {
		if !fn(n) {
			return false
		}
		for _, c := range n.Children {
			if !walk(c) {
				return false
			}
		}
		return true
	}
	walk(e)
}

// Descendants returns every descendant (not the element itself) in document
// order.
func (e *Element) Descendants() []*Element {
	var out []*Element
	e.Walk(func(n *Element) bool {
		if n != e {
			out = append(out, n)
		}
		return true
	})
	return out
}

// Describe names an element for a diagnostic: its type, name and id.
func (e *Element) Describe() string {
	if e == nil {
		return "<nil>"
	}
	var b strings.Builder
	if e.Type != "" {
		b.WriteString(e.Type)
	} else {
		b.WriteString(e.Tag)
	}
	if n := e.Name(); n != "" {
		fmt.Fprintf(&b, " %q", n)
	}
	if e.ID != "" {
		fmt.Fprintf(&b, " (%s)", e.ID)
	}
	return b.String()
}

// Document is a parsed XMI file: its root and an index of every xmi:id.
type Document struct {
	Root *Element
	byID map[string]*Element
}

// ByID resolves an xmi:id within the document, nil when it names nothing.
func (d *Document) ByID(id string) *Element {
	if d == nil || id == "" {
		return nil
	}
	return d.byID[id]
}

// Parse reads an XMI document. It returns an error for malformed XML and for an
// xmi:id declared twice, and never panics on unexpected content.
func Parse(r io.Reader) (*Document, error) {
	dec := xml.NewDecoder(r)
	dec.CharsetReader = charset.NewReaderLabel
	doc := &Document{byID: make(map[string]*Element)}
	var stack []*Element
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("xmi: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			e := newElement(t)
			e.Line, _ = dec.InputPos()
			if err := doc.place(e, stack); err != nil {
				return nil, err
			}
			stack = append(stack, e)
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(t)
			}
		case xml.EndElement:
			if len(stack) == 0 {
				return nil, fmt.Errorf("xmi: unbalanced end element %s", t.Name.Local)
			}
			stack = stack[:len(stack)-1]
		}
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("xmi: document ends inside %s", stack[len(stack)-1].Describe())
	}
	if doc.Root == nil {
		return nil, fmt.Errorf("xmi: document has no root element")
	}
	return doc, nil
}

// newElement reads a start tag: its xmi:type and xmi:id, its namespace
// declarations, then the remaining attributes by local name.
func newElement(t xml.StartElement) *Element {
	e := &Element{
		Tag:      t.Name.Local,
		Space:    t.Name.Space,
		Attrs:    make(map[string]string, len(t.Attr)),
		XMIAttrs: make(map[string]string),
	}
	for _, a := range t.Attr {
		switch {
		case isXMI(a) && a.Name.Local == "type":
			e.Type = a.Value
		case isXMI(a) && a.Name.Local == "id":
			e.ID = a.Value
		case isXMI(a):
			e.XMIAttrs[a.Name.Local] = a.Value
		case a.Name.Space == "xmlns":
			e.declare(a.Name.Local, a.Value)
		case a.Name.Local == "xmlns":
			e.declare("", a.Value)
		default:
			e.Attrs[a.Name.Local] = a.Value
		}
	}
	return e
}

func (e *Element) declare(prefix, uri string) {
	if e.Namespaces == nil {
		e.Namespaces = map[string]string{}
	}
	e.Namespaces[prefix] = uri
}

// place indexes e by id and files it under the open element, or as the root.
func (d *Document) place(e *Element, stack []*Element) error {
	if e.ID != "" {
		if prior, dup := d.byID[e.ID]; dup {
			return fmt.Errorf("xmi: id %s declared twice (%s and %s)", e.ID, prior.Describe(), e.Describe())
		}
		d.byID[e.ID] = e
	}
	switch {
	case len(stack) > 0:
		parent := stack[len(stack)-1]
		e.Parent = parent
		parent.Children = append(parent.Children, e)
	case d.Root == nil:
		d.Root = e
	default:
		return fmt.Errorf("xmi: second root element %s", e.Describe())
	}
	return nil
}
