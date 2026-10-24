package opensysml_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/client/opensysml"
)

// sole is the one element of a one-element Sequence: what a bare (`[0..*]`) action
// parameter holding one value reports as.
func sole(t *testing.T, name string, v opensysml.Value) opensysml.Value {
	t.Helper()
	seq, ok := v.(opensysml.Sequence)
	if !ok || len(seq) != 1 {
		t.Fatalf("%s = %#v, want a one-element Sequence", name, v)
	}
	return seq[0]
}
