package opensysml

import (
	"context"
	"maps"
	"strings"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Check is what CheckBehavior answers: the verdict over every schedule of a
// behavior with each violation, divergence and final outcome witnessed.
type Check struct {
	// Verdict is "holds", "fails" or "undecided".
	Verdict string
	// Lines is the report as `sysml -check-property` prints it.
	Lines []string
	// Search is the explicit-state search's statistics, nil for a symbolic answer.
	Search *CheckSearch
	// Violations is each property, deadlock or failure witnessed.
	Violations []CheckViolation
	// Divergences is each feature whose final value the schedule decided.
	Divergences []CheckDivergence
	// Finals is each final outcome of an exhaustive search.
	Finals []CheckFinal
	// Claim is the symbolic engine's claim; FreeInputs the inputs it ranged
	// over and Assumptions what it assumed.
	Claim       string
	FreeInputs  []string
	Assumptions []string
	// Witness and Contrast are the symbolic engine's witness and counter-witness.
	Witness  *CheckWitness
	Contrast *CheckWitness
	// Standing is the engine's name, strength and the bounds it ran under.
	Standing Standing
	// Reason says why the verdict is undecided, when it is.
	Reason string
}

// CheckSearch is the explicit-state search's statistics.
type CheckSearch struct {
	States, Moves, MaxDepth int
	BoundsHit               []string
	Horizon                 float64
	HorizonSet              bool
	MassBounded             bool
	Notes, Scope            []string
	Status                  string
}

// CheckViolation is one witnessed violation: Kind is "property", "deadlock" or
// "failure"; Property names the property for the first, Error the failure.
type CheckViolation struct {
	Kind, Property, Error string
	Depth                 int
	Probability           float64
	Witness               *CheckWitness
	Text                  string
}

// CheckDivergence is a feature whose final value the schedule decides.
type CheckDivergence struct {
	Feature string
	Values  []CheckDivergentValue
}

// CheckDivergentValue is one final value a feature reaches and its witness.
type CheckDivergentValue struct {
	Value   string
	Witness *CheckWitness
}

// CheckFinal is one final outcome of an exhaustive search.
type CheckFinal struct {
	Outcome string
	Values  map[string]string
	Witness *CheckWitness
}

// CheckWitness replays one schedule: Schedule is the policy to pass as a run's
// schedule, Draws/Choices/Inputs what it took, FileName/FileContent its file.
type CheckWitness struct {
	Schedule               string
	Draws, Choices, Inputs []string
	FileName, FileContent  string
}

// CheckOption configures CheckBehavior.
type CheckOption func(*checkOptions)

type checkOptions struct {
	inputs     map[string]Value
	performer  string
	properties []string
	diverge    []string
	freeInputs []string
	assume     []string
	witness    bool
	depth      int32
	states     int32
	unroll     int32
	timeout    float64
	engine     string
}

// CheckInputs binds the behavior's input parameters before the search.
func CheckInputs(inputs map[string]Value) CheckOption {
	return func(o *checkOptions) { o.inputs = inputs }
}

// CheckPerformer names the part or usage whose object performs the behavior.
func CheckPerformer(symbolID string) CheckOption {
	return func(o *checkOptions) { o.performer = symbolID }
}

// CheckProperties names constraints or requirements evaluated at every stable state.
func CheckProperties(names ...string) CheckOption {
	return func(o *checkOptions) { o.properties = append(o.properties, names...) }
}

// CheckDiverge names features whose final value the schedule may decide.
func CheckDiverge(features ...string) CheckOption {
	return func(o *checkOptions) { o.diverge = append(o.diverge, features...) }
}

// CheckFreeInputs leaves inputs free for the symbolic engine to range over.
func CheckFreeInputs(names ...string) CheckOption {
	return func(o *checkOptions) { o.freeInputs = append(o.freeInputs, names...) }
}

// CheckAssume names constraints or requirements the symbolic engine assumes.
func CheckAssume(names ...string) CheckOption {
	return func(o *checkOptions) { o.assume = append(o.assume, names...) }
}

// CheckWitnessFiles asks for each witness's file to be carried in the answer.
func CheckWitnessFiles() CheckOption {
	return func(o *checkOptions) { o.witness = true }
}

// CheckBounds limits the search: moves per schedule, states visited, loop
// unrollings and seconds; zero leaves each at the engine's default.
func CheckBounds(depth, states, unroll int32, timeout float64) CheckOption {
	return func(o *checkOptions) { o.depth, o.states, o.unroll, o.timeout = depth, states, unroll, timeout }
}

// CheckEngine selects the engine asked: "check" (the default), "smt" or one ListEngines names.
func CheckEngine(name string) CheckOption {
	return func(o *checkOptions) { o.engine = name }
}

// CheckAction searches every schedule of an action. Requires the check capability.
func (c *client) CheckAction(ctx context.Context, model *Model, actionID string, opts ...CheckOption) (*Check, error) {
	return c.checkBehavior(ctx, model, actionID, "", opts)
}

// CheckStateMachine searches every schedule of a state machine. Requires the check capability.
func (c *client) CheckStateMachine(ctx context.Context, model *Model, stateMachineID string, opts ...CheckOption) (*Check, error) {
	return c.checkBehavior(ctx, model, "", stateMachineID, opts)
}

func (c *client) checkBehavior(ctx context.Context, model *Model, actionID, stateMachineID string, opts []CheckOption) (*Check, error) {
	var options checkOptions
	for _, opt := range opts {
		opt(&options)
	}
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireCapabilities(ctx, CapabilityCheck); err != nil {
		return nil, err
	}
	if options.performer != "" {
		if err := c.requireCapabilities(ctx, CapabilityPerformer); err != nil {
			return nil, err
		}
	}
	inputs, err := valuesToProto(options.inputs)
	if err != nil {
		return nil, err
	}
	req := &pb.CheckBehaviorRequest{
		ModelHash:            hash,
		ActionSymbolId:       actionID,
		StateMachineSymbolId: stateMachineID,
		Inputs:               inputs,
		PerformerSymbolId:    options.performer,
		CheckProperty:        options.properties,
		CheckDiverge:         options.diverge,
		CheckInput:           options.freeInputs,
		CheckAssume:          options.assume,
		CheckWitness:         options.witness,
		CheckDepth:           options.depth,
		CheckStates:          options.states,
		CheckUnroll:          options.unroll,
		CheckTimeout:         options.timeout,
		Engine:               options.engine,
	}
	resp, err := c.caller.checkBehavior(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		op := "CheckAction"
		if actionID == "" {
			op = "CheckStateMachine"
		}
		return nil, &FailureError{Op: op, Message: resp.Error}
	}
	return checkFromProto(resp), nil
}

func checkFromProto(resp *pb.CheckBehaviorResponse) *Check {
	out := &Check{
		Verdict:     resp.Verdict,
		Lines:       append([]string(nil), resp.Lines...),
		Claim:       resp.Claim,
		FreeInputs:  append([]string(nil), resp.FreeInputs...),
		Assumptions: append([]string(nil), resp.Assumptions...),
		Witness:     checkWitnessFromProto(resp.Witness),
		Contrast:    checkWitnessFromProto(resp.Contrast),
		Standing:    standingFromProto(resp.Engine, resp.Strength, resp.Bounds),
		Reason:      resp.Reason,
	}
	if s := resp.Search; s != nil {
		out.Search = &CheckSearch{
			States: int(s.States), Moves: int(s.Moves), MaxDepth: int(s.MaxDepth),
			BoundsHit: append([]string(nil), s.BoundsHit...), Horizon: s.Horizon, HorizonSet: s.HorizonSet,
			MassBounded: s.MassBounded, Notes: append([]string(nil), s.Notes...), Scope: append([]string(nil), s.Scope...),
			Status: s.Status,
		}
	}
	for _, v := range resp.Violations {
		if v == nil {
			continue
		}
		out.Violations = append(out.Violations, CheckViolation{
			Kind: v.Kind, Property: v.Property, Error: v.Error, Depth: int(v.Depth), Probability: v.Probability,
			Witness: checkWitnessFromProto(v.Witness), Text: v.Text,
		})
	}
	for _, d := range resp.Divergences {
		if d == nil {
			continue
		}
		divergence := CheckDivergence{Feature: d.Feature}
		for _, value := range d.Values {
			if value != nil {
				divergence.Values = append(divergence.Values, CheckDivergentValue{Value: value.Value, Witness: checkWitnessFromProto(value.Witness)})
			}
		}
		out.Divergences = append(out.Divergences, divergence)
	}
	for _, f := range resp.Finals {
		if f == nil {
			continue
		}
		final := CheckFinal{Outcome: f.Outcome, Witness: checkWitnessFromProto(f.Witness)}
		if len(f.Values) > 0 {
			final.Values = maps.Clone(f.Values)
		}
		out.Finals = append(out.Finals, final)
	}
	return out
}

func checkWitnessFromProto(w *pb.CheckWitness) *CheckWitness {
	if w == nil {
		return nil
	}
	return &CheckWitness{
		Schedule: w.Schedule, Draws: append([]string(nil), w.Draws...), Choices: append([]string(nil), w.Choices...),
		Inputs: append([]string(nil), w.Inputs...), FileName: w.FileName, FileContent: w.FileContent,
	}
}

// String renders the check as its report lines.
func (c *Check) String() string {
	if c == nil {
		return ""
	}
	if len(c.Lines) == 0 {
		return c.Verdict
	}
	return strings.Join(c.Lines, "\n")
}
