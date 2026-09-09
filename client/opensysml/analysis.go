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
	// Evaluations are the applications the run made of the case's own calcs, in
	// the order made: for a trade study, its evaluationFunction applied to each
	// alternative in subject order. Empty for a service not advertising
	// CapabilityCaseEvaluations.
	Evaluations []Evaluation
	// Instances are the objects the run reported: those reachable from the
	// subject, including it, and those the outputs and evaluations refer to.
	Instances []*Instance
	// Diagnostics the run reported.
	Diagnostics []Diagnostic
}

// Evaluation is one application of a calc the case declares, made while it
// ran. For a trade study it is the evaluationFunction scoring one alternative.
type Evaluation struct {
	// FunctionID is the FQN of the calc applied.
	FunctionID string
	// Arguments it was applied to, in parameter order; an alternative is an
	// InstanceID that Analysis.Instance resolves.
	Arguments []Value
	// Result is what it computed, nil when Error says why it computed nothing.
	Result Value
	// Error is set when the application failed rather than computed a value.
	Error string
	// Selected marks the evaluation whose argument selectOne picked and the case
	// returned: the alternative a trade study selected.
	Selected bool
	// Tied marks an evaluation computing what the selected one did without
	// being it: an alternative scoring the same as the one selected.
	Tied bool
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

// Selected are the evaluations of the alternatives the case selected, in the
// order made; none for a case that selected nothing.
func (a *Analysis) Selected() []Evaluation {
	var out []Evaluation
	for _, evaluation := range a.Evaluations {
		if evaluation.Selected {
			out = append(out, evaluation)
		}
	}
	return out
}

// Instance resolves an InstanceID an output, a verdict or an evaluation
// referred to, nil when the run reported no such object.
func (a *Analysis) Instance(id InstanceID) *Instance {
	if a == nil {
		return nil
	}
	for _, instance := range a.Instances {
		if instance != nil && instance.ID == int64(id) {
			return instance
		}
	}
	return nil
}

// AnalysisError is a run of an analysis case that failed leaving something to
// inspect: the evaluations a trade study made before an alternative failed, the
// outputs computed before one failed, and each objective and required assertion
// undecided with the failure as its reason — as an unbound subject leaves an
// objective. It is a VerifyError, so errors.As recovers one and
// errors.Is(err, ErrFailure) matches. A request refused before the run, or a
// failure that left nothing to report, is a *VerifyError alone: its Reason says
// why, ReasonWrongKind for a symbol that is no case.
type AnalysisError struct {
	VerifyError
	// Partial is what the run left: the outputs and evaluations made, the
	// objects they name, each verdict undecided.
	Partial *Analysis
}

// Unwrap exposes the classified failure, so errors.As recovers a *VerifyError
// and, through it, a *FailureError.
func (e *AnalysisError) Unwrap() error { return &e.VerifyError }

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
// the schedule capability.
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
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	values := append([]Value(nil), options.positional...)
	for _, arg := range options.named {
		values = append(values, arg.value)
	}
	if err := c.requireValueCapabilities(ctx, values...); err != nil {
		return nil, err
	}
	if err := c.requireSchedule(ctx, options.schedule); err != nil {
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
	resp, err := c.caller.runAnalysis(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		failure := VerifyError{
			FailureError: FailureError{Op: "RunAnalysis", Message: resp.Error, Diagnostics: diagnostics},
			Reason:       Reason(resp.FailureReason),
		}
		// A request refused before the run, or a failure that left nothing to report, has no partial answer.
		if len(resp.Outputs) == 0 && len(resp.Verdicts) == 0 && len(resp.Evaluations) == 0 && len(resp.Instances) == 0 {
			return nil, &failure
		}
		return nil, &AnalysisError{VerifyError: failure, Partial: analysisFromProto(resp, diagnostics)}
	}
	return analysisFromProto(resp, diagnostics), nil
}

func analysisFromProto(resp *pb.RunAnalysisResponse, diagnostics []Diagnostic) *Analysis {
	out := &Analysis{
		Instances:     instancesFromProto(resp.Instances),
		Diagnostics:   diagnostics,
		Verifications: verificationVerdictsFromProto(resp.VerificationVerdicts),
		Evaluations:   evaluationsFromProto(resp.Evaluations),
	}
	for _, output := range resp.Outputs {
		out.Outputs = append(out.Outputs, CalcOutput{Name: output.Name, Value: valueFromProto(output.Value)})
	}
	for _, verdict := range resp.Verdicts {
		if converted := verdictFromProto(verdict); converted != nil {
			out.Verdicts = append(out.Verdicts, *converted)
		}
	}
	return out
}

func evaluationsFromProto(evaluations []*pb.CaseEvaluation) []Evaluation {
	if len(evaluations) == 0 {
		return nil
	}
	out := make([]Evaluation, 0, len(evaluations))
	for _, evaluation := range evaluations {
		if evaluation == nil {
			continue
		}
		converted := Evaluation{
			FunctionID: evaluation.FunctionId,
			Arguments:  make([]Value, 0, len(evaluation.Arguments)),
			Error:      evaluation.Error,
			Selected:   evaluation.Selected,
			Tied:       evaluation.Tied,
		}
		for _, argument := range evaluation.Arguments {
			converted.Arguments = append(converted.Arguments, valueFromProto(argument))
		}
		if evaluation.Error == "" && evaluation.Result != nil {
			converted.Result = valueFromProto(evaluation.Result)
		}
		out = append(out, converted)
	}
	return out
}
