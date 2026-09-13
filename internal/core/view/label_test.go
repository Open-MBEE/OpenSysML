package view

import (
	"fmt"
	"strings"
	"testing"
)

// A diagram label leads with the name — the declared type after a colon for a
// typed usage — then the kind in guillemets, then the notes; a node with no
// name leads with its kind and has no keyword line.
func TestLabelLines(t *testing.T) {
	cases := []struct {
		name string
		node *Node
		want []string
	}{
		{"typed usage", &Node{Kind: "part", Name: "pump", Type: "Pump"}, []string{"pump : Pump", "«part»"}},
		{"untyped usage", &Node{Kind: "port", Name: "p"}, []string{"p", "«port»"}},
		{"definition", &Node{Kind: "part def", Name: "Plant::Loop"}, []string{"Plant::Loop", "«part def»"}},
		{"name-less", &Node{Kind: "connect"}, []string{"connect"}},
		{"name-less typed", &Node{Kind: "part", Type: "Pump"}, []string{"part"}},
		{"name-less with note", &Node{Kind: "connect", Detail: "already shown"}, []string{"connect", "already shown"}},
		{"notes", &Node{Kind: "part", Name: "sensor", Type: "Pump", Detail: "already shown as n1"},
			[]string{"sensor : Pump", "«part»", "already shown as n1"}},
		{"state note", &Node{Kind: "state", Name: "off", Detail: "initial, entry"}, []string{"off", "«state»", "initial, entry"}},
	}
	for _, tc := range cases {
		got := labelLines(tc.node)
		if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
			t.Errorf("%s: labelLines = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// The text form keeps the keyword leading, writes the type after a colon and
// the notes in parentheses after it.
func TestTextLabelShape(t *testing.T) {
	rendering := &Rendering{View: "V", Kind: KindInterconnection, Roots: []*Node{
		{ID: "n0", Kind: "part def", Name: "Plant::Loop", Children: []*Node{
			{ID: "n1", Kind: "part", Name: "pump", Type: "Pump"},
			{ID: "n2", Kind: "part", Name: "sensor", Type: "Pump", Detail: "already shown as n1"},
			{ID: "n3", Kind: "port", Name: "p"},
			{ID: "n4", Kind: "connect", Detail: "already shown"},
		}},
	}}
	text := rendering.Text()
	for _, want := range []string{
		"part def Plant::Loop\n",
		"  part pump : Pump\n",
		"  part sensor : Pump (already shown as n1)\n",
		"  port p\n",
		"  connect (already shown)\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "(Pump)") || strings.Contains(text, "«") {
		t.Errorf("text writes the type parenthesised or the kind in guillemets:\n%s", text)
	}
}

// Every Mermaid form carries the same label: the lines joined with `<br>`,
// which a flowchart node, a state and a sequence participant all break at,
// whether or not the renderer draws HTML labels.
func TestMermaidLabelShapePerForm(t *testing.T) {
	roots := []*Node{
		{ID: "n0", Kind: "state def", Name: "Machines::Lamp", Children: []*Node{
			{ID: "n1", Kind: "state", Name: "off", Detail: "initial"},
			{ID: "n2", Kind: "state", Name: "on"},
		}},
		{ID: "n3", Kind: "part", Name: "pump", Type: "Pump"},
	}
	edges := []Edge{{From: "n1", To: "n2", Label: "switch", Kind: EdgeTransition}}
	cases := []struct {
		kind Kind
		want []string
	}{
		{KindInterconnection, []string{
			`subgraph n0 ["Machines::Lamp<br>«state def»"]`,
			`n1["off<br>«state»<br>initial"]`,
			`n3["pump : Pump<br>«part»"]`,
		}},
		{KindState, []string{
			`state "Machines::Lamp<br>«state def»" as n0 {`,
			`state "off<br>«state»<br>initial" as n1`,
			`state "pump : Pump<br>«part»" as n3`,
			`n1 --> n2 : switch`,
		}},
		{KindSequence, []string{
			`participant n0 as Machines::Lamp<br>«state def»`,
			`participant n3 as pump : Pump<br>«part»`,
			`n1->>n2: switch`,
		}},
	}
	for _, tc := range cases {
		rendering := &Rendering{View: "V", Kind: tc.kind, Roots: roots, Edges: edges}
		mermaid := rendering.Mermaid()
		for _, want := range tc.want {
			if !strings.Contains(mermaid, want) {
				t.Errorf("%s Mermaid lacks %q:\n%s", tc.kind, want, mermaid)
			}
		}
		for _, stale := range []string{"part pump", "state off", "(Pump)"} {
			if strings.Contains(mermaid, stale) {
				t.Errorf("%s Mermaid still leads with the keyword, %q:\n%s", tc.kind, stale, mermaid)
			}
		}
	}
}

// A flowchart whose cluster title spans several lines leads with the Mermaid
// frontmatter reserving the extra height, sized by its tallest title; a flowchart
// without such a cluster, a tree, a state or a sequence diagram carries none.
func TestMermaidFrontmatterReservesClusterTitleHeight(t *testing.T) {
	cluster := func(children ...*Node) []*Node {
		return []*Node{{ID: "n0", Kind: "part def", Name: "Plant::Loop", Children: children}}
	}
	leaf := &Node{ID: "n1", Kind: "part", Name: "pump", Type: "Pump"}
	noted := &Node{ID: "n2", Kind: "action", Name: "monitor", Detail: "own flow", Children: []*Node{{ID: "n3", Kind: "initial", Name: "begin"}}}
	frontmatter := func(bottom int) string {
		return fmt.Sprintf("---\nconfig:\n  flowchart:\n    subGraphTitleMargin:\n      bottom: %d\n---\n%%%% V — ", bottom)
	}
	cases := []struct {
		name  string
		kind  Kind
		roots []*Node
		want  string
	}{
		{"two-line cluster title", KindInterconnection, cluster(leaf), frontmatter(24)},
		{"nested three-line title", KindAction, cluster(leaf, noted), frontmatter(48)},
		{"anonymous cluster", KindInterconnection, []*Node{{ID: "n0", Kind: "connect", Children: []*Node{leaf}}}, "%% V — "},
		{"no cluster", KindInterconnection, []*Node{leaf}, "%% V — "},
		{"tree", KindTree, cluster(leaf), "%% V — "},
		{"state", KindState, cluster(leaf), "%% V — "},
		{"sequence", KindSequence, cluster(leaf), "%% V — "},
	}
	for _, tc := range cases {
		rendering := &Rendering{View: "V", Kind: tc.kind, Roots: tc.roots}
		if mermaid := rendering.Mermaid(); !strings.HasPrefix(mermaid, tc.want) {
			t.Errorf("%s: Mermaid starts with %q, want %q", tc.name, mermaid[:min(len(mermaid), len(tc.want))], tc.want)
		}
	}
}
