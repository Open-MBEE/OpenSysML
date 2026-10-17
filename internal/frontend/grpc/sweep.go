package grpc

import (
	"context"
	"errors"
	"fmt"
	"sort"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/sweep"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
	v, err := s.newVerifyContext(ctx, req.ModelHash, req.Engine)
	if err != nil {
		return nil, err
	}
	defer v.release()
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

	table, answered, resp, err := s.sweepTable(ctx, v, sym, isCase, req)
	if resp != nil || err != nil {
		return resp, err
	}
	return v.sweepResponse(table, s.standingOf(answered)), nil
}

// sweepTable runs the sweep the request asks for and answers its table, or
// the response or call failure that stopped it before any row ran.
func (s *Service) sweepTable(ctx context.Context, v *verifyContext, sym *symbols.Symbol, isCase bool, req *pb.RunSweepRequest) (sweep.SweepTable, analysis.Plan, *pb.RunSweepResponse, error) {
	positional, named, resp, err := v.sweepArguments(req)
	if resp != nil || err != nil {
		return sweep.SweepTable{}, analysis.Plan{}, resp, err
	}
	plan, resp, err := v.sweepPlan(req)
	if resp != nil || err != nil {
		return sweep.SweepTable{}, analysis.Plan{}, resp, err
	}
	names := make([]string, 0, len(named))
	for name := range named {
		names = append(names, name)
	}
	sort.Strings(names)
	plan, err = sweep.ResolvePlan(v.runtime, sym, plan, len(positional), names)
	if err != nil {
		return sweep.SweepTable{}, analysis.Plan{}, sweepFailure(err), nil
	}

	scope := v.declaringScope(sym)
	run := func(rt *runtime.Context, bindings []sweep.SweepBinding) (sweep.SweepRunResult, error) {
		row := v.on(rt)
		subject, err := row.subject(req.SubjectSymbolId)
		if err != nil {
			return sweep.SweepRunResult{}, err
		}
		positional, named, resp, err := row.sweepArguments(req)
		if err != nil {
			return sweep.SweepRunResult{}, err
		}
		if resp != nil {
			return sweep.SweepRunResult{}, errors.New(resp.Error)
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
				return sweep.SweepRunResult{}, err
			}
			return sweep.SweepRunResult{
				Outputs: []runtime.CalcOutputValue{{Name: sweepResultName, Value: value}},
			}, nil
		}
		args := runtime.AnalysisArgs{Subject: subject, Positional: positional, Named: bound}
		result, err := rt.RunAnalysis(sym, args, scope, nil)
		// The case reports the subject it ran on: the one supplied, or the one
		// the usage or the enclosing case bound.
		return sweep.SweepRunResult{
			Outputs:     result.Outputs,
			Verdicts:    result.Verdicts,
			Subject:     result.Subject,
			Evaluations: result.Evaluations,
		}, err
	}

	schedule := v.runtime.Schedule()
	answered, err := s.engines.Sweep(ctx, analysis.Request{
		Model:     s.model(v.cached),
		Subject:   req.SymbolId,
		Schedule:  schedule,
		Budget:    analysis.BudgetOf(s.budgets, schedule, analysis.Sweep, s.jobs),
		Selection: v.engine,
	}, plan, run)
	if err != nil {
		// A caller that went away is the call failing, not a table reporting it.
		if ctx.Err() != nil {
			return sweep.SweepTable{}, answered, nil, ctx.Err()
		}
		failed := sweepFailure(err)
		st := s.standingOf(answered)
		failed.Engine, failed.Strength, failed.Bounds = st.engine, st.strength, st.bounds
		return sweep.SweepTable{}, answered, failed, nil
	}
	return answered.Result.Table(), answered, nil, nil
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
func (v *verifyContext) sweepPlan(req *pb.RunSweepRequest) (sweep.SweepPlan, *pb.RunSweepResponse, error) {
	plan := sweep.SweepPlan{
		Ranges:  make([]sweep.SweepRange, 0, len(req.Ranges)),
		Sampled: req.Samples > 0,
		Samples: req.Samples,
		Seed:    req.Seed,
	}
	if req.Samples < 0 {
		return plan, sweepFailure(fmt.Errorf("%w: draw at least one sample, got %d",
			sweep.ErrSweepSamples, req.Samples)), nil
	}
	for _, r := range req.Ranges {
		if r.Start == nil || r.End == nil {
			return plan, sweepFailure(fmt.Errorf("%w: range %s states no %s endpoint",
				sweep.ErrSweepRange, r.Parameter, endpointName(r))), nil
		}
		out := sweep.SweepRange{Param: r.Parameter, HasStep: r.Step != nil}
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
	out, err := protoconv.ProtoToRuntimeValue(v.runtime, val, v.cached.Index, v.sem())
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
func (v *verifyContext) sweepResponse(table sweep.SweepTable, st standing) *pb.RunSweepResponse {
	resp, _ := v.sweepResponseIDs(table, st)
	return resp
}

// sweepResponseIDs is sweepResponse with the renumbering its rows ended on.
func (v *verifyContext) sweepResponseIDs(table sweep.SweepTable, st standing) (*pb.RunSweepResponse, rowIDs) {
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
		graph := in.instanceGraphs(in.runRoots(row.Subject, nil, row.Outputs, reported))
		ids.row(out, graph)
		resp.Instances = append(resp.Instances, graph...)
		resp.Rows = append(resp.Rows, out)
	}
	return resp, ids
}

// rowIDs numbers a table's objects across its rows: each row's ids, which its own
// context counted from 1, are shifted past the greatest id a row before it took.
type rowIDs struct {
	shift, last int64
	// rowShift is the shift the latest row was renumbered by, for what shares its objects.
	rowShift int64
}

// lastRow renumbers as the latest row was, so a conclusion drawn in that row's
// context names the row's objects rather than fresh ones.
func (r rowIDs) lastRow() rowIDs {
	return rowIDs{shift: r.rowShift, last: r.last}
}

// row renumbers every object reference a row and its graph make, then moves the
// shift past them for the row after.
func (r *rowIDs) row(out *pb.SweepRow, graph []*pb.Instance) {
	r.rowShift = r.shift
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
	for _, nested := range protoconv.NestedValues(v) {
		r.value(nested)
	}
}
