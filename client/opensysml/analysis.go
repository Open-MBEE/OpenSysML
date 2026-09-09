package opensysml

import (
	"context"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Analysis is what one run of an analysis case produced.
type Analysis struct {
	// Outputs are the case's out and return parameters in declaration order; a
	// value the body returned into an unnamed result is named "result".
	Outputs []CalcOutput
	// Verdicts are the case's objectives, in order, then the assertions in its
	// body. Kind is "objective" or "assertion".
	Verdicts []Verdict
	// Verifications are the verdicts the bodies of the case and of the
	// verification cases it performs produced, empty for an analysis case.
	Verifications []VerificationVerdict
	// Instances are the objects reachable from the subject the run was given,
	// including it; empty when the case bound its own subject.
	Instances []*Instance
	// Diagnostics the run reported.
	Diagnostics []Diagnostic
}

// Holds reports whether every objective and assertion was decided and held. A
// case stating none holds trivially.
func (a *Analysis) Holds() bool {
	for i := range a.Verdicts {
		if a.Verdicts[i].Undecided() || !a.Verdicts[i].Holds {
			return false
		}
	}
	return true
}

// Output is the value of the named output, and whether the case reported one.
func (a *Analysis) Output(name string) (Value, bool) {
	for _, out := range a.Outputs {
		if out.Name == name {
			return out.Value, true
		}
	}
	return nil, false
}

// AnalysisOption configures RunAnalysis.
type AnalysisOption func(*analysisOptions)

type analysisOptions struct {
	subjectSymbolID string
	positional      []Value
	named           []namedArgument
	schedule        string
}

type namedArgument struct {
	name  string
	value Value
}

// Subject names a part or usage to instantiate as the case's subject, for a
// case whose usage binds none or to run one against another object. It is
// what Against is to VerifyRequirement.
func Subject(symbolID string) AnalysisOption {
	return func(o *analysisOptions) { o.subjectSymbolID = symbolID }
}

// Arguments bind the case's non-subject inputs in declaration order, after
// any given earlier.
func Arguments(values ...Value) AnalysisOption {
	return func(o *analysisOptions) { o.positional = append(o.positional, values...) }
}

// Argument binds the input named, which may be a defaulted one or the subject.
func Argument(name string, value Value) AnalysisOption {
	return func(o *analysisOptions) { o.named = append(o.named, namedArgument{name, value}) }
}

// Schedule names the scheduling policy the actions the case performs resolve
// their choice points under, as WithSchedule does for ExecuteAction. Requires
// the schedule capability. An exploring spelling belongs to ExploreAnalysis,
// which requires schedule_explore too.
func Schedule(policy string) AnalysisOption {
	return func(o *analysisOptions) { o.schedule = policy }
}

func (c *client) RunAnalysis(
	ctx context.Context,
	model *Model,
	symbolID string,
	opts ...AnalysisOption,
) (*Analysis, error) {
	var options analysisOptions
	for _, opt := range opts {
		opt(&options)
	}
	if err := refuseExploring("RunAnalysis", "ExploreAnalysis", options.schedule); err != nil {
		return nil, err
	}
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	resp, err := c.analysisRequest(ctx, hash, symbolID, &options, func() error {
		return c.requireSchedule(ctx, options.schedule)
	})
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &VerifyError{
			FailureError: FailureError{Op: "RunAnalysis", Message: resp.Error, Diagnostics: diagnostics},
			Reason:       Reason(resp.FailureReason),
		}
	}
	out := &Analysis{
		Instances:     instancesFromProto(resp.Instances),
		Diagnostics:   diagnostics,
		Verifications: verificationVerdictsFromProto(resp.VerificationVerdicts),
	}
	for _, output := range resp.Outputs {
		out.Outputs = append(out.Outputs, CalcOutput{Name: output.Name, Value: valueFromProto(output.Value)})
	}
	for _, verdict := range resp.Verdicts {
		if converted := verdictFromProto(verdict); converted != nil {
			out.Verdicts = append(out.Verdicts, *converted)
		}
	}
	return out, nil
}

// analysisRequest sends the run once its arguments and, by requireSchedule,
// its policy are checked against the capabilities they need.
func (c *client) analysisRequest(ctx context.Context, hash, symbolID string, options *analysisOptions, requireSchedule func() error) (*pb.RunAnalysisResponse, error) {
	values := append([]Value(nil), options.positional...)
	for _, arg := range options.named {
		values = append(values, arg.value)
	}
	if err := c.requireValueCapabilities(ctx, values...); err != nil {
		return nil, err
	}
	if err := requireSchedule(); err != nil {
		return nil, err
	}
	req := &pb.RunAnalysisRequest{
		ModelHash:       hash,
		SymbolId:        symbolID,
		SubjectSymbolId: options.subjectSymbolID,
		Schedule:        options.schedule,
	}
	for _, argument := range options.positional {
		sent, err := valueToProto(argument)
		if err != nil {
			return nil, err
		}
		req.Arguments = append(req.Arguments, sent)
	}
	if len(options.named) > 0 {
		req.NamedArguments = make(map[string]*pb.Value, len(options.named))
		for _, argument := range options.named {
			sent, err := valueToProto(argument.value)
			if err != nil {
				return nil, err
			}
			req.NamedArguments[argument.name] = sent
		}
	}
	return c.caller.runAnalysis(ctx, req)
}
