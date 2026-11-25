package opensysml

import (
	"context"
	"os"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// countingCaller counts Query and GetSymbol RPCs: a listing that re-resolves
// each subject itself would issue one GetSymbol per row.
type countingCaller struct {
	*inprocess
	queries int
	symbols int
}

func (c *countingCaller) query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	c.queries++
	return c.inprocess.query(ctx, req)
}

func (c *countingCaller) getSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error) {
	c.symbols++
	return c.inprocess.getSymbol(ctx, req)
}

// TestQueryBoundSubjectNeedsNoPerRowLookup counts one Query and zero GetSymbol
// RPCs: every case's bound declaration is already on its row.
func TestQueryBoundSubjectNeedsNoPerRowLookup(t *testing.T) {
	ctx := context.Background()
	data, err := os.ReadFile("../../tests/testdata/bound-subject/verification_subjects.sysml")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	svc, err := sysmlgrpc.NewService(16, "test")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	parsed, err := svc.ParseFile(ctx, &pb.ParseFileRequest{
		Source: &pb.ParseFileRequest_Content{Content: string(data)},
	})
	if err != nil {
		t.Fatalf("ParseFile: %v", err)
	}
	counter := &countingCaller{inprocess: &inprocess{svc: svc}}
	client := &client{caller: counter}
	elements, err := client.Query(ctx, &Model{Hash: parsed.ModelHash}, Query{
		Select: []string{"boundSubject"},
	})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	got := map[string]string{}
	for _, element := range elements {
		if value, ok := element.Properties["boundSubject"]; ok {
			got[element.ID] = value
		}
	}
	if got["Verification::maintainedSizing"] != "Structure::boilerAsMaintained" ||
		got["Verification::spareSizing"] != "Structure::spareBoiler" {
		t.Errorf("boundSubject rows = %v, want both bound declarations", got)
	}
	if counter.queries != 1 {
		t.Errorf("queries = %d, want one Query for the whole listing", counter.queries)
	}
	if counter.symbols != 0 {
		t.Errorf("getSymbol calls = %d, want none: the row must carry the FQN already", counter.symbols)
	}
}
