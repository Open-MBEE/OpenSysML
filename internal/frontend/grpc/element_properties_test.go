package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// elementPropertiesModel has a specialization with inherited features, a
// conjugated port, two unnamed connectors (chained and plain ends) and a
// bound expression.
const elementPropertiesModel = `
package EP {
    port def CmdPort {
        out attribute cmd : ScalarValues::Integer;
        in attribute ack : ScalarValues::Boolean;
    }
    part def Unit { port p : CmdPort; port q : ~CmdPort; }
    part def Pair :> Unit {
        part u1 : Unit;
        part u2 : Unit;
        connect u1.p to u2.q;
        connect u1 to u2;
        attribute x : ScalarValues::Real = 2.0 + 1.0;
    }
}
`

type elementPropertiesReader struct {
	t    *testing.T
	srv  *Service
	hash string
}

func parseForElementProperties(t *testing.T, content string) *elementPropertiesReader {
	t.Helper()
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: content},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	for _, d := range parsed.Diagnostics {
		if d.Severity == "error" {
			t.Fatalf("unexpected diagnostic: %s", d.Message)
		}
	}
	return &elementPropertiesReader{t: t, srv: srv, hash: parsed.ModelHash}
}

func (r *elementPropertiesReader) read(element string, properties ...string) *pb.GetElementPropertiesResponse {
	r.t.Helper()
	resp, err := r.srv.GetElementProperties(context.Background(), &pb.GetElementPropertiesRequest{
		ModelHash: r.hash, ElementId: element, Properties: properties,
	})
	if err != nil {
		r.t.Fatalf("GetElementProperties(%s, %v): %v", element, properties, err)
	}
	return resp
}

func (r *elementPropertiesReader) property(element, name string) *pb.ElementProperty {
	r.t.Helper()
	resp := r.read(element, name)
	if resp.Error != "" {
		r.t.Fatalf("GetElementProperties(%s, %s): %s", element, name, resp.Error)
	}
	if len(resp.Properties) != 1 || resp.Properties[0].Name != name {
		r.t.Fatalf("GetElementProperties(%s, %s) answered %v", element, name, resp.Properties)
	}
	return resp.Properties[0]
}

func elementNames(values []*pb.ElementPropertyValue) []string {
	var names []string
	for _, value := range values {
		if element := value.GetElement(); element != nil {
			names = append(names, element.QualifiedName)
		}
	}
	return names
}

func containsAll(have []string, want ...string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if h == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestElementPropertiesReadsDerivedPropertiesBySpecName(t *testing.T) {
	r := parseForElementProperties(t, elementPropertiesModel)

	inherited := r.property("EP::Pair", "inheritedFeature")
	if inherited.DefiningClass != "Type" || !inherited.Many || !inherited.Derived || inherited.Unsupported != "" {
		t.Errorf("inheritedFeature declared as %+v", inherited)
	}
	names := elementNames(inherited.Values)
	if !containsAll(names, "EP::Unit::p", "EP::Unit::q") {
		t.Errorf("EP::Pair.inheritedFeature = %v, want EP::Unit::p and EP::Unit::q among them", names)
	}
	library := false
	for _, value := range inherited.Values {
		if element := value.GetElement(); element != nil && strings.HasPrefix(element.QualifiedName, "Parts::") {
			library = true
		}
	}
	if !library {
		t.Errorf("EP::Pair.inheritedFeature names no Parts:: library feature: %v", names)
	}

	direction := r.property("EP::CmdPort::cmd", "direction")
	if len(direction.Values) != 1 || direction.Values[0].GetStringValue() != "out" || direction.Many {
		t.Errorf("EP::CmdPort::cmd.direction = %v", direction.Values)
	}
	composite := r.property("EP::Pair::u1", "isComposite")
	if len(composite.Values) != 1 || !composite.Values[0].GetBoolValue() {
		t.Errorf("EP::Pair::u1.isComposite = %v", composite.Values)
	}

	value := r.property("EP::Pair::x", "value")
	if len(value.Values) != 1 || value.Values[0].GetElement().GetMetaclass() != "OperatorExpression" {
		t.Fatalf("EP::Pair::x.value = %v", value.Values)
	}
	expression := value.Values[0].GetElement().GetId()
	if operator := r.property(expression, "operator"); len(operator.Values) != 1 || operator.Values[0].GetStringValue() != "+" {
		t.Errorf("%s.operator = %v", expression, operator.Values)
	}
}

func TestElementPropertiesAddressesUnnamedElementsByPositionAndFormID(t *testing.T) {
	r := parseForElementProperties(t, elementPropertiesModel)
	everything := r.read("EP::Pair")
	if everything.Error != "" || everything.Element.GetMetaclass() != "PartDefinition" || everything.Element.GetQualifiedName() != "EP::Pair" {
		t.Fatalf("EP::Pair = %v (%s)", everything.Element, everything.Error)
	}
	if len(everything.Properties) < 10 {
		t.Errorf("an unselected read answers %d properties, want the element's whole form", len(everything.Properties))
	}
	var connectors []*pb.ElementRef
	for _, property := range everything.Properties {
		if property.Name != "ownedMember" {
			continue
		}
		for _, value := range property.Values {
			if value.GetElement().GetMetaclass() == "ConnectionUsage" {
				connectors = append(connectors, value.GetElement())
			}
		}
	}
	if len(connectors) != 2 {
		t.Fatalf("EP::Pair.ownedMember lists %d connectors, want 2", len(connectors))
	}
	chained, plain := connectors[0], connectors[1]
	if chained.QualifiedName != "" {
		t.Errorf("an unnamed connector is named %q", chained.QualifiedName)
	}
	ends := r.property(chained.Id, "connectorEnd")
	if len(ends.Values) != 2 {
		t.Fatalf("connectorEnd = %v", ends.Values)
	}
	end := ends.Values[0].GetElement().GetId()
	subsetting := r.property(end, "ownedReferenceSubsetting")
	if len(subsetting.Values) != 1 || subsetting.Values[0].GetElement().GetMetaclass() != "ReferenceSubsetting" {
		t.Fatalf("%s.ownedReferenceSubsetting = %v", end, subsetting.Values)
	}
	chain := r.property(subsetting.Values[0].GetElement().GetId(), "referencedFeature")
	if len(chain.Values) != 1 {
		t.Fatalf("referencedFeature = %v", chain.Values)
	}
	if segments := elementNames(r.property(chain.Values[0].GetElement().GetId(), "chainingFeature").Values); !containsAll(segments, "EP::Pair::u1", "EP::Unit::p") {
		t.Errorf("chainingFeature = %v, want EP::Pair::u1 then EP::Unit::p", segments)
	}
	related := r.property(plain.Id, "relatedFeature")
	if names := elementNames(related.Values); !containsAll(names, "EP::Pair::u1", "EP::Pair::u2") {
		t.Errorf("relatedFeature = %v, want EP::Pair::u1 and EP::Pair::u2", names)
	}
	if source := elementNames(r.property(plain.Id, "sourceFeature").Values); len(source) != 1 || source[0] != "EP::Pair::u1" {
		t.Errorf("sourceFeature = %v", source)
	}
	byPosition := r.read("EP::Pair::@3", "connectorEnd")
	if byPosition.Error != "" || byPosition.Element.GetId() != plain.Id {
		t.Errorf("the positional id answers %v (%s), want %s", byPosition.Element, byPosition.Error, plain.Id)
	}
}

// A library element a value references is readable by the id the value
// carries, and by its qualified name; what it answers is followable in turn.
func TestElementPropertiesFollowsReferencesIntoTheStandardLibrary(t *testing.T) {
	r := parseForElementProperties(t, elementPropertiesModel)
	inherited := r.property("EP::Pair", "inheritedFeature")
	var ports *pb.ElementRef
	for _, value := range inherited.Values {
		if element := value.GetElement(); element != nil && element.QualifiedName == "Parts::Part::ownedPorts" {
			ports = element
		}
	}
	if ports == nil {
		t.Fatalf("EP::Pair.inheritedFeature names no Parts::Part::ownedPorts: %v", elementNames(inherited.Values))
	}
	byID := r.read(ports.Id, "owner", "type", "isAbstract")
	if byID.Error != "" {
		t.Fatalf("GetElementProperties(%s): %s", ports.Id, byID.Error)
	}
	if byID.Element.QualifiedName != "Parts::Part::ownedPorts" || byID.Element.Metaclass != "PortUsage" || byID.Element.Id != ports.Id {
		t.Errorf("library element read by id = %+v", byID.Element)
	}
	owner := byID.Properties[0]
	if len(owner.Values) != 1 || owner.Values[0].GetElement().GetQualifiedName() != "Parts::Part" {
		t.Errorf("Parts::Part::ownedPorts.owner = %+v", owner)
	}
	typed := byID.Properties[1]
	if len(typed.Values) != 1 || typed.Values[0].GetElement().GetQualifiedName() != "Ports::Port" {
		t.Errorf("Parts::Part::ownedPorts.type = %+v", typed)
	}
	if abstract := byID.Properties[2]; abstract.Unsupported != "" || len(abstract.Values) != 1 || !abstract.Values[0].GetBoolValue() {
		t.Errorf("Parts::Part::ownedPorts.isAbstract = %+v", abstract)
	}

	acrossDocuments := r.read(typed.Values[0].GetElement().Id, "name", "isAbstract")
	if acrossDocuments.Error != "" || acrossDocuments.Element.QualifiedName != "Ports::Port" ||
		acrossDocuments.Properties[0].Values[0].GetStringValue() != "Port" || !acrossDocuments.Properties[1].Values[0].GetBoolValue() {
		t.Errorf("a reference into another library document answers %+v", acrossDocuments)
	}
	byName := r.read("Parts::Part", "ownedFeature")
	if byName.Error != "" || byName.Element.Id != owner.Values[0].GetElement().Id || !containsAll(elementNames(byName.Properties[0].Values), "Parts::Part::ownedPorts") {
		t.Errorf("a library element read by qualified name answers %+v", byName)
	}
	if unknown := r.read("Parts::Nowhere", "name"); unknown.Error == "" || !strings.Contains(unknown.Error, "element not found") {
		t.Errorf("an unknown library name answers %v", unknown)
	}
}

func TestElementPropertiesReportsWhyAPropertyHasNoAnswer(t *testing.T) {
	r := parseForElementProperties(t, elementPropertiesModel)

	undeclared := r.property("EP::Pair", "connectorEnd")
	if undeclared.DefiningClass != "" || !strings.Contains(undeclared.Unsupported, "PartDefinition declares no property connectorEnd") {
		t.Errorf("an undeclared property answers %+v", undeclared)
	}
	empty := r.property("EP::Pair", "ownedSubclassification")
	if empty.DefiningClass != "Classifier" || empty.Unsupported != "" || len(empty.Values) != 1 {
		t.Errorf("EP::Pair.ownedSubclassification = %+v", empty)
	}
	none := r.property("EP::Unit", "ownedSubclassification")
	if none.Unsupported != "" || len(none.Values) != 0 {
		t.Errorf("EP::Unit.ownedSubclassification = %+v, want declared and empty", none)
	}
	perElement := r.property("EP::CmdPort::cmd", "inheritedFeature")
	if len(perElement.Values) != 0 || !strings.Contains(perElement.Unsupported, "Type::inheritedFeature is not derived for this element") {
		t.Errorf("a property the evaluator could not derive for one element answers %+v", perElement)
	}
	if inherited := r.property("EP::Pair", "inheritedFeature"); inherited.Unsupported != "" || len(inherited.Values) == 0 {
		t.Errorf("EP::Pair.inheritedFeature = %+v, want derived", inherited)
	}
	omitted := r.property("EP::Pair", "ownedDisjoining")
	if omitted.Unsupported != "" && !strings.Contains(omitted.Unsupported, "not derived by OpenSysML") {
		t.Errorf("an omitted property answers %+v", omitted)
	}

	missing := r.read("EP::Nowhere", "name")
	if missing.Error == "" || !strings.Contains(missing.Error, "element not found") {
		t.Errorf("a missing element answers %v", missing)
	}
	if _, err := r.srv.GetElementProperties(context.Background(), &pb.GetElementPropertiesRequest{ModelHash: r.hash}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("an empty element_id: %v", err)
	}
	if _, err := r.srv.GetElementProperties(context.Background(), &pb.GetElementPropertiesRequest{ModelHash: "nope", ElementId: "EP"}); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("an unknown model: %v", err)
	}
}

func TestElementPropertiesRefusesUnreadableNotation(t *testing.T) {
	srv := mustNewService(t, 10)
	parsed, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: "package Broken { part def A { ; }"},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	resp, err := srv.GetElementProperties(context.Background(), &pb.GetElementPropertiesRequest{ModelHash: parsed.ModelHash, ElementId: "Broken"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Error == "" {
		t.Errorf("a model the parser could not read whole answers %v", resp.Properties)
	}
}

func TestElementPropertiesCapabilityIsAdvertised(t *testing.T) {
	srv := mustNewService(t, 10)
	info, err := srv.GetServerInfo(context.Background(), &pb.ServerInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, capability := range info.Capabilities {
		if capability == CapabilityElementProperties {
			found = true
		}
	}
	if !found {
		t.Errorf("capabilities %v lack %q", info.Capabilities, CapabilityElementProperties)
	}
}
