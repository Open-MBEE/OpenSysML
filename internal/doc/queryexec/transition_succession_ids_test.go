package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// The succession a transition owns reads the elementId the conversion writes for it,
// `<transition id>_succession`, its owning membership `<transition id>_succession_om`.
func TestTransitionSuccessionReadsTheConversionIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	state def S {
		state idle;
		state busy;
		transition go first idle then busy;
	}
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) string {
		id, err := ElementIDOf(read, fixture.index, sym)
		if err != nil {
			t.Fatalf("%v id: %v", sym.Decl, err)
		}
		return id
	}
	transition := fixture.symbol(t, "S::go")
	successions, _ := fixture.model.ReflectiveElements(transition, "succession")
	if len(successions) != 1 || elementID(successions[0]) != "Observatory__S__go_succession" {
		t.Fatalf("succession ids = %d, %q; want Observatory__S__go_succession", len(successions), elementIDs(elementID, successions))
	}
	owned, _ := fixture.model.ReflectiveElements(transition, "ownedRelationship")
	for _, rel := range owned {
		if elementID(rel) == "Observatory__S__go_succession_om" {
			return
		}
	}
	t.Fatalf("ownedRelationship ids = %q, want Observatory__S__go_succession_om among them", elementIDs(elementID, owned))
}

func elementIDs(id func(*symbols.Symbol) string, syms []*symbols.Symbol) []string {
	out := make([]string, 0, len(syms))
	for _, sym := range syms {
		out = append(out, id(sym))
	}
	return out
}
