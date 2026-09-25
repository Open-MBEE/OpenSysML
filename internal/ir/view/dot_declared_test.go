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
		if !strings.Contains(source, `"note:0" [`) {
			t.Errorf("%s: the note on an undrawn anchor is not drawn free:\n%s", style, source)
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
	dot, err := rendering.DOT()
	if err != nil {
		t.Fatalf("DOT: %v", err)
	}
	checkDOTSyntax(t, dot)
	if undeclared := dotUndeclaredEnds(t, dot); len(undeclared) != 0 {
		t.Errorf("edge ends with no declaration: %v\n%s", undeclared, dot)
	}
	requireDotAccepts(t, dot)
	// The note in the stated 66x14 box is fitted to it like a node's label.
	idx := slices.IndexFunc(rendering.Notes, func(n Note) bool { return n.Text == "doAcquisition" })
	if idx < 0 {
		t.Fatalf("no note carries %q", "doAcquisition")
	}
	var noteLine string
	for _, line := range strings.Split(dot, "\n") {
		if strings.Contains(line, fmt.Sprintf(`"note:%d" [`, idx)) {
			noteLine = line
		}
	}
	for _, want := range []string{"margin=0", `<font point-size=`, "fixedsize=true"} {
		if !strings.Contains(noteLine, want) {
			t.Errorf("the stated note's line lacks %q: %s\n%s", want, noteLine, dot)
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
