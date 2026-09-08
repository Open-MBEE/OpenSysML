package grpc

import (
	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// exploredRun is what one explored run left for the response: its outcome's
// values and its notes, spelled while the run's own context was at hand.
type exploredRun struct {
	outputs     map[string]*pb.Value
	diagnostics []*pb.Diagnostic
}

// explore runs a behavior once per linearization, each run on a fresh context over
// the semantics held; a run that fails is an outcome, only a diverging replay fails the call.
func (s *Service) explore(policy runtime.SchedulePolicy, cached *CachedModel, rs *runtimeSemantics, run func(*runtime.Context) (runtime.Outcome, error)) ([]*pb.Outcome, *pb.ExplorationStatus, error) {
	var runs []exploredRun
	fresh := func() (*runtime.Context, error) {
		return s.newRuntimeOver(rs), nil
	}
	recorded := func(ctx *runtime.Context) (runtime.Outcome, error) {
		outcome, err := run(ctx)
		rec := exploredRun{
			diagnostics: s.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(ctx.Notes(), cached)),
		}
		if err == nil && len(outcome.Outputs) > 0 {
			rec.outputs = make(map[string]*pb.Value, len(outcome.Outputs))
			for name, val := range outcome.Outputs {
				rec.outputs[name] = s.valueToProto(ctx, val, cached.Index)
			}
		}
		runs = append(runs, rec)
		return outcome, err
	}
	x, err := runtime.Explore(policy, fresh, recorded)
	if err != nil {
		return nil, nil, statusError(connect.CodeFailedPrecondition, err.Error())
	}
	outcomes := make([]*pb.Outcome, 0, len(x.Outcomes))
	for _, o := range x.Outcomes {
		witness := runs[o.WitnessRun-1]
		out := &pb.Outcome{
			Outputs:        witness.outputs,
			FinalState:     o.Outcome.FinalState,
			StatesVisited:  o.Outcome.StateVisits,
			Linearizations: int32Clamp(o.Linearizations),
			Diagnostics:    witness.diagnostics,
		}
		if o.Outcome.Err != nil {
			out.Error = o.Outcome.Err.Error()
		}
		for _, c := range o.Witness {
			out.Witness = append(out.Witness, c.String())
		}
		outcomes = append(outcomes, out)
	}
	status := &pb.ExplorationStatus{
		Complete:    x.Complete(),
		Runs:        int32Clamp(x.Runs),
		BudgetsHit:  x.BudgetsHit,
		RunsBudget:  int32Clamp(x.Budget.Runs),
		DepthBudget: int32Clamp(x.Budget.Depth),
	}
	return outcomes, status, nil
}
