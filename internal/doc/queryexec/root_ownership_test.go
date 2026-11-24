package queryexec

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A root-level element reads no owner, owning namespace, membership or relationship: the document's
// RootNamespace and the memberships it owns are elements no conversion states. A nested one keeps its own.
func TestExecuteRootLevelElementReadsNoOwnership(t *testing.T) {
	fixture := loadExecutionSource(t, `
package W { part def A; }
part def RootDef;
part rootPart : RootDef;
attribute def RootAttribute;
action def RootAction;
requirement def RootRequirement;
comment RootComment /* a root comment */
package Observatory {`+executionImports+`
	calc def Ownership :> Query {
		in root : Element;
		Project(source = root, properties = ("owner", "owningNamespace", "owningMembership", "owningRelationship"),
			columns = (Column(name = "Path", cell = { in row : Element; row.owningNamespace.ownedMembership })))
	}
}`)
	ownership := func(name string) []int {
		t.Helper()
		matches := symbols.PreferDeclared(fixture.index.LookupQualified(name))
		if len(matches) != 1 {
			t.Fatalf("lookup %s: got %d symbols", name, len(matches))
		}
		result, err := fixture.execute(t, "Ownership", Bindings{"root": {ElementValue(matches[0])}}, Options{})
		if err != nil {
			t.Fatalf("Ownership of %s: %v", name, err)
		}
		if len(result.Rows()) != 1 {
			t.Fatalf("Ownership of %s: %d rows, want 1", name, len(result.Rows()))
		}
		var counts []int
		for _, cell := range result.Rows()[0].Cells() {
			counts = append(counts, len(cell.Values()))
		}
		return counts
	}
	for _, name := range []string{"W", "RootDef", "rootPart", "RootAttribute", "RootAction", "RootRequirement", "RootComment", "Observatory"} {
		for column, count := range ownership(name) {
			if count != 0 {
				t.Errorf("%s: cell %d holds %d values, want none", name, column, count)
			}
		}
	}
	nested := ownership("W::A")
	for column, want := range []int{1, 1, 1, 1, 1} {
		if nested[column] != want {
			t.Errorf("W::A: cell %d holds %d values, want %d", column, nested[column], want)
		}
	}
}
