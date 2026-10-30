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
	// Performer is the attributes the object the action ran on holds when the
	// run ends, keyed as an explored Outcome's Outputs key them ("this.level");
	// empty without PerformedBy.
	Performer map[string]Value
	// FinalTime is the run's simulation clock when it ended, in seconds from
	// the 0 it started at; 0 from a service without CapabilityFinalTime.
	FinalTime float64
	// Trace is the run's records (token steps, waits, accepts, decisions,
	// assignments, completion) when requested with WithTrace.
	Trace []DocumentEvent
	// TraceDropped is the number of oldest trace records the service discarded.
	TraceDropped int
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
	// Trace is the documented execution record sequence, when requested.
	Trace []DocumentEvent
	// TraceDropped is the number of oldest trace records the service discarded.
	TraceDropped int
	// Diagnostics the execution reported.
	Diagnostics []Diagnostic
}

// ExecuteOption configures ExecuteAction and ExecuteState.
type ExecuteOption func(*executeOptions)

type executeOptions struct {
	schedule  string
	performer string
	trace     bool
	events    []string
	advance   *float64
	seed      *uint64
	draws     string
	clockStep float64
}

// WithEvents posts the named signals to the object an action is performed on
// (PerformedBy), in order, each once every token of the run waits, as the REPL's
// %send does; the accept waiting for it takes it on the next step.
func WithEvents(events ...string) ExecuteOption {
	return func(o *executeOptions) { o.events = append(o.events, events...) }
}

// WithAdvance runs the behavior for that many simulated seconds, as sysml
// -advance does, instead of to completion; a run still waiting then is a
// *FailureError that still reports FinalTime.
func WithAdvance(seconds float64) ExecuteOption {
	return func(o *executeOptions) { o.advance = &seconds }
}

// WithSeed seeds the model's own draws, as sysml -seed does, so RandomFunctions
// answer the same values each run.
func WithSeed(seed uint64) ExecuteOption {
	return func(o *executeOptions) { o.seed = &seed }
}

// WithDraws chooses how the model's draws resolve, as sysml -draws spells it:
// "random", "min", "max" or "average".
func WithDraws(policy string) ExecuteOption {
	return func(o *executeOptions) { o.draws = policy }
}

// WithClockStep ticks the run's clock in steps of that many seconds, as sysml
// -clock-step does; 0 is a continuous clock.
func WithClockStep(seconds float64) ExecuteOption {
	return func(o *executeOptions) { o.clockStep = seconds }
}

// WithSchedule names the policy a run resolves its choice points under, as sysml
// -schedule spells it; "explore[...]" belongs to ExploreAction and ExploreState.
func WithSchedule(policy string) ExecuteOption {
	return func(o *executeOptions) { o.schedule = policy }
}

// PerformedBy names the object performing the behavior, as sysml -action "<action> <object>"
// does: a declaration the run creates an object of, or a path into one ("Mission::mission.vehicle").
func PerformedBy(path string) ExecuteOption {
	return func(o *executeOptions) { o.performer = path }
}

// WithTrace asks ExecuteAction or ExecuteState to return the run's execution
// trace; the service must have CapabilityActionTrace or CapabilityStateTrace.
func WithTrace() ExecuteOption {
	return func(o *executeOptions) { o.trace = true }
}

func (c *client) requireTrace(ctx context.Context, trace bool, capability string) error {
	if !trace {
		return nil
	}
	info, err := c.serverInfo(ctx)
	if err != nil {
		return err
	}
	if !info.Has(capability) {
		return &StatusError{
			Code:    CodeUnimplemented,
			Message: fmt.Sprintf("capability %q is unavailable", capability),
		}
	}
	return nil
}

// requirePerformer refuses to send a performer to a service without the
// performer capability, which would run outside any object rather than refuse it.
func (c *client) requirePerformer(ctx context.Context, performer string) error {
	if performer == "" {
		return nil
	}
	info, err := c.serverInfo(ctx)
	if err != nil {
		return err
	}
	if !info.Has(CapabilityPerformer) {
		return &StatusError{
			Code:    CodeUnimplemented,
			Message: fmt.Sprintf("capability %q is unavailable", CapabilityPerformer),
		}
	}
	return nil
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
	if len(options.events) > 0 && options.performer == "" {
		return nil, &StatusError{Code: CodeInvalidArgument, Message: "WithEvents posts to the object the action is performed on: name one with PerformedBy"}
	}
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireSchedule(ctx, options.schedule); err != nil {
		return nil, err
	}
	if err := c.requirePerformer(ctx, options.performer); err != nil {
		return nil, err
	}
	if err := c.requireTrace(ctx, options.trace, CapabilityActionTrace); err != nil {
		return nil, err
	}
	req := &pb.ExecuteActionRequest{
		ModelHash:         hash,
		ActionSymbolId:    actionSymbolID,
		Schedule:          options.schedule,
		PerformerSymbolId: options.performer,
		Advance:           options.advance,
		Seed:              options.seed,
		Draws:             options.draws,
		ClockStep:         options.clockStep,
		Trace:             options.trace,
		Events:            append([]string(nil), options.events...),
	}
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
		if err := c.fitRationals(ctx, slices.Collect(maps.Values(req.Inputs))...); err != nil {
			return nil, err
		}
	}
	resp, err := c.caller.executeAction(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &FailureError{
			Op:           "ExecuteAction",
			Message:      resp.Error,
			Diagnostics:  diagnostics,
			Trace:        documentEventsFromProto(resp.Trace),
			TraceDropped: int(resp.TraceDropped),
			FinalTime:    resp.FinalTime,
		}
	}
	return &ActionRun{
		Outputs:      valuesFromProto(resp.Outputs),
		Performer:    valuesFromProto(resp.PerformerAttributes),
		FinalTime:    resp.FinalTime,
		Trace:        documentEventsFromProto(resp.Trace),
		TraceDropped: int(resp.TraceDropped),
		Diagnostics:  diagnostics,
	}, nil
}

func refuseTrace(trace bool, message string) error {
	if !trace {
		return nil
	}
	return &StatusError{Code: CodeInvalidArgument, Message: message}
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
	if err := c.requirePerformer(ctx, options.performer); err != nil {
		return nil, err
	}
	if err := c.requireTrace(ctx, options.trace, CapabilityStateTrace); err != nil {
		return nil, err
	}
	if len(options.events) > 0 {
		return nil, &StatusError{Code: CodeInvalidArgument, Message: "ExecuteState takes its events as an argument, not WithEvents"}
	}
	resp, err := c.caller.executeState(ctx, &pb.ExecuteStateRequest{
		ModelHash:            hash,
		StateMachineSymbolId: stateMachineSymbolID,
		Events:               append([]string(nil), events...),
		Schedule:             options.schedule,
		PerformerSymbolId:    options.performer,
		Trace:                options.trace,
		Advance:              options.advance,
		Seed:                 options.seed,
		Draws:                options.draws,
		ClockStep:            options.clockStep,
	})
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &FailureError{
			Op:           "ExecuteState",
			Message:      resp.Error,
			Diagnostics:  diagnostics,
			Trace:        documentEventsFromProto(resp.Trace),
			TraceDropped: int(resp.TraceDropped),
			FinalTime:    resp.FinalTime,
		}
	}
	return &StateRun{
		Visited:      append([]string(nil), resp.StatesVisited...),
		Context:      valuesFromProto(resp.FinalContext),
		FinalTime:    resp.FinalTime,
		Trace:        documentEventsFromProto(resp.Trace),
		TraceDropped: int(resp.TraceDropped),
		Diagnostics:  diagnostics,
	}, nil
}
