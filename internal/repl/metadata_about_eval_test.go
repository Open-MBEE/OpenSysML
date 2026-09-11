package repl

import (
	"strings"
	"testing"
)

// aboutAnnotatedModel annotates one part inline and from `about` usages in
// other packages, one written before the part and one after, and annotates the
// package itself.
const aboutAnnotatedModel = `package Before {
    metadata Probe::Safety about Probe::seatBelt { level = 7; }
}
package Probe {
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
// answer the `about`-form annotations too, in textual order with the inline
// ones: the prompt evaluates against its own scope tree, so reaching them
// cannot depend on holding the indexed symbol.
func TestEvalMetadataReadsAboutAnnotations(t *testing.T) {
	s := NewSession()
	submitModel(t, s, aboutAnnotatedModel)

	got := metaOK(t, s, "%eval Probe::seatBelt.metadata")
	if strings.Count(got, "Instance(") != 4 {
		t.Errorf("seatBelt.metadata = %q, want the two inline annotations and the two `about` ones", got)
	}
	if got := metaOK(t, s, "%eval Probe::seatBelt.metadata#(1).level"); !strings.Contains(got, "7") {
		t.Errorf("the `about` annotation written first has level = %q, want the bound 7", got)
	}
	if got := metaOK(t, s, "%eval Probe::seatBelt.metadata#(2).level"); !strings.Contains(got, "3") {
		t.Errorf("the inline annotation's level = %q, want the bound 3", got)
	}
	if got := metaOK(t, s, "%eval Probe::seatBelt.metadata#(4).level"); !strings.Contains(got, "9") {
		t.Errorf("the `about` annotation written last has level = %q, want the bound 9", got)
	}
	if got := metaOK(t, s, "%eval Probe.metadata"); strings.Count(got, "Instance(") != 1 {
		t.Errorf("Probe.metadata = %q, want the annotation about the package", got)
	}
}
