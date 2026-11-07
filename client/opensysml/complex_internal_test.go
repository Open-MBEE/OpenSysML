package opensysml

import (
	"context"
	"errors"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// oldCaller stands in for a service that predates complex_values: it answers
// ServerInfo without the capability and must never be sent a Complex.
type oldCaller struct {
	caller
	t            *testing.T
	infoErr      error
	capabilities []string
}

func (o *oldCaller) serverInfo(context.Context) (*pb.ServerInfoResponse, error) {
	if o.infoErr != nil {
		return nil, o.infoErr
	}
	return &pb.ServerInfoResponse{Version: "old", Capabilities: o.capabilities}, nil
}

func (o *oldCaller) executeAction(context.Context, *pb.ExecuteActionRequest) (*pb.ExecuteActionResponse, error) {
	o.t.Fatal("a Complex input was sent to a service without complex_values")
	return nil, nil
}

func (o *oldCaller) evaluateCalc(context.Context, *pb.EvaluateCalcRequest) (*pb.EvaluateCalcResponse, error) {
	o.t.Fatal("a Complex argument was sent to a service without complex_values")
	return nil, nil
}

func (o *oldCaller) instancePaths(context.Context, *pb.InstancePathsRequest) (*pb.InstancePathsResponse, error) {
	o.t.Fatal("InstancePaths was sent to a service without instance_paths")
	return nil, nil
}

func (o *oldCaller) close() error { return nil }

// InstancePaths is refused before it leaves the client when the service lacks
// instance_paths, as the other optional calls are.
func TestInstancePathsIsNotSentWithoutTheCapability(t *testing.T) {
	c := &client{caller: &oldCaller{t: t, capabilities: []string{CapabilityFeatureValues}}}
	_, err := c.InstancePaths(context.Background(), &Model{Hash: "h"}, ObjectRef{InstanceID: 1})
	wantUnimplemented(t, "InstancePaths", err)
}

// A Complex is refused before it leaves the client when the service lacks
// complex_values, since such a service would read it as null: whether the
// service predates the capability or the GetServerInfo RPC itself.
func TestComplexInputIsNotSentWithoutComplexValues(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	for name, old := range map[string]*oldCaller{
		"predates complex_values": {t: t, capabilities: []string{CapabilityFeatureValues}},
		"predates GetServerInfo":  {t: t, infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			for label, input := range map[string]Value{
				"complex": Complex(complex(1, 2)),
				"nested":  Sequence{Int(1), Sequence{Complex(complex(1, 2))}},
			} {
				_, err := c.ExecuteAction(ctx, model, "A", map[string]Value{"z": input})
				wantUnimplemented(t, "ExecuteAction "+label, err)
				_, err = c.EvaluateCalc(ctx, model, "f", Int(1), input)
				wantUnimplemented(t, "EvaluateCalc "+label, err)
			}
		})
	}
}

// A call sending no Complex asks nothing of the service beyond the call itself.
func TestOnlyAComplexInputNeedsComplexValues(t *testing.T) {
	sent := false
	old := &oldCaller{t: t, infoErr: errors.New("ServerInfo must not be asked")}
	c := &client{caller: &plainCaller{oldCaller: old, sent: &sent}}
	_, err := c.ExecuteAction(context.Background(), &Model{Hash: "h"}, "A",
		map[string]Value{"n": Int(1), "xs": Sequence{Real(1.5), String("s")}})
	if err != nil || !sent {
		t.Errorf("ExecuteAction without a Complex: err = %v, sent = %v; want it sent without a preflight", err, sent)
	}
}

type plainCaller struct {
	*oldCaller
	sent *bool
}

func (p *plainCaller) executeAction(context.Context, *pb.ExecuteActionRequest) (*pb.ExecuteActionResponse, error) {
	*p.sent = true
	return &pb.ExecuteActionResponse{}, nil
}

func wantUnimplemented(t *testing.T, op string, err error) {
	t.Helper()
	var status *StatusError
	if !errors.As(err, &status) || status.Code != CodeUnimplemented {
		t.Errorf("%s: err = %v, want CodeUnimplemented", op, err)
	}
}
