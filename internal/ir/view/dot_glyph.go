package view

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
)

// dotDefinitionClass is the class of a typing edge, which Graphviz writes on the edge's SVG group.
const dotDefinitionClass = "definition"

// The dots of the notation's `definition` figure in units of its triangle's length:
// how far behind the base and off the line their centres sit, and their radius.
const (
	definitionDotBack   = 0.43
	definitionDotOffset = 0.36
	definitionDotRadius = 0.17
)

// DrawDOTGlyphs completes a Graphviz SVG drawing of the DOT form with what no
// arrowhead draws: the two dots beside the base of each typing edge's triangle.
func DrawDOTGlyphs(svg []byte) ([]byte, error) {
	if !bytes.Contains(svg, []byte(dotDefinitionClass)) {
		return svg, nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	var out []byte
	var dots string
	written, depth := 0, 0
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read the SVG drawing: %w", err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			switch {
			case depth > 0:
				depth++
				if element.Name.Local == "polygon" {
					dots = definitionDots(element)
				}
			case element.Name.Local == "g" && definitionEdge(svgAttr(element, "class")):
				depth = 1
			}
		case xml.EndElement:
			if depth == 0 {
				continue
			}
			depth--
			if dots != "" {
				end := int(decoder.InputOffset())
				out = append(append(out, svg[written:end]...), dots...)
				written, dots = end, ""
			}
		}
	}
	if out == nil {
		return svg, nil
	}
	return append(out, svg[written:]...), nil
}

// definitionEdge reports whether an SVG group of these classes is a typing edge as Graphviz draws it.
func definitionEdge(class string) bool {
	classes := strings.Fields(class)
	return slices.Contains(classes, "edge") && slices.Contains(classes, dotDefinitionClass)
}

// definitionDots is the markup of the two dots a hollow triangle of a typing edge
// takes, in the edge's colour; a polygon that is no triangle takes none.
func definitionDots(polygon xml.StartElement) string {
	tip, base, ok := svgTriangle(svgAttr(polygon, "points"))
	if !ok {
		return ""
	}
	length := math.Hypot(tip.X-base.X, tip.Y-base.Y)
	if length == 0 {
		return ""
	}
	along := Point{X: (base.X - tip.X) / length, Y: (base.Y - tip.Y) / length}
	centre := Point{X: base.X + along.X*definitionDotBack*length, Y: base.Y + along.Y*definitionDotBack*length}
	fill := ` fill="` + svgAttribute(svgAttr(polygon, "stroke")) + `"`
	if opacity := svgAttr(polygon, "stroke-opacity"); opacity != "" {
		fill += ` fill-opacity="` + svgAttribute(opacity) + `"`
	}
	var dots strings.Builder
	for _, side := range []float64{1, -1} {
		offset := side * definitionDotOffset * length
		fmt.Fprintf(&dots, "\n<circle%s stroke=\"none\" cx=\"%s\" cy=\"%s\" r=\"%s\"/>", fill,
			svgNumber(centre.X-along.Y*offset), svgNumber(centre.Y+along.X*offset), svgNumber(definitionDotRadius*length))
	}
	return dots.String()
}

// svgTriangle reads a polygon's points as an arrowhead triangle, closed or not:
// its tip, the corner opposite its shortest side, and that side's midpoint.
func svgTriangle(points string) (tip, base Point, ok bool) {
	var corners []Point
	for _, pair := range strings.Fields(points) {
		x, y, found := strings.Cut(pair, ",")
		px, errX := strconv.ParseFloat(x, 64)
		py, errY := strconv.ParseFloat(y, 64)
		if !found || errX != nil || errY != nil {
			return Point{}, Point{}, false
		}
		corners = append(corners, Point{X: px, Y: py})
	}
	if len(corners) == 4 && corners[0] == corners[3] {
		corners = corners[:3]
	}
	if len(corners) != 3 {
		return Point{}, Point{}, false
	}
	shortest := math.Inf(1)
	for i := range corners {
		a, b := corners[(i+1)%3], corners[(i+2)%3]
		if side := math.Hypot(a.X-b.X, a.Y-b.Y); side < shortest {
			shortest, tip, base = side, corners[i], Point{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2}
		}
	}
	return tip, base, true
}

// svgAttr is the value of an element's attribute of that local name, empty without one.
func svgAttr(element xml.StartElement, name string) string {
	for _, attr := range element.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}
