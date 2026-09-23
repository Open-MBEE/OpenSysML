// Package mtip reads the HUDS XML an Open-MBEE MTIP export writes: the diagram
// records carrying each shown element's bounds and each connector's route, so
// a SysML v1 migration can lay out the views it writes. Model content in the
// file is ignored entirely; the export is a presentation augment, never a
// second model input.
package mtip

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Export is the diagram layer of one MTIP export.
type Export struct {
	MTIPVersion, CameoVersion, ExportTime string
	Diagrams                              []Diagram // in file order
	Records                               int       // every <data> seen, for the mismatch message
}

// Diagram is one exported diagram: the elements it shows placed, and its
// connectors routed.
type Diagram struct {
	ID, Type, Name string         // Name from attributes/attribute[key="name"]/attribute[key="value"], "" if absent
	Placements     []Placement    // in key order
	Connectors     []Connector    // in key order
	Unsupported    map[string]int // unknown relationship_metadata child tags, by tag
	Malformed      []string       // per-record problems; the record part they name is skipped
}

// Placement is one shown element's box in pixels, y down from the top left.
type Placement struct {
	ID, Type            string
	X, Y, Width, Height float64
}

// Connector is one drawn edge's route, source to target, flattened as
// (x0, y0, x1, y1, …).
type Connector struct {
	ID, Type string
	Points   []float64
}

// node is one decoded HUDS element: its tag, text, key and children. HUDS
// repeats the container's tag for each child, so children are found by name
// and ordered by their key.
type node struct {
	tag      string
	key      string
	text     string
	children []*node
}

func (n *node) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	n.tag = start.Name.Local
	for _, a := range start.Attr {
		if a.Name.Local == "key" {
			n.key = a.Value
		}
	}
	for {
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t := t.(type) {
		case xml.StartElement:
			c := &node{}
			if err := d.DecodeElement(c, &t); err != nil {
				return err
			}
			n.children = append(n.children, c)
		case xml.CharData:
			n.text += string(t)
		case xml.EndElement:
			return nil
		}
	}
}

// child is the first child tagged tag; nil when absent.
func (n *node) child(tag string) *node {
	for _, c := range n.children {
		if c.tag == tag {
			return c
		}
	}
	return nil
}

// textOf is the trimmed text of the child tagged tag; "" when absent.
func (n *node) textOf(tag string) string {
	if c := n.child(tag); c != nil {
		return strings.TrimSpace(c.text)
	}
	return ""
}

// keyed returns the children in document order, stable-sorted by numeric key.
// A non-numeric key leaves the document order as the file wrote it.
func keyed(children []*node) []*node {
	ordered := append([]*node(nil), children...)
	sort.SliceStable(ordered, func(i, j int) bool {
		a, errA := strconv.Atoi(ordered[i].key)
		b, errB := strconv.Atoi(ordered[j].key)
		return errA == nil && errB == nil && a < b
	})
	return ordered
}

// Parse reads one HUDS export: the packet's metadata and every <data> record,
// keeping only the diagram records — those carrying an element placement list
// or a diagramConnector list. A document that is not well-formed XML fails
// with the position; one whose root is not <packet> is not a HUDS export and
// is refused, as is one with no root element at all.
func Parse(data []byte) (*Export, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	ex := &Export{}
	var depth int
	var root bool
	for {
		t, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("the MTIP export is not well-formed XML: %w", err)
		}
		switch t := t.(type) {
		case xml.StartElement:
			depth++
			if depth == 1 {
				root = true
				if t.Name.Local != "packet" {
					return nil, fmt.Errorf("the MTIP export's root element is <%s>, not <packet>", t.Name.Local)
				}
			}
			switch depth {
			case 2:
				if t.Name.Local == "metadata" {
					var n node
					if err := dec.DecodeElement(&n, &t); err != nil {
						return nil, fmt.Errorf("the MTIP export is not well-formed XML: %w", err)
					}
					ex.MTIPVersion = n.textOf("mtipVersion")
					ex.CameoVersion = n.textOf("cameoVersion")
					ex.ExportTime = n.textOf("exportTime")
					depth--
				} else if t.Name.Local == "data" {
					var n node
					if err := dec.DecodeElement(&n, &t); err != nil {
						return nil, fmt.Errorf("the MTIP export is not well-formed XML: %w", err)
					}
					ex.Records++
					if d, ok := diagram(&n); ok {
						ex.Diagrams = append(ex.Diagrams, *d)
					}
					depth--
				}
			}
		case xml.EndElement:
			depth--
		}
	}
	if !root {
		return nil, fmt.Errorf("the MTIP export is empty")
	}
	return ex, nil
}

// diagram interprets one <data> record; ok is false when it is model content
// rather than a diagram.
func diagram(n *node) (*Diagram, bool) {
	rel := n.child("relationships")
	if rel == nil {
		return nil, false
	}
	elements := rel.child("element")
	connectors := rel.child("diagramConnector")
	if elements == nil && connectors == nil {
		return nil, false
	}
	d := &Diagram{
		Type:        n.textOf("type"),
		Name:        recordName(n),
		Unsupported: map[string]int{},
	}
	if id := n.child("id"); id != nil {
		d.ID = id.textOf("cameo")
	} else {
		d.Malformed = append(d.Malformed, "the diagram record has no <id>")
	}
	if elements != nil {
		for i, el := range keyed(elements.children) {
			d.placement(el, i)
		}
	}
	if connectors != nil {
		for i, c := range keyed(connectors.children) {
			d.connector(c, i)
		}
	}
	return d, true
}

// recordName reads attributes/attribute[key="name"]/attribute[key="value"].
func recordName(n *node) string {
	attrs := n.child("attributes")
	if attrs == nil {
		return ""
	}
	for _, a := range attrs.children {
		if a.tag == "attribute" && a.key == "name" {
			for _, v := range a.children {
				if v.tag == "attribute" && v.key == "value" {
					return strings.TrimSpace(v.text)
				}
			}
		}
	}
	return ""
}

// number parses a bound or coordinate; empty is a missing one, and a
// non-finite value is malformed rather than an invalid token written out.
func number(n *node) (float64, bool) {
	v, err := strconv.ParseFloat(strings.TrimSpace(n.text), 64)
	return v, err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

// unsupported counts every relationship_metadata child that is not a known tag.
func (d *Diagram) unsupported(meta *node, known ...string) {
	for _, c := range meta.children {
		keep := false
		for _, k := range known {
			if c.tag == k {
				keep = true
				break
			}
		}
		if !keep {
			d.Unsupported[c.tag]++
		}
	}
}

// placement reads one shown element's bounds. Cameo stores y negated with the
// top at or below zero, so x = left, y = -top, width = right - left,
// height = top - bottom; negative results are kept as-is.
func (d *Diagram) placement(el *node, ordinal int) {
	meta := el.child("relationship_metadata")
	id := ""
	if c := el.child("id"); c != nil {
		id = strings.TrimSpace(c.text)
	}
	what := id
	if what == "" {
		what = fmt.Sprintf("placement #%d", ordinal)
	}
	fail := func(problem string) {
		d.Malformed = append(d.Malformed, what+": "+problem)
	}
	if id == "" {
		fail("no <id>")
		return
	}
	if meta == nil {
		// A record carrying no relationship_metadata holds no geometry to
		// skip or flag: like an empty element list, it is a valid record.
		return
	}
	d.unsupported(meta, "top", "bottom", "left", "right")
	bounds := map[string]float64{}
	for _, tag := range []string{"top", "bottom", "left", "right"} {
		c := meta.child(tag)
		if c == nil {
			fail("no <" + tag + "> bound")
			return
		}
		v, ok := number(c)
		if !ok {
			fail("a non-numeric <" + tag + "> bound " + strconv.Quote(strings.TrimSpace(c.text)))
			return
		}
		bounds[tag] = v
	}
	d.Placements = append(d.Placements, Placement{
		ID:     id,
		Type:   el.textOf("type"),
		X:      bounds["left"],
		Y:      -bounds["top"],
		Width:  bounds["right"] - bounds["left"],
		Height: bounds["top"] - bounds["bottom"],
	})
}

// point reads one xCoordinate/yCoordinate pair.
func point(n *node) (x, y float64, ok bool) {
	cx, cy := n.child("xCoordinate"), n.child("yCoordinate")
	if cx == nil || cy == nil {
		return 0, 0, false
	}
	x, okX := number(cx)
	y, okY := number(cy)
	return x, y, okX && okY
}

// connector reads one drawn edge's route. The client point is the edge's
// source end and the supplier point its target end; the breakpoints run
// supplier to client, so the written route is client, breakpoints reversed,
// supplier.
func (d *Diagram) connector(c *node, ordinal int) {
	id := ""
	if el := c.child("id"); el != nil {
		id = strings.TrimSpace(el.text)
	}
	what := id
	if what == "" {
		what = fmt.Sprintf("connector #%d", ordinal)
	}
	fail := func(problem string) {
		d.Malformed = append(d.Malformed, what+": "+problem)
	}
	if id == "" {
		fail("no <id>")
		return
	}
	meta := c.child("relationship_metadata")
	if meta == nil {
		// No relationship_metadata means the export recorded the edge but no
		// route for it: a valid record, not a malformed one.
		return
	}
	d.unsupported(meta, "supplierPoint", "clientPoint", "breakPoint")
	supplier, client := meta.child("supplierPoint"), meta.child("clientPoint")
	if supplier == nil || client == nil {
		fail("no <supplierPoint> or <clientPoint>")
		return
	}
	sx, sy, okS := point(supplier)
	cx, cy, okC := point(client)
	if !okS || !okC {
		fail("a point is missing a coordinate")
		return
	}
	var breaks [][2]float64
	if bp := meta.child("breakPoint"); bp != nil {
		for _, b := range keyed(bp.children) {
			x, y, ok := point(b)
			if !ok {
				fail("a <breakPoint> is missing a coordinate")
				return
			}
			breaks = append(breaks, [2]float64{x, y})
		}
	}
	points := []float64{cx, cy}
	for i := len(breaks) - 1; i >= 0; i-- {
		points = append(points, breaks[i][0], breaks[i][1])
	}
	points = append(points, sx, sy)
	d.Connectors = append(d.Connectors, Connector{ID: id, Type: c.textOf("type"), Points: points})
}
