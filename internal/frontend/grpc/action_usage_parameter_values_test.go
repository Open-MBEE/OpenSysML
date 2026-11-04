package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

const actionUsageParameterValuesSource = `package P {
	action def Step {
		in x;
		out y;
		first start;
		then assign y := x + 1;
	}
	action s : Step {
		in x = 9;
	}
	action r : Step {
		in x redefines x = 9;
	}
}
`

func executeActionUsage(t *testing.T, srv *Service, hash, action string, inputs map[string]*pb.Value) int64 {
	t.Helper()
	resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{
		ModelHash:      hash,
		ActionSymbolId: action,
		Inputs:         inputs,
	})
	if err != nil {
		t.Fatalf("ExecuteAction(%s): %v", action, err)
	}
	if resp.Error != "" {
		t.Fatalf("ExecuteAction(%s) reported %q", action, resp.Error)
	}
	y, ok := resp.Outputs["y"]
	if !ok {
		t.Fatalf("ExecuteAction(%s) reported no y: %v", action, resp.Outputs)
	}
	return soleElement(t, "y", y).GetIntValue()
}

// ExecuteAction on a usage with no inputs performs it with its own `in` value,
// whether the parameter redefines the definition's by name or explicitly.
func TestExecuteActionUsageWithoutInputsUsesItsOwnValues(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, actionUsageParameterValuesSource, "execute-action-usage-values")
	for _, action := range []string{"P::s", "P::r"} {
		if got := executeActionUsage(t, srv, hash, action, nil); got != 10 {
			t.Errorf("%s: y = %d, want 10", action, got)
		}
	}
	inputs := map[string]*pb.Value{"x": {Kind: &pb.Value_IntValue{IntValue: 4}}}
	if got := executeActionUsage(t, srv, hash, "P::s", inputs); got != 5 {
		t.Errorf("P::s with x = 4: y = %d, want 5", got)
	}
}

// EvaluateCalc on a usage whose output invokes another calc from its inputs
// reports the computed output.
func TestEvaluateCalcUsageNestedInvocation(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, `package P {
	calc def Area { in w; in h; w * h }
	calc def Volume {
		in w; in h; in d;
		out r = Area(w, h) * d;
	}
	calc box : Volume { in w = 2; in h = 3; in d = 4; }
}
`, "evaluate-calc-usage-nested")
	resp, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{ModelHash: hash, SymbolId: "P::box"})
	if err != nil {
		t.Fatalf("EvaluateCalc: %v", err)
	}
	if resp.Error != "" {
		t.Fatalf("EvaluateCalc reported %q", resp.Error)
	}
	if len(resp.Outputs) != 1 || resp.Outputs[0].Name != "r" {
		t.Fatalf("got outputs %v, want r alone", resp.Outputs)
	}
	if got := soleElement(t, "r", resp.Outputs[0].Value).GetIntValue(); got != 24 {
		t.Errorf("r = %d, want 24", got)
	}
}
