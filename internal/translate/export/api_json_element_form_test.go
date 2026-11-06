package export

import (
	"encoding/json"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

const elementFormModel = `package M {
    port def P { in attribute a : ScalarValues::Integer; out attribute b : ScalarValues::Real; }
    part def Unit { port p : P; }
    part def Pair :> Unit {
        part u1 : Unit;
        part u2 : Unit;
        connect u1.p to u2.p;
        attribute x : ScalarValues::Real = 2.0 + 1.0;
    }
}
`

func parseElementForm(t *testing.T, text string) *ElementForm {
	t.Helper()
	sf := source.New("m.sysml", []byte(text))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) > 0 {
		t.Fatalf("parse: %v", p.Diagnostics)
	}
	form, err := ModelElementForm([]ModelDocument{{File: sf, Root: root}})
	if err != nil {
		t.Fatal(err)
	}
	return form
}

func formPropertyOf(t *testing.T, element APIJSONElement, name string) any {
	t.Helper()
	for _, property := range element.Properties {
		if property.Name == name {
			return property.Value
		}
	}
	t.Fatalf("%s has no %s; has %v", element.ID, name, element.Properties)
	return nil
}

func formReferenceIDs(t *testing.T, value any) []string {
	t.Helper()
	var out []string
	switch v := value.(type) {
	case APIJSONReference:
		out = append(out, v.ID)
	case []any:
		for _, member := range v {
			ref, ok := member.(APIJSONReference)
			if !ok {
				t.Fatalf("collection member %T is no reference", member)
			}
			out = append(out, ref.ID)
		}
	default:
		t.Fatalf("%T is no reference value", value)
	}
	return out
}

func TestElementFormReadsDerivedPropertiesPerElement(t *testing.T) {
	form := parseElementForm(t, elementFormModel)

	pair, ok := form.ByQualifiedName("M::Pair")
	if !ok {
		t.Fatal("M::Pair not in the form")
	}
	if pair.Metaclass != "PartDefinition" {
		t.Fatalf("M::Pair metaclass %q", pair.Metaclass)
	}
	if again, ok := form.ByID(pair.ID); !ok || again.QualifiedName != "M::Pair" {
		t.Fatalf("ByID(%q) = %v, %v", pair.ID, again.QualifiedName, ok)
	}
	inherited := formReferenceIDs(t, formPropertyOf(t, pair, "inheritedFeature"))
	unitP, _ := form.ByQualifiedName("M::Unit::p")
	foundOwn, foundLibrary := false, false
	for _, id := range inherited {
		if id == unitP.ID {
			foundOwn = true
		}
		if name, metaclass, ok := form.NameOf(id); ok && name != "" && metaclass != "" && id != unitP.ID {
			foundLibrary = true
		}
	}
	if !foundOwn {
		t.Fatalf("M::Pair.inheritedFeature %v lacks M::Unit::p (%s)", inherited, unitP.ID)
	}
	if !foundLibrary {
		t.Fatalf("no library-inherited feature of %v resolves to a name", inherited)
	}

	var connector APIJSONElement
	for _, element := range form.Elements() {
		if element.Metaclass == "ConnectionUsage" {
			connector = element
		}
	}
	if connector.ID == "" {
		t.Fatal("no ConnectionUsage in the form")
	}
	if connector.QualifiedName != "" {
		t.Fatalf("an unnamed connector has qualified name %q", connector.QualifiedName)
	}
	if ends := formReferenceIDs(t, formPropertyOf(t, connector, "connectorEnd")); len(ends) != 2 {
		t.Fatalf("connectorEnd = %v", ends)
	}

	x, _ := form.ByQualifiedName("M::Pair::x")
	valueID := formReferenceIDs(t, formPropertyOf(t, x, "value"))[0]
	expression, ok := form.ByID(valueID)
	if !ok || expression.Metaclass != "OperatorExpression" {
		t.Fatalf("M::Pair::x.value %q is %v (%v)", valueID, expression.Metaclass, ok)
	}
	if operator := formPropertyOf(t, expression, "operator"); operator != "+" {
		t.Fatalf("operator = %v", operator)
	}
}

func TestElementFormValuesHaveDocumentedShapes(t *testing.T) {
	form := parseElementForm(t, elementFormModel)
	var check func(id, name string, value any)
	check = func(id, name string, value any) {
		switch v := value.(type) {
		case nil, string, bool, int64, float64, APIJSONReference, json.Number:
		case []any:
			for _, member := range v {
				check(id, name, member)
			}
		default:
			t.Errorf("%s.%s holds a %T", id, name, value)
		}
	}
	for _, element := range form.Elements() {
		if element.ID == "" || element.Metaclass == "" {
			t.Errorf("element without id or metaclass: %+v", element)
		}
		for _, property := range element.Properties {
			check(element.ID, property.Name, property.Value)
		}
	}
}
