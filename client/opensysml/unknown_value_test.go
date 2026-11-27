package opensysml_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protowire"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/frontend/grpc"
)

// startNewerService is a service whose every Evaluate answers with a value arm
// this client's schema does not have, nested one level inside a sequence.
func startNewerService(t *testing.T) string {
	t.Helper()
	svc, err := sysmlgrpc.NewService(16, "test")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	t.Cleanup(svc.Close)
	newer := connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			resp, err := next(ctx, req)
			if answer, ok := resp.(*connect.Response[pb.EvaluateResponse]); err == nil && ok {
				unknown := &pb.Value{}
				unknown.ProtoReflect().SetUnknown(protowire.AppendVarint(protowire.AppendTag(nil, 9999, protowire.VarintType), 1))
				answer.Msg.Result = &pb.Value{Kind: &pb.Value_Sequence{Sequence: &pb.ValueSequence{
					Elements: []*pb.Value{answer.Msg.Result, unknown},
				}}}
			}
			return resp, err
		}
	})
	mux := http.NewServeMux()
	mux.Handle(protoconnect.NewSysMLServiceHandler(sysmlgrpc.NewConnectAdapter(svc), connect.WithInterceptors(newer)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

// A value of a kind this client does not know is a refusal, in either body
// encoding, rather than a null the service never sent.
func TestAnUnknownValueArmIsUnimplemented(t *testing.T) {
	address := startNewerService(t)
	for name, opts := range map[string][]opensysml.DialOption{
		"protobuf": nil,
		"json":     {opensysml.WithJSONBody()},
	} {
		t.Run(name, func(t *testing.T) {
			client := dialClient(t, address, opts...)
			ctx := context.Background()
			model, err := client.ParseSource(ctx, vehicleSource)
			if err != nil {
				t.Fatalf("ParseSource: %v", err)
			}
			value, err := client.Evaluate(ctx, model, "2 + 2")
			if !errors.Is(err, opensysml.CodeUnimplemented) {
				t.Fatalf("Evaluate = %v, %v; want CodeUnimplemented", value, err)
			}
		})
	}
}

// The same service's answers without the unknown arm still read.
func TestKnownValuesStillRead(t *testing.T) {
	client := dialClient(t, startService(t))
	ctx := context.Background()
	model, err := client.ParseSource(ctx, vehicleSource)
	if err != nil {
		t.Fatalf("ParseSource: %v", err)
	}
	value, err := client.Evaluate(ctx, model, "(1, 2)")
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if sequence, ok := value.(opensysml.Sequence); !ok || len(sequence) != 2 {
		t.Fatalf("Evaluate = %#v, want a sequence of two", value)
	}
}
