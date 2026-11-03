package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// TestConnectorEndChainFeatureHasItsOwnElementID: the chain Feature a chain end's ReferenceSubsetting
// denotes (`connect a.o to b.i`) is identified as the conversion mints it, under the end at `chain`.
func TestConnectorEndChainFeatureHasItsOwnElementID(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	part def A { port o; }
	part def B { port i; }
	part a : A;
	part b : B;
	connection c connect a.o to b.i;
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) (string, error) { return ElementIDOf(read, fixture.index, sym) }
	connection := fixture.symbol(t, "c")
	ends, _ := fixture.model.ReflectiveElements(connection, "connectorEnd")
	if len(ends) != 2 {
		t.Fatalf("connectorEnd = %d ends, want 2", len(ends))
	}
	for i, end := range ends {
		endID, err := elementID(end)
		if err != nil || endID == "" {
			t.Fatalf("end %d id: %q, %v", i, endID, err)
		}
		relationships := fixture.model.ImplicitRelationships(end)
		if len(relationships) != 1 {
			t.Fatalf("end %d owns %d relationships, want its ReferenceSubsetting", i, len(relationships))
		}
		targets, _ := fixture.model.ReflectiveElements(relationships[0], "target")
		if len(targets) != 1 || targets[0].Chain == nil {
			t.Fatalf("end %d: target = %v, want the chain Feature", i, targets)
		}
		chainID, err := elementID(targets[0])
		if err != nil {
			t.Fatalf("end %d chain id: %v", i, err)
		}
		if chainID != endID+"_pchain" || !strings.HasSuffix(endID, "_pend"+string(rune('0'+i))) {
			t.Errorf("end %d: chain Feature id %q, want %q", i, chainID, endID+"_pchain")
		}
	}
}

// TestConnectorEndRedefinitionChainIsNotTheAttachment: a chained redefinition the end also owns
// (`src ::> a.o :>> a.q`) is not the attachment's chain Feature, so it is not identified as one.
func TestConnectorEndRedefinitionChainIsNotTheAttachment(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	part def A { port o; port q; }
	part a : A;
	connection def K { end ref src; }
	connection c : K connect (src ::> a.o :>> a.q);
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	ends, _ := fixture.model.ReflectiveElements(fixture.symbol(t, "c"), "connectorEnd")
	if len(ends) != 1 {
		t.Fatalf("connectorEnd = %d ends, want 1", len(ends))
	}
	endID, _ := ElementIDOf(read, fixture.index, ends[0])
	ids := map[string]string{}
	for _, rel := range fixture.model.ImplicitRelationships(ends[0]) {
		targets, _ := fixture.model.ReflectiveElements(rel, "target")
		if len(targets) != 1 || targets[0].Chain == nil {
			t.Fatalf("%s: target = %v, want a chain Feature", rel.Implicit.Kind, targets)
		}
		id, err := ElementIDOf(read, fixture.index, targets[0])
		if err != nil {
			t.Fatalf("%s chain id: %v", rel.Implicit.Kind, err)
		}
		ids[rel.Implicit.Kind.String()] = id
	}
	if got := ids["references"]; got != endID+"_pchain" {
		t.Errorf("attachment chain id %q, want %q", got, endID+"_pchain")
	}
	if got, ok := ids["redefines"]; !ok || got == endID+"_pchain" {
		t.Errorf("redefinition chain id %q (present %v), want one other than the attachment's", got, ok)
	}
}
