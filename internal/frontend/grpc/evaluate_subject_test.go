package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// subjectModel is a definition with a default and a usage redefining it, the
// shape a subject is meant to tell apart.
const subjectModel = `
package Demo {
  part def Vehicle {
    attribute mass = 1500.0;
  }
  part sedan : Vehicle {
    attribute :>> mass = 1200.0;
  }
}
`

// evaluateIn parses subjectModel and evaluates expression with the given context
// and subject.
func evaluateIn(t *testing.T, expression, contextID, subjectID string) *pb.EvaluateResponse {
	t.Helper()
	srv := mustNewService(t, 10)

	parseResp, err := srv.ParseFile(context.Background(), &pb.ParseFileRequest{
		Source:      &pb.ParseFileRequest_Content{Content: subjectModel},
		ContentHash: "evaluate-subject",
	})
	if err != nil {
		t.Fatalf("ParseFile failed: %v", err)
	}
	for _, diag := range parseResp.Diagnostics {
		if diag.Severity == "ERROR" {
			t.Fatalf("unexpected parse diagnostic: %s", diag.Message)
		}
	}

	resp, err := srv.Evaluate(context.Background(), &pb.EvaluateRequest{
		ModelHash:       parseResp.ModelHash,
		Expression:      expression,
		ContextSymbolId: contextID,
		SubjectSymbolId: subjectID,
	})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}
	return resp
}

// TestEvaluateWithSubjectReadsThatObject verifies a subject binds the object a
// feature is read from, so a redefined value wins over the declared default.
func TestEvaluateWithSubjectReadsThatObject(t *testing.T) {
	resp := evaluateIn(t, "mass", "", "Demo::sedan")
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if got := resp.Result.GetRealValue(); got != 1200.0 {
		t.Errorf("mass against Demo::sedan = %v, want 1200", got)
	}
}

// TestEvaluateWithSubjectAppliesToExpressions verifies the bound object is read
// wherever the feature appears, not only as the whole expression.
func TestEvaluateWithSubjectAppliesToExpressions(t *testing.T) {
	resp := evaluateIn(t, "mass * 2", "", "Demo::sedan")
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if got := resp.Result.GetRealValue(); got != 2400.0 {
		t.Errorf("mass * 2 against Demo::sedan = %v, want 2400", got)
	}
}

// TestEvaluateWithSubjectInNamedContext verifies a named context still resolves
// the expression while the subject supplies the object read from.
func TestEvaluateWithSubjectInNamedContext(t *testing.T) {
	resp := evaluateIn(t, "mass", "Demo::Vehicle", "Demo::sedan")
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if got := resp.Result.GetRealValue(); got != 1200.0 {
		t.Errorf("mass in Demo::Vehicle against Demo::sedan = %v, want 1200", got)
	}
}

// TestEvaluateWithoutSubjectReadsTheDeclaredDefault verifies the no-subject call
// is unchanged: a scope alone reads the declaration.
func TestEvaluateWithoutSubjectReadsTheDeclaredDefault(t *testing.T) {
	resp := evaluateIn(t, "mass", "Demo::Vehicle", "")
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if got := resp.Result.GetRealValue(); got != 1500.0 {
		t.Errorf("mass in Demo::Vehicle = %v, want 1500", got)
	}
}

// TestEvaluateSubjectNotFound verifies an unknown subject is reported in-band
// rather than evaluated as something else.
func TestEvaluateSubjectNotFound(t *testing.T) {
	resp := evaluateIn(t, "mass", "", "Demo::nosuch")
	if !strings.Contains(resp.Error, "subject not found: Demo::nosuch") {
		t.Errorf("error = %q, want it to name the missing subject", resp.Error)
	}
	if resp.Result != nil {
		t.Errorf("expected no result, got %v", resp.Result)
	}
}
