package queryexec

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// A metadata usage's FeatureTyping, annotation and body membership read the ids the conversion
// mints: `<metadata>_ft0`, `_an0` and the body member's owning membership.
func TestMetadataUsageTypingReadsTheConversionID(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
    metadata def Heritage { attribute mission : String; }
    part telescope;
    @Heritage about telescope { mission = "Hubble"; }
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) string {
		id, err := ElementIDOf(read, fixture.index, sym)
		if err != nil {
			t.Fatalf("%v id: %v", sym.Decl, err)
		}
		return id
	}
	members, _ := fixture.model.ReflectiveElements(fixture.symbol(t, "telescope").Owner(), "ownedMember")
	var metadata *symbols.Symbol
	for _, member := range members {
		if member.Name == "" {
			metadata = member
		}
	}
	if metadata == nil {
		t.Fatal("no metadata usage among Observatory's members")
	}
	owned, _ := fixture.model.ReflectiveElements(metadata, "ownedRelationship")
	if got, want := strings.Join(elementIDs(elementID, owned), " "), "Observatory___402_ft0 Observatory___402_an0 Observatory___402__mission_om"; got != want {
		t.Fatalf("the metadata usage's ownedRelationship ids = %q, want %q", got, want)
	}
}
