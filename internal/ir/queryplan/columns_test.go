package queryplan

import (
	"slices"
	"testing"
)

func entryDefinition(t *testing.T, program *Program) Definition {
	t.Helper()
	for _, definition := range program.Definitions() {
		if definition.Name() == program.Entry() {
			return definition
		}
	}
	t.Fatal("missing entry definition")
	return Definition{}
}

const computedFixture = `
part def Subsystem {
	attribute mass : Real;
	attribute alloc : Real;
	attribute count : Integer;
}
`

func TestCompileComputedColumns(t *testing.T) {
	fixture := loadQueryFixture(t, computedFixture+`
calc def Margins :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (
			Column(name = "margin", expression = Subsystem::alloc - Subsystem::mass),
			Column(name = "label", expression = "sub: " + Element::name)
		)
	)
}
`)
	program := fixture.compile(t, "Margins")
	definition := entryDefinition(t, program)
	project := definition.Expression()
	if project.Operation() != OperationProject {
		t.Fatalf("operation = %s", project.Operation())
	}
	var columns Expression
	for _, argument := range project.Arguments() {
		if argument.Name == "columns" {
			columns = argument.Value
		}
	}
	if columns.Operation() != OperationSequence {
		t.Fatalf("columns operation = %s", columns.Operation())
	}
	elements := columns.Arguments()
	if len(elements) != 2 {
		t.Fatalf("columns = %d, want 2", len(elements))
	}
	var names []string
	for _, element := range elements {
		if element.Value.Operation() != OperationColumn {
			t.Fatalf("element operation = %s", element.Value.Operation())
		}
		if !element.Value.Origin().Located() {
			t.Fatal("planned columns must carry source provenance")
		}
		names = append(names, element.Value.Target())
	}
	if names[0] != "margin" || names[1] != "label" {
		t.Fatalf("column names = %v", names)
	}
}

func TestCompiledColumnsAreImmutableToCallers(t *testing.T) {
	fixture := loadQueryFixture(t, computedFixture+`
calc def Margins :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "margin", expression = Subsystem::alloc - Subsystem::mass))
	)
}
`)
	program := fixture.compile(t, "Margins")
	definition := entryDefinition(t, program)
	arguments := definition.Expression().Arguments()
	for i := range arguments {
		arguments[i].Name = "mutated"
	}
	for _, argument := range definition.Expression().Arguments() {
		if argument.Name == "mutated" {
			t.Fatal("plan arguments must be immutable to callers")
		}
	}
}

func TestCompilePositionalProjectOmitsDefaultedColumns(t *testing.T) {
	fixture := loadQueryFixture(t, computedFixture+`
calc def Named :> Query {
	in root : Element;
	Project(Descendants(source = root, maxDepth = 1), ("name"))
}
`)
	program := fixture.compile(t, "Named")
	definition := entryDefinition(t, program)
	project := definition.Expression()
	if project.Operation() != OperationProject {
		t.Fatalf("operation = %s", project.Operation())
	}
	arguments := project.Arguments()
	if len(arguments) != 2 {
		t.Fatalf("arguments = %d, want 2", len(arguments))
	}
	if arguments[0].Name != "source" || arguments[1].Name != "properties" {
		t.Fatalf("argument names = %s, %s", arguments[0].Name, arguments[1].Name)
	}
}

func TestCompilePositionalSurplusArgumentsAreTyped(t *testing.T) {
	fixture := loadQueryFixture(t, computedFixture+`
calc def Bad :> Query {
	in root : Element;
	Project(Descendants(source = root, maxDepth = 1), ("name"), null, null)
}
`)
	_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Bad"))
	planning := planningError(t, err, ErrorArgumentCount)
	if !planning.Origin.Located() {
		t.Fatal("planning diagnostics must carry source spans")
	}
}

func TestCompileComputedColumnDiagnostics(t *testing.T) {
	cases := []struct {
		name string
		body string
		kind ErrorKind
	}{
		{
			name: "unknown property reference",
			kind: ErrorUnknownColumnProperty,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "margin", expression = Subsystem::missing - Subsystem::mass))
	)
}`,
		},
		{
			name: "non-literal column name",
			kind: ErrorColumnName,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = 42, expression = Subsystem::mass))
	)
}`,
		},
		{
			name: "static type mismatch",
			kind: ErrorColumnType,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "bad", expression = Subsystem::mass + "kg"))
	)
}`,
		},
		{
			name: "unsupported operator",
			kind: ErrorColumnOperator,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "bad", expression = Subsystem::mass ** 2.0))
	)
}`,
		},
		{
			name: "duplicate column name",
			kind: ErrorDuplicateColumn,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (Column(name = "name", expression = Subsystem::mass))
	)
}`,
		},
		{
			name: "empty projection",
			kind: ErrorEmptyProjection,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(source = Descendants(source = root, maxDepth = 1))
}`,
		},
		{
			name: "explicit null properties",
			kind: ErrorEmptyProjection,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(source = Descendants(source = root, maxDepth = 1), properties = null)
}`,
		},
		{
			name: "explicit null columns",
			kind: ErrorEmptyProjection,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(source = Descendants(source = root, maxDepth = 1), columns = null)
}`,
		},
		{
			name: "explicit null properties and columns",
			kind: ErrorEmptyProjection,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = null,
		columns = null
	)
}`,
		},
		{
			name: "missing column expression",
			kind: ErrorMissingArgument,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "empty"))
	)
}`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := loadQueryFixture(t, computedFixture+test.body)
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Bad"))
			planning := planningError(t, err, test.kind)
			if !planning.Origin.Located() {
				t.Fatal("planning diagnostics must carry source spans")
			}
		})
	}
}

func TestCompileRelatedColumns(t *testing.T) {
	fixture := loadQueryFixture(t, computedFixture+`
calc def Matrix :> Query {
	in root : Element;
	in depth : Integer = 1;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (
			RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", maxDepth = depth),
			RelatedColumn("verifications", "verification", "incoming", 1, "count"),
			RelatedColumn(name = "verified", relationshipKind = "verification", direction = "incoming", maxDepth = 1, aggregate = "any"),
			Column(name = "label", expression = "req: " + Element::name)
		)
	)
}
`)
	program := fixture.compile(t, "Matrix")
	definition := entryDefinition(t, program)
	var columns Expression
	for _, argument := range definition.Expression().Arguments() {
		if argument.Name == "columns" {
			columns = argument.Value
		}
	}
	elements := columns.Arguments()
	if len(elements) != 4 {
		t.Fatalf("columns = %d, want 4", len(elements))
	}
	want := []struct {
		operation Operation
		name      string
		arguments []string
	}{
		{OperationRelatedColumn, "satisfiedBy", []string{"relationshipKind", "direction", "maxDepth"}},
		{OperationRelatedColumn, "verifications", []string{"relationshipKind", "direction", "maxDepth", "aggregate"}},
		{OperationRelatedColumn, "verified", []string{"relationshipKind", "direction", "maxDepth", "aggregate"}},
		{OperationColumn, "label", []string{"expression"}},
	}
	for i, element := range elements {
		column := element.Value
		if column.Operation() != want[i].operation || column.Target() != want[i].name {
			t.Fatalf("column %d = %s %q, want %s %q", i, column.Operation(), column.Target(), want[i].operation, want[i].name)
		}
		if !column.Origin().Located() {
			t.Fatalf("column %d must carry source provenance", i)
		}
		var names []string
		for _, argument := range column.Arguments() {
			names = append(names, argument.Name)
		}
		if len(names) != len(want[i].arguments) {
			t.Fatalf("column %d arguments = %v, want %v", i, names, want[i].arguments)
		}
		for j, name := range names {
			if name != want[i].arguments[j] {
				t.Fatalf("column %d arguments = %v, want %v", i, names, want[i].arguments)
			}
		}
	}
	depth, _ := argumentOf(t, elements[0].Value, "maxDepth")
	if depth.Operation() != OperationParameter || depth.Target() != "depth" {
		t.Fatalf("maxDepth = %s %q, want the depth parameter", depth.Operation(), depth.Target())
	}
	aggregate, _ := argumentOf(t, elements[1].Value, "aggregate")
	if kind, text := aggregate.Literal(); aggregate.Operation() != OperationLiteral || kind != LiteralString || text != `"count"` {
		t.Fatalf("aggregate = %s %s %s", aggregate.Operation(), kind, text)
	}
}

func TestCompilePropertyColumns(t *testing.T) {
	fixture := loadQueryFixture(t, computedFixture+`
calc def Ledger :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (
			PropertyColumn(name = "name"),
			Column(name = "label", expression = "req: " + Element::name),
			PropertyColumn(name = "Notes", property = "documentation"),
			PropertyColumn("qualifiedName")
		)
	)
}
`)
	program := fixture.compile(t, "Ledger")
	definition := entryDefinition(t, program)
	var columns Expression
	for _, argument := range definition.Expression().Arguments() {
		if argument.Name == "columns" {
			columns = argument.Value
		}
	}
	elements := columns.Arguments()
	if len(elements) != 4 {
		t.Fatalf("columns = %d, want 4", len(elements))
	}
	want := []struct {
		operation Operation
		name      string
		arguments []string
	}{
		{OperationPropertyColumn, "name", nil},
		{OperationColumn, "label", []string{"expression"}},
		{OperationPropertyColumn, "Notes", []string{"property"}},
		{OperationPropertyColumn, "qualifiedName", nil},
	}
	for i, element := range elements {
		column := element.Value
		if column.Operation() != want[i].operation || column.Target() != want[i].name {
			t.Fatalf("column %d = %s %q, want %s %q", i, column.Operation(), column.Target(), want[i].operation, want[i].name)
		}
		if !column.Origin().Located() {
			t.Fatalf("column %d must carry source provenance", i)
		}
		var names []string
		for _, argument := range column.Arguments() {
			names = append(names, argument.Name)
		}
		if !slices.Equal(names, want[i].arguments) {
			t.Fatalf("column %d arguments = %v, want %v", i, names, want[i].arguments)
		}
	}
	property, _ := argumentOf(t, elements[2].Value, "property")
	if kind, text := property.Literal(); property.Operation() != OperationLiteral || kind != LiteralString || text != `"documentation"` {
		t.Fatalf("property = %s %s %s", property.Operation(), kind, text)
	}
}

func TestCompilePropertyColumnDiagnostics(t *testing.T) {
	cases := []struct {
		name string
		body string
		kind ErrorKind
	}{
		{
			name: "non-literal column name",
			kind: ErrorColumnName,
			body: `
calc def Bad :> Query {
	in root : Element;
	in label : String;
	Project(source = Descendants(source = root, maxDepth = 1), columns = (PropertyColumn(name = label)))
}`,
		},
		{
			name: "missing name",
			kind: ErrorMissingArgument,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(source = Descendants(source = root, maxDepth = 1), columns = (PropertyColumn(property = "name")))
}`,
		},
		{
			name: "mistyped property",
			kind: ErrorArgumentType,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(source = Descendants(source = root, maxDepth = 1), columns = (PropertyColumn(name = "n", property = 1)))
}`,
		},
		{
			name: "unknown argument",
			kind: ErrorUnknownArgument,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(source = Descendants(source = root, maxDepth = 1), columns = (PropertyColumn(name = "n", expression = "x")))
}`,
		},
		{
			name: "too many positional arguments",
			kind: ErrorArgumentCount,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(source = Descendants(source = root, maxDepth = 1), columns = (PropertyColumn("n", "name", "extra")))
}`,
		},
		{
			name: "duplicate of a projected property",
			kind: ErrorDuplicateColumn,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(source = Descendants(source = root, maxDepth = 1), properties = ("name"), columns = (PropertyColumn(name = "name")))
}`,
		},
		{
			name: "duplicate of a computed column",
			kind: ErrorDuplicateColumn,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (Column(name = "name", expression = Element::name), PropertyColumn(name = "name"))
	)
}`,
		},
		{
			name: "property column outside a projection",
			kind: ErrorInvalidColumn,
			body: `
calc def Bad :> Query {
	in root : Element;
	PropertyColumn(name = "name")
}`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := loadQueryFixture(t, computedFixture+test.body)
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Bad"))
			planning := planningError(t, err, test.kind)
			if !planning.Origin.Located() {
				t.Fatal("planning diagnostics must carry source spans")
			}
		})
	}
}

func argumentOf(t *testing.T, expression Expression, name string) (Expression, bool) {
	t.Helper()
	for _, argument := range expression.Arguments() {
		if argument.Name == name {
			return argument.Value, true
		}
	}
	t.Fatalf("missing argument %s", name)
	return Expression{}, false
}

func TestCompileRelatedColumnDiagnostics(t *testing.T) {
	cases := []struct {
		name string
		body string
		kind ErrorKind
	}{
		{
			name: "non-literal column name",
			kind: ErrorColumnName,
			body: `
calc def Bad :> Query {
	in root : Element;
	in label : String;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (RelatedColumn(name = label, relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1))
	)
}`,
		},
		{
			name: "non-literal positional column name",
			kind: ErrorColumnName,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (RelatedColumn(42, "satisfaction", "incoming", 1))
	)
}`,
		},
		{
			name: "missing name",
			kind: ErrorMissingArgument,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (RelatedColumn(relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1))
	)
}`,
		},
		{
			name: "missing direction",
			kind: ErrorMissingArgument,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", maxDepth = 1))
	)
}`,
		},
		{
			name: "too few positional arguments",
			kind: ErrorArgumentCount,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (RelatedColumn("satisfiedBy", "satisfaction"))
	)
}`,
		},
		{
			name: "too many positional arguments",
			kind: ErrorArgumentCount,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (RelatedColumn("satisfiedBy", "satisfaction", "incoming", 1, "list", root, "extra"))
	)
}`,
		},
		{
			name: "unknown argument",
			kind: ErrorUnknownArgument,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1, depth = 2))
	)
}`,
		},
		{
			name: "duplicate argument",
			kind: ErrorDuplicateArgument,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", direction = "outgoing", maxDepth = 1))
	)
}`,
		},
		{
			name: "mistyped depth",
			kind: ErrorArgumentType,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", maxDepth = "one"))
	)
}`,
		},
		{
			name: "mistyped relationship kind",
			kind: ErrorArgumentType,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (RelatedColumn(name = "satisfiedBy", relationshipKind = 1, direction = "incoming", maxDepth = 1))
	)
}`,
		},
		{
			name: "unsupported aggregate",
			kind: ErrorColumnAggregate,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		columns = (RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1, aggregate = "sum"))
	)
}`,
		},
		{
			name: "duplicate related column name",
			kind: ErrorDuplicateColumn,
			body: `
calc def Bad :> Query {
	in root : Element;
	Project(
		source = Descendants(source = root, maxDepth = 1),
		properties = ("name"),
		columns = (RelatedColumn(name = "name", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1))
	)
}`,
		},
		{
			name: "related column outside a projection",
			kind: ErrorInvalidColumn,
			body: `
calc def Bad :> Query {
	in root : Element;
	RelatedColumn(name = "satisfiedBy", relationshipKind = "satisfaction", direction = "incoming", maxDepth = 1)
}`,
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			fixture := loadQueryFixture(t, computedFixture+test.body)
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Bad"))
			planning := planningError(t, err, test.kind)
			if !planning.Origin.Located() {
				t.Fatal("planning diagnostics must carry source spans")
			}
		})
	}
}
