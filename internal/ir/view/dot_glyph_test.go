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

	"github.com/Open-MBEE/OpenSysML/internal/ir/view/layout"
)

// drawnTriangle is a hollow triangle of a drawn edge, with the circles that follow it in its group.
type drawnTriangle struct {
	edge       string
	definition bool
	tip, base  Point
	stroke     string
	opacity    string
	dots       []drawnDot
}

// drawnDot is a circle of a drawing: its centre, radius and paint.
type drawnDot struct {
	centre                Point
	radius                float64
	fill, stroke, opacity string
}

// drawnTriangles reads the arrowhead triangles of a Graphviz SVG, edge by edge.
func drawnTriangles(t *testing.T, svg []byte) []drawnTriangle {
	t.Helper()
	var triangles []drawnTriangle
	var edge string
	definition, depth := false, 0
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return triangles
		}
		if err != nil {
			t.Fatalf("the drawing is not well-formed: %v\n%s", err, svg)
		}
		switch element := token.(type) {
		case xml.StartElement:
			class := strings.Fields(svgAttr(element, "class"))
			switch {
			case depth > 0:
				depth++
			case element.Name.Local == "g" && len(class) > 0 && class[0] == "edge":
				edge, definition, depth = svgAttr(element, "id"), len(class) > 1 && class[1] == "definition", 1
			}
			if depth == 0 {
				continue
			}
			switch element.Name.Local {
			case "polygon":
				if tip, base, ok := svgTriangle(svgAttr(element, "points")); ok {
					triangles = append(triangles, drawnTriangle{edge: edge, definition: definition, tip: tip, base: base,
						stroke: svgAttr(element, "stroke"), opacity: svgAttr(element, "stroke-opacity")})
				}
			case "circle":
				if len(triangles) == 0 || triangles[len(triangles)-1].edge != edge {
					t.Fatalf("%s draws a circle ahead of any triangle", edge)
				}
				last := &triangles[len(triangles)-1]
				last.dots = append(last.dots, drawnDot{
					centre: Point{X: svgFloat(t, element, "cx"), Y: svgFloat(t, element, "cy")}, radius: svgFloat(t, element, "r"),
					fill: svgAttr(element, "fill"), stroke: svgAttr(element, "stroke"), opacity: svgAttr(element, "fill-opacity"),
				})
			}
		case xml.EndElement:
			if depth > 0 {
				depth--
			}
		}
	}
}

func svgFloat(t *testing.T, element xml.StartElement, name string) float64 {
	t.Helper()
	value, err := strconv.ParseFloat(svgAttr(element, name), 64)
	if err != nil {
		t.Fatalf("%s %s: %v", element.Name.Local, name, err)
	}
	return value
}

// checkDefinitionGlyph holds a drawn typing edge to the notation's `definition` figure: two small dots
// abreast across the line just behind the triangle's base, one each side, in the edge's colour.
func checkDefinitionGlyph(t *testing.T, triangle drawnTriangle) {
	t.Helper()
	if len(triangle.dots) != 2 {
		t.Errorf("%s: the triangle has %d dots, want 2", triangle.edge, len(triangle.dots))
		return
	}
	length := math.Hypot(triangle.tip.X-triangle.base.X, triangle.tip.Y-triangle.base.Y)
	along := Point{X: (triangle.base.X - triangle.tip.X) / length, Y: (triangle.base.Y - triangle.tip.Y) / length}
	// The drawing's coordinates are written to two decimals.
	const rounding = 0.01
	var sides []float64
	for _, dot := range triangle.dots {
		dx, dy := dot.centre.X-triangle.base.X, dot.centre.Y-triangle.base.Y
		behind, beside := dx*along.X+dy*along.Y, -dx*along.Y+dy*along.X
		if math.Abs(behind-definitionDotBack*length) > rounding {
			t.Errorf("%s: a dot sits %.2f behind the triangle's base, want %.2f, the figure's share of a %.2f triangle", triangle.edge, behind, definitionDotBack*length, length)
		}
		if math.Abs(math.Abs(beside)-definitionDotOffset*length) > rounding {
			t.Errorf("%s: a dot sits %.2f off the line, want %.2f", triangle.edge, math.Abs(beside), definitionDotOffset*length)
		}
		if math.Abs(dot.radius-definitionDotRadius*length) > rounding {
			t.Errorf("%s: a dot's radius is %.2f, want %.2f", triangle.edge, dot.radius, definitionDotRadius*length)
		}
		if dot.fill != triangle.stroke || dot.stroke != "none" || dot.opacity != triangle.opacity {
			t.Errorf("%s: a dot is painted fill=%q stroke=%q fill-opacity=%q, want the triangle's stroke %q at opacity %q and no outline",
				triangle.edge, dot.fill, dot.stroke, dot.opacity, triangle.stroke, triangle.opacity)
		}
		sides = append(sides, beside)
	}
	if sides[0]*sides[1] >= 0 {
		t.Errorf("%s: both dots sit on one side of the line (%.2f and %.2f off it)", triangle.edge, sides[0], sides[1])
	}
}

// The figure's dots are small beside its triangle and clear of the line between them.
func TestDefinitionDotsFollowTheFigure(t *testing.T) {
	// The figure's triangle is 11 long, its dots 1.86 in radius, 4.74 behind the base and 4 off the line.
	const figureLength, figureRadius, figureBack, figureOffset = 11, 1.857, 4.74, 4.0
	for name, pair := range map[string][2]float64{
		"back":   {definitionDotBack, figureBack / figureLength},
		"offset": {definitionDotOffset, figureOffset / figureLength},
		"radius": {definitionDotRadius, figureRadius / figureLength},
	} {
		if math.Abs(pair[0]-pair[1]) > 0.005 {
			t.Errorf("the dots' %s is %.3f of the triangle's length, the figure's %.3f", name, pair[0], pair[1])
		}
	}
	if definitionDotOffset-definitionDotRadius <= 0.05 {
		t.Errorf("the dots reach the line between them: offset %.2f, radius %.2f", definitionDotOffset, definitionDotRadius)
	}
	if definitionDotBack-definitionDotRadius <= 0 {
		t.Errorf("the dots reach the triangle's base: back %.2f, radius %.2f", definitionDotBack, definitionDotRadius)
	}
}

var drawnDotLine = regexp.MustCompile(`\n<circle [^>]*/>`)

// Graphviz 16's drawing of testdata/definition-glyph.dot takes two dots at each typing edge's triangle and
// nothing else: slanted, linked, dashed, heavy, enlarged and translucent edges, one with a tail diamond.
func TestDrawDOTGlyphsDotsTheDefinitionTriangles(t *testing.T) {
	recorded, err := os.ReadFile(filepath.Join("testdata", "definition-glyph.graphviz.svg"))
	if err != nil {
		t.Fatal(err)
	}
	drawn, err := DrawDOTGlyphs(recorded)
	if err != nil {
		t.Fatalf("DrawDOTGlyphs: %v", err)
	}
	if rest := drawnDotLine.ReplaceAll(drawn, nil); !bytes.Equal(rest, recorded) {
		t.Errorf("the drawing changed beyond the dots added:\n%s", drawn)
	}
	dotted := map[string]int{}
	for _, triangle := range drawnTriangles(t, drawn) {
		dotted[triangle.edge] = len(triangle.dots)
		if triangle.definition {
			checkDefinitionGlyph(t, triangle)
		}
	}
	// edge4 is a specialization; edge5, a membership, has a diamond and no triangle.
	want := map[string]int{"edge1": 2, "edge2": 2, "edge3": 2, "edge4": 0, "edge6": 2}
	if len(dotted) != len(want) {
		t.Errorf("triangles of %v, want those of %v", dotted, want)
	}
	for edge, dots := range want {
		if dotted[edge] != dots {
			t.Errorf("%s has %d dots, want %d", edge, dotted[edge], dots)
		}
	}
	// The plain typing edge's triangle points down-left from (218.99,-39.06)-(215.5,-45.14) to (208.57,-37.12).
	for _, dot := range []string{
		`<circle fill="#181818" stroke="none" cx="222.77" cy="-41.12" r="1.7"/>`,
		`<circle fill="#181818" stroke="none" cx="219.18" cy="-47.36" r="1.7"/>`,
	} {
		if !bytes.Contains(drawn, []byte(dot)) {
			t.Errorf("the drawing lacks %s", dot)
		}
	}
	if !bytes.Contains(drawn, []byte(`fill="#ff0000" fill-opacity="0.501961" stroke="none"`)) {
		t.Errorf("the translucent edge's dots do not take its opacity:\n%s", drawn)
	}
	again, err := DrawDOTGlyphs(recorded)
	if err != nil || !bytes.Equal(again, drawn) {
		t.Errorf("a second drawing differs: %v", err)
	}
}

// A drawing without a typing edge's triangle is returned as it is, and one that is not well-formed is refused.
func TestDrawDOTGlyphsLeavesOtherDrawings(t *testing.T) {
	for name, svg := range map[string]string{
		"no edge":          `<svg xmlns="http://www.w3.org/2000/svg"><text>definition</text></svg>`,
		"a specialization": `<svg xmlns="http://www.w3.org/2000/svg"><g id="edge1" class="edge"><polygon points="3.5,-10 0,0 -3.5,-10 3.5,-10"/></g></svg>`,
		"a node":           `<svg xmlns="http://www.w3.org/2000/svg"><g id="node1" class="node definition"><polygon points="3.5,-10 0,0 -3.5,-10 3.5,-10"/></g></svg>`,
		"no triangle":      `<svg xmlns="http://www.w3.org/2000/svg"><g id="edge1" class="edge definition"><polygon points="0,0 4,6 0,12 -4,6 0,0"/><polygon points="0,0 x,1 2,2"/></g></svg>`,
		"a point":          `<svg xmlns="http://www.w3.org/2000/svg"><g id="edge1" class="edge definition"><polygon points="1,1 1,1 1,1 1,1"/></g></svg>`,
	} {
		drawn, err := DrawDOTGlyphs([]byte(svg))
		if err != nil || string(drawn) != svg {
			t.Errorf("%s: drawn as %s (%v), want it unchanged", name, drawn, err)
		}
	}
	if drawn, err := DrawDOTGlyphs([]byte(`<svg><g class="edge definition"><polygon points="3.5,-10 0,0 -3.5,-10"></g></svg>`)); err == nil {
		t.Errorf("a drawing that is not well-formed is drawn as %s", drawn)
	}
}

// An open triangle and one whose points start at the tip are dotted alike: the tip is the corner
// opposite the shortest side, whatever the order.
func TestDrawDOTGlyphsFindsTheTipByShape(t *testing.T) {
	for _, points := range []string{"3.5,-10 0,0 -3.5,-10 3.5,-10", "0,0 -3.5,-10 3.5,-10", "-3.5,-10 3.5,-10 0,0 -3.5,-10"} {
		svg := `<svg xmlns="http://www.w3.org/2000/svg"><g id="edge1" class="edge definition"><polygon stroke="black" points="` + points + `"/></g></svg>`
		drawn, err := DrawDOTGlyphs([]byte(svg))
		if err != nil {
			t.Fatalf("%s: %v", points, err)
		}
		triangles := drawnTriangles(t, drawn)
		if len(triangles) != 1 || triangles[0].tip != (Point{}) || triangles[0].base != (Point{X: 0, Y: -10}) {
			t.Fatalf("%s: read as %+v", points, triangles)
		}
		checkDefinitionGlyph(t, triangles[0])
		for _, dot := range []string{`cx="-3.6" cy="-14.3" r="1.7"`, `cx="3.6" cy="-14.3" r="1.7"`} {
			if !bytes.Contains(drawn, []byte(dot)) {
				t.Errorf("%s: the drawing lacks the dot %s:\n%s", points, dot, drawn)
			}
		}
	}
}

// anyDot is an installed Graphviz dot of any version; without one the test skips, or fails where the toolchain is required.
func anyDot(t *testing.T) string {
	t.Helper()
	path, err := layout.Locate()
	if err != nil {
		if os.Getenv("OPENSYSML_REQUIRE_PDF_TOOLCHAIN") != "" {
			t.Fatalf("OPENSYSML_REQUIRE_PDF_TOOLCHAIN is set but %v", err)
		}
		t.Skipf("%v; the drawn glyphs are unchecked", err)
	}
	return path
}

// An installed Graphviz draws every typing edge of the DOT form with a triangle DrawDOTGlyphs dots as the
// figure does, and no other edge so: a tree laid out by dot and a general view drawn along its stated routes.
func TestGraphvizDrawsTheDefinitionGlyph(t *testing.T) {
	dot := anyDot(t)
	membership := render(t, "feature-membership.sysml", "MembershipViews::structure")
	layered, err := membership.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	general := render(t, "general.sysml", "GeneralViews::definitionView")
	routed, err := general.DOTWith(orthoOptions)
	if err != nil {
		t.Fatalf("DOTWith: %v", err)
	}
	for _, drawing := range []struct {
		name      string
		source    string
		rendering *Rendering
	}{{"membership", layered, membership}, {"general", routed, general}} {
		typings := strings.Count(drawing.source, "arrowhead=empty, class=definition")
		want := 0
		for _, edge := range drawing.rendering.Edges {
			if edge.Kind == EdgeTyping {
				want++
			}
		}
		if typings == 0 || typings < want {
			t.Fatalf("%s: the DOT marks %d typing edges of the rendering's %d:\n%s", drawing.name, typings, want, drawing.source)
		}
		engine, flags := DOTLayout(drawing.source)
		if engine == "" {
			engine = "dot"
		}
		command := exec.Command(dot, append(append([]string{"-K" + engine}, flags...), "-Tsvg")...)
		command.Stdin = strings.NewReader(drawing.source)
		var stderr strings.Builder
		command.Stderr = &stderr
		svg, err := command.Output()
		if err != nil {
			t.Fatalf("%s: %s -Tsvg: %v\n%s", drawing.name, engine, err, stderr.String())
		}
		drawn, err := DrawDOTGlyphs(svg)
		if err != nil {
			t.Fatalf("%s: DrawDOTGlyphs: %v", drawing.name, err)
		}
		dotted := 0
		for _, triangle := range drawnTriangles(t, drawn) {
			if !triangle.definition {
				if len(triangle.dots) != 0 {
					t.Errorf("%s: %s is no typing edge and has %d dots", drawing.name, triangle.edge, len(triangle.dots))
				}
				continue
			}
			dotted++
			checkDefinitionGlyph(t, triangle)
		}
		if dotted != typings {
			t.Errorf("%s: %d typing edges are drawn with a triangle, want %d:\n%s", drawing.name, dotted, typings, drawn)
		}
	}
}
