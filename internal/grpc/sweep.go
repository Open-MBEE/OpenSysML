package grpc

import (
	"context"
	"errors"
	"fmt"
	"sort"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// sweepResultName names a calc's returned value in a row, so a calc row and an
// analysis case's outputs read alike.
const sweepResultName = "result"

// RunSweep runs one analysis case or calc once per row of a parameter sweep, as
// the CLI's -sweep and the REPL's %sweep do: every row is an ordinary run, in a
// context of its own, with the swept parameters bound to that row's values. A
// run that failed is that row's error; only a plan that no run follows from
// fails the table. The request's subject and arguments are read once on the
// request's runtime, which answers one that cannot be read, and again in every
// row's, which is what the row's run binds.
func (s *Service) RunSweep(ctx context.Context, req *pb.RunSweepRequest) (*pb.RunSweepResponse, error) {
	if err := s.requireCapability(CapabilityVerification); err != nil {
		return nil, err
	}
	v, err := s.newVerifyContext(req.ModelHash, req.Engine)
	if err != nil {
		return nil, err
	}
	sym, err := v.lookup(req.SymbolId)
	if err != nil {
		return &pb.RunSweepResponse{Error: err.Error()}, nil
	}
	// A sweep reports a row's outputs and objective verdicts, which carry no
	// verification verdict, so a verification case runs through RunVerification.
	if runtime.IsVerificationCaseSymbol(sym) {
		return sweepFailure(fmt.Errorf("%w: %s is a verification case, which a sweep does not run; run it with RunAnalysis",
			runtime.ErrNotAnAnalysis, req.SymbolId)), nil
	}
	isCase := runtime.IsRunnableCaseSymbol(sym)
	if !isCase {
		switch sym.Kind {
		case symbols.SymbolCalcDef, symbols.SymbolCalcUsage:
		default:
			return sweepFailure(fmt.Errorf("%w: %s declares neither an analysis case nor a calc",
				runtime.ErrNotACalc, req.SymbolId)), nil
		}
		if req.SubjectSymbolId != "" {
			return sweepFailure(fmt.Errorf("%s is a calc, which has no subject", req.SymbolId)), nil
		}
	}
	if _, err := v.subject(req.SubjectSymbolId); err != nil {
		return &pb.RunSweepResponse{Error: err.Error()}, nil
	}

	positional, named, resp, err := v.sweepArguments(req)
	if resp != nil || err != nil {
		return resp, err
	}
	plan, resp, err := v.sweepPlan(req)
	if resp != nil || err != nil {
		return resp, err
	}
	names := make([]string, 0, len(named))
	for name := range named {
		names = append(names, name)
	}
	sort.Strings(names)
	plan, err = v.runtime.ResolveSweepPlan(sym, plan, len(positional), names)
	if err != nil {
		return sweepFailure(err), nil
	}

	scope := v.declaringScope(sym)
	run := func(rt *runtime.Context, bindings []runtime.SweepBinding) (runtime.SweepRunResult, error) {
		row := v.on(rt)
		subject, err := row.subject(req.SubjectSymbolId)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		positional, named, resp, err := row.sweepArguments(req)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		if resp != nil {
			return runtime.SweepRunResult{}, errors.New(resp.Error)
		}
		bound := make(map[string]runtime.Value, len(named)+len(bindings))
		for name, value := range named {
			bound[name] = value
		}
		for _, b := range bindings {
			bound[b.Param] = b.Value
		}
		if !isCase {
			value, err := rt.InvokeCalcWith(sym, positional, bound, scope)
			if err != nil {
				return runtime.SweepRunResult{}, err
			}
			return runtime.SweepRunResult{
				Outputs: []runtime.CalcOutputValue{{Name: sweepResultName, Value: value}},
			}, nil
		}
		args := runtime.AnalysisArgs{Subject: subject, Positional: positional, Named: bound}
		result, err := rt.RunAnalysis(sym, args, scope, nil)
		// The case reports the subject it ran on: the one supplied, or the one
		// the usage or the enclosing case bound.
		return runtime.SweepRunResult{
			Outputs:     result.Outputs,
			Verdicts:    result.Verdicts,
			Subject:     result.Subject,
			Evaluations: result.Evaluations,
		}, err
	}

	schedule := v.runtime.Schedule()
	answered, err := s.engines.Sweep(ctx, s.model(v.cached), req.SymbolId, schedule, plan, run, analysis.BudgetOf(s.budgets, schedule, analysis.Sweep, s.jobs), v.engine)
	if err != nil {
		// A caller that went away is the call failing, not a table reporting it.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		failed := sweepFailure(err)
		st := s.standingOf(answered)
		failed.Engine, failed.Strength, failed.Bounds = st.engine, st.strength, st.bounds
		return failed, nil
	}
	return v.sweepResponse(answered.Result.Table(), s.standingOf(answered)), nil
}

// sweepArguments reads the arguments every row of the sweep binds, the named
// ones in name order so a failure names the same argument every time.
func (v *verifyContext) sweepArguments(req *pb.RunSweepRequest) ([]runtime.Value, map[string]runtime.Value, *pb.RunSweepResponse, error) {
	positional := make([]runtime.Value, 0, len(req.Arguments))
	for _, arg := range req.Arguments {
		val, resp, err := v.sweepValue(arg, "argument")
		if resp != nil || err != nil {
			return nil, nil, resp, err
		}
		positional = append(positional, val)
	}
	names := make([]string, 0, len(req.NamedArguments))
	for name := range req.NamedArguments {
		names = append(names, name)
	}
	sort.Strings(names)
	named := make(map[string]runtime.Value, len(names))
	for _, name := range names {
		val, resp, err := v.sweepValue(req.NamedArguments[name], "argument "+name)
		if resp != nil || err != nil {
			return nil, nil, resp, err
		}
		named[name] = val
	}
	return positional, named, nil, nil
}

// sweepPlan reads the ranges and, where the request draws rather than steps,
// the number of draws and the seed they are drawn from.
func (v *verifyContext) sweepPlan(req *pb.RunSweepRequest) (runtime.SweepPlan, *pb.RunSweepResponse, error) {
	plan := runtime.SweepPlan{
		Ranges:  make([]runtime.SweepRange, 0, len(req.Ranges)),
		Sampled: req.Samples > 0,
		Samples: req.Samples,
		Seed:    req.Seed,
	}
	if req.Samples < 0 {
		return plan, sweepFailure(fmt.Errorf("%w: draw at least one sample, got %d",
			runtime.ErrSweepSamples, req.Samples)), nil
	}
	for _, r := range req.Ranges {
		if r.Start == nil || r.End == nil {
			return plan, sweepFailure(fmt.Errorf("%w: range %s states no %s endpoint",
				runtime.ErrSweepRange, r.Parameter, endpointName(r))), nil
		}
		out := runtime.SweepRange{Param: r.Parameter, HasStep: r.Step != nil}
		from, resp, err := v.sweepValue(r.Start, "start of range "+r.Parameter)
		if resp != nil || err != nil {
			return plan, resp, err
		}
		to, resp, err := v.sweepValue(r.End, "end of range "+r.Parameter)
		if resp != nil || err != nil {
			return plan, resp, err
		}
		out.From, out.To = from, to
		if out.HasStep {
			step, resp, err := v.sweepValue(r.Step, "step of range "+r.Parameter)
			if resp != nil || err != nil {
				return plan, resp, err
			}
			out.Step = step
		}
		plan.Ranges = append(plan.Ranges, out)
	}
	return plan, nil, nil
}

// endpointName names the endpoint a range left unstated.
func endpointName(r *pb.SweepRange) string {
	if r.Start == nil {
		return "start"
	}
	return "end"
}

// sweepValue reads one value off the wire against the model's index and the
// run's runtime, so a quantity keeps its units and a function binds its calc.
func (v *verifyContext) sweepValue(val *pb.Value, what string) (runtime.Value, *pb.RunSweepResponse, error) {
	if err := v.service.requireValueCapabilities(val); err != nil {
		return runtime.Value{}, nil, err
	}
	out, err := ProtoToRuntimeValue(v.runtime, val, v.cached.Index, v.sem())
	if err != nil {
		return runtime.Value{}, &pb.RunSweepResponse{
			Error:         fmt.Sprintf("sweep %s could not be read: %v", what, err),
			FailureReason: failureReason(err),
		}, nil
	}
	return out, nil, nil
}

// sweepFailure answers a request no run followed from.
func sweepFailure(err error) *pb.RunSweepResponse {
	return &pb.RunSweepResponse{Error: err.Error(), FailureReason: failureReason(err)}
}

// sweepResponse spells a table on the wire: one row per run, in plan order, under
// the standing of the plan that ran it. A row's values and objects are read through
// the row's own context, the one that produced them, and the contexts number their
// objects alike, so each row's are renumbered after the rows before it: the table
// then names every object once, and a row's references resolve to that row's objects.
func (v *verifyContext) sweepResponse(table runtime.SweepTable, st standing) *pb.RunSweepResponse {
	resp := &pb.RunSweepResponse{
		Parameters: table.Params,
		Sampled:    table.Sampled,
		Seed:       table.Seed,
		Rows:       make([]*pb.SweepRow, 0, len(table.Rows)),
		Engine:     st.engine,
		Strength:   st.strength,
		Bounds:     st.bounds,
	}
	var ids rowIDs
	evaluations := v.service.capabilities.has(CapabilityCaseEvaluations)
	for i := range table.Rows {
		row := &table.Rows[i]
		in := v.on(row.Context)
		out := &pb.SweepRow{ElapsedMicros: row.Elapsed.Microseconds()}
		for _, binding := range row.Bindings {
			out.Inputs = append(out.Inputs, &pb.CalcOutput{
				Name:  binding.Param,
				Value: in.service.valueToProto(in.runtime, binding.Value, in.cached.Index),
			})
		}
		if row.Err != nil {
			out.Error = row.Err.Error()
			out.FailureReason = failureReason(row.Err)
			// A client predating case_evaluations reads a failed row as its error alone.
			if !evaluations {
				resp.Rows = append(resp.Rows, out)
				continue
			}
		}
		for _, output := range row.Outputs {
			out.Outputs = append(out.Outputs, &pb.CalcOutput{
				Name:  output.Name,
				Value: in.service.valueToProto(in.runtime, output.Value, in.cached.Index),
			})
		}
		for j := range row.Verdicts {
			out.Verdicts = append(out.Verdicts, st.stamp(in.analysisVerdict(&row.Verdicts[j], row.Subject)))
		}
		var reported []runtime.AnalysisEvaluation
		if evaluations {
			reported = row.Evaluations
			out.Evaluations = in.caseEvaluations(reported)
		}
		graph := in.instanceGraphs(in.runRoots(row.Subject, row.Outputs, reported))
		ids.row(out, graph)
		resp.Instances = append(resp.Instances, graph...)
		resp.Rows = append(resp.Rows, out)
	}
	return resp
}

// rowIDs numbers a table's objects across its rows: each row's ids, which its own
// context counted from 1, are shifted past the greatest id a row before it took.
type rowIDs struct {
	shift, last int64
}

// row renumbers every object reference a row and its graph make, then moves the
// shift past them for the row after.
func (r *rowIDs) row(out *pb.SweepRow, graph []*pb.Instance) {
	for _, in := range out.Inputs {
		r.value(in.Value)
	}
	for _, output := range out.Outputs {
		r.value(output.Value)
	}
	for _, verdict := range out.Verdicts {
		r.id(&verdict.InstanceId)
	}
	for _, e := range out.Evaluations {
		for _, arg := range e.Arguments {
			r.value(arg)
		}
		r.value(e.Result)
	}
	for _, inst := range graph {
		r.id(&inst.Id)
		for _, fv := range inst.FeatureValues {
			r.value(fv.Value)
			for _, v := range fv.Values {
				r.value(v)
			}
		}
	}
	r.shift = r.last
}

// id shifts one reference; 0 names no object and stays so.
func (r *rowIDs) id(id *int64) {
	if *id == 0 {
		return
	}
	*id += r.shift
	r.last = max(r.last, *id)
}

// value shifts the references a value makes: an object, a function's object, and
// those of every element it holds.
func (r *rowIDs) value(v *pb.Value) {
	switch k := v.GetKind().(type) {
	case *pb.Value_InstanceId:
		r.id(&k.InstanceId)
	case *pb.Value_Function:
		if k.Function != nil {
			r.id(&k.Function.SelfId)
		}
	}
	for _, nested := range nestedValues(v) {
		r.value(nested)
	}
}
