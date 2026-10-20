package view

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const relatedModel = `package Rel {
	part def Vehicle;
	part def Car :> Vehicle;
	part def Truck :> Vehicle;
	part def Engine;
	part def Wheel;
	part car : Car {
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
		first paint then ship;
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

// A state relates to the states its machine's transitions reach from it and
// leave for it; an action to the actions its owner's successions join it to.
func TestRelatedGroupsBehaviorRelationships(t *testing.T) {
	wantRelated(t, "Rel::Lifecycle::on", map[RelatedKind]string{
		RelatedOwner:       "Rel::Lifecycle",
		RelatedTransitions: "Rel::Lifecycle::off Rel::Lifecycle::broken",
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
