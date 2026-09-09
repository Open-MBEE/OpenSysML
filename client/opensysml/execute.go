package opensysml

import (
	"context"
	"fmt"
	"maps"
	"slices"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// ActionRun is what one action execution produced.
type ActionRun struct {
	// Outputs are the action's output parameters by name, empty for an action
	// that produces none.
	Outputs map[string]Value
	// FinalTime is the run's simulation clock when it ended, in seconds from
	// the 0 it started at; 0 from a service without CapabilityFinalTime.
	FinalTime float64
	// Diagnostics the execution reported.
	Diagnostics []Diagnostic
}

// StateRun is what one state machine execution produced.
type StateRun struct {
	// Visited is the trace of states entered, in order.
	Visited []string
	// Context is the machine's context when execution stopped, by feature name.
	Context map[string]Value
	// FinalTime is the run's simulation clock when it ended, in seconds from
	// the 0 it started at; 0 from a service without CapabilityFinalTime.
	FinalTime float64
	// Diagnostics the execution reported.
	Diagnostics []Diagnostic
}

// ExecuteOption configures ExecuteAction and ExecuteState.
type ExecuteOption func(*executeOptions)

type executeOptions struct {
	schedule string
}

// WithSchedule names the policy a run resolves its choice points under, as sysml
// -schedule spells it; "explore[...]" belongs to ExploreAction and ExploreState.
func WithSchedule(policy string) ExecuteOption {
	return func(o *executeOptions) { o.schedule = policy }
}

// requireSchedule refuses to send a policy to a service without the schedule
// capability, which would run under the default rather than refuse it.
func (c *client) requireSchedule(ctx context.Context, policy string) error {
	if policy == "" {
		return nil
	}
	info, err := c.serverInfo(ctx)
	if err != nil {
		return err
	}
	if !info.Has(CapabilitySchedule) {
		return &StatusError{
			Code:    CodeUnimplemented,
			Message: fmt.Sprintf("capability %q is unavailable", CapabilitySchedule),
		}
	}
	return nil
}

func (c *client) ExecuteAction(
	ctx context.Context,
	model *Model,
	actionSymbolID string,
	inputs map[string]Value,
	opts ...ExecuteOption,
) (*ActionRun, error) {
	var options executeOptions
	for _, opt := range opts {
		opt(&options)
	}
	if err := refuseExploring("ExecuteAction", "ExploreAction", options.schedule); err != nil {
		return nil, err
	}
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireSchedule(ctx, options.schedule); err != nil {
		return nil, err
	}
	req := &pb.ExecuteActionRequest{ModelHash: hash, ActionSymbolId: actionSymbolID, Schedule: options.schedule}
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
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &FailureError{Op: "ExecuteAction", Message: resp.Error, Diagnostics: diagnostics}
	}
	return &ActionRun{Outputs: valuesFromProto(resp.Outputs), FinalTime: resp.FinalTime, Diagnostics: diagnostics}, nil
}

func (c *client) ExecuteState(
	ctx context.Context,
	model *Model,
	stateMachineSymbolID string,
	events []string,
	opts ...ExecuteOption,
) (*StateRun, error) {
	var options executeOptions
	for _, opt := range opts {
		opt(&options)
	}
	if err := refuseExploring("ExecuteState", "ExploreState", options.schedule); err != nil {
		return nil, err
	}
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireSchedule(ctx, options.schedule); err != nil {
		return nil, err
	}
	resp, err := c.caller.executeState(ctx, &pb.ExecuteStateRequest{
		ModelHash:            hash,
		StateMachineSymbolId: stateMachineSymbolID,
		Events:               append([]string(nil), events...),
		Schedule:             options.schedule,
	})
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &FailureError{Op: "ExecuteState", Message: resp.Error, Diagnostics: diagnostics}
	}
	return &StateRun{
		Visited:     append([]string(nil), resp.StatesVisited...),
		Context:     valuesFromProto(resp.FinalContext),
		FinalTime:   resp.FinalTime,
		Diagnostics: diagnostics,
	}, nil
}
