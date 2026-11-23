package queryexec

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// The FeatureTyping of a type argument's parameter (`@M`, `x istype T`, `x as T`) reads the id the
// conversion mints off the parameter (`_ft0`), a `filter` condition's included.
func TestTypeArgumentTypingReadsTheConversionID(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
    metadata def M;
    attribute def A;
    attribute b;
    attribute x = b istype A;
    calc c { b as A }
    private filter @M;
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	written := apiJSONClasses(t, fixture)
	var got []string
	seen := map[*symbols.Symbol]bool{}
	var walk func(sym *symbols.Symbol)
	walk = func(sym *symbols.Symbol) {
		if seen[sym] {
			return
		}
		seen[sym] = true
		if sym.Implicit != nil && sym.Implicit.Form == symbols.ImplicitTypeArgumentTyping {
			id, err := ElementIDOf(read, fixture.index, sym)
			if err != nil {
				t.Fatalf("type argument typing id: %v", err)
			}
			if written[id] != "FeatureTyping" {
				t.Errorf("the conversion writes %q as %q, want a FeatureTyping", id, written[id])
			}
			got = append(got, id)
		}
		for _, rel := range fixture.model.OwnedRelationships(sym) {
			walk(rel)
		}
		if related, ok := fixture.model.ReflectiveElements(sym, "ownedRelatedElement"); ok {
			for _, element := range related {
				walk(element)
			}
		}
	}
	observatory := symbols.PreferDeclared(fixture.index.LookupQualified("Observatory"))
	if len(observatory) != 1 {
		t.Fatalf("lookup Observatory: got %d symbols", len(observatory))
	}
	walk(observatory[0])
	want := []string{"Observatory__x_pvalue_pin1_ft0", "Observatory__c___400_pin1_ft0", "Observatory___405_pfilter_pin0_ft0"}
	if !slices.Equal(got, want) {
		t.Errorf("type argument typing ids = %q, want %q", got, want)
	}
}
