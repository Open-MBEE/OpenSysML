package passes

import (
	"strings"
	"testing"
)

// wantDiags asserts the type diagnostics contain each want, one apiece.
func wantDiags(t *testing.T, src string, want ...string) {
	t.Helper()
	diags := exprDiags(t, src)
	if len(diags) != len(want) {
		t.Fatalf("expected %d type diagnostics, got %v", len(want), diags)
	}
	for i, w := range want {
		if !strings.Contains(diags[i].Message, w) {
			t.Fatalf("diagnostic %d = %q, want it to contain %q", i, diags[i].Message, w)
		}
	}
}

// TestAssignInStateEntryActionMustConform is the reported reproduction: an
// entry action of an exhibited state writing values of the wrong type.
func TestAssignInStateEntryActionMustConform(t *testing.T) {
	wantDiags(t, `package PE {
		part def Rig {
			attribute reading : ScalarValues::Real = 0.0;
			attribute label : ScalarValues::String = "a";

			exhibit state run {
				entry; then go;
				state go {
					entry action set {
						assign reading := "not a number";
						assign label := 7;
					}
				}
			}
		}
	}`,
		"cannot bind String value to a feature typed by Real",
		"cannot bind Natural value to a feature typed by String")
}

func TestAssignInActionBodyMustConform(t *testing.T) {
	wantOneDiag(t, `package P {
		action def Set {
			attribute reading : ScalarValues::Real = 0.0;
			action step {
				assign reading := "no";
			}
			first step;
		}
	}`, "cannot bind String value to a feature typed by Real")
}

func TestAssignInCalcBodyMustConform(t *testing.T) {
	wantOneDiag(t, `package P {
		calc def Scale {
			out total : ScalarValues::Integer;
			assign total := "no";
		}
	}`, "cannot bind String value to a feature typed by Integer")
}

func TestAssignInTransitionEffectMustConform(t *testing.T) {
	wantOneDiag(t, `package P {
		part def Rig {
			attribute reading : ScalarValues::Real = 0.0;
			exhibit state run {
				entry; then idle;
				state idle;
				state busy;
				transition first idle then busy do assign reading := "no";
			}
		}
	}`, "cannot bind String value to a feature typed by Real")
}

// TestAssignWidensNumerically: the numeric conformance the analyzer already
// applies to an initial value governs a write too.
func TestAssignWidensNumerically(t *testing.T) {
	wantNoDiags(t, `package P {
		action def Set {
			attribute reading : ScalarValues::Real = 0.0;
			action step {
				assign reading := 4;
			}
			first step;
		}
	}`)
}

// TestAssignOfAnUnknownTypeStaysSilent: where a value's type is not statically
// known the static pass says nothing and the run time is the enforcer.
func TestAssignOfAnUnknownTypeStaysSilent(t *testing.T) {
	wantNoDiags(t, `package P {
		part def Cell { attribute mark; }
		part def Board {
			part cell : Cell;
			attribute reading : ScalarValues::Real = 0.0;
			action def Set {
				action step {
					assign reading := cell.mark;
				}
				first step;
			}
		}
	}`)
}

// TestAssignMustSatisfyMultiplicity: the written collection answers to the
// target's multiplicity, by the rule an initial value answers to.
func TestAssignMustSatisfyMultiplicity(t *testing.T) {
	wantOneDiag(t, `package P {
		action def Set {
			attribute samples : ScalarValues::Integer[2] nonunique = (0, 0);
			action step {
				assign samples := (1, 2, 3);
			}
			first step;
		}
	}`, "3")
}
