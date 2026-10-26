package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// subrequirementModel nests requirement usages in a requirement definition:
// light takes the container's subject, pow declares its own (SysML v2 §7.21.2).
const subrequirementModel = `package Demo {
	private import ScalarValues::*;
	part def Eng { attribute power : Real; }
	part def Veh { attribute mass : Real; part eng : Eng; }
	requirement def PowerReq { subject e : Eng; require constraint { e.power > 100.0 } }
	requirement def R {
		subject v : Veh;
		requirement light { require constraint { v.mass < 10.0 } }
		requirement pow : PowerReq { subject e = v.eng; }
	}
	part heavy : Veh { attribute :>> mass = 20.0; part :>> eng { attribute :>> power = 200.0; } }
	part feather : Veh { attribute :>> mass = 5.0; part :>> eng { attribute :>> power = 200.0; } }
	requirement group : R;
}
`

// TestVerifyRequirementCountsSubrequirements: VerifyRequirement reports a
// requirement whose subrequirement fails for the supplied subject as not holding,
// naming the failing condition, and as holding when every subrequirement does.
func TestVerifyRequirementCountsSubrequirements(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, subrequirementModel, "subrequirements")

	verify := func(subject string) *pb.VerifyRequirementResponse {
		t.Helper()
		resp, err := srv.VerifyRequirement(context.Background(), &pb.VerifyRequirementRequest{
			ModelHash: hash, SymbolId: "Demo::group", SubjectSymbolId: subject,
		})
		if err != nil {
			t.Fatalf("VerifyRequirement by %s: %v", subject, err)
		}
		if resp.Error != "" {
			t.Fatalf("VerifyRequirement by %s reported %q", subject, resp.Error)
		}
		return resp
	}
	if resp := verify("Demo::heavy"); resp.Verdict.Holds || resp.Verdict.Condition == "" {
		t.Errorf("group by heavy: holds = %v, condition %q; want a failing v.mass < 10.0", resp.Verdict.Holds, resp.Verdict.Condition)
	}
	if resp := verify("Demo::feather"); !resp.Verdict.Holds {
		t.Errorf("group by feather: holds = false, want true (%s)", resp.Verdict.Condition)
	}
}
