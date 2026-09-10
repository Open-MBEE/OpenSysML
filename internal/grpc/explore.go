package grpc

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// exploredRun is what one explored run left for the response: its outcome's
// values and its notes, spelled while the run's own context was at hand.
type exploredRun struct {
	outputs     map[string]*pb.Value
	diagnostics []*pb.Diagnostic
}

// explored is what an exploration answered for the wire: its outcomes, how it
// ended, and the plan that ran it.
type explored struct {
	outcomes []*pb.Outcome
	status   *pb.ExplorationStatus
	plan     analysis.Plan
}

// explore puts the behavior's outcomes to the engines under selection, run
// performing it once per linearization on a fresh context; a failed run is an
// outcome, a diverging replay an error.
func (s *Service) explore(ctx context.Context, subject string, policy runtime.SchedulePolicy, selection analysis.Selection, cached *CachedModel, rs *runtimeSemantics, run func(*runtime.Context) (runtime.Outcome, error)) (explored, error) {
	var runs []exploredRun
	fresh := func() (*runtime.Context, error) {
		return s.newRuntimeOver(rs), nil
	}
	recorded := func(rt *runtime.Context) (runtime.Outcome, error) {
		outcome, err := run(rt)
		rec := exploredRun{
			diagnostics: s.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(rt.Notes(), cached)),
		}
		if err == nil && len(outcome.Outputs) > 0 {
			rec.outputs = make(map[string]*pb.Value, len(outcome.Outputs))
			for name, val := range outcome.Outputs {
				rec.outputs[name] = s.valueToProto(rt, val, cached.Index)
			}
		}
		runs = append(runs, rec)
		return outcome, err
	}
	plan, err := s.engines.Explore(ctx, &analysis.Model{Fresh: fresh}, subject, policy, recorded, analysis.BudgetOf(s.budgets, policy, analysis.Outcomes), selection)
	if err != nil {
		// A caller that went away is the call failing, not a precondition unmet.
		if ctx.Err() != nil {
			return explored{}, ctx.Err()
		}
		return explored{}, statusError(connect.CodeFailedPrecondition, err.Error())
	}
	x := plan.Result.Exploration()
	if x == nil {
		return explored{}, statusErrorf(connect.CodeFailedPrecondition, "exploration of %s reached no outcome: %s", subject, plan.Standing())
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
	return explored{outcomes: outcomes, status: status, plan: plan}, nil
}
