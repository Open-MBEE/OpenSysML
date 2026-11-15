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

// A type and a general are reported by the id GetSymbol reports them under: an escaped id,
// a quoted `@N` name beside an unnamed sibling, and a private member all read back as ids.
func TestTypeAndGeneralAreReportedByElementID(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := mustVerifyModel(t, srv, fmt.Sprintf(reportedIDCollision, "part def", "{ }")+`
package Uses {
	part def '@2'; part filler; part;
	private part def Hidden;
	part escaped : Col__V__c_pend0; part def EscapedSub :> Col__V__c_pend0;
	part quoted : '@2'; part def QuotedSub :> '@2';
	part hidden : Hidden; part def HiddenSub :> Hidden;
}
`, "reported-id-type")
	for user, want := range map[string]string{
		"Uses::escaped": "Col__V__c_pend0", "Uses::EscapedSub": "Col__V__c_pend0",
		"Uses::quoted": "@2", "Uses::QuotedSub": "@2",
		"Uses::hidden": "Hidden", "Uses::HiddenSub": "Hidden",
	} {
		resp, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
			ModelHash: hash, SymbolId: user, Lookup: pb.LookupKind_LOOKUP_KIND_QUALIFIED_NAME,
		})
		if err != nil || resp.GetSymbol() == nil {
			t.Fatalf("GetSymbol(%s): %v, %v", user, err, resp.GetError())
		}
		ids := []string{resp.Symbol.GetTypeInfo().GetResolvedId()}
		for _, spec := range resp.Symbol.Specializations {
			ids = append(ids, spec.TargetId)
		}
		read := 0
		for _, id := range ids {
			if id == "" {
				continue
			}
			read++
			target, err := srv.GetSymbol(context.Background(), &pb.GetSymbolRequest{
				ModelHash: hash, SymbolId: id, Lookup: pb.LookupKind_LOOKUP_KIND_ELEMENT_ID,
			})
			if err != nil || target.GetSymbol() == nil {
				t.Errorf("%s reports %q, which no element id reads: %v, %v", user, id, err, target.GetError())
				continue
			}
			if target.Symbol.Id != id || target.Symbol.Name != want {
				t.Errorf("%s reports %q, read as %s named %q, want the element named %q", user, id, target.Symbol.Id, target.Symbol.Name, want)
			}
		}
		if read == 0 {
			t.Errorf("%s reports no type or general", user)
		}
	}
}
