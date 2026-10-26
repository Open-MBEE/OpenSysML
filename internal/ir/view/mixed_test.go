package view

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// mixedFixture renders view of file and resolves nodes by the qualified name
// of the element they draw.
func mixedFixture(t *testing.T, file, view string) (*Rendering, func(fqn string) *Node) {
	t.Helper()
	renderer, index := loadFixture(t, file)
	rendering, err := renderer.Render(lookup(t, index, view))
	if err != nil {
		t.Fatalf("Render(%s): %v", view, err)
	}
	return rendering, func(fqn string) *Node {
		origin := symbolOrigin(lookup(t, index, fqn))
		var found *Node
		var visit func(nodes []*Node)
		visit = func(nodes []*Node) {
			for _, node := range nodes {
				// A behavior's node carries its declaration's origin, so it is found by name.
				if node.Origin == origin || found == nil && (node.Name == fqn || strings.Trim(node.Name, "'") == symbols.LastSegment(fqn)) {
					found = node
				}
				visit(node.Children)
			}
		}
		visit(rendering.Roots)
		if found == nil {
			t.Fatalf("no node draws %s", fqn)
		}
		return found
	}
}

// membershipEdgeTo is the one membership edge drawn from owner to member;
// the test fails on none or more than one.
func membershipEdgeTo(t *testing.T, rendering *Rendering, owner, member *Node) Edge {
	t.Helper()
	var found []Edge
	for _, edge := range rendering.Edges {
		if edge.From == owner.ID && edge.To == member.ID && membershipEdge(edge.Kind) {
			found = append(found, edge)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d membership edge(s) from %s to %s, want one: %+v", len(found), owner.Name, member.Name, found)
	}
	return found[0]
}

// A mixed rendering draws the membership of every member nested in its
// owner's node with the glyph the notation gives that membership: the diamond
// of a feature membership, filled or hollow by the model's isComposite, the
// circle-plus of an owned membership for a member that is no feature, and the
// keyword glyph of a membership drawn as a relationship of its own.
func TestMixedNestedMembershipEdgesTakeTheMembershipGlyph(t *testing.T) {
	rendering, node := mixedFixture(t, "mixed-memberships.sysml", "MixedMemberships::memberships")
	for _, tc := range []struct {
		owner, member string
		kind          EdgeKind
		label         string
	}{
		{"MixedMemberships", "MixedMemberships::Library", EdgeContainment, ""},
		{"MixedMemberships", "MixedMemberships::Impeller", EdgeContainment, ""},
		{"MixedMemberships", "MixedMemberships::Safety", EdgeContainment, ""},
		{"MixedMemberships::Library", "MixedMemberships::Library::Pressure", EdgeContainment, ""},
		{"MixedMemberships::Library", "MixedMemberships::Library::FluidPort", EdgeContainment, ""},
		{"MixedMemberships::Pump", "MixedMemberships::Pump::pressure", EdgeReference, ""},
		{"MixedMemberships::Pump", "MixedMemberships::Pump::outlet", EdgeReference, ""},
		{"MixedMemberships::Pump", "MixedMemberships::Pump::impeller", EdgeComposition, ""},
		{"MixedMemberships::Pump", "MixedMemberships::Pump::spare", EdgeReference, ""},
		{"MixedMemberships::Pump", "MixedMemberships::Pump::prime", EdgeComposition, ""},
		{"MixedMemberships::Pump", "MixedMemberships::Pump::armed", EdgeComposition, ""},
		{"MixedMemberships::Pump", "MixedMemberships::Pump::Seal", EdgeContainment, ""},
		{"MixedMemberships::Safety", "MixedMemberships::Safety::minPressure", EdgeRequire, "require"},
		{"MixedMemberships::Safety", "MixedMemberships::Safety::primedPump", EdgeAssume, "assume"},
		{"MixedMemberships::Safety", "MixedMemberships::Safety::sealed", EdgeAssert, "assert"},
		{"MixedMemberships::Safety", "MixedMemberships::Safety::contained", EdgeFrame, "frame"},
		{"MixedMemberships", "MixedMemberships::memberships", EdgeComposition, ""},
	} {
		member := node(tc.member)
		edge := membershipEdgeTo(t, rendering, node(tc.owner), member)
		if edge.Kind != tc.kind || edge.Label != tc.label {
			t.Errorf("%s → %s drawn as %s %q, want %s %q", tc.owner, tc.member, edge.Kind, edge.Label, tc.kind, tc.label)
		}
		if edge.Origin.Doc != member.Origin.Doc || edge.Origin.Span != member.Origin.Span {
			t.Errorf("%s → %s has origin %+v, want the member's %+v", tc.owner, tc.member, edge.Origin, member.Origin)
		}
	}
	// Every one of them is a nesting edge, the one the writers draw in place
	// of, or by, the nesting.
	nesting := rendering.nestingEdges()
	for _, edge := range rendering.Edges {
		if membershipEdge(edge.Kind) && !nesting[[2]string{edge.From, edge.To}] {
			t.Errorf("membership edge %s → %s (%s) is not a nesting edge", edge.From, edge.To, edge.Kind)
		}
	}
	if len(nesting) != 20 {
		t.Errorf("%d nesting edges, want 20 (17 checked above, the concern def, the pump def and the layout view)", len(nesting))
	}
}

// The body of a behavior and the roles of a case are drawn in their own
// notation: a state inside its state def, an action node inside its action
// def, an actor beside its case, each hangs off no membership edge.
func TestMixedLeavesBehaviorBodiesAndCaseRolesToTheirNotation(t *testing.T) {
	rendering, node := mixedFixture(t, "mixed.sysml", "MixedExamples::mixedDiagram")
	for _, member := range []string{"MixedExamples::Modes::ready", "MixedExamples::Operate::step", "MixedExamples::Inspection::operator", "MixedExamples::Inspection::plant"} {
		id := node(member).ID
		for _, edge := range rendering.Edges {
			if edge.To == id && membershipEdge(edge.Kind) {
				t.Errorf("%s hangs off a %s membership edge: %+v", member, edge.Kind, edge)
			}
		}
	}
	// The package's members, structural and behavioral alike, do.
	pkg := node("MixedExamples")
	for _, tc := range []struct {
		member string
		kind   EdgeKind
	}{
		{"MixedExamples::Modes", EdgeContainment}, {"MixedExamples::Operate", EdgeContainment}, {"MixedExamples::Inspection", EdgeContainment},
		{"MixedExamples::Plant", EdgeContainment}, {"MixedExamples::mixedDiagram", EdgeComposition},
	} {
		if edge := membershipEdgeTo(t, rendering, pkg, node(tc.member)); edge.Kind != tc.kind {
			t.Errorf("MixedExamples → %s drawn as %s, want %s", tc.member, edge.Kind, tc.kind)
		}
	}
	if edge := membershipEdgeTo(t, rendering, node("MixedExamples::Plant"), node("MixedExamples::Plant::pump")); edge.Kind != EdgeComposition {
		t.Errorf("Plant → pump drawn as %s, want composition", edge.Kind)
	}
}

// The forms draw each nested membership once. The text form lists it; PlantUML
// draws the glyph from the container to the member inside it; DOT and Mermaid,
// which could only run such an edge from an invisible anchor inside the
// container, leave it to the nesting.
func TestMixedNestedMembershipEdgesPerForm(t *testing.T) {
	rendering, node := mixedFixture(t, "mixed-memberships.sysml", "MixedMemberships::memberships")
	pump, impeller, seal := node("MixedMemberships::Pump"), node("MixedMemberships::Pump::impeller"), node("MixedMemberships::Pump::Seal")
	text := rendering.Text()
	for _, line := range []string{"  Pump *-- impeller\n", "  Pump o-- spare\n", "  Pump +-- Seal\n", "  MixedMemberships +-- Library\n", "  Safety -> minPressure: require\n"} {
		if !strings.Contains(text, line) {
			t.Errorf("text lists no %q:\n%s", strings.TrimSpace(line), text)
		}
	}
	plantuml, err := rendering.Write(FormPlantUML)
	if err != nil {
		t.Fatalf("write plantuml: %v", err)
	}
	for _, line := range []string{pump.ID + " *-- " + impeller.ID + "\n", pump.ID + " +-- " + seal.ID + "\n"} {
		if !strings.Contains(plantuml, line) {
			t.Errorf("PlantUML draws no %q:\n%s", strings.TrimSpace(line), plantuml)
		}
	}
	dot, err := rendering.Write(FormDot)
	if err != nil {
		t.Fatalf("write dot: %v", err)
	}
	if strings.Contains(dot, dotQuote(pump.ID)+" -> "+dotQuote(impeller.ID)) || strings.Contains(dot, "arrowtail=diamond") || strings.Contains(dot, "arrowtail=odot") {
		t.Errorf("DOT draws a membership edge into the owner's cluster:\n%s", dot)
	}
	if !strings.Contains(dot, dotQuote(impeller.ID)+" -> ") {
		t.Errorf("DOT lost the typing edge of impeller:\n%s", dot)
	}
	mermaid := rendering.Mermaid()
	if strings.Contains(mermaid, pump.ID+"_anchor") || strings.Contains(mermaid, "◆") || strings.Contains(mermaid, "◇") {
		t.Errorf("Mermaid draws a membership edge from an anchor inside the subgraph:\n%s", mermaid)
	}
}

// The edges of a mixed rendering come in one order, the nodes' order, an
// owner's memberships before its members' own.
func TestMixedMembershipEdgeOrderIsDeterministic(t *testing.T) {
	first, node := mixedFixture(t, "mixed-memberships.sysml", "MixedMemberships::memberships")
	for i := 0; i < 3; i++ {
		again, _ := mixedFixture(t, "mixed-memberships.sysml", "MixedMemberships::memberships")
		if !reflect.DeepEqual(again.Edges, first.Edges) {
			t.Fatalf("edge order differs between renderings:\n%+v\n%+v", first.Edges, again.Edges)
		}
	}
	at := map[string]int{}
	for i, edge := range first.Edges {
		if edge.Kind == EdgeContainment {
			at[edge.To] = i + 1
		}
	}
	library, pressure := node("MixedMemberships::Library").ID, node("MixedMemberships::Library::Pressure").ID
	if at[library] == 0 || at[pressure] == 0 || at[library] > at[pressure] {
		t.Errorf("the package's membership of Library (edge %d) comes after Library's of Pressure (edge %d)", at[library], at[pressure])
	}
}

// A Route about a nested usage steers its membership edge, and a positioned
// drawing that leaves the usage's node unplaced folds the membership and the
// usage's typing into the owner's labelled edge to the definition, routed as
// the usage was — as a positioned tree folds them.
func TestMixedPositionedNestedMembershipFolds(t *testing.T) {
	rendering, node := mixedFixture(t, "mixed-memberships.sysml", "MixedMemberships::membershipsLayout")
	pump, impeller, impellerDef := node("MixedMemberships::Pump"), node("MixedMemberships::Pump::impeller"), node("MixedMemberships::Impeller")
	edge := membershipEdgeTo(t, rendering, pump, impeller)
	if !reflect.DeepEqual(edge.Route, []Point{{60, 40}, {90, 70}}) {
		t.Errorf("membership route = %+v, want (60, 40) to (90, 70)", edge.Route)
	}
	for _, other := range rendering.Edges {
		if other.From == impeller.ID && other.Kind == EdgeTyping && other.Route != nil {
			t.Errorf("the typing of impeller carries the usage's route too: %+v", other.Route)
		}
	}
	dot, err := rendering.Write(FormDot)
	if err != nil {
		t.Fatalf("write dot: %v", err)
	}
	if !strings.Contains(dot, dotQuote(pump.ID)+" -> "+dotQuote(impellerDef.ID)) || !strings.Contains(dot, `headlabel="impeller"`) {
		t.Errorf("DOT does not fold the unplaced impeller into Pump's labelled edge to Impeller:\n%s", dot)
	}
}
