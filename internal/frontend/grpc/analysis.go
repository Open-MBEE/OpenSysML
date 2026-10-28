package grpc

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// RunAnalysis runs an analysis case, as the REPL's %analysis does: the subject
// named is instantiated and bound, the arguments bind the case's inputs, and
// the response carries every output with the verdict of each objective and
// assertion (SysML 7.22).
func (s *Service) RunAnalysis(ctx context.Context, req *pb.RunAnalysisRequest) (*pb.RunAnalysisResponse, error) {
	schedule, err := s.schedulePolicy(req.Schedule)
	if err != nil {
		return nil, err
	}
	if err := s.requireCapability(CapabilityVerification); err != nil {
		return nil, err
	}
	v, err := s.newVerifyContext(ctx, req.ModelHash, req.Engine)
	if err != nil {
		return nil, err
	}
	defer v.release()
	policy, explores := analysis.Explores(v.engine, schedule)
	if req.ToolDryRun && explores {
		return nil, statusError(connect.CodeInvalidArgument, "a tool dry run previews one run of the case; it is unavailable under an explore schedule or engine")
	}
	sym, err := v.lookup(req.SymbolId)
	if err != nil {
		return &pb.RunAnalysisResponse{Error: err.Error()}, nil
	}
	if err := v.runtime.RequireAnalysisCase(sym); err != nil {
		return &pb.RunAnalysisResponse{Error: err.Error(), FailureReason: failureReason(err)}, nil
	}
	args, resp, err := v.analysisArgs(req)
	if resp != nil || err != nil {
		return resp, err
	}

	if explores {
		return s.exploreAnalysis(ctx, policy, v, req, sym)
	}
	// A case naming no subject runs over the objects the model holds, a type-owned
	// one as a feature of the object of its type held, as the REPL's %analysis does.
	if on, unlock := v.overHeld(ctx, args.Subject); on != v {
		defer unlock()
		v = on
		// Read again on the runtime the run is made on, as an explored run reads them.
		if args, resp, err = v.analysisArgs(req); resp != nil || err != nil {
			return resp, err
		}
	}
	reset, err := v.scheduled(schedule)
	if err != nil {
		return nil, statusError(connect.CodeInvalidArgument, err.Error())
	}
	defer reset()
	if req.ToolDryRun {
		return s.toolDryRun(ctx, v, sym, args)
	}

	// The choices a run made are reported with its outcome, failed or not.
	run, plan, err := v.runCase(ctx, sym, args)
	if gone := callerGone(ctx, err); gone != nil {
		return nil, gone
	}
	if status := v.exhaustedRun(run, err); status != nil {
		return nil, status
	}
	return v.caseResponse(run, plan, err), nil
}

// exhaustedRun is the RESOURCE_EXHAUSTED status of a run over the held population
// that would pass its bound, whether the call failed of it or a verification case's
// verdict carries the failure, as a case run reports its own; nil for any other run.
func (v *verifyContext) exhaustedRun(run caseRun, err error) error {
	if status := v.exhausted(err); status != nil {
		return status
	}
	for _, verdict := range run.verdicts() {
		if status := v.exhausted(verdict.Err); status != nil {
			return status
		}
	}
	return nil
}

// scheduled sets the runtime's schedule for this request and returns what puts
// back the one it had, so a request leaves the held population's runtime as it
// found it; a policy the runtime cannot be set to is the error.
func (v *verifyContext) scheduled(policy runtime.SchedulePolicy) (func(), error) {
	was := v.runtime.Schedule()
	if err := v.runtime.SetSchedule(policy); err != nil {
		return nil, err
	}
	return func() {
		// The policy the runtime held was set once already, so it is settable.
		_ = v.runtime.SetSchedule(was)
	}, nil
}

// caseResponse reports one run of a case: its outputs, verdicts, evaluations
// and instances, or the failure that stopped it, with the plan's standing.
func (v *verifyContext) caseResponse(run caseRun, plan analysis.Plan, err error) *pb.RunAnalysisResponse {
	result, verdicts := run.result, run.verdicts()
	st := v.service.standingOf(plan)
	resp := analysisResponse(&pb.RunAnalysisResponse{}, st)
	if err != nil {
		resp.Error = err.Error()
		resp.FailureReason = failureReason(err)
		// A client predating case_evaluations reads a failed run as its error alone.
		if !v.service.capabilities.has(CapabilityCaseEvaluations) {
			resp.Diagnostics = v.service.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(v.runtime.Notes(), v.cached))
			return resp
		}
	}
	resp.Diagnostics = v.service.filterDiagnosticCapabilities(RunNoteDiagnosticsToProto(v.runtime.Notes(), v.cached))
	// The case reports the subject it ran on: the one supplied, or the one the
	// usage or the enclosing case bound.
	subject := result.Subject
	for _, out := range result.Outputs {
		resp.Outputs = append(resp.Outputs, &pb.CalcOutput{
			Name:  out.Name,
			Value: v.service.valueToProto(v.runtime, out.Value, v.cached.Index),
		})
	}
	for i := range result.Verdicts {
		resp.Verdicts = append(resp.Verdicts, st.stamp(v.analysisVerdict(&result.Verdicts[i], subject)))
	}
	// A case run for itself answers for no requirement, so nothing associates it.
	resp.VerificationVerdicts = v.verificationVerdicts(verdicts, "")
	// A client predating case_evaluations sees no evaluation, so no object only one names.
	var evaluations []runtime.AnalysisEvaluation
	if v.service.capabilities.has(CapabilityCaseEvaluations) {
		evaluations = result.Evaluations
		resp.Evaluations = v.caseEvaluations(evaluations)
	}
	resp.Instances = v.instanceGraphs(v.runRoots(subject, verdicts, result.Outputs, evaluations))
	return resp
}

// analysisResponse writes on a case's response the standing of the plan that ran it.
func analysisResponse(resp *pb.RunAnalysisResponse, st standing) *pb.RunAnalysisResponse {
	resp.Engine, resp.Strength, resp.Bounds = st.engine, st.strength, st.bounds
	return resp
}

// analysisArgs reads the request's subject and arguments on the context's own
// runtime; an unreadable one answers the request, a capability gap fails the call.
func (v *verifyContext) analysisArgs(req *pb.RunAnalysisRequest) (runtime.AnalysisArgs, *pb.RunAnalysisResponse, error) {
	subject, err := v.subject(req.SubjectSymbolId)
	if err != nil {
		return runtime.AnalysisArgs{}, &pb.RunAnalysisResponse{Error: err.Error()}, nil
	}
	args := runtime.AnalysisArgs{Subject: subject}
	for _, arg := range req.Arguments {
		val, resp, err := v.analysisArgument(arg)
		if resp != nil || err != nil {
			return runtime.AnalysisArgs{}, resp, err
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
				return runtime.AnalysisArgs{}, resp, err
			}
			args.Named[name] = val
		}
	}
	return args, nil, nil
}

// caseRun is one run of a case: a verification case's verdict beside the run.
type caseRun struct {
	result   runtime.AnalysisResult
	verified *runtime.VerificationResult
}

// verdicts are the verification verdicts the run answered, none for an analysis case.
func (r caseRun) verdicts() []runtime.VerificationVerdict {
	if r.verified == nil {
		return nil
	}
	return append([]runtime.VerificationVerdict{r.verified.Verdict}, r.verified.Subcases...)
}

// answer is what the run established: a verification case its verdict, an
// analysis case its outputs.
func (r caseRun) answer(err error) analysis.Answer {
	if r.verified != nil {
		return analysis.VerificationAnswer(*r.verified, err)
	}
	return analysis.CaseAnswer(r.result, err)
}

// runCaseOn runs the case once on the context's runtime; a run failing after
// computing something keeps that beside the error.
func (v *verifyContext) runCaseOn(sym *symbols.Symbol, args runtime.AnalysisArgs) (caseRun, error) {
	if runtime.IsVerificationCaseSymbol(sym) {
		verified, err := v.runtime.RunVerification(sym, args, v.declaringScope(sym), nil)
		if err != nil {
			return caseRun{}, fmt.Errorf("verification run failed: %w", err)
		}
		return caseRun{result: verified.Run, verified: &verified}, nil
	}
	result, err := v.runtime.RunAnalysis(sym, args, v.declaringScope(sym), nil)
	if err != nil {
		return caseRun{result: result}, fmt.Errorf("analysis run failed: %w", err)
	}
	return caseRun{result: result}, nil
}

// runCase puts one run of the case on the context's runtime to the engines.
func (v *verifyContext) runCase(ctx context.Context, sym *symbols.Symbol, args runtime.AnalysisArgs) (caseRun, analysis.Plan, error) {
	return perform(ctx, v, v.cached.Index.GetFQN(sym), func(*runtime.Context) (caseRun, error) {
		return v.runCaseOn(sym, args)
	}, caseRun.answer)
}

// exploreAnalysis runs the case once per linearization on a context of its own
// and answers every distinct outcome of outputs and verdicts.
func (s *Service) exploreAnalysis(ctx context.Context, schedule runtime.SchedulePolicy, v *verifyContext, req *pb.RunAnalysisRequest, sym *symbols.Symbol) (*pb.RunAnalysisResponse, error) {
	x, err := s.explore(ctx, v.cached.Index.GetFQN(sym), schedule, v.engine, v.cached, func(rt *runtime.Context) (runtime.Outcome, error) {
		fresh := v.on(rt)
		args, resp, err := fresh.analysisArgs(req)
		if err != nil {
			return runtime.Outcome{}, err
		}
		if resp != nil {
			return runtime.Outcome{}, errors.New(resp.Error)
		}
		run, err := fresh.runCaseOn(sym, args)
		if err != nil {
			return runtime.Outcome{}, err
		}
		return rt.VerifiedOutcome(run.result, run.verdicts()), nil
	})
	if err != nil {
		return nil, err
	}
	if x.setupErr != nil {
		return analysisResponse(&pb.RunAnalysisResponse{
			Error:         x.setupErr.Error(),
			FailureReason: failureReason(x.setupErr),
		}, s.standingOf(x.plan)), nil
	}
	return analysisResponse(&pb.RunAnalysisResponse{Outcomes: x.outcomes, Exploration: x.status}, s.standingOf(x.plan)), nil
}

// runRoots are the objects a case run reports: its subject, the object each
// verification verdict is about (the one a type-owned case ran as a feature of,
// which need not be its subject) and every object an output or a reported
// evaluation names — a trade study's alternatives.
func (v *verifyContext) runRoots(subject *runtime.Instance, verdicts []runtime.VerificationVerdict, outputs []runtime.CalcOutputValue, evaluations []runtime.AnalysisEvaluation) []*runtime.Instance {
	roots := []*runtime.Instance{subject}
	for _, verdict := range verdicts {
		if verdict.Subject != nil {
			roots = append(roots, verdict.Subject)
		}
	}
	for _, out := range outputs {
		roots = append(roots, v.namedInstances(out.Value)...)
	}
	for _, e := range evaluations {
		for _, arg := range e.Arguments {
			roots = append(roots, v.namedInstances(arg)...)
		}
		roots = append(roots, v.namedInstances(e.Result)...)
	}
	return roots
}

// caseEvaluations spells for the wire each application the run made of one of
// the case's calcs as a function value, in the order made.
func (v *verifyContext) caseEvaluations(evaluations []runtime.AnalysisEvaluation) []*pb.CaseEvaluation {
	if len(evaluations) == 0 {
		return nil
	}
	out := make([]*pb.CaseEvaluation, 0, len(evaluations))
	for _, e := range evaluations {
		pe := &pb.CaseEvaluation{FunctionId: e.Function, Selected: e.Selected, Tied: e.Tied}
		for _, arg := range e.Arguments {
			pe.Arguments = append(pe.Arguments, v.service.valueToProto(v.runtime, arg, v.cached.Index))
		}
		if e.Error != nil {
			pe.Error = e.Error.Error()
		} else {
			pe.Result = v.service.valueToProto(v.runtime, e.Result, v.cached.Index)
		}
		out = append(out, pe)
	}
	return out
}

// namedInstances are the objects a value's wire form refers to by identity: the
// instance it is or a variant materialized, the object a function was read off,
// and those named within a sequence's, a set's or an array's elements.
func (v *verifyContext) namedInstances(val runtime.Value) []*runtime.Instance {
	if id, ok := val.Object(); ok {
		if inst, ok := v.runtime.Instance(id); ok {
			return []*runtime.Instance{inst}
		}
		return nil
	}
	if val.Kind == runtime.ValFunction {
		if self := val.FunctionSelf(); self != nil {
			return []*runtime.Instance{self}
		}
		return nil
	}
	var elements []runtime.Value
	switch val.Kind {
	case runtime.ValSequence:
		if val.Sequence() != nil {
			_, elements = v.runtime.MadeElements(val)
		}
	case runtime.ValSet:
		if set := val.Set(); set != nil {
			elements = set.Elements()
		}
	case runtime.ValArray:
		if arr := val.Array(); arr != nil {
			elements = arr.Elements
		}
	}
	var out []*runtime.Instance
	for _, elem := range elements {
		out = append(out, v.namedInstances(elem)...)
	}
	return out
}

// instanceGraphs is the instance graph of every root, in root order, each
// object reported once; a nil root contributes nothing.
func (v *verifyContext) instanceGraphs(roots []*runtime.Instance) []*pb.Instance {
	var all []*pb.Instance
	seen := make(map[int64]bool)
	for _, root := range roots {
		if root == nil || seen[root.ID] {
			continue
		}
		for _, inst := range v.instanceGraph(root) {
			if !seen[inst.Id] {
				seen[inst.Id] = true
				all = append(all, inst)
			}
		}
	}
	return all
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
		wire := &pb.VerificationVerdict{
			CaseId:        verdict.Case,
			Kind:          string(verdict.Kind),
			Detail:        verdict.Detail,
			Subcase:       verdict.Subcase,
			RequirementId: requirementID,
		}
		if verdict.Subject != nil {
			wire.InstanceId = verdict.Subject.ID
			wire.InstanceTypeId = namedFQN(v.cached.Index, verdict.Subject.Type)
		}
		out = append(out, wire)
	}
	return out
}

// analysisArgument reads one argument off the wire against the model's index,
// so a quantity keeps the units it is commensurable with. One the service
// cannot read answers the request; one needing a capability it lacks fails the call.
func (v *verifyContext) analysisArgument(arg *pb.Value) (runtime.Value, *pb.RunAnalysisResponse, error) {
	val, readErr, err := v.argument(arg)
	if err != nil {
		return runtime.Value{}, nil, err
	}
	if readErr != nil {
		return runtime.Value{}, &pb.RunAnalysisResponse{
			Error:         fmt.Sprintf("analysis argument could not be read: %v", readErr),
			FailureReason: failureReason(readErr),
		}, nil
	}
	return val, nil, nil
}

// argument reads one argument value. A value the service cannot serve is the
// second error (a status); one that does not read as a value is the first.
func (v *verifyContext) argument(arg *pb.Value) (runtime.Value, error, error) {
	if err := v.service.requireValueCapabilities(arg); err != nil {
		return runtime.Value{}, nil, err
	}
	val, err := protoconv.ProtoToRuntimeValue(v.runtime, arg, v.cached.Index, v.sem())
	if err != nil {
		return runtime.Value{}, err, nil
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
