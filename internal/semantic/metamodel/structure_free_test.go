package metamodel

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
)

func TestSemanticModelPropertiesWithoutStructure(t *testing.T) {
	const (
		name = "structure-free.sysml"
		text = `package Vehicles {
			part def Base specializes Parts::Part {
				part inherited;
				in attribute input;
			}
			part def Derived specializes Base {
				constant part local;
			}
			attribute def Value specializes ScalarValues::Real {
				attribute quantity;
			}
			part vehicle : Derived;
		}`
	)
	index := libs.NewModelIndex()
	parsed := parser.New(source.New(name, []byte(text)))
	root := parsed.ParseFile()
	if len(parsed.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", parsed.Diagnostics)
	}
	index.AddDocumentWithKind(name, root, source.KindOf(name))
	resolver := resolve.New(index)
	model := semantics.NewModel(resolver)
	resolver.SetModel(model)
	resolver.ResolveDocument(name, root)
	evaluator := New(resolver, model)

	pkg, ok := index.DocumentRoot(name).LookupLocal("Vehicles")
	if !ok {
		t.Fatal("package Vehicles not found")
	}
	derived, ok := pkg.Scope.LookupLocal("Derived")
	if !ok {
		t.Fatal("definition Derived not found")
	}
	base, ok := pkg.Scope.LookupLocal("Base")
	if !ok {
		t.Fatal("definition Base not found")
	}
	local, ok := derived.Scope.LookupLocal("local")
	if !ok {
		t.Fatal("feature local not found")
	}
	vehicle, ok := pkg.Scope.LookupLocal("vehicle")
	if !ok {
		t.Fatal("usage vehicle not found")
	}
	input, ok := base.Scope.LookupLocal("input")
	if !ok {
		t.Fatal("feature input not found")
	}
	valueType, ok := pkg.Scope.LookupLocal("Value")
	if !ok {
		t.Fatal("attribute definition Value not found")
	}
	quantity, ok := valueType.Scope.LookupLocal("quantity")
	if !ok {
		t.Fatal("attribute quantity not found")
	}

	features := readProperty(t, evaluator, ElementOf(derived), "Type", "feature")
	if features.Kind != SequenceValue ||
		!containsValueElementSymbol(features.Values, local) ||
		!containsElementName(features.Values, "inherited") ||
		!containsElementName(features.Values, "start") ||
		!containsElementName(features.Values, "done") {
		t.Fatalf("Derived::feature = %#v, want local and Parts::Part features", features)
	}
	inherited := readProperty(t, evaluator, ElementOf(derived), "Type", "inheritedMembership")
	if inherited.Kind != SequenceValue ||
		!containsMembershipSymbol(inherited.Values, "inherited") ||
		!containsMembershipSymbol(inherited.Values, "start") ||
		!containsMembershipSymbol(inherited.Values, "done") {
		t.Fatalf("Derived::inheritedMembership = %#v, want local and Parts::Part memberships", inherited)
	}
	types := readProperty(t, evaluator, ElementOf(vehicle), "Feature", "type")
	if types.Kind != SequenceValue || !containsValueElementSymbol(types.Values, derived) {
		t.Fatalf("vehicle::type = %#v, want Derived", types)
	}
	definition := readProperty(t, evaluator, ElementOf(vehicle), "Usage", "definition")
	if definition.Kind != SequenceValue || len(definition.Values) != 1 || definition.Values[0].Element.Symbol != derived {
		t.Fatalf("vehicle::definition = %#v, want Derived", definition)
	}
	qualifiedName := readProperty(t, evaluator, ElementOf(local), "Element", "qualifiedName")
	if qualifiedName.Kind != StringValue || qualifiedName.String != "Vehicles::Derived::local" {
		t.Fatalf("local::qualifiedName = %#v", qualifiedName)
	}
	owner := readProperty(t, evaluator, ElementOf(local), "Element", "owner")
	if owner.Kind != ElementValue || owner.Element.Symbol != derived {
		t.Fatalf("local::owner = %#v, want Derived", owner)
	}
	namespace := readProperty(t, evaluator, ElementOf(local), "Element", "owningNamespace")
	if namespace.Kind != ElementValue || namespace.Element.Symbol != derived {
		t.Fatalf("local::owningNamespace = %#v, want Derived", namespace)
	}
	composite := readProperty(t, evaluator, ElementOf(vehicle), "Feature", "isComposite")
	if composite.Kind != BooleanValue || composite.Boolean {
		t.Fatalf("vehicle::isComposite = %#v, want false without an owning Type", composite)
	}
	reference := readProperty(t, evaluator, ElementOf(vehicle), "Usage", "isReference")
	if reference.Kind != BooleanValue || !reference.Boolean {
		t.Fatalf("vehicle::isReference = %#v, want true", reference)
	}
	direction := readProperty(t, evaluator, ElementOf(input), "Feature", "direction")
	if direction.Kind != EnumValue || direction.Enum != "in" {
		t.Fatalf("input::direction = %#v, want in", direction)
	}
	featuringType := readProperty(t, evaluator, ElementOf(quantity), "Feature", "featuringType")
	if featuringType.Kind != SequenceValue || !containsValueElementSymbol(featuringType.Values, valueType) {
		t.Fatalf("local::featuringType = %#v, want Derived", featuringType)
	}
	elementName := readProperty(t, evaluator, ElementOf(local), "Element", "name")
	if elementName.Kind != StringValue || elementName.String != "local" {
		t.Fatalf("local::name = %#v, want local", elementName)
	}
}

func readProperty(t *testing.T, evaluator *Evaluator, element Element, class, name string) Value {
	t.Helper()
	value, ok := evaluator.Property(element, class, name)
	if !ok {
		t.Fatalf("%s::%s was not derivable", class, name)
	}
	return value
}

func containsValueElementSymbol(values []Value, symbol *symbols.Symbol) bool {
	for _, value := range values {
		if value.Kind == ElementValue && value.Element.Symbol == symbol {
			return true
		}
	}
	return false
}

func containsElementName(values []Value, name string) bool {
	for _, value := range values {
		if value.Kind == ElementValue && value.Element.Symbol != nil && value.Element.Symbol.Name == name {
			return true
		}
	}
	return false
}

func containsMembershipSymbol(values []Value, name string) bool {
	for _, value := range values {
		if value.Kind == MembershipValue && value.Membership.Member != nil && value.Membership.Member.Name == name {
			return true
		}
	}
	return false
}
