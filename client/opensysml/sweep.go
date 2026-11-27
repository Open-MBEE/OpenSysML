package opensysml

import (
	"context"
	"fmt"
	"maps"
	"slices"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// SweepRange is one parameter stepped (or drawn) from Start to End.
type SweepRange struct {
	// Parameter names the input parameter of the case or calc the range binds.
	Parameter string
	// Start and End are the range's endpoints, both included. Step is nil for
	// a sampled sweep, and may be nil between whole numbers, which step by one.
	Start, End, Step Value
}

// Sweep is what a sweep answers: the table and the parameters it covered.
type Sweep struct {
	// Rows is the table, one per run in the order the runs were made:
	// lexicographically over the ranges as given, or in draw order when Sampled.
	Rows []RunRow
	// Parameters are the swept parameters in the order their ranges were given,
	// which is the order of each row's Inputs.
	Parameters []string
	// Sampled is whether the rows were drawn rather than stepped through, and
	// Seed the seed they were drawn from.
	Sampled bool
	Seed    uint64
	// Instances are the objects the rows' verdicts and values refer to, by id.
	Instances []*Instance
	// Standing is the engine that ran the table, the strength of its evidence
	// and the bounds it ran under: Engine, Strength and Bounds.
	Standing
	Diagnostics []Diagnostic
}

// Instance resolves an InstanceID a row's output, verdict or evaluation
// referred to, nil when the sweep reported no such object.
func (s *Sweep) Instance(id InstanceID) *Instance {
	if s == nil {
		return nil
	}
	for _, instance := range s.Instances {
		if instance != nil && instance.ID == int64(id) {
			return instance
		}
	}
	return nil
}

// SweepOption configures Sweep.
type SweepOption func(*sweepOptions)

type sweepOptions struct {
	subject   string
	arguments []Value
	named     []namedArgument
	samples   int64
	seed      uint64
	engine    string
}

// SweepSubject names the part/usage to instantiate as an analysis case's subject.
func SweepSubject(symbolID string) SweepOption {
	return func(o *sweepOptions) { o.subject = symbolID }
}

// SweepArguments binds input parameters positionally, as every row binds them.
func SweepArguments(values ...Value) SweepOption {
	return func(o *sweepOptions) { o.arguments = append(o.arguments, values...) }
}

// SweepArgument binds one input parameter by name, as every row binds it.
func SweepArgument(name string, value Value) SweepOption {
	return func(o *sweepOptions) { o.named = append(o.named, namedArgument{name: name, value: value}) }
}

// SweepSamples draws rows uniformly from each range instead of stepping
// through it; it needs SweepSeed and ranges stating no step.
func SweepSamples(rows int64) SweepOption {
	return func(o *sweepOptions) { o.samples = rows }
}

// SweepSeed seeds the draws of a sampled sweep: one seed draws one table.
func SweepSeed(seed uint64) SweepOption {
	return func(o *sweepOptions) { o.seed = seed }
}

// SweepEngine names the engine the sweep is put to, as WithEngine does for a
// verification. Requires the engines capability.
func SweepEngine(name string) SweepOption {
	return func(o *sweepOptions) { o.engine = name }
}

// Sweep runs the analysis case or calc over the ranges and answers one row per
// run, in the service's order. Requires the verification capability.
func (c *client) Sweep(ctx context.Context, model *Model, symbolID string, ranges []SweepRange, opts ...SweepOption) (*Sweep, error) {
	var options sweepOptions
	for _, opt := range opts {
		opt(&options)
	}
	hash, done, err := c.call(model)
	if err != nil {
		return nil, err
	}
	defer done()
	if err := c.requireCapabilities(ctx, CapabilityVerification); err != nil {
		return nil, err
	}
	values := slices.Clone(options.arguments)
	for _, arg := range options.named {
		values = append(values, arg.value)
	}
	for _, r := range ranges {
		values = append(values, r.Start, r.End, r.Step)
	}
	if err := c.requireValueCapabilities(ctx, values...); err != nil {
		return nil, err
	}
	if err := c.requireEngine(ctx, options.engine); err != nil {
		return nil, err
	}
	arguments, named, err := argumentsToProto(options.arguments, options.named)
	if err != nil {
		return nil, err
	}
	req := &pb.RunSweepRequest{
		ModelHash:       hash,
		SymbolId:        symbolID,
		SubjectSymbolId: options.subject,
		Arguments:       arguments,
		NamedArguments:  named,
		Samples:         options.samples,
		Seed:            options.seed,
		Engine:          engineField(options.engine),
	}
	sent := append(slices.Clone(arguments), slices.Collect(maps.Values(named))...)
	for _, r := range ranges {
		converted, err := sweepRangeToProto(r)
		if err != nil {
			return nil, fmt.Errorf("Sweep %s: %w", r.Parameter, err)
		}
		req.Ranges = append(req.Ranges, converted)
		sent = append(sent, converted.Start, converted.End, converted.Step)
	}
	if err := c.fitRationals(ctx, slices.DeleteFunc(sent, func(v *pb.Value) bool { return v == nil })...); err != nil {
		return nil, err
	}
	resp, err := c.caller.runSweep(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &VerifyError{
			FailureError: FailureError{Op: "Sweep", Message: resp.Error, Diagnostics: diagnostics},
			Reason:       Reason(resp.FailureReason),
		}
	}
	return &Sweep{
		Rows:        runRowsFromProto(resp.Rows),
		Parameters:  resp.Parameters,
		Sampled:     resp.Sampled,
		Seed:        resp.Seed,
		Instances:   instancesFromProto(resp.Instances),
		Standing:    standingFromProto(resp.Engine, resp.Strength, resp.Bounds),
		Diagnostics: diagnostics,
	}, nil
}

// sweepRangeToProto converts a range, leaving an endpoint or step not given unset.
func sweepRangeToProto(r SweepRange) (*pb.SweepRange, error) {
	out := &pb.SweepRange{Parameter: r.Parameter}
	for _, bound := range []struct {
		into **pb.Value
		from Value
	}{{&out.Start, r.Start}, {&out.End, r.End}, {&out.Step, r.Step}} {
		if bound.from == nil {
			continue
		}
		converted, err := valueToProto(bound.from)
		if err != nil {
			return nil, err
		}
		*bound.into = converted
	}
	return out, nil
}
