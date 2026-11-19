package repl

import "testing"

// %features renders no verdict for an objective's `verify r;`: the member references a
// requirement, stating none, so it is neither satisfied nor violated on its own.
func TestFeatureValuesGiveAVerifyMemberNoVerdict(t *testing.T) {
	s := NewSession()
	if res := s.Submit(`package V {
		requirement def LightEnough { attribute mass : ScalarValues::Real; require constraint { mass < 10.0 } }
		part def Probe { attribute mass : ScalarValues::Real = 5.0; }
		requirement lightEnough : LightEnough;
		verification def Weigh {
			subject probe : Probe;
			objective { verify lightEnough; }
		}
	}`); len(errorDiagnostics(res.Diagnostics)) > 0 {
		t.Fatalf("model did not analyse cleanly: %v", res.Diagnostics)
	}
	run(t, s, "%instantiate V::Weigh::obj")
	got := run(t, s, "%features V::Weigh::obj")
	wants(t, got, "lightEnough = Instance(")
	rejects(t, got, "<requirement:")
}
