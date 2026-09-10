package grpc

import (
	"context"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// CapabilityEngines names the ListEngines RPC, the `engine` field of the
// verification and sweep requests, and the `engine`, `strength` and `bounds`
// fields of their responses and of every Verdict.
const CapabilityEngines = "engines"

// engineSelection reads a request's engine field. Empty is auto; anything else
// needs the engines capability and must name an engine, "auto" or "all".
func (s *Service) engineSelection(spelling string) (analysis.Selection, error) {
	if spelling == "" {
		return analysis.Auto(), nil
	}
	if err := s.requireCapability(CapabilityEngines); err != nil {
		return analysis.Selection{}, err
	}
	selection, err := s.engines.Select(spelling)
	if err != nil {
		return analysis.Selection{}, statusError(connect.CodeInvalidArgument, err.Error())
	}
	// "explore" asks what the explore schedule asks, so it needs that capability too.
	if _, explores := analysis.Explores(selection, runtime.DefaultSchedulePolicy); explores {
		if err := s.requireCapability(CapabilityScheduleExplore); err != nil {
			return analysis.Selection{}, err
		}
	}
	return selection, nil
}

// ListEngines lists the engines this build registers, in name order.
func (s *Service) ListEngines(_ context.Context, _ *pb.ListEnginesRequest) (*pb.ListEnginesResponse, error) {
	if err := s.requireCapability(CapabilityEngines); err != nil {
		return nil, err
	}
	listings := s.engines.Listings()
	resp := &pb.ListEnginesResponse{Engines: make([]*pb.EngineInfo, 0, len(listings))}
	for _, l := range listings {
		info := &pb.EngineInfo{
			Name:         l.Engine,
			Authority:    l.Authority.String(),
			Answers:      make([]string, 0, len(l.Questions)),
			Bounds:       l.Description.Bounds,
			Process:      l.Description.Process,
			ProcessFound: l.Status.Process,
			Ready:        l.Ready(),
		}
		for _, kind := range l.Questions {
			info.Answers = append(info.Answers, kind.String())
		}
		if l.Status.Err != nil {
			info.Unavailable = l.Status.Err.Error()
		}
		resp.Engines = append(resp.Engines, info)
	}
	return resp, nil
}

// perform puts one execution on the request's runtime to the service's engines
// under the request's selection, under the schedule that runtime was set.
func perform[T any](ctx context.Context, v *verifyContext, subject string, call func(*runtime.Context) (T, error), answer func(T, error) analysis.Answer) (T, analysis.Plan, error) {
	return performOn(ctx, v.service, v.runtime, v.engine, subject, call, answer)
}

// check puts one constraint, requirement or satisfaction check to the engines.
func (v *verifyContext) check(ctx context.Context, subject string, call func(*runtime.Context) (runtime.CheckResult, error)) (runtime.CheckResult, analysis.Plan, error) {
	return perform(ctx, v, subject, call, analysis.CheckAnswer)
}

// performOn puts one execution on rt to the service's engines under selection,
// under the schedule rt was set.
func performOn[T any](ctx context.Context, s *Service, rt *runtime.Context, selection analysis.Selection, subject string, call func(*runtime.Context) (T, error), answer func(T, error) analysis.Answer) (T, analysis.Plan, error) {
	schedule := rt.Schedule()
	return analysis.Perform(ctx, s.engines, analysis.Held(rt, nil), subject, schedule, analysis.BudgetOf(s.budgets, schedule, analysis.Evaluate), selection, call, answer)
}

// standing spells for the wire which engine a plan's answer is, how strong its
// evidence is and the bounds it ran under; a plan no engine answered is unnamed.
type standing struct {
	engine   string
	strength string
	bounds   []*pb.Bound
}

// standingOf is what a plan established, empty when the service withholds the
// engines capability or no engine was asked.
func (s *Service) standingOf(plan analysis.Plan) standing {
	if !s.capabilities.has(CapabilityEngines) || len(plan.Steps) == 0 {
		return standing{}
	}
	result := plan.Result
	out := standing{engine: result.Engine, strength: result.Strength.String()}
	for _, b := range result.Bounds {
		out.bounds = append(out.bounds, &pb.Bound{Name: b.Name, Limit: b.Limit, Reached: b.Reached})
	}
	return out
}

// stamp writes a plan's standing on a verdict.
func (st standing) stamp(verdict *pb.Verdict) *pb.Verdict {
	verdict.Engine = st.engine
	verdict.Strength = st.strength
	verdict.Bounds = st.bounds
	return verdict
}

// callerGone is the caller's own error when a run failed because the caller went
// away, which fails the call rather than being reported as the run's failure.
func callerGone(ctx context.Context, err error) error {
	if err != nil {
		return ctx.Err()
	}
	return nil
}

// heldAnswer is what a behavior run established: the values it left, or nothing.
func heldAnswer(held map[string]runtime.Value, err error) analysis.Answer {
	return analysis.ValuesAnswer(analysis.ValuesOf(held), err)
}

// stateRun is what one state machine run left: its final data and the states it visited.
type stateRun struct {
	final   map[string]runtime.Value
	visited []string
}
