package grpc

import (
	"context"
	"fmt"
	"sort"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// RunAnalysis runs an analysis case, as the REPL's %analysis does: the subject
// named is instantiated and bound, the arguments bind the case's inputs, and
// the response carries every output with the verdict of each objective and
// assertion (SysML 7.22).
func (s *Service) RunAnalysis(ctx context.Context, req *pb.RunAnalysisRequest) (*pb.RunAnalysisResponse, error) {
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
		return &pb.RunAnalysisResponse{Error: err.Error()}, nil
	}
	if err := v.runtime.RequireAnalysisCase(sym); err != nil {
		return &pb.RunAnalysisResponse{Error: err.Error(), FailureReason: failureReason(err)}, nil
	}
	subject, err := v.subject(req.SubjectSymbolId)
	if err != nil {
		return &pb.RunAnalysisResponse{Error: err.Error()}, nil
	}

	args := runtime.AnalysisArgs{Subject: subject}
	for _, arg := range req.Arguments {
		val, resp, err := v.analysisArgument(arg)
		if resp != nil || err != nil {
			return resp, err
		}
		args.Positional = append(args.Positional, val)
	}
	if len(req.NamedArguments) > 0 {
		// Read in name order so a failure names the same argument every time.
		names := make([]string, 0, len(req.NamedArguments))
		for name := range req.NamedArguments {
			names = append(names, name)
		}
		sort.Strings(names)
		args.Named = make(map[string]runtime.Value, len(names))
		for _, name := range names {
			val, resp, err := v.analysisArgument(req.NamedArguments[name])
			if resp != nil || err != nil {
				return resp, err
			}
			args.Named[name] = val
		}
	}

	// A verification case runs the same body, and its run answers with the
	// verdict that body produced as well. The choices a run made are reported
	// with its outcome, failed or not.
	var verdicts []runtime.VerificationVerdict
	var result runtime.AnalysisResult
	if runtime.IsVerificationCaseSymbol(sym) {
		verified, verr := v.runtime.RunVerification(sym, args, v.declaringScope(sym), nil)
		if verr != nil {
			return &pb.RunAnalysisResponse{
				Error:         fmt.Sprintf("verification run failed: %v", verr),
				FailureReason: failureReason(verr),
				Diagnostics:   v.service.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(v.runtime.Notes(), v.cached)),
			}, nil
		}
		result = verified.Run
		verdicts = append([]runtime.VerificationVerdict{verified.Verdict}, verified.Subcases...)
	} else if result, err = v.runtime.RunAnalysis(sym, args, v.declaringScope(sym), nil); err != nil {
		return &pb.RunAnalysisResponse{
			Error:         fmt.Sprintf("analysis run failed: %v", err),
			FailureReason: failureReason(err),
			Diagnostics:   v.service.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(v.runtime.Notes(), v.cached)),
		}, nil
	}
	diags := v.service.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(v.runtime.Notes(), v.cached))
	// The case reports the subject it ran on: the one supplied, or the one the
	// usage or the enclosing case bound.
	subject = result.Subject
	resp := &pb.RunAnalysisResponse{Instances: v.instanceGraph(subject), Diagnostics: diags}
	for _, out := range result.Outputs {
		resp.Outputs = append(resp.Outputs, &pb.CalcOutput{
			Name:  out.Name,
			Value: v.service.valueToProto(v.runtime, out.Value, v.cached.Index),
		})
	}
	for i := range result.Verdicts {
		resp.Verdicts = append(resp.Verdicts, v.analysisVerdict(&result.Verdicts[i], subject))
	}
	// A case run for itself answers for no requirement, so nothing associates it.
	resp.VerificationVerdicts = v.verificationVerdicts(verdicts, "")
	return resp, nil
}

// verificationVerdicts spells for the wire what the bodies of verification cases
// answered, in the order they were reported. requirementID names the requirement
// they were reported for, so a response covering several keeps them apart.
func (v *verifyContext) verificationVerdicts(verdicts []runtime.VerificationVerdict, requirementID string) []*pb.VerificationVerdict {
	if len(verdicts) == 0 {
		return nil
	}
	out := make([]*pb.VerificationVerdict, 0, len(verdicts))
	for _, verdict := range verdicts {
		out = append(out, &pb.VerificationVerdict{
			CaseId:        verdict.Case,
			Kind:          string(verdict.Kind),
			Detail:        verdict.Detail,
			Subcase:       verdict.Subcase,
			RequirementId: requirementID,
		})
	}
	return out
}

// analysisArgument reads one argument off the wire against the model's index,
// so a quantity keeps the units it is commensurable with. One the service
// cannot read answers the request; one needing a capability it lacks fails the call.
func (v *verifyContext) analysisArgument(arg *pb.Value) (runtime.Value, *pb.RunAnalysisResponse, error) {
	if err := v.service.requireValueCapabilities(arg); err != nil {
		return runtime.Value{}, nil, err
	}
	val, err := ProtoToRuntimeValue(v.runtime, arg, v.cached.Index, v.sem)
	if err != nil {
		return runtime.Value{}, &pb.RunAnalysisResponse{
			Error:         fmt.Sprintf("analysis argument could not be read: %v", err),
			FailureReason: failureReason(err),
		}, nil
	}
	return val, nil, nil
}

// analysisVerdict spells what a check of the case decided as a Verdict of kind
// "objective" or "assertion": satisfied holds, not satisfied names the violated
// condition, and undecided is an evaluation failure rather than an answer.
func (v *verifyContext) analysisVerdict(verdict *runtime.AnalysisVerdict, subject *runtime.Instance) *pb.Verdict {
	out := &pb.Verdict{
		Kind:    verdict.Kind,
		Element: verdict.Name,
		Holds:   verdict.Status == runtime.VerdictSatisfied,
	}
	if verdict.Symbol != nil {
		out.ElementId = namedFQN(v.cached.Index, verdict.Symbol)
	}
	if subject != nil {
		out.InstanceId = subject.ID
		out.InstanceTypeId = namedFQN(v.cached.Index, subject.Type)
	}
	switch verdict.Status {
	case runtime.VerdictNotSatisfied:
		out.Condition = verdict.Detail
	case runtime.VerdictUndecided:
		out.Error = verdict.Detail
		out.FailureReason = pb.FailureReason_FAILURE_REASON_EVALUATION
	}
	return out
}
