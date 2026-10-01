package queryexec

import (
	"errors"
	"slices"
	"testing"
)

const navigationBody = `
part def Bench;
part def Rack;
package Interfaces {
	port def Digital {
		attribute def Frame;
		inout attribute : ScalarValues::String;
	}
	port def USB :> Digital;
	port def Serial {
		in attribute rx : ScalarValues::String;
		out attribute tx : ScalarValues::String;
	}
	port def Spare;
}
part def CCD {
	port usb : Interfaces::USB;
}
part def Motor {
	port link : Interfaces::Serial;
}
package System {
	part bench : Bench {
		part ccd : CCD {
			doc /* The camera on the bench. */
		}
		part focus : Motor;
	}
	part rack : Rack {
		part ccd : CCD;
		part controller : Motor;
	}
	connection camera connect bench.ccd.usb to rack.ccd.usb;
	connection drive connect bench.focus.link to rack.controller.link;
}
`

func navigationRows(t *testing.T, query, root string) *RowSet {
	t.Helper()
	fixture := loadExecutionFixture(t, navigationBody+query)
	result, err := fixture.execute(t, "Q", Bindings{
		"root": {ElementValue(fixture.symbol(t, root))},
	}, Options{})
	if err != nil {
		t.Fatalf("execute Q: %v", err)
	}
	return result
}

// `->SequenceFunctions::size()` over the usages a definition types counts them at execution: a
// query operation invoked inside a cell reads the cell's row.
func TestExecuteCellCountsElementsTypedByRow(t *testing.T) {
	result := navigationRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PortDefinition"),
		properties = ("name"),
		columns = (Column(name = "Count", cell = { in row : Element;
			RelatedElements(source = row, relationshipKind = "typing", direction = "incoming", maxDepth = 1)->SequenceFunctions::size()
		}))
	)
}`, "Interfaces")
	if got := cellStrings(t, result, "name"); !slices.EqualFunc(got, [][]string{{"Digital"}, {"USB"}, {"Serial"}, {"Spare"}}, slices.Equal) {
		t.Fatalf("name cells = %q", got)
	}
	if got := cellNumbers(t, result, "Count"); !slices.EqualFunc(got, [][]float64{{0}, {1}, {1}, {0}}, slices.Equal) {
		t.Fatalf("Count cells = %v, want 0, 1, 1, 0", got)
	}
}

// A connector row navigates Connector::connectorEnd: each end's chaining
// features, their types and the end's name are read structurally, so a cell
// can pick the part on the end that passes through a part typed Bench.
func TestExecuteCellNavigatesConnectorEnds(t *testing.T) {
	result := navigationRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "ConnectionUsage"),
		properties = ("name"),
		columns = (
			Column(name = "Bench side", cell = { in row : KerML::Kernel::Connector;
				row.connectorEnd
					->ControlFunctions::select {in e; e.chainingFeature->ControlFunctions::exists {in y; y.type.name == "Bench"}}
					.chainingFeature->ControlFunctions::select {in z; z.type.name != "Bench"}.name
			}),
			Column(name = "Ends", cell = { in row : KerML::Kernel::Connector; row.connectorEnd->SequenceFunctions::size() }),
			Column(name = "Type", cell = { in row : KerML::Kernel::Connector;
				row.connectorEnd.chainingFeature->SequenceFunctions::last().type.name->Distinct()
			}),
			Column(name = "Direction", cell = { in row : KerML::Kernel::Connector;
				row.connectorEnd->SequenceFunctions::head().chainingFeature->SequenceFunctions::last().type.member
					->ControlFunctions::select {in y : KerML::Core::Feature; y.direction->SequenceFunctions::notEmpty()}.direction->Distinct()
			})
		)
	)
}`, "System")
	if got := cellStrings(t, result, "name"); !slices.EqualFunc(got, [][]string{{"camera"}, {"drive"}}, slices.Equal) {
		t.Fatalf("name cells = %q", got)
	}
	if got := cellStrings(t, result, "Bench side"); !slices.EqualFunc(got, [][]string{{"ccd", "usb"}, {"focus", "link"}}, slices.Equal) {
		t.Fatalf("Bench side cells = %q", got)
	}
	if got := cellNumbers(t, result, "Ends"); !slices.EqualFunc(got, [][]float64{{2}, {2}}, slices.Equal) {
		t.Fatalf("Ends cells = %v", got)
	}
	if got := cellStrings(t, result, "Type"); !slices.EqualFunc(got, [][]string{{"USB"}, {"Serial"}}, slices.Equal) {
		t.Fatalf("Type cells = %q", got)
	}
	if got := cellStrings(t, result, "Direction"); !slices.EqualFunc(got, [][]string{{"inout"}, {"in", "out"}}, slices.Equal) {
		t.Fatalf("Direction cells = %q", got)
	}
}

// Boolean connectives and comparisons compose inside a body; `->ControlFunctions::exists` and
// `->ControlFunctions::forAll` yield one Boolean; `not` negates it.
func TestExecuteCellLogicalOperators(t *testing.T) {
	result := navigationRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "ConnectionUsage"),
		properties = ("name"),
		columns = (
			Column(name = "USB", cell = { in row : KerML::Kernel::Connector;
				row.connectorEnd->ControlFunctions::forAll {in e; e.chainingFeature->SequenceFunctions::last().type.name == "USB" or e.chainingFeature->SequenceFunctions::last().type.name == "Spare"}
			}),
			Column(name = "Mixed", cell = { in row : KerML::Kernel::Connector;
				not (row.connectorEnd->ControlFunctions::exists {in e; e.chainingFeature->ControlFunctions::exists {in y; y.type.name == "Rack" and y.name == "rack"}})
			})
		)
	)
}`, "System")
	if got := cellBooleans(t, result, "USB"); !slices.EqualFunc(got, [][]bool{{true}, {false}}, slices.Equal) {
		t.Fatalf("USB cells = %v", got)
	}
	if got := cellBooleans(t, result, "Mixed"); !slices.EqualFunc(got, [][]bool{{false}, {false}}, slices.Equal) {
		t.Fatalf("Mixed cells = %v", got)
	}
}

// cellBooleans reads every Boolean value of the named column, one slice per row.
func cellBooleans(t *testing.T, result *RowSet, column string) [][]bool {
	t.Helper()
	position := slices.IndexFunc(result.Columns(), func(c Column) bool { return c.Name() == column })
	if position < 0 {
		t.Fatalf("no column %s in %v", column, result.Columns())
	}
	var out [][]bool
	for _, row := range result.Rows() {
		var values []bool
		for _, value := range row.Cells()[position].Values() {
			truth, ok := value.Boolean()
			if !ok {
				t.Fatalf("%s cell = %+v, want Booleans", column, row.Cells()[position].Values())
			}
			values = append(values, truth)
		}
		out = append(out, values)
	}
	return out
}

// A cell collects from its row and projects an attribute of every element it
// reaches: `->ControlFunctions::collect` flattens, a query operation may read a lambda variable,
// `->SequenceFunctions::excluding` drops a value, and `documentation` reads the comment prose.
func TestExecuteCellCollectsAndProjects(t *testing.T) {
	result := navigationRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = Named(qualifiedName = ("System::bench", "System::rack")),
		properties = ("name"),
		columns = (
			Column(name = "Parts", cell = { in row : Element;
				Descendants(source = row, maxDepth = 1)->ControlFunctions::collect {in p; p.name}
			}),
			Column(name = "Port types", cell = { in row : Element;
				Descendants(source = row, maxDepth = 1)
					->ControlFunctions::collect {in p; RelatedElements(source = p, relationshipKind = "typing", direction = "outgoing", maxDepth = 1)}
					->ControlFunctions::collect {in d; Descendants(source = d, maxDepth = 1)}
					->ControlFunctions::collect {in u; RelatedElements(source = u, relationshipKind = "typing", direction = "outgoing", maxDepth = 1)}
					.name->Distinct()
			}),
			Column(name = "Docs", cell = { in row : Element;
				Descendants(source = row, maxDepth = 1).documentation
			}),
			Column(name = "Path", cell = { in row : Element;
				Ancestors(source = row)->SequenceFunctions::excluding(Ancestors(source = row)->SequenceFunctions::last()).name
			})
		)
	)
}`, "System")
	if got := cellStrings(t, result, "name"); !slices.EqualFunc(got, [][]string{{"bench"}, {"rack"}}, slices.Equal) {
		t.Fatalf("name cells = %q", got)
	}
	if got := cellStrings(t, result, "Parts"); !slices.EqualFunc(got, [][]string{{"ccd", "focus"}, {"ccd", "controller"}}, slices.Equal) {
		t.Fatalf("Parts cells = %q", got)
	}
	if got := cellStrings(t, result, "Port types"); !slices.EqualFunc(got, [][]string{{"USB", "Serial"}, {"USB", "Serial"}}, slices.Equal) {
		t.Fatalf("Port types cells = %q", got)
	}
	if got := cellStrings(t, result, "Docs"); !slices.EqualFunc(got, [][]string{{"The camera on the bench."}, {}}, slices.Equal) {
		t.Fatalf("Docs cells = %q", got)
	}
	if got := cellStrings(t, result, "Path"); !slices.EqualFunc(got, [][]string{{"System"}, {"System"}}, slices.Equal) {
		t.Fatalf("Path cells = %q", got)
	}
}

// The conditional `and`, `or` and `implies` decide on their first operand
// where it settles them, leaving the second unevaluated: a second operand
// that yields two Booleans fails only when it is reached, as `&` and `|`
// always reach it.
func TestExecuteCellLogicalOperatorsShortCircuit(t *testing.T) {
	const twoBooleans = `row.connectorEnd->ControlFunctions::collect {in e; e.name == "x"}`
	for _, tc := range []struct {
		expr string
		want bool
	}{
		{"false and " + twoBooleans, false},
		{"true or " + twoBooleans, true},
		{"false implies " + twoBooleans, true},
	} {
		result := navigationRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "ConnectionUsage"),
		properties = ("name"),
		columns = (Column(name = "Decided", cell = { in row : KerML::Kernel::Connector; `+tc.expr+` })))
}`, "System")
		if got := cellBooleans(t, result, "Decided"); !slices.EqualFunc(got, [][]bool{{tc.want}, {tc.want}}, slices.Equal) {
			t.Errorf("%s = %v, want %v on every row", tc.expr, got, tc.want)
		}
	}
	for _, expr := range []string{
		"true and " + twoBooleans,
		"false or " + twoBooleans,
		"true implies " + twoBooleans,
		"false & " + twoBooleans,
		"true | " + twoBooleans,
	} {
		fixture := loadExecutionFixture(t, navigationBody+`
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "ConnectionUsage"),
		properties = ("name"),
		columns = (Column(name = "Reached", cell = { in row : KerML::Kernel::Connector; `+expr+` })))
}`)
		_, err := fixture.execute(t, "Q", Bindings{"root": {ElementValue(fixture.symbol(t, "System"))}}, Options{})
		var execErr *Error
		if !errors.As(err, &execErr) || execErr.Kind != ErrorColumnOperand {
			t.Errorf("%s: error = %v, want %v for the two-valued operand it reaches", expr, err, ErrorColumnOperand)
		}
	}
}

// `ownedElement` lists every element a type's body declares: the named
// members and the ones declared without a name, each once.
func TestExecuteCellOwnedElementIncludesAnonymousMembers(t *testing.T) {
	result := navigationRows(t, `
calc def Q :> Query {
	in root : Element;
	Project(
		source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PortDefinition"),
		properties = ("name"),
		columns = (
			Column(name = "Owned", cell = { in row : KerML::Root::Element; row.ownedElement->SequenceFunctions::size() }),
			Column(name = "Named", cell = { in row : KerML::Root::Element; row.ownedElement.name })))
}`, "Interfaces")
	if got := cellNumbers(t, result, "Owned"); !slices.EqualFunc(got, [][]float64{{2}, {0}, {2}, {0}}, slices.Equal) {
		t.Fatalf("Owned cells = %v, want Digital's attribute def and anonymous attribute, Serial's rx and tx", got)
	}
	if got := cellStrings(t, result, "Named"); !slices.EqualFunc(got, [][]string{{"Frame"}, nil, {"rx", "tx"}, nil}, slices.Equal) {
		t.Fatalf("Named cells = %v", got)
	}
}
