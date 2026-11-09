package export

import (
	"encoding/json"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/identity"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/metamodel"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/libs"
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
    part def Lone { attribute broken : Missing; }
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

// An integer beyond int64 keeps its decimal text rather than rounding through
// float64; a real form becomes a float64 unless float64 cannot hold it.
func TestElementFormValueKeepsLargeIntegersExact(t *testing.T) {
	cases := map[any]any{
		json.Number("9223372036854775809"): json.Number("9223372036854775809"),
		json.Number("42"):                  int64(42),
		json.Number("2.5"):                 2.5,
		json.Number("1e3"):                 1000.0,
		json.Number("1e400"):               json.Number("1e400"),
		uint64(1<<63 + 1):                  json.Number("9223372036854775809"),
	}
	for in, want := range cases {
		got, err := elementFormValue(in)
		if err != nil {
			t.Fatalf("elementFormValue(%v): %v", in, err)
		}
		if got != want {
			t.Errorf("elementFormValue(%v) = %#v, want %#v", in, got, want)
		}
	}
}

// A declared property the writer leaves out of one element's object — because
// the evaluator could not derive it for that element — is recorded with its
// reason, so a reader can tell it from a property that is legitimately empty.
func TestElementFormRecordsPerElementOmissions(t *testing.T) {
	form := parseElementForm(t, elementFormModel)
	a, ok := form.ByQualifiedName("M::Lone::broken")
	if !ok {
		t.Fatal("M::Lone::broken is not in the form")
	}
	reason, omitted := form.Omission(a.ID, "inheritedFeature")
	if !omitted || reason.Category != metamodel.ReasonUnresolvedReference {
		t.Fatalf("M::Lone::broken.inheritedFeature omission = %v, %v; want an unresolved-reference reason", reason, omitted)
	}
	for _, property := range a.Properties {
		if property.Name == "inheritedFeature" {
			t.Fatalf("M::Lone::broken carries inheritedFeature %v and records its omission", property.Value)
		}
	}
	typed, _ := form.ByQualifiedName("M::P::a")
	if _, omitted := form.Omission(typed.ID, "inheritedFeature"); omitted {
		t.Error("M::P::a.inheritedFeature is carried through its library type, yet an omission is recorded")
	}
	pair, _ := form.ByQualifiedName("M::Pair")
	if _, omitted := form.Omission(pair.ID, "inheritedFeature"); omitted {
		t.Error("M::Pair.inheritedFeature is carried, yet an omission is recorded")
	}
	unit, _ := form.ByQualifiedName("M::Unit")
	if _, omitted := form.Omission(unit.ID, "ownedSubclassification"); omitted {
		t.Error("M::Unit.ownedSubclassification is empty, yet an omission is recorded")
	}
	if _, omitted := form.Omission("no-such-id", "name"); omitted {
		t.Error("an unknown id records an omission")
	}
}

// A library element a model's form references is read from its own document's
// form under the same id, by id or by qualified name; references out of that
// document are readable the same way.
func TestLibraryElementFormReadsReferencedLibraryElements(t *testing.T) {
	form := parseElementForm(t, elementFormModel)
	pair, _ := form.ByQualifiedName("M::Pair")
	var portsID string
	for _, id := range formReferenceIDs(t, formPropertyOf(t, pair, "inheritedFeature")) {
		if name, _, _ := form.NameOf(id); name == "Parts::Part::ownedPorts" {
			portsID = id
		}
	}
	if portsID == "" {
		t.Fatal("M::Pair.inheritedFeature references no Parts::Part::ownedPorts")
	}
	ports, libraryForm, known, err := LibraryElementForm(portsID)
	if err != nil || !known || ports.ID != portsID || ports.Metaclass != "PortUsage" || ports.QualifiedName != "Parts::Part::ownedPorts" {
		t.Fatalf("LibraryElementForm(%s) = %+v, %v, %v", portsID, ports, known, err)
	}
	typeIDs := formReferenceIDs(t, formPropertyOf(t, ports, "type"))
	if len(typeIDs) != 1 {
		t.Fatalf("Parts::Part::ownedPorts.type = %v", typeIDs)
	}
	if name, _, ok := libraryForm.NameOf(typeIDs[0]); !ok || name != "Ports::Port" {
		t.Errorf("the library form names %s as %q, %v", typeIDs[0], name, ok)
	}
	port, _, known, err := LibraryElementForm(typeIDs[0])
	if err != nil || !known || port.QualifiedName != "Ports::Port" {
		t.Errorf("LibraryElementForm(%s) = %+v, %v, %v", typeIDs[0], port, known, err)
	}
	byName, _, known, err := LibraryElementForm("Parts::Part")
	if err != nil || !known || byName.Metaclass != "PartDefinition" {
		t.Errorf("LibraryElementForm(Parts::Part) = %+v, %v, %v", byName, known, err)
	}
	if _, _, known, err := LibraryElementForm("Parts::Nowhere"); known || err != nil {
		t.Errorf("an unknown name is known %v, %v", known, err)
	}
}

// overriddenLibrarySource serves one document's text in place of the library's.
type overriddenLibrarySource struct {
	libs.Source
	doc  string
	text []byte
}

func (s overriddenLibrarySource) Read(name string) ([]byte, error) {
	if name == s.doc {
		return s.text, nil
	}
	return s.Source.Read(name)
}

// A library document's form is read from the text the library index was built
// from — the configured library's, not the embedded copy's.
func TestLibraryElementFormReadsTheIndexedLibrarysText(t *testing.T) {
	shared := sharedLibraryIndex()
	part, ok := shared.catalog.ElementNamed("Parts::Part")
	if !ok {
		t.Fatal("Parts::Part is not catalogued")
	}
	doc := part.Symbol.DocName
	published, err := shared.source.Read(doc)
	if err != nil {
		t.Fatalf("read %s: %v", doc, err)
	}
	probe := strings.Replace(string(published), "abstract part def Part :> Item {", "abstract part def Part :> Item {\n\t\tattribute probeFromTheConfiguredLibrary : ScalarValues::Boolean;", 1)
	if probe == string(published) {
		t.Fatalf("%s declares Part otherwise than expected", doc)
	}
	index := &libraryIndex{
		model:     shared.model,
		catalog:   shared.catalog,
		anonymous: shared.anonymous,
		source:    overriddenLibrarySource{Source: shared.source, doc: doc, text: []byte(probe)},
	}
	form, err := index.documentElementForm(doc)
	if err != nil {
		t.Fatalf("documentElementForm(%s): %v", doc, err)
	}
	if _, ok := form.ByQualifiedName("Parts::Part::probeFromTheConfiguredLibrary"); !ok {
		t.Error("the form is read from other text than the index's source serves")
	}
	sharedForm, err := shared.documentElementForm(doc)
	if err != nil {
		t.Fatalf("shared documentElementForm(%s): %v", doc, err)
	}
	if _, ok := sharedForm.ByQualifiedName("Parts::Part::probeFromTheConfiguredLibrary"); ok {
		t.Error("an index's forms are cached on another index")
	}
}

// libraryMembersModel is the model whose `P::a` inherits, through Parts::Part,
// unnamed library members: documentation, invariants, connectors, a declared range.
const libraryMembersModel = `
package P {
    private import ScalarValues::*;
    part def Unit;
    part def V { part x : Unit; }
    part a : V;
}
`

// Every id an answer carries reads back: the owning membership of an unnamed
// library member and the member itself are written by the member's own library
// document under the ids the model's form referenced them by.
func TestLibraryElementFormReadsEveryIdAnInheritedMembershipCarries(t *testing.T) {
	form := parseElementForm(t, libraryMembersModel)
	a, ok := form.ByQualifiedName("P::a")
	if !ok {
		t.Fatal("no P::a")
	}
	readBack := func(id string) (APIJSONElement, bool) {
		if element, found := form.ByID(id); found {
			return element, true
		}
		element, _, known, err := LibraryElementForm(id)
		if err != nil || !known {
			t.Errorf("%s reads back as %v, %v", id, known, err)
			return APIJSONElement{}, false
		}
		return element, true
	}
	unnamed := map[string]int{}
	for _, property := range []string{"inheritedMembership", "inheritedFeature"} {
		ids := formReferenceIDs(t, formPropertyOf(t, a, property))
		if len(ids) == 0 {
			t.Fatalf("P::a.%s is empty", property)
		}
		for _, id := range ids {
			element, ok := readBack(id)
			if !ok {
				continue
			}
			if element.ID != id {
				t.Errorf("%s reads back as %s", id, element.ID)
			}
			if property != "inheritedMembership" {
				continue
			}
			for _, memberID := range formReferenceIDs(t, formPropertyOf(t, element, "memberElement")) {
				if member, ok := readBack(memberID); ok && member.QualifiedName == "" {
					unnamed[member.Metaclass]++
				}
			}
		}
	}
	for _, metaclass := range []string{"Documentation", "MultiplicityRange", "Connector", "BindingConnectorAsUsage"} {
		if unnamed[metaclass] == 0 {
			t.Errorf("P::a inherits no unnamed %s to read back; unnamed members by metaclass: %v", metaclass, unnamed)
		}
	}
}

// A library document's form writes its unnamed members under the ids this
// implementation mints for them (identity.AnonymousMemberOf), so the id a model's
// answer references them by is the one the form knows.
func TestLibraryElementFormWritesAnonymousMembersUnderTheirMintedIDs(t *testing.T) {
	index := sharedLibraryIndex()
	part, ok := index.catalog.ElementNamed("Parts::Part")
	if !ok {
		t.Fatal("Parts::Part is not catalogued")
	}
	parts, ok := index.catalog.ElementNamed("Parts::parts")
	if !ok {
		t.Fatal("Parts::parts is not catalogued")
	}
	cases := []struct{ id, metaclass, path string }{
		{identity.AnonymousMemberID(part.ID, "Documentation", 0), "Documentation", "Parts::Part/Documentation[0]"},
		{identity.AnonymousMemberOwningMembershipID(part.ID, "Documentation", 0), "OwningMembership", "Parts::Part/OwningMembership[0]"},
		{identity.AnonymousMemberID(parts.ID, "MultiplicityRange", 0), "MultiplicityRange", "Parts::parts/MultiplicityRange[0]"},
		{identity.AnonymousMemberOwningMembershipID(parts.ID, "MultiplicityRange", 0), "OwningMembership", "Parts::parts/OwningMembership[0]"},
	}
	for _, c := range cases {
		element, _, known, err := LibraryElementForm(c.id)
		if err != nil || !known {
			t.Errorf("LibraryElementForm(%s) for %s = %v, %v", c.id, c.path, known, err)
			continue
		}
		if element.ID != c.id || element.Metaclass != c.metaclass || element.QualifiedName != "" {
			t.Errorf("%s reads back as %+v", c.path, element)
		}
	}
}
