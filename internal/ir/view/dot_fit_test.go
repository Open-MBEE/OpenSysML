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
// ellipsized. The box itself is never resized.
func TestDOTFitsTheLabelToAStatedBox(t *testing.T) {
	cases := []struct {
		name string
		node *Node
		want string
	}{
		{"room for everything", stated(&Node{ID: "n", Kind: "action", Name: "call", Type: "doTracking"}, 200, 80),
			`label=<<b>call : doTracking</b><br/><font point-size="10"><i>«action»</i></font>>, pos="100,-40!", pin=true, width=2.7777777777777777, height=1.1111111111111112, fixedsize=true];`},
		{"head wraps at the width", stated(&Node{ID: "n", Kind: "action", Name: "Execute Find and Identify Algorithm"}, 100, 100),
			`label=<<b>Execute<br/>Find and<br/>Identify<br/>Algorithm</b><br/><font point-size="10"><i>«action»</i></font>>`},
		{"keyword dropped for want of height", stated(&Node{ID: "n", Kind: "action", Name: "call", Type: "doTracking"}, 200, 24),
			`label=<<b>call : doTracking</b>>`},
		{"detail only while height remains", stated(&Node{ID: "n", Kind: "state", Name: "idle", Detail: "entry, do"}, 200, 40),
			`label=<<b>idle</b><br/><font point-size="10"><i>«state»</i></font>>`},
		{"detail when it fits", stated(&Node{ID: "n", Kind: "state", Name: "idle", Detail: "entry, do"}, 200, 60),
			`label=<<b>idle</b><br/><font point-size="10"><i>«state»</i></font><br/>entry, do>`},
		{"a compartment row shrinks to one line", stated(&Node{ID: "n", Kind: "attribute", Name: "errorReq", Type: "Real"}, 449, 14),
			`label=<<font point-size="11"><b>errorReq : Real</b></font>>, pos="224.5,-7!", pin=true, width=6.236111111111111, height=0.19444444444444445, fixedsize=true];`},
		{"a name too long for the floor is ellipsized", stated(&Node{ID: "n", Kind: "attribute", Name: "'a name that runs on well past the width of the row it is drawn in'"}, 120, 14),
			`label=<<font point-size="8"><b>&#39;a name that runs on…</b></font>>, pos="60,-7!", pin=true, width=1.6666666666666667, height=0.19444444444444445, fixedsize=true];`},
		{"a word wider than the box is broken across lines", stated(&Node{ID: "n", Kind: "action", Name: "Reconfiguration"}, 60, 60),
			`label=<<b>Reconf<br/>igurat<br/>ion</b>>`},
		{"wrapping comes before shrinking", stated(&Node{ID: "n", Kind: "part", Name: "pump", Type: "Pump"}, 60, 40),
			`label=<<b>pump :<br/>Pump</b>>`},
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
// whose wrapped lines stack within the height.
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
		{"abcdefghijklmnopqrstuvwxyz", 40, 14, 8, []string{"abcdef…"}, false},
		{"abcdefghijklmnopqrstuvwxyz", 40, 30, 8, []string{"abcdefg", "hijklmn", "opqrst…"}, false},
	}
	for _, tc := range cases {
		size, lines, fits := dotFitHead(tc.head, tc.width, tc.height)
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
		{"port", stated(&Node{ID: "n", Kind: "port", Name: "cmdIn", Type: "CmdPort"}, 12, 12),
			`"n" [label="", xlabel="cmdIn : CmdPort", pos="6,-6!", pin=true, width=0.16666666666666666, height=0.16666666666666666, fixedsize=true];`},
		{"ref port", stated(&Node{ID: "n", Kind: "ref port", Name: "p"}, 12, 12), `"n" [label="", xlabel="p", pos=`},
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

// A synthesized name is drawn nowhere a symbol stands, but a plain node keeps
// its name in the box: the bit only silences the text beside a symbol.
func TestDOTSynthesizedNameOnAPlainNode(t *testing.T) {
	dot, err := (&Rendering{View: "V", Kind: KindAction, Roots: []*Node{stated(&Node{ID: "n", Kind: "action", Name: "send2", NameSynthesized: true}, 100, 40)}}).DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if want := `label=<<b>send2</b><br/><font point-size="10"><i>«action»</i></font>>`; !strings.Contains(dot, want) {
		t.Errorf("DOT lacks %q:\n%s", want, dot)
	}
}
