package grpc

import (
	"context"
	"strings"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// verdictObject is the kind of the verdict about a validated object as a whole.
const verdictObject = "object"

// ValidateInstance checks every assertion about an object of the part named and the
// objects it holds, reporting one verdict per assertion beside a summary for the object.
func (s *Service) ValidateInstance(ctx context.Context, req *pb.ValidateInstanceRequest) (*pb.ValidateInstanceResponse, error) {
	if err := s.requireCapability(CapabilityVerification); err != nil {
		return nil, err
	}
	v, err := s.newVerifyContext(req.ModelHash, req.Engine)
	if err != nil {
		return nil, err
	}
	defer v.release()
	if req.SymbolId == "" {
		return &pb.ValidateInstanceResponse{Error: "symbol_id names no part to validate an object of"}, nil
	}
	root, err := v.subject(req.SymbolId)
	if err != nil {
		return &pb.ValidateInstanceResponse{Error: err.Error(), FailureReason: failureReason(err)}, nil
	}

	// One question to the engines, the object as a whole, whose standing every
	// verdict of the answer carries.
	report, plan, evalErr := perform(ctx, v, req.SymbolId, func(rt *runtime.Context) (runtime.ValidationReport, error) {
		return rt.ValidateObject(root, v.cached.DocumentRoots())
	}, analysis.ValidationAnswer)
	if err := callerGone(ctx, evalErr); err != nil {
		return nil, err
	}
	if evalErr != nil {
		return &pb.ValidateInstanceResponse{
			Error:         evalErr.Error(),
			FailureReason: failureReason(evalErr),
			Instances:     v.instanceGraph(root),
		}, nil
	}

	resp := &pb.ValidateInstanceResponse{
		Summary:   v.summaryVerdict(req.SymbolId, root, report, plan),
		Instances: v.instanceGraph(root),
		Bounded:   report.Bounded,
	}
	// The cases verifying one requirement answer once for the response, however
	// many verdicts are about that requirement.
	verified := map[*symbols.Symbol]bool{}
	for _, ov := range report.Verdicts {
		resp.Verdicts = append(resp.Verdicts, v.objectVerdict(ov, plan))
		if req := ov.Requirement; req != nil && !verified[req] {
			verified[req] = true
			resp.VerificationVerdicts = append(resp.VerificationVerdicts, v.requirementVerifications(req)...)
		}
	}
	return resp, nil
}

// objectVerdict is one assertion's verdict about one object of a validated tree,
// naming the object by its path from the validated one.
func (v *verifyContext) objectVerdict(ov runtime.ObjectVerdict, plan analysis.Plan) *pb.Verdict {
	kind := verdictConstraint
	switch ov.Kind {
	case runtime.AssertionRequirement:
		kind = verdictRequirement
	case runtime.AssertionSatisfaction:
		kind = verdictSatisfy
	}
	out := v.verdict(kind, ov.Element, ov.Text, ov.Subject, ov.Status == runtime.ValidationHolds, ov.Err, plan)
	out.InstancePath = strings.Join(ov.Path, ".")
	if ov.Requirement != nil {
		out.RequirementId = namedFQN(v.cached.Index, ov.Requirement)
	}
	return out
}

// summaryVerdict is the verdict about the object as a whole: it holds when every
// assertion holds and the walk was complete, and is undecided with the reason otherwise.
func (v *verifyContext) summaryVerdict(symbolID string, root *runtime.Instance, report runtime.ValidationReport, plan analysis.Plan) *pb.Verdict {
	out := v.service.standingOf(plan).stamp(&pb.Verdict{
		Kind:           verdictObject,
		ElementId:      symbolID,
		Element:        symbolID,
		Holds:          report.Valid(),
		InstanceId:     root.ID,
		InstanceTypeId: namedFQN(v.cached.Index, root.Type),
	})
	if !report.Valid() && report.Status() != runtime.ValidationViolated {
		out.Error = analysis.ValidationReason(report)
		out.FailureReason = pb.FailureReason_FAILURE_REASON_EVALUATION
	}
	return out
}
