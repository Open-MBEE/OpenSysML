package queryexec

import (
	"strconv"
	"strings"
	"testing"
)

const treeBody = `
package Site {
	part def Station {
		part pumps : Pump[*];
	}
	part def Pump;
	package North {
		part def Inlet;
		package Deep {
			part def Sump;
		}
	}
	part def Outlet;
	individual part def Run1 :> Station {
		individual part :>> pumps : Run1Pump;
	}
	individual part def Run1Pump :> Pump {
		individual part seal : Run1Seal;
	}
	individual part def Run1Seal;
	individual part def Run2Pump :> Pump;
}
`

const treeQueries = `
calc def Nested :> Query {
	in root : Element;
	Tree(source = WhereType(source = Descendants(source = root), type = "PartDefinition"))
}
calc def Complete :> Query {
	in root : Element;
	Tree(
		source = WhereType(source = Descendants(source = root), type = "PartDefinition"),
		ancestors = Descendants(source = root))
}
calc def NestedNames :> Query {
	in root : Element;
	Project(
		source = OrderBy(
			source = Tree(source = IndividualDefinitions(root = root)),
			property = "name", direction = "descending", missing = "last", multiple = "first"),
		properties = ("name"))
}
calc def Individuals :> Query {
	in root : Element;
	Project(
		source = Tree(source = IndividualDefinitions(root = root)),
		properties = ("name"))
}
calc def IndividualDefinitions :> Query {
	in root : Element;
	WhereFeature(
		source = WhereType(source = Descendants(source = root), type = "PartDefinition"),
		'feature' = "isIndividual", operator = "=", value = "true")
}
calc def Subtracted :> Query {
	in root : Element;
	in exclude : Element[0..*] ordered;
	Except(
		source = Tree(source = WhereType(source = Descendants(source = root), type = "PartDefinition")),
		exclude = exclude)
}
`

// treeOutline renders rows as "depth:name" in order.
func treeOutline(rows *RowSet) string {
	var out []string
	for _, row := range rows.Rows() {
		sym, _ := row.Element().Element()
		out = append(out, strconv.FormatInt(row.Depth(), 10)+":"+sym.Name)
	}
	return strings.Join(out, " ")
}

// Tree nests every row under the nearest row owning it, skipping owners that
// are no rows, in pre-order with siblings in source order; the individuals
// whose parts are typed by other individuals nest by that typing since their
// definitions own nothing of each other.
func TestExecuteTreeNestsRowsByContainment(t *testing.T) {
	fixture := loadExecutionFixture(t, treeBody+treeQueries)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "Site"))}}

	nested, err := fixture.execute(t, "Nested", root, Options{})
	if err != nil {
		t.Fatalf("Nested: %v", err)
	}
	// The packages North and Deep are no rows: Inlet and Sump are top-level,
	// where the walk listed them; Run1Pump nests under Run1 (its pumps part),
	// Run1Seal under Run1Pump.
	want := "0:Station 0:Pump 0:Outlet 0:Run1 1:Run1Pump 2:Run1Seal 0:Run2Pump 0:Inlet 0:Sump"
	if got := treeOutline(nested); got != want {
		t.Fatalf("Nested = %q, want %q", got, want)
	}

	// With the scope's descendants as ancestors the packages join as levels,
	// listed after the rows themselves.
	complete, err := fixture.execute(t, "Complete", root, Options{})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	want = "0:Station 0:Pump 0:Outlet 0:Run1 1:Run1Pump 2:Run1Seal 0:Run2Pump 0:North 1:Inlet 1:Deep 2:Sump"
	if got := treeOutline(complete); got != want {
		t.Fatalf("Complete = %q, want %q", got, want)
	}
}

// The depth survives projection, ordering and subtraction, and a flat query
// reports every row at depth 0.
func TestExecuteTreeDepthSurvivesLaterOperations(t *testing.T) {
	fixture := loadExecutionFixture(t, treeBody+treeQueries)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "Site"))}}

	individuals, err := fixture.execute(t, "Individuals", root, Options{})
	if err != nil {
		t.Fatalf("Individuals: %v", err)
	}
	if got, want := treeOutline(individuals), "0:Run1 1:Run1Pump 2:Run1Seal 0:Run2Pump"; got != want {
		t.Fatalf("Individuals = %q, want %q", got, want)
	}
	rows := individuals.Rows()
	if name, _ := rows[1].Cells()[0].Values()[0].String(); name != "Run1Pump" {
		t.Fatalf("projected cell of the nested row = %q", name)
	}

	ordered, err := fixture.execute(t, "NestedNames", root, Options{})
	if err != nil {
		t.Fatalf("NestedNames: %v", err)
	}
	if got, want := treeOutline(ordered), "0:Run2Pump 2:Run1Seal 1:Run1Pump 0:Run1"; got != want {
		t.Fatalf("NestedNames = %q, want %q", got, want)
	}

	subtracted, err := fixture.execute(t, "Subtracted", Bindings{
		"root":    {ElementValue(fixture.symbol(t, "Site"))},
		"exclude": {ElementValue(fixture.symbol(t, "Site::Run1Pump")), ElementValue(fixture.symbol(t, "Site::Station"))},
	}, Options{})
	if err != nil {
		t.Fatalf("Subtracted: %v", err)
	}
	// Run1Seal loses its parent Run1Pump and nests directly under Run1.
	if got, want := treeOutline(subtracted), "0:Pump 0:Outlet 0:Run1 1:Run1Seal 0:Run2Pump 0:Inlet 0:Sump"; got != want {
		t.Fatalf("Subtracted = %q, want %q", got, want)
	}

	flat, err := fixture.execute(t, "Nested", Bindings{"root": {ElementValue(fixture.symbol(t, "Site::North"))}}, Options{})
	if err != nil {
		t.Fatalf("Nested North: %v", err)
	}
	if got, want := treeOutline(flat), "0:Inlet 0:Sump"; got != want {
		t.Fatalf("Nested North = %q, want %q", got, want)
	}
}
