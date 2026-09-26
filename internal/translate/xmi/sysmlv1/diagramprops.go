package sysmlv1

import (
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi"
)

// DiagramProperty is one property a tool saved with a diagram: MagicDraw's
// diagramProperties attribute holds them as an XML document of typed
// properties, each keyed by a property id.
type DiagramProperty struct {
	// ID is the propertyID, such as OPTION_FILTER_SEARCHING_TEXT.
	ID string
	// Class is the property's elementClass: BooleanProperty, StringProperty,
	// ChoiceProperty, NumberProperty...
	Class string
	// Value is the value as written: a value element's xmi:value attribute,
	// else its text; "" when the property holds none.
	Value string
	// Choices are a ChoiceProperty's choices, in serialized order.
	Choices []string
}

// Property finds the diagram property with the given id; false when the
// tool saved none by that id.
func (d *Diagram) Property(id string) (DiagramProperty, bool) {
	for _, p := range d.Properties {
		if p.ID == id {
			return p, true
		}
	}
	return DiagramProperty{}, false
}

// Flag reports whether the diagram property id is a boolean saved as true.
func (d *Diagram) Flag(id string) bool {
	p, ok := d.Property(id)
	return ok && p.Value == "true"
}

// decodeDiagramProperties reads MagicDraw's diagramProperties attribute: the
// bytes of an XML document, each written as a hexadecimal number separated by
// spaces. nil when the attribute is absent or not of that form.
func decodeDiagramProperties(encoded string) []DiagramProperty {
	fields := strings.Fields(encoded)
	if len(fields) == 0 {
		return nil
	}
	raw := make([]byte, 0, len(fields))
	for _, f := range fields {
		b, err := strconv.ParseUint(f, 16, 8)
		if err != nil {
			return nil
		}
		raw = append(raw, byte(b))
	}
	doc, err := xmi.Parse(strings.NewReader(string(raw)))
	if err != nil {
		return nil
	}
	var out []DiagramProperty
	doc.Root.Walk(func(n *xmi.Element) bool {
		if n.Tag != "mdElement" {
			return true
		}
		id := n.First("propertyID")
		if id == nil {
			return true
		}
		p := DiagramProperty{ID: strings.TrimSpace(id.Text), Class: n.Attr("elementClass")}
		if v := n.First("value"); v != nil {
			p.Value = v.Attr("value")
			if p.Value == "" {
				p.Value = strings.TrimSpace(v.Text)
			}
		}
		for _, c := range n.Tagged("choice") {
			p.Choices = append(p.Choices, c.Attr("value"))
		}
		out = append(out, p)
		return true
	})
	return out
}
