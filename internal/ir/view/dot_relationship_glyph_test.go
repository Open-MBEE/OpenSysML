package view

import (
	"bytes"
	"encoding/xml"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// drawnEdge is an edge group of a Graphviz SVG: its id and glyph class, its markup,
// and what is drawn in it.
type drawnEdge struct {
	id, class string
	markup    string
	ellipses  []xml.StartElement
	circles   []drawnDot
	paths     []xml.StartElement
	triangles []drawnTriangle
	texts     []string
}

// drawnEdges reads the edge groups of a Graphviz SVG in order.
func drawnEdges(t *testing.T, svg []byte) []drawnEdge {
	t.Helper()
	var edges []drawnEdge
	depth, start := 0, 0
	inText := false
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	for {
		before := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			return edges
		}
		if err != nil {
			t.Fatalf("the drawing is not well-formed: %v\n%s", err, svg)
		}
		switch element := token.(type) {
		case xml.StartElement:
			class := strings.Fields(svgAttr(element, "class"))
			if depth == 0 {
				if element.Name.Local != "g" || len(class) == 0 || class[0] != "edge" {
					continue
				}
				edges = append(edges, drawnEdge{id: svgAttr(element, "id"), class: strings.Join(class[1:], " ")})
				depth, start = 1, before
				continue
			}
			depth++
			edge := &edges[len(edges)-1]
			switch element.Name.Local {
			case "ellipse":
				edge.ellipses = append(edge.ellipses, element.Copy())
			case "path":
				edge.paths = append(edge.paths, element.Copy())
			case "polygon":
				if tip, base, ok := svgTriangle(svgAttr(element, "points")); ok {
					edge.triangles = append(edge.triangles, drawnTriangle{edge: edge.id, tip: tip, base: base,
						stroke: svgAttr(element, "stroke"), opacity: svgAttr(element, "stroke-opacity")})
				}
			case "circle":
				edge.circles = append(edge.circles, drawnDot{
					centre: Point{X: svgFloat(t, element, "cx"), Y: svgFloat(t, element, "cy")}, radius: svgFloat(t, element, "r"),
					fill: svgAttr(element, "fill"), stroke: svgAttr(element, "stroke"), opacity: svgAttr(element, "fill-opacity"),
				})
			case "text":
				inText = true
			}
		case xml.CharData:
			if inText {
				edges[len(edges)-1].texts = append(edges[len(edges)-1].texts, string(element))
			}
		case xml.EndElement:
			inText = false
			if depth == 0 {
				continue
			}
			if depth--; depth == 0 {
				edges[len(edges)-1].markup = string(svg[start:decoder.InputOffset()])
			}
		}
	}
}

var svgPathNumber = regexp.MustCompile(`-?[0-9]+(?:\.[0-9]+)?`)

// pathPoints are the coordinate pairs of a path's data whose every number is a coordinate.
func pathPoints(t *testing.T, data string) []Point {
	t.Helper()
	numbers := svgPathNumber.FindAllString(data, -1)
	if len(numbers)%2 != 0 {
		t.Fatalf("path data %q has %d numbers", data, len(numbers))
	}
	var points []Point
	for i := 0; i < len(numbers); i += 2 {
		x, _ := strconv.ParseFloat(numbers[i], 64)
		y, _ := strconv.ParseFloat(numbers[i+1], 64)
		points = append(points, Point{X: x, Y: y})
	}
	return points
}

// The drawing's coordinates are written to two decimals, so a point computed from two of them is this near.
const drawnRounding = 0.02

// glyphPaint is the paint a drawn edge's line carries, which its glyph takes.
type glyphPaint struct{ stroke, opacity, width string }

func (e drawnEdge) paint() glyphPaint {
	line := e.paths[0]
	return glyphPaint{svgAttr(line, "stroke"), svgAttr(line, "stroke-opacity"), svgAttr(line, "stroke-width")}
}

// checkPortionGlyph holds a drawn portion edge to the notation's `portion-relationship` figure: no full
// circle, but a filled three-quarter circle whose missing quarter faces the line, which reaches its centre.
func checkPortionGlyph(t *testing.T, edge drawnEdge) {
	t.Helper()
	if len(edge.ellipses) != 0 || len(edge.paths) != 3 {
		t.Errorf("%s: drawn with %d full circles and %d paths, want the line, its stem and the three-quarter circle:\n%s", edge.id, len(edge.ellipses), len(edge.paths), edge.markup)
		return
	}
	paint := edge.paint()
	line, stem, wedge := pathPoints(t, svgAttr(edge.paths[0], "d")), pathPoints(t, svgAttr(edge.paths[1], "d")), edge.paths[2]
	data := svgAttr(wedge, "d")
	arc := regexp.MustCompile(`^M(\S+),(\S+)L(\S+),(\S+)A(\S+),(\S+) 0 1 1 (\S+),(\S+)Z$`).FindStringSubmatch(data)
	if arc == nil {
		t.Errorf("%s: the glyph's path %q is no wedge closed by one large arc", edge.id, data)
		return
	}
	var n [8]float64
	for i := range n {
		n[i], _ = strconv.ParseFloat(arc[i+1], 64)
	}
	centre, one, radius, other := Point{X: n[0], Y: n[1]}, Point{X: n[2], Y: n[3]}, n[4], Point{X: n[6], Y: n[7]}
	if n[4] != n[5] || math.Abs(distance(centre, one)-radius) > drawnRounding || math.Abs(distance(centre, other)-radius) > drawnRounding {
		t.Errorf("%s: the arc of %q is no circle's about its start", edge.id, data)
	}
	// A quarter is missing: the two radii stand at right angles, a chord of √2 radii apart.
	if math.Abs(distance(one, other)-radius*math.Sqrt2) > drawnRounding {
		t.Errorf("%s: the mouth is %.2f wide, want %.2f, a quarter of a circle of radius %.2f", edge.id, distance(one, other), radius*math.Sqrt2, radius)
	}
	end := line[0]
	if distance(line[len(line)-1], centre) < distance(end, centre) {
		end = line[len(line)-1]
	}
	mouth := Point{X: (one.X + other.X) / 2, Y: (one.Y + other.Y) / 2}
	if facing := (mouth.X-centre.X)*(end.X-centre.X) + (mouth.Y-centre.Y)*(end.Y-centre.Y); facing <= 0 || math.Abs(distance(mouth, end)+distance(mouth, centre)-distance(centre, end)) > drawnRounding {
		t.Errorf("%s: the missing quarter, about (%.2f,%.2f), does not face the line's end (%.2f,%.2f) from (%.2f,%.2f)", edge.id, mouth.X, mouth.Y, end.X, end.Y, centre.X, centre.Y)
	}
	if len(stem) != 2 || stem[0] != end || stem[1] != centre {
		t.Errorf("%s: the stem runs %v, want from the line's end %v to the centre %v", edge.id, stem, end, centre)
	}
	if got := (glyphPaint{svgAttr(edge.paths[1], "stroke"), svgAttr(edge.paths[1], "stroke-opacity"), svgAttr(edge.paths[1], "stroke-width")}); got != paint {
		t.Errorf("%s: the stem is painted %+v, want the line's %+v", edge.id, got, paint)
	}
	if svgAttr(wedge, "fill") != paint.stroke || svgAttr(wedge, "fill-opacity") != paint.opacity || svgAttr(wedge, "stroke") != "none" {
		t.Errorf("%s: the three-quarter circle is painted as %s, want filled in the line's %+v", edge.id, edge.markup, paint)
	}
}

// checkReferenceSubsettingGlyph holds a drawn edge to the `reference-subsetting` figure: a hollow triangle with
// four small dots behind its base, two each side of the line, which runs on between them to the base.
func checkReferenceSubsettingGlyph(t *testing.T, edge drawnEdge) {
	t.Helper()
	if len(edge.ellipses) != 0 || len(edge.triangles) != 1 || len(edge.circles) != 4 || len(edge.paths) != 2 {
		t.Errorf("%s: drawn with %d full-size dots, %d triangles, %d small dots and %d paths, want one triangle, four dots, the line and its stem:\n%s",
			edge.id, len(edge.ellipses), len(edge.triangles), len(edge.circles), len(edge.paths), edge.markup)
		return
	}
	paint, triangle := edge.paint(), edge.triangles[0]
	length := distance(triangle.tip, triangle.base)
	along := Point{X: (triangle.base.X - triangle.tip.X) / length, Y: (triangle.base.Y - triangle.tip.Y) / length}
	const rounding = 0.01
	sides := map[float64]float64{}
	for _, dot := range edge.circles {
		dx, dy := dot.centre.X-triangle.base.X, dot.centre.Y-triangle.base.Y
		behind, beside := dx*along.X+dy*along.Y, -dx*along.Y+dy*along.X
		row := math.Round((behind/length - definitionDotBack) / referenceSubsettingDotStep)
		if row != 0 && row != 1 || math.Abs(behind-(definitionDotBack+row*referenceSubsettingDotStep)*length) > rounding {
			t.Errorf("%s: a dot sits %.2f behind the base of a %.2f triangle, in neither of the figure's two rows", edge.id, behind, length)
		}
		if math.Abs(math.Abs(beside)-definitionDotOffset*length) > rounding || math.Abs(dot.radius-definitionDotRadius*length) > rounding {
			t.Errorf("%s: a dot of radius %.2f sits %.2f off the line, want %.2f at %.2f", edge.id, dot.radius, beside, definitionDotRadius*length, definitionDotOffset*length)
		}
		if dot.fill != paint.stroke || dot.stroke != "none" || dot.opacity != paint.opacity {
			t.Errorf("%s: a dot is painted fill=%q stroke=%q fill-opacity=%q, want the line's %+v and no outline", edge.id, dot.fill, dot.stroke, dot.opacity, paint)
		}
		sides[row] += beside
	}
	// Each row's two dots sit opposite one another, so their offsets cancel.
	if len(sides) != 2 || math.Abs(sides[0]) > 2*rounding || math.Abs(sides[1]) > 2*rounding {
		t.Errorf("%s: the dots are not two pairs astride the line: %v", edge.id, edge.circles)
	}
	// The rows clear one another and the base.
	if referenceSubsettingDotStep <= 2*definitionDotRadius || definitionDotBack <= definitionDotRadius {
		t.Errorf("the rows of dots touch: step %.2f, back %.2f, radius %.2f", referenceSubsettingDotStep, definitionDotBack, definitionDotRadius)
	}
	line, stem := pathPoints(t, svgAttr(edge.paths[0], "d")), pathPoints(t, svgAttr(edge.paths[1], "d"))
	end := line[0]
	if distance(line[len(line)-1], triangle.base) < distance(end, triangle.base) {
		end = line[len(line)-1]
	}
	if len(stem) != 2 || stem[0] != end || distance(stem[1], triangle.base) > rounding {
		t.Errorf("%s: the stem runs %v, want from the line's end %v to the triangle's base %v", edge.id, stem, end, triangle.base)
	}
}

// checkOwnedMembershipGlyph holds a drawn edge to the `owned-membership` figure: a hollow circle with a plus
// inside it, along and across the line, and no `+` label standing for one.
func checkOwnedMembershipGlyph(t *testing.T, edge drawnEdge, labels []string) {
	t.Helper()
	if len(edge.ellipses) != 1 || len(edge.paths) != 2 {
		t.Errorf("%s: drawn with %d circles and %d paths, want one circle, the line and the plus:\n%s", edge.id, len(edge.ellipses), len(edge.paths), edge.markup)
		return
	}
	if strings.Join(edge.texts, "|") != strings.Join(labels, "|") {
		t.Errorf("%s: labelled %q, want %q with the tail's `+` drawn in the circle", edge.id, edge.texts, labels)
	}
	circle := edge.ellipses[0]
	centre, radius, ok := svgCircle(circle)
	if !ok || svgAttr(circle, "fill") != "none" {
		t.Fatalf("%s: the circle is no hollow one:\n%s", edge.id, edge.markup)
	}
	line, plus := pathPoints(t, svgAttr(edge.paths[0], "d")), pathPoints(t, svgAttr(edge.paths[1], "d"))
	if len(plus) != 4 {
		t.Fatalf("%s: the plus is %q, want two strokes", edge.id, svgAttr(edge.paths[1], "d"))
	}
	for _, point := range plus {
		if math.Abs(distance(point, centre)-radius) > drawnRounding {
			t.Errorf("%s: a stroke of the plus ends %.2f from the centre of a circle of radius %.2f", edge.id, distance(point, centre), radius)
		}
	}
	end := line[0]
	if distance(line[len(line)-1], centre) < distance(end, centre) {
		end = line[len(line)-1]
	}
	first := Point{X: plus[1].X - plus[0].X, Y: plus[1].Y - plus[0].Y}
	second := Point{X: plus[3].X - plus[2].X, Y: plus[3].Y - plus[2].Y}
	toLine := Point{X: end.X - centre.X, Y: end.Y - centre.Y}
	// Both strokes are diameters, one along the line and one across it.
	if math.Abs(distance(plus[0], plus[1])-2*radius) > drawnRounding || math.Abs(distance(plus[2], plus[3])-2*radius) > drawnRounding ||
		math.Abs(first.X*toLine.Y-first.Y*toLine.X) > drawnRounding*distance(centre, end)*2 || math.Abs(second.X*toLine.X+second.Y*toLine.Y) > drawnRounding*distance(centre, end)*2 {
		t.Errorf("%s: the plus %v is not a diameter along the line and one across it, about %v towards %v", edge.id, plus, centre, end)
	}
	if got, want := svgStroke(edge.paths[1]), svgStroke(circle); got != want || svgAttr(edge.paths[1], "fill") != "none" {
		t.Errorf("%s: the plus is stroked%s, want the circle's%s", edge.id, got, want)
	}
}

// checkRelationshipGlyph holds a drawn edge to the figure of its class, and reports whether it has one.
func checkRelationshipGlyph(t *testing.T, edge drawnEdge) bool {
	t.Helper()
	switch edge.class {
	case dotPortionClass:
		checkPortionGlyph(t, edge)
	case dotReferenceSubsettingClass:
		checkReferenceSubsettingGlyph(t, edge)
	case dotOwnedMembershipClass:
		var labels []string
		for _, text := range edge.texts {
			if text != "+" {
				labels = append(labels, text)
			}
		}
		checkOwnedMembershipGlyph(t, edge, labels)
	default:
		return false
	}
	return true
}

// Graphviz 16's drawing of testdata/relationship-glyphs.dot takes the three-quarter circle, the four dots and the
// circle-plus at the edges classed for them and nothing else: slanted, linked, dashed, heavy, enlarged and translucent ones.
func TestDrawDOTGlyphsDrawsTheRelationshipGlyphs(t *testing.T) {
	recorded, err := os.ReadFile(filepath.Join("testdata", "relationship-glyphs.graphviz.svg"))
	if err != nil {
		t.Fatal(err)
	}
	drawn, err := DrawDOTGlyphs(recorded)
	if err != nil {
		t.Fatalf("DrawDOTGlyphs: %v", err)
	}
	before, after := drawnEdges(t, recorded), drawnEdges(t, drawn)
	if len(before) != 9 || len(after) != 9 {
		t.Fatalf("the drawings have %d and %d edges, want 9", len(before), len(after))
	}
	for _, edge := range after[:4] {
		if !checkRelationshipGlyph(t, edge) {
			t.Errorf("%s has no glyph class: %q", edge.id, edge.class)
		}
	}
	// The second membership's own `+` label and its head label stay; only the tail's stood for the glyph.
	checkOwnedMembershipGlyph(t, after[4], nil)
	checkOwnedMembershipGlyph(t, after[5], []string{"+", "h"})
	if strings.Contains(after[5].markup, "taillabel") || !strings.Contains(after[5].markup, "headlabel") {
		t.Errorf("the linked membership keeps the tail label's link or loses the head label's:\n%s", after[5].markup)
	}
	// An alias's hollow circle, a subsetting's triangle and an unclassed filled circle are other figures.
	for i := 6; i < 9; i++ {
		if after[i].markup != before[i].markup {
			t.Errorf("%s is no classed edge and is redrawn:\n%s", after[i].id, after[i].markup)
		}
	}
	// Nothing outside the classed edges moves.
	rest := drawn
	for i, edge := range after[:6] {
		rest = bytes.Replace(rest, []byte(edge.markup), []byte(before[i].markup), 1)
	}
	if !bytes.Equal(rest, recorded) {
		t.Errorf("the drawing changed outside the classed edges:\n%s", drawn)
	}
	for _, glyph := range []string{
		// The plain portion's line ends at (71.65,-82.98) below-right of a circle of radius 4 about (68.6,-85.9).
		`<path fill="none" stroke="#181818" d="M71.65,-82.98L68.6,-85.9"/>` + "\n" + `<path fill="#181818" stroke="none" d="M68.6,-85.9L68.69,-81.9A4,4 0 1 1 72.6,-85.99Z"/>`,
		// The heavy translucent one keeps its width and opacity.
		`<path fill="none" stroke="#ff0000" stroke-width="3" stroke-opacity="0.501961" d="M47.38,-70.05L47.38,-79.51"/>`,
		`<path fill="#ff0000" fill-opacity="0.501961" stroke="none" d="M47.38,-79.51L41.72,-73.85A8,8 0 1 1 53.04,-73.85Z"/>`,
		// The enlarged reference subsetting's triangle points down from a base at (295.38,-57.72), 20 long.
		`<circle fill="#ff0000" fill-opacity="0.501961" stroke="none" cx="302.58" cy="-66.32" r="3.4"/>`,
		`<circle fill="#ff0000" fill-opacity="0.501961" stroke="none" cx="288.18" cy="-76.32" r="3.4"/>`,
		// The upright membership's circle of radius 4 about (499.38,-84.51).
		`<path fill="none" stroke="#181818" d="M499.38,-88.51L499.38,-80.51M503.38,-84.51L495.38,-84.51"/>`,
	} {
		if !bytes.Contains(drawn, []byte(glyph)) {
			t.Errorf("the drawing lacks %s", glyph)
		}
	}
}

// A classed edge whose arrowhead or line is not what Graphviz draws for it is left as it is.
func TestDrawDOTGlyphsLeavesAnEdgeWithoutItsArrowhead(t *testing.T) {
	const line = `<path fill="none" stroke="black" d="M0,0C0,10 0,20 0,30"/>`
	for name, group := range map[string]string{
		"a portion without a line":                 `<g id="edge1" class="edge portion&#45;relationship"><ellipse fill="black" stroke="black" cx="0" cy="-4" rx="4" ry="4"/></g>`,
		"a portion on its line's end":              `<g id="edge1" class="edge portion&#45;relationship">` + line + `<ellipse fill="black" stroke="black" cx="0" cy="0" rx="4" ry="4"/></g>`,
		"a portion without a radius":               `<g id="edge1" class="edge portion&#45;relationship">` + line + `<ellipse fill="black" stroke="black" cx="0" cy="-4" rx="0" ry="0"/></g>`,
		"a portion of a relative line":             `<g id="edge1" class="edge portion&#45;relationship"><path d="m0,0 l0,30"/><ellipse fill="black" stroke="black" cx="0" cy="-4" rx="4" ry="4"/></g>`,
		"a reference subsetting with no triangle":  `<g id="edge1" class="edge reference&#45;subsetting">` + line + `<ellipse fill="black" stroke="black" cx="0" cy="34" rx="4" ry="4"/></g>`,
		"a membership without a circle":            `<g id="edge1" class="edge owned&#45;membership">` + line + `<text>+</text></g>`,
		"a membership without a line":              `<g id="edge1" class="edge owned&#45;membership"><ellipse fill="none" stroke="black" cx="0" cy="-4" rx="4" ry="4"/><text>+</text></g>`,
		"a node of the class":                      `<g id="node1" class="node owned&#45;membership">` + line + `<ellipse fill="none" stroke="black" cx="0" cy="-4" rx="4" ry="4"/><text>+</text></g>`,
		"an unclassed edge":                        `<g id="edge1" class="edge">` + line + `<ellipse fill="none" stroke="black" cx="0" cy="-4" rx="4" ry="4"/><text>+</text></g>`,
		"an edge of a class that is another's end": `<g id="edge1" class="edge not&#45;owned&#45;membership">` + line + `<ellipse fill="none" stroke="black" cx="0" cy="-4" rx="4" ry="4"/><text>+</text></g>`,
	} {
		svg := `<svg xmlns="http://www.w3.org/2000/svg">` + group + `</svg>`
		drawn, err := DrawDOTGlyphs([]byte(svg))
		if err != nil || string(drawn) != svg {
			t.Errorf("%s: drawn as %s (%v), want it unchanged", name, drawn, err)
		}
	}
	// A membership whose tail label is something else keeps it beside the plus drawn in the circle.
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><g id="edge1" class="edge owned&#45;membership">` + line + `<ellipse fill="none" stroke="black" cx="0" cy="-4" rx="4" ry="4"/><text>1..*</text></g></svg>`
	drawn, err := DrawDOTGlyphs([]byte(svg))
	if err != nil {
		t.Fatal(err)
	}
	checkOwnedMembershipGlyph(t, drawnEdges(t, drawn)[0], []string{"1..*"})
}

// An installed Graphviz draws every portion, reference subsetting and owned membership of the DOT form with an
// arrowhead DrawDOTGlyphs completes as the figure, laid out by dot and drawn along stated routes, and no other edge so.
func TestGraphvizDrawsTheRelationshipGlyphs(t *testing.T) {
	dot := anyDot(t)
	seen := map[string]int{}
	for _, name := range []string{"definitionGlyphs", "packageGlyphs", "treeGlyphs"} {
		rendering := render(t, "notation-glyphs.sysml", "NotationGlyphs::Diagrams::"+name)
		want := map[string]int{}
		for _, edge := range rendering.Edges {
			switch edge.Kind {
			case EdgePortion:
				want[dotPortionClass]++
			case EdgeReferenceSubsetting:
				want[dotReferenceSubsettingClass]++
			case EdgeContainment:
				want[dotOwnedMembershipClass]++
			}
		}
		for _, options := range []Options{{}, orthoOptions} {
			source, err := rendering.DOTWith(options)
			if err != nil {
				t.Fatalf("%s: DOTWith: %v", name, err)
			}
			engine, flags := DOTLayout(source)
			if engine == "" {
				engine = "dot"
			}
			command := exec.Command(dot, append(append([]string{"-K" + engine}, flags...), "-Tsvg")...)
			command.Stdin = strings.NewReader(source)
			var stderr strings.Builder
			command.Stderr = &stderr
			svg, err := command.Output()
			if err != nil {
				t.Fatalf("%s: %s -Tsvg: %v\n%s", name, engine, err, stderr.String())
			}
			drawn, err := DrawDOTGlyphs(svg)
			if err != nil {
				t.Fatalf("%s: DrawDOTGlyphs: %v", name, err)
			}
			got := map[string]int{}
			before := drawnEdges(t, svg)
			for i, edge := range drawnEdges(t, drawn) {
				switch {
				case checkRelationshipGlyph(t, edge):
					got[edge.class]++
					seen[edge.class]++
				case edge.class == "" && edge.markup != before[i].markup:
					t.Errorf("%s: %s is no classed edge and is redrawn:\n%s", name, edge.id, edge.markup)
				}
			}
			for _, class := range []string{dotPortionClass, dotReferenceSubsettingClass, dotOwnedMembershipClass} {
				if got[class] != want[class] {
					t.Errorf("%s (%s): %d %s edges are drawn, want the rendering's %d:\n%s", name, options.Lines, got[class], class, want[class], source)
				}
			}
		}
	}
	for _, class := range []string{dotPortionClass, dotReferenceSubsettingClass, dotOwnedMembershipClass} {
		if seen[class] == 0 {
			t.Errorf("no view of the fixture draws a %s edge", class)
		}
	}
}
