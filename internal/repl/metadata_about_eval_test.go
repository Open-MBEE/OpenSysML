package repl

import (
	"strings"
	"testing"
)

// aboutAnnotatedModel annotates one part inline and from an `about` usage in
// another package, and annotates the package itself.
const aboutAnnotatedModel = `package Probe {
    private import ScalarValues::*;
    metadata def Safety { attribute level : Integer = 2; }
    metadata def Legacy;
    part def Vehicle;
    part seatBelt : Vehicle {
        @Safety { level = 3; }
        @Legacy;
    }
}
package About {
    metadata Probe::Safety about Probe::seatBelt { level = 9; }
    metadata Probe::Legacy about Probe;
}`

// TestEvalMetadataReadsAboutAnnotations requires `.metadata` at the prompt to
// answer the `about`-form annotations too: the prompt evaluates against its own
// scope tree, so reaching them cannot depend on holding the indexed symbol.
func TestEvalMetadataReadsAboutAnnotations(t *testing.T) {
	s := NewSession()
	submitModel(t, s, aboutAnnotatedModel)

	got := metaOK(t, s, "%eval Probe::seatBelt.metadata")
	if strings.Count(got, "Instance(") != 3 {
		t.Errorf("seatBelt.metadata = %q, want the two inline annotations and the `about` one", got)
	}
	if got := metaOK(t, s, "%eval Probe::seatBelt.metadata#(3).level"); !strings.Contains(got, "9") {
		t.Errorf("the `about` annotation's level = %q, want the bound 9", got)
	}
	if got := metaOK(t, s, "%eval Probe.metadata"); strings.Count(got, "Instance(") != 1 {
		t.Errorf("Probe.metadata = %q, want the annotation about the package", got)
	}
}
