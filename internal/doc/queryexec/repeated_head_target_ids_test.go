package queryexec

import (
	"slices"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/export"
)

// A head clause naming one target twice (`: A, A`) states two relationships, and the query leg
// reads each by the id the conversion mints: the repeat after the kind's distinct targets.
func TestRepeatedHeadTargetsHaveDistinctElementIDs(t *testing.T) {
	fixture := loadExecutionSource(t, `package Observatory {
	classifier A;
	feature g : A;
	classifier B specializes A, A;
	feature h : A, A;
	feature s subsets g, g;
	feature r redefines g, g;
	classifier U unions A, A;
	classifier I intersects A, A;
	classifier D differences A, A;
	classifier C disjoint from A, A;
	feature f featured by A, A;
	feature x references g, g;
	feature c crosses g, g;
	feature t : A, A, B;
}`)
	read := export.NewElementIDReader([]export.ElementIDDocument{{File: fixture.file, Root: fixture.root}})
	for name, suffixes := range map[string][]string{
		"B": {"_sc0", "_sc1"},
		"h": {"_ft0", "_ft1"},
		"s": {"_ss0", "_ss1"},
		"r": {"_rd0", "_rd1"},
		"U": {"_un0", "_un1"},
		"I": {"_is0", "_is1"},
		"D": {"_df0", "_df1"},
		"C": {"_dj0", "_dj1"},
		"f": {"_tf0", "_tf1"},
		"x": {"_rs0", "_rs1"},
		"c": {"_cs0", "_cs1"},
		"t": {"_ft0", "_ft2", "_ft1"},
	} {
		owner := fixture.symbol(t, name)
		ownerID, err := ElementIDOf(read, fixture.index, owner)
		if err != nil || ownerID == "" {
			t.Fatalf("%s id: %q, %v", name, ownerID, err)
		}
		relationships, _ := fixture.model.ReflectiveElements(owner, "ownedRelationship")
		var got, want []string
		for _, relationship := range relationships {
			id, err := ElementIDOf(read, fixture.index, relationship)
			if err != nil {
				t.Fatalf("%s: relationship id: %v", name, err)
			}
			got = append(got, id)
		}
		for _, suffix := range suffixes {
			want = append(want, ownerID+suffix)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s: ownedRelationship ids %v, want %v", name, got, want)
		}
	}
}
