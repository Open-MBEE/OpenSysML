package opensysml

import (
	"context"
	"testing"
	"time"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"google.golang.org/protobuf/proto"
)

func (o *oldCaller) runSweep(context.Context, *pb.RunSweepRequest) (*pb.RunSweepResponse, error) {
	o.t.Fatal("a sweep was sent to a service that does not announce what it needs")
	return nil, nil
}

// sweepRecorder announces the capabilities given and answers every sweep with
// reply, keeping the request it was sent.
type sweepRecorder struct {
	caller
	capabilities []string
	reply        *pb.RunSweepResponse
	sent         *pb.RunSweepRequest
}

func (r *sweepRecorder) serverInfo(context.Context) (*pb.ServerInfoResponse, error) {
	return &pb.ServerInfoResponse{Version: "test", Capabilities: r.capabilities}, nil
}

func (r *sweepRecorder) runSweep(_ context.Context, req *pb.RunSweepRequest) (*pb.RunSweepResponse, error) {
	r.sent = req
	return r.reply, nil
}

// Every option reaches the request field the service reads it from.
func TestSweepRequestCarriesEveryOption(t *testing.T) {
	recorder := &sweepRecorder{capabilities: []string{CapabilityVerification, CapabilityEngines}, reply: &pb.RunSweepResponse{}}
	c := &client{caller: recorder}
	_, err := c.Sweep(context.Background(), &Model{Hash: "h"}, "P::case",
		[]SweepRange{
			{Parameter: "a", Start: Int(1), End: Int(3), Step: Int(2)},
			{Parameter: "b", Start: Real(0.5), End: Real(1.5)},
		},
		SweepSubject("P::s"), SweepArguments(Int(7), Bool(true)), SweepArgument("k", String("v")),
		SweepSamples(5), SweepSeed(11), SweepEngine("run"))
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	intValue := func(n int64) *pb.Value { return &pb.Value{Kind: &pb.Value_IntValue{IntValue: n}} }
	realValue := func(f float64) *pb.Value { return &pb.Value{Kind: &pb.Value_RealValue{RealValue: f}} }
	want := &pb.RunSweepRequest{
		ModelHash: "h", SymbolId: "P::case", SubjectSymbolId: "P::s",
		Arguments:      []*pb.Value{intValue(7), {Kind: &pb.Value_BoolValue{BoolValue: true}}},
		NamedArguments: map[string]*pb.Value{"k": {Kind: &pb.Value_StringValue{StringValue: "v"}}},
		Ranges: []*pb.SweepRange{
			{Parameter: "a", Start: intValue(1), End: intValue(3), Step: intValue(2)},
			{Parameter: "b", Start: realValue(0.5), End: realValue(1.5)},
		},
		Samples: 5, Seed: 11, Engine: "run",
	}
	if !proto.Equal(recorder.sent, want) {
		t.Errorf("sent\n%v\nwant\n%v", recorder.sent, want)
	}

	if _, err := c.Sweep(context.Background(), &Model{Hash: "h"}, "P::case", nil, SweepEngine(EngineAuto)); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if !proto.Equal(recorder.sent, &pb.RunSweepRequest{ModelHash: "h", SymbolId: "P::case"}) {
		t.Errorf("a sweep stating nothing sent %v, want only the model and the symbol", recorder.sent)
	}
}

// Every field of the response and of its rows reaches the Sweep returned.
func TestSweepDecodesEveryResponseField(t *testing.T) {
	intValue := func(n int64) *pb.Value { return &pb.Value{Kind: &pb.Value_IntValue{IntValue: n}} }
	recorder := &sweepRecorder{capabilities: []string{CapabilityVerification}, reply: &pb.RunSweepResponse{
		Rows: []*pb.SweepRow{
			{
				Inputs:        []*pb.CalcOutput{{Name: "n", Value: intValue(1)}},
				Outputs:       []*pb.CalcOutput{{Name: "result", Value: intValue(2)}},
				Verdicts:      []*pb.Verdict{{Kind: "objective", Holds: true}},
				ElapsedMicros: 1500,
				Evaluations:   []*pb.CaseEvaluation{{FunctionId: "P::f", Arguments: []*pb.Value{intValue(1)}, Result: intValue(2), Selected: true}},
			},
			{
				Inputs: []*pb.CalcOutput{{Name: "n", Value: intValue(2)}},
				Error:  "boom", FailureReason: pb.FailureReason_FAILURE_REASON_EVALUATION,
			},
		},
		Parameters: []string{"n"}, Sampled: true, Seed: 9,
		Diagnostics: []*pb.Diagnostic{{Message: "note"}},
		Instances:   []*pb.Instance{{Id: 4, TypeSymbolId: "P::s"}},
		Engine:      "run", Strength: "sample",
		Bounds: []*pb.Bound{{Name: "steps", Limit: 10, Reached: true}},
	}}
	c := &client{caller: recorder}
	sweep, err := c.Sweep(context.Background(), &Model{Hash: "h"}, "P::case", nil)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if len(sweep.Rows) != 2 || len(sweep.Parameters) != 1 || sweep.Parameters[0] != "n" || !sweep.Sampled || sweep.Seed != 9 {
		t.Fatalf("sweep = %+v, want two sampled rows over n from seed 9", sweep)
	}
	if sweep.Engine != "run" || sweep.Strength != "sample" || len(sweep.Bounds) != 1 || sweep.Bounds[0].Name != "steps" || sweep.Bounds[0].Limit != 10 || !sweep.Bounds[0].Reached {
		t.Errorf("standing = %+v, want run/sample stopped at 10 steps", sweep.Standing)
	}
	if len(sweep.Diagnostics) != 1 || sweep.Diagnostics[0].Message != "note" {
		t.Errorf("diagnostics = %+v, want the note", sweep.Diagnostics)
	}
	if got := sweep.Instance(4); got == nil || got.TypeSymbolID != "P::s" || sweep.Instance(5) != nil {
		t.Errorf("instances = %+v, want instance 4 of P::s alone", sweep.Instances)
	}
	ran, failed := sweep.Rows[0], sweep.Rows[1]
	if len(ran.Inputs) != 1 || ran.Inputs[0].Name != "n" || ran.Inputs[0].Value != Int(1) ||
		len(ran.Outputs) != 1 || ran.Outputs[0].Name != "result" || ran.Outputs[0].Value != Int(2) ||
		len(ran.Verdicts) != 1 || !ran.Verdicts[0].Holds || ran.Elapsed != 1500*time.Microsecond || ran.Error != "" || ran.Reason != 0 {
		t.Errorf("row 0 = %+v, want n=1 -> 2 holding after 1.5ms", ran)
	}
	if len(ran.Evaluations) != 1 || ran.Evaluations[0].FunctionID != "P::f" || ran.Evaluations[0].Result != Int(2) || !ran.Evaluations[0].Selected {
		t.Errorf("row 0 evaluations = %+v, want P::f(1) = 2 selected", ran.Evaluations)
	}
	if failed.Error != "boom" || failed.Reason != ReasonEvaluation || len(failed.Evaluations) != 0 {
		t.Errorf("row 1 = %+v, want boom with reason EVALUATION", failed)
	}
}

// A sweep is refused before it leaves the client without the verification
// capability, and a named engine without engines, as its sibling calls are.
func TestSweepIsNotSentWithoutItsCapabilities(t *testing.T) {
	ctx := context.Background()
	model := &Model{Hash: "h"}
	ranges := []SweepRange{{Parameter: "n", Start: Int(1), End: Int(2)}}

	c := &client{caller: &oldCaller{t: t, capabilities: []string{CapabilityRuns}}}
	_, err := c.Sweep(ctx, model, "A", ranges)
	wantMissingCapability(t, "Sweep", err, CapabilityVerification)

	c = &client{caller: &oldCaller{t: t, capabilities: []string{CapabilityVerification}}}
	_, err = c.Sweep(ctx, model, "A", ranges, SweepEngine("run"))
	wantUnimplemented(t, "Sweep with an engine", err)

	_, err = c.Sweep(ctx, model, "A", []SweepRange{{Parameter: "z", Start: Complex(1), End: Int(2)}})
	wantUnimplemented(t, "Sweep over a Complex", err)
}
