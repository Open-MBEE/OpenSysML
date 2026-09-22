package sysmlv1

import (
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi"
)

// Diagram is one diagram of the model as a tool serialized it inside an
// xmi:Extension: what it is called, what kind of diagram it is, which element
// owns it and which elements it shows. Layout is not read.
type Diagram struct {
	// ID is the diagram's xmi:id; "" when the tool wrote none.
	ID string
	// Name is the diagram's name attribute.
	Name string
	// Kind is the tool's diagram type, such as "SysML Block Definition Diagram"
	// or "Requirement Table"; "" when the tool serialized none.
	Kind string
	// UMLKind is the UML diagram type Kind derives from, such as "Class Diagram"
	// or "Composite Structure Diagram"; "" when the tool serialized none.
	UMLKind string
	// OwnerID is the ownerOfDiagram attribute as written; "" when absent.
	OwnerID string
	// Owner is the element OwnerID resolves to; nil when it is absent or dangling.
	Owner *Element
	// Holder is the element whose xmi:Extension holds the diagram; nil at the
	// document root or inside a stereotype application.
	Holder *Element
	// Shown lists the elements the diagram shows in serialized order, each id
	// once, however many times the tool listed it.
	Shown []ElementRef
	// Extender is the tool named by the extension the diagram sits in.
	Extender string
}

// ElementRef is one element a diagram shows.
type ElementRef struct {
	// ID is the xmi:id the tool wrote, or the href of an element of another
	// document, with a local href's fragment marker removed.
	ID string
	// Element is the element ID resolves to: one the read documents define, or
	// the proxy standing for one of another document; nil when it is dangling.
	Element *Element
}

// Represented reports whether the tool serialized what the diagram is and shows;
// a diagram with neither a kind nor contents is a name and nothing more.
func (d *Diagram) Represented() bool {
	return d.Kind != "" || d.UMLKind != "" || len(d.Shown) > 0
}

// isDiagram reports whether a raw extension element is a diagram: a UML
// Diagram, whatever XML element name the tool owns it under.
func isDiagram(raw *xmi.Element) bool {
	return local(raw.Type) == "Diagram"
}

// diagram reads one serialized diagram: its kind from the representation
// object's type and umlType attributes, its contents from the usedElements ids
// and usedObjects hrefs beneath that object; without a representation object
// nothing is shown, whatever other tool content lists. Owner and shown elements
// are resolved by linkDiagrams once every document is read, since a diagram may
// precede what it names.
func (m *Model) diagram(raw *xmi.Element, ext *Extension) {
	d := Diagram{
		ID: raw.ID, Name: raw.Name(), OwnerID: raw.Attr("ownerOfDiagram"),
		Holder: ext.Owner, Extender: ext.Extender,
	}
	rep := representation(raw)
	if rep == nil {
		m.Diagrams = append(m.Diagrams, d)
		return
	}
	d.Kind, d.UMLKind = rep.Attrs["type"], rep.Attrs["umlType"]
	seen := map[string]bool{}
	walkDiagram(rep, func(n *xmi.Element) {
		if n.Tag != "usedElements" && n.Tag != "usedObjects" {
			return
		}
		id := strings.TrimSpace(n.Text)
		if href := n.Href(); href != "" {
			id = strings.TrimPrefix(href, "#")
		}
		if id != "" && !seen[id] {
			seen[id] = true
			d.Shown = append(d.Shown, ElementRef{ID: id})
		}
	})
	m.Diagrams = append(m.Diagrams, d)
}

// representation finds the diagram's representation object by how it is held or
// tagged, else by a umlType; a plain type names a UML type, never a diagram kind.
func representation(raw *xmi.Element) *xmi.Element {
	var structural, typed *xmi.Element
	walkDiagram(raw, func(n *xmi.Element) {
		if n == raw {
			return
		}
		if structural == nil && isRepresentationObject(n) {
			structural = n
		}
		if typed == nil && n.Attrs["umlType"] != "" {
			typed = n
		}
	})
	if structural != nil {
		return structural
	}
	return typed
}

// isRepresentationObject reports whether n is serialized as a diagram's
// representation object: by its tag, or by the diagramRepresentation holding it.
func isRepresentationObject(n *xmi.Element) bool {
	return n.Tag == "DiagramRepresentationObject" ||
		n.Parent != nil && n.Parent.Tag == "diagramRepresentation"
}

// walkDiagram visits root and its descendants in document order, staying out
// of any diagram nested beneath it, which is read as a diagram of its own.
func walkDiagram(root *xmi.Element, fn func(*xmi.Element)) {
	fn(root)
	for _, c := range root.Children {
		if !isDiagram(c) {
			walkDiagram(c, fn)
		}
	}
}

// linkDiagrams resolves each diagram's owner and shown elements to the elements
// the read documents define or proxy, leaving dangling ids unresolved.
func (m *Model) linkDiagrams() {
	for i := range m.Diagrams {
		d := &m.Diagrams[i]
		d.Owner = m.byID[d.OwnerID]
		for j := range d.Shown {
			d.Shown[j].Element = m.shown(d.Shown[j].ID)
		}
	}
}

// Diagram finds the diagram with xmi:id id, or the one an href's fragment
// names; nil when no diagram has it.
func (m *Model) Diagram(id string) *Diagram {
	if i := strings.LastIndexByte(id, '#'); i >= 0 {
		id = id[i+1:]
	}
	if id == "" {
		return nil
	}
	for i := range m.Diagrams {
		if m.Diagrams[i].ID == id {
			return &m.Diagrams[i]
		}
	}
	return nil
}

// shown resolves the id of a shown element: an xmi:id of the read documents,
// an href into one of them, an href another document's proxy stands for, or
// the bare fragment of such an href, as a tool writes a module element's id
// once it has referenced the element by href.
func (m *Model) shown(id string) *Element {
	if e := m.byID[id]; e != nil {
		return e
	}
	if i := strings.LastIndexByte(id, '#'); i >= 0 {
		if e := m.byID[id[i+1:]]; e != nil {
			return e
		}
	}
	if p := m.proxies[id]; p != nil {
		return p
	}
	return m.fragments[id]
}
