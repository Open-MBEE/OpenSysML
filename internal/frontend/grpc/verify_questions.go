package grpc

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/solve"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// The questions a verification request asks, as Verdict.question reports them;
// the empty field is an evaluation, unchanged.
const (
	questionEvaluate    = "evaluate"
	questionHolds       = "holds"
	questionSatisfiable = "satisfiable"
)

// The statuses a Verdict reports: the answer to the question asked, undecided
// when an engine or a translation decided nothing.
const (
	statusHolds         = "holds"
	statusViolated      = "violated"
	statusUndecided     = "undecided"
	statusSatisfiable   = "satisfiable"
	statusUnsatisfiable = "unsatisfiable"
)

// verificationQuestion reads a request's question field: unset is evaluate, the
// two solver questions need the verification_questions capability, anything
// else is the caller's error.
func (s *Service) verificationQuestion(spelling string) (string, error) {
	switch spelling {
	case "", questionEvaluate:
		return questionEvaluate, nil
	case questionHolds, questionSatisfiable:
		if err := s.requireCapability(CapabilityVerificationQuestions); err != nil {
			return "", err
		}
		return spelling, nil
	default:
		return "", statusErrorf(connect.CodeInvalidArgument, "unknown verification question %q: evaluate, holds or satisfiable", spelling)
	}
}

// owningElement is the element a condition was written in, whose declared
// values a query about that condition reads.
func owningElement(sym *symbols.Symbol) *symbols.Symbol {
	if sym == nil || sym.OwnerScope == nil {
		return nil
	}
	return sym.OwnerScope.Owner()
}

// symbolicVerdict answers a holds or satisfiable question about one element:
// the queries given are put to the engines and the plan's claim mapped to the
// verdict's status, a witnessed sat naming the violating or satisfying
// assignment. A translation refusal is an undecided verdict, asked of no
// engine.
// verdictSubject is what a symbolic verdict judges: the verdict's kind, the
// symbol and element text it is about, and the object it is asked of.
type verdictSubject struct {
	kind    string
	sym     *symbols.Symbol
	element string
	inst    *runtime.Instance
}

func (v *verifyContext) symbolicVerdict(ctx context.Context, question string, subject verdictSubject, queries []*solve.Query, terr error) (*pb.Verdict, error) {
	verdict := v.verdict(subject.kind, subject.sym, subject.element, subject.inst, false, nil, analysis.Plan{})
	verdict.Question, verdict.Status = question, statusUndecided
	verdict.FailureReason = pb.FailureReason_FAILURE_REASON_UNDECIDED
	if terr != nil {
		verdict.Error = terr.Error()
		return verdict, nil
	}
	plan, err := v.askSolvers(ctx, question, queries)
	v.service.standingOf(plan).stamp(verdict)
	if err != nil {
		verdict.Error = err.Error()
		return verdict, nil
	}
	switch plan.Result.Claim {
	case analysis.ClaimHolds, analysis.ClaimUnsatisfiable:
		// A proof over the assignments that read no unreadable feature is the
		// evaluator's verdict only where no assignment the question is about
		// reads one; a witness is one assignment, which the guards keep clear.
		if err := v.unreached(ctx, queries); err != nil {
			verdict.Error = err.Error()
			break
		}
		if plan.Result.Claim == analysis.ClaimHolds {
			verdict.Holds, verdict.Status = true, statusHolds
		} else {
			verdict.Status = statusUnsatisfiable
		}
	case analysis.ClaimViolated:
		verdict.Status = statusViolated
		verdict.Witness = v.witnessOf(plan, queries)
	case analysis.ClaimSatisfiable:
		verdict.Holds, verdict.Status = true, statusSatisfiable
		verdict.Witness = v.witnessOf(plan, queries)
	default:
		verdict.Error = plan.Result.Reason
		if verdict.Error == "" {
			verdict.Error = "the engines decided nothing"
		}
	}
	if verdict.Status != statusUndecided {
		verdict.FailureReason = pb.FailureReason_FAILURE_REASON_UNSPECIFIED
	}
	return verdict, nil
}

// unreached is nil when no assignment the queries are about reads a feature
// whose value could not be read, and otherwise says which query reads which.
func (v *verifyContext) unreached(ctx context.Context, queries []*solve.Query) error {
	for _, q := range queries {
		reached := q.Reached()
		if reached == nil {
			continue
		}
		plan, err := v.askSolvers(ctx, questionSatisfiable, []*solve.Query{reached})
		if err != nil {
			return err
		}
		switch plan.Result.Claim {
		case analysis.ClaimUnsatisfiable:
		case analysis.ClaimSatisfiable:
			return q.ReachedError()
		default:
			reason := plan.Result.Reason
			if reason == "" {
				reason = "the engines decided nothing"
			}
			return fmt.Errorf("whether %s %s reads %s, whose value could not be read, was not decided: %s",
				q.Kind, q.Element, unreadableNames(q), reason)
		}
	}
	return nil
}

// unreadableNames lists the features a query guards the reads of.
func unreadableNames(q *solve.Query) string {
	names := make([]string, 0, len(q.Unreadable))
	for _, u := range q.Unreadable {
		names = append(names, u.Name)
	}
	return strings.Join(names, ", ")
}

// askSolvers puts the queries of a holds or satisfiable question to the
// engines under the request's selection; a refusal — no covering engine, an
// absent solver — is the error a verdict reports as undecided.
func (v *verifyContext) askSolvers(ctx context.Context, question string, queries []*solve.Query) (analysis.Plan, error) {
	kind := analysis.Holds
	if question == questionSatisfiable {
		kind = analysis.Satisfiable
	}
	schedule := v.runtime.Schedule()
	req := analysis.Request{
		Model:     analysis.Held(v.runtime),
		Schedule:  schedule,
		ModelSeed: analysis.ModelSeedOf(v.runtime),
		Draws:     analysis.DrawsOf(v.runtime),
		ClockStep: analysis.ClockStepOf(v.runtime),
		Budget:    analysis.BudgetOf(v.service.budgets, schedule, kind, v.service.jobs),
		Selection: v.engine,
	}
	if question == questionHolds {
		return v.service.engines.Prove(ctx, req, queries)
	}
	return v.service.engines.Solve(ctx, req, queries, (*solve.Solver).Solve)
}

// symbolicElement translates one constraint or requirement's conditions into
// the query the question asks — violation queries for holds — and puts it to
// the engines. The kind is checked as an evaluation's is, so a wrong symbol
// answers a wrong-kind verdict rather than an undecided one.
func (v *verifyContext) symbolicElement(ctx context.Context, question, kind string, sym *symbols.Symbol, inst *runtime.Instance, wrongKind func(*symbols.Symbol) error, translate func(*runtime.Context, []solve.Pin) (*solve.Query, error)) (*pb.Verdict, *runtime.Instance, error) {
	verdict := v.verdict(kind, sym, "", inst, false, nil, analysis.Plan{})
	verdict.Question, verdict.Status = question, statusUndecided
	if err := wrongKind(sym); err != nil {
		verdict.Error = err.Error()
		verdict.FailureReason = failureReason(err)
		return verdict, inst, nil
	}
	// The question is about the object carrying the element, resolved as
	// evaluation resolves it — a nested carrier, or an ambiguity an evaluation
	// reports the same way — never the supplied object read directly.
	resolved, err := v.runtime.ConditionCarrier(kind, sym.Name, sym, inst)
	if err != nil {
		verdict.Error = err.Error()
		verdict.FailureReason = failureReason(err)
		return verdict, inst, nil
	}
	var objectType *symbols.Symbol
	if resolved != nil {
		objectType = resolved.Type
	}
	pins, unfixed := solve.FixedFor(v.runtime, solve.Fixing{
		Element:    sym,
		Owner:      owningElement(sym),
		Object:     resolved,
		ObjectType: objectType,
	})
	q, terr := v.translatedQuestion(translate, pins, unfixed, solve.ObjectReader(v.runtime, resolved), resolved)
	var queries []*solve.Query
	if terr == nil {
		queries = []*solve.Query{q}
	}
	verdict, verr := v.symbolicVerdict(ctx, question, verdictSubject{kind: kind, sym: sym, inst: resolved}, queries, terr)
	return verdict, resolved, verr
}

// translatedQuestion guards or refuses the unreadable values a query reads, then
// reads the values its chains name as its conditions would and translates again
// with them fixed — values left free would answer about assignments the model
// does not hold. A chain whose declared value does not evaluate is guarded or
// refused as a direct read is, so it is never left free either.
func (v *verifyContext) translatedQuestion(translate func(*runtime.Context, []solve.Pin) (*solve.Query, error), pins []solve.Pin, unfixed []solve.Unfixed, read solve.Reader, resolved *runtime.Instance) (*solve.Query, error) {
	guarded := func(pins []solve.Pin, unfixed []solve.Unfixed) (*solve.Query, error) {
		q, err := translate(v.runtime, pins)
		if err != nil {
			return nil, err
		}
		if err := solve.UnfixedRead(v.runtime, q, unfixed); err != nil {
			return nil, err
		}
		return q, nil
	}
	q, err := guarded(pins, unfixed)
	if err != nil {
		return nil, err
	}
	chainPins, chainUnfixed := solve.ChainPins(v.runtime, q, read, resolved)
	if len(chainPins) == 0 {
		if err := solve.UnfixedRead(v.runtime, q, chainUnfixed); err != nil {
			return nil, err
		}
		return q, nil
	}
	return guarded(append(pins, chainPins...), append(unfixed, chainUnfixed...))
}

// proveConstraint answers a holds or satisfiable question about a constraint.
func (v *verifyContext) proveConstraint(ctx context.Context, question string, sym *symbols.Symbol, inst *runtime.Instance) (*pb.VerifyConstraintResponse, error) {
	verdict, resolved, err := v.symbolicElement(ctx, question, verdictConstraint, sym, inst, runtime.RequireConstraint,
		func(rt *runtime.Context, pins []solve.Pin) (*solve.Query, error) {
			if question == questionHolds {
				return solve.ConstraintViolation(rt, sym, v.declaringScope(sym), pins)
			}
			return solve.ConstraintWith(rt, sym, v.declaringScope(sym), pins)
		})
	if err != nil {
		return nil, err
	}
	return &pb.VerifyConstraintResponse{Verdict: verdict, Instances: v.instanceGraph(resolved)}, nil
}

// proveRequirement answers a holds or satisfiable question about a requirement.
func (v *verifyContext) proveRequirement(ctx context.Context, question string, sym *symbols.Symbol, inst *runtime.Instance) (*pb.VerifyRequirementResponse, error) {
	verdict, resolved, err := v.symbolicElement(ctx, question, verdictRequirement, sym, inst, runtime.RequireRequirement,
		func(rt *runtime.Context, pins []solve.Pin) (*solve.Query, error) {
			if question == questionHolds {
				return solve.RequirementViolation(rt, sym, v.declaringScope(sym), pins)
			}
			return solve.RequirementWith(rt, sym, v.declaringScope(sym), pins)
		})
	if err != nil {
		return nil, err
	}
	return &pb.VerifyRequirementResponse{Verdict: verdict, Instances: v.instanceGraph(resolved)}, nil
}

// symbolicSatisfy answers a holds or satisfiable question about one
// satisfaction assertion, against an object of its subject built as an
// evaluation's is.
func (v *verifyContext) symbolicSatisfy(ctx context.Context, question string, a *runtime.SatisfyAssertion) (*pb.Verdict, []*pb.Instance, error) {
	var subject *runtime.Instance
	if a.Subject != nil {
		inst, err := v.runtime.SatisfySubject(a)
		if err != nil {
			verdict := v.verdict(verdictSatisfy, a.Symbol, a.Text(), nil, false, err, analysis.Plan{})
			verdict.Question = question
			v.associateRequirement(verdict, a)
			return verdict, nil, nil
		}
		subject = inst
	}
	// The question is about the object carrying the requirement, resolved as
	// evaluation resolves it beyond the `by` object.
	carrying := a.Requirement
	if carrying == nil {
		carrying = a.Symbol
	}
	resolved, err := v.runtime.ConditionCarrier("satisfaction", a.Text(), carrying, subject)
	if err != nil {
		verdict := v.verdict(verdictSatisfy, a.Symbol, a.Text(), nil, false, err, analysis.Plan{})
		verdict.Question = question
		v.associateRequirement(verdict, a)
		return verdict, nil, nil
	}
	var objectType *symbols.Symbol
	if resolved != nil {
		objectType = resolved.Type
	}
	pins, unfixed := solve.FixedFor(v.runtime, solve.Fixing{
		Element:    a.Symbol,
		Owner:      owningElement(a.Symbol),
		Object:     resolved,
		ObjectType: objectType,
	})
	q, terr := v.translatedQuestion(func(rt *runtime.Context, pins []solve.Pin) (*solve.Query, error) {
		if question == questionHolds {
			return solve.SatisfactionViolation(rt, a, pins)
		}
		return solve.SatisfactionWith(rt, a, pins)
	}, pins, unfixed, solve.SatisfactionReader(v.runtime, a, resolved), resolved)
	var queries []*solve.Query
	if terr == nil {
		queries = []*solve.Query{q}
	}
	verdict, err := v.symbolicVerdict(ctx, question, verdictSubject{kind: verdictSatisfy, sym: a.Symbol, element: a.Text(), inst: resolved}, queries, terr)
	if err != nil {
		return nil, nil, err
	}
	v.associateRequirement(verdict, a)
	return verdict, v.instanceGraph(resolved), nil
}

// witnessOf is the assignment witnessing a sat answer: the query's free
// variables in query order, each with the value the evaluator replayed, its
// base units and the solver's exact spelling.
func (v *verifyContext) witnessOf(plan analysis.Plan, queries []*solve.Query) []*pb.WitnessAssignment {
	if len(queries) != 1 {
		return nil
	}
	var model []solve.Assignment
	for _, answered := range plan.Result.Values {
		if answered.Solved != nil && answered.Solved.Status == solve.StatusSat {
			model = answered.Solved.Model
			break
		}
	}
	if model == nil {
		return nil
	}
	byName := make(map[string]solve.Assignment, len(model))
	for _, a := range model {
		byName[a.Var.Name] = a
	}
	var out []*pb.WitnessAssignment
	for _, variable := range queries[0].Free() {
		a, ok := byName[variable.Name]
		if !ok {
			continue
		}
		value, err := solve.DecodeValue(a)
		if err != nil {
			continue
		}
		assignment := &pb.WitnessAssignment{
			Feature: variable.Name,
			Unit:    variable.Unit,
			Exact:   value.Render(variable),
		}
		if held, ok := v.witnessValue(value); ok {
			assignment.Value = v.service.valueToProto(v.runtime, held, v.cached.Index)
		}
		out = append(out, assignment)
	}
	return out
}

// witnessValue is the evaluator's value of one witnessed assignment, as the
// wire's Value encodes it; false where the decoded value names no runtime
// value — an enum literal no symbol of the index names.
func (v *verifyContext) witnessValue(value solve.ModelValue) (runtime.Value, bool) {
	switch value.Kind {
	case solve.SortBool:
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: value.Bool}}, true
	case solve.SortInt:
		if value.Number != nil && value.Number.IsInt() {
			return runtime.Value{Kind: runtime.ValConst, Const: semantics.BigIntValue(new(big.Int).Set(value.Number.Num()))}, true
		}
	case solve.SortReal:
		if f, _ := value.Number.Float64(); !math.IsInf(f, 0) {
			return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValReal, Real: f}}, true
		}
	case solve.SortString:
		return runtime.NewStringValue(value.Text), true
	case solve.SortDatatype:
		syms := symbolfacts.LookupNamed(v.cached.Index, value.Text)
		if len(syms) == 1 {
			return runtime.NewEnumLiteral(syms[0]), true
		}
	}
	return runtime.Value{}, false
}
