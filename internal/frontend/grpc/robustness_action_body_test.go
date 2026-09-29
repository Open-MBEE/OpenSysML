package grpc

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func TestGRPCRobustnessActionBodyStatements(t *testing.T) {
	service := mustNewService(t, 10)
	hash := mustParsedModel(t, service, `package Demo {
    action def A { first start; }
}`)

	root := &pb.AddSequenceEdit{
		Owner: "Demo::A", Keyword: "then", MemberKind: "if", Condition: "true",
	}
	current := root
	for range 130 {
		child := &pb.AddSequenceEdit{MemberKind: "if", Condition: "true"}
		current.Body = []*pb.AddSequenceEdit{child}
		current = child
	}
	_, err := service.ApplyEdits(context.Background(), &pb.ApplyEditsRequest{
		ModelHash: hash,
		Operations: []*pb.EditOperation{{
			Operation: &pb.EditOperation_AddSequence{AddSequence: root},
		}},
	})
	if connect.CodeOf(err) != connect.CodeInvalidArgument ||
		!strings.Contains(err.Error(), "maximum depth") {
		t.Fatalf("deep nested edit error = %v, want INVALID_ARGUMENT naming maximum depth", err)
	}
}
