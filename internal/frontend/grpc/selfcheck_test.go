package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// selfCheckModelSource declares a case whose subject is not its first
// parameter, which SysMLValidation rejects, and a rule package whose one
// constraint has no typed input, which the check skips with a warning.
const selfCheckModelSource = `package S10 {
	part def P;
	case def CD {
		in x : P;
		subject s : P;
	}
}
package Rules {
	constraint def Untyped { in x; true }
}`

func selfCheck(t *testing.T, srv *Service, req *pb.SelfCheckRequest) *pb.SelfCheckResponse {
	t.Helper()
	resp, err := srv.SelfCheck(context.Background(), req)
	if err != nil {
		t.Fatalf("SelfCheck: %v", err)
	}
	return resp
}

// TestSelfCheckReportsViolations verifies the check applies the bundled
// constraints to every element and reports each violation as a finding with
// its element, location and the line the CLI prints.
func TestSelfCheckReportsViolations(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, selfCheckModelSource)

	resp := selfCheck(t, srv, &pb.SelfCheckRequest{ModelHash: hash})
	if resp.Error != "" {
		t.Fatalf("SelfCheck reported %q", resp.Error)
	}
	if resp.Status != "fails" || resp.Violations == 0 || resp.Checks < resp.Violations || resp.Elements == 0 {
		t.Fatalf("status %q, %d elements, %d checks, %d violations; want a failing check", resp.Status, resp.Elements, resp.Checks, resp.Violations)
	}
	var violation *pb.SelfCheckFinding
	for _, f := range resp.Findings {
		if f.Status == "fails" && f.Element == "S10::CD" {
			violation = f
		}
	}
	if violation == nil {
		t.Fatalf("findings %v; want S10::CD failing", resp.Findings)
	}
	if violation.Constraint == "" || !strings.HasPrefix(violation.Location, "<content>:") ||
		violation.Line != violation.Location+": error[self-check-rule]: "+violation.Constraint+" fails for S10::CD" {
		t.Errorf("violation = %v; want its constraint, location and line", violation)
	}
	if !strings.HasPrefix(resp.Summary, "Self-model check: ") || len(resp.Applications) == 0 {
		t.Errorf("summary %q, applications %v; want the totals", resp.Summary, resp.Applications)
	}
	var applied int64
	for _, a := range resp.Applications {
		applied += a.Count
	}
	if applied != resp.Checks {
		t.Errorf("applications total %d; want checks %d", applied, resp.Checks)
	}
}

// TestSelfCheckRulePackages verifies a rule package's constraints apply after
// the bundled ones, that an untyped one is skipped with a warning, and that a
// package nothing declares is the in-band error.
func TestSelfCheckRulePackages(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, selfCheckModelSource)

	resp := selfCheck(t, srv, &pb.SelfCheckRequest{ModelHash: hash, SelfCheckPackage: []string{"Rules"}})
	if resp.Error != "" {
		t.Fatalf("SelfCheck reported %q", resp.Error)
	}
	warnings := 0
	for _, f := range resp.Findings {
		if f.Status == "warning" && strings.Contains(f.Message, "Rules::Untyped is skipped") && strings.HasPrefix(f.Line, f.Location+": warning: ") {
			warnings++
		}
		if f.Status == "warning" && strings.Contains(f.Message, "Rules has no applicable constraint definitions") && f.Line == "warning: "+f.Message {
			warnings++
		}
	}
	if warnings != 2 {
		t.Errorf("findings %v; want the skipped constraint and the empty package warned about", resp.Findings)
	}

	missing := selfCheck(t, srv, &pb.SelfCheckRequest{ModelHash: hash, SelfCheckPackage: []string{"Nope"}})
	if !strings.Contains(missing.Error, "Nope") || len(missing.Findings) != 0 || missing.Checks != 0 {
		t.Errorf("unknown rule package: error %q, %d findings, %d checks; want the error alone", missing.Error, len(missing.Findings), missing.Checks)
	}
}

// TestGRPCRobustnessSelfCheck verifies the typed refusals of SelfCheck.
func TestGRPCRobustnessSelfCheck(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustParse(t, srv, selfCheckModelSource)
	t.Run("unknown model", func(t *testing.T) {
		_, err := srv.SelfCheck(context.Background(), &pb.SelfCheckRequest{ModelHash: "nope"})
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("got %v; want NOT_FOUND", err)
		}
	})
	t.Run("unreadable package name", func(t *testing.T) {
		resp := selfCheck(t, srv, &pb.SelfCheckRequest{ModelHash: hash, SelfCheckPackage: []string{"not a name"}})
		if !strings.Contains(resp.Error, "not a name") {
			t.Fatalf("error = %q; want the name reported", resp.Error)
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := srv.SelfCheck(ctx, &pb.SelfCheckRequest{ModelHash: hash})
		if err == nil {
			t.Log("a cancelled context answered before the check noticed; the check ends all the same")
		}
	})
}
