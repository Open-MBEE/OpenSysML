package view

import (
	"bytes"
	"cmp"
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

// The classes of the edges whose figure no Graphviz arrowhead draws whole, named for their productions.
const (
	dotPortionClass             = "portion-relationship"
	dotReferenceSubsettingClass = "reference-subsetting"
	dotOwnedMembershipClass     = "owned-membership"
)

// dotGlyphClasses are the edge classes DrawDOTGlyphs completes.
var dotGlyphClasses = []string{dotDefinitionClass, dotPortionClass, dotReferenceSubsettingClass, dotOwnedMembershipClass}

// The second pair of a reference subsetting's four dots sits this share of the triangle's length behind the first.
const referenceSubsettingDotStep = 0.5

// svgEdit replaces the bytes [from, to) of a drawing with markup.
type svgEdit struct {
	from, to int
	markup   string
}

// glyphEdge is a marked edge group being read: its class, the ends of its line,
// its triangle, whether the child being read holds a `+` label, the last such child and whether its circle took the plus.
type glyphEdge struct {
	class     string
	ends      []Point
	tip, base Point
	triangle  bool
	labelled  bool
	plus      *svgEdit
	crossed   bool
}

// DrawDOTGlyphs completes a Graphviz SVG drawing of the DOT form with what no arrowhead draws: the two
// dots of a typing edge, a reference subsetting's four, a portion's three-quarter circle and a membership's circle-plus.
func DrawDOTGlyphs(svg []byte) ([]byte, error) {
	// Graphviz writes a classed edge's group as class="edge …", a hyphen of the class as a character reference.
	if !bytes.Contains(svg, []byte(`class="edge `)) {
		return svg, nil
	}
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	scan := glyphScan{svg: svg}
	for {
		before := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read the SVG drawing: %w", err)
		}
		switch element := token.(type) {
		case xml.StartElement:
			scan.start(element, before)
		case xml.CharData:
			if scan.edge != nil {
				scan.text.Write(element)
			}
		case xml.EndElement:
			if scan.edge != nil {
				scan.end(int(decoder.InputOffset()))
			}
		}
	}
	edits := scan.edits
	if len(edits) == 0 {
		return svg, nil
	}
	slices.SortStableFunc(edits, func(a, b svgEdit) int { return a.from - b.from })
	var out []byte
	written := 0
	for _, edit := range edits {
		out = append(append(out, svg[written:edit.from]...), edit.markup...)
		written = edit.to
	}
	return append(out, svg[written:]...), nil
}

// glyphScan is DrawDOTGlyphs' read of the drawing: the edits so far, the marked edge group being
// read with its open elements and where each starts, and the text of the element being read.
type glyphScan struct {
	svg    []byte
	edits  []svgEdit
	edge   *glyphEdge
	open   []xml.StartElement
	starts []int
	text   strings.Builder
}

// start opens element, read at offset before: a marked edge group, or an element inside one.
func (s *glyphScan) start(element xml.StartElement, before int) {
	switch {
	case s.edge != nil:
		s.open, s.starts = append(s.open, element.Copy()), append(s.starts, before)
		s.text.Reset()
	case element.Name.Local == "g":
		if class := dotGlyphClass(svgAttr(element, "class")); class != "" {
			s.edge = &glyphEdge{class: class}
			s.open, s.starts = append(s.open, element.Copy()), append(s.starts, before)
		}
	}
}

// end closes the innermost open element of the edge group at offset end, drawing what it calls for.
func (s *glyphScan) end(end int) {
	edge := s.edge
	last := len(s.open) - 1
	s.edits = append(s.edits, edge.draw(s.open[last], s.starts[last], end, s.text.String())...)
	if last == 1 && edge.labelled {
		// The label goes with the link Graphviz wraps it in.
		edge.plus, edge.labelled = &svgEdit{from: s.starts[last], to: end}, false
	}
	s.open, s.starts = s.open[:last], s.starts[:last]
	if last > 0 {
		return
	}
	if edge.plus != nil && edge.crossed {
		// The label's line goes with it.
		if bytes.HasPrefix(s.svg[edge.plus.to:], []byte("\n")) {
			edge.plus.to++
		}
		s.edits = append(s.edits, *edge.plus)
	}
	s.edge = nil
}

// dotGlyphClass is the glyph class of an SVG group of these classes when it is
// an edge as Graphviz draws it, empty otherwise.
func dotGlyphClass(class string) string {
	classes := strings.Fields(class)
	if !slices.Contains(classes, "edge") {
		return ""
	}
	for _, glyph := range dotGlyphClasses {
		if slices.Contains(classes, glyph) {
			return glyph
		}
	}
	return ""
}

// draw is what the element of the edge's group closed at [from, to) takes: Graphviz
// writes the line ahead of its arrowheads and the tail label last of the labels.
func (e *glyphEdge) draw(element xml.StartElement, from, to int, text string) []svgEdit {
	switch element.Name.Local {
	case "text":
		e.labelled = e.class == dotOwnedMembershipClass && text == "+"
	case "path":
		if ends := svgPathEnds(svgAttr(element, "d")); ends != nil {
			e.ends = ends
		}
	case "polygon":
		e.tip, e.base, e.triangle = svgTriangle(svgAttr(element, "points"))
		if e.triangle && e.class == dotDefinitionClass {
			if dots := triangleDots(element, e.tip, e.base, definitionDotBack); dots != "" {
				return []svgEdit{{from: to, to: to, markup: dots}}
			}
		}
	case "ellipse":
		centre, radius, ok := svgCircle(element)
		if !ok {
			return nil
		}
		switch e.class {
		case dotPortionClass:
			if markup := portionGlyph(element, centre, radius, e.ends); markup != "" {
				return []svgEdit{{from: from, to: to, markup: markup}}
			}
		case dotReferenceSubsettingClass:
			if markup := e.fourDots(element, centre); markup != "" {
				return []svgEdit{{from: from, to: to, markup: markup}}
			}
		case dotOwnedMembershipClass:
			if along, ok := towardsLine(centre, e.ends); ok {
				e.crossed = true
				cross := fmt.Sprintf("M%s,%sL%s,%sM%s,%sL%s,%s",
					svgNumber(centre.X-along.X*radius), svgNumber(centre.Y-along.Y*radius), svgNumber(centre.X+along.X*radius), svgNumber(centre.Y+along.Y*radius),
					svgNumber(centre.X+along.Y*radius), svgNumber(centre.Y-along.X*radius), svgNumber(centre.X-along.Y*radius), svgNumber(centre.Y+along.X*radius))
				return []svgEdit{{from: to, to: to, markup: "\n<path fill=\"none\"" + svgStroke(element) + ` d="` + cross + `"/>`}}
			}
		}
	}
	return nil
}

// fourDots is the notation's `reference-subsetting` figure in place of Graphviz's one dot: the line
// carried on to the triangle's base between two pairs of dots; without a triangle the dot stays.
func (e *glyphEdge) fourDots(dot xml.StartElement, centre Point) string {
	if !e.triangle {
		return ""
	}
	end, ok := nearestEnd(centre, e.ends)
	if !ok {
		return ""
	}
	first := triangleDots(dot, e.tip, e.base, definitionDotBack)
	if first == "" {
		return ""
	}
	return strings.TrimPrefix(svgStem(dot, end, e.base), "\n") + first + triangleDots(dot, e.tip, e.base, definitionDotBack+referenceSubsettingDotStep)
}

// portionGlyph is the notation's `portion-relationship` figure in place of Graphviz's filled circle:
// a three-quarter circle whose missing quarter faces the line, which is carried on to its centre.
func portionGlyph(circle xml.StartElement, centre Point, radius float64, ends []Point) string {
	along, ok := towardsLine(centre, ends)
	end, _ := nearestEnd(centre, ends)
	if !ok {
		return ""
	}
	reach := radius * math.Sqrt2 / 2
	across := Point{X: -along.Y, Y: along.X}
	one := Point{X: centre.X + (along.X+across.X)*reach, Y: centre.Y + (along.Y+across.Y)*reach}
	other := Point{X: centre.X + (along.X-across.X)*reach, Y: centre.Y + (along.Y-across.Y)*reach}
	return fmt.Sprintf("%s\n<path%s stroke=\"none\" d=\"M%s,%sL%s,%sA%s,%s 0 1 1 %s,%sZ\"/>", strings.TrimPrefix(svgStem(circle, end, centre), "\n"), svgFill(circle),
		svgNumber(centre.X), svgNumber(centre.Y), svgNumber(one.X), svgNumber(one.Y), svgNumber(radius), svgNumber(radius), svgNumber(other.X), svgNumber(other.Y))
}

// svgStem is a line from a to b in the stroke of the arrowhead it replaces part of, undashed as arrowheads are.
func svgStem(arrowhead xml.StartElement, a, b Point) string {
	return fmt.Sprintf("\n<path fill=\"none\"%s d=\"M%s,%sL%s,%s\"/>", svgStroke(arrowhead), svgNumber(a.X), svgNumber(a.Y), svgNumber(b.X), svgNumber(b.Y))
}

// svgStroke is the stroke of an element as attributes.
func svgStroke(element xml.StartElement) string {
	stroke := ` stroke="` + svgAttribute(svgAttr(element, "stroke")) + `"`
	if value := svgAttr(element, "stroke-width"); value != "" {
		stroke += ` stroke-width="` + svgAttribute(value) + `"`
	}
	if opacity := svgAttr(element, "stroke-opacity"); opacity != "" {
		stroke += ` stroke-opacity="` + svgAttribute(opacity) + `"`
	}
	return stroke
}

// svgFill is an element's stroke colour and opacity as a fill.
func svgFill(element xml.StartElement) string {
	fill := ` fill="` + svgAttribute(svgAttr(element, "stroke")) + `"`
	if opacity := svgAttr(element, "stroke-opacity"); opacity != "" {
		fill += ` fill-opacity="` + svgAttribute(opacity) + `"`
	}
	return fill
}

// svgCircle reads an ellipse's centre and horizontal radius.
func svgCircle(ellipse xml.StartElement) (Point, float64, bool) {
	x, errX := strconv.ParseFloat(svgAttr(ellipse, "cx"), 64)
	y, errY := strconv.ParseFloat(svgAttr(ellipse, "cy"), 64)
	radius, errR := strconv.ParseFloat(svgAttr(ellipse, "rx"), 64)
	return Point{X: x, Y: y}, radius, errX == nil && errY == nil && errR == nil && radius > 0
}

// svgPathEnds are the first and last points of a path's data as Graphviz writes an
// edge's line, absolute and comma-paired; nil for any other data.
func svgPathEnds(data string) []Point {
	var points []Point
	for _, pair := range strings.FieldsFunc(data, func(r rune) bool { return r == 'M' || r == 'C' || r == 'L' || r == ' ' }) {
		x, y, found := strings.Cut(pair, ",")
		px, errX := strconv.ParseFloat(x, 64)
		py, errY := strconv.ParseFloat(y, 64)
		if !found || errX != nil || errY != nil {
			return nil
		}
		points = append(points, Point{X: px, Y: py})
	}
	if len(points) < 2 {
		return nil
	}
	return []Point{points[0], points[len(points)-1]}
}

// nearestEnd is the line end nearest a point, false without a line.
func nearestEnd(to Point, ends []Point) (Point, bool) {
	if len(ends) == 0 {
		return Point{}, false
	}
	return slices.MinFunc(ends, func(a, b Point) int {
		return cmp.Compare(math.Hypot(a.X-to.X, a.Y-to.Y), math.Hypot(b.X-to.X, b.Y-to.Y))
	}), true
}

// towardsLine is the unit direction from an arrowhead's centre to the end of the line it stands at.
func towardsLine(centre Point, ends []Point) (Point, bool) {
	end, ok := nearestEnd(centre, ends)
	length := math.Hypot(end.X-centre.X, end.Y-centre.Y)
	if !ok || length == 0 {
		return Point{}, false
	}
	return Point{X: (end.X - centre.X) / length, Y: (end.Y - centre.Y) / length}, true
}

// triangleDots is the markup of two dots abreast across the line, back triangle lengths behind the
// base of a hollow triangle, in the colour of the arrowhead painted; a triangle without length takes none.
func triangleDots(painted xml.StartElement, tip, base Point, back float64) string {
	length := math.Hypot(tip.X-base.X, tip.Y-base.Y)
	if length == 0 {
		return ""
	}
	along := Point{X: (base.X - tip.X) / length, Y: (base.Y - tip.Y) / length}
	centre := Point{X: base.X + along.X*back*length, Y: base.Y + along.Y*back*length}
	fill := svgFill(painted)
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

// dotClass is the DOT attribute setting an edge's class, quoted as a hyphenated name must be.
func dotClass(class string) string { return `class="` + class + `"` }
