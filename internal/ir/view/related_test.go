package view

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const relatedModel = `package Rel {
	private import DiagramLayout::*;
	part def Vehicle;
	part def Car :> Vehicle;
	part def Truck :> Vehicle;
	part def Engine;
	part def Wheel;
	part car : Car {
		metadata Layout about engine { x = 10; y = 20; }
		part engine : Engine;
		part wheel : Wheel;
		part frontWheel :> wheel;
		connect engine to wheel;
		flow from engine.torque to wheel.torque;
		bind engine.mass = wheel.mass;
	}
	part truck : Truck;
	state def Lifecycle {
		entry; then off;
		state off;
		state on;
		state broken;
		transition off_on first off then on;
		transition on_broken first on then broken;
	}
	action def Build {
		action weld;
		action paint;
		action ship;
		first weld then paint;
		succession checked first paint if true then ship;
	}
}
`

func relatedOf(t *testing.T, fqn string) map[RelatedKind][]string {
	t.Helper()
	r, idx := loadSources(t, []string{"rel.sysml"}, [][]byte{[]byte(relatedModel)})
	out := map[RelatedKind][]string{}
	for _, group := range r.Related(lookup(t, idx, fqn)) {
		if group.Label == "" {
			t.Errorf("%s: group %s has no label", fqn, group.Kind)
		}
		for _, elem := range group.Elements {
			out[group.Kind] = append(out[group.Kind], idx.GetFQN(elem))
		}
	}
	return out
}

func wantRelated(t *testing.T, fqn string, want map[RelatedKind]string) {
	t.Helper()
	got := relatedOf(t, fqn)
	for kind, elems := range want {
		if strings.Join(got[kind], " ") != elems {
			t.Errorf("%s %s = %q, want %q", fqn, kind, strings.Join(got[kind], " "), elems)
		}
	}
	for kind := range got {
		if _, listed := want[kind]; !listed {
			t.Errorf("%s lists %s = %v, want no such group", fqn, kind, got[kind])
		}
	}
}

// A definition relates to what it specializes, what specializes it, the usages
// it types, its owner and its members; a usage to its type, its owner and the
// elements connectors, flows and bindings join it to.
func TestRelatedGroupsStructuralRelationships(t *testing.T) {
	wantRelated(t, "Rel::Vehicle", map[RelatedKind]string{
		RelatedSpecializations: "Rel::Car Rel::Truck",
		RelatedOwner:           "Rel",
	})
	wantRelated(t, "Rel::Car", map[RelatedKind]string{
		RelatedGeneralizations: "Rel::Vehicle",
		RelatedTypedUsages:     "Rel::car",
		RelatedOwner:           "Rel",
	})
	wantRelated(t, "Rel::car", map[RelatedKind]string{
		RelatedTypes:        "Rel::Car",
		RelatedOwner:        "Rel",
		RelatedOwnedMembers: "Rel::car::engine Rel::car::wheel Rel::car::frontWheel",
	})
	wantRelated(t, "Rel::car::engine", map[RelatedKind]string{
		RelatedTypes:       "Rel::Engine",
		RelatedOwner:       "Rel::car",
		RelatedConnections: "Rel::car::wheel",
		RelatedFlows:       "Rel::car::wheel",
		RelatedBindings:    "Rel::car::wheel",
	})
	wantRelated(t, "Rel::car::frontWheel", map[RelatedKind]string{
		RelatedGeneralizations: "Rel::car::wheel",
		RelatedOwner:           "Rel::car",
	})
}

// An action relates to the actions its owner's successions join it to; a state
// has no transitions group, being drawn only within its machine.
func TestRelatedGroupsBehaviorRelationships(t *testing.T) {
	wantRelated(t, "Rel::Lifecycle::on", map[RelatedKind]string{
		RelatedOwner: "Rel::Lifecycle",
	})
	wantRelated(t, "Rel::Build::paint", map[RelatedKind]string{
		RelatedOwner:       "Rel::Build",
		RelatedSuccessions: "Rel::Build::weld Rel::Build::ship",
	})
}

func TestRelatedKindsListEveryGroupKind(t *testing.T) {
	kinds := RelatedKinds()
	if len(kinds) != len(relatedLabels) {
		t.Fatalf("RelatedKinds lists %d kinds, %d labelled", len(kinds), len(relatedLabels))
	}
	for _, kind := range kinds {
		if relatedLabels[kind] == "" {
			t.Errorf("%s has no label", kind)
		}
	}
	var none *symbols.Symbol
	r, _ := loadSources(t, []string{"rel.sysml"}, [][]byte{[]byte(relatedModel)})
	if got := r.Related(none); got != nil {
		t.Errorf("Related(nil) = %v, want nil", got)
	}
}

// Actions of one action exposed as roots of their own are joined by its
// successions between them; beside the action itself they take no root edge.
func TestActionRootsAreJoinedByTheirOwnersSuccessions(t *testing.T) {
	r, idx := loadSources(t, []string{"rel.sysml"}, [][]byte{[]byte(relatedModel)})
	actions, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, "Rel::Build::weld"), lookup(t, idx, "Rel::Build::ship"), lookup(t, idx, "Rel::Build::paint")}, KindAction, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(actions.Roots) != 3 || len(actions.Edges) != 2 {
		t.Fatalf("actions: roots %d, edges %+v, notices %v; want weld -> paint -> ship", len(actions.Roots), actions.Edges, actions.Notices)
	}
	for i, want := range [][3]string{{"Rel::Build::weld", "Rel::Build::paint", ""}, {"Rel::Build::paint", "Rel::Build::ship", "[true]"}} {
		edge := actions.Edges[i]
		if edge.Kind != EdgeSuccession || actions.Roots[rootIndex(actions, edge.From)].Name != want[0] || actions.Roots[rootIndex(actions, edge.To)].Name != want[1] || edge.Label != want[2] {
			t.Errorf("edge %d = %+v, want succession %s -> %s labelled %q", i, edge, want[0], want[1], want[2])
		}
	}
	whole, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, "Rel::Build")}, KindAction, "")
	if err != nil {
		t.Fatal(err)
	}
	var within []string
	for _, edge := range whole.Edges {
		if edge.Kind == EdgeSuccession {
			within = append(within, edge.Label)
		}
	}
	if strings.Join(within, "|") != strings.Join([]string{actions.Edges[0].Label, actions.Edges[1].Label}, "|") {
		t.Errorf("labels within Build = %q, want the root edges' %q", within, []string{actions.Edges[0].Label, actions.Edges[1].Label})
	}
	beside, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, "Rel::Build"), lookup(t, idx, "Rel::Build::paint"), lookup(t, idx, "Rel::Build::ship")}, KindAction, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range beside.Roots[1:] {
		for _, edge := range beside.Edges {
			if edge.From == root.ID || edge.To == root.ID {
				t.Errorf("an action exposed beside its owner takes no root edge: %+v", edge)
			}
		}
	}
}

func rootIndex(rendering *Rendering, id string) int {
	for i, root := range rendering.Roots {
		if root.ID == id {
			return i
		}
	}
	return -1
}

// An action inherited from another document is looked up in that document, so a
// succession between inherited actions never joins the owner's own actions whose
// declarations happen to sit at the same spans.
func TestInheritedSuccessionsDoNotJoinSameSpanActions(t *testing.T) {
	base := "package B {\n action def Base            {\n  action x;\n  action y;\n  first x then y;\n }\n}\n"
	next := "package N {\n action def Next :> B::Base {\n  action a;\n  action b;\n }\n}\n"
	if strings.Index(base, "action x;") != strings.Index(next, "action a;") || strings.Index(base, "action y;") != strings.Index(next, "action b;") {
		t.Fatalf("fixture: the declarations must share spans (%d/%d, %d/%d)", strings.Index(base, "action x;"), strings.Index(next, "action a;"), strings.Index(base, "action y;"), strings.Index(next, "action b;"))
	}
	r, idx := loadSources(t, []string{"base.sysml", "next.sysml"}, [][]byte{[]byte(base), []byte(next)})
	rendering, err := r.RenderExposed([]*symbols.Symbol{lookup(t, idx, "N::Next::a"), lookup(t, idx, "N::Next::b")}, KindAction, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rendering.Roots) != 2 || len(rendering.Edges) != 0 {
		t.Errorf("roots %d, edges %+v; want a and b unjoined: x -> y is Base's", len(rendering.Roots), rendering.Edges)
	}
}
