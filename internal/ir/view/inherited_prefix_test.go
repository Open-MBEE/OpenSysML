package view

import (
	"encoding/json"
	"strings"
	"testing"
)

// A usage drawn under an owner that does not declare it — through typing,
// subclassification or subsetting — is marked Inherited; one the owner
// declares, a redefinition included, is not.
func TestInheritedUsagesAreMarkedUnderEveryKindOfOwner(t *testing.T) {
	rendering := render(t, "inherited-prefix.sysml", "InheritedPrefixViews::rigTree")
	rig := nodeAt(rendering.Roots, "InheritedPrefixViews::Rig")
	if rig == nil {
		t.Fatalf("Rig is not drawn; roots: %v", nodeNames(rendering.Roots))
	}
	for _, tc := range []struct {
		owner, member string
		inherited     bool
	}{
		{"a", "aPort", true}, {"a", "mass", true}, {"a", "engine", true},
		{"b", "mass", true}, {"b", "cooler", true}, {"b", "aPort", true},
		{"c", "aPort", true}, {"c", "mass", true}, {"c", "engine", true},
		{"d", "mass", false}, {"d", "aPort", true}, {"d", "engine", true},
	} {
		node := nodeAt(rig.Children, tc.owner, tc.member)
		if node == nil {
			t.Errorf("%s.%s is not drawn", tc.owner, tc.member)
			continue
		}
		if node.Inherited != tc.inherited {
			t.Errorf("%s.%s Inherited = %v, want %v", tc.owner, tc.member, node.Inherited, tc.inherited)
		}
	}
	for _, child := range rig.Children {
		if child.Inherited {
			t.Errorf("Rig declares %s, yet it is marked inherited", child.Name)
		}
	}
}

// Each writer puts the notation's `^` before an inherited usage's name and
// nothing before a declared one; the type and the rest of the label stay.
func TestWritersPrefixInheritedUsages(t *testing.T) {
	rendering := render(t, "inherited-prefix.sysml", "InheritedPrefixViews::rigTree")
	for form, wants := range map[Form][]string{
		FormText:     {"  part d : APart\n    attribute mass\n    port ^aPort : APort\n    part ^engine : Engine\n", "  part c\n    port ^aPort : APort\n"},
		FormMermaid:  {"**^aPort : APort**", "**^mass**", "**mass**`\")", "**^cooler : Engine**"},
		FormDot:      {"<b>^aPort : APort</b>", "<b>^mass</b>", "<b>mass</b>", "<b>^cooler : Engine</b>"},
		FormPlantUML: {"**^aPort : APort**", "**^mass**", "**mass**\" as", "**^cooler : Engine**"},
		FormD2:       {"\\n^aPort : APort\"", "\\n^mass\"", "\\nmass\"", "\\n^cooler : Engine\""},
	} {
		out, err := rendering.WriteWith(form, Options{Show: AllShown()})
		if err != nil {
			t.Fatalf("%s: %v", form, err)
		}
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Errorf("%s lacks %q:\n%s", form, want, out)
			}
		}
	}
}

// A compartment row lists an inherited member after `^`; a redefinition the
// owner declares is listed bare.
func TestCompartmentRowsPrefixInheritedUsages(t *testing.T) {
	rendering := render(t, "inherited-prefix.sysml", "InheritedPrefixViews::rigTree")
	folded := foldCompartments(rendering.showing(AllShown()), CompartmentsPorts)
	rig := nodeAt(folded.Roots, "InheritedPrefixViews::Rig")
	if rig == nil {
		t.Fatalf("Rig is not drawn; roots: %v", nodeNames(folded.Roots))
	}
	a, d := nodeAt(rig.Children, "a"), nodeAt(rig.Children, "d")
	if a == nil || d == nil {
		t.Fatalf("a and d are not drawn; children: %v", childNames(rig))
	}
	if got := compartmentText(a); got != "ports: ^aPort : APort" {
		t.Errorf("a's compartments = %q", got)
	}
	most := foldCompartments(rendering.showing(AllShown()), CompartmentsMost)
	if got := compartmentText(nodeAt(most.Roots, "InheritedPrefixViews::Rig")); !strings.Contains(got, "a : APart") || strings.Contains(got, "^") {
		t.Errorf("Rig's own members fold bare: %q", got)
	}
	text := folded.textWith(Options{Show: AllShown(), Compartments: CompartmentsPorts})
	for _, want := range []string{"      port ^aPort : APort\n", "    attribute mass\n", "    attribute ^mass\n"} {
		if !strings.Contains(text, want) {
			t.Errorf("folded text lacks %q:\n%s", want, text)
		}
	}
}

// The edge a folded row's membership and typing fold into names the row the
// way its label does: after `^` when the row is inherited.
func TestFoldedMembershipEdgeLabelsPrefixInheritedUsages(t *testing.T) {
	owner := &Node{ID: "n0", Kind: "part def", Name: "Rig"}
	def := &Node{ID: "n1", Kind: "port def", Name: "APort"}
	own := &Node{ID: "n2", Kind: "port", Name: "own", Type: "APort"}
	inherited := &Node{ID: "n3", Kind: "port", Name: "got", Type: "APort", Inherited: true}
	owner.Children = []*Node{own, inherited}
	rendering := &Rendering{Kind: KindTree, Shown: AllShown(), Roots: []*Node{owner, def}, Edges: []Edge{
		{From: owner.ID, To: own.ID, Kind: EdgeReference}, {From: own.ID, To: def.ID, Kind: EdgeTyping},
		{From: owner.ID, To: inherited.ID, Kind: EdgeReference, ToLabel: "[2]"}, {From: inherited.ID, To: def.ID, Kind: EdgeTyping},
	}}
	folded := foldCompartments(rendering, CompartmentsPorts)
	var labels []string
	for _, edge := range folded.Edges {
		labels = append(labels, edge.ToLabel)
	}
	if got, want := strings.Join(labels, ","), "own,^got[2]"; got != want {
		t.Errorf("folded edge labels = %q, want %q", got, want)
	}
}

// An interconnection pins the ports a part has from its type as inherited and
// draws them after `^` in every form; a nested usage from the type likewise.
func TestInterconnectionPrefixesInheritedPortsAndMembers(t *testing.T) {
	rendering := render(t, "inherited-prefix.sysml", "InheritedPrefixViews::rigLinks")
	rig := nodeAt(rendering.Roots, "InheritedPrefixViews::Rig")
	if rig == nil {
		t.Fatalf("Rig is not drawn; roots: %v", nodeNames(rendering.Roots))
	}
	a := nodeAt(rig.Children, "a")
	if a == nil || len(a.Ports) != 1 || a.Ports[0].Name != "aPort" || !a.Ports[0].Inherited {
		t.Fatalf("a's ports = %+v, want the inherited aPort", a)
	}
	if engine := nodeAt(a.Children, "engine"); engine == nil || !engine.Inherited {
		t.Errorf("a.engine = %+v, want an inherited part", engine)
	}
	for form, wants := range map[Form][]string{
		FormText:     {"    port ^aPort\n", "    part ^engine : Engine\n", "    attribute mass\n"},
		FormMermaid:  {"[\"^aPort\"]", "**^engine : Engine**"},
		FormDot:      {`xlabel="^aPort"`, "<b>^engine : Engine</b>"},
		FormPlantUML: {"port \"^aPort\" as", "**^engine : Engine**"},
		FormD2:       {"\"^aPort\" { class: pin }", "\\n^engine : Engine\""},
	} {
		out, err := rendering.WriteWith(form, Options{Show: AllShown()})
		if err != nil {
			t.Fatalf("%s: %v", form, err)
		}
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Errorf("%s lacks %q:\n%s", form, want, out)
			}
		}
	}
	full, err := rendering.WriteWith(FormPlantUML, Options{Show: AllShown(), Ports: PortsFull})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(full, "port \"^aPort : APort\" as") {
		t.Errorf("the full display keeps the prefix before the typed pin:\n%s", full)
	}
}

// The data form carries the mark as a field and keeps the bare name, so a
// client keying on names is unmoved and one drawing the notation adds the `^`.
func TestDataMarksInheritedUsagesApartFromTheirNames(t *testing.T) {
	rendering := render(t, "inherited-prefix.sysml", "InheritedPrefixViews::part1Tree")
	data := rendering.Data()
	byName := map[string]NodeData{}
	for _, node := range data.Nodes {
		byName[node.Name] = node
	}
	for name, want := range map[string]bool{"InheritedPrefixViews::part1": false, "aPort": true, "mass": true, "engine": true} {
		node, ok := byName[name]
		if !ok {
			t.Errorf("no node named %q in %v", name, data.Nodes)
			continue
		}
		if node.Inherited != want {
			t.Errorf("%s Inherited = %v, want %v", name, node.Inherited, want)
		}
	}
	encoded, err := json.Marshal(data.Nodes)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"Name":"aPort","NameSynthesized":false,"Type":"","Detail":"","Text":"","StandIn":false,"Inherited":true`) ||
		strings.Contains(string(encoded), "^") {
		t.Errorf("the JSON carries Inherited beside the bare name:\n%s", encoded)
	}
	links := render(t, "inherited-prefix.sysml", "InheritedPrefixViews::rigLinks").Data()
	var pinned bool
	for _, node := range links.Nodes {
		for _, port := range node.Ports {
			if port.Name == "aPort" && port.Inherited {
				pinned = true
			}
		}
	}
	if !pinned {
		t.Errorf("no inherited aPort pin in the interconnection data: %+v", links.Nodes)
	}
}

// A behaviour's pins from its type are inherited too.
func TestActionPinsFromTheTypeAreInherited(t *testing.T) {
	rendering := renderActionPins(t, "Pins::ToastBread")
	var heat *Node
	for _, root := range rendering.Roots {
		if heat = nodeAt(root.Children, "heat"); heat != nil {
			break
		}
	}
	if heat == nil {
		t.Fatalf("heat is not drawn; roots: %v", nodeNames(rendering.Roots))
	}
	got := map[string]bool{}
	for _, port := range heat.Ports {
		got[port.Name] = port.Inherited
	}
	if inherited, ok := got["t"]; !ok || !inherited {
		t.Errorf("heat's pins = %v, want the out t from Heat marked inherited", got)
	}
	if inherited, ok := got["b"]; !ok || inherited {
		t.Errorf("heat's pins = %v, want its own in b unmarked", got)
	}
}
