package passes

import "testing"

func TestW8DVerificationOutsideObjective(t *testing.T) {
	const src = `package P {
		requirement def R {
			verify requirement : R;
		}
		verification def VC {
			requirement {
				verify r;
			}
		}
		case def Plan {
			objective {
				verify r;
			}
		}
		requirement r : R;
	}`
	w8dWantLines(t, src, "verification-owning-type", 3, 7, 12)
}

// A `verify` in the objective of a verification case definition or usage stays
// silent, and a `satisfy` is no verification.
func TestW8DVerificationInObjectiveStaysSilent(t *testing.T) {
	const src = `package P {
		requirement def R;
		requirement r : R;
		part p;
		verification def VC {
			objective {
				verify requirement : R;
				verify r;
			}
		}
		verification vc : VC {
			objective {
				verify r;
			}
		}
		satisfy r by p;
	}`
	if diags := only(w8dDiags(t, src), "verification-owning-type"); len(diags) != 0 {
		t.Fatalf("legal verifications reported: %v", diags)
	}
}
