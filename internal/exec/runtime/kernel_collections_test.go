package runtime

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

const kernelCollectionWalkModel = `
	package test {
		part def Sub;
		part def System { part Subsystems : Sub[*]; }
		part def Sat :> System {
			part subsystems : Sub[*] :>> Subsystems;
			part bus : Sub :> subsystems;
		}
		part sat : Sat;
	}
`

// TestSubsetsTransitivelyWalksDeclaredImplicitAndRedefined pins the walk a derived read follows:
// a declared `:>`, an implicit nesting subsetting and a redefinition, through out-of-shape features.
func TestSubsetsTransitivelyWalksDeclaredImplicitAndRedefined(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, kernelCollectionWalkModel))
	symbol := func(qualified string) *symbols.Symbol {
		t.Helper()
		matches := idx.LookupQualified(qualified)
		if len(matches) != 1 {
			t.Fatalf("%s: %d matching symbols, want 1", qualified, len(matches))
		}
		return matches[0]
	}
	inst, err := ctx.Instantiate(symbol("test::sat"))
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	shape := func(name string) *symbols.Symbol {
		t.Helper()
		for _, feat := range ctx.FeaturesOf(inst.Type) {
			if feat.Name == name {
				return feat.Symbol
			}
		}
		t.Fatalf("%s: not a shape feature of %s", name, SymbolText(inst.Type))
		return nil
	}
	member := func(name string) *symbols.Symbol {
		t.Helper()
		if strings.Contains(name, "::") {
			return symbol(name)
		}
		sym, found := ctx.model.semantics.LookupMember(inst.Type, name)
		if !found || sym == nil {
			t.Fatalf("%s: not a member of %s", name, SymbolText(inst.Type))
		}
		return sym
	}
	for _, tc := range []struct {
		name    string
		feature *symbols.Symbol
		target  string
		want    bool
	}{
		{"declared", shape("bus"), "subsystems", true},
		{"declared_then_redefined", shape("bus"), "test::System::Subsystems", true},
		{"implicit", shape("bus"), "subparts", true},
		{"implicit_then_declared_through_the_library", shape("bus"), "subobjects", true},
		{"implicit_then_declared_to_the_kernel", shape("bus"), "suboccurrences", true},
		{"redefinition_is_not_subsetting_backwards", shape("Subsystems"), "subsystems", false},
		{"unrelated_kernel_collection", shape("bus"), "successors", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ctx.subsetsTransitively(tc.feature, inst.Type, member(tc.target)); got != tc.want {
				t.Errorf("subsetsTransitively(%s, %s) = %v, want %v", SymbolText(tc.feature), tc.target, got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"suboccurrences", true},
		{"subobjects", true},
		{"successors", false},
		{"withoutOccurrences", false},
		{"timeEnclosedOccurrences", false},
		{"superoccurrence", false},
		{"timeSlices", false},
		{"snapshots", false},
		{"portions", false},
		{"self", false},
		{"subparts", false},
	} {
		t.Run("kernel_collection_"+tc.name, func(t *testing.T) {
			if _, got := ctx.kernelCollection(inst.Type, tc.name); got != tc.want {
				t.Errorf("kernelCollection(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

const kernelCollectionOpenModel = `
	package test {
		part def Child;
		part def Q {
			part children : Child[1..*];
		}
		part q : Q;
	}
`

// TestKernelCollectionOpenReadStaysOpen pins that a model-level read stops where a subsetter's
// count is open, at the subsetter's fewest, while a read of the object as held fills the subsetter.
func TestKernelCollectionOpenReadStaysOpen(t *testing.T) {
	idx, _, ctx := buildRuntimeWithLibraries(t, "<test>", parseAndBuild(t, kernelCollectionOpenModel))
	matches := idx.LookupQualified("test::q")
	if len(matches) != 1 {
		t.Fatalf("test::q: %d matching symbols, want 1", len(matches))
	}
	inst, err := ctx.Instantiate(matches[0])
	if err != nil {
		t.Fatalf("Instantiate: %v", err)
	}
	fv, open, err := inst.openFeatureValue(ctx, "suboccurrences")
	if err != nil {
		t.Fatalf("openFeatureValue(suboccurrences): %v", err)
	}
	if fewest := max(int64(len(open.Contributed)), open.AtLeast); !open.Stopped || fewest != 1 {
		t.Fatalf("open read of suboccurrences = %+v, want stopped at no fewer than 1: children's count is open", *open)
	}
	if fv.Materialized {
		t.Fatalf("open read of suboccurrences reports a materialized value")
	}
	held, err := inst.GetFeatureValue(ctx, "suboccurrences")
	if err != nil {
		t.Fatalf("GetFeatureValue(suboccurrences): %v", err)
	}
	if got := len(elementsOf(held.HeldValue())); got != 1 {
		t.Fatalf("suboccurrences as held: %d elements, want 1 (children made up to its lower bound)", got)
	}
}
