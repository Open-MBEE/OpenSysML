package grpc

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// explored is what an exploration answered for the wire: its outcomes, how it
// ended, and the plan that ran it.
type explored struct {
	outcomes []*pb.Outcome
	status   *pb.ExplorationStatus
	plan     analysis.Plan
}

// explore puts the behavior's outcomes to the engines under selection, run
// performing it once per linearization on a context of its own on one of the plan's
// workers; a failed run is an outcome, a diverging replay an error. An outcome's values
// and notes are spelled from its witness run's context, which the outcome keeps.
func (s *Service) explore(ctx context.Context, subject string, policy runtime.SchedulePolicy, selection analysis.Selection, cached *CachedModel, run func(*runtime.Context) (runtime.Outcome, error)) (explored, error) {
	plan, err := s.engines.Explore(ctx, s.model(cached), subject, policy, run, analysis.BudgetOf(s.budgets, policy, analysis.Outcomes, s.jobs), selection)
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
		out := &pb.Outcome{
			FinalState:     o.Outcome.FinalState,
			StatesVisited:  o.Outcome.StateVisits,
			Linearizations: int32Clamp(o.Linearizations),
		}
		if rt := o.Outcome.Context(); rt != nil {
			out.Diagnostics = s.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(rt.Notes(), cached))
			if o.Outcome.Err == nil && len(o.Outcome.Outputs) > 0 {
				out.Outputs = make(map[string]*pb.Value, len(o.Outcome.Outputs))
				for name, val := range o.Outcome.Outputs {
					out.Outputs[name] = s.valueToProto(rt, val, cached.Index)
				}
			}
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
