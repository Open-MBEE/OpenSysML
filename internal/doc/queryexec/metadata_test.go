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

func TestExecuteColumnMetadataPathReadsLikeProjectProperty(t *testing.T) {
	fixture := loadExecutionFixture(t, metadataColumnsBody+`
calc def Keys :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root), type = "RequirementDefinition"),
		properties = ("Observatory::Reqs::Properties::key"),
		columns = (Column(name = "keyPath", path = "Observatory::Reqs::Properties::key"))
	)
}
`)
	result, err := fixture.execute(t, "Keys", Bindings{
		"root": {ElementValue(fixture.symbol(t, "Reqs"))},
	}, Options{})
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	for i, row := range result.Rows() {
		projected := cellText(row.Cells()[0])
		path := cellText(row.Cells()[1])
		if path != projected {
			t.Errorf("row %d metadata path = %q, Project property = %q", i, path, projected)
		}
	}
}

func TestExecuteQuotedMetadataFeaturePathReadsLikeProjectProperty(t *testing.T) {
	fixture := loadExecutionSource(t, `
metadata def Meta {
	attribute 'tag with space' : String;
	attribute tag : String;
}
requirement def Tagged {
	@Meta { 'tag with space' = "ready"; tag = "plain"; }
}
package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	calc def Paths :> Query {
		Project(
			source = Named(qualifiedName = "Tagged"),
			properties = ("Meta::'tag with space'", "Meta::tag"),
			columns = (
				Column(name = "quotedTag", path = "Meta::'tag with space'"),
				Column(name = "tag", path = "Meta::tag")
			)
		)
	}
}
`)
	result, err := fixture.execute(t, "Paths", nil, Options{})
	if err != nil {
		t.Fatalf("Paths: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 1 {
		t.Fatalf("Paths returned %d rows, want 1", len(rows))
	}
	cells := rows[0].Cells()
	if len(cells) != 4 {
		t.Fatalf("Paths returned %d cells, want 4", len(cells))
	}
	got := []string{
		cellText(cells[0]), cellText(cells[1]), cellText(cells[2]), cellText(cells[3]),
	}
	want := []string{"ready", "plain", "ready", "plain"}
	if strings.Join(got, " | ") != strings.Join(want, " | ") {
		t.Fatalf("metadata path cells = %v, want %v", got, want)
	}
}

func TestExecuteQuotedMetadataDefinitionProjectProperty(t *testing.T) {
	fixture := loadExecutionSource(t, `
metadata def 'My Meta' {
	attribute tag : String;
}
requirement def Tagged {
	@'My Meta' { tag = "quoted"; }
}
package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	calc def Paths :> Query {
		Project(
			source = Named(qualifiedName = "Tagged"),
			properties = ("'My Meta'::tag")
		)
	}
}
`)
	result, err := fixture.execute(t, "Paths", nil, Options{})
	if err != nil {
		t.Fatalf("Paths: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 1 || cellText(rows[0].Cells()[0]) != "quoted" {
		t.Fatalf("quoted metadata property rows = %v, want one row with quoted", rows)
	}
}

func TestExecuteQuotedMetadataDefinitionColumnPath(t *testing.T) {
	fixture := loadExecutionSource(t, `
metadata def 'My Meta' {
	attribute tag : String;
}
requirement def Tagged {
	@'My Meta' { tag = "quoted"; }
}
package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	calc def Paths :> Query {
		Project(
			source = Named(qualifiedName = "Tagged"),
			properties = ("'My Meta'::tag"),
			columns = (Column(name = "tag", path = "'My Meta'::tag"))
		)
	}
}
`)
	result, err := fixture.execute(t, "Paths", nil, Options{})
	if err != nil {
		t.Fatalf("Paths: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 1 || len(rows[0].Cells()) != 2 {
		t.Fatalf("quoted metadata path rows = %v, want one row with two cells", rows)
	}
	for i, cell := range rows[0].Cells() {
		if got := cellText(cell); got != "quoted" {
			t.Errorf("cell %d = %q, want quoted", i, got)
		}
	}
}

func TestExecuteDottedMetadataFeaturePathsReadLikeProjectProperties(t *testing.T) {
	fixture := loadExecutionSource(t, `
metadata def Meta {
	attribute 'x.y' : String;
}
requirement def Tagged {
	@Meta { 'x.y' = "ready"; }
}
package Observatory {
	private import DocumentQueries::*;
	private import KerML::Root::Element;
	private import ScalarValues::*;
	calc def Paths :> Query {
		Project(
			source = Named(qualifiedName = "Tagged"),
			properties = ("Meta::x.y", "Meta::'x.y'"),
			columns = (
				Column(name = "raw", path = "Meta::x.y"),
				Column(name = "quoted", path = "Meta::'x.y'")
			)
		)
	}
}
`)
	result, err := fixture.execute(t, "Paths", nil, Options{})
	if err != nil {
		t.Fatalf("Paths: %v", err)
	}
	rows := result.Rows()
	if len(rows) != 1 || len(rows[0].Cells()) != 4 {
		t.Fatalf("dotted metadata path rows = %v, want one row with four cells", rows)
	}
	for i, cell := range rows[0].Cells() {
		if got := cellText(cell); got != "ready" {
			t.Errorf("cell %d = %q, want ready", i, got)
		}
	}
}

func TestMetadataPathValuesRejectsUndeclaredDottedFeature(t *testing.T) {
	fixture := loadExecutionSource(t, `
metadata def Meta {
	attribute 'x.y' : String;
}
requirement def Tagged {
	@Meta { 'x.y' = "ready"; }
}
`)
	matches := fixture.index.LookupQualified("Tagged")
	if len(matches) != 1 {
		t.Fatalf("Tagged lookup returned %d symbols, want 1", len(matches))
	}
	e := executor{context: Context{
		Index:    fixture.index,
		Resolver: fixture.resolver,
		Model:    fixture.model,
	}}
	_, present, err := e.metadataPathValues(matches[0], "Meta::nosuch.thing")
	if err != nil {
		t.Fatalf("metadataPathValues: %v", err)
	}
	if present {
		t.Fatal("undeclared literal metadata feature is present")
	}
}
