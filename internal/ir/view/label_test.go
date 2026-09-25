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
		{"name-less typed", &Node{Kind: "part", Type: "Pump"}, []string{": Pump", "«part»"}},
		{"synthesized name", &Node{Kind: "action", Name: "call", Type: "P::doTracking", NameSynthesized: true}, []string{": doTracking", "«action»"}},
		{"synthesized name, untyped", &Node{Kind: "action", Name: "stamp2", NameSynthesized: true}, []string{"action"}},
		{"name-less with note", &Node{Kind: "connect", Detail: "already shown"}, []string{"connect", "already shown"}},
		{"notes", &Node{Kind: "part", Name: "sensor", Type: "Pump", Detail: "already shown as n1"},
			[]string{"sensor : Pump", "«part»", "already shown as n1"}},
		{"state note", &Node{Kind: "state", Name: "off", Detail: "initial, entry"}, []string{"off", "«state»", "initial, entry"}},
	}
	for _, tc := range cases {
		got := (labeller{}).lines(tc.node)
		if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
			t.Errorf("%s: labelLines = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A rendering's roots are headed under the namespace they share: a lone root
// by its own name, several by their names below the longest common qualifier,
// roots of unrelated namespaces in full; a quoted name holding `::` is one
// name, and a type is headed by its own name, or as written when unreadable.
func TestLabelsHeadRootsUnderTheirSharedNamespace(t *testing.T) {
	cases := []struct {
		name  string
		roots []*Node
		want  []string
	}{
		{"lone root", []*Node{{Kind: "part def", Name: "Plant::Loop"}}, []string{"Loop"}},
		{"shared namespace", []*Node{{Kind: "part def", Name: "Systems::Radio"}, {Kind: "part def", Name: "Systems::Braking::Brake"}},
			[]string{"Radio", "Braking::Brake"}},
		{"unrelated namespaces", []*Node{{Kind: "part def", Name: "Plant::Loop"}, {Kind: "part def", Name: "Ctrl::Unit"}},
			[]string{"Plant::Loop", "Ctrl::Unit"}},
		{"unqualified root", []*Node{{Kind: "part def", Name: "Loop"}, {Kind: "part def", Name: "Plant::Pump"}}, []string{"Loop", "Plant::Pump"}},
		{"quoted names", []*Node{{Kind: "part", Name: "'Sep::Pkg'::'x::y'"}, {Kind: "part", Name: "'Sep::Pkg'::'a b'"}}, []string{"'x::y'", "'a b'"}},
		{"anonymous root", []*Node{{Kind: "connect"}, {Kind: "part def", Name: "Plant::Loop"}}, []string{"connect", "Loop"}},
		{"typed root", []*Node{{Kind: "part", Name: "TMT::Segments::tcs", Type: "TMT::Systems::TCS"}}, []string{"tcs : TCS"}},
		{"typings", []*Node{{Kind: "part", Name: "Rig::base", Type: "Frame::Mount, ~Ports::Cart"}, {Kind: "part", Name: "Rig::root", Type: "$::Spelled::Mount"}},
			[]string{"base : Mount, ~Cart", "root : Mount"}},
		{"unreadable type", []*Node{{Kind: "part", Name: "Rig::t", Type: "T<U>"}}, []string{"t : T<U>"}},
	}
	for _, tc := range cases {
		labels := labelsOf(tc.roots, false)
		var got []string
		for _, root := range tc.roots {
			got = append(got, labels.head(root))
		}
		if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
			t.Errorf("%s: heads = %q, want %q", tc.name, got, tc.want)
		}
	}
	labels := labelsOf([]*Node{{Kind: "part def", Name: "Plant::Loop"}}, false)
	if got := labels.head(&Node{Kind: "part", Name: "Other::pump", Type: "Plant::Pumps::Pump"}); got != "Other::pump : Pump" {
		t.Errorf("a node named outside the roots' namespace is headed %q, want it whole", got)
	}
}

// A node whose qualified name continues that of another drawn node is headed
// by its name below that owner — the nearest one drawn — whether the owner is
// a root or a child named under its parent; a node whose owner is not drawn
// keeps its name under the shared namespace, and a nested view's exposed
// elements, named in full, are owners in their own right.
func TestLabelsHeadMembersUnderTheirDrawnOwner(t *testing.T) {
	roots := []*Node{
		{Kind: "part def", Name: "TMT::Budget::'K-Mirror Offset'", Children: []*Node{
			{Kind: "attribute", Name: "errorReq", Type: "ScalarValues::Real"},
			{Kind: "part", Name: "stage", Children: []*Node{{Kind: "port", Name: "inlet"}}},
		}},
		{Kind: "attribute", Name: "TMT::Budget::'K-Mirror Offset'::errorReq", Type: "ScalarValues::Real"},
		{Kind: "part", Name: "TMT::Budget::'K-Mirror Offset'::'interpolation Error'", Type: "TMT::Budget::'Interpolation Error'"},
		{Kind: "port", Name: "TMT::Budget::'K-Mirror Offset'::stage::inlet"},
		{Kind: "attribute", Name: "TMT::Budget::'Shear Plate'::errorCBE"},
		{Kind: "attribute", Name: "TMT::Budget::'K-Mirror Offset'::'interpolation Error'::deep::errorMargin"},
		{Kind: "view", Name: "TMT::Budget::Views::details", Children: []*Node{
			{Kind: "part def", Name: "TMT::Budget::Lens"},
			{Kind: "attribute", Name: "TMT::Budget::Lens::focal"},
		}},
	}
	labels := labelsOf(roots, false)
	want := []string{
		"'K-Mirror Offset'",
		"errorReq : Real",
		"'interpolation Error' : 'Interpolation Error'",
		"inlet",
		"'Shear Plate'::errorCBE",
		"deep::errorMargin",
		"Views::details",
	}
	var got []string
	for _, root := range roots {
		got = append(got, labels.head(root))
	}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("heads = %q, want %q", got, want)
	}
	for node, want := range map[*Node]string{
		roots[0].Children[0]:             "errorReq : Real",
		roots[0].Children[1].Children[0]: "inlet",
		roots[6].Children[1]:             "focal",
	} {
		if got := labels.head(node); got != want {
			t.Errorf("child %q headed %q, want %q", node.Name, got, want)
		}
	}
	rendering := &Rendering{View: "V", Kind: KindTree, Roots: roots}
	if text := rendering.Text(); !strings.Contains(text, "attribute TMT::Budget::'K-Mirror Offset'::errorReq : ScalarValues::Real\n") {
		t.Errorf("the text form is headed by the diagram label:\n%s", text)
	}
}

// A member of a drawn usage's type is headed by its name below that type, as
// the usage's box holds it — an interconnection rendering exposes the parts and
// ports of a part's type flat; a member of a type no drawn usage has keeps its
// qualifier, and a type written without one names no owner.
func TestLabelsHeadMembersUnderTheirDrawnOwnersType(t *testing.T) {
	roots := []*Node{
		{Kind: "part", Name: "TMT::Design::'Optical Bench'::sH", Type: "TMT::Design::'SH Assembly'::SH"},
		{Kind: "part", Name: "TMT::Design::'SH Assembly'::SH::'SH Filter Wheel'", Type: "TMT::Design::Parts::'Rotational Filter Wheel'"},
		{Kind: "port", Name: "TMT::Design::Parts::'Rotational Filter Wheel'::'bN sensor2'", Type: "~TMT::Design::Parts::'BN sensor'"},
		{Kind: "port", Name: "TMT::Design::Parts::Shutter::digital3", Type: "Digital"},
		{Kind: "part", Name: "TMT::Design::'Optical Bench'::pIT", Type: "PIT"},
		{Kind: "part", Name: "TMT::Design::PIT::'PIT CCD'", Type: "CCD"},
		// A type written by its bare name is drawn as the element it resolves to.
		{Kind: "part", Name: "TMT::Design::'APS Physical'::'summit Installation'", Type: "'Summit Installation'",
			Typings: []string{"TMT::Design::Physical::'Summit Installation'"}},
		{Kind: "part", Name: "TMT::Design::Physical::'Summit Installation'::computer", Type: "Control"},
	}
	want := []string{
		"'Optical Bench'::sH : SH",
		"'SH Filter Wheel' : 'Rotational Filter Wheel'",
		"'bN sensor2' : ~'BN sensor'",
		"Parts::Shutter::digital3 : Digital",
		"'Optical Bench'::pIT : PIT",
		"PIT::'PIT CCD' : CCD",
		"'APS Physical'::'summit Installation' : 'Summit Installation'",
		"computer : Control",
	}
	labels := labelsOf(roots, false)
	var got []string
	for _, root := range roots {
		got = append(got, labels.head(root))
	}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("heads = %q, want %q", got, want)
	}
	// From a model: the usage's type is written by its imported bare name and
	// resolves to the element whose member is exposed beside it.
	rendering := render(t, "typings.sysml", "SpelledViews::siteView")
	labels = labelsOf(rendering.Roots, false)
	got = got[:0]
	for _, root := range rendering.Roots {
		got = append(got, labels.head(root))
	}
	want = []string{"Physical::'APS Physical'::'summit Installation' : 'Summit Installation'", "computer : Control"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("heads = %q, want %q", got, want)
	}
	if text := rendering.Text(); !strings.Contains(text, "part Sites::'Summit Installation'::computer : Control") {
		t.Errorf("the text form is headed by the diagram label:\n%s", text)
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
