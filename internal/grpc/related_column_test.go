package grpc

import (
	"context"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// traceMatrixFixture declares a requirement matrix whose columns are
// relationship-derived: satisfier and verifier lists, a count and a flag.
const traceMatrixFixture = "../core/queryexec/testdata/trace_matrix.sysml"

// elementIDs spells a cell's element values by qualified name.
func elementIDs(cell *pb.DocumentQueryCell) []string {
	ids := make([]string, 0, len(cell.Values))
	for _, value := range cell.Values {
		ids = append(ids, value.GetElementId())
	}
	return ids
}

// TestRunDocumentQueryCarriesRelatedColumns: a RelatedColumn list is answered as
// every related element in traversal order, an empty list as an empty cell,
// and the count and any aggregates as an integer and a Boolean.
func TestRunDocumentQueryCarriesRelatedColumns(t *testing.T) {
	srv := mustNewService(t, 10)
	hash := parseFixture(t, srv, traceMatrixFixture)

	resp, err := srv.RunDocumentQuery(context.Background(), &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: "Observatory::Matrix",
		Bindings: []*pb.DocumentQueryBinding{binding("root", element("Observatory"))},
	})
	if err != nil {
		t.Fatalf("RunDocumentQuery: %v", err)
	}
	var columns []string
	for _, column := range resp.Columns {
		columns = append(columns, column.Name)
	}
	if got := strings.Join(columns, ","); got != "name,satisfiedBy,verifiedBy,verifications,verified" {
		t.Fatalf("columns = %s", got)
	}
	if len(resp.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(resp.Rows))
	}

	mass := resp.Rows[0]
	if got := strings.Join(elementIDs(mass.Cells[1]), ","); got != "Observatory::telescope,Observatory::groundStation" {
		t.Errorf("mass satisfiedBy = %s", got)
	}
	if got := mass.Cells[1].Values[0].GetElementType(); got != "PartUsage" {
		t.Errorf("satisfier type = %s, want PartUsage", got)
	}
	if got := strings.Join(elementIDs(mass.Cells[2]), ","); got != "Observatory::massVerification,Observatory::pointingVerification" {
		t.Errorf("mass verifiedBy = %s", got)
	}
	if got := mass.Cells[3].Values; len(got) != 1 || got[0].GetIntValue() != 2 {
		t.Errorf("mass verifications = %v, want 2", got)
	}
	if got := mass.Cells[4].Values; len(got) != 1 || !got[0].GetBoolValue() {
		t.Errorf("mass verified = %v, want true", got)
	}

	data := resp.Rows[2]
	if len(data.Cells[1].Values) != 0 || len(data.Cells[2].Values) != 0 {
		t.Errorf("data requirement related cells = %v %v, want empty", data.Cells[1].Values, data.Cells[2].Values)
	}
	if got := data.Cells[3].Values; len(got) != 1 || got[0].GetIntValue() != 0 {
		t.Errorf("data verifications = %v, want 0", got)
	}
	if got := data.Cells[4].Values; len(got) != 1 || got[0].GetBoolValue() {
		t.Errorf("data verified = %v, want false", got)
	}
}
