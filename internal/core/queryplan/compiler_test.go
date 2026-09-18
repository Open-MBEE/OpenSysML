package queryplan

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

const queryImports = `
private import DocumentQueries::*;
private import KerML::Root::Element;
private import ScalarValues::*;
`

type queryFixture struct {
	index    *symbols.Index
	model    *semantics.Model
	resolver *resolve.Resolver
	content  string
}

func loadQueryFixture(t *testing.T, body string) queryFixture {
	t.Helper()
	index := libs.NewModelIndex()
	name := "queries.sysml"
	content := "package Fixture {" + queryImports + body + "}"
	p := parser.New(source.New(name, []byte(content)))
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse query fixture: %v", p.Diagnostics)
	}
	index.AddDocument(name, root)
	index.ExpandWildcardImports()
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	return queryFixture{index: index, model: model, resolver: resolver, content: content}
}

func (f queryFixture) symbol(t *testing.T, name string) *symbols.Symbol {
	t.Helper()
	matches := symbols.PreferDeclared(f.index.LookupQualified("Fixture::" + name))
	if len(matches) != 1 {
		t.Fatalf("lookup %s: got %d symbols", name, len(matches))
	}
	return matches[0]
}

func (f queryFixture) compile(t *testing.T, name string) *Program {
	t.Helper()
	program, err := Compile(f.index, f.model, f.resolver, f.symbol(t, name))
	if err != nil {
		t.Fatalf("compile %s: %v", name, err)
	}
	return program
}

func planningError(t *testing.T, err error, kind ErrorKind) *Error {
	t.Helper()
	var planning *Error
	if !errors.As(err, &planning) {
		t.Fatalf("got %T, want *queryplan.Error", err)
	}
	if planning.Kind != kind {
		t.Fatalf("error kind = %s, want %s (%v)", planning.Kind, kind, planning)
	}
	return planning
}

func TestCompileQueryCompositionDependencyOrder(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def InterfacesFor :> Query {
	in subsystem : Element;
	Descendants(source = subsystem, maxDepth = 2)
}
calc def APSInterfaces :> Query {
	in subsystem : Element;
	InterfacesFor(subsystem = subsystem)
}
calc def ProvidedAPSInterfaces :> Query {
	in subsystem : Element;
	WhereType(
		source = APSInterfaces(subsystem = subsystem),
		type = "InterfaceUsage"
	)
}
`)
	program := fixture.compile(t, "ProvidedAPSInterfaces")
	if program.Entry() != "Fixture::ProvidedAPSInterfaces" {
		t.Fatalf("entry = %q", program.Entry())
	}
	definitions := program.Definitions()
	var names []string
	for _, definition := range definitions {
		names = append(names, definition.Name())
	}
	want := []string{"Fixture::InterfacesFor", "Fixture::APSInterfaces", "Fixture::ProvidedAPSInterfaces"}
	if !slices.Equal(names, want) {
		t.Fatalf("definition order = %v, want %v", names, want)
	}
	if got := definitions[1].Dependencies(); !slices.Equal(got, []string{"Fixture::InterfacesFor"}) {
		t.Fatalf("APSInterfaces dependencies = %v", got)
	}
	if got := definitions[2].Dependencies(); !slices.Equal(got, []string{"Fixture::APSInterfaces"}) {
		t.Fatalf("ProvidedAPSInterfaces dependencies = %v", got)
	}
	if definitions[2].Expression().Operation() != OperationWhereType {
		t.Fatalf("provided operation = %s", definitions[2].Expression().Operation())
	}
	parameters := definitions[2].Parameters()
	if len(parameters) != 1 || parameters[0].Name != "subsystem" ||
		parameters[0].Type != "KerML::Root::Element" ||
		parameters[0].Multiplicity != (Multiplicity{Lower: 1, Upper: 1, Known: true}) {
		t.Fatalf("provided parameters = %+v", parameters)
	}
	result := definitions[2].Result()
	if result.Type != "KerML::Root::Element" ||
		result.Multiplicity != (Multiplicity{Lower: 0, UpperInfinite: true, Known: true}) {
		t.Fatalf("provided result = %+v", result)
	}
	if !definitions[2].Origin().Located() || !definitions[2].Expression().Origin().Located() {
		t.Fatal("compiled definition and expression must retain source provenance")
	}
}

func TestCompileMemoizesRepeatedDependency(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Base :> Query {
	in subsystem : Element;
	OwnedElements(source = subsystem)
}
calc def Combined :> Query {
	in subsystem : Element;
	(Base(subsystem = subsystem), Base(subsystem = subsystem))
}
`)
	definitions := fixture.compile(t, "Combined").Definitions()
	if len(definitions) != 2 {
		t.Fatalf("compiled definitions = %d, want Base and Combined once each", len(definitions))
	}
	if got := definitions[1].Dependencies(); !slices.Equal(got, []string{"Fixture::Base"}) {
		t.Fatalf("Combined dependencies = %v", got)
	}
}

func TestCompileRetainsRedefinedParameterMetadata(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Base :> Query {
	in source : Element[0..*] = null;
	OwnedElements(source = source)
}
calc def Positional :> Base {
	in source;
	OwnedElements(source = source)
}
calc def Restated :> Positional {
	in redefines source;
	OwnedElements(source = source)
}
calc def Caller :> Query {
	Restated()
}
`)
	program := fixture.compile(t, "Caller")
	definitions := program.Definitions()
	if len(definitions) != 2 {
		t.Fatalf("compiled definitions = %d, want Restated and Caller", len(definitions))
	}
	params := definitions[0].Parameters()
	if len(params) != 1 {
		t.Fatalf("Restated parameters = %+v", params)
	}
	param := params[0]
	if param.Type != "KerML::Root::Element" ||
		param.Multiplicity != (Multiplicity{Lower: 0, UpperInfinite: true, Known: true}) ||
		!param.HasDefault {
		t.Fatalf("Restated source metadata = %+v", param)
	}
}

func literalOf(t *testing.T, expression Expression, kind LiteralKind, value string) {
	t.Helper()
	gotKind, gotValue := expression.Literal()
	if expression.Operation() != OperationLiteral || gotKind != kind || gotValue != value {
		t.Fatalf("expression = %+v, want %s literal %q", expression, kind, value)
	}
}

func TestCompileRetainsParameterDefaults(t *testing.T) {
	fixture := loadQueryFixture(t, `
part shared;
part spare;
calc def Helper :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Defaulted :> Query {
	in source : Element = shared;
	in pattern : String default "m";
	in depth : Integer = 2;
	in candidates : Element[0..*] = Helper(source = source);
	in optional : Element[0..*] = null;
	in roots : Element[0..*] = (shared, spare);
	in mixed : Element[0..*] = (spare, source, Helper(source = source));
	in owned : Element[0..*] = OwnedElements(source = spare);
	in helped : Element[0..*] = Helper(source = spare);
	WhereName(source = candidates, operator = "startsWith", value = pattern)
}
`)
	program := fixture.compile(t, "Defaulted")
	definitions := program.Definitions()
	if len(definitions) != 2 || definitions[0].Name() != "Fixture::Helper" {
		t.Fatalf("definitions = %+v, want Helper compiled ahead of Defaulted", definitions)
	}
	definition := definitions[1]
	if got := definition.Dependencies(); !slices.Equal(got, []string{"Fixture::Helper"}) {
		t.Fatalf("dependencies = %v, want the default's invocation recorded", got)
	}
	params := make(map[string]Parameter)
	for _, param := range definition.Parameters() {
		if !param.HasDefault {
			t.Fatalf("parameter %s must retain its default", param.Name)
		}
		if param.DefaultQuery != "Fixture::Defaulted" {
			t.Fatalf("parameter %s declaring query = %q", param.Name, param.DefaultQuery)
		}
		if !param.Default.Origin().Located() {
			t.Fatalf("parameter %s default must retain provenance", param.Name)
		}
		params[param.Name] = param
	}
	source := params["source"].Default
	element, ok := source.Element()
	if source.Operation() != OperationElement || !ok || element != fixture.symbol(t, "shared") ||
		source.Target() != "Fixture::shared" {
		t.Fatalf("source default = %+v, want the shared element bound", source)
	}
	literalOf(t, params["pattern"].Default, LiteralString, `"m"`)
	literalOf(t, params["depth"].Default, LiteralInteger, "2")
	candidates := params["candidates"].Default
	if candidates.Operation() != OperationInvoke || candidates.Target() != "Fixture::Helper" {
		t.Fatalf("candidates default = %+v", candidates)
	}
	if arguments := candidates.Arguments(); len(arguments) != 1 ||
		arguments[0].Value.Operation() != OperationParameter || arguments[0].Value.Target() != "source" {
		t.Fatalf("candidates default arguments = %+v", arguments)
	}
	literalOf(t, params["optional"].Default, LiteralNull, "null")
	roots := params["roots"].Default
	if roots.Operation() != OperationSequence || len(roots.Arguments()) != 2 {
		t.Fatalf("roots default = %+v, want a sequence of two elements", roots)
	}
	for i, name := range []string{"shared", "spare"} {
		member := roots.Arguments()[i].Value
		element, ok := member.Element()
		if !ok || element != fixture.symbol(t, name) {
			t.Fatalf("roots default member %d = %+v, want %s bound", i, member, name)
		}
	}
	mixed := params["mixed"].Default.Arguments()
	if len(mixed) != 3 {
		t.Fatalf("mixed default = %+v, want three members", mixed)
	}
	if element, ok := mixed[0].Value.Element(); !ok || element != fixture.symbol(t, "spare") {
		t.Fatalf("mixed default member 0 = %+v, want spare bound", mixed[0].Value)
	}
	if mixed[1].Value.Operation() != OperationParameter || mixed[1].Value.Target() != "source" {
		t.Fatalf("mixed default member 1 = %+v, want the source parameter", mixed[1].Value)
	}
	if mixed[2].Value.Operation() != OperationInvoke || mixed[2].Value.Target() != "Fixture::Helper" {
		t.Fatalf("mixed default member 2 = %+v, want Helper invoked", mixed[2].Value)
	}
	for name, operation := range map[string]Operation{
		"owned":  OperationOwnedElements,
		"helped": OperationInvoke,
	} {
		nested := params[name].Default
		if nested.Operation() != operation || len(nested.Arguments()) != 1 {
			t.Fatalf("%s default = %+v, want %s with one argument", name, nested, operation)
		}
		argument := nested.Arguments()[0].Value
		if element, ok := argument.Element(); !ok || element != fixture.symbol(t, "spare") ||
			!argument.Origin().Located() {
			t.Fatalf("%s default argument = %+v, want spare bound", name, argument)
		}
	}

	mutable := definition.Parameters()
	mutable[3].Default.Arguments()[0] = Argument{}
	mutable[3] = Parameter{}
	fresh := definition.Parameters()[3]
	if !fresh.HasDefault || fresh.Default.Arguments()[0].Name != "source" {
		t.Fatal("mutating returned parameters changed the compiled default")
	}
}

func TestCompileBindsElementsNamedInQueryBodies(t *testing.T) {
	fixture := loadQueryFixture(t, `
part def Telescope;
part telescope : Telescope;
part groundStation;
calc def Helper :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Pointed :> Query {
	in scope : Telescope;
	OwnedElements(source = telescope)
}
calc def FixedBody :> Query {
	in pattern : String;
	WhereName(source = OwnedElements(source = telescope), operator = "startsWith", value = pattern)
}
calc def FixedQueryArgument :> Query {
	Helper(source = telescope)
}
calc def FixedList :> Query {
	in extra : Element[0..*];
	OwnedElements(source = (telescope, extra, groundStation))
}
calc def FixedTyped :> Query {
	Pointed(scope = telescope)
}
calc def NamesParameterType :> Query {
	in scope : Telescope;
	OwnedElements(source = Telescope)
}
`)
	body := fixture.compile(t, "FixedBody").Definitions()[0].Expression()
	if body.Operation() != OperationWhereName || len(body.Arguments()) != 3 {
		t.Fatalf("body = %+v", body)
	}
	source := body.Arguments()[0].Value
	if source.Operation() != OperationOwnedElements || len(source.Arguments()) != 1 {
		t.Fatalf("source = %+v, want OwnedElements", source)
	}
	if element, ok := source.Arguments()[0].Value.Element(); !ok || element != fixture.symbol(t, "telescope") {
		t.Fatalf("source argument = %+v, want telescope bound", source.Arguments()[0].Value)
	}
	if value := body.Arguments()[2].Value; value.Operation() != OperationParameter || value.Target() != "pattern" {
		t.Fatalf("value = %+v, want the pattern parameter", value)
	}

	definitions := fixture.compile(t, "FixedQueryArgument").Definitions()
	invoked := definitions[len(definitions)-1]
	if got := invoked.Dependencies(); !slices.Equal(got, []string{"Fixture::Helper"}) {
		t.Fatalf("dependencies = %v", got)
	}
	call := invoked.Expression()
	if call.Operation() != OperationInvoke || len(call.Arguments()) != 1 {
		t.Fatalf("call = %+v", call)
	}
	if element, ok := call.Arguments()[0].Value.Element(); !ok || element != fixture.symbol(t, "telescope") {
		t.Fatalf("call argument = %+v, want telescope bound", call.Arguments()[0].Value)
	}

	list := fixture.compile(t, "FixedList").Definitions()[0].Expression().Arguments()[0].Value
	if list.Operation() != OperationSequence || len(list.Arguments()) != 3 {
		t.Fatalf("list = %+v", list)
	}
	if element, ok := list.Arguments()[0].Value.Element(); !ok || element != fixture.symbol(t, "telescope") {
		t.Fatalf("list member 0 = %+v", list.Arguments()[0].Value)
	}
	if member := list.Arguments()[1].Value; member.Operation() != OperationParameter || member.Target() != "extra" {
		t.Fatalf("list member 1 = %+v", member)
	}
	if element, ok := list.Arguments()[2].Value.Element(); !ok || element != fixture.symbol(t, "groundStation") {
		t.Fatalf("list member 2 = %+v", list.Arguments()[2].Value)
	}

	fixture.compile(t, "FixedTyped")

	typed := fixture.compile(t, "NamesParameterType").Definitions()[0]
	if params := typed.Parameters(); len(params) != 1 || params[0].Name != "scope" || params[0].Type != "Fixture::Telescope" {
		t.Fatalf("parameters = %+v", params)
	}
	source = typed.Expression().Arguments()[0].Value
	if element, ok := source.Element(); !ok || element != fixture.symbol(t, "Telescope") {
		t.Fatalf("source = %+v, want the Telescope definition, not the scope parameter", source)
	}
}

func TestCompileBindsEnumerationLiterals(t *testing.T) {
	fixture := loadQueryFixture(t, `
part telescope;
enum def Color { red; green; }
enum def Warm :> Color;
calc def TakesColor :> Query {
	in hue : Color;
	OwnedElements(source = telescope)
}
calc def LiteralArgument :> Query {
	TakesColor(hue = Color::green)
}
calc def SpecializedLiteralArgument :> Query {
	TakesColor(hue = Warm::red)
}
calc def LiteralDefault :> Query {
	in hue : Color = Color::red;
	TakesColor(hue = hue)
}
calc def LiteralListDefault :> Query {
	in hues : Color[0..*] = (Color::red, Color::green);
	OwnedElements(source = telescope)
}
`)
	definitions := fixture.compile(t, "LiteralArgument").Definitions()
	call := definitions[len(definitions)-1].Expression()
	if element, ok := call.Arguments()[0].Value.Element(); !ok || element != fixture.symbol(t, "Color::green") {
		t.Fatalf("call argument = %+v, want Color::green bound", call.Arguments()[0].Value)
	}

	fixture.compile(t, "SpecializedLiteralArgument")

	definitions = fixture.compile(t, "LiteralDefault").Definitions()
	hue := definitions[len(definitions)-1].Parameters()[0]
	if element, ok := hue.Default.Element(); !hue.HasDefault || !ok || element != fixture.symbol(t, "Color::red") {
		t.Fatalf("hue = %+v, want Color::red as its default", hue)
	}

	hues := fixture.compile(t, "LiteralListDefault").Definitions()[0].Parameters()[0]
	if hues.Default.Operation() != OperationSequence || len(hues.Default.Arguments()) != 2 {
		t.Fatalf("hues default = %+v", hues.Default)
	}
	if element, ok := hues.Default.Arguments()[1].Value.Element(); !ok || element != fixture.symbol(t, "Color::green") {
		t.Fatalf("hues default member 1 = %+v", hues.Default.Arguments()[1].Value)
	}
}

func TestCompileValidatesElementsNamedInArguments(t *testing.T) {
	fixture := loadQueryFixture(t, `
part def Telescope;
part telescope : Telescope;
part groundStation;
attribute label : String = "scope";
attribute def Tag;
attribute tag : Tag;
enum def Color { red; green; }
enum def Size { small; large; }
attribute paint : Color = red;
calc def Pointed :> Query {
	in scope : Telescope;
	OwnedElements(source = telescope)
}
calc def TakesScalar :> Query {
	in amount : ScalarValue;
	OwnedElements(source = telescope)
}
calc def TakesTag :> Query {
	in marker : Tag;
	OwnedElements(source = telescope)
}
calc def TakesColor :> Query {
	in hue : Color;
	OwnedElements(source = telescope)
}
calc def NeedsMany :> Query {
	in sources : Element[2..*];
	OwnedElements(source = sources)
}
calc def WrongElementType :> Query {
	Pointed(scope = groundStation)
}
calc def ElementAsString :> Query {
	WhereName(source = OwnedElements(source = telescope), operator = "startsWith", value = telescope)
}
calc def StringAttributeAsString :> Query {
	WhereName(source = OwnedElements(source = telescope), operator = "startsWith", value = label)
}
calc def AttributeAsScalarValue :> Query {
	TakesScalar(amount = label)
}
calc def AttributeAsTag :> Query {
	TakesTag(marker = tag)
}
calc def AttributeAsColor :> Query {
	TakesColor(hue = paint)
}
calc def OtherEnumerationAsColor :> Query {
	TakesColor(hue = Size::small)
}
calc def LiteralAsString :> Query {
	WhereName(source = OwnedElements(source = telescope), operator = "startsWith", value = Color::red)
}
calc def OneElementForMany :> Query {
	NeedsMany(sources = telescope)
}
calc def Unresolved :> Query {
	OwnedElements(source = missing)
}
calc def ResultAsValue :> Query {
	OwnedElements(source = result)
}
`)
	tests := []struct {
		name      string
		kind      ErrorKind
		parameter string
		expected  string
		actual    string
		argument  string
	}{
		{"WrongElementType", ErrorArgumentType, "scope", "Fixture::Telescope", "Fixture::groundStation", "groundStation"},
		{"ElementAsString", ErrorArgumentType, "value", "ScalarValues::String", "Fixture::telescope", "telescope"},
		{"StringAttributeAsString", ErrorArgumentType, "value", "ScalarValues::String", "Fixture::label", "label"},
		{"AttributeAsScalarValue", ErrorArgumentType, "amount", "ScalarValues::ScalarValue", "Fixture::label", "label"},
		{"AttributeAsTag", ErrorArgumentType, "marker", "Fixture::Tag", "Fixture::tag", "tag"},
		{"AttributeAsColor", ErrorArgumentType, "hue", "Fixture::Color", "Fixture::paint", "paint"},
		{"OtherEnumerationAsColor", ErrorArgumentType, "hue", "Fixture::Color", "Fixture::Size::small", "Size::small"},
		{"LiteralAsString", ErrorArgumentType, "value", "ScalarValues::String", "Fixture::Color::red", "Color::red"},
		{"OneElementForMany", ErrorArgumentMultiplicity, "sources", "[2..*]", "[1..1]", "telescope"},
		{"Unresolved", ErrorUnknownParameter, "missing", "", "", "missing"},
		{"ResultAsValue", ErrorUnknownParameter, "result", "", "", "result"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.name))
			planning := planningError(t, err, test.kind)
			if planning.Parameter != test.parameter ||
				planning.Expected != test.expected ||
				planning.Actual != test.actual {
				t.Fatalf("error = %+v", planning)
			}
			span := planning.Origin.Span
			if got := fixture.content[span.Offset:span.End()]; got != test.argument {
				t.Fatalf("argument origin = %q, want %q", got, test.argument)
			}
		})
	}
}

func TestCompileValidatesParameterDefaults(t *testing.T) {
	fixture := loadQueryFixture(t, `
part def Telescope;
part telescope : Telescope;
part groundStation;
attribute label : String = "scope";
attribute def Tag;
attribute tag : Tag;
attribute limit : Natural;
enum def Color { red; green; }
enum def Size { small; large; }
attribute paint : Color = red;
calc def Named :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def ElementAsString :> Query {
	in pattern : String = telescope;
	WhereName(source = OwnedElements(source = telescope), operator = "startsWith", value = pattern)
}
calc def StringAttributeAsString :> Query {
	in pattern : String = label;
	WhereName(source = OwnedElements(source = telescope), operator = "startsWith", value = pattern)
}
calc def LiteralAsString :> Query {
	in pattern : String default 3;
	WhereName(source = OwnedElements(source = telescope), operator = "startsWith", value = pattern)
}
calc def AttributeAsScalarValue :> Query {
	in amount : ScalarValue = label;
	OwnedElements(source = telescope)
}
calc def AttributeAsTag :> Query {
	in marker : Tag = tag;
	OwnedElements(source = telescope)
}
calc def AttributeAsColor :> Query {
	in hue : Color = paint;
	OwnedElements(source = telescope)
}
calc def OtherEnumerationAsColor :> Query {
	in hue : Color = Size::small;
	OwnedElements(source = telescope)
}
calc def WrongElementType :> Query {
	in scope : Telescope = groundStation;
	OwnedElements(source = scope)
}
calc def OverfullSequence :> Query {
	in source : Element = (telescope, groundStation);
	OwnedElements(source = source)
}
calc def OverfullInvocation :> Query {
	in source : Element = OwnedElements(source = telescope);
	OwnedElements(source = source)
}
calc def OverfullNamedInvocation :> Query {
	in source : Element = Named(source = telescope);
	OwnedElements(source = source)
}
calc def UnderfullSequence :> Query {
	in sources : Element[3..*] = (telescope, groundStation);
	OwnedElements(source = sources)
}
calc def InheritedMismatch :> Query {
	in hue : Color = Color::red;
	OwnedElements(source = telescope)
}
calc def NarrowedInheritedDefault :> InheritedMismatch {
	in redefines hue : Size;
}
calc def DynamicMultiplicity :> Query {
	in pool : Element[0..limit];
	in source : Element = pool;
	OwnedElements(source = source)
}
`)
	tests := []struct {
		name      string
		kind      ErrorKind
		parameter string
		expected  string
		actual    string
		value     string
	}{
		{"ElementAsString", ErrorDefaultType, "pattern", "ScalarValues::String", "Fixture::telescope", "telescope"},
		{"StringAttributeAsString", ErrorDefaultType, "pattern", "ScalarValues::String", "Fixture::label", "label"},
		{"LiteralAsString", ErrorDefaultType, "pattern", "ScalarValues::String", "ScalarValues::Integer", "3"},
		{"AttributeAsScalarValue", ErrorDefaultType, "amount", "ScalarValues::ScalarValue", "Fixture::label", "label"},
		{"AttributeAsTag", ErrorDefaultType, "marker", "Fixture::Tag", "Fixture::tag", "tag"},
		{"AttributeAsColor", ErrorDefaultType, "hue", "Fixture::Color", "Fixture::paint", "paint"},
		{"OtherEnumerationAsColor", ErrorDefaultType, "hue", "Fixture::Color", "Fixture::Size::small", "Size::small"},
		{"WrongElementType", ErrorDefaultType, "scope", "Fixture::Telescope", "Fixture::groundStation", "groundStation"},
		{"OverfullSequence", ErrorDefaultMultiplicity, "source", "[1..1]", "[2..2]", "(telescope, groundStation)"},
		{"OverfullInvocation", ErrorDefaultMultiplicity, "source", "[1..1]", "[0..*]", "OwnedElements(source = telescope)"},
		{"OverfullNamedInvocation", ErrorDefaultMultiplicity, "source", "[1..1]", "[0..*]", "Named(source = telescope)"},
		{"UnderfullSequence", ErrorDefaultMultiplicity, "sources", "[3..*]", "[2..2]", "(telescope, groundStation)"},
		{"NarrowedInheritedDefault", ErrorDefaultType, "hue", "Fixture::Size", "Fixture::Color::red", "Color::red"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.name))
			planning := planningError(t, err, test.kind)
			if planning.Parameter != test.parameter ||
				planning.Expected != test.expected ||
				planning.Actual != test.actual {
				t.Fatalf("error = %+v", planning)
			}
			span := planning.Origin.Span
			if got := fixture.content[span.Offset:span.End()]; got != test.value {
				t.Fatalf("default origin = %q, want %q", got, test.value)
			}
		})
	}
	params := fixture.compile(t, "DynamicMultiplicity").Definitions()[0].Parameters()
	if len(params) != 2 || params[0].Multiplicity.Known || !params[1].HasDefault ||
		params[1].Default.Operation() != OperationParameter {
		t.Fatalf("DynamicMultiplicity parameters = %+v, want an unknown pool multiplicity and a retained default", params)
	}
}

func TestCompileResolvesInheritedAndRedefinedDefaults(t *testing.T) {
	fixture := loadQueryFixture(t, `
package Library {
	part fallback;
	calc def Base :> Query {
		in source : Element = fallback;
		in pattern : String default "m";
		WhereName(source = OwnedElements(source = source), operator = "startsWith", value = pattern)
	}
}
part fallback;
calc def Inherits :> Library::Base {
	in redefines source;
}
calc def Redefines :> Library::Base {
	in redefines pattern default "s";
}
calc def Restated :> Redefines {
	in redefines pattern;
}
`)
	inherited := fixture.compile(t, "Inherits").Definitions()[0].Parameters()
	if len(inherited) != 2 {
		t.Fatalf("Inherits parameters = %+v", inherited)
	}
	source := inherited[0]
	element, ok := source.Default.Element()
	if source.Name != "source" || !source.HasDefault || !ok ||
		element != fixture.symbol(t, "Library::fallback") ||
		source.DefaultQuery != "Fixture::Library::Base" {
		t.Fatalf("inherited source default = %+v, want Library::fallback from Base", source)
	}
	if pattern := inherited[1]; pattern.Name != "pattern" || !pattern.HasDefault ||
		pattern.DefaultQuery != "Fixture::Library::Base" {
		t.Fatalf("inherited pattern default = %+v", pattern)
	}
	literalOf(t, inherited[1].Default, LiteralString, `"m"`)

	for _, name := range []string{"Redefines", "Restated"} {
		params := make(map[string]Parameter)
		for _, param := range fixture.compile(t, name).Definitions()[0].Parameters() {
			params[param.Name] = param
		}
		if len(params) != 2 {
			t.Fatalf("%s parameters = %+v", name, params)
		}
		if pattern := params["pattern"]; !pattern.HasDefault || pattern.DefaultQuery != "Fixture::Redefines" {
			t.Fatalf("%s pattern default = %+v, want the redefining default", name, pattern)
		}
		literalOf(t, params["pattern"].Default, LiteralString, `"s"`)
		if source := params["source"]; !source.HasDefault || source.DefaultQuery != "Fixture::Library::Base" {
			t.Fatalf("%s source default = %+v, want the inherited default", name, source)
		}
	}
}

func TestCompileRejectsUnrepresentableDefaults(t *testing.T) {
	fixture := loadQueryFixture(t, `
part root;
calc def Chained :> Query {
	in source : Element = root.name;
	OwnedElements(source = source)
}
calc def Arithmetic :> Query {
	in source : Element;
	in depth : Integer = 1 + 1;
	Descendants(source = source, maxDepth = depth)
}
calc def Inherited :> Arithmetic {
	in redefines source;
}
calc def Caller :> Query {
	in source : Element;
	Chained(source = source)
}
`)
	for _, test := range []struct {
		name  string
		param string
	}{
		{"Chained", "source"},
		{"Arithmetic", "depth"},
		{"Inherited", "depth"},
		{"Caller", "source"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.name))
			planning := planningError(t, err, ErrorUnsupportedDefault)
			if planning.Parameter != test.param || !planning.Origin.Located() {
				t.Fatalf("error = %+v, want parameter %s with provenance", planning, test.param)
			}
			if !strings.Contains(planning.Error(), test.param) {
				t.Fatalf("message %q must name the parameter", planning.Error())
			}
		})
	}
}

func TestCompileInheritsQueryResultExpression(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Base :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def Specialized :> Base {
	in redefines source;
}
`)
	definition := fixture.compile(t, "Specialized").Definitions()[0]
	if definition.Expression().Operation() != OperationOwnedElements {
		t.Fatalf("Specialized operation = %s, want inherited owned-elements", definition.Expression().Operation())
	}
	arguments := definition.Expression().Arguments()
	if len(arguments) != 1 || arguments[0].Value.Operation() != OperationParameter ||
		arguments[0].Value.Target() != "source" {
		t.Fatalf("Specialized inherited arguments = %+v", arguments)
	}
	if definition.Expression().Origin().Span.Offset >= definition.Origin().Span.Offset {
		t.Fatalf("expression origin = %+v, want Base body before Specialized declaration", definition.Expression().Origin())
	}
}

func TestCompileRejectsConflictingInheritedResults(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Left :> Query { null }
calc def Right :> Query { null }
calc def Ambiguous :> Left, Right {
	return result : Element[0..*] ordered;
}
`)
	_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Ambiguous"))
	planningError(t, err, ErrorConflictingResult)
}

func TestCompiledProgramIsImmutableToCallers(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def InterfacesFor :> Query {
	in subsystem : Element;
	Descendants(source = subsystem, maxDepth = 2)
}
`)
	program := fixture.compile(t, "InterfacesFor")
	definitions := program.Definitions()
	definitions[0] = Definition{}
	args := program.Definitions()[0].Expression().Arguments()
	args[0] = Argument{}

	fresh := program.Definitions()
	if fresh[0].Name() != "Fixture::InterfacesFor" {
		t.Fatal("mutating returned definitions changed the compiled program")
	}
	if fresh[0].Expression().Arguments()[0].Name != "source" {
		t.Fatal("mutating returned arguments changed the compiled program")
	}
}

func TestCompileRejectsDirectAndIndirectCompositionCycles(t *testing.T) {
	tests := []struct {
		name string
		body string
		root string
		path []string
		call string
	}{
		{
			name: "direct",
			body: `
calc def DirectCycle :> Query {
	in subsystem : Element;
	DirectCycle(subsystem = subsystem)
}`,
			root: "DirectCycle",
			path: []string{"Fixture::DirectCycle", "Fixture::DirectCycle"},
			call: "DirectCycle(subsystem = subsystem)",
		},
		{
			name: "indirect",
			body: `
calc def CycleA :> Query { in subsystem : Element; CycleB(subsystem = subsystem) }
calc def CycleB :> Query { in subsystem : Element; CycleC(subsystem = subsystem) }
calc def CycleC :> Query { in subsystem : Element; CycleA(subsystem = subsystem) }
`,
			root: "CycleA",
			path: []string{"Fixture::CycleA", "Fixture::CycleB", "Fixture::CycleC", "Fixture::CycleA"},
			call: "CycleA(subsystem = subsystem)",
		},
		{
			name: "default invokes its own query",
			body: `
calc def SelfDefault :> Query {
	in candidates : Element[0..*] = SelfDefault();
	OwnedElements(source = candidates)
}`,
			root: "SelfDefault",
			path: []string{"Fixture::SelfDefault", "Fixture::SelfDefault"},
			call: "SelfDefault()",
		},
		{
			name: "default invokes the query being compiled",
			body: `
calc def Outer :> Query { in subsystem : Element; Inner(subsystem = subsystem) }
calc def Inner :> Query {
	in subsystem : Element;
	in candidates : Element[0..*] = Outer(subsystem = subsystem);
	OwnedElements(source = candidates)
}`,
			root: "Outer",
			path: []string{"Fixture::Outer", "Fixture::Inner", "Fixture::Outer"},
			call: "Outer(subsystem = subsystem)",
		},
		{
			name: "defaults invoke each other",
			body: `
calc def DefaultA :> Query {
	in candidates : Element[0..*] = DefaultB();
	OwnedElements(source = candidates)
}
calc def DefaultB :> Query {
	in candidates : Element[0..*] = DefaultA();
	OwnedElements(source = candidates)
}`,
			root: "DefaultA",
			path: []string{"Fixture::DefaultA", "Fixture::DefaultB", "Fixture::DefaultA"},
			call: "DefaultA()",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := loadQueryFixture(t, test.body)
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.root))
			planning := planningError(t, err, ErrorCompositionCycle)
			if !slices.Equal(planning.Path, test.path) {
				t.Fatalf("cycle path = %v, want %v", planning.Path, test.path)
			}
			if !strings.Contains(planning.Error(), strings.Join(test.path, " -> ")) {
				t.Fatalf("cycle error does not include full path: %v", planning)
			}
			span := planning.Origin.Span
			if got := strings.TrimSpace(fixture.content[span.Offset:span.End()]); got != test.call {
				t.Fatalf("cycle origin = %q, want %q", got, test.call)
			}
		})
	}
}

func TestCompileRejectsPositionalQueryInvocation(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Base :> Query {
	in subsystem : Element;
	OwnedElements(source = subsystem)
}
calc def Positional :> Query {
	in subsystem : Element;
	Base(subsystem)
}
`)
	_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Positional"))
	planningError(t, err, ErrorPositionalQueryArgs)
}

func TestCompileRetainsPositionalBuiltinInvocation(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def PositionalBuiltin :> Query {
	in subsystem : Element;
	Descendants(subsystem, 2)
}
`)
	expression := fixture.compile(t, "PositionalBuiltin").Definitions()[0].Expression()
	args := expression.Arguments()
	if len(args) != 2 || args[0].Named || args[1].Named {
		t.Fatalf("positional builtin arguments = %+v", args)
	}
}

func TestCompileReportsDistinctDefinitionFaults(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Helper {
	in subsystem : Element;
	subsystem
}
calc def MissingResult :> Query {
	in subsystem : Element;
}
calc def UnsupportedResult :> Query {
	in subsystem : Element;
	subsystem.name
}
calc def UnknownTarget :> Query {
	in subsystem : Element;
	Helper(subsystem)
}
`)
	tests := []struct {
		name string
		kind ErrorKind
	}{
		{"MissingResult", ErrorMissingResult},
		{"UnsupportedResult", ErrorUnsupportedExpression},
		{"UnknownTarget", ErrorUnknownInvocation},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.name))
			planningError(t, err, test.kind)
		})
	}
	_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "Helper"))
	planningError(t, err, ErrorNotQueryDefinition)
}

func TestCompileValidatesNamedQueryBindings(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def Base :> Query {
	in subsystem : Element;
	in status : String;
	OwnedElements(source = subsystem)
}
calc def Missing :> Query {
	in subsystem : Element;
	Base(subsystem = subsystem)
}
calc def Unknown :> Query {
	in subsystem : Element;
	Base(subsystem = subsystem, status = "ready", extra = "value")
}
calc def Duplicate :> Query {
	in subsystem : Element;
	Base(subsystem = subsystem, status = "ready", status = "again")
}
`)
	tests := []struct {
		name string
		kind ErrorKind
	}{
		{"Missing", ErrorMissingArgument},
		{"Unknown", ErrorUnknownArgument},
		{"Duplicate", ErrorDuplicateArgument},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.name))
			planningError(t, err, test.kind)
		})
	}
}

func TestCompileValidatesNamedQueryBindingTypes(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def NeedsStatus :> Query {
	in source : Element;
	in status : String;
	OwnedElements(source = source)
}
calc def WrongUserType :> Query {
	in source : Element;
	NeedsStatus(source = source, status = source)
}
calc def WrongBuiltinType :> Query {
	in source : Element;
	Descendants(source = source, maxDepth = "all")
}
calc def WrongBuiltinElementType :> Query {
	OwnedElements(source = "none")
}
calc def WrongPositionalBuiltinType :> Query {
	in source : Element;
	Descendants(source, "all")
}
calc def WrongBuiltinInfinity :> Query {
	in source : Element;
	Descendants(source = source, maxDepth = *)
}
calc def WrongPositionalBuiltinInfinity :> Query {
	in source : Element;
	Descendants(source, *)
}
`)
	base, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "NeedsStatus"))
	if err != nil {
		t.Fatalf("compile target query: %v", err)
	}
	definitions := base.Definitions()
	if got := definitions[len(definitions)-1].Parameters()[1].Type; got != "ScalarValues::String" {
		t.Fatalf("status parameter type = %q", got)
	}
	tests := []struct {
		name      string
		parameter string
		expected  string
		actual    string
		argument  string
	}{
		{"WrongUserType", "status", "ScalarValues::String", "KerML::Root::Element", "source"},
		{"WrongBuiltinType", "maxDepth", "ScalarValues::Integer", "ScalarValues::String", `"all"`},
		{"WrongBuiltinElementType", "source", "KerML::Root::Element", "ScalarValues::String", `"none"`},
		{"WrongPositionalBuiltinType", "maxDepth", "ScalarValues::Integer", "ScalarValues::String", `"all"`},
		{"WrongBuiltinInfinity", "maxDepth", "ScalarValues::Integer", "infinity", "*"},
		{"WrongPositionalBuiltinInfinity", "maxDepth", "ScalarValues::Integer", "infinity", "*"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.name))
			planning := planningError(t, err, ErrorArgumentType)
			if planning.Parameter != test.parameter ||
				planning.Expected != test.expected ||
				planning.Actual != test.actual ||
				planning.Origin.Doc != "queries.sysml" {
				t.Fatalf("argument type error = %+v", planning)
			}
			span := planning.Origin.Span
			if got := fixture.content[span.Offset:span.End()]; got != test.argument {
				t.Fatalf("argument origin = %q, want %q", got, test.argument)
			}
		})
	}
}

func TestCompileValidatesNamedQueryBindingMultiplicity(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def NeedsOne :> Query {
	in source : Element;
	OwnedElements(source = source)
}
calc def HasMany :> Query {
	in sources : Element[0..*];
	NeedsOne(source = sources)
}
calc def NeedsMany :> Query {
	in sources : Element[2..*];
	OwnedElements(source = sources)
}
calc def HasOne :> Query {
	in source : Element;
	NeedsMany(sources = source)
}
`)
	tests := []struct {
		name      string
		parameter string
		expected  string
		actual    string
		argument  string
	}{
		{"HasMany", "source", "[1..1]", "[0..*]", "sources"},
		{"HasOne", "sources", "[2..*]", "[1..1]", "source"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, test.name))
			planning := planningError(t, err, ErrorArgumentMultiplicity)
			if planning.Parameter != test.parameter ||
				planning.Expected != test.expected ||
				planning.Actual != test.actual ||
				planning.Origin.Doc != "queries.sysml" {
				t.Fatalf("argument multiplicity error = %+v", planning)
			}
			span := planning.Origin.Span
			if got := fixture.content[span.Offset:span.End()]; got != test.argument {
				t.Fatalf("argument origin = %q, want %q", got, test.argument)
			}
		})
	}
}

func TestCompileRejectsNonInputQueryParameter(t *testing.T) {
	fixture := loadQueryFixture(t, `
calc def InvalidParameter :> Query {
	out scratch : Element;
	OwnedElements(source = scratch)
}
`)
	_, err := Compile(fixture.index, fixture.model, fixture.resolver, fixture.symbol(t, "InvalidParameter"))
	planningError(t, err, ErrorInvalidParameter)
}

func TestCompileRequiresSemanticContext(t *testing.T) {
	_, err := Compile(nil, nil, nil, nil)
	planningError(t, err, ErrorInvalidContext)
}

func TestDocumentQueryVocabularyIsBundled(t *testing.T) {
	fixture := loadQueryFixture(t, "")
	for _, name := range []string{
		queryBaseFQN,
		"DocumentQueries::OwnedElements",
		"DocumentQueries::Descendants",
		"DocumentQueries::Ancestors",
		"DocumentQueries::RelatedElements",
		"DocumentQueries::WhereType",
		"DocumentQueries::WhereMetadata",
		"DocumentQueries::WhereName",
		"DocumentQueries::WhereFeature",
		"DocumentQueries::OrderBy",
		"DocumentQueries::Project",
		"DocumentQueries::WhereRelated",
		"DocumentQueries::Except",
		"DocumentQueries::Union",
	} {
		if got := symbols.PreferDeclared(fixture.index.LookupQualified(name)); len(got) != 1 {
			t.Errorf("bundled declaration %s: got %d symbols", name, len(got))
		}
	}
}
