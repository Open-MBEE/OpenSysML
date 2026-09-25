package view

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A note stated in a nested view's body is drawn in that view alone: the
// rendering of the enclosing view holds no `note:` box for it, while the nested
// view's own rendering draws it free.
func TestNotesScopedToTheViewStatingThem(t *testing.T) {
	outer := render(t, "positioned-views.sysml", "PositionedViews::outerView")
	dot, err := outer.DOT()
	if err != nil {
		t.Fatalf("outer DOT: %v", err)
	}
	if strings.Contains(dot, `"note:`) {
		t.Errorf("outer rendering draws a note the nested view stated:\n%s", dot)
	}
	nested := render(t, "positioned-views.sysml", "Scoped::Housing::detail")
	dot, err = nested.DOT()
	if err != nil {
		t.Fatalf("nested DOT: %v", err)
	}
	if !strings.Contains(dot, `"note:0"`) {
		t.Errorf("nested view's own rendering draws no note box:\n%s", dot)
	}
}

// An anchored note whose node a partial positioned drawing omits draws no box:
// the note is dropped with a notice, as the edges at the node are.
func TestAnchoredNoteOnAnOmittedNodeDrawsNoBox(t *testing.T) {
	rendering := render(t, "positioned-views.sysml", "PositionedViews::partialView")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if strings.Contains(dot, `"note:`) {
		t.Errorf("a note is drawn for the omitted member:\n%s", dot)
	}
	if !strings.Contains(dot, "no note is drawn") {
		t.Errorf("DOT reports no dropped note:\n%s", dot)
	}
}

// An exposed element the containment tree already draws below another exposed
// element is not appended as a root of its own: member is drawn once, under
// Whole, in every form the rendering feeds.
func TestExposedMemberIsNoSecondRoot(t *testing.T) {
	rendering := render(t, "positioned-views.sysml", "PositionedViews::dupView")
	if len(rendering.Roots) != 1 {
		t.Fatalf("roots = %d, want the one owner; member is its child", len(rendering.Roots))
	}
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if got := strings.Count(dot, "member"); got != 1 {
		t.Errorf("member is declared %d times, want once:\n%s", got, dot)
	}
}

// An exposed element contained only in suppressed containers' trees stands as
// a root itself: a tree that cuts its walk short still draws it, once. The
// depth bound is lowered so the chain need not nest to the real limit.
func TestExposedDescendantsRestoresOrphanedElement(t *testing.T) {
	r, idx := loadFixtures(t, "deep-tree.sysml")
	r.treeDepthBound = 2
	rendering, err := r.Render(lookup(t, idx, "ChainViews::deepView"))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(rendering.Roots) != 2 {
		t.Fatalf("roots = %d, want a's surviving tree and d as roots:\n%+v", len(rendering.Roots), rendering.Roots)
	}
	drawn := 0
	var count func(nodes []*Node)
	count = func(nodes []*Node) {
		for _, node := range nodes {
			if strings.HasSuffix(node.Name, "::d") || node.Name == "d" {
				drawn++
			}
			count(node.Children)
		}
	}
	count(rendering.Roots)
	if drawn != 1 {
		t.Errorf("d is drawn %d times, want once", drawn)
	}
}

// A positioned or cameo drawing heads a node by the minimal suffix of its
// qualified name that distinguishes it: scattered roots of unrelated
// namespaces by their endings, not the whole name.
func TestPositionedDrawingLabelsSimply(t *testing.T) {
	rendering := render(t, "positioned-views.sysml", "PositionedViews::cameoView")
	dot, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{"<b>A::X</b>", "<b>B::X</b>", "<b>Y</b>"} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks a simple head %s:\n%s", want, dot)
		}
	}
	for _, stale := range []string{"Named::A::X", "Named::B::X", "Elsewhere::Y"} {
		if strings.Contains(dot, stale) {
			t.Errorf("DOT still heads %s in full:\n%s", stale, dot)
		}
	}
}

// A node a drawing omits for want of a place does not qualify another's name:
// the one X drawn heads as X alone. Drawn in a strip instead, both X's keep
// their distinguishing suffixes.
func TestPositionedDrawingLabelsIgnoreOmittedNodes(t *testing.T) {
	rendering := render(t, "positioned-views.sysml", "PositionedViews::cameoPartialView")
	dot, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	if !strings.Contains(dot, "<b>X</b>") {
		t.Errorf("DOT lacks the unqualified head <b>X</b>:\n%s", dot)
	}
	if strings.Contains(dot, "A::X") {
		t.Errorf("DOT still qualifies the drawn X:\n%s", dot)
	}
	dot, err = rendering.DOTWith(Options{Style: StyleCameo, Unplaced: UnplacedStrip})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{"<b>A::X</b>", "<b>B::X</b>"} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks a simple head %s:\n%s", want, dot)
		}
	}
}

// The Cameo frame names the first root the drawing declares, not the first
// exposed: an unplaced first root is skipped, and the header takes the drawn
// one's simple name.
func TestCameoFrameNamesFirstDrawnRoot(t *testing.T) {
	rendering := render(t, "positioned-views.sysml", "PositionedViews::cameoUnplacedFirstView")
	dot, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	header := dot
	if i := strings.Index(dot, "label=<<b>"); i >= 0 {
		header = dot[i : strings.IndexByte(dot[i:], '\n')+i]
	}
	if !strings.Contains(header, "[Block] X [") {
		t.Errorf("frame header does not name the drawn root X:\n%s", header)
	}
	if strings.Contains(header, "B::X") {
		t.Errorf("frame header names the omitted root:\n%s", header)
	}
}

// A child drawn inside its owner's box is a compartment row of it, so the
// positioned DOT tree writes no containment edge — the row already nests the
// member. A child boxed outside the owner's box keeps its edge, and an
// unpositioned tree keeps them all. The Mermaid and PlantUML trees draw no
// compartments, so their edges stand whatever the geometry.
func TestCompartmentRowDrawsNoContainmentEdge(t *testing.T) {
	rendering := render(t, "positioned-views.sysml", "PositionedViews::compartmentView")
	owner := findNode(t, rendering.Roots, "Compartment::Owner")
	leg := findNode(t, rendering.Roots, "leg")
	var rows []*Node
	for _, child := range owner.Children {
		if child == leg {
			continue
		}
		rows = append(rows, child)
	}
	if len(rows) != 2 {
		t.Fatalf("compartment rows = %d, want a and b", len(rows))
	}
	for _, options := range []Options{{}, {Style: StyleCameo}} {
		dot, err := rendering.DOTWith(options)
		if err != nil {
			t.Fatalf("DOT(%+v): %v", options, err)
		}
		for _, row := range rows {
			edge := dotQuote(owner.ID) + " -> " + dotQuote(row.ID)
			if strings.Contains(dot, edge) {
				t.Errorf("DOT(%+v) draws an edge to compartment row %s:\n%s", options, row.Name, dot)
			}
		}
		if edge := dotQuote(owner.ID) + " -> " + dotQuote(leg.ID); !strings.Contains(dot, edge) {
			t.Errorf("DOT(%+v) drops the edge to leg, boxed outside Owner's box:\n%s", options, dot)
		}
		if got := strings.Count(dot, "a : Real"); got != 1 {
			t.Errorf("DOT(%+v) labels a %d times, want once:\n%s", options, got, dot)
		}
	}

	rendering = render(t, "positioned-views.sysml", "PositionedViews::unplacedCompartmentView")
	owner = findNode(t, rendering.Roots, "Compartment::Owner")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("unpositioned DOT: %v", err)
	}
	for _, child := range owner.Children {
		edge := dotQuote(owner.ID) + " -> " + dotQuote(child.ID)
		if !strings.Contains(dot, edge) {
			t.Errorf("unpositioned DOT lacks %s:\n%s", edge, dot)
		}
	}
	if mermaid := rendering.Mermaid(); !strings.Contains(mermaid, owner.ID+" --- "+owner.Children[0].ID) {
		t.Errorf("Mermaid lacks a containment edge; it draws no compartments:\n%s", mermaid)
	}
	plantuml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	if !strings.Contains(plantuml, owner.ID+" -- "+owner.Children[0].ID) {
		t.Errorf("PlantUML lacks a containment edge; it draws no compartments:\n%s", plantuml)
	}
}

// A note whose stated box encloses a node's is written before the nodes, so
// Graphviz paints it behind them — the Cameo text box used as a group frame —
// while a note enclosing nothing stays after, on top.
func TestEnclosingNotePaintsBehindTheNodes(t *testing.T) {
	rendering := render(t, "positioned-views.sysml", "PositionedViews::noteFrameView")
	outer := findNode(t, rendering.Roots, "Compartment::Outer")
	dot, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	frame, node, small := strings.Index(dot, `"note:0"`), strings.Index(dot, `"`+outer.ID+`" [`), strings.Index(dot, `"note:1"`)
	if frame < 0 || node < 0 || small < 0 {
		t.Fatalf("missing a node or note box: frame %d, node %d, small %d:\n%s", frame, node, small, dot)
	}
	if frame > node {
		t.Errorf("the enclosing note is written after the node it frames:\n%s", dot)
	}
	if small < node {
		t.Errorf("the small note is written before the nodes, where it would hide under them:\n%s", dot)
	}
	line := func(at int) string {
		end := strings.IndexByte(dot[at:], '\n')
		return dot[at : at+end]
	}
	if !strings.Contains(line(frame), "labelloc=t") {
		t.Errorf("the frame note's caption is not set at its top:\n%s", line(frame))
	}
	if strings.Contains(line(small), "labelloc") {
		t.Errorf("the small note is captioned at its top:\n%s", line(small))
	}
}

// A quoted name's escapes decode in the label: `\n` a line break, `\'` a
// quote, `\\` one backslash — in every form, the quotes kept.
func TestQuotedNameEscapesDecodeInLabels(t *testing.T) {
	rendering := render(t, "positioned-views.sysml", "PositionedViews::escapedView")
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	for _, want := range []string{"First Line<br/>Second Line", "It&#39;s", `back\slash`} {
		if !strings.Contains(dot, want) {
			t.Errorf("DOT lacks %q:\n%s", want, dot)
		}
	}
	if mermaid := rendering.Mermaid(); !strings.Contains(mermaid, "First Line<br>Second Line") {
		t.Errorf("Mermaid lacks a <br> break:\n%s", mermaid)
	}
	plantuml, err := rendering.PlantUML()
	if err != nil {
		t.Fatalf("PlantUML: %v", err)
	}
	if !strings.Contains(plantuml, `First Line\nSecond Line`) {
		t.Errorf("PlantUML lacks a \\n break:\n%s", plantuml)
	}
	path, err := exec.LookPath("dot")
	if err != nil {
		t.Skip("no dot installed; label text asserted but Graphviz is unchecked")
	}
	file := filepath.Join(t.TempDir(), "escaped.dot")
	if err := os.WriteFile(file, []byte(dot), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr strings.Builder
	cmd := exec.Command(path, "-Tsvg", file, "-o", file+".svg")
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("dot -Tsvg: %v, stderr:\n%s", err, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("dot -Tsvg warns:\n%s", stderr.String())
	}
}
