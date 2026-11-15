package runtime

import (
	"testing"
)

const subsettingElsewherePrelude = `
		private import ScalarValues::Real;
		private import ScalarValues::Integer;
		private import RealFunctions::*;
		private import SequenceFunctions::size;
		part def Sub { attribute mass : Real default 1.0; }
		part def Heavy :> Sub { attribute :>> mass = 5.0; }
		part def Tray { part items : Sub[*]; attribute held : Integer = size(items); }
		part def V {
			part subsystem : Sub[*];
			attribute n : Integer = size(subsystem);
			attribute total : Real = sum(subsystem.mass);
			part tray : Tray;
		}
`

// TestSubsetterDeclaredElsewhereIsHeld pins that a feature subsetting a feature of another
// object is one of the values that feature holds (KerML 1.1 §7.3.4.4): the subsetted feature
// need only be accessible to it, by nesting or by a feature chain.
func TestSubsetterDeclaredElsewhereIsHeld(t *testing.T) {
	cases := []struct {
		name, model, root, path, want string
	}{
		{"nested under a usage of the definition", `
			part def W :> V { part frame { part beam : Heavy :> subsystem; } }
			part w : W;`, "test::w", "total", "5.0"},
		{"nested two usages deep", `
			part def W :> V { part frame { part joint { part pin : Sub :> subsystem; } } }
			part w : W;`, "test::w", "n", "1"},
		{"nested under each object of a usage", `
			part def W :> V { part wheels[3] { part hub : Sub :> subsystem; } }
			part w : W;`, "test::w", "n", "3"},
		{"beside a subsetter of the same object", `
			part def W :> V { part own : Sub :> subsystem; part frame { part beam : Heavy :> subsystem; } }
			part w : W;`, "test::w", "total", "6.0"},
		{"nested in a usage body", `
			part w : V { part frame { part beam : Heavy :> subsystem; } }`, "test::w", "total", "5.0"},
		{"a chain to a feature of a nested object", `
			part def W :> V { part stowed : Sub :> tray.items; }
			part w : W;`, "test::w", "tray.held", "1"},
		{"a chain leaves the object's own feature alone", `
			part def W :> V { part stowed : Sub :> tray.items; }
			part w : W;`, "test::w", "n", "0"},
		{"a chain from a nested usage", `
			part def W :> V { part frame { part beam : Sub :> tray.items; } }
			part w : W;`, "test::w", "tray.held", "1"},
		{"inherited by a specialization", `
			part def W :> V { part frame { part beam : Sub :> subsystem; } }
			part def X :> W { part extra : Sub :> subsystem; }
			part x : X;`, "test::x", "n", "2"},
		{"redefined nested member keeps its membership", `
			part def W :> V { part frame { part beam : Sub :> subsystem; } }
			part def X :> W { part :>> frame { part :>> beam { attribute :>> mass = 7.0; } part brace : Sub :> subsystem; } }
			part x : X;`, "test::x", "total", "8.0"},
		{"redefined collection read under the redefined name", `
			part def W :> V { part parts : Sub[*] :>> subsystem; part frame { part beam : Sub :> parts; } }
			part w : W;`, "test::w", "n", "1"},
		{"default null yields to a nested subsetter", `
			part def D { part subs : Sub[*] default null; attribute n : Integer = size(subs); part frame { part beam : Sub :> subs; } }
			part d : D;`, "test::d", "n", "1"},
		{"default null nothing subsets stays empty", `
			part def D { part subs : Sub[*] default null; attribute n : Integer = size(subs); part frame { part beam : Sub; } }
			part d : D;`, "test::d", "n", "0"},
		{"the nearest object carrying the feature is the context", `
			part def W :> V { part inner : V { part beam : Sub :> subsystem; } }
			part w : W;`, "test::w", "n", "0"},
		{"the nearest object holds it", `
			part def W :> V { part inner : V { part beam : Sub :> subsystem; } }
			part w : W;`, "test::w", "inner.n", "1"},
		{"an object held twice is held once", `
			part def W :> V { part own : Sub :> subsystem; part frame { ref part again : Sub :> subsystem = own; } }
			part w : W;`, "test::w", "n", "1"},
		{"a namespace usage subsetting through a chain", `
			part rig : V;
			part extra : Heavy :> rig.subsystem;
			part bench { part clamp : Sub :> rig.subsystem; part tote : Sub :> rig.tray.items; }`, "test::rig", "total", "6.0"},
		{"a namespace chain reaches the nested object", `
			part rig : V;
			part bench { part tote : Sub :> rig.tray.items; }`, "test::rig", "tray.held", "1"},
		{"another namespace usage of the type is not reached", `
			part rig : V;
			part spare : V;
			part extra : Heavy :> rig.subsystem;`, "test::spare", "n", "0"},
		{"recursive roll-up through nested stacks", `
			part def C { attribute m : Real default 1.0; part subcomponents : C[*] default null;
				attribute totalMass : Real = m + sum(subcomponents.totalMass); }
			part def Leaf :> C { attribute :>> m = 10.0; }
			part def Stack :> C { part shelf { part a : Leaf :> subcomponents; part b : Leaf :> subcomponents; } }
			part def Tower :> C { part base { part s : Stack :> subcomponents; } }
			part tower : Tower;`, "test::tower", "totalMass", "22.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, inst, err := instantiateWithLibraries(t, "package test {"+subsettingElsewherePrelude+tc.model+"\n}", tc.root)
			if err != nil {
				t.Fatalf("instantiate %s: %v", tc.root, err)
			}
			fv, err := featureValueAtPath(t, ctx, inst, tc.path)
			if err != nil {
				t.Fatalf("%s: %v", tc.path, err)
			}
			if got := FormatValue(fv.HeldValue()); got != tc.want {
				t.Errorf("%s = %s, want %s", tc.path, got, tc.want)
			}
		})
	}
}

// TestSubsetterDeclaredElsewhereDrawsFromABoundCollection pins that an explicit binding of
// the subsetted feature stays authoritative: a subsetter nested elsewhere holds one of the
// values bound rather than adding an object of its own.
func TestSubsetterDeclaredElsewhereDrawsFromABoundCollection(t *testing.T) {
	ctx, inst, err := instantiateWithLibraries(t, "package test {"+subsettingElsewherePrelude+`
		part x1 : Sub;
		part x2 : Sub;
		part def B { part subs : Sub[*] = (x1, x2); part frame { part beam : Sub :> subs; } }
		part b : B;
	}`, "test::b")
	if err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	subs, err := inst.GetFeatureValue(ctx, "subs")
	if err != nil {
		t.Fatalf("subs: %v", err)
	}
	held := elementsOf(subs.HeldValue())
	if len(held) != 2 {
		t.Fatalf("subs holds %d objects, want the 2 bound", len(held))
	}
	beam, err := featureValueAtPath(t, ctx, inst, "frame.beam")
	if err != nil {
		t.Fatalf("frame.beam: %v", err)
	}
	got := elementsOf(beam.HeldValue())
	if len(got) != 1 || got[0].Kind != ValInstance || got[0].Instance != held[0].Instance {
		t.Errorf("frame.beam = %v, want the first object subs is bound to (%v)", got, held[0])
	}
}
