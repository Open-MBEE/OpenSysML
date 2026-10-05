package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

func TestLinkActionsKeepTheirPinsAsApproximateActions(t *testing.T) {
	r := migrateFixtureFile(t, "link_actions")
	wantClean(t, "link_actions.sysml", r)

	tests := []struct {
		id, name, input, output, note string
	}{
		{"_clear", "clearAssociation", "clearInput[1]", "clearResult[1]", "SysML v2 has no link action: the ClearAssociationAction is written as an action with its pins and does not clear the link"},
		{"_create", "createLink", "createInput[1]", "createResult[1]", "SysML v2 has no link action: the CreateLinkAction is written as an action with its pins and does not create the link"},
		{"_createObject", "createLinkObject", "createObjectInput[1]", "createObjectResult[1]", "SysML v2 has no link action: the CreateLinkObjectAction is written as an action with its pins and does not create the link"},
		{"_destroy", "destroyLink", "destroyInput[1]", "destroyResult[1]", "SysML v2 has no link action: the DestroyLinkAction is written as an action with its pins and does not destroy the link"},
		{"_read", "readLink", "readInput[1]", "readResult[1]", "SysML v2 has no link action: the ReadLinkAction is written as an action with its pins and does not read the link"},
		{"_readEnd", "readLinkObjectEnd", "readEndInput[1]", "readEndResult[1]", "SysML v2 has no link action: the ReadLinkObjectEndAction is written as an action with its pins and does not read the link"},
		{"_readQualifier", "readLinkObjectEndQualifier", "readQualifierInput[1]", "readQualifierResult[1]", "SysML v2 has no link action: the ReadLinkObjectEndQualifierAction is written as an action with its pins and does not read the link"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			entries := entriesFor(r, tc.id)
			if len(entries) != 1 || entries[0].Verdict != migrate.Approximated || !strings.Contains(entries[0].Note, tc.note) {
				t.Fatalf("entries for %s = %+v, want one approximated entry with note %q", tc.id, entries, tc.note)
			}
			for _, want := range []string{
				"ref action " + tc.name + " {",
				"in " + tc.input + ";",
				"out " + tc.output + ";",
			} {
				if !strings.Contains(string(r.Notation), want) {
					t.Errorf("notation lacks %q:\n%s", want, r.Notation)
				}
			}
			if strings.Contains(string(r.Notation), "\n    action "+tc.name+" {") {
				t.Errorf("%s is written as an executable action:\n%s", tc.name, r.Notation)
			}
		})
	}
	if strings.Contains(string(r.Notation), "not migrated:") {
		t.Errorf("link action notation contains a placeholder comment:\n%s", r.Notation)
	}
}
