package grpc

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/objref"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// Verdict kinds, as reported in Verdict.kind.
const (
	verdictConstraint  = "constraint"
	verdictRequirement = "requirement"
	verdictSatisfy     = "satisfy"
	verdictBinding     = "binding"
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
		errors.Is(err, runtime.ErrNotAnObject),
		errors.Is(err, runtime.ErrNotACalc),
		errors.Is(err, runtime.ErrNotAnAnalysis):
		return pb.FailureReason_FAILURE_REASON_WRONG_KIND
	default:
		return pb.FailureReason_FAILURE_REASON_EVALUATION
	}
}

// verifyContext is everything a verification RPC needs from a cached model: the
// runtime it evaluates in, on a worker of the request's own, and the index its names
// resolve against.
type verifyContext struct {
	service *Service
	cached  *CachedModel
	runtime *runtime.Context
	// engine is the selection the request's questions are put to the engines under.
	engine analysis.Selection
	// release gives the runtime's worker back once the request is answered; the RPC defers it.
	release func()
	// held is the population the request evaluates over, nil on a worker of the request's own.
	held *heldObjects
}

// newVerifyContext reads the request's engine, looks the model up and builds a
// runtime over it, the same way every other runtime RPC in this service does.
func (s *Service) newVerifyContext(ctx context.Context, modelHash, engine string) (*verifyContext, error) {
	selection, err := s.engineSelection(engine)
	if err != nil {
		return nil, err
	}
	cached, ok := s.cache.Get(modelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, "model not found: %s", modelHash)
	}
	rt, release := s.newRuntime(ctx, cached)
	return &verifyContext{service: s, cached: cached, runtime: rt, engine: selection, release: release}, nil
}

// on is this request's context over a run's own runtime, so what the run made is
// read through the context that made it.
func (v *verifyContext) on(rt *runtime.Context) *verifyContext {
	return &verifyContext{service: v.service, cached: v.cached, runtime: rt, engine: v.engine, release: releaseNothing}
}

// releaseNothing stands in for release on a context over a run's own runtime,
// which borrowed no worker and so has nothing to give back.
func releaseNothing() {
	// The parent context releases the worker it borrowed; this one holds none.
}

// overHeld is this request's context over the objects Instantiate holds for the model,
// under the population's lock, which the returned function releases: a request naming no
// subject is about those objects, as a REPL check is about the objects the session holds.
// A request naming its subject, or a model holding no object, keeps the request's own
// worker and releases nothing.
func (v *verifyContext) overHeld(ctx context.Context, subject *runtime.Instance) (*verifyContext, func()) {
	if subject != nil {
		return v, releaseNothing
	}
	held, ok := v.service.heldObjects(v.cached)
	if !ok {
		return v, releaseNothing
	}
	unlock := held.lock(ctx)
	if held.empty() {
		unlock()
		return v, releaseNothing
	}
	return v.onHeld(held), unlock
}

// onHeld is this request's context over a held population's runtime, which the
// caller holds the lock of; its failures at the population's bound are statuses.
func (v *verifyContext) onHeld(held *heldObjects) *verifyContext {
	on := v.on(held.rt)
	on.held = held
	return on
}

// exhausted is the RESOURCE_EXHAUSTED status of an evaluation over the held population
// that would pass its bound; nil for any other failure, and on a worker of the request's own.
func (v *verifyContext) exhausted(err error) error {
	if v.held == nil {
		return nil
	}
	return v.held.exhausted(err)
}

// sem is the semantic model the request's runtime evaluates against.
func (v *verifyContext) sem() *semantics.Model { return v.runtime.Semantics() }

// lookup resolves an FQN to the symbol it names.
func (v *verifyContext) lookup(symbolID string) (*symbols.Symbol, error) {
	syms := symbolfacts.LookupNamed(v.cached.Index, symbolID)
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
// verdict is then about declared defaults. A path (`Mission::mission.vehicle`) names a nested one.
func (v *verifyContext) subject(symbolID string) (*runtime.Instance, error) {
	return v.named("subject", symbolID)
}

// performer is the object a request's action or machine is performed by, named
// as a subject is; empty is no performer.
func (v *verifyContext) performer(symbolID string) (*runtime.Instance, error) {
	return v.named("performer", symbolID)
}

// named is the object symbolID names in the role given: an object of the declaration it
// names, or the one a declaration-rooted path reaches, spelled with `.` or `::` as the CLI
// reads it — `Wire::pair::craft` walks into pair's object as `Wire::pair.craft` does.
func (v *verifyContext) named(role, symbolID string) (*runtime.Instance, error) {
	if symbolID == "" {
		return nil, nil
	}
	if ref, err := objref.Parse(symbolID); objref.LooksLikePath(symbolID) || err == nil && len(ref.Segments) > 1 {
		return v.objectAt(role, symbolID)
	}
	sym, err := v.lookup(symbolID)
	if err != nil {
		return nil, err
	}
	return v.instantiate(role, symbolID, sym)
}

// objectAt is the object a declaration-rooted path names: the longest leading run of
// segments naming a declaration is instantiated, the rest walked from it; an id names none.
// The index holds declarations alone, so a member reached through a usage's type
// (`pair::craft`) names no root and is walked inside the usage's object.
func (v *verifyContext) objectAt(role, path string) (*runtime.Instance, error) {
	ref, err := objref.Parse(path)
	if err != nil {
		return nil, err
	}
	if ref.ID > 0 {
		return nil, fmt.Errorf("%s %s names an object by id, which a call creates none of: name a declaration, or a path from one such as mission.vehicle", role, path)
	}
	for i := objref.Head(ref.Segments); i > 0; i-- {
		name := objref.JoinTyped(ref.Segments[:i])
		syms := symbolfacts.LookupNamed(v.cached.Index, name)
		if len(syms) == 0 {
			continue
		}
		if i > 1 && len(symbolfacts.LookupNamed(v.cached.Index, objref.DeclaredRun(ref.Segments[:i]))) == 0 {
			continue
		}
		if objref.IsNamespace(syms[0]) {
			break
		}
		root, err := v.instantiate(role, name, syms[0])
		if err != nil {
			return nil, err
		}
		walker := objref.Walker{Runtime: v.runtime, Index: v.cached.Index}
		inst, _, err := walker.Walk(root, name, ref.Segments[i:])
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", role, path, err)
		}
		return inst, nil
	}
	return nil, fmt.Errorf("symbol not found: %s", objref.JoinTyped(ref.Segments[:max(objref.Head(ref.Segments), 1)]))
}

// object instantiates the symbol a request named to validate as a whole, which
// must have objects: a package or an attribute is ErrNotAnObject, a wrong kind.
func (v *verifyContext) object(symbolID string) (*runtime.Instance, error) {
	sym, err := v.lookup(symbolID)
	if err != nil {
		return nil, err
	}
	if err := runtime.RequireObject(sym); err != nil {
		return nil, err
	}
	return v.instantiate("subject", symbolID, sym)
}

// instantiate materializes an object of sym, named in a failure by its role and
// as the request spelt it.
func (v *verifyContext) instantiate(role, symbolID string, sym *symbols.Symbol) (*runtime.Instance, error) {
	inst, err := v.runtime.Instantiate(sym)
	if err != nil {
		return nil, fmt.Errorf("instantiation of %s %s failed: %w", role, symbolID, err)
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
		s = s.OwnerScope.OwningElement()
	}
	return idx.GetFQN(sym)
}

// verdict builds the answer to one verification. A condition that evaluated to
// false is the model's answer, reported as holds=false with the condition named;
// any other error is a failure to evaluate, reported in Verdict.error. The plan
// is how the engines answered, whose standing the verdict carries.
func (v *verifyContext) verdict(kind string, sym *symbols.Symbol, element string, inst *runtime.Instance, holds bool, err error, plan analysis.Plan) *pb.Verdict {
	out := v.service.standingOf(plan).stamp(&pb.Verdict{
		Kind:    kind,
		Element: element,
		Holds:   holds && err == nil,
	})
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
	return stampEvaluation(out)
}

// stampEvaluation marks out as the answer to an evaluate question: undecided when
// it carries an error, else holds or violated.
func stampEvaluation(out *pb.Verdict) *pb.Verdict {
	out.Question = questionEvaluate
	switch {
	case out.Error != "":
		out.Status = statusUndecided
	case out.Holds:
		out.Status = statusHolds
	default:
		out.Status = statusViolated
	}
	return out
}

// instanceGraph serializes the object a verdict is about, so a client can read
// the feature values behind a failure without a follow-up call.
func (v *verifyContext) instanceGraph(inst *runtime.Instance) []*pb.Instance {
	if inst == nil {
		return nil
	}
	return v.service.instanceGraphToProto(v.runtime, inst, v.cached.Index).All
}

// VerifyConstraint evaluates a constraint definition or usage, as the REPL's
// %constraint does.
func (s *Service) VerifyConstraint(ctx context.Context, req *pb.VerifyConstraintRequest) (*pb.VerifyConstraintResponse, error) {
	if err := s.requireCapability(CapabilityVerification); err != nil {
		return nil, err
	}
	question, err := s.verificationQuestion(req.Question)
	if err != nil {
		return nil, err
	}
	v, err := s.newVerifyContext(ctx, req.ModelHash, req.Engine)
	if err != nil {
		return nil, err
	}
	defer v.release()
	sym, err := v.lookup(req.SymbolId)
	if err != nil {
		return &pb.VerifyConstraintResponse{Error: err.Error()}, nil
	}
	inst, err := v.subject(req.SubjectSymbolId)
	if err != nil {
		return &pb.VerifyConstraintResponse{Error: err.Error()}, nil
	}
	v, unlock := v.overHeld(ctx, inst)
	defer unlock()
	args, readErr, err := v.checkArgs(question, req.Arguments, req.NamedArguments)
	if err != nil {
		return nil, err
	}
	if readErr != nil {
		return &pb.VerifyConstraintResponse{Error: readErr.Error()}, nil
	}
	if question != questionEvaluate {
		return v.proveConstraint(ctx, question, sym, inst)
	}

	result, plan, evalErr := v.check(ctx, req.SymbolId, func(rt *runtime.Context) (runtime.CheckResult, error) {
		return rt.CheckConstraintWith(sym, v.declaringScope(sym), inst, args)
	})
	if err := callerGone(ctx, evalErr); err != nil {
		return nil, err
	}
	if status := v.exhausted(evalErr); status != nil {
		return nil, status
	}
	subject := subjectOf(result, inst)
	return &pb.VerifyConstraintResponse{
		Verdict:   v.verdict(verdictConstraint, sym, "", subject, result.Holds, evalErr, plan),
		Instances: v.instanceGraph(subject),
	}, nil
}

// checkArgs reads a verification request's argument bindings. Naming any needs
// the verification_arguments capability; a proof question takes none, the
// parameters' values being the solver's to find. The first error is a value
// that does not read, spelled for the response; the second is a status.
func (v *verifyContext) checkArgs(question string, positional []*pb.Value, named map[string]*pb.Value) (runtime.CheckArgs, error, error) {
	if len(positional) == 0 && len(named) == 0 {
		return runtime.CheckArgs{}, nil, nil
	}
	if err := v.service.requireCapability(CapabilityVerificationArguments); err != nil {
		return runtime.CheckArgs{}, nil, err
	}
	if question != questionEvaluate {
		return runtime.CheckArgs{}, nil, statusErrorf(connect.CodeInvalidArgument,
			"question %q takes no arguments: the solver finds the parameters' values", question)
	}
	var args runtime.CheckArgs
	for _, arg := range positional {
		val, readErr, err := v.argument(arg)
		if readErr != nil || err != nil {
			return runtime.CheckArgs{}, wrapArgumentRead(readErr), err
		}
		args.Positional = append(args.Positional, val)
	}
	if len(named) > 0 {
		// Read in name order so a failure names the same argument every time.
		names := make([]string, 0, len(named))
		for name := range named {
			names = append(names, name)
		}
		sort.Strings(names)
		args.Named = make(map[string]runtime.Value, len(names))
		for _, name := range names {
			val, readErr, err := v.argument(named[name])
			if readErr != nil || err != nil {
				return runtime.CheckArgs{}, wrapArgumentRead(readErr), err
			}
			args.Named[name] = val
		}
	}
	return args, nil, nil
}

func wrapArgumentRead(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("argument could not be read: %w", err)
}

// VerifyRequirement evaluates a requirement definition or usage, as the REPL's
// %requirement does.
func (s *Service) VerifyRequirement(ctx context.Context, req *pb.VerifyRequirementRequest) (*pb.VerifyRequirementResponse, error) {
	if err := s.requireCapability(CapabilityVerification); err != nil {
		return nil, err
	}
	question, err := s.verificationQuestion(req.Question)
	if err != nil {
		return nil, err
	}
	v, err := s.newVerifyContext(ctx, req.ModelHash, req.Engine)
	if err != nil {
		return nil, err
	}
	defer v.release()
	sym, err := v.lookup(req.SymbolId)
	if err != nil {
		return &pb.VerifyRequirementResponse{Error: err.Error()}, nil
	}
	inst, err := v.subject(req.SubjectSymbolId)
	if err != nil {
		return &pb.VerifyRequirementResponse{Error: err.Error()}, nil
	}
	v, unlock := v.overHeld(ctx, inst)
	defer unlock()
	args, readErr, err := v.checkArgs(question, req.Arguments, req.NamedArguments)
	if err != nil {
		return nil, err
	}
	if readErr != nil {
		return &pb.VerifyRequirementResponse{Error: readErr.Error()}, nil
	}
	if question != questionEvaluate {
		return v.proveRequirement(ctx, question, sym, inst)
	}

	result, plan, evalErr := v.check(ctx, req.SymbolId, func(rt *runtime.Context) (runtime.CheckResult, error) {
		return rt.CheckRequirementWith(sym, v.declaringScope(sym), inst, args)
	})
	if err := callerGone(ctx, evalErr); err != nil {
		return nil, err
	}
	if status := v.exhausted(evalErr); status != nil {
		return nil, status
	}
	subject := subjectOf(result, inst)
	// Beside the requirement's verdict: what the cases verifying it answered when
	// their bodies ran, a type-owned one on each object of the type the model holds.
	verifications, err := v.requirementVerificationRuns(sym, subject, nil)
	if err != nil {
		return nil, err
	}
	return &pb.VerifyRequirementResponse{
		Verdict: v.verdict(verdictRequirement, sym, "", subject, result.Holds, evalErr, plan),
		// The subject's graph, and that of every object a case ran on.
		Instances:            v.instanceGraphs(v.runRoots(subject, verifications, nil, nil)),
		VerificationVerdicts: v.verificationVerdicts(verifications, namedFQN(v.cached.Index, sym)),
	}, nil
}

// VerifySatisfaction evaluates the satisfaction assertions a model states, as the
// REPL's %satisfy does: every one, or, given a symbol, the ones that element
// states — or that element itself when it is a named satisfy assertion.
func (s *Service) VerifySatisfaction(ctx context.Context, req *pb.VerifySatisfactionRequest) (*pb.VerifySatisfactionResponse, error) {
	if err := s.requireCapability(CapabilityVerification); err != nil {
		return nil, err
	}
	question, err := s.verificationQuestion(req.Question)
	if err != nil {
		return nil, err
	}
	v, err := s.newVerifyContext(ctx, req.ModelHash, req.Engine)
	if err != nil {
		return nil, err
	}
	defer v.release()

	// Every document of the model states assertions, unless one scope is named.
	scopes := v.cached.DocumentRoots()
	if req.SymbolId != "" {
		sym, lerr := v.lookup(req.SymbolId)
		if lerr != nil {
			return &pb.VerifySatisfactionResponse{Error: lerr.Error()}, nil
		}
		// A named `satisfy requirement r by p` is itself one assertion.
		if a, aerr := v.runtime.SatisfyAssertionOf(sym); aerr == nil {
			if question != questionEvaluate {
				verdict, instances, err := v.symbolicSatisfy(ctx, question, a)
				if err != nil {
					return nil, err
				}
				return &pb.VerifySatisfactionResponse{
					Verdicts:  []*pb.Verdict{verdict},
					Instances: instances,
				}, nil
			}
			verdicts, instances, verifications, err := v.satisfyVerdicts(ctx, a, &runtime.VerificationRuns{})
			if err != nil {
				return nil, err
			}
			return &pb.VerifySatisfactionResponse{
				Verdicts:             verdicts,
				Instances:            instances,
				VerificationVerdicts: verifications,
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
	// A case verifying a requirement runs once for the response, a type-owned
	// one once per object checked, however many assertions report it.
	runs := &runtime.VerificationRuns{}
	for _, scope := range scopes {
		for _, a := range v.runtime.SatisfyAssertionsIn(scope) {
			var verdicts []*pb.Verdict
			var instances []*pb.Instance
			var verifications []*pb.VerificationVerdict
			var err error
			if question != questionEvaluate {
				var verdict *pb.Verdict
				verdict, instances, err = v.symbolicSatisfy(ctx, question, a)
				verdicts = []*pb.Verdict{verdict}
			} else {
				verdicts, instances, verifications, err = v.satisfyVerdicts(ctx, a, runs)
			}
			if err != nil {
				return nil, err
			}
			resp.Verdicts = append(resp.Verdicts, verdicts...)
			resp.VerificationVerdicts = append(resp.VerificationVerdicts, verifications...)
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

// satisfyVerdicts evaluates one assertion: against an object of its subject, or,
// for an assertion a type states with no `by`, against each object of that type
// the model holds (the population Instantiate built), one verdict per object as
// ValidateInstance decides it. A definition's assertion with no such object is
// undecided, saying an object is needed; a usage's names the object it is about.
// The verification cases verifying the requirement run on the same objects, once
// each over runs, and their verdicts follow.
func (v *verifyContext) satisfyVerdicts(ctx context.Context, a *runtime.SatisfyAssertion, runs *runtime.VerificationRuns) ([]*pb.Verdict, []*pb.Instance, []*pb.VerificationVerdict, error) {
	one := func(verdict *pb.Verdict, instances []*pb.Instance, subject *runtime.Instance, err error) ([]*pb.Verdict, []*pb.Instance, []*pb.VerificationVerdict, error) {
		if err != nil {
			return nil, nil, nil, err
		}
		verifications, err := v.assertionVerifications(a, subject, runs)
		if err != nil {
			return nil, nil, nil, err
		}
		return []*pb.Verdict{verdict}, instances, verifications, nil
	}
	if a.StatingType() == nil {
		return one(v.satisfyVerdict(ctx, a))
	}
	held, ok := v.service.heldObjects(v.cached)
	if !ok {
		return one(v.satisfyVerdict(ctx, a))
	}
	defer held.lock(ctx)()
	subjects := held.rt.SatisfactionSubjects(a, held.rootObjects())
	if len(subjects) == 0 {
		return one(v.satisfyVerdict(ctx, a))
	}
	on := v.onHeld(held)
	var verdicts []*pb.Verdict
	var instances []*pb.Instance
	var verifications []*pb.VerificationVerdict
	for _, held := range subjects {
		inst := held.Object
		result, plan, err := on.check(ctx, a.Text(), func(rt *runtime.Context) (runtime.CheckResult, error) {
			return rt.CheckSatisfactionOn(a, inst)
		})
		if gone := callerGone(ctx, err); gone != nil {
			return nil, nil, nil, gone
		}
		subject := subjectOf(result, inst)
		verdict := on.verdict(verdictSatisfy, a.Symbol, a.Text(), subject, result.Holds, err, plan)
		verdict.InstancePath = instancePath(held.Path)
		on.associateRequirement(verdict, a)
		verdicts = append(verdicts, verdict)
		instances = append(instances, on.instanceGraph(subject)...)
		verified, err := on.assertionVerifications(a, subject, runs)
		if err != nil {
			return nil, nil, nil, err
		}
		verifications = append(verifications, verified...)
	}
	return verdicts, instances, verifications, nil
}

// satisfyVerdict evaluates one assertion against an object of its subject, built
// for this call so the verdict is about the values that subject holds. The error
// is the caller's own, having gone away before the check.
func (v *verifyContext) satisfyVerdict(ctx context.Context, a *runtime.SatisfyAssertion) (*pb.Verdict, []*pb.Instance, *runtime.Instance, error) {
	var subject *runtime.Instance
	if a.Subject != nil || a.StatingType() != nil {
		// Created here rather than inside the evaluation so that the object the
		// verdict is about can be reported with it.
		inst, err := v.runtime.SatisfySubject(a)
		if err != nil {
			// The assertion cannot be evaluated without the object it is about,
			// which is a failure to evaluate rather than a verdict of false.
			verdict := v.verdict(verdictSatisfy, a.Symbol, a.Text(), nil, false, err, analysis.Plan{})
			v.associateRequirement(verdict, a)
			return verdict, nil, nil, nil
		}
		subject = inst
	}
	result, plan, err := v.check(ctx, a.Text(), func(rt *runtime.Context) (runtime.CheckResult, error) {
		return rt.CheckSatisfactionOn(a, subject)
	})
	if gone := callerGone(ctx, err); gone != nil {
		return nil, nil, nil, gone
	}
	subject = subjectOf(result, subject)
	verdict := v.verdict(verdictSatisfy, a.Symbol, a.Text(), subject, result.Holds, err, plan)
	v.associateRequirement(verdict, a)
	return verdict, v.instanceGraph(subject), subject, nil
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
	v, err := s.newVerifyContext(ctx, req.ModelHash, req.Engine)
	if err != nil {
		return nil, err
	}
	defer v.release()
	sym, err := v.lookup(req.SymbolId)
	if err != nil {
		return &pb.EvaluateCalcResponse{Error: err.Error()}, nil
	}

	if len(req.Arguments) == 0 {
		outputs, handled, plan, cerr := v.calcUsageOutputs(ctx, req.SymbolId, sym)
		if gone := callerGone(ctx, cerr); gone != nil {
			return nil, gone
		}
		if cerr != nil {
			return calcResponse(&pb.EvaluateCalcResponse{
				Error:         cerr.Error(),
				FailureReason: failureReason(cerr),
			}, v.service.standingOf(plan)), nil
		}
		if handled {
			return calcResponse(&pb.EvaluateCalcResponse{Outputs: outputs}, v.service.standingOf(plan)), nil
		}
	}

	// Converted against the model's index, so a quantity argument keeps the base
	// units it is commensurable with instead of arriving as an unusable value.
	args := make([]runtime.Value, 0, len(req.Arguments))
	for _, arg := range req.Arguments {
		if err := s.requireValueCapabilities(arg); err != nil {
			return nil, err
		}
		val, cerr := protoconv.ProtoToRuntimeValue(v.runtime, arg, v.cached.Index, v.sem())
		if cerr != nil {
			return &pb.EvaluateCalcResponse{
				Error:         fmt.Sprintf("calc argument could not be read: %v", cerr),
				FailureReason: failureReason(cerr),
			}, nil
		}
		args = append(args, val)
	}

	result, plan, err := perform(ctx, v, req.SymbolId, func(rt *runtime.Context) (runtime.Value, error) {
		return rt.InvokeCalc(sym, args, v.declaringScope(sym))
	}, func(result runtime.Value, err error) analysis.Answer {
		return analysis.ValuesAnswer([]analysis.Evaluation{{Name: sweepResultName, Value: result}}, err)
	})
	if gone := callerGone(ctx, err); gone != nil {
		return nil, gone
	}
	st := v.service.standingOf(plan)
	if err != nil {
		return calcResponse(&pb.EvaluateCalcResponse{
			Error:         fmt.Sprintf("calc invocation failed: %v", err),
			FailureReason: failureReason(err),
		}, st), nil
	}
	return calcResponse(&pb.EvaluateCalcResponse{Result: v.service.valueToProto(v.runtime, result, v.cached.Index)}, st), nil
}

// calcResponse writes on a calc's response the standing of the plan that answered it.
func calcResponse(resp *pb.EvaluateCalcResponse, st standing) *pb.EvaluateCalcResponse {
	resp.Engine, resp.Strength, resp.Bounds = st.engine, st.strength, st.bounds
	return resp
}

// calcUsageOutputs evaluates a calc usage from its own member values. It reports
// handled=false when sym is not a calc usage, or is one computing no output
// features, so those are invoked as calculations instead.
func (v *verifyContext) calcUsageOutputs(ctx context.Context, subject string, sym *symbols.Symbol) ([]*pb.CalcOutput, bool, analysis.Plan, error) {
	usage, ok := sym.Decl.(*ast.Usage)
	if !ok || usage.Kind != ast.UsageCalc {
		return nil, false, analysis.Plan{}, nil
	}
	outputs, plan, err := perform(ctx, v, subject, func(rt *runtime.Context) ([]runtime.CalcOutputValue, error) {
		return rt.CalcUsageOutputs(sym, sym.OwnerScope, nil)
	}, func(outputs []runtime.CalcOutputValue, err error) analysis.Answer {
		return analysis.ValuesAnswer(analysis.OutputValues(outputs), err)
	})
	if err != nil {
		return nil, true, plan, fmt.Errorf("calc usage evaluation failed: %w", err)
	}
	if len(outputs) == 0 {
		return nil, false, plan, nil
	}
	pbOutputs := make([]*pb.CalcOutput, 0, len(outputs))
	for _, out := range outputs {
		pbOutputs = append(pbOutputs, &pb.CalcOutput{
			Name:  out.Name,
			Value: v.service.valueToProto(v.runtime, out.Value, v.cached.Index),
		})
	}
	return pbOutputs, true, plan, nil
}

// requirementVerificationRuns runs the whole model's verification cases verifying req (the
// case need not share the requirement's document), type-owned ones about subject, and
// reports their body verdicts; a run over the held population that would pass its bound
// is the RESOURCE_EXHAUSTED status instead.
func (v *verifyContext) requirementVerificationRuns(req *symbols.Symbol, subject *runtime.Instance, runs *runtime.VerificationRuns) ([]runtime.VerificationVerdict, error) {
	verdicts := v.runtime.VerificationVerdictsOnce(v.cached.DocumentRoots(), req, subject, runs)
	for _, verdict := range verdicts {
		if status := v.exhausted(verdict.Err); status != nil {
			return nil, status
		}
	}
	return verdicts, nil
}

// requirementVerifications are requirementVerificationRuns' verdicts spelled for the wire.
func (v *verifyContext) requirementVerifications(req *symbols.Symbol, subject *runtime.Instance, runs *runtime.VerificationRuns) ([]*pb.VerificationVerdict, error) {
	verdicts, err := v.requirementVerificationRuns(req, subject, runs)
	if err != nil {
		return nil, err
	}
	return v.verificationVerdicts(verdicts, namedFQN(v.cached.Index, req)), nil
}

// assertionVerifications are the body verdicts of the verification cases whose
// objective verifies the requirement an assertion satisfies, run about subject.
func (v *verifyContext) assertionVerifications(a *runtime.SatisfyAssertion, subject *runtime.Instance, runs *runtime.VerificationRuns) ([]*pb.VerificationVerdict, error) {
	req := a.AssertedRequirement()
	if req == nil {
		return nil, nil
	}
	return v.requirementVerifications(req, subject, runs)
}
