package view

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// cameoBehaviorDOT is the DOT form, in the Cameo style, of one of the
// positioned behaviour views of cameo-behavior.sysml.
func cameoBehaviorDOT(t *testing.T, view string) (*Rendering, string) {
	t.Helper()
	rendering := render(t, "cameo-behavior.sysml", view)
	source, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOTWith: %v", err)
	}
	checkDOTSyntax(t, source)
	return rendering, source
}

// dotLine is the DOT statement declaring the node or edge, "" when there is none.
func dotLine(source, subject string) string {
	for _, line := range strings.Split(source, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), subject+" [") {
			return line
		}
	}
	return ""
}

// The Cameo drawing of an action view locks the behaviour notation of a
// migrated activity diagram: pins are ports on the action border, object flows
// run pin to pin, synthesized names give way to what the node does, forks and
// joins are bars at their bounds, and the notes hang on dashed anchors.
func TestDOTCameoActionNotation(t *testing.T) {
	_, source := cameoBehaviorDOT(t, "NotationViews::acquireView")
	checkGolden(t, "testdata/cameo-behavior-action.cameo.dot.golden", source)

	// Pins: ports at the action's border, named outside in small type, and
	// never a note-shaped node of their own.
	for port, want := range map[string][]string{
		`"n5.0"`: {`xlabel="mask"`, `fontsize=8`, `pos="90,406!"`, `width=0.16666666666666666`},
		`"n5.1"`: {`xlabel="guide"`, `pos="180,406!"`},
		`"n5.2"`: {`xlabel="image"`, `pos="180,334!"`},
		`"n6.0"`: {`xlabel="image"`, `pos="180,286!"`},
		`"n1.0"`: {`xlabel="result"`, `pos="60,494!"`},
	} {
		line := dotLine(source, port)
		if line == "" {
			t.Fatalf("no port %s:\n%s", port, source)
		}
		for _, attr := range want {
			if !strings.Contains(line, attr) {
				t.Errorf("port %s lacks %s: %s", port, attr, line)
			}
		}
	}
	if n := strings.Count(source, "shape=note"); n != 1 {
		t.Errorf("%d note-shaped nodes, want the one comment:\n%s", n, source)
	}
	// Object flows end at the pins, not at the actions.
	for _, flow := range []string{`"n1.0" -> "n5.0" [style=dashed`, `"n3.0" -> "n5.1" [style=dashed`, `"n5.2" -> "n6.0" [style=dashed`} {
		if !strings.Contains(source, flow) {
			t.Errorf("flow %s not drawn pin to pin:\n%s", flow, source)
		}
	}
	// Synthesized names: the value a value specification binds, the
	// assignment a step makes; never the bare kind, never `own flow`.
	for _, text := range []string{`label=<<b>&#34;SH-0&#34;</b>>`, `label=<<b>true</b>>`, `label=<<b>i := i + 1</b>>`, `label=<<b>sense : Sense</b>>`} {
		if !strings.Contains(source, text) {
			t.Errorf("label %s missing:\n%s", text, source)
		}
	}
	for _, text := range []string{`<b>action</b>`, `own flow`, `>pick<`, `>flag<`, `>step<`} {
		if strings.Contains(source, text) {
			t.Errorf("synthesized name or kind shown as %s:\n%s", text, source)
		}
	}
	// Fork and join: bars at their stated 200×10 bounds with nothing written in
	// or beside them, since their names are synthesized.
	for _, bar := range []string{`"n9"`, `"n10"`} {
		line := dotLine(source, bar)
		for _, attr := range []string{`fillcolor=black`, `label=""`, `width=2.7777777777777777`, `height=0.1388888888888889`, `fixedsize=true`} {
			if !strings.Contains(line, attr) {
				t.Errorf("bar %s lacks %s: %s", bar, attr, line)
			}
		}
		if strings.Contains(line, "xlabel") {
			t.Errorf("bar %s names its synthesized name: %s", bar, line)
		}
	}
	// Decision and merge are diamonds at their bounds, the terminate a
	// bull's-eye, the start a dot.
	for node, want := range map[string][]string{
		`"n11"`: {`shape=diamond`, `pos="250,70!"`},
		`"n12"`: {`shape=diamond`, `pos="250,120!"`},
		`"n13"`: {`shape=doublecircle`, `pos="250,20!"`},
		`"n14"`: {`shape=circle`, `fillcolor=black`, `pos="337.5,522.5!"`, `width=0.20833333333333334`},
	} {
		line := dotLine(source, node)
		for _, attr := range want {
			if !strings.Contains(line, attr) {
				t.Errorf("symbol %s lacks %s: %s", node, attr, line)
			}
		}
	}
	// The note sits at its bounds and hangs on a dashed anchor.
	if line := dotLine(source, `"note:0"`); !strings.Contains(line, `pos="550,295!"`) || !strings.Contains(line, "Acquire a new star") {
		t.Errorf("note misplaced: %s", line)
	}
	if !strings.Contains(source, `"note:0" -> "n5" [style=dashed, arrowhead=none]`) {
		t.Errorf("note anchor not dashed:\n%s", source)
	}
}

// The Cameo drawing of a state view locks the state-machine notation: state
// text with the behaviours' names, one label per transition in Cameo's form,
// every pseudostate as its UML symbol at its bounds, notes on dashed anchors.
func TestDOTCameoStateNotation(t *testing.T) {
	rendering, source := cameoBehaviorDOT(t, "NotationViews::alignmentView")
	checkGolden(t, "testdata/cameo-behavior-state.cameo.dot.golden", source)

	// State text: the name, then `entry / x`, `do / y`, `exit / z` with the
	// behaviour's name, the action's text when it has none; no «state».
	for _, text := range []string{
		`<b>Initializing</b></td></tr><hr/><tr><td align="left">do / Initialize</td>`,
		`<b>Aligning</b></td></tr><hr/><tr><td align="left">entry / prime<br/>do / align<br/>exit / Settle</td>`,
		`<b>Phasing</b></td></tr><hr/><tr><td align="left">entry / Initialize</td>`,
	} {
		if !strings.Contains(source, text) {
			t.Errorf("state text %s missing:\n%s", text, source)
		}
	}
	for _, text := range []string{"«state»", ">do<", "defers", "accept"} {
		if strings.Contains(source, text) {
			t.Errorf("state view writes %q:\n%s", text, source)
		}
	}
	// Transition labels: `trigger [guard] / effect`, once, as `label` with a
	// stated `lp`, never doubled by an `xlabel`.
	for edge, label := range map[string]string{
		`"n1" -> "n2"`:  `label="Done"`,
		`"n2" -> "n11"`: `label="Done [timer > 0]"`,
		`"n2" -> "n1"`:  `label="Abort / retry"`,
		`"n12" -> "n5"`: `label="[timer > 0]"`,
	} {
		line := dotLine(source, edge)
		if !strings.Contains(line, label) || !strings.Contains(line, " lp=") {
			t.Errorf("transition %s lacks %s with a position: %s", edge, label, line)
		}
	}
	for _, line := range strings.Split(source, "\n") {
		if !strings.Contains(line, " -> ") {
			continue
		}
		if n := strings.Count(line, "label="); n > 1 || strings.Contains(line, "xlabel=") {
			t.Errorf("transition labelled twice: %s", line)
		}
	}
	checkLabelsOutsideStates(t, rendering, source)
	// Pseudostates: the initial dot in the machine and in a composite state,
	// the final bull's-eye, choice diamond, junction dot, fork and join bars
	// oriented by their bounds, the history rings lettered H and H*; each at its
	// stated bounds, named beside it when the name is the model's.
	for node, want := range map[string][]string{
		`"n18"`: {`shape=circle`, `fillcolor=black`, `pos="47.5,362.5!"`, `width=0.20833333333333334`},
		`"n19"`: {`shape=circle`, `fillcolor=black`, `pos="567.5,332.5!"`},
		`"n11"`: {`shape=doublecircle`, `pos="360,70!"`, `width=0.2777777777777778`},
		`"n12"`: {`shape=diamond`, `label=""`, `xlabel="which"`, `pos="600,170!"`},
		`"n13"`: {`shape=circle`, `fillcolor=black`, `label=""`, `xlabel="settle"`, `pos="600,102!"`, `width=0.2222222222222222`},
		`"n14"`: {`fillcolor=black`, `label=""`, `xlabel="spread"`, `pos="730,102!"`, `width=0.8333333333333334`, `height=0.1388888888888889`},
		`"n15"`: {`fillcolor=black`, `label=""`, `xlabel="collect"`, `pos="935,90!"`, `width=0.1388888888888889`, `height=1.6666666666666667`},
		`"n16"`: {`shape=circle`, `fillcolor=white`, `label="H"`, `xlabel="resume"`, `pos="870,310!"`},
		`"n17"`: {`shape=circle`, `fillcolor=white`, `label="H*"`, `xlabel="recall"`, `pos="930,310!"`},
	} {
		line := dotLine(source, node)
		if line == "" {
			t.Fatalf("no symbol %s:\n%s", node, source)
		}
		for _, attr := range want {
			if !strings.Contains(line, attr) {
				t.Errorf("symbol %s lacks %s: %s", node, attr, line)
			}
		}
	}
	for _, edge := range []string{`"n18" -> "n1" [pos="e,47,300 47,355`, `"n19" -> "n4" [pos="e,567,280 567,325`, `"n2" -> "n16" [label="Resume"`, `"n16" -> "n4" [pos="e,660,280`, `"n1" -> "n17" [label="Recall"`, `"n17" -> "n5" [pos="e,780,200`} {
		if !strings.Contains(source, edge) {
			t.Errorf("transition %s not drawn:\n%s", edge, source)
		}
	}
	if line := dotLine(source, `"note:0"`); !strings.Contains(line, `pos="375,130!"`) {
		t.Errorf("note misplaced: %s", line)
	}
	if !strings.Contains(source, `"note:0" -> "n2" [style=dashed, arrowhead=none]`) {
		t.Errorf("note anchor not dashed:\n%s", source)
	}
	if strings.Contains(source, "not represented") {
		t.Errorf("positioned view left something undrawn:\n%s", source)
	}
}

// A routed edge keeps both of its stated endpoints whatever arrows it draws:
// the curve starts and ends at the waypoints when an end has no arrow, and an
// arrow's end is stated `s,x,y` or `e,x,y` with the curve stopping short of it.
func TestDOTSplineKeepsBothRouteEnds(t *testing.T) {
	w := &dotWriter{canvas: &Canvas{Width: 200, Height: 100, HasSize: true}}
	route := []Point{{X: 20, Y: 50}, {X: 120, Y: 50}, {X: 120, Y: 90}}
	for _, tc := range []struct {
		tailed, headed bool
		want           string
	}{
		{false, false, "20,50 20,50 120,50 120,50 120,50 120,10 120,10"},
		{false, true, "e,120,10 20,50 20,50 120,50 120,50 120,50 120,20 120,20"},
		{true, false, "s,20,50 30,50 30,50 120,50 120,50 120,50 120,10 120,10"},
		{true, true, "s,20,50 e,120,10 30,50 30,50 120,50 120,50 120,50 120,20 120,20"},
	} {
		if got := w.dotSpline(route, tc.tailed, tc.headed); got != tc.want {
			t.Errorf("dotSpline(tailed=%v, headed=%v) = %q, want %q", tc.tailed, tc.headed, got, tc.want)
		}
	}
	for _, tc := range []struct {
		attrs          []string
		tailed, headed bool
	}{
		{nil, false, true},
		{[]string{"arrowhead=none"}, false, false},
		{[]string{"arrowhead=none", "dir=back", "arrowtail=diamond"}, true, false},
		{[]string{"dir=both"}, true, true},
		{[]string{"dir=both", "arrowtail=none"}, false, true},
	} {
		if got := dotArrowtailed(tc.attrs); got != tc.tailed {
			t.Errorf("dotArrowtailed(%v) = %v", tc.attrs, got)
		}
		if got := dotArrowheaded(tc.attrs); got != tc.headed {
			t.Errorf("dotArrowheaded(%v) = %v", tc.attrs, got)
		}
	}
}

var dotLPAttribute = regexp.MustCompile(`lp="([-0-9.]+),([-0-9.]+)"`)

// checkLabelsOutsideStates fails for a transition label positioned inside a
// state box other than the one enclosing both its ends.
func checkLabelsOutsideStates(t *testing.T, rendering *Rendering, source string) {
	t.Helper()
	height := rendering.Canvas.Height
	var boxes []*Node
	var walk func(nodes []*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			if node.Geometry != nil && node.Geometry.HasSize && len(node.Children) == 0 && !isSymbolKind(node.Kind) {
				boxes = append(boxes, node)
			}
			walk(node.Children)
		}
	}
	walk(rendering.Roots)
	for _, line := range strings.Split(source, "\n") {
		m := dotLPAttribute.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		x, _ := strconv.ParseFloat(m[1], 64)
		y, _ := strconv.ParseFloat(m[2], 64)
		y = height - y
		for _, box := range boxes {
			g := box.Geometry
			if x > g.X && x < g.X+g.Width && y > g.Y && y < g.Y+g.Height {
				t.Errorf("label at (%g,%g) inside %s: %s", x, y, box.Name, line)
			}
		}
	}
}

// graphvizPath is the Graphviz binary OPENSYSML_DOT names; the test skips
// without it.
func graphvizPath(t *testing.T) string {
	t.Helper()
	dot := os.Getenv("OPENSYSML_DOT")
	if dot == "" {
		t.Skip("OPENSYSML_DOT not set")
	}
	return dot
}

// graphvizJSON is Graphviz's JSON rendering of a positioned DOT source.
type graphvizJSON struct {
	Objects []struct {
		GVID int    `json:"_gvid"`
		Name string `json:"name"`
	} `json:"objects"`
	Edges []struct {
		Tail  int          `json:"tail"`
		Head  int          `json:"head"`
		Draw  []graphvizOp `json:"_draw_"`
		HDraw []graphvizOp `json:"_hdraw_"`
	} `json:"edges"`
}

type graphvizOp struct {
	Op     string       `json:"op"`
	Points [][2]float64 `json:"points"`
}

// Every routed edge of the two Cameo-positioned behaviour views is drawn by
// Graphviz from its stated first point to its stated last, the arrowhead's
// tip included: the boxes match their bounds, so nothing is clipped short.
func TestDOTRoutedEdgesEndAtTheirRoutes(t *testing.T) {
	dot := graphvizPath(t)
	const tolerance = 3.0
	for _, view := range []string{"NotationViews::acquireView", "NotationViews::alignmentView"} {
		rendering, source := cameoBehaviorDOT(t, view)
		cmd := exec.Command(dot, "-Kneato", "-n2", "-Tjson")
		cmd.Stdin = strings.NewReader(source)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s: dot: %v\n%s", view, err, stderr.String())
		}
		if stderr.Len() != 0 {
			t.Errorf("%s: dot warned: %s", view, stderr.String())
		}
		var graph graphvizJSON
		if err := json.Unmarshal(out, &graph); err != nil {
			t.Fatalf("%s: dot -Tjson: %v", view, err)
		}
		names := map[int]string{}
		for _, object := range graph.Objects {
			names[object.GVID] = object.Name
		}
		type ends struct{ from, to Point }
		drawn := map[[2]string]ends{}
		for _, edge := range graph.Edges {
			var points [][2]float64
			for _, op := range edge.Draw {
				if op.Op == "b" || op.Op == "B" {
					points = op.Points
				}
			}
			if len(points) == 0 {
				continue
			}
			base := Point{X: points[len(points)-1][0], Y: points[len(points)-1][1]}
			e := ends{from: Point{X: points[0][0], Y: points[0][1]}, to: base}
			// The arrowhead's tip is its vertex farthest along the spline's
			// final direction.
			heading := base
			for i := len(points) - 2; i >= 0 && heading == base; i-- {
				heading = Point{X: points[i][0], Y: points[i][1]}
			}
			dx, dy := base.X-heading.X, base.Y-heading.Y
			farthest := 0.0
			for _, op := range edge.HDraw {
				for _, p := range op.Points {
					if along := (p[0]-base.X)*dx + (p[1]-base.Y)*dy; along > farthest {
						farthest, e.to = along, Point{X: p[0], Y: p[1]}
					}
				}
			}
			drawn[[2]string{names[edge.Tail], names[edge.Head]}] = e
		}
		routed := 0
		for _, edge := range rendering.Edges {
			if len(edge.Route) < 2 {
				continue
			}
			from, to := edge.From, edge.To
			if edge.FromPort != "" {
				from = edge.FromPort
			}
			if edge.ToPort != "" {
				to = edge.ToPort
			}
			e, ok := drawn[[2]string{from, to}]
			if !ok {
				t.Errorf("%s: edge %s -> %s not drawn", view, from, to)
				continue
			}
			routed++
			flip := func(p Point) Point { return Point{X: p.X, Y: rendering.Canvas.Height - p.Y} }
			start, end := flip(edge.Route[0]), flip(edge.Route[len(edge.Route)-1])
			if d := distance(e.from, start); d > tolerance {
				t.Errorf("%s: %s -> %s starts %.1f px from its route's %v, at %v", view, from, to, d, start, e.from)
			}
			if d := distance(e.to, end); d > tolerance {
				t.Errorf("%s: %s -> %s ends %.1f px from its route's %v, at %v", view, from, to, d, end, e.to)
			}
		}
		if routed == 0 {
			t.Fatalf("%s: no routed edges", view)
		}
	}
}

func distance(a, b Point) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}
