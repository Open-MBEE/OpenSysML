package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// TestMetadataBodyRedefinitionHasItsElementID: the Redefinition a metadata body's
// `mission = …` implies is identified as the conversion mints it, `_rd0` of the body feature.
func TestMetadataBodyRedefinitionHasItsElementID(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	metadata def Heritage { attribute mission : String; }
	part def Telescope;
	part telescope : Telescope;
	@Heritage about telescope { mission = "Hubble"; }
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) (string, error) { return ElementIDOf(read, fixture.index, sym) }
	members, _ := fixture.model.ReflectiveElements(fixture.symbol(t, "telescope").OwnerScope.Owner(), "ownedMember")
	checked := 0
	for _, member := range members {
		if member.Kind != symbols.SymbolMetadataUsage {
			continue
		}
		features, _ := fixture.model.ReflectiveElements(member, "ownedFeature")
		if len(features) != 1 {
			t.Fatalf("metadata usage owns %d features, want the mission restatement", len(features))
		}
		featureID, err := elementID(features[0])
		if err != nil || featureID == "" {
			t.Fatalf("body feature id: %q, %v", featureID, err)
		}
		rels, _ := fixture.model.ReflectiveElements(features[0], "ownedRelationship")
		if len(rels) == 0 {
			t.Fatal("body feature owns no relationship, want its Redefinition first")
		}
		if meta := fixture.model.MetaclassOf(rels[0]); meta == nil || meta.Name != "Redefinition" {
			t.Fatalf("body feature ownedRelationship = %v, want its Redefinition first", rels)
		}
		if id, err := elementID(rels[0]); err != nil || id != featureID+"_rd0" {
			t.Errorf("Redefinition id %q, %v; want %q", id, err, featureID+"_rd0")
		}
		checked++
	}
	if checked != 1 {
		t.Fatalf("checked %d metadata usages, want 1", checked)
	}
}
