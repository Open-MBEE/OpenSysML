package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// TestAboutAnnotationHasItsOwnElementID: the Annotation an `about` states is identified
// as the conversion mints it — the annotating element's id and `_an<i>`, one per target.
func TestAboutAnnotationHasItsOwnElementID(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	metadata def Heritage;
	part def Telescope;
	part telescope : Telescope;
	part camera : Telescope;
	@Heritage about telescope, camera;
	comment note about camera /* one */
	metadata m : Heritage about camera, telescope;
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	elementID := func(sym *symbols.Symbol) (string, error) { return ElementIDOf(read, fixture.index, sym) }
	members, _ := fixture.model.ReflectiveElements(fixture.symbol(t, "telescope").OwnerScope.Owner(), "ownedMember")
	seen := 0
	for _, member := range members {
		if member.Kind != symbols.SymbolMetadataUsage && member.Kind != symbols.SymbolComment {
			continue
		}
		ownerID, err := elementID(member)
		if err != nil || ownerID == "" {
			t.Fatalf("%v id: %q, %v", member.Decl, ownerID, err)
		}
		annotations, _ := fixture.model.ReflectiveElements(member, "ownedAnnotatingRelationship")
		if len(annotations) == 0 {
			t.Fatalf("%s: no ownedAnnotatingRelationship", ownerID)
		}
		for i, annotation := range annotations {
			want := ownerID + "_an" + string(rune('0'+i))
			if id, err := elementID(annotation); err != nil || id != want {
				t.Errorf("%s: Annotation %d id %q, %v; want %q", ownerID, i, id, err, want)
			}
			seen++
		}
	}
	if seen != 5 {
		t.Fatalf("checked %d annotations, want 5", seen)
	}
}
