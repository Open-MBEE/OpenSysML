package opensysml

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Outcome is one distinct outcome an exploration reached, with how many
// linearizations reached it and the choices of one run that did.
type Outcome struct {
	// Outputs are an action's outputs, a state machine's final context, or a
	// case's outputs and verdicts ("objective <name>", "assertion <name>", "verdict <case>").
	Outputs map[string]Value
	// FinalState is the state a machine rests in and Visited the states it
	// entered, in order; both empty for an action or a case.
	FinalState string
	Visited    []string
	// Error is what the runs reaching this outcome failed with; empty when they completed.
	Error string
	// Linearizations is how many runs within the budget reached this outcome.
	Linearizations int
	// Witness is one run's choices in run order, one per choice point it
	// resolved, each spelling the alternatives and the one taken.
	Witness []string
	// Diagnostics are what the witness run reported about itself.
	Diagnostics []Diagnostic
}

// Failed reports an outcome of runs that failed rather than completed.
func (o *Outcome) Failed() bool { return o != nil && o.Error != "" }

// Exploration is what exploring a behavior found: every distinct outcome, in
// the service's canonical order, and how the search ended.
type Exploration struct {
	Outcomes []Outcome
	// Complete is true when every linearization within the budget was run, so
	// Outcomes is the whole set.
	Complete bool
	// Runs is how many runs the exploration made.
	Runs int
	// BudgetsHit names the budgets that ended the search, "runs" before
	// "depth"; empty when Complete.
	BudgetsHit []string
	// RunsBudget and DepthBudget are the budget the search ran under: runs it
	// may make, and choice points one run may resolve.
	RunsBudget  int
	DepthBudget int
}

// Status renders how the exploration ended as the sysml command does:
// "complete (N runs)", or the budget hit after how many runs.
func (e *Exploration) Status() string {
	if e.Complete {
		return fmt.Sprintf("complete (%d runs)", e.Runs)
	}
	named := make([]string, len(e.BudgetsHit))
	for i, budget := range e.BudgetsHit {
		limit := e.RunsBudget
		if budget == "depth" {
			limit = e.DepthBudget
		}
		named[i] = fmt.Sprintf("%s budget %d", budget, limit)
	}
	return fmt.Sprintf("incomplete: %s hit after %d runs", strings.Join(named, " and "), e.Runs)
}

// explores reports a policy spelled as the explore schedule, with or without
// its budget.
func explores(policy string) bool {
	return policy == "explore" || strings.HasPrefix(policy, "explore:")
}

// refuseExploring keeps an exploring policy off a single-run call, whose
// answer would be an empty run rather than the outcomes.
func refuseExploring(op, exploreOp, policy string) error {
	if !explores(policy) {
		return nil
	}
	return &StatusError{
		Code:    CodeInvalidArgument,
		Message: fmt.Sprintf("%s runs once, so schedule %q is answered by %s", op, policy, exploreOp),
	}
}

// explorePolicy is the policy an Explore call sends: the one given, or
// "explore" when none was.
func explorePolicy(op, policy string) (string, error) {
	if policy == "" {
		return "explore", nil
	}
	if !explores(policy) {
		return "", &StatusError{
			Code:    CodeInvalidArgument,
			Message: fmt.Sprintf("%s explores, so schedule %q must be spelled explore or explore:runs=<n>,depth=<d>", op, policy),
		}
	}
	return policy, nil
}

// requireExplore refuses to send an exploring policy to a service without the
// schedule_explore capability.
func (c *client) requireExplore(ctx context.Context) error {
	info, err := c.serverInfo(ctx)
	if err != nil {
		return err
	}
	for _, capability := range []string{CapabilitySchedule, CapabilityScheduleExplore} {
		if !info.Has(capability) {
			return &StatusError{
				Code:    CodeUnimplemented,
				Message: fmt.Sprintf("capability %q is unavailable", capability),
			}
		}
	}
	return nil
}

func (c *client) ExploreAction(
	ctx context.Context,
	model *Model,
	actionSymbolID string,
	inputs map[string]Value,
	opts ...ExecuteOption,
) (*Exploration, error) {
	var options executeOptions
	for _, opt := range opts {
		opt(&options)
	}
	policy, err := explorePolicy("ExploreAction", options.schedule)
	if err != nil {
		return nil, err
	}
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireExplore(ctx); err != nil {
		return nil, err
	}
	req := &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: actionSymbolID, Schedule: policy}
	if len(inputs) > 0 {
		if err := c.requireValueCapabilities(ctx, slices.Collect(maps.Values(inputs))...); err != nil {
			return nil, err
		}
		req.Inputs = make(map[string]*pb.Value, len(inputs))
		for name, value := range inputs {
			sent, err := valueToProto(value)
			if err != nil {
				return nil, err
			}
			req.Inputs[name] = sent
		}
	}
	resp, err := c.caller.executeAction(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "ExploreAction", Message: resp.Error, Diagnostics: diagnosticsFromProto(resp.Diagnostics)}
	}
	return explorationFromProto(resp.Outcomes, resp.Exploration), nil
}

func (c *client) ExploreState(
	ctx context.Context,
	model *Model,
	stateMachineSymbolID string,
	events []string,
	opts ...ExecuteOption,
) (*Exploration, error) {
	var options executeOptions
	for _, opt := range opts {
		opt(&options)
	}
	policy, err := explorePolicy("ExploreState", options.schedule)
	if err != nil {
		return nil, err
	}
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireExplore(ctx); err != nil {
		return nil, err
	}
	resp, err := c.caller.executeState(ctx, &pb.ExecuteStateRequest{
		ModelHash:            hash,
		StateMachineSymbolId: stateMachineSymbolID,
		Events:               append([]string(nil), events...),
		Schedule:             policy,
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &FailureError{Op: "ExploreState", Message: resp.Error, Diagnostics: diagnosticsFromProto(resp.Diagnostics)}
	}
	return explorationFromProto(resp.Outcomes, resp.Exploration), nil
}

func (c *client) ExploreAnalysis(
	ctx context.Context,
	model *Model,
	symbolID string,
	opts ...AnalysisOption,
) (*Exploration, error) {
	var options analysisOptions
	for _, opt := range opts {
		opt(&options)
	}
	policy, err := explorePolicy("ExploreAnalysis", options.schedule)
	if err != nil {
		return nil, err
	}
	options.schedule = policy
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	resp, err := c.analysisRequest(ctx, hash, symbolID, &options, func() error {
		return c.requireExplore(ctx)
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, &VerifyError{
			FailureError: FailureError{Op: "ExploreAnalysis", Message: resp.Error, Diagnostics: diagnosticsFromProto(resp.Diagnostics)},
			Reason:       Reason(resp.FailureReason),
		}
	}
	return explorationFromProto(resp.Outcomes, resp.Exploration), nil
}

func explorationFromProto(outcomes []*pb.Outcome, status *pb.ExplorationStatus) *Exploration {
	out := &Exploration{Outcomes: make([]Outcome, 0, len(outcomes))}
	for _, outcome := range outcomes {
		out.Outcomes = append(out.Outcomes, Outcome{
			Outputs:        valuesFromProto(outcome.Outputs),
			FinalState:     outcome.FinalState,
			Visited:        append([]string(nil), outcome.StatesVisited...),
			Error:          outcome.Error,
			Linearizations: int(outcome.Linearizations),
			Witness:        append([]string(nil), outcome.Witness...),
			Diagnostics:    diagnosticsFromProto(outcome.Diagnostics),
		})
	}
	if status != nil {
		out.Complete = status.Complete
		out.Runs = int(status.Runs)
		out.BudgetsHit = append([]string(nil), status.BudgetsHit...)
		out.RunsBudget = int(status.RunsBudget)
		out.DepthBudget = int(status.DepthBudget)
	}
	return out
}
