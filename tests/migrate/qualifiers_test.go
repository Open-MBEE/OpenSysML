package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

func TestAssociationEndQualifiers(t *testing.T) {
	r := migrateFixtureFile(t, "association_qualifiers")
	wantClean(t, "association_qualifiers.sysml", r)
	notation := string(r.Notation)
	for _, line := range []string{
		"end a : A {",
		"attribute ownedKey : Key;",
		"end b : B {",
		"attribute memberKey : Key;",
		"ref occurrence b : B {",
	} {
		wantLine(t, r.Notation, line)
	}
	if got := strings.Count(notation, "attribute memberKey : Key;"); got != 2 {
		t.Errorf("memberKey qualifier count = %d, want 2\n%s", got, notation)
	}
	wantNote(t, r, "_ownedQualifier", migrate.Mapped, "")
	wantNote(t, r, "_classifierQualifier", migrate.Mapped, "")
}
