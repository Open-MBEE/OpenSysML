package repl

import (
	"slices"
	"strings"
	"testing"
)

// An object written into a collection populated only through subsetting implied
// by nesting has no declared feature path to be enumerated under: once the
// collection is materialized, the session's walks yield the object under the
// collection's own path, while objects a declared feature already carries keep
// the path that holds them.
func TestObjectsWrittenToAnImpliedCollectionAreEnumerated(t *testing.T) {
	s := loadSource(t, `package A {
		private import ScalarValues::*;
		private import SequenceFunctions::*;
		private import OccurrenceFunctions::*;
		part def Wheel { attribute size : Integer = 1; }
		part vehicle {
			part wheels : Wheel[1];
			perform action build {
				first start;
				then action make { assign subparts := addNew(subparts, new Wheel()); }
				then done;
			}
		}
	}`)
	run(t, s, "%instantiate A::vehicle")
	// Reading the collection materializes it: the wheel the part declares and the
	// one the build wrote into it directly, with no other path to reach it.
	wants(t, run(t, s, "%eval in A::vehicle : subparts"), "Instance(ID: ")

	vehicle := s.instances["A::vehicle"]
	if vehicle == nil {
		t.Fatal("no vehicle object")
	}
	fv := vehicle.FeatureValues["subparts"]
	if fv == nil || !fv.Materialized {
		t.Fatal("subparts was not materialized")
	}
	var labels []string
	for _, id := range s.heldIDs() {
		if id == vehicle.ID {
			continue
		}
		label, ok := s.heldLabel(id)
		if !ok {
			t.Errorf("held object #%d has no path", id)
			continue
		}
		labels = append(labels, label)
	}
	slices.Sort(labels)
	if !slices.Equal(labels, []string{"A::vehicle.build", "A::vehicle.subparts[2]", "A::vehicle.wheels"}) {
		t.Errorf("held paths = %v, want the wheel under its declared feature and the written one under subparts", labels)
	}

	got := s.Complete("%features A::vehicle.", len("%features A::vehicle.")).Candidates
	var subparts []string
	for _, c := range got {
		if strings.HasSuffix(c, "subparts[2]") {
			subparts = append(subparts, c)
		}
	}
	if len(subparts) != 1 {
		t.Errorf("completion offered %v: the object written into subparts was not offered", got)
	}
	for _, c := range got {
		if _, _, err := s.resolveObject(c); err != nil {
			t.Errorf("completion %q does not resolve: %v", c, err)
		}
	}
}
