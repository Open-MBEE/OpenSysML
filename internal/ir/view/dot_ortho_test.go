package view

import (
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/ir/view/layout"
)

// The orthogonal DOT form's goldens: every drawing a port ends an edge in,
// the issue's nested parts, a ported toaster and the pinned actions of a flow.
var orthoFixtures = []struct {
	name, file, view string
	pins             bool // an edge ends at a pin
}{
	{"ortho-site", "ortho-site.sysml", "Ctx::v", true},
	{"interconnection-ports", "interconnection-ports.sysml", "ToasterViews::toasterView", true},
	{"action", "action.sysml", "FlowViews::driveView", true},
	// Ports drawn as nodes of their own, under a Layout that folds engine without placing it.
	{"layout-collapsed-links", "layout-collapsed.sysml", "FoldedViews::foldOnlyLinks", false},
}

// orthoOptions ask for the orthogonal routing with every edge label shown.
var orthoOptions = Options{Lines: LinesOrtho, Show: Shown{ElementEdgeLabels: true}}

// pinnedGraphviz is the Graphviz version the layered goldens are recorded with.
const pinnedGraphviz = "16.1.0"

// Without a Graphviz dot the orthogonal form lays the drawing out on the
// grid, deterministically, and says so; it is still routed, so every edge
// ends on its port square along axis-parallel segments.
func TestDOTOrthoGridGoldens(t *testing.T) {
	t.Setenv(layout.EnvVar, "no-such-dot")
	for _, fixture := range orthoFixtures {
		got, err := render(t, fixture.file, fixture.view).DOTWith(orthoOptions)
		if err != nil {
			t.Fatalf("%s: %v", fixture.view, err)
		}
		checkGolden(t, filepath.Join("testdata", fixture.name+".ortho-grid.dot.golden"), got)
		want := "// not represented: lines ortho laid out on a grid, not layered: no Graphviz dot at OPENSYSML_DOT=no-such-dot\n"
		if !strings.Contains(got, want) {
			t.Errorf("%s does not notice the grid:\n%s", fixture.view, got)
		}
		checkOrthoDOT(t, fixture.view, got, fixture.pins)
	}
}

// With the pinned Graphviz the orthogonal form lays the drawing out layered,
// by dot, and routes it itself; the goldens are recorded against that one
// version, so another is refused where the toolchain is required.
func TestDOTOrthoLayeredWithInstalledDot(t *testing.T) {
	dot := installedDot(t)
	t.Setenv(layout.EnvVar, dot)
	for _, fixture := range orthoFixtures {
		got, err := render(t, fixture.file, fixture.view).DOTWith(orthoOptions)
		if err != nil {
			t.Fatalf("%s: %v", fixture.view, err)
		}
		checkGolden(t, filepath.Join("testdata", fixture.name+".ortho.dot.golden"), got)
		if strings.Contains(got, "not represented: lines ortho") {
			t.Errorf("%s is noticed, not laid out:\n%s", fixture.view, got)
		}
		checkOrthoDOT(t, fixture.view, got, fixture.pins)
		drawWithGraphviz(t, dot, fixture.name, "-n2", got)
	}
}

// installedDot is the Graphviz dot the layered tests run: the one
// OPENSYSML_DOT names, else the one on PATH, at the pinned version. Without
// one the test skips, unless the toolchain is required, when it fails.
func installedDot(t *testing.T) string {
	t.Helper()
	required := os.Getenv("OPENSYSML_REQUIRE_PDF_TOOLCHAIN") != ""
	path, err := layout.Locate()
	if err != nil {
		if required {
			t.Fatalf("OPENSYSML_REQUIRE_PDF_TOOLCHAIN is set but %v", err)
		}
		t.Skipf("%v; the layered goldens are unchecked", err)
	}
	version, err := layout.Version(path)
	if err != nil {
		t.Fatal(err)
	}
	if version != pinnedGraphviz {
		if required {
			t.Fatalf("%s is Graphviz %s; the goldens are recorded with %s, which scripts/download-doc-pdf-toolchain.sh provisions", path, version, pinnedGraphviz)
		}
		t.Skipf("%s is Graphviz %s, not the pinned %s; the layered goldens are unchecked", path, version, pinnedGraphviz)
	}
	return path
}

// drawWithGraphviz draws the DOT as its header says, neato under the flag, and fails
// on anything Graphviz has to say about it: an edge it had to reroute, say.
func drawWithGraphviz(t *testing.T, dot, name, flag, source string) {
	t.Helper()
	engine, flags := DOTLayout(source)
	if engine != "neato" || len(flags) != 1 || flags[0] != flag {
		t.Fatalf("%s: layout header names %s %v, not neato %s", name, engine, flags, flag)
	}
	file := filepath.Join(t.TempDir(), name+".dot")
	if err := os.WriteFile(file, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd := exec.Command(dot, "-K"+engine, flags[0], "-Tsvg", file, "-o", file+".svg")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s: %s %s -Tsvg: %v, stderr:\n%s", name, engine, flag, err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("%s: %s %s -Tsvg warns:\n%s", name, engine, flag, stderr.String())
	}
}

var (
	orthoNodeLine = regexp.MustCompile(`^\s*"([^"]+)" \[.*width=([0-9.]+), height=([0-9.]+), fixedsize=true, pos="([-0-9.]+),([-0-9.]+)!"`)
	orthoBBAttr   = regexp.MustCompile(`bb="([-0-9.]+),([-0-9.]+),([-0-9.]+),([-0-9.]+)"`)
	orthoPinLine  = regexp.MustCompile(`^\s*"([^"]+)" \[shape=box, (?:style="rounded,filled", )?label="[^"]*", xlabel=.*?width=([0-9.]+), height=([0-9.]+), fixedsize=true, pos="([-0-9.]+),([-0-9.]+)!"`)
	orthoEdgeLine = regexp.MustCompile(`^\s*"([^"]+)" -> "([^"]+)" \[(.*)\];$`)
	orthoPosAttr  = regexp.MustCompile(`pos="([^"]*)"`)
	orthoNodeID   = regexp.MustCompile(`^\s*"([^"]+)" \[`)
	orthoPinned   = regexp.MustCompile(`pos="([-0-9.]+),([-0-9.]+)!"`)
	orthoSize     = regexp.MustCompile(`width=([0-9.]+), height=([0-9.]+)`)
)

// checkAnchorsClear checks that no cluster's sizeless anchor is pinned on a
// sized node's box: Graphviz then routes no orthogonal edge of the drawing.
func checkAnchorsClear(t *testing.T, view, got string) {
	t.Helper()
	boxes, anchors := map[string]nodeBox{}, map[string]Point{}
	for _, line := range strings.Split(got, "\n") {
		id, pos, size := orthoNodeID.FindStringSubmatch(line), orthoPinned.FindStringSubmatch(line), orthoSize.FindStringSubmatch(line)
		if id == nil || pos == nil || size == nil {
			continue
		}
		x, _ := strconv.ParseFloat(pos[1], 64)
		y, _ := strconv.ParseFloat(pos[2], 64)
		w, _ := strconv.ParseFloat(size[1], 64)
		h, _ := strconv.ParseFloat(size[2], 64)
		if w == 0 && h == 0 {
			anchors[id[1]] = Point{X: x, Y: y}
			continue
		}
		boxes[id[1]] = nodeBox{low: Point{X: x - w*36, Y: y - h*36}, high: Point{X: x + w*36, Y: y + h*36}}
	}
	if len(anchors) == 0 {
		t.Errorf("%s pins no anchor:\n%s", view, got)
	}
	for id, anchor := range anchors {
		for node, box := range boxes {
			if anchor.X >= box.low.X && anchor.X <= box.high.X && anchor.Y >= box.low.Y && anchor.Y <= box.high.Y {
				t.Errorf("%s: the anchor %s at %v is on node %s, %v to %v", view, id, anchor, node, box.low, box.high)
			}
		}
	}
}

// Graphviz routes a drawing the model places, and no right angle once two nodes
// touch: the anchor is pinned off the decision diamond at the cluster's centre.
func TestDOTOrthoPlacedAnchorClearOfNodes(t *testing.T) {
	const view = "ChargingViews::placed"
	got, err := render(t, "ortho-placed.sysml", view).DOTWith(orthoOptions)
	if err != nil {
		t.Fatal(err)
	}
	checkGolden(t, filepath.Join("testdata", "ortho-placed.ortho.dot.golden"), got)
	checkAnchorsClear(t, view, got)
	drawWithGraphviz(t, installedDot(t), "ortho-placed", "-n", got)
}

// checkOrthoDOT checks a routed orthogonal drawing: Graphviz is told to draw
// where the writer put things, every node sits inside the clusters around it
// (a pin straddles its owner's border), every edge at a pin is routed with a
// plain label on its route, each route is axis-parallel and ends inside the
// pin squares it joins, and only an edge left unrouted keeps an external label.
func checkOrthoDOT(t *testing.T, view, got string, pins bool) {
	t.Helper()
	if !strings.Contains(got, "// layout: neato -n2\n") || !strings.Contains(got, "layout=neato, inputscale=72, dpi=72") {
		t.Errorf("%s is not drawn where it was laid out:\n%s", view, got)
	}
	pinBoxes := map[string]nodeBox{}
	var clusters []nodeBox
	routed, pinned := 0, 0
	for _, line := range strings.Split(got, "\n") {
		switch {
		case strings.HasPrefix(strings.TrimSpace(line), "subgraph "):
			clusters = append(clusters, nodeBox{})
		case strings.TrimSpace(line) == "}" && len(clusters) > 0:
			clusters = clusters[:len(clusters)-1]
		case len(clusters) > 0 && orthoBBAttr.MatchString(line):
			m := orthoBBAttr.FindStringSubmatch(line)
			x1, _ := strconv.ParseFloat(m[1], 64)
			y1, _ := strconv.ParseFloat(m[2], 64)
			x2, _ := strconv.ParseFloat(m[3], 64)
			y2, _ := strconv.ParseFloat(m[4], 64)
			clusters[len(clusters)-1] = nodeBox{low: Point{X: x1, Y: y1}, high: Point{X: x2, Y: y2}}
		}
		m := orthoNodeLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		w, _ := strconv.ParseFloat(m[2], 64)
		h, _ := strconv.ParseFloat(m[3], 64)
		x, _ := strconv.ParseFloat(m[4], 64)
		y, _ := strconv.ParseFloat(m[5], 64)
		w, h = w*72, h*72
		box := nodeBox{low: Point{X: x - w/2, Y: y - h/2}, high: Point{X: x + w/2, Y: y + h/2}}
		if orthoPinLine.MatchString(line) {
			pinBoxes[m[1]] = box
			continue
		}
		for _, cluster := range clusters {
			if cluster.low == cluster.high {
				continue
			}
			if !cluster.holds(box.low) || !cluster.holds(box.high) {
				t.Errorf("%s: %s %v lies outside its cluster %v: %s", view, m[1], box, cluster, line)
			}
		}
	}
	checkAnchorsClear(t, view, got)
	for _, line := range strings.Split(got, "\n") {
		m := orthoEdgeLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		from, to, attrs := m[1], m[2], m[3]
		pos := orthoPosAttr.FindStringSubmatch(attrs)
		if strings.Contains(attrs, "xlabel=") {
			if pos != nil {
				t.Errorf("%s: a routed edge carries an external label: %s", view, line)
			}
			continue
		}
		if strings.Contains(attrs, "label=") && pos != nil && !strings.Contains(attrs, "lp=") {
			t.Errorf("%s: a routed edge's label is not placed: %s", view, line)
		}
		_, fromPin := pinBoxes[from]
		_, toPin := pinBoxes[to]
		if fromPin || toPin {
			pinned++
			if pos == nil {
				t.Errorf("%s: an edge at a pin is left for Graphviz to route: %s", view, line)
				continue
			}
		}
		if pos == nil {
			continue
		}
		routed++
		points, start, end := dotPosPoints(t, pos[1])
		for i := 1; i < len(points); i++ {
			if points[i].X != points[i-1].X && points[i].Y != points[i-1].Y {
				t.Errorf("%s: route segment %v -> %v is not axis-parallel: %s", view, points[i-1], points[i], line)
			}
		}
		if box, ok := pinBoxes[from]; ok && !box.holds(start) {
			t.Errorf("%s: route starts at %v, outside pin %s %v: %s", view, start, from, box, line)
		}
		if box, ok := pinBoxes[to]; ok && !box.holds(end) {
			t.Errorf("%s: route ends at %v, outside pin %s %v: %s", view, end, to, box, line)
		}
	}
	if routed == 0 || pins && pinned == 0 || !pins && pinned > 0 {
		t.Errorf("%s: %d edge(s) at a pin, %d routed; the fixture proves nothing:\n%s", view, pinned, routed, got)
	}
}

// dotPosPoints reads an edge's pos spline back: its control points, and where
// it starts and ends, the arrow tips (`s,x,y` and `e,x,y`) when it has them.
func dotPosPoints(t *testing.T, pos string) (points []Point, start, end Point) {
	t.Helper()
	var arrowStart, arrowEnd *Point
	for _, field := range strings.Fields(pos) {
		marker := ""
		if len(field) > 2 && field[1] == ',' && (field[0] == 's' || field[0] == 'e') {
			marker, field = field[:1], field[2:]
		}
		xy := strings.Split(field, ",")
		if len(xy) != 2 {
			t.Fatalf("pos point %q", field)
		}
		x, errX := strconv.ParseFloat(xy[0], 64)
		y, errY := strconv.ParseFloat(xy[1], 64)
		if errX != nil || errY != nil {
			t.Fatalf("pos point %q", field)
		}
		p := Point{X: x, Y: y}
		switch marker {
		case "s":
			arrowStart = &p
		case "e":
			arrowEnd = &p
		default:
			points = append(points, p)
		}
	}
	start, end = points[0], points[len(points)-1]
	if arrowStart != nil {
		start = *arrowStart
	}
	if arrowEnd != nil {
		end = *arrowEnd
	}
	return points, start, end
}

// The orthogonal form keeps the node names of an unpositioned drawing: the
// layout is the writer's own, not a Layout's, so names are not shortened. A
// notation symbol a positioned drawing draws as its glyph carries its name
// as an external label. A Layout that only folds a node still folds it.
func TestDOTOrthoKeepsNodeLabels(t *testing.T) {
	t.Setenv(layout.EnvVar, "no-such-dot")
	for _, fixture := range orthoFixtures {
		rendering := render(t, fixture.file, fixture.view)
		splines, err := rendering.DOTWith(Options{Lines: LinesSplines, Show: orthoOptions.Show})
		if err != nil {
			t.Fatal(err)
		}
		ortho, err := rendering.DOTWith(orthoOptions)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range dotBoldNames(splines) {
			plain := html.UnescapeString(strings.TrimSuffix(strings.TrimPrefix(name, "<b>"), "</b>"))
			if !strings.Contains(ortho, name) && !strings.Contains(ortho, "xlabel="+dotQuote(plain)) {
				t.Errorf("%s: ortho drops the name %s the default routing writes:\n%s", fixture.view, name, ortho)
			}
		}
		if folded := strings.Count(splines, `comment="collapsed"`); strings.Count(ortho, `comment="collapsed"`) != folded || fixture.name == "layout-collapsed-links" && folded == 0 {
			t.Errorf("%s: the default routing folds %d node(s), ortho %d:\n%s", fixture.view, folded, strings.Count(ortho, `comment="collapsed"`), ortho)
		}
	}
}

var dotBoldName = regexp.MustCompile(`<b>[^<]*</b>`)

func dotBoldNames(dot string) []string { return dotBoldName.FindAllString(dot, -1) }

// A drawing the model positions, a tree and a drawing with no edge are left
// as they were: Graphviz lays them out and routes them, as for every routing.
func TestDOTOrthoLeavesGraphvizItsOwn(t *testing.T) {
	t.Setenv(layout.EnvVar, "no-such-dot")
	for _, fixture := range []struct{ file, view string }{
		{"tree.sysml", "VehicleViews::vehicleView"},
		{"positioned-views.sysml", "PositionedViews::outerView"},
	} {
		rendering := render(t, fixture.file, fixture.view)
		arranged, notices, err := rendering.arranged(orthoOptions, "")
		if err != nil {
			t.Fatal(err)
		}
		if arranged != rendering || notices != nil {
			t.Errorf("%s is arranged (notices %q); the writer should leave it to Graphviz", fixture.view, notices)
		}
		got, err := rendering.DOTWith(orthoOptions)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(got, "splines=ortho") || strings.Contains(got, "not represented: lines ortho") {
			t.Errorf("%s with lines ortho:\n%s", fixture.view, got)
		}
	}
}

// A note placed beside the node at a cluster's centre is a Graphviz node too:
// the anchor is pinned clear of both.
func TestDOTOrthoPlacedAnchorClearOfNotes(t *testing.T) {
	const view = "V"
	place := func(x, y float64) *Geometry { return &Geometry{X: x, Y: y, Width: 80, Height: 40, HasSize: true} }
	rendering := &Rendering{View: view, Kind: KindInterconnection,
		Roots: []*Node{{ID: "c", Name: "c", Children: []*Node{
			{ID: "a", Name: "a", Geometry: place(0, 0)},
			{ID: "b", Name: "b", Geometry: place(200, 100)},
			{ID: "d", Name: "d", Geometry: place(400, 200)},
		}}},
		Edges: []Edge{{From: "a", To: "b"}, {From: "b", To: "d"}},
		Notes: []Note{{Text: "n", X: 180, Y: 30, Width: 120, Height: 66, HasSize: true}}}
	got, err := rendering.DOTWith(orthoOptions)
	if err != nil {
		t.Fatal(err)
	}
	checkAnchorsClear(t, view, got)
	drawWithGraphviz(t, installedDot(t), "ortho-note", "-n", got)
}

// Notes closer to a cluster's one node than twice the clearance leave no point
// that far from all of them: the anchor is pinned in the gap, not on the node.
func TestDOTOrthoPlacedAnchorInNarrowGap(t *testing.T) {
	const view = "V"
	place := func(x, y float64) *Geometry { return &Geometry{X: x, Y: y, Width: 80, Height: 40, HasSize: true} }
	note := func(x, y, width, height float64) Note {
		return Note{Text: "n", X: x, Y: y, Width: width, Height: height, HasSize: true}
	}
	rendering := &Rendering{View: view, Kind: KindInterconnection,
		Roots: []*Node{
			{ID: "c", Name: "c", Children: []*Node{{ID: "b", Name: "b", Geometry: place(200, 100)}}},
			{ID: "e", Name: "e", Geometry: place(500, 100)},
		},
		Edges: []Edge{{From: "b", To: "e"}},
		Notes: []Note{note(134, 90, 60, 60), note(286, 90, 60, 60), note(200, 64, 80, 30), note(200, 146, 80, 30)}}
	got, err := rendering.DOTWith(orthoOptions)
	if err != nil {
		t.Fatal(err)
	}
	checkAnchorsClear(t, view, got)
	drawWithGraphviz(t, installedDot(t), "ortho-gap", "-n", got)
}
