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

// TestSubsetsTransitivelyWalksDeclaredImplicitAndRedefined pins the walk a derived
// Kernel collection read follows: a declared `:>`, the implicit subsetting a nested
// usage states, and a redefinition, chained through library features out of the shape.
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
