package queryexec

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/query"
)

// The ownership walk of Descendants(relationships = true): KerML 8.3.2.1 ownedRelationship and
// ownedRelatedElement, so the relationship elements and the implicit elements they own are rows.
const relationshipsBody = `
part def Mirror {
	attribute diameter : Real = 2.4;
}
part telescope {
	part primary : Mirror;
	part secondary : Mirror;
	connect primary to secondary;
}
`

func descendantsOfKind(t *testing.T, relationships bool, metaclass string) int {
	t.Helper()
	fixture := loadExecutionFixture(t, relationshipsBody+fmt.Sprintf(`
calc def Kind :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, relationships = %t), type = "%s", exact = true),
		properties = ("elementId")
	)
}
`, relationships, metaclass))
	result, err := fixture.execute(t, "Kind", Bindings{
		"root": {ElementValue(fixture.symbol(t, "telescope"))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Kind(%t, %s): %v", relationships, metaclass, err)
	}
	return len(result.Rows())
}

func TestDescendantsRelationshipsReachOwnedRelationships(t *testing.T) {
	for _, tc := range []struct {
		relationships bool
		metaclass     string
		want          int
	}{
		{false, "PartUsage", 2},
		{false, "ConnectionUsage", 1},
		{false, "FeatureMembership", 0},
		{false, "FeatureTyping", 0},
		{false, "EndFeatureMembership", 0},
		{false, "ReferenceUsage", 2},
		{true, "PartUsage", 2},
		{true, "ConnectionUsage", 1},
		{true, "FeatureMembership", 3},
		{true, "FeatureTyping", 2},
		{true, "EndFeatureMembership", 2},
		{true, "ReferenceUsage", 2},
	} {
		if got := descendantsOfKind(t, tc.relationships, tc.metaclass); got != tc.want {
			t.Errorf("Descendants(relationships = %t) %s rows = %d, want %d", tc.relationships, tc.metaclass, got, tc.want)
		}
	}
}

// An exact walk keeps an element under the metaclass reflection derives for it (Model.MetaclassOf): a chain
// end's chain Feature is a Feature, a conjugated port definition a ConjugatedPortDefinition, a transition a TransitionUsage.
func TestDescendantsRelationshipsExactKeepsTheReflectiveMetaclass(t *testing.T) {
	fixture := loadExecutionFixture(t, `
package Probe {
	part def A { port o; }
	part def B { port i; }
	port def Optical;
	state def Machine {
		state idle;
		state busy;
		transition idle_to_busy first idle then busy;
	}
	part telescope {
		part a : A;
		part b : B;
		connection c connect a.o to b.i;
	}
}
calc def Kind :> Query {
	in root : Element;
	in type : String;
	Project(
		source = WhereType(source = Descendants(source = root, relationships = true), type = type, exact = true),
		properties = ("elementId")
	)
}
calc def Ends :> Query {
	in root : Element;
	in type : String;
	Project(
		source = WhereType(source = Descendants(source = root, relationships = true), type = type, exact = true),
		properties = ("elementId", "isEnd")
	)
}
`)
	for _, tc := range []struct {
		metaclass string
		want      int
		isEnd     string
	}{
		// The connection's two ends, and the two the transition's succession owns.
		{"ReferenceUsage", 4, "true"},
		{"Feature", 2, "false"},
		{"PortDefinition", 1, ""},
		{"ConjugatedPortDefinition", 1, ""},
		{"ActionUsage", 0, ""},
		{"TransitionUsage", 1, ""},
	} {
		queryName := "Kind"
		if tc.isEnd != "" {
			queryName = "Ends"
		}
		result, err := fixture.execute(t, queryName, Bindings{
			"root": {ElementValue(fixture.symbol(t, "Probe"))},
			"type": {StringValue(tc.metaclass)},
		}, Options{})
		if err != nil {
			t.Fatalf("execute %s(%s): %v", queryName, tc.metaclass, err)
		}
		if got := len(result.Rows()); got != tc.want {
			t.Errorf("exact %s rows = %d, want %d", tc.metaclass, got, tc.want)
		}
		// Each row's own @type is the metaclass it was kept under.
		for _, row := range result.Rows() {
			if got := query.MetamodelTypeNameOf(row.Element().Declaration()); got != tc.metaclass {
				t.Errorf("exact %s row %v @type = %q, want %s", tc.metaclass, row.Element(), got, tc.metaclass)
			}
		}
		if tc.isEnd == "" {
			continue
		}
		// The two ends are end features, the two chain Features they reference are not.
		for _, row := range metaclassPropertyCells(t, result) {
			if !strings.HasSuffix(row, " "+tc.isEnd) {
				t.Errorf("exact %s row %q, want isEnd %s", tc.metaclass, row, tc.isEnd)
			}
		}
	}
}
