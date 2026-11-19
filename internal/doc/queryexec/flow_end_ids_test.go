package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// A flow's FlowEnds and everything they own read the ids the conversion mints: `<flow>_pend0`, `_pend0_om`,
// `_pend0_prs`, `_pend0_pff`, `_pend0_pff_om`, `_pend0_pff_rd0`.
func TestFlowEndsReadTheConversionIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
    item def Image;
    part acquire { out item image : Image; }
    part readout { in item image : Image; }
    flow capture from acquire.image to readout.image;
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) string {
		id, err := ElementIDOf(read, fixture.index, sym)
		if err != nil {
			t.Fatalf("%v id: %v", sym.Decl, err)
		}
		return id
	}
	flow := fixture.symbol(t, "capture")
	owned, _ := fixture.model.ReflectiveElements(flow, "ownedRelationship")
	if got, want := strings.Join(elementIDs(elementID, owned), " "), "Observatory__capture_pend0_om Observatory__capture_pend1_om"; got != want {
		t.Fatalf("the flow's ownedRelationship ids = %q, want %q", got, want)
	}
	ends, _ := fixture.model.ReflectiveElements(flow, "connectorEnd")
	if got, want := strings.Join(elementIDs(elementID, ends), " "), "Observatory__capture_pend0 Observatory__capture_pend1"; got != want {
		t.Fatalf("the flow's connectorEnd ids = %q, want %q", got, want)
	}
	for i, end := range ends {
		prefix := "Observatory__capture_pend" + string(rune('0'+i))
		owned, _ := fixture.model.ReflectiveElements(end, "ownedRelationship")
		if got, want := strings.Join(elementIDs(elementID, owned), " "), prefix+"_prs "+prefix+"_pff_om"; got != want {
			t.Fatalf("end %d's ownedRelationship ids = %q, want %q", i, got, want)
		}
		features, _ := fixture.model.ReflectiveElements(end, "ownedFeature")
		if got, want := strings.Join(elementIDs(elementID, features), " "), prefix+"_pff"; got != want {
			t.Fatalf("end %d's ownedFeature ids = %q, want %q", i, got, want)
		}
		redefinitions, _ := fixture.model.ReflectiveElements(features[0], "ownedRelationship")
		if got, want := strings.Join(elementIDs(elementID, redefinitions), " "), prefix+"_pff_rd0"; got != want {
			t.Fatalf("end %d's feature's ownedRelationship ids = %q, want %q", i, got, want)
		}
	}
}
