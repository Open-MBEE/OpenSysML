package queryexec

import (
	"strings"
	"testing"
)

const metadataColumnsBody = `
package Reqs {
	metadata def Properties {
		attribute key : String;
		attribute driving : String[0..*] ordered;
		attribute rank : Rank[0..*] ordered;
	}
	metadata def Reviewed :> Properties;
	enum def Rank { high; low; }
	requirement def <'8'> Alignment {
		@Properties { key = "Key"; driving = ("Not Driving", "Driver - Cost"); rank = (Rank::high, Rank::low); }
	}
	requirement def <'9'> Blank {
		@Reviewed { key = ""; }
	}
	requirement def <'10'> Plain;
}
`

const metadataColumnsQueries = `
calc def Tagged :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root), type = "RequirementDefinition"),
		properties = ("shortName"),
		columns = (
			Column(name = "Key", expression = Reqs::Properties::key ?? ""),
			Column(name = "Driving", expression = Reqs::Properties::driving ?? ""),
			Column(name = "Rank", expression = Reqs::Properties::rank ?? "")))
}
calc def Keyed :> Query {
	in root : Element;
	WhereFeature(source = Tagged(root = root), 'feature' = "Key", operator = "matches", value = "(?i)key")
}
calc def ByKey :> Query {
	in root : Element;
	OrderBy(
		source = WhereType(source = Descendants(source = root), type = "RequirementDefinition"),
		property = "Observatory::Reqs::Properties::key", direction = "ascending", missing = "last", multiple = "first")
}
`

// cellText joins the values of a projected cell.
func cellText(cell Cell) string {
	var parts []string
	for _, value := range cell.Values() {
		if sym, ok := value.Element(); ok {
			parts = append(parts, sym.Name)
			continue
		}
		text, _ := value.String()
		parts = append(parts, text)
	}
	return strings.Join(parts, "; ")
}

// A feature of a metadata def, referenced by a column expression or named as
// a property, reads what the row's annotations of that def — or of a def
// specializing it — bind the feature to: one value per element of a
// sequence, an element of the model for a reference, nothing when unannotated.
func TestExecuteMetadataFeatureColumns(t *testing.T) {
	fixture := loadExecutionFixture(t, metadataColumnsBody+metadataColumnsQueries)
	root := Bindings{"root": {ElementValue(fixture.symbol(t, "Reqs"))}}

	tagged, err := fixture.execute(t, "Tagged", root, Options{})
	if err != nil {
		t.Fatalf("Tagged: %v", err)
	}
	var got []string
	for _, row := range tagged.Rows() {
		var cells []string
		for _, cell := range row.Cells() {
			cells = append(cells, cellText(cell))
		}
		got = append(got, strings.Join(cells, " | "))
	}
	want := []string{
		"8 | Key | Not Driving; Driver - Cost | high; low",
		"9 |  |  | ",
		"10 |  |  | ",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Tagged rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	keyed, err := fixture.execute(t, "Keyed", root, Options{})
	if err != nil {
		t.Fatalf("Keyed: %v", err)
	}
	if rows := keyed.Rows(); len(rows) != 1 || cellText(rows[0].Cells()[0]) != "8" {
		t.Fatalf("Keyed selected %d rows, want the one whose Key is set", len(rows))
	}

	ordered, err := fixture.execute(t, "ByKey", root, Options{})
	if err != nil {
		t.Fatalf("ByKey: %v", err)
	}
	var names []string
	for _, row := range ordered.Rows() {
		sym, _ := row.Element().Element()
		names = append(names, sym.Name)
	}
	// "" sorts before "Key"; the unannotated row has no value and comes last.
	if strings.Join(names, " ") != "Blank Alignment Plain" {
		t.Fatalf("ByKey = %v", names)
	}
}
