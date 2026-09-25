package view

import (
	"strings"
	"testing"
)

// stated is a node pinned in a box of the given size, the geometry a Layout states.
func stated(node *Node, width, height float64) *Node {
	node.Geometry = &Geometry{X: 0, Y: 0, Width: width, Height: height, HasSize: true}
	return node
}

// A label composed for a stated box fits it: the head word-wraps at the box's
// width, keeps 14pt while the wrapped lines fit the height and shrinks a point at
// a time to 8pt when they do not; the keyword and detail lines follow only while
// height remains; a head that overruns at 8pt is cut to the lines that fit and
// ellipsized. The box itself is never resized, and its margin is zeroed so the
// whole box is the label's, as the fit assumes.
func TestDOTFitsTheLabelToAStatedBox(t *testing.T) {
	cases := []struct {
		name string
		node *Node
		want string
	}{
		{"room for everything", stated(&Node{ID: "n", Kind: "action", Name: "call", Type: "doTracking"}, 200, 80),
			`label=<<b>call : doTracking</b><br/><font point-size="10"><i>«action»</i></font>>, margin=0, pos="100,-40!", pin=true, width=2.7777777777777777, height=1.1111111111111112, fixedsize=true];`},
		{"head wraps at the width", stated(&Node{ID: "n", Kind: "action", Name: "Execute Find and Identify Algorithm"}, 100, 100),
			`label=<<b>Execute<br/>Find and<br/>Identify<br/>Algorithm</b><br/><font point-size="10"><i>«action»</i></font>>`},
		{"keyword dropped for want of height", stated(&Node{ID: "n", Kind: "action", Name: "call", Type: "doTracking"}, 200, 24),
			`label=<<b>call : doTracking</b>>`},
		{"detail only while height remains", stated(&Node{ID: "n", Kind: "state", Name: "idle", Detail: "entry, do"}, 200, 40),
			`label=<<b>idle</b><br/><font point-size="10"><i>«state»</i></font>>`},
		{"detail when it fits", stated(&Node{ID: "n", Kind: "state", Name: "idle", Detail: "entry, do"}, 200, 60),
			`label=<<b>idle</b><br/><font point-size="10"><i>«state»</i></font><br/>entry, do>`},
		{"a compartment row shrinks to one line", stated(&Node{ID: "n", Kind: "attribute", Name: "errorReq", Type: "Real"}, 449, 14),
			`label=<<font point-size="11"><b>errorReq : Real</b></font>>, margin=0, pos="224.5,-7!", pin=true, width=6.236111111111111, height=0.19444444444444445, fixedsize=true];`},
		{"a name too long for the floor is ellipsized", stated(&Node{ID: "n", Kind: "attribute", Name: "'a name that runs on well past the width of the row it is drawn in'"}, 120, 14),
			`label=<<font point-size="8"><b>&#39;a name that runs on…</b></font>>, margin=0, pos="60,-7!", pin=true, width=1.6666666666666667, height=0.19444444444444445, fixedsize=true];`},
		{"a word wider than the box is broken across lines", stated(&Node{ID: "n", Kind: "action", Name: "Reconfiguration"}, 60, 60),
			`label=<<b>Reconf<br/>igurat<br/>ion</b>>`},
		{"shrinking to keep a word whole comes before breaking it", stated(&Node{ID: "n", Kind: "part", Name: "EventStream"}, 77, 32),
			`label=<<font point-size="10"><b>EventStream</b></font><br/><font point-size="7"><i>«part»</i></font>>`},
		{"wrapping comes before shrinking", stated(&Node{ID: "n", Kind: "part", Name: "pump", Type: "Pump"}, 60, 40),
			`label=<<b>pump<br/>: Pump</b>>`},
		// Twenty bold 14pt glyphs measure 184.8pt: the whole width is the label's, no margin off it.
		{"a head filling the width keeps its size", stated(&Node{ID: "n", Kind: "action", Name: "doughboundhoundpound"}, 185, 20),
			`label=<<b>doughboundhoundpound</b>>, margin=0, pos="92.5,-10!"`},
		{"a head a point over the width shrinks", stated(&Node{ID: "n", Kind: "action", Name: "doughboundhoundpound"}, 184, 20),
			`label=<<font point-size="13"><b>doughboundhoundpound</b></font>>, margin=0, pos="92,-10!"`},
	}
	for _, tc := range cases {
		dot, err := (&Rendering{View: "V", Kind: KindAction, Roots: []*Node{tc.node}}).DOT()
		if err != nil {
			t.Fatalf("%s: DOT: %v", tc.name, err)
		}
		checkDOTSyntax(t, dot)
		if !strings.Contains(dot, tc.want) {
			t.Errorf("%s: DOT lacks %q:\n%s", tc.name, tc.want, dot)
		}
		if !strings.Contains(dot, "fixedsize=true") {
			t.Errorf("%s: the stated box is not fixed:\n%s", tc.name, dot)
		}
		if !strings.Contains(dot, ", margin=0, ") {
			t.Errorf("%s: the stated box keeps Graphviz's margin, which the fit did not allow for:\n%s", tc.name, dot)
		}
	}
	// A node with no stated size keeps the label-fitted box and the plain label.
	loose := &Node{ID: "n", Kind: "action", Name: "Execute Find and Identify Algorithm", Geometry: &Geometry{X: 0, Y: 0}}
	dot, err := (&Rendering{View: "V", Kind: KindAction, Roots: []*Node{loose}}).DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if want := `label=<<b>Execute Find and Identify Algorithm</b><br/><font point-size="10"><i>«action»</i></font>>, pos="`; !strings.Contains(dot, want) || strings.Contains(dot, "fixedsize") {
		t.Errorf("unsized node's DOT lacks %q or fixes its size:\n%s", want, dot)
	}
}

// The fitting estimates with the writer's glyph metrics: dotFitHead wraps a
// head at the runes a bold line of the size holds and picks the largest size
// whose wrapped lines stack within the height, with every word whole where a
// size down to the floor allows it.
func TestDOTFitHead(t *testing.T) {
	cases := []struct {
		head          string
		width, height float64
		size          float64
		lines         []string
		fits          bool
	}{
		{"call : doTracking", 200, 80, 14, []string{"call : doTracking"}, true},
		{"call : doTracking", 100, 80, 14, []string{"call :", "doTracking"}, true},
		{"call : doTracking", 100, 20, 8, []string{"call : doTracking"}, true},
		{"errorReq : Real", 449, 14, 11, []string{"errorReq : Real"}, true},
		{"EventStream", 77, 32, 10, []string{"EventStream"}, true},
		{"Reconfiguration", 60, 60, 14, []string{"Reconf", "igurat", "ion"}, true},
		{"abcdefghijklmnopqrstuvwxyz", 40, 14, 8, []string{"abcdef…"}, false},
		{"abcdefghijklmnopqrstuvwxyz", 40, 30, 8, []string{"abcdefg", "hijklmn", "opqrst…"}, false},
	}
	for _, tc := range cases {
		size, lines, fits := dotFitHead(tc.head, tc.width, tc.height, dotFontSize)
		if size != tc.size || fits != tc.fits || strings.Join(lines, "|") != strings.Join(tc.lines, "|") {
			t.Errorf("dotFitHead(%q, %v, %v) = %v, %q, %v; want %v, %q, %v", tc.head, tc.width, tc.height, size, lines, fits, tc.size, tc.lines, tc.fits)
		}
	}
	for _, tc := range []struct {
		text   string
		across int
		want   []string
	}{
		{"a b c", 3, []string{"a b", "c"}},
		{"a b c", 1, []string{"a", "b", "c"}},
		{"abcdef gh", 4, []string{"abcd", "ef", "gh"}},
		{"  spaced   out  ", 10, []string{"spaced out"}},
		{"", 5, []string{""}},
		{"call : Pump", 6, []string{"call", ": Pump"}},
		{": Pump", 6, []string{": Pump"}},
		{"a :", 1, []string{"a", ":"}},
	} {
		if got := dotWrap(tc.text, tc.across); strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Errorf("dotWrap(%q, %d) = %q, want %q", tc.text, tc.across, got, tc.want)
		}
	}
}

// A symbol kind in a stated box is drawn as its notation with no text inside:
// decision, merge and choice a diamond, fork and join a filled bar, initial a
// filled dot, final and a terminate action the double ring, a port its square.
// A name the rendering has is set beside the symbol; a synthesized one is not
// drawn. Without a stated box the kinds keep their labelled shapes.
func TestDOTSymbolsInStatedBoxes(t *testing.T) {
	cases := []struct {
		name string
		node *Node
		want string
	}{
		{"decision", stated(&Node{ID: "n", Kind: "decision", Name: "decide2", NameSynthesized: true}, 24, 12),
			`"n" [shape=diamond, label="", pos="12,-6!", pin=true, width=0.3333333333333333, height=0.16666666666666666, fixedsize=true];`},
		{"merge", stated(&Node{ID: "n", Kind: "merge"}, 24, 12), `"n" [shape=diamond, label="", pos=`},
		{"choice", stated(&Node{ID: "n", Kind: "choice", Name: "which"}, 24, 12), `"n" [shape=diamond, label="", xlabel="which", pos=`},
		{"fork", stated(&Node{ID: "n", Kind: "fork", Name: "fork", NameSynthesized: true}, 120, 6),
			`"n" [fillcolor=black, label="", pos="60,-3!", pin=true, width=1.6666666666666667, height=0.08333333333333333, fixedsize=true];`},
		{"join", stated(&Node{ID: "n", Kind: "join", Name: "sync"}, 120, 6), `"n" [fillcolor=black, label="", xlabel="sync", pos=`},
		{"initial", stated(&Node{ID: "n", Kind: "initial", Name: "start", NameSynthesized: true}, 20, 20),
			`"n" [shape=circle, fillcolor=black, label="", pos="10,-10!", pin=true, width=0.2777777777777778, height=0.2777777777777778, fixedsize=true];`},
		{"final", stated(&Node{ID: "n", Kind: "final", Name: "final", NameSynthesized: true}, 20, 20),
			`"n" [shape=doublecircle, fillcolor=black, label="", pos=`},
		{"terminate action", stated(&Node{ID: "n", Kind: terminateKind, Name: "final", NameSynthesized: true}, 20, 20),
			`"n" [shape=doublecircle, fillcolor=black, label="", pos=`},
		{"named final", stated(&Node{ID: "n", Kind: "final", Name: "done"}, 20, 20),
			`"n" [shape=doublecircle, fillcolor=black, label="", xlabel="done", pos=`},
		{"junction", stated(&Node{ID: "n", Kind: "junction", Name: "j"}, 16, 16),
			`"n" [shape=circle, fillcolor=black, label="", xlabel="j", pos=`},
		{"port", stated(&Node{ID: "n", Kind: "port", Name: "cmdIn", Type: "CmdPort"}, 12, 12),
			`"n" [label="", xlabel="cmdIn : CmdPort", pos="6,-6!", pin=true, width=0.16666666666666666, height=0.16666666666666666, fixedsize=true];`},
		{"ref port", stated(&Node{ID: "n", Kind: "ref port", Name: "p"}, 12, 12), `"n" [label="", xlabel="p", pos=`},
		{"typed anonymous port", stated(&Node{ID: "n", Kind: "port", Type: "DataPort"}, 12, 12), `"n" [label="", xlabel=": DataPort", pos=`},
		{"typed synthesized-name port", stated(&Node{ID: "n", Kind: "port", Name: "port2", NameSynthesized: true, Type: "DataPort"}, 12, 12),
			`"n" [label="", xlabel=": DataPort", pos=`},
		{"untyped anonymous port", stated(&Node{ID: "n", Kind: "port"}, 12, 12), `"n" [label="", pos=`},
	}
	for _, tc := range cases {
		dot, err := (&Rendering{View: "V", Kind: KindAction, Roots: []*Node{tc.node}}).DOT()
		if err != nil {
			t.Fatalf("%s: DOT: %v", tc.name, err)
		}
		checkDOTSyntax(t, dot)
		if !strings.Contains(dot, tc.want) {
			t.Errorf("%s: DOT lacks %q:\n%s", tc.name, tc.want, dot)
		}
		if strings.Contains(dot, "«") {
			t.Errorf("%s: a symbol carries a keyword line:\n%s", tc.name, dot)
		}
	}
	// A port under a palette keeps its family fill, so the square is coloured.
	dot, err := (&Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{stated(&Node{ID: "n", Kind: "port", Name: "p"}, 12, 12)}}).DOTWith(Options{Palette: PaletteOkabeIto})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if want := `"n" [fillcolor="#99D8C7", color="#009E73", penwidth=1, label="", xlabel="p", pos=`; !strings.Contains(dot, want) {
		t.Errorf("palette port DOT lacks %q:\n%s", want, dot)
	}
	// A port def is a definition, not a symbol; a symbol kind with no stated size
	// is labelled as before.
	for _, tc := range []struct {
		name string
		node *Node
		want string
	}{
		{"port def", stated(&Node{ID: "n", Kind: "port def", Name: "CmdPort"}, 120, 40),
			`label=<<b>CmdPort</b><br/><font point-size="10"><i>«port def»</i></font>>`},
		{"unsized decision", &Node{ID: "n", Kind: "decision", Name: "decide", Geometry: &Geometry{X: 5, Y: 5}},
			`"n" [label=<<b>decide</b><br/><font point-size="10"><i>«decision»</i></font>>, pos=`},
		{"unsized terminate action", &Node{ID: "n", Kind: terminateKind, Name: "final"},
			`"n" [style="rounded,filled", label=<<b>final</b><br/><font point-size="10"><i>«terminate action»</i></font>>];`},
		{"unsized named initial", &Node{ID: "n", Kind: "initial", Name: "begin"},
			`"n" [shape=circle, label=<<b>begin</b><br/><font point-size="10"><i>«initial»</i></font>>];`},
		{"unsized initial with a synthesized name", &Node{ID: "n", Kind: "initial", Name: "start", NameSynthesized: true},
			`"n" [shape=circle, fillcolor=black, label="", width=0.2];`},
	} {
		dot, err := (&Rendering{View: "V", Kind: KindAction, Roots: []*Node{tc.node}}).DOT()
		if err != nil {
			t.Fatalf("%s: DOT: %v", tc.name, err)
		}
		if !strings.Contains(dot, tc.want) {
			t.Errorf("%s: DOT lacks %q:\n%s", tc.name, tc.want, dot)
		}
	}
}

// A synthesized name is drawn nowhere: a plain node with one is drawn as its
// source drew it, unnamed, so a typed one reads ": Type" and an untyped one its kind.
func TestDOTSynthesizedNameOnAPlainNode(t *testing.T) {
	for _, tc := range []struct {
		name string
		node *Node
		want string
	}{
		{"typed", &Node{ID: "n", Kind: "action", Name: "call", Type: "doTracking", NameSynthesized: true},
			`label=<<b>: doTracking</b><br/><font point-size="10"><i>«action»</i></font>>`},
		{"untyped", &Node{ID: "n", Kind: "action", Name: "send2", NameSynthesized: true}, `label=<<b>action</b>>`},
	} {
		dot, err := (&Rendering{View: "V", Kind: KindAction, Roots: []*Node{stated(tc.node, 140, 40)}}).DOT()
		if err != nil {
			t.Fatalf("%s: DOT: %v", tc.name, err)
		}
		if !strings.Contains(dot, tc.want) {
			t.Errorf("%s: DOT lacks %q:\n%s", tc.name, tc.want, dot)
		}
	}
}

// A stated box is written before the stated boxes it encloses, whatever order
// the rendering lists them in, so Graphviz paints the enclosed ones on top;
// boxes that do not nest, and boxes not stated (here the strip's), keep the
// rendering's order.
func TestDOTWritesAnEnclosingBoxFirst(t *testing.T) {
	inner := &Node{ID: "inner", Kind: "part", Name: "sensor", Geometry: &Geometry{X: 20, Y: 20, Width: 60, Height: 30, HasSize: true}}
	outer := &Node{ID: "outer", Kind: "part", Name: "bench", Geometry: &Geometry{X: 0, Y: 0, Width: 300, Height: 200, HasSize: true}}
	beside := &Node{ID: "beside", Kind: "part", Name: "rack", Geometry: &Geometry{X: 400, Y: 0, Width: 60, Height: 30, HasSize: true}}
	loose := &Node{ID: "loose", Kind: "part", Name: "spare"}
	dot, err := (&Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{inner, beside, loose, outer}}).DOTWith(Options{Unplaced: UnplacedStrip})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	last := -1
	for _, id := range []string{"outer", "inner", "beside", "loose"} {
		at := strings.Index(dot, "\n  "+dotQuote(id)+" [")
		if at < 0 {
			t.Fatalf("DOT lacks node %q:\n%s", id, dot)
		}
		if at < last {
			t.Errorf("node %q written out of draw order:\n%s", id, dot)
		}
		last = at
	}
}

// A stated box that holds other stated boxes keeps its title in the strip above
// the topmost of them, as the notation's header compartment does: the label is
// fitted to that strip's height and set at the top, so no member covers it. A
// box holding none, or with a name too long for the strip, is fitted as before,
// to the whole box, or cut to the strip and ellipsized; a strip too thin for one
// line at the floor sets the head outside the box instead. A stated box drawn as
// a cluster round its children is fitted the same way.
func TestDOTHeadsAnEnclosingBoxAboveItsMembers(t *testing.T) {
	// Cameo's IBD: a 300×200 part with two members from 40px down, a third box beside it.
	member := func(id string, x, y float64) *Node {
		return &Node{ID: id, Kind: "part", Name: id, Geometry: &Geometry{X: x, Y: y, Width: 80, Height: 30, HasSize: true}}
	}
	outer := &Node{ID: "outer", Kind: "part", Name: "'summit Installation'", Type: "'Summit Installation'",
		Geometry: &Geometry{X: 0, Y: 0, Width: 300, Height: 200, HasSize: true}}
	beside := &Node{ID: "beside", Kind: "part", Name: "rack", Geometry: &Geometry{X: 400, Y: 0, Width: 300, Height: 200, HasSize: true}}
	dot, err := (&Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{outer, member("computer", 20, 60), member("sensor", 120, 40), beside}}).DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	for _, want := range []string{
		// The 40px strip holds the head wrapped to two 14pt lines (33.6px), not the keyword line too; the label sits at the top.
		`"outer" [style="rounded,filled", label=<<b>&#39;summit Installation&#39; : &#39;Summit<br/>Installation&#39;</b>>, margin=0, labelloc=t, pos="150,-100!", pin=true, width=4.166666666666667, height=2.7777777777777777, fixedsize=true];`,
		// A box holding nothing is fitted to the whole of it and centred; the members keep their stated boxes.
		`"beside" [style="rounded,filled", label=<<b>rack</b><br/><font point-size="10"><i>«part»</i></font>>, margin=0, pos="550,-100!"`,
		`"computer" [style="rounded,filled", label=<<b>computer</b><br/><font point-size="10"><i>«part»</i></font>>, margin=0, pos="60,-75!", pin=true, width=1.1111111111111112, height=0.4166666666666667, fixedsize=true];`,
	} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	// A node the Layout only places, its box sized to its label, is no member: the title is fitted to the whole box.
	placed := &Node{ID: "loose", Kind: "action", Name: "spare", Geometry: &Geometry{X: 20, Y: 20}}
	dot, err = (&Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{outer, placed}}).DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if want := `"outer" [style="rounded,filled", label=<<b>&#39;summit Installation&#39; : &#39;Summit<br/>Installation&#39;</b><br/><font point-size="10"><i>«part»</i></font>>, margin=0, pos="150,-100!"`; !strings.Contains(dot, want) {
		t.Errorf("DOT lacks %q:\n%s", want, dot)
	}
	// A member 10px below the top leaves the title one 8pt line, cut to the width and ellipsized.
	narrow := &Node{ID: "outer", Kind: "part", Name: outer.Name, Type: outer.Type, Geometry: &Geometry{X: 0, Y: 0, Width: 150, Height: 200, HasSize: true}}
	dot, err = (&Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{narrow, member("computer", 20, 10)}}).DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if want := `"outer" [style="rounded,filled", label=<<font point-size="8"><b>&#39;summit Installation&#39;…</b></font>>, margin=0, labelloc=t, pos="75,-100!"`; !strings.Contains(dot, want) {
		t.Errorf("DOT lacks %q:\n%s", want, dot)
	}
	// A member 9px below the top leaves no room for a line even at the floor: the head is set outside.
	dot, err = (&Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{narrow, member("computer", 20, 9)}}).DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if want := `"outer" [style="rounded,filled", label="", xlabel="'summit Installation' : 'Summit Installation'", pos="75,-100!"`; !strings.Contains(dot, want) {
		t.Errorf("DOT lacks %q:\n%s", want, dot)
	}
	// A cluster's label is fitted to its stated box the same way: the title wrapped
	// to the 40px strip above its child, the keyword line left out.
	framed := &Node{ID: "outer", Kind: "part", Name: outer.Name, Type: outer.Type, Geometry: outer.Geometry, Children: []*Node{member("computer", 20, 40)}}
	dot, err = (&Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{framed}}).DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if want := "subgraph \"cluster_outer\" {\n    label=<<b>&#39;summit Installation&#39; : &#39;Summit<br/>Installation&#39;</b>>;\n"; !strings.Contains(dot, want) {
		t.Errorf("DOT lacks %q:\n%s", want, dot)
	}
	// A cluster's label has no outside to go to: a strip thinner than a line still gets one line at the floor.
	framed.Children[0].Geometry.Y = 4
	dot, err = (&Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{framed}}).DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if want := "subgraph \"cluster_outer\" {\n    label=<<font point-size=\"8\"><b>&#39;summit Installation&#39; : &#39;Summit Installation&#39;</b></font>>;\n"; !strings.Contains(dot, want) {
		t.Errorf("DOT lacks %q:\n%s", want, dot)
	}
}

// A stated box too short for one line at the floor, or too narrow for one glyph,
// holds no text: its head is set outside as `xlabel`, as a symbol's is, and a
// box with nothing but its kind to show is left bare.
func TestDOTSetsATinyBoxsHeadOutside(t *testing.T) {
	for _, tc := range []struct {
		name string
		node *Node
		want string
	}{
		{"short", &Node{ID: "n", Kind: "action", Name: "tick", Geometry: &Geometry{X: 0, Y: 0, Width: 20, Height: 6, HasSize: true}},
			`"n" [style="rounded,filled", label="", xlabel="tick", pos="10,-3!", pin=true, width=0.2777777777777778, height=0.08333333333333333, fixedsize=true];`},
		{"narrow", &Node{ID: "n", Kind: "part", Type: "Pump", Geometry: &Geometry{X: 0, Y: 0, Width: 4, Height: 40, HasSize: true}},
			`"n" [style="rounded,filled", label="", xlabel=": Pump", pos="2,-20!"`},
		{"bare", &Node{ID: "n", Kind: "action", Geometry: &Geometry{X: 0, Y: 0, Width: 20, Height: 6, HasSize: true}},
			`"n" [style="rounded,filled", label="", pos="10,-3!"`},
		{"one line", &Node{ID: "n", Kind: "action", Name: "tick", Geometry: &Geometry{X: 0, Y: 0, Width: 30, Height: 10, HasSize: true}},
			`"n" [style="rounded,filled", label=<<font point-size="8"><b>tick</b></font>>, margin=0, pos="15,-5!"`},
	} {
		dot, err := (&Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{tc.node}}).DOT()
		if err != nil {
			t.Fatalf("%s: DOT: %v", tc.name, err)
		}
		checkDOTSyntax(t, dot)
		if !strings.Contains(dot, tc.want) {
			t.Errorf("%s: DOT lacks %q:\n%s", tc.name, tc.want, dot)
		}
	}
}

// dotFitText fits each entry wrapped separately, so a line boundary is kept:
// two one-rune lines stay two lines where one wrap would join them, and a word
// one entry must break fails the whole-word pass for all.
func TestDOTFitTextWrapsEachLine(t *testing.T) {
	size, lines, fits := dotFitText([]string{"a", "b"}, dotGlyphEm, 100, 100, dotFontSize)
	if size != dotFontSize || !fits || strings.Join(lines, "|") != "a|b" {
		t.Errorf("dotFitText(a, b) = %v, %q, %v; want 14, a|b, true", size, lines, fits)
	}
	// "wordier" is seven runes: under 30pt of width it breaks even at the floor's
	// six-rune line, so the broken pass applies to both entries.
	size, lines, fits = dotFitText([]string{"wordier", "line"}, dotGlyphEm, 30, 100, dotFontSize)
	if size != 14 || !fits || strings.Join(lines, "|") != "wor|die|r|lin|e" {
		t.Errorf("dotFitText(wordier, line) = %v, %q, %v; want 14, wor|die|r|lin|e, true", size, lines, fits)
	}
}

// A note's label is its text; in a stated box it is composed to fit as a node's
// is, in plain glyphs at the label size or shrunk — `margin=0` so the whole box
// is the label's — and a box holding no line even at the floor sets the text
// outside as `xlabel`. The Cameo «comment» head rides above the fitted text.
func TestDOTNoteLabelFitsAStatedBox(t *testing.T) {
	box := func(width, height float64) *nodeBox {
		return &nodeBox{high: Point{X: width, Y: height}, stated: true}
	}
	pilot := &dotWriter{labels: labeller{}, skin: skinOf(StylePilot)}
	for _, tc := range []struct {
		name string
		note Note
		box  *nodeBox
		want string
	}{
		{"unstated", Note{Text: "call"}, nil, `label="call"`},
		{"unstated box", Note{Text: "call"}, &nodeBox{high: Point{X: 200, Y: 40}}, `label="call"`},
		{"stated holds 14pt", Note{Text: "call"}, box(200, 40), `margin=0, label=<call>`},
		{"stated shrinks", Note{Text: "ok"}, box(66, 14), `margin=0, label=<<font point-size="11">ok</font>>`},
		{"stated holds no line", Note{Text: "call"}, box(200, 4), `label="", xlabel="call"`},
	} {
		got := strings.Join(pilot.dotNoteLabel(tc.note, tc.box), ", ")
		if got != tc.want {
			t.Errorf("%s: dotNoteLabel = %q, want %q", tc.name, got, tc.want)
		}
	}
	cameo := &dotWriter{labels: labeller{skin: skinOf(StyleCameo)}, skin: skinOf(StyleCameo)}
	got := strings.Join(cameo.dotNoteLabel(Note{Text: "ok"}, box(200, 40)), ", ")
	if want := `margin=0, label=<<font point-size="9">«comment»</font><br/>ok>`; got != want {
		t.Errorf("cameo: dotNoteLabel = %q, want %q", got, want)
	}
	if got := strings.Join(cameo.dotNoteLabel(Note{Text: "ok"}, nil), ", "); got != `label=<<font point-size="9">«comment»</font><br/>ok>` {
		t.Errorf("cameo unstated: dotNoteLabel = %q", got)
	}
}
