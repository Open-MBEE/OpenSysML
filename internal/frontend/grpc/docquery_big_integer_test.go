package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
)

const bigIntegerDocumentModel = `
package D {
  private import DocumentQueries::*;
  private import KerML::Root::Element;
  private import ScalarValues::*;

  part def Tally { attribute count : Integer; }
  part ledger {
    part wide : Tally { attribute :>> count = 2 ** 70; }
  }
  part narrowLedger {
    part narrow : Tally { attribute :>> count = 2 ** 62; }
  }

  calc def Counts :> Query {
    in root : Element = ledger;
    Project(
      source = WhereType(source = Descendants(source = root, maxDepth = 1), type = "PartUsage"),
      properties = ("name", "count")
    )
  }
}
`

// A service withholding big_int_values refuses a document query bound to, or
// answering, an Integer beyond int64: DocumentValue has no unsupported arm to
// stand in for one. One within int64 is unaffected, and a service offering the
// capability answers the wide cell as big_int_value.
func TestBigIntCapabilityDocumentQuery(t *testing.T) {
	ctx := context.Background()
	withheld := mustNewServiceWithout(t, CapabilityBigIntValues)
	hash := mustParse(t, withheld, bigIntegerDocumentModel)

	_, err := withheld.RunDocumentQuery(ctx, &pb.RunDocumentQueryRequest{ModelHash: hash, QueryId: "D::Counts"})
	if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityBigIntValues) {
		t.Errorf("wide cell without %s: err = %v, want UNIMPLEMENTED naming the capability", CapabilityBigIntValues, err)
	}
	narrow, err := withheld.RunDocumentQuery(ctx, &pb.RunDocumentQueryRequest{
		ModelHash: hash, QueryId: "D::Counts",
		Bindings: []*pb.DocumentQueryBinding{binding("root", element("D::narrowLedger"))},
	})
	if err != nil {
		t.Fatalf("narrow cell without %s: %v", CapabilityBigIntValues, err)
	}
	if got := documentCount(t, narrow); got.GetIntValue() != 1<<62 {
		t.Errorf("narrow cell without %s = %v, want int_value 2**62", CapabilityBigIntValues, got)
	}

	wide := &pb.DocumentValue{Kind: &pb.DocumentValue_BigIntValue{BigIntValue: "1180591620717411303424"}}
	quantity := &pb.DocumentValue{Kind: &pb.DocumentValue_Quantity{Quantity: &pb.Quantity{
		Magnitude: &pb.Quantity_BigIntMagnitude{BigIntMagnitude: "1180591620717411303424"},
		UnitTerm:  &pb.UnitTerm{ScaleNum: 1, ScaleDen: 1},
	}}}
	event := &pb.DocumentValue{Kind: &pb.DocumentValue_Event{Event: &pb.DocumentEvent{Time: quantity}}}
	if !protoconv.DocumentValueHoldsBigInt(event) {
		t.Errorf("an event row timed by a wide quantity does not hold a wide Integer")
	}
	for name, bound := range map[string]*pb.DocumentValue{"wide": wide, "quantity": quantity} {
		_, err := withheld.RunDocumentQuery(ctx, &pb.RunDocumentQueryRequest{
			ModelHash: hash, QueryId: "D::Counts",
			Bindings: []*pb.DocumentQueryBinding{binding("root", bound)},
		})
		if connect.CodeOf(err) != connect.CodeUnimplemented || !strings.Contains(err.Error(), CapabilityBigIntValues) {
			t.Errorf("%s binding without %s: err = %v, want UNIMPLEMENTED naming the capability", name, CapabilityBigIntValues, err)
		}
	}

	available := mustNewService(t, 10)
	t.Cleanup(available.Close)
	availableHash := mustParse(t, available, bigIntegerDocumentModel)
	answered, err := available.RunDocumentQuery(ctx, &pb.RunDocumentQueryRequest{ModelHash: availableHash, QueryId: "D::Counts"})
	if err != nil {
		t.Fatalf("wide cell with %s: %v", CapabilityBigIntValues, err)
	}
	if got := documentCount(t, answered); got.GetBigIntValue() != "1180591620717411303424" {
		t.Errorf("wide cell = %v, want big_int_value 2**70", got)
	}
}

// documentCount is the count cell of a Counts answer's one row.
func documentCount(t *testing.T, resp *pb.RunDocumentQueryResponse) *pb.DocumentValue {
	t.Helper()
	if len(resp.Rows) != 1 || len(resp.Rows[0].Cells) != 2 || len(resp.Rows[0].Cells[1].Values) != 1 {
		t.Fatalf("Counts answered %v, want one row of a name and a count", resp)
	}
	return resp.Rows[0].Cells[1].Values[0]
}

// A vector holding a component beyond int64 is withheld whole, as unsupported,
// rather than left a vector with a null component no vector decoder takes.
func TestBigIntCapabilityVectorWithheldWhole(t *testing.T) {
	withheld := mustNewServiceWithout(t, CapabilityBigIntValues)
	vector := &pb.Value{Kind: &pb.Value_Vector{Vector: &pb.Vector{Components: []*pb.Value{
		{Kind: &pb.Value_IntValue{IntValue: 1}},
		{Kind: &pb.Value_BigIntValue{BigIntValue: "1180591620717411303424"}},
	}}}}
	if !protoconv.ValueHoldsBigInt(vector) {
		t.Errorf("a vector with a big_int_value component does not hold a wide Integer")
	}
	filtered := proto.Clone(vector).(*pb.Value)
	withheld.filterValueCapabilities(filtered)
	reason, ok := protoconv.UnsupportedReason(filtered)
	if !ok || !strings.Contains(reason, "1180591620717411303424") {
		t.Errorf("vector without %s = %v, want it reported unsupported whole, showing its components", CapabilityBigIntValues, filtered)
	}
	nested := &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{Elements: []*pb.Value{proto.Clone(vector).(*pb.Value)}}}}
	withheld.filterValueCapabilities(nested)
	if _, ok := protoconv.UnsupportedReason(nested.GetSequence().GetElements()[0]); !ok {
		t.Errorf("nested vector without %s = %v, want it reported unsupported whole", CapabilityBigIntValues, nested)
	}
	narrow := &pb.Value{Kind: &pb.Value_Vector{Vector: &pb.Vector{Components: []*pb.Value{{Kind: &pb.Value_IntValue{IntValue: 1}}}}}}
	withheld.filterValueCapabilities(narrow)
	if narrow.GetVector() == nil {
		t.Errorf("narrow vector without %s = %v, want it kept", CapabilityBigIntValues, narrow)
	}
}
