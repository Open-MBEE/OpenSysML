package grpc

import (
	"context"
	"fmt"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// A connector end's generated id spells this declaration's name, so the declaration is
// reported under an escaped id: the id GetSymbol reports must name it in every other RPC.
const reportedIDCollision = `%s Col__V__c_pend0 %s
package Col { port def P; part def V { port q : P; port r : P; connection c connect q to r; } }
`

// escapedID is the id GetSymbol reports for the colliding declaration, which is not its name.
func escapedID(t *testing.T, srv *Service, hash string) string {
	t.Helper()
	const name = "Col__V__c_pend0"
	resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
		ModelHash: hash, SymbolId: name, Lookup: pb.LookupKind_LOOKUP_KIND_QUALIFIED_NAME,
	})
	if err != nil || resp.GetSymbol() == nil {
		t.Fatalf("GetSymbol(%s): %v, %v", name, err, resp.GetError())
	}
	id := resp.Symbol.Id
	if id == name {
		t.Fatalf("%s is reported under its own name: the model states no collision", name)
	}
	return id
}

func TestEvaluateCalcReadsTheReportedID(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, fmt.Sprintf(reportedIDCollision, "calc def",
		"{ return : ScalarValues::Integer = 2 + 3; }"), "reported-id-calc")
	id := escapedID(t, srv, hash)
	resp, err := srv.EvaluateCalc(context.Background(), &pb.EvaluateCalcRequest{ModelHash: hash, SymbolId: id})
	if err != nil {
		t.Fatalf("EvaluateCalc(%s): %v", id, err)
	}
	if resp.Error != "" {
		t.Fatalf("EvaluateCalc(%s) reported %q", id, resp.Error)
	}
	if got := soleElement(t, "result", resp.Result).GetIntValue(); got != 5 {
		t.Errorf("EvaluateCalc(%s) = %d, want 5", id, got)
	}
}

func TestExecuteActionReadsTheReportedID(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, fmt.Sprintf(reportedIDCollision, "action def",
		"{ out y : ScalarValues::Integer = 7; }"), "reported-id-action")
	id := escapedID(t, srv, hash)
	resp, err := srv.ExecuteAction(context.Background(), &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: id})
	if err != nil {
		t.Fatalf("ExecuteAction(%s): %v", id, err)
	}
	if resp.Error != "" {
		t.Fatalf("ExecuteAction(%s) reported %q", id, resp.Error)
	}
	if _, ok := resp.Outputs["y"]; !ok {
		t.Errorf("ExecuteAction(%s) reported no y: %v", id, resp.Outputs)
	}
}

func TestQueryScopeReadsTheReportedID(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, fmt.Sprintf(reportedIDCollision, "package", "{ part marker; }"), "reported-id-scope")
	id := escapedID(t, srv, hash)
	resp, err := srv.Query(context.Background(), &pb.QueryRequest{ModelHash: hash, Query: &pb.Query{Scope: []string{id}}})
	if err != nil {
		t.Fatalf("Query(scope %s): %v", id, err)
	}
	if len(resp.Elements) != 2 {
		t.Errorf("Query(scope %s) answered %d elements, want the package and its marker: %v", id, len(resp.Elements), resp.Elements)
	}
}
