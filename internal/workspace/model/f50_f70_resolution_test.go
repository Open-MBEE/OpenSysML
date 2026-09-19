package model

import (
	"strings"
	"testing"
)

func TestF50F70DeclaredFeatureAndRepresentationResolve(t *testing.T) {
	ws := NewWorkspace()
	ws.Open("f50-f70.sysml", []byte(`package P {
		classifier C;
		classifier D {
			abstract var feature x [0..*];
			inv check {
				x;
				rep inOCL language "ocl" /* self.x > 0.0 */
			}
		}
	}`), 1)

	for _, diagnostic := range ws.Diagnostics("f50-f70.sysml") {
		if strings.HasPrefix(diagnostic.Message, "unresolved reference:") {
			t.Fatalf("unexpected unresolved reference: %s", diagnostic.Message)
		}
	}
}
