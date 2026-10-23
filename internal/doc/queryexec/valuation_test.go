package queryexec

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

const valuationBody = `
	enum def Mode { on; off; }
	part def Tank {
		attribute mass : ISQ::MassValue = 5 [SI::kg];
		attribute doubled : ISQ::MassValue = mass * 2;
		attribute mode : Mode = Mode::on;
		attribute label : String = "main";
		attribute open : Real;
		attribute valuation : String = "shadowed";
		part def Nested;
	}
	part def Full :> Tank {
		attribute :>> open = 7.5;
	}
	calc def Own :> Query {
		Project(
			source = WhereType(source = OwnedElements(source = Named(qualifiedName = "Observatory::Tank")), type = "AttributeUsage"),
			properties = ("name", "valuation")
		)
	}
	calc def Mixed :> Query {
		Project(
			source = OwnedElements(source = Named(qualifiedName = "Observatory::Tank")),
			properties = ("name"),
			columns = (Column(name = "bound", path = "valuation"))
		)
	}
	calc def Redefined :> Query {
		Project(source = OwnedElements(source = Named(qualifiedName = "Observatory::Full")), properties = ("name", "valuation"))
	}
	calc def Masses :> Query {
		OrderBy(
			source = WhereType(source = OwnedElements(source = Named(qualifiedName = "Observatory::Tank")), type = "AttributeUsage"),
			property = "valuation", direction = "descending", missing = "last", multiple = "first")
	}
	calc def Definitions :> Query {
		Project(source = Named(qualifiedName = "Observatory::Tank"), properties = ("name", "valuation"))
	}
`

func valuationRows(t *testing.T, fixture executionFixture, query string) []string {
	t.Helper()
	result, err := fixture.execute(t, query, nil, Options{})
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	var got []string
	for _, row := range result.Rows() {
		var cells []string
		for _, cell := range row.Cells() {
			cells = append(cells, valuationCellText(cell))
		}
		got = append(got, strings.Join(cells, " | "))
	}
	return got
}

func valuationCellText(cell Cell) string {
	var parts []string
	for _, value := range cell.Values() {
		switch {
		case value.Kind() == ValueQuantity:
			quantity, _ := value.Quantity()
			parts = append(parts, quantity.String())
		case value.Kind() == ValueReal:
			realValue, _ := value.Real()
			parts = append(parts, semantics.FormatReal(realValue))
		case value.Kind() == ValueInteger:
			integer, _ := value.Integer()
			parts = append(parts, strconv.FormatInt(integer, 10))
		default:
			parts = append(parts, cellText(cell))
		}
	}
	return strings.Join(parts, "; ")
}

// The valuation property reads what a row feature's own FeatureValue binds: a
// quantity with its unit, an expression evaluated, an enumeration literal, a
// string; a feature bound to nothing is absent, and the built-in name wins
// over a member spelled the same.
func TestExecuteProjectsValuation(t *testing.T) {
	fixture := loadExecutionFixture(t, valuationBody)
	got := valuationRows(t, fixture, "Own")
	want := []string{
		"mass | 5 [SI::kg]",
		"doubled | 10 [SI::kg]",
		"mode | on",
		"label | main",
		"open | ",
		"valuation | shadowed",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("Own rows:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A Column path of the one segment reads the same property, and a row that is
// not a feature has no valuation rather than failing the column.
func TestExecuteValuationColumnPathOverMixedRows(t *testing.T) {
	fixture := loadExecutionFixture(t, valuationBody)
	got := valuationRows(t, fixture, "Mixed")
	if len(got) != 7 || got[0] != "mass | 5 [SI::kg]" || got[6] != "Nested | " {
		t.Fatalf("Mixed rows = %q", got)
	}
}

// A redefinition that binds the value is the feature's valuation.
func TestExecuteValuationReadsRedefinedBinding(t *testing.T) {
	fixture := loadExecutionFixture(t, valuationBody)
	if got := valuationRows(t, fixture, "Redefined"); len(got) != 1 || got[0] != "open | 7.5" {
		t.Fatalf("Redefined rows = %q", got)
	}
}

// Rows that are not features read valuation as absent, so the property is
// known and the column empty rather than an unknown-property failure.
func TestExecuteValuationOfDefinitionIsAbsent(t *testing.T) {
	fixture := loadExecutionFixture(t, valuationBody)
	if got := valuationRows(t, fixture, "Definitions"); len(got) != 1 || got[0] != "Tank | " {
		t.Fatalf("Definitions rows = %q", got)
	}
}

// OrderBy sorts by valuation where the values compare; a mix of quantities,
// literals and strings is the incomparable-values failure any property gives.
func TestExecuteOrderByValuationRejectsIncomparableValues(t *testing.T) {
	fixture := loadExecutionFixture(t, valuationBody)
	_, err := fixture.execute(t, "Masses", nil, Options{})
	var typed *Error
	if !errors.As(err, &typed) || typed.Kind != ErrorInvalidOrder {
		t.Fatalf("Masses error = %v, want incomparable values", err)
	}
}

func TestExecuteOrderByValuationSortsQuantities(t *testing.T) {
	fixture := loadExecutionFixture(t, `
	part def Stage {
		attribute dry : ISQ::MassValue = 300 [SI::kg];
		attribute wet : ISQ::MassValue = 2000 [SI::kg];
		attribute payload : ISQ::MassValue = 1500 [SI::kg];
	}
	calc def Heaviest :> Query {
		OrderBy(
			source = OwnedElements(source = Named(qualifiedName = "Observatory::Stage")),
			property = "valuation", direction = "descending", missing = "last", multiple = "first")
	}
`)
	result, err := fixture.execute(t, "Heaviest", nil, Options{})
	if err != nil {
		t.Fatalf("Heaviest: %v", err)
	}
	var names []string
	for _, row := range result.Rows() {
		sym, _ := row.Element().Element()
		names = append(names, sym.Name)
	}
	if strings.Join(names, " ") != "wet payload dry" {
		t.Fatalf("Heaviest = %v", names)
	}
}

// A parameter that redefines its definition's parameter by position, without
// a `:>>` clause, takes that parameter's binding: valuation on the parameter
// row and the member form on its owner's row read the same value.
func TestExecuteValuationFollowsImplicitParameterRedefinition(t *testing.T) {
	fixture := loadExecutionFixture(t, `
	action def Focus { in level : Integer = 3; in gain : Integer; }
	action focus : Focus { in level; in :>> gain = 2; }
	calc def Params :> Query {
		Project(source = OwnedElements(source = Named(qualifiedName = "Observatory::focus")), properties = ("name", "valuation"))
	}
	calc def Member :> Query {
		Project(source = Named(qualifiedName = "Observatory::focus"), properties = ("level", "gain"))
	}
`)
	if got := valuationRows(t, fixture, "Params"); strings.Join(got, "\n") != "level | 3\ngain | 2" {
		t.Fatalf("Params rows = %q", got)
	}
	if got := valuationRows(t, fixture, "Member"); strings.Join(got, "\n") != "3 | 2" {
		t.Fatalf("Member rows = %q", got)
	}
}

// An object row is no feature: valuation is an empty cell rather than an
// unknown property, unless the object holds a member spelled valuation.
func TestExecuteValuationOverObjectRowsIsAbsent(t *testing.T) {
	fixture := loadExecutionFixture(t, `
	part def Held;
	part def Car :> Held { attribute mass : Integer = 1200; }
	part def Ledger :> Held { attribute valuation : Integer = 7; }
	part car : Car;
	part ledger : Ledger;
	calc def HeldObjects :> Query {
		Project(source = Objects(type = "Held"), properties = ("name", "valuation"))
	}
`)
	ctx := runtime.NewContext(runtime.NewModel(fixture.model, fixture.resolver), runtime.DefaultMaxSteps)
	car, err := ctx.Instantiate(fixture.symbol(t, "car"))
	if err != nil {
		t.Fatalf("Instantiate car: %v", err)
	}
	ledger, err := ctx.Instantiate(fixture.symbol(t, "ledger"))
	if err != nil {
		t.Fatalf("Instantiate ledger: %v", err)
	}
	context := Context{
		Index: fixture.index, Resolver: fixture.resolver, Model: fixture.model, Runtime: ctx,
		Roots: []Root{{Label: "car", Object: car}, {Label: "ledger", Object: ledger}},
	}
	result, err := Execute(fixture.program(t, "HeldObjects"), context, nil, Options{})
	if err != nil {
		t.Fatalf("HeldObjects: %v", err)
	}
	var got []string
	for _, row := range result.Rows() {
		got = append(got, valuationCellText(row.Cells()[0])+" | "+valuationCellText(row.Cells()[1]))
	}
	if strings.Join(got, "\n") != "car | \nledger | 7" {
		t.Fatalf("Held rows = %q", got)
	}
}
