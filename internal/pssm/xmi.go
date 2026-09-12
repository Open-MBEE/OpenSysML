// Package pssm reads the OMG PSSM state-machine test suite (ptc/18-11-06,
// PSSM_TestSuite.xmi), classifies each of its tests by the UML constructs the
// test's state machine uses, and translates the expressible ones into SysML v2
// textual notation for cmd/pssm-referee. The suite is downloaded by
// scripts/download-pssm-suite.sh and never vendored; docs/project/pssm-referee.md
// records what a result from it means.
package pssm

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// xmiNamespace is the XMI 2.5 namespace the suite's xmi:type and xmi:id use.
const xmiNamespace = "http://www.omg.org/spec/XMI/20131001"

// Element is one XML element of an XMI document: its local tag, its xmi:type
// and xmi:id, every other attribute by local name, and its children in
// document order. A parsed document is never modified after Parse returns.
type Element struct {
	Tag      string
	Type     string
	ID       string
	Attrs    map[string]string
	Text     string
	Children []*Element
	Parent   *Element
	Line     int
}

// Attr returns the attribute named by its local name, or "" when absent.
func (e *Element) Attr(name string) string {
	if e == nil {
		return ""
	}
	return e.Attrs[name]
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
			e := &Element{Tag: t.Name.Local, Attrs: make(map[string]string, len(t.Attr))}
			e.Line, _ = dec.InputPos()
			for _, a := range t.Attr {
				switch {
				case a.Name.Space == xmiNamespace && a.Name.Local == "type":
					e.Type = a.Value
				case a.Name.Space == xmiNamespace && a.Name.Local == "id":
					e.ID = a.Value
				case a.Name.Space == "xmlns" || a.Name.Local == "xmlns":
				default:
					e.Attrs[a.Name.Local] = a.Value
				}
			}
			if e.ID != "" {
				if prior, dup := doc.byID[e.ID]; dup {
					return nil, fmt.Errorf("xmi: id %s declared twice (%s and %s)", e.ID, prior.Describe(), e.Describe())
				}
				doc.byID[e.ID] = e
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				e.Parent = parent
				parent.Children = append(parent.Children, e)
			} else if doc.Root == nil {
				doc.Root = e
			} else {
				return nil, fmt.Errorf("xmi: second root element %s", e.Describe())
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
