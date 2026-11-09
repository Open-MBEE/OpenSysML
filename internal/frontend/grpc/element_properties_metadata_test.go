package grpc

import (
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A metadata feature written as an annotation (`@M { … }`) is typed by the
// metadata definition it names (SysML 8.3.19.3), and a rational literal in its
// body states its value (KerML 8.3.4.8.x LiteralRational::value).
func TestGetElementPropertiesTypesAnAnnotationMetadataUsage(t *testing.T) {
	r := parseForElementProperties(t, `package P {
		private import ScalarValues::*;
		metadata def M { attribute k : Real; }
		part def D;
		@M { k = 2.5; }
	}`)
	first := func(id, name string) *pb.ElementPropertyValue {
		t.Helper()
		resp := r.read(id, name)
		if resp.Error != "" {
			t.Fatalf("%s.%s: %s", id, name, resp.Error)
		}
		if len(resp.Properties) != 1 || len(resp.Properties[0].Values) == 0 {
			t.Fatalf("%s.%s = %v", id, name, resp.Properties)
		}
		return resp.Properties[0].Values[0]
	}
	var metadata string
	for _, value := range r.read("P", "ownedElement").Properties[0].Values {
		if element := value.GetElement(); element != nil && element.Metaclass == "MetadataUsage" {
			metadata = element.Id
		}
	}
	if metadata == "" {
		t.Fatal("P owns no MetadataUsage")
	}
	for _, name := range []string{"type", "metadataDefinition", "definition"} {
		if got := first(metadata, name).GetElement(); got == nil || got.Id != "P__M" {
			t.Errorf("%s.%s = %v, want P__M", metadata, name, got)
		}
	}
	k := first(metadata, "ownedFeature").GetElement()
	if k == nil {
		t.Fatalf("%s owns no feature", metadata)
	}
	var valuation *pb.ElementRef
	for _, value := range r.read(k.Id, "ownedRelationship").Properties[0].Values {
		if element := value.GetElement(); element != nil && element.Metaclass == "FeatureValue" {
			valuation = element
		}
	}
	if valuation == nil {
		t.Fatalf("%s owns no FeatureValue", k.Id)
	}
	literal := first(valuation.Id, "value").GetElement()
	if literal == nil || literal.Metaclass != "LiteralRational" {
		t.Fatalf("%s.value = %v, want a LiteralRational", valuation.Id, literal)
	}
	if got := first(literal.Id, "value").GetRealValue(); got != 2.5 {
		t.Errorf("%s.value = %v, want 2.5", literal.Id, got)
	}
}
