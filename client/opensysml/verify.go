package opensysml

import (
	"context"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Reason says what kind of failure an undecided verdict or a failed calculation
// reports, so a caller acts on the kind rather than on the message text.
type Reason int32

// The failure kinds a verification or calculation reports.
const (
	// ReasonUnspecified is no failure, or one the service did not classify.
	ReasonUnspecified Reason = Reason(pb.FailureReason_FAILURE_REASON_UNSPECIFIED)
	// ReasonEvaluation is a condition or calculation that could not be evaluated.
	ReasonEvaluation Reason = Reason(pb.FailureReason_FAILURE_REASON_EVALUATION)
	// ReasonWrongKind is a symbol that declares something else than was asked about.
	ReasonWrongKind Reason = Reason(pb.FailureReason_FAILURE_REASON_WRONG_KIND)
	// ReasonAmbiguousSubject is several objects carrying the element: name one.
	ReasonAmbiguousSubject Reason = Reason(pb.FailureReason_FAILURE_REASON_AMBIGUOUS_SUBJECT)
)

// String names the reason as the wire enum spells it.
func (r Reason) String() string {
	return pb.FailureReason(r).String()
}

// Verdict is one verification's answer: whether the condition held, and, when
// it did not, what the model answered false about.
type Verdict struct {
	// Kind is what was verified: "constraint", "requirement" or "satisfy".
	Kind string
	// ElementID is the FQN verified, empty for an anonymous satisfy assertion.
	ElementID string
	// Element is the element as a reader names it, or the assertion as written.
	Element string
	// Holds is the model's answer. False with an empty Error is a verdict of
	// false; false with an Error is no verdict at all — see Undecided.
	Holds bool
	// Condition is the condition that evaluated to false, as written, when the
	// runtime names one.
	Condition string
	// InstanceID is the object the verdict is about, 0 when it is about declared
	// values alone. Verification.Instances carries its feature values.
	InstanceID int64
	// InstanceTypeID is the FQN of that object's type.
	InstanceTypeID string
	// Error is set when evaluation failed rather than the model answering false.
	Error string
	// Reason says what kind of failure Error reports.
	Reason Reason
	// RequirementID is the FQN of the requirement a satisfaction verdict
	// asserts satisfied, empty when the assertion names none.
	RequirementID string
	// Verifications are the body verdicts of that requirement's verification
	// cases, reported beside this verdict rather than instead of it.
	Verifications []VerificationVerdict
	// Standing is how strongly the verdict stands: the engine that answered, the
	// strength of its evidence and the bounds it ran under.
	Standing Standing
}

// VerdictKind is the verdict a run of a verification case's body produced, one
// literal of the library's VerdictKind enumeration.
type VerdictKind string

// The verdicts a verification case's body produces.
const (
	// VerdictPass is a body whose verdict value is VerdictKind::pass.
	VerdictPass VerdictKind = "pass"
	// VerdictFail is a body whose verdict value is VerdictKind::fail.
	VerdictFail VerdictKind = "fail"
	// VerdictInconclusive is a body that ran and produced no verdict value.
	VerdictInconclusive VerdictKind = "inconclusive"
	// VerdictError is a body whose run could not be carried out.
	VerdictError VerdictKind = "error"
)

// VerificationVerdict is what the body of a verification case answered when it
// ran, which is a separate answer from whether a requirement is satisfied.
// Reported by a service advertising CapabilityVerificationVerdicts.
type VerificationVerdict struct {
	// CaseID is the FQN of the verification case that ran.
	CaseID string
	// Kind is the VerdictKind the body produced.
	Kind VerdictKind
	// Detail is the text of an error verdict, or why an inconclusive one
	// decided nothing; empty for a pass and a fail.
	Detail string
	// Subcase marks the verdict of a case another performed, which the library
	// states no roll-up for and which is therefore reported on its own.
	Subcase bool
	// RequirementID is the FQN of the requirement this verdict was reported
	// for, empty when the case ran for itself rather than for a requirement.
	RequirementID string
}

// Undecided reports a verdict that is no answer about the model: evaluation
// failed instead of the condition holding or not.
func (v *Verdict) Undecided() bool { return v != nil && v.Error != "" }

// Verification is one constraint's or requirement's verdict, with the objects
// it is about.
type Verification struct {
	// Verdict is the answer.
	Verdict *Verdict
	// Verifications are the body verdicts of the verification cases verifying
	// this requirement, empty for a constraint and for a service reporting none.
	Verifications []VerificationVerdict
	// Instances are the objects reachable from the verdict's subject, including
	// it, so its feature values need no follow-up call.
	Instances []*Instance
	// Diagnostics the verification reported.
	Diagnostics []Diagnostic
}

// Satisfaction is the verdict of each satisfaction assertion evaluated, in
// declaration order. A model stating none answers with no verdicts.
type Satisfaction struct {
	// Verdicts is one verdict per assertion evaluated, each carrying the body
	// verdicts of its own requirement.
	Verdicts []Verdict
	// Verifications are the body verdicts of every requirement the response
	// covers, each naming the requirement it was reported for.
	Verifications []VerificationVerdict
	// Instances are the objects the verdicts are about, each reported once.
	Instances []*Instance
	// Diagnostics the verification reported.
	Diagnostics []Diagnostic
}

// Holds reports whether every assertion was decided and held. A model stating
// no assertion holds trivially.
func (s *Satisfaction) Holds() bool {
	for i := range s.Verdicts {
		if s.Verdicts[i].Undecided() || !s.Verdicts[i].Holds {
			return false
		}
	}
	return true
}

// Calculation is what one calculation computed: the value an invocation
// returned, or the output features a calc usage evaluated from its own members.
type Calculation struct {
	// Result is what an invocation returned, nil when Outputs carries the answer.
	Result Value
	// Outputs are a calc usage's output features in declaration order, empty for
	// an invocation with arguments.
	Outputs []CalcOutput
	// Standing is how strongly the calculation stands: the engine that answered,
	// the strength of its evidence and the bounds it ran under.
	Standing Standing
	// Diagnostics the calculation reported.
	Diagnostics []Diagnostic
}

// CalcOutput is one output feature a calc usage computed.
type CalcOutput struct {
	Name  string
	Value Value
}

// VerifyOption configures VerifyConstraint and VerifyRequirement.
type VerifyOption func(*verifyOptions)

type verifyOptions struct {
	subjectSymbolID string
	engine          string
}

// Against names a part or usage to instantiate and verify against, so the
// verdict is about that object's values rather than declared defaults. It is
// what WithSubject is to Evaluate.
func Against(symbolID string) VerifyOption {
	return func(o *verifyOptions) { o.subjectSymbolID = symbolID }
}

func (c *client) VerifyConstraint(
	ctx context.Context,
	model *Model,
	symbolID string,
	opts ...VerifyOption,
) (*Verification, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	options, err := c.verifyOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	resp, err := c.caller.verifyConstraint(ctx, &pb.VerifyConstraintRequest{
		ModelHash:       hash,
		SymbolId:        symbolID,
		SubjectSymbolId: options.subjectSymbolID,
		Engine:          engineField(options.engine),
	})
	if err != nil {
		return nil, err
	}
	return verification("VerifyConstraint", resp.Verdict, resp.Instances, resp.Error,
		ReasonUnspecified, resp.Diagnostics)
}

func (c *client) VerifyRequirement(
	ctx context.Context,
	model *Model,
	symbolID string,
	opts ...VerifyOption,
) (*Verification, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	options, err := c.verifyOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	resp, err := c.caller.verifyRequirement(ctx, &pb.VerifyRequirementRequest{
		ModelHash:       hash,
		SymbolId:        symbolID,
		SubjectSymbolId: options.subjectSymbolID,
		Engine:          engineField(options.engine),
	})
	if err != nil {
		return nil, err
	}
	out, err := verification("VerifyRequirement", resp.Verdict, resp.Instances, resp.Error,
		ReasonUnspecified, resp.Diagnostics)
	if err != nil {
		return nil, err
	}
	out.Verifications = verificationVerdictsFromProto(resp.VerificationVerdicts)
	return out, nil
}

func (c *client) VerifySatisfaction(
	ctx context.Context, model *Model, symbolID string, opts ...VerifyOption,
) (*Satisfaction, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	options, err := c.verifyOptions(ctx, opts)
	if err != nil {
		return nil, err
	}
	if options.subjectSymbolID != "" {
		return nil, &StatusError{
			Code:    CodeInvalidArgument,
			Message: "VerifySatisfaction takes no subject: the assertions name their own",
		}
	}
	resp, err := c.caller.verifySatisfaction(ctx, &pb.VerifySatisfactionRequest{
		ModelHash: hash,
		SymbolId:  symbolID,
		Engine:    engineField(options.engine),
	})
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &VerifyError{
			FailureError: FailureError{Op: "VerifySatisfaction", Message: resp.Error, Diagnostics: diagnostics},
			Reason:       Reason(resp.FailureReason),
		}
	}
	out := &Satisfaction{
		Instances:     instancesFromProto(resp.Instances),
		Diagnostics:   diagnostics,
		Verifications: verificationVerdictsFromProto(resp.VerificationVerdicts),
	}
	for _, verdict := range resp.Verdicts {
		if converted := verdictFromProto(verdict); converted != nil {
			converted.Verifications = verificationsOf(out.Verifications, converted.RequirementID)
			out.Verdicts = append(out.Verdicts, *converted)
		}
	}
	return out, nil
}

func (c *client) EvaluateCalc(
	ctx context.Context,
	model *Model,
	symbolID string,
	arguments ...Value,
) (*Calculation, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireValueCapabilities(ctx, arguments...); err != nil {
		return nil, err
	}
	req := &pb.EvaluateCalcRequest{ModelHash: hash, SymbolId: symbolID}
	for _, argument := range arguments {
		sent, err := valueToProto(argument)
		if err != nil {
			return nil, err
		}
		req.Arguments = append(req.Arguments, sent)
	}
	resp, err := c.caller.evaluateCalc(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &VerifyError{
			FailureError: FailureError{Op: "EvaluateCalc", Message: resp.Error, Diagnostics: diagnostics},
			Reason:       Reason(resp.FailureReason),
		}
	}
	out := &Calculation{
		Result:      valueFromProto(resp.Result),
		Standing:    standingFromProto(resp.Engine, resp.Strength, resp.Bounds),
		Diagnostics: diagnostics,
	}
	for _, output := range resp.Outputs {
		out.Outputs = append(out.Outputs, CalcOutput{Name: output.Name, Value: valueFromProto(output.Value)})
	}
	return out, nil
}

// verifyOptions reads the options and checks the engine they name against the
// capability it needs.
func (c *client) verifyOptions(ctx context.Context, opts []VerifyOption) (verifyOptions, error) {
	var options verifyOptions
	for _, opt := range opts {
		opt(&options)
	}
	if err := c.requireEngine(ctx, options.engine); err != nil {
		return verifyOptions{}, err
	}
	return options, nil
}

// verification builds the answer a constraint or requirement verification gives.
// A request that could not be answered at all is a failure, while a condition
// that could not be evaluated is an undecided verdict.
func verification(
	op string,
	verdict *pb.Verdict,
	instances []*pb.Instance,
	failure string,
	reason Reason,
	diags []*pb.Diagnostic,
) (*Verification, error) {
	diagnostics := diagnosticsFromProto(diags)
	if failure != "" {
		return nil, &VerifyError{
			FailureError: FailureError{Op: op, Message: failure, Diagnostics: diagnostics},
			Reason:       reason,
		}
	}
	return &Verification{
		Verdict:     verdictFromProto(verdict),
		Instances:   instancesFromProto(instances),
		Diagnostics: diagnostics,
	}, nil
}

func verdictFromProto(verdict *pb.Verdict) *Verdict {
	if verdict == nil {
		return nil
	}
	return &Verdict{
		Kind:           verdict.Kind,
		ElementID:      verdict.ElementId,
		Element:        verdict.Element,
		Holds:          verdict.Holds,
		Condition:      verdict.Condition,
		InstanceID:     verdict.InstanceId,
		InstanceTypeID: verdict.InstanceTypeId,
		Error:          verdict.Error,
		Reason:         Reason(verdict.FailureReason),
		RequirementID:  verdict.RequirementId,
		Standing:       standingFromProto(verdict.Engine, verdict.Strength, verdict.Bounds),
	}
}

func verificationVerdictsFromProto(verdicts []*pb.VerificationVerdict) []VerificationVerdict {
	if len(verdicts) == 0 {
		return nil
	}
	out := make([]VerificationVerdict, 0, len(verdicts))
	for _, verdict := range verdicts {
		if verdict == nil {
			continue
		}
		out = append(out, VerificationVerdict{
			CaseID:        verdict.CaseId,
			Kind:          VerdictKind(verdict.Kind),
			Detail:        verdict.Detail,
			Subcase:       verdict.Subcase,
			RequirementID: verdict.RequirementId,
		})
	}
	return out
}

// verificationsOf is the body verdicts of one requirement. A response answering
// several assertions carries the cases of every requirement it covers, so a
// verdict takes only its own; naming no requirement takes none rather than all.
func verificationsOf(verdicts []VerificationVerdict, requirementID string) []VerificationVerdict {
	if requirementID == "" {
		return nil
	}
	var out []VerificationVerdict
	for _, verdict := range verdicts {
		if verdict.RequirementID == requirementID {
			out = append(out, verdict)
		}
	}
	return out
}
