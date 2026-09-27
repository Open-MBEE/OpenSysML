package queryexec

import (
	"strings"
	"testing"
)

const whereTextQueries = `
calc def KeyRows :> Query {
	in root : Element;
	WhereText(source = Tagged(root = root), columns = ("Key"), operator = "matches", value = "(?i)^key$")
}
calc def AnyDriver :> Query {
	in root : Element;
	WhereText(source = Tagged(root = root), operator = "contains", value = "Driver")
}
calc def NoSuchColumn :> Query {
	in root : Element;
	WhereText(source = Tagged(root = root), columns = ("Owner"), operator = "=", value = "x")
}
calc def Unprojected :> Query {
	in root : Element;
	WhereText(source = Descendants(source = root), operator = "=", value = "x")
}
calc def Generals :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root), type = "PartDefinition"),
		properties = ("name", "general"))
}
calc def UsageGenerals :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root), type = "PartUsage"),
		properties = ("name", "general"))
}
`

// WhereText searches the text of projected cells: the named columns, or every
// column when none is named, each value of a multi-valued cell on its own.
func TestExecuteWhereTextSearchesProjectedCells(t *testing.T) {
	fixture := loadExecutionFixture(t, metadataColumnsBody+metadataColumnsQueries+whereTextQueries)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "Reqs"))}}

	keyed, err := fixture.execute(t, "KeyRows", root, Options{})
	if err != nil {
		t.Fatalf("KeyRows: %v", err)
	}
	if got := cellTexts(t, keyed, 0); strings.Join(got, ",") != "8" {
		t.Fatalf("KeyRows = %v", got)
	}
	if columns := keyed.Columns(); len(columns) != 4 || columns[1].Name() != "Key" {
		t.Fatalf("KeyRows keeps the source columns, got %d", len(columns))
	}

	drivers, err := fixture.execute(t, "AnyDriver", root, Options{})
	if err != nil {
		t.Fatalf("AnyDriver: %v", err)
	}
	if got := cellTexts(t, drivers, 0); strings.Join(got, ",") != "8" {
		t.Fatalf("AnyDriver = %v", got)
	}

	_, err = fixture.execute(t, "NoSuchColumn", root, Options{})
	if unknown := executionError(t, err, ErrorUnknownProperty); unknown.Property != "Owner" {
		t.Fatalf("NoSuchColumn = %v", unknown)
	}
	_, err = fixture.execute(t, "Unprojected", root, Options{})
	if invalid := executionError(t, err, ErrorInvalidArgument); invalid.Parameter != "source" {
		t.Fatalf("Unprojected = %v", invalid)
	}
}

// The `general` property lists the definitions a definition specializes and
// the types a usage is typed by, as elements.
func TestExecuteProjectsGenerals(t *testing.T) {
	fixture := loadExecutionFixture(t, treeBody+whereTextQueries+metadataColumnsQueries)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "Site"))}}
	for name, want := range map[string]string{
		"Generals":      "Station=,Pump=,Outlet=,Run1=Station,Run1Pump=Pump,Run1Seal=,Run2Pump=Pump,Inlet=,Sump=",
		"UsageGenerals": "pumps=Pump,pumps=Run1Pump,seal=Run1Seal",
	} {
		result, err := fixture.execute(t, name, root, Options{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var got []string
		for _, row := range result.Rows() {
			cells := row.Cells()
			got = append(got, cellText(cells[0])+"="+cellText(cells[1]))
		}
		if strings.Join(got, ",") != want {
			t.Fatalf("%s = %v", name, got)
		}
	}
}
