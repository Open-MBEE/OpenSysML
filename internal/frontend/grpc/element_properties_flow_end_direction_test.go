package grpc

import (
	"slices"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A flow end's owned feature takes the direction of the feature it redefines, so the end's
// output and input derive (SysML v2 8.3.16.6 FlowUsage sourceOutputFeature, targetInputFeature).
func TestElementPropertiesDirectAFlowEndsFeature(t *testing.T) {
	r := parseForElementProperties(t, `package P {
	part def A { out x : A; in y : A; }
	part def V { part a : A; part b : A; flow f1 from a.x to b.y; }
}`)
	for _, tc := range []struct{ end, property, other string }{
		{"P__V__f1_pend0", "output", "input"},
		{"P__V__f1_pend1", "input", "output"},
	} {
		flowFeature := tc.end + "_pff"
		if got := propertyElementIDs(r.property(tc.end, tc.property).Values); !slices.Contains(got, flowFeature) {
			t.Errorf("%s.%s = %v, want its flow feature %s", tc.end, tc.property, got, flowFeature)
		}
		if got := propertyElementIDs(r.property(tc.end, tc.other).Values); slices.Contains(got, flowFeature) {
			t.Errorf("%s.%s = %v, want it without the flow feature", tc.end, tc.other, got)
		}
	}
}

// propertyElementIDs are the ids of the elements values name.
func propertyElementIDs(values []*pb.ElementPropertyValue) []string {
	var out []string
	for _, value := range values {
		out = append(out, value.GetElement().GetId())
	}
	return out
}
