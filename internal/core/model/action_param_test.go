package model

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/diag"
)

func TestActionParameterTyping(t *testing.T) {
	ws := NewWorkspace()

	// From Assignment Example.sysml lines 4-12
	src := `package Test {
	    action def StraightLineDynamics {
	        in power : ISQ::PowerValue;
	        in mass : ISQ::MassValue;
	        in delta_t : ISQ::TimeValue;
	        in x_in : ISQ::LengthValue;
	        in v_in : ISQ::SpeedValue;
	        out x_out : ISQ::LengthValue;
	        out v_out : ISQ::SpeedValue;
	    }
	}`

	ws.Open("test.sysml", []byte(src), 1)
	diags := ws.Diagnostics("test.sysml")

	var errs []string
	for _, d := range diags {
		if d.Severity == diag.SeverityError {
			errs = append(errs, d.Message)
			t.Logf("ERROR: %s", d.Message)
		}
	}

	if len(errs) > 0 {
		t.Fatalf("Expected no errors, got %d errors", len(errs))
	}
}
