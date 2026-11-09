package grpc

import (
	"strings"
	"testing"
)

// TestGetElementPropertiesReadsInheritedFeaturesThroughALibraryType covers the
// features a workspace attribute inherits through a standard-library type.
func TestGetElementPropertiesReadsInheritedFeaturesThroughALibraryType(t *testing.T) {
	r := parseForElementProperties(t, `package P {
		private import ScalarValues::*;
		item def Ping { attribute seq : Integer; }
	}`)
	for _, property := range []string{"inheritedFeature", "feature"} {
		resp := r.read("P__Ping__seq", property)
		if len(resp.Properties) != 1 {
			t.Fatalf("%s: %d properties answered, want 1", property, len(resp.Properties))
		}
		answered := resp.Properties[0]
		if answered.Unsupported != "" {
			t.Fatalf("%s: unsupported: %s", property, answered.Unsupported)
		}
		var names []string
		for _, value := range answered.Values {
			names = append(names, value.GetElement().GetQualifiedName())
		}
		if joined := strings.Join(names, " "); !strings.Contains(joined, "Base::DataValue::self") {
			t.Errorf("%s = %s, want Base::DataValue::self among them", property, joined)
		}
	}
}
