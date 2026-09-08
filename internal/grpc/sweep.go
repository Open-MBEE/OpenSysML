package grpc

import (
	"context"
	"fmt"
	"sort"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// sweepResultName names a calc's returned value in a row, so a calc row and an
// analysis case's outputs read alike.
const sweepResultName = "result"

// RunSweep runs one analysis case or calc once per row of a parameter sweep, as
// the CLI's -sweep and the REPL's %sweep do: every row is an ordinary run with
// the swept parameters bound to that row's values. A run that failed is that
// row's error; only a plan that no run follows from fails the table.
func (s *Service) RunSweep(ctx context.Context, req *pb.RunSweepRequest) (*pb.RunSweepResponse, error) {
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
		return &pb.RunSweepResponse{Error: err.Error()}, nil
	}
	// A sweep reports a row's outputs and objective verdicts, which carry no
	// verification verdict, so a verification case runs through RunVerification.
	if runtime.IsVerificationCaseSymbol(sym) {
		return sweepFailure(fmt.Errorf("%w: %s is a verification case, which a sweep does not run; run it with RunAnalysis",
			runtime.ErrNotAnAnalysis, req.SymbolId)), nil
	}
	analysis := runtime.IsRunnableCaseSymbol(sym)
	if !analysis {
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
	subject, err := v.subject(req.SubjectSymbolId)
	if err != nil {
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
	if err := v.runtime.CheckSweepParameters(sym, plan, len(positional), names); err != nil {
		return sweepFailure(err), nil
	}

	scope := v.declaringScope(sym)
	run := func(bindings []runtime.SweepBinding) (runtime.SweepRunResult, error) {
		bound := make(map[string]runtime.Value, len(named)+len(bindings))
		for name, value := range named {
			bound[name] = value
		}
		for _, b := range bindings {
			bound[b.Param] = b.Value
		}
		if !analysis {
			value, err := v.runtime.InvokeCalcWith(sym, positional, bound, scope)
			if err != nil {
				return runtime.SweepRunResult{}, err
			}
			return runtime.SweepRunResult{
				Outputs: []runtime.CalcOutputValue{{Name: sweepResultName, Value: value}},
			}, nil
		}
		args := runtime.AnalysisArgs{Subject: subject, Positional: positional, Named: bound}
		result, err := v.runtime.RunAnalysis(sym, args, scope, nil)
		if err != nil {
			return runtime.SweepRunResult{}, err
		}
		// The case reports the subject it ran on: the one supplied, or the one
		// the usage or the enclosing case bound.
		return runtime.SweepRunResult{
			Outputs:  result.Outputs,
			Verdicts: result.Verdicts,
			Subject:  result.Subject,
		}, nil
	}

	table, err := v.runtime.RunSweep(ctx, req.SymbolId, plan, run)
	if err != nil {
		// A caller that went away is the call failing, not a table reporting it.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return sweepFailure(err), nil
	}
	return v.sweepResponse(table), nil
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

// sweepValue reads one value off the wire against the model's index, so a
// quantity keeps the units it is commensurable with.
func (v *verifyContext) sweepValue(val *pb.Value, what string) (runtime.Value, *pb.RunSweepResponse, error) {
	if err := v.service.requireValueCapabilities(val); err != nil {
		return runtime.Value{}, nil, err
	}
	out, err := ProtoToValueIn(val, v.cached.Index, v.sem)
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

// sweepResponse spells a table on the wire: one row per run, in the order the
// runs were made.
func (v *verifyContext) sweepResponse(table runtime.SweepTable) *pb.RunSweepResponse {
	resp := &pb.RunSweepResponse{
		Parameters: table.Params,
		Sampled:    table.Sampled,
		Seed:       table.Seed,
		Rows:       make([]*pb.SweepRow, 0, len(table.Rows)),
	}
	seen := make(map[int64]bool)
	for i := range table.Rows {
		row := &table.Rows[i]
		out := &pb.SweepRow{ElapsedMicros: row.Elapsed.Microseconds()}
		for _, binding := range row.Bindings {
			out.Inputs = append(out.Inputs, &pb.CalcOutput{
				Name:  binding.Param,
				Value: v.service.valueToProto(v.runtime, binding.Value, v.cached.Index),
			})
		}
		for _, output := range row.Outputs {
			out.Outputs = append(out.Outputs, &pb.CalcOutput{
				Name:  output.Name,
				Value: v.service.valueToProto(v.runtime, output.Value, v.cached.Index),
			})
		}
		for j := range row.Verdicts {
			out.Verdicts = append(out.Verdicts, v.analysisVerdict(&row.Verdicts[j], row.Subject))
		}
		resp.Instances = appendInstances(resp.Instances, seen, v.instanceGraph(row.Subject))
		if row.Err != nil {
			out.Error = row.Err.Error()
			out.FailureReason = failureReason(row.Err)
		}
		resp.Rows = append(resp.Rows, out)
	}
	return resp
}

// appendInstances adds the objects of one run to the table's, each once, so
// every row's verdict resolves against the same graph.
func appendInstances(all []*pb.Instance, seen map[int64]bool, graph []*pb.Instance) []*pb.Instance {
	for _, inst := range graph {
		if seen[inst.Id] {
			continue
		}
		seen[inst.Id] = true
		all = append(all, inst)
	}
	return all
}
