package opensysml

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func (o *oldCaller) executeState(context.Context, *pb.ExecuteStateRequest) (*pb.ExecuteStateResponse, error) {
	o.t.Fatal("a schedule was sent to a service without schedule")
	return nil, nil
}

func (o *oldCaller) runAnalysis(context.Context, *pb.RunAnalysisRequest) (*pb.RunAnalysisResponse, error) {
	o.t.Fatal("a schedule was sent to a service without schedule")
	return nil, nil
}

// A policy is refused before it leaves the client when the service lacks the
// schedule capability, since such a service would run under the default
// instead: whether it predates the capability or the GetServerInfo RPC itself.
func TestAScheduleIsNotSentWithoutTheCapability(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	for name, old := range map[string]*oldCaller{
		"predates schedule":      {t: t, capabilities: []string{CapabilityVerification, CapabilityFeatureValues}},
		"predates GetServerInfo": {t: t, infoErr: &StatusError{Code: CodeUnimplemented, Message: "unknown method"}},
	} {
		t.Run(name, func(t *testing.T) {
			old.t = t
			c := &client{caller: old}
			_, err := c.ExecuteAction(ctx, model, "A", nil, WithSchedule("declared"))
			wantUnimplemented(t, "ExecuteAction", err)
			_, err = c.ExecuteState(ctx, model, "M", nil, WithSchedule("declared"))
			wantUnimplemented(t, "ExecuteState", err)
			_, err = c.RunAnalysis(ctx, model, "an", Schedule("declared"))
			wantUnimplemented(t, "RunAnalysis", err)
		})
	}
}
