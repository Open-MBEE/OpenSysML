package view

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// A node declaration and an edge line of a DOT source, `"id" [` and `"a" -> "b"`.
var (
	dotDeclLine = regexp.MustCompile(`^\s*"([^"]+)"\s+\[`)
	dotEdgeLine = regexp.MustCompile(`^\s*"([^"]+)"\s+->\s+"([^"]+)"`)
)

// dotUndeclaredEnds is the edge endpoints of a DOT source that no `"id" [` line
// declares a node for: `dot -n` fails on each.
func dotUndeclaredEnds(t *testing.T, source string) []string {
	t.Helper()
	declared := map[string]bool{}
	var ends []string
	for _, line := range strings.Split(source, "\n") {
		if m := dotDeclLine.FindStringSubmatch(line); m != nil {
			declared[m[1]] = true
		}
		if m := dotEdgeLine.FindStringSubmatch(line); m != nil {
			ends = append(ends, m[1], m[2])
		}
	}
	var undeclared []string
	for _, id := range ends {
		if !declared[id] {
			undeclared = append(undeclared, id)
		}
	}
	return undeclared
}

// treeIDs is the ID of every node in the rendering's trees.
func treeIDs(r *Rendering) map[string]bool {
	ids := map[string]bool{}
	var walk func(nodes []*Node)
	walk = func(nodes []*Node) {
		for _, node := range nodes {
			ids[node.ID] = true
			walk(node.Children)
		}
	}
	walk(r.Roots)
	return ids
}

// requireDotAccepts runs `dot -Kneato -n` on a positioned drawing's source when
// Graphviz is installed: it must draw it without a word of complaint, an edge
// end or a size it has no node or room for failing or warning.
func requireDotAccepts(t *testing.T, source string) {
	t.Helper()
	dot, err := exec.LookPath("dot")
	if err != nil {
		return
	}
	cmd := exec.Command(dot, "-Kneato", "-n", "-Tsvg")
	cmd.Stdin = strings.NewReader(source)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("dot -n: %v\nstderr: %s\nsource:\n%s", err, stderr.String(), source)
	}
	if out := stderr.String(); out != "" {
		t.Errorf("dot -n warned: %s\nsource:\n%s", out, source)
	}
}

// Every edge the DOT form draws, and every anchor it draws a note by, ends at a
// node it declares: an end the rendering holds no node for drops the edge with a
// notice, and a note anchored to one is drawn free. A nested action's notes once
// anchored to a node the drawing never held, which `dot -n` refuses.
func TestDOTDeclaresEveryEdgeEnd(t *testing.T) {
	rendering := &Rendering{View: "V", Kind: KindAction,
		Roots: []*Node{
			{ID: "n0", Kind: "action", Name: "own", Geometry: &Geometry{X: 0, Y: 0, Width: 100, Height: 40, HasSize: true}},
			{ID: "n1", Kind: "action", Name: "call", Geometry: &Geometry{X: 150, Y: 0, Width: 100, Height: 40, HasSize: true}},
		},
		Edges: []Edge{{From: "n1", To: "n9", Kind: EdgeFlow}},
		Notes: []Note{
			{Text: "anchored away", Anchor: "n9", X: 300, Y: 0, Width: 60, Height: 20, HasSize: true},
			{Text: "on an undrawn edge", EdgeFrom: "n1", EdgeTo: "n9", X: 300, Y: 40, Width: 80, Height: 20, HasSize: true},
		}}
	for _, style := range []DrawingStyle{StylePilot, StyleCameo} {
		source, err := rendering.DOTWith(Options{Style: style})
		if err != nil {
			t.Fatalf("%s: DOTWith: %v", style, err)
		}
		checkDOTSyntax(t, source)
		if undeclared := dotUndeclaredEnds(t, source); len(undeclared) != 0 {
			t.Errorf("%s: edge ends with no declaration: %v\n%s", style, undeclared, source)
		}
		unwrapped := strings.ReplaceAll(source, "<br/>", " ")
		if !strings.Contains(unwrapped, "anchored away") {
			t.Errorf("%s: the note on an undeclared anchor is not drawn free:\n%s", style, source)
		}
		if !strings.Contains(unwrapped, "on an undrawn edge") {
			t.Errorf("%s: the note on an edge with an undeclared end is not drawn free:\n%s", style, source)
		}
		if strings.Contains(source, `"note:0" ->`) || strings.Contains(source, `"note:1" ->`) {
			t.Errorf("%s: an anchor edge to an undeclared node is drawn:\n%s", style, source)
		}
		if !strings.Contains(source, "// not represented: edge n1->n9 has an end the rendering draws no node for; no edge is drawn") {
			t.Errorf("%s: the dropped edge is not noticed:\n%s", style, source)
		}
		requireDotAccepts(t, source)
	}
}

// A note anchored to a node the drawing declares but omits, or to an edge with
// such an end, is dropped with a notice rather than drawn free: its box is the
// omitted node's to carry. Neither inflates the canvas.
func TestDOTDropsNotesOnOmittedNodes(t *testing.T) {
	rendering := &Rendering{View: "V", Kind: KindInterconnection,
		Roots: []*Node{
			{ID: "n0", Kind: "part", Name: "placed", Geometry: &Geometry{X: 0, Y: 0, Width: 100, Height: 40, HasSize: true}},
			{ID: "n1", Kind: "part", Name: "loose"},
		},
		Edges: []Edge{{From: "n0", To: "n1", Kind: EdgeConnection}},
		Notes: []Note{
			{Text: "stray anchored", Anchor: "n1", X: 0, Y: 0, Width: 60, Height: 20, HasSize: true},
			{Text: "stray edge", EdgeFrom: "n0", EdgeTo: "n1", X: 5000, Y: 0, Width: 80, Height: 20, HasSize: true},
		}}
	source, err := rendering.DOTWith(Options{Style: StyleCameo})
	if err != nil {
		t.Fatalf("DOTWith: %v", err)
	}
	for _, stale := range []string{"stray anchored", "stray edge", "5000"} {
		if strings.Contains(source, stale) {
			t.Errorf("DOT holds %q, which a dropped note stated:\n%s", stale, source)
		}
	}
	for _, want := range []string{
		"note on n1, a node the rendering draws no box for; no note is drawn",
		"note on edge n0->n1, an edge the rendering draws no node for; no note is drawn",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("DOT lacks the notice %q:\n%s", want, source)
		}
	}
	requireDotAccepts(t, source)
}

// A nested action's notes anchor to the node drawn for it, once each: the root
// the recursive lowering builds is discarded, so dressing it anchored them to a
// node no drawing holds, beside the copy on the drawn node.
func TestActionNestedFlowNotesOnce(t *testing.T) {
	rendering := render(t, "nested-notes.sysml", "NestedViews::alignView")
	ids := treeIDs(rendering)
	var value4 *Node
	for _, child := range rendering.Roots[0].Children {
		if child.Name == "value4" {
			value4 = child
		}
	}
	if value4 == nil {
		t.Fatalf("no node for value4; roots: %+v", rendering.Roots[0].Children)
	}
	var found int
	for _, note := range rendering.Notes {
		if note.Text == "these values don't matter" {
			found++
			if note.Anchor != value4.ID {
				t.Errorf("note anchor = %q, want value4's node %q", note.Anchor, value4.ID)
			}
		}
		for _, end := range []string{note.Anchor, note.EdgeFrom, note.EdgeTo} {
			if end != "" && !ids[end] {
				t.Errorf("note %q ends at %q, a node no tree holds", note.Text, end)
			}
		}
	}
	if found != 1 {
		t.Errorf("%d notes carry %q, want one", found, "these values don't matter")
	}
	for _, edge := range rendering.Edges {
		if !ids[edge.From] || !ids[edge.To] {
			t.Errorf("edge %s -> %s has an end no tree holds", edge.From, edge.To)
		}
	}
	// The note in the stated 66x14 box is fitted to it like a node's label.
	idx := slices.IndexFunc(rendering.Notes, func(n Note) bool { return n.Text == "doAcquisition" })
	if idx < 0 {
		t.Fatalf("no note carries %q", "doAcquisition")
	}
	tall := slices.IndexFunc(rendering.Notes, func(n Note) bool { return n.Text == "these values don't matter" })
	for _, style := range []DrawingStyle{StylePilot, StyleCameo} {
		dot, err := rendering.DOTWith(Options{Style: style})
		if err != nil {
			t.Fatalf("%s: DOTWith: %v", style, err)
		}
		checkDOTSyntax(t, dot)
		if undeclared := dotUndeclaredEnds(t, dot); len(undeclared) != 0 {
			t.Errorf("%s: edge ends with no declaration: %v\n%s", style, undeclared, dot)
		}
		requireDotAccepts(t, dot)
		var noteLine, tallLine string
		for _, line := range strings.Split(dot, "\n") {
			if strings.Contains(line, fmt.Sprintf(`"note:%d" [`, idx)) {
				noteLine = line
			}
			if strings.Contains(line, fmt.Sprintf(`"note:%d" [`, tall)) {
				tallLine = line
			}
		}
		for _, want := range []string{"margin=0", `<font point-size=`, "fixedsize=true"} {
			if !strings.Contains(noteLine, want) {
				t.Errorf("%s: the stated note's line lacks %q: %s\n%s", style, want, noteLine, dot)
			}
		}
		if style == StyleCameo {
			if strings.Contains(noteLine, "«comment»") {
				t.Errorf("%s: the one-line note keeps the header: %s", style, noteLine)
			}
			if !strings.Contains(tallLine, "«comment»") {
				t.Errorf("%s: the tall note lost the header: %s", style, tallLine)
			}
		}
	}
	// Mermaid names the same nodes the DOT form declares, no other.
	mermaid := rendering.Mermaid()
	for _, id := range regexp.MustCompile(`\bn\d+\b`).FindAllString(mermaid, -1) {
		if !ids[id] {
			t.Errorf("Mermaid names %s, a node no tree holds:\n%s", id, mermaid)
		}
	}
}
