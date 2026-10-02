package passes

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
)

func TestBehaviorOrderFeaturingDiagnostics(t *testing.T) {
	const src = `package P {
		part def Robot { perform action move; perform action grip; }
		part r : Robot;
		first r::move then r::grip;
		part part1 { action action1; }
		requirement requirement1;
		first part1::action1 then requirement1;
		part b { action g; action m; }
		first b.g then b.m;
		part p1 { action a; }
		part p2 { action b; }
		first p1::a then p2::b;
	}`
	diags := constraintDiags(t, src)
	featuring := only(diags, "connector-type-featuring")
	if len(featuring) != 1 || featuring[0].Severity != diag.SeverityError ||
		featuring[0].Message != msgConnectorTypeFeaturing {
		t.Fatalf("connector-type-featuring diagnostics = %+v, want only the exact error for p1::a then p2::b", featuring)
	}
	nothing := only(diags, "succession-orders-nothing")
	if len(nothing) != 1 || nothing[0].Severity != diag.SeverityWarning ||
		nothing[0].Message == "" {
		t.Fatalf("succession-orders-nothing diagnostics = %+v, want one requirement-end warning", nothing)
	}
}

func TestBehaviorOrderPartsOnlyHasNoDiagnostic(t *testing.T) {
	diags := constraintDiags(t, `package P {
		part a;
		part b;
		first a then b;
	}`)
	if got := append(only(diags, "succession-orders-nothing"), only(diags, "connector-type-featuring")...); len(got) != 0 {
		t.Fatalf("non-behavior succession diagnostics = %+v, want none", got)
	}
}

func TestBehaviorOrderEventsOnlyHasNoDiagnostic(t *testing.T) {
	diags := constraintDiags(t, `package P {
		part def Events {
			event occurrence first;
			event occurrence second;
			first first then second;
		}
	}`)
	if got := only(diags, "succession-orders-nothing"); len(got) != 0 {
		t.Fatalf("event succession diagnostics = %+v, want none", got)
	}
}

func TestBehaviorOrderInterfaceFlowsHaveNoDiagnostic(t *testing.T) {
	diags := constraintDiags(t, `package P {
		interface i {
			flow f1;
			flow f2;
			first f1.start then f2.done;
		}
	}`)
	if got := only(diags, "succession-orders-nothing"); len(got) != 0 {
		t.Fatalf("interface-flow succession diagnostics = %+v, want none", got)
	}
}

func TestBehaviorOrderEventAndPerformedActionWarns(t *testing.T) {
	diags := constraintDiags(t, `package P {
		part def Sequenced {
			event occurrence event;
			perform action grip;
			first event then grip;
		}
	}`)
	nothing := only(diags, "succession-orders-nothing")
	if len(nothing) != 1 || nothing[0].Severity != diag.SeverityWarning {
		t.Fatalf("event/action succession diagnostics = %+v, want one warning", nothing)
	}
}

func TestBehaviorOrderCycleWarning(t *testing.T) {
	diags := constraintDiags(t, `part def Base {
		action a;
		action b;
		first a then b;
	}
	part def Derived :> Base {
		action :>> a;
		action :>> b;
		first b then a;
	}`)
	cycles := only(diags, "succession-order-cycle")
	if len(cycles) == 0 || cycles[0].Severity != diag.SeverityWarning {
		t.Fatalf("succession-order-cycle diagnostics = %+v, want a warning", cycles)
	}
}

func TestBehaviorOrderCycleUsesTheWholeFeaturePath(t *testing.T) {
	diags := constraintDiags(t, `part def Arm {
		action earlier;
		action later;
	}
	part def Robot {
		part left : Arm;
		part right : Arm;
		first left.earlier then left.later;
		first right.later then right.earlier;
	}`)
	if cycles := only(diags, "succession-order-cycle"); len(cycles) != 0 {
		t.Fatalf("succession-order-cycle diagnostics = %+v, want none for unrelated feature paths", cycles)
	}
}

func TestBehaviorOrderDoesNotFeatureNamespaceParserFixtures(t *testing.T) {
	for _, name := range []string{"behavior_namespace_succession.sysml", "succession_as_usage.sysml"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "tests", "parser", "testdata", "parse", name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			diags := constraintDiags(t, string(data))
			if got := only(diags, "connector-type-featuring"); len(got) != 0 {
				t.Fatalf("connector-type-featuring diagnostics = %+v, want none", got)
			}
		})
	}
}
