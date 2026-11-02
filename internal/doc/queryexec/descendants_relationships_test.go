package queryexec

import (
	"fmt"
	"testing"
)

// The ownership walk of Descendants(relationships = true): KerML 8.3.2.1 ownedRelationship and
// ownedRelatedElement, so the relationship elements and the implicit elements they own are rows.
const relationshipsBody = `
part def Mirror {
	attribute diameter : Real = 2.4;
}
part telescope {
	part primary : Mirror;
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
		{false, "PartUsage", 1},
		{false, "FeatureMembership", 0},
		{false, "FeatureTyping", 0},
		{true, "PartUsage", 1},
		{true, "FeatureMembership", 1},
		{true, "FeatureTyping", 1},
	} {
		if got := descendantsOfKind(t, tc.relationships, tc.metaclass); got != tc.want {
			t.Errorf("Descendants(relationships = %t) %s rows = %d, want %d", tc.relationships, tc.metaclass, got, tc.want)
		}
	}
}
