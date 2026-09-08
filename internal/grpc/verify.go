package grpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// Verdict kinds, as reported in Verdict.kind.
const (
	verdictConstraint  = "constraint"
	verdictRequirement = "requirement"
	verdictSatisfy     = "satisfy"
)

// failureReason classifies a failure for the wire, so a client acts on the kind
// of failure rather than on the message text.
func failureReason(err error) pb.FailureReason {
	switch {
	case err == nil:
		return pb.FailureReason_FAILURE_REASON_UNSPECIFIED
	case errors.Is(err, runtime.ErrAmbiguousSubject):
		return pb.FailureReason_FAILURE_REASON_AMBIGUOUS_SUBJECT
	case errors.Is(err, runtime.ErrNotAConstraint),
		errors.Is(err, runtime.ErrNotARequirement),
		errors.Is(err, runtime.ErrNotASatisfaction),
		errors.Is(err, runtime.ErrNotACalc),
		errors.Is(err, runtime.ErrNotAnAnalysis):
		return pb.FailureReason_FAILURE_REASON_WRONG_KIND
	default:
		return pb.FailureReason_FAILURE_REASON_EVALUATION
	}
}

// verifyContext is everything a verification RPC needs from a cached model: the
// runtime it evaluates in and the index its names resolve against.
type verifyContext struct {
	service *Service
	cached  *CachedModel
	runtime *runtime.Context
	sem     *semantics.Model
}

// newVerifyContext looks the model up and builds a runtime over it, the same way
// every other runtime RPC in this service does; the caller defers release.
func (s *Service) newVerifyContext(modelHash string) (*verifyContext, func(), error) {
	cached, ok := s.cache.Get(modelHash)
	if !ok {
		return nil, nil, statusErrorf(connect.CodeNotFound, "model not found: %s", modelHash)
	}
	rt, sem, release := s.newRuntime(cached)
	return &verifyContext{service: s, cached: cached, runtime: rt, sem: sem}, release, nil
}

// lookup resolves an FQN to the symbol it names.
func (v *verifyContext) lookup(symbolID string) (*symbols.Symbol, error) {
	syms := lookupNamed(v.cached.Index, symbolID)
	if len(syms) == 0 {
		return nil, fmt.Errorf("symbol not found: %s", symbolID)
	}
	return syms[0], nil
}

// declaringScope is the scope an element's conditions were written in, which is
// what their names resolve against; the document root is the fallback for a
// symbol carrying no declaring scope.
func (v *verifyContext) declaringScope(sym *symbols.Symbol) *symbols.Scope {
	if sym != nil && sym.OwnerScope != nil {
		return sym.OwnerScope
	}
	return v.cached.PrimaryRoot()
}

// subject instantiates the part/usage a request named, so a verdict can be about
// concrete values. An empty name is no subject, which is not an error: the
// verdict is then about declared defaults.
func (v *verifyContext) subject(symbolID string) (*runtime.Instance, error) {
	if symbolID == "" {
		return nil, nil
	}
	sym, err := v.lookup(symbolID)
	if err != nil {
		return nil, err
	}
	inst, err := v.runtime.Instantiate(sym)
	if err != nil {
		return nil, fmt.Errorf("instantiation of subject %s failed: %w", symbolID, err)
	}
	return inst, nil
}

// namedFQN is the symbol's qualified name, or "" where it or one of its owners
// is anonymous: such an element has no name a caller can look up.
func namedFQN(idx *symbols.Index, sym *symbols.Symbol) string {
	for s := sym; s != nil; {
		if s.Name == "" {
			return ""
		}
		if s.OwnerScope == nil {
			break
		}
		s = s.OwnerScope.Owner()
	}
	return idx.GetFQN(sym)
}

// verdict builds the answer to one verification. A condition that evaluated to
// false is the model's answer, reported as holds=false with the condition named;
// any other error is a failure to evaluate, reported in Verdict.error.
func (v *verifyContext) verdict(kind string, sym *symbols.Symbol, element string, inst *runtime.Instance, holds bool, err error) *pb.Verdict {
	out := &pb.Verdict{
		Kind:    kind,
		Element: element,
		Holds:   holds && err == nil,
	}
	if sym != nil {
		out.ElementId = namedFQN(v.cached.Index, sym)
		if out.Element == "" {
			out.Element = out.ElementId
		}
	}
	if inst != nil {
		out.InstanceId = inst.ID
		out.InstanceTypeId = namedFQN(v.cached.Index, inst.Type)
	}

	var violation *runtime.ViolationError
	switch {
	case err == nil:
		// Nothing to explain: holds is the model's answer either way.
	case errors.As(err, &violation):
		out.Condition = violation.Condition
	case errors.Is(err, runtime.ErrViolated):
		// A verdict of false the runtime did not attribute to one condition.
	default:
		out.Error = err.Error()
		out.FailureReason = failureReason(err)
	}
	return out
}

// instanceGraph serializes the object a verdict is about, so a client can read
// the feature values behind a failure without a follow-up call.
func (v *verifyContext) instanceGraph(inst *runtime.Instance) []*pb.Instance {
	if inst == nil {
		return nil
	}
	_, all := v.service.instanceGraphToProto(v.runtime, inst, v.cached.Index)
	return all
}

// VerifyConstraint evaluates a constraint definition or usage, as the REPL's
// %constraint does.
func (s *Service) VerifyConstraint(ctx context.Context, req *pb.VerifyConstraintRequest) (*pb.VerifyConstraintResponse, error) {
	if err := s.requireCapability(CapabilityVerification); err != nil {
		return nil, err
	}
	v, release, err := s.newVerifyContext(req.ModelHash)
	if err != nil {
		return nil, err
	}
	defer release()
	sym, err := v.lookup(req.SymbolId)
	if err != nil {
		return &pb.VerifyConstraintResponse{Error: err.Error()}, nil
	}
	inst, err := v.subject(req.SubjectSymbolId)
	if err != nil {
		return &pb.VerifyConstraintResponse{Error: err.Error()}, nil
	}

	result, evalErr := v.runtime.CheckConstraintOn(sym, v.declaringScope(sym), inst)
	subject := subjectOf(result, inst)
	return &pb.VerifyConstraintResponse{
		Verdict:   v.verdict(verdictConstraint, sym, "", subject, result.Holds, evalErr),
		Instances: v.instanceGraph(subject),
	}, nil
}

// VerifyRequirement evaluates a requirement definition or usage, as the REPL's
// %requirement does.
func (s *Service) VerifyRequirement(ctx context.Context, req *pb.VerifyRequirementRequest) (*pb.VerifyRequirementResponse, error) {
	if err := s.requireCapability(CapabilityVerification); err != nil {
		return nil, err
	}
	v, release, err := s.newVerifyContext(req.ModelHash)
	if err != nil {
		return nil, err
	}
	defer release()
	sym, err := v.lookup(req.SymbolId)
	if err != nil {
		return &pb.VerifyRequirementResponse{Error: err.Error()}, nil
	}
	inst, err := v.subject(req.SubjectSymbolId)
	if err != nil {
		return &pb.VerifyRequirementResponse{Error: err.Error()}, nil
	}

	result, evalErr := v.runtime.CheckRequirementOn(sym, v.declaringScope(sym), inst)
	subject := subjectOf(result, inst)
	return &pb.VerifyRequirementResponse{
		Verdict:   v.verdict(verdictRequirement, sym, "", subject, result.Holds, evalErr),
		Instances: v.instanceGraph(subject),
		// Beside the satisfaction verdict: what the cases verifying this
		// requirement answered when their bodies ran.
		VerificationVerdicts: v.requirementVerifications(sym),
	}, nil
}

// VerifySatisfaction evaluates the satisfaction assertions a model states, as the
// REPL's %satisfy does: every one, or, given a symbol, the ones that element
// states — or that element itself when it is a named satisfy assertion.
func (s *Service) VerifySatisfaction(ctx context.Context, req *pb.VerifySatisfactionRequest) (*pb.VerifySatisfactionResponse, error) {
	if err := s.requireCapability(CapabilityVerification); err != nil {
		return nil, err
	}
	v, release, err := s.newVerifyContext(req.ModelHash)
	if err != nil {
		return nil, err
	}
	defer release()

	// Every document of the model states assertions, unless one scope is named.
	scopes := v.cached.DocumentRoots()
	if req.SymbolId != "" {
		sym, lerr := v.lookup(req.SymbolId)
		if lerr != nil {
			return &pb.VerifySatisfactionResponse{Error: lerr.Error()}, nil
		}
		// A named `satisfy requirement r by p` is itself one assertion.
		if a, aerr := v.runtime.SatisfyAssertionOf(sym); aerr == nil {
			verdict, instances := v.satisfyVerdict(a)
			return &pb.VerifySatisfactionResponse{
				Verdicts:             []*pb.Verdict{verdict},
				Instances:            instances,
				VerificationVerdicts: v.assertionVerifications(a),
			}, nil
		}
		// A symbol that is neither an assertion nor a scope stating any is the
		// wrong kind of thing to ask about, not an undecided answer.
		if sym.Scope == nil {
			return &pb.VerifySatisfactionResponse{
				Error:         fmt.Sprintf("%s states no satisfaction assertion", req.SymbolId),
				FailureReason: pb.FailureReason_FAILURE_REASON_WRONG_KIND,
			}, nil
		}
		scopes = []*symbols.Scope{sym.Scope}
	}

	resp := &pb.VerifySatisfactionResponse{}
	seen := map[int64]bool{}
	// The cases verifying one requirement answer once for the response, however
	// many assertions of that requirement it reports.
	verified := map[*symbols.Symbol]bool{}
	for _, scope := range scopes {
		for _, a := range v.runtime.SatisfyAssertionsIn(scope) {
			verdict, instances := v.satisfyVerdict(a)
			resp.Verdicts = append(resp.Verdicts, verdict)
			if req := a.AssertedRequirement(); req != nil && !verified[req] {
				verified[req] = true
				resp.VerificationVerdicts = append(resp.VerificationVerdicts, v.assertionVerifications(a)...)
			}
			// One graph per response, so two assertions about the same object do
			// not report it twice.
			for _, inst := range instances {
				if !seen[inst.Id] {
					seen[inst.Id] = true
					resp.Instances = append(resp.Instances, inst)
				}
			}
		}
	}
	return resp, nil
}

// satisfyVerdict evaluates one assertion against an object of its subject, built
// for this call so the verdict is about the values that subject holds.
func (v *verifyContext) satisfyVerdict(a *runtime.SatisfyAssertion) (*pb.Verdict, []*pb.Instance) {
	var subject *runtime.Instance
	if a.Subject != nil {
		// Created here rather than inside the evaluation so that the object the
		// verdict is about can be reported with it.
		inst, err := v.runtime.SatisfySubject(a)
		if err != nil {
			// The assertion cannot be evaluated without the object it is about,
			// which is a failure to evaluate rather than a verdict of false.
			verdict := v.verdict(verdictSatisfy, a.Symbol, a.Text(), nil, false, err)
			v.associateRequirement(verdict, a)
			return verdict, nil
		}
		subject = inst
	}
	result, err := v.runtime.CheckSatisfactionOn(a, subject)
	subject = subjectOf(result, subject)
	verdict := v.verdict(verdictSatisfy, a.Symbol, a.Text(), subject, result.Holds, err)
	v.associateRequirement(verdict, a)
	return verdict, v.instanceGraph(subject)
}

// associateRequirement names on a satisfaction verdict the requirement it
// asserts satisfied, which the body verdicts of that requirement also name.
func (v *verifyContext) associateRequirement(verdict *pb.Verdict, a *runtime.SatisfyAssertion) {
	if req := a.AssertedRequirement(); req != nil {
		verdict.RequirementId = namedFQN(v.cached.Index, req)
	}
}

// subjectOf is the object a verdict is about: the one the runtime evaluated the
// check against, which for a check reached through a nested redefinition is not
// the object supplied. fallback covers a check that never reached evaluation.
func subjectOf(result runtime.CheckResult, fallback *runtime.Instance) *runtime.Instance {
	if result.Subject != nil {
		return result.Subject
	}
	return fallback
}

// EvaluateCalc invokes a calculation, as the REPL's %calc does: a calc usage
// named with no arguments is evaluated from its own members and reports every
// output feature it computes (SysML 7.17); anything else is invoked with the
// arguments given, bound positionally.
func (s *Service) EvaluateCalc(ctx context.Context, req *pb.EvaluateCalcRequest) (*pb.EvaluateCalcResponse, error) {
	if err := s.requireCapability(CapabilityVerification); err != nil {
		return nil, err
	}
	v, release, err := s.newVerifyContext(req.ModelHash)
	if err != nil {
		return nil, err
	}
	defer release()
	sym, err := v.lookup(req.SymbolId)
	if err != nil {
		return &pb.EvaluateCalcResponse{Error: err.Error()}, nil
	}

	if len(req.Arguments) == 0 {
		outputs, handled, cerr := v.calcUsageOutputs(sym)
		if cerr != nil {
			return &pb.EvaluateCalcResponse{
				Error:         cerr.Error(),
				FailureReason: failureReason(cerr),
			}, nil
		}
		if handled {
			return &pb.EvaluateCalcResponse{Outputs: outputs}, nil
		}
	}

	// Converted against the model's index, so a quantity argument keeps the base
	// units it is commensurable with instead of arriving as an unusable value.
	args := make([]runtime.Value, 0, len(req.Arguments))
	for _, arg := range req.Arguments {
		if err := s.requireValueCapabilities(arg); err != nil {
			return nil, err
		}
		val, cerr := ProtoToValueIn(arg, v.cached.Index, v.sem)
		if cerr != nil {
			return &pb.EvaluateCalcResponse{
				Error:         fmt.Sprintf("calc argument could not be read: %v", cerr),
				FailureReason: failureReason(cerr),
			}, nil
		}
		args = append(args, val)
	}

	result, err := v.runtime.InvokeCalc(sym, args, v.declaringScope(sym))
	if err != nil {
		return &pb.EvaluateCalcResponse{
			Error:         fmt.Sprintf("calc invocation failed: %v", err),
			FailureReason: failureReason(err),
		}, nil
	}
	return &pb.EvaluateCalcResponse{Result: v.service.valueToProto(v.runtime, result, v.cached.Index)}, nil
}

// calcUsageOutputs evaluates a calc usage from its own member values. It reports
// handled=false when sym is not a calc usage, or is one computing no output
// features, so those are invoked as calculations instead.
func (v *verifyContext) calcUsageOutputs(sym *symbols.Symbol) ([]*pb.CalcOutput, bool, error) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageCalc {
		return nil, false, nil
	}
	outputs, err := v.runtime.CalcUsageOutputs(sym, sym.OwnerScope, nil)
	if err != nil {
		return nil, true, fmt.Errorf("calc usage evaluation failed: %w", err)
	}
	if len(outputs) == 0 {
		return nil, false, nil
	}
	pbOutputs := make([]*pb.CalcOutput, 0, len(outputs))
	for _, out := range outputs {
		pbOutputs = append(pbOutputs, &pb.CalcOutput{
			Name:  out.Name,
			Value: v.service.valueToProto(v.runtime, out.Value, v.cached.Index),
		})
	}
	return pbOutputs, true, nil
}

// requirementVerifications are the body verdicts of the verification cases of
// the whole model whose objective verifies req, since the case need not be
// written in the document the requirement is.
func (v *verifyContext) requirementVerifications(req *symbols.Symbol) []*pb.VerificationVerdict {
	return v.verificationVerdicts(
		v.runtime.VerificationVerdictsIn(v.cached.DocumentRoots(), req),
		namedFQN(v.cached.Index, req))
}

// assertionVerifications are the body verdicts of the verification cases whose
// objective verifies the requirement an assertion satisfies.
func (v *verifyContext) assertionVerifications(a *runtime.SatisfyAssertion) []*pb.VerificationVerdict {
	req := a.AssertedRequirement()
	if req == nil {
		return nil
	}
	return v.requirementVerifications(req)
}
