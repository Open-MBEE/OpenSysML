package grpc

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/sweep"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// monteCarloObservable names the column a Monte Carlo case's runs report: the
// value the case declares it observes.
const monteCarloObservable = "observed"

// RunRuns runs an action, or a Simulation::MonteCarlo analysis case, the
// requested number of times, as the CLI's -runs/-observe and the REPL's %runs
// do: every run is an ordinary run in a context of its own, drawing its modeled
// randomness from a seed derived from the request's, and the table reports
// what each run observed with each observable's distribution. A run that
// failed is that row's error; only a request no run follows from fails the table.
func (s *Service) RunRuns(ctx context.Context, req *pb.RunRunsRequest) (*pb.RunRunsResponse, error) {
	if err := s.requireCapability(CapabilityRuns); err != nil {
		return nil, err
	}
	if (req.ActionSymbolId == "") == (req.AnalysisSymbolId == "") {
		return nil, statusErrorf(connect.CodeInvalidArgument, "exactly one of action_symbol_id and analysis_symbol_id names what to run")
	}
	if req.Runs <= 0 {
		return nil, statusErrorf(connect.CodeInvalidArgument, "runs must be positive, got %d", req.Runs)
	}
	draws := runtime.DrawRandom
	if req.Draws != "" {
		policy, err := runtime.ParseDrawPolicy(req.Draws)
		if err != nil {
			return nil, statusErrorf(connect.CodeInvalidArgument, "%v", err)
		}
		draws = policy
	}
	if req.Seed == nil && !draws.Fixed() {
		return nil, statusErrorf(connect.CodeInvalidArgument, "the runs draw at random; set seed, or fix the draws as min, max or average")
	}
	if req.ClockStep < 0 || math.IsNaN(req.ClockStep) || math.IsInf(req.ClockStep, 0) {
		return nil, statusErrorf(connect.CodeInvalidArgument, "clock_step must be a finite, non-negative number, got %v", req.ClockStep)
	}
	if req.AnalysisSymbolId != "" && len(req.Observe) > 0 {
		return nil, statusErrorf(connect.CodeInvalidArgument, "a Monte Carlo case observes what it declares; observe names an action's features")
	}
	seen := make(map[string]bool, len(req.Observe))
	for _, name := range req.Observe {
		if seen[name] {
			return nil, statusErrorf(connect.CodeInvalidArgument, "observe names %s twice; each feature is observed once", name)
		}
		seen[name] = true
	}
	if req.ActionSymbolId != "" && (len(req.Arguments) > 0 || len(req.NamedArguments) > 0 || req.SubjectSymbolId != "") {
		return nil, statusErrorf(connect.CodeInvalidArgument, "an action takes inputs and a performer; arguments, named_arguments and subject_symbol_id are a case's")
	}
	if req.AnalysisSymbolId != "" && (len(req.Inputs) > 0 || req.PerformerSymbolId != "") {
		return nil, statusErrorf(connect.CodeInvalidArgument, "a case takes arguments and a subject; inputs and performer_symbol_id are an action's")
	}
	if req.PerformerSymbolId != "" {
		if err := s.requireCapability(CapabilityPerformer); err != nil {
			return nil, err
		}
	}
	for _, pv := range req.Inputs {
		if err := s.requireValueCapabilities(pv); err != nil {
			return nil, err
		}
	}

	v, err := s.newVerifyContext(ctx, req.ModelHash, req.Engine)
	if err != nil {
		return nil, err
	}
	defer v.release()
	v.runtime.SetDrawPolicy(draws)
	if req.ClockStep > 0 {
		if err := v.runtime.SetClockStep(req.ClockStep); err != nil {
			return nil, statusErrorf(connect.CodeInvalidArgument, "%v", err)
		}
	}

	plan, seedRow := s.runsPlan(req)
	if req.ActionSymbolId != "" {
		return s.runAction(ctx, v, req, plan, seedRow)
	}
	return s.runMonteCarloCase(ctx, v, req, plan, seedRow)
}

// runsPlan is the plan of the request's runs and how each seeds its own context.
func (s *Service) runsPlan(req *pb.RunRunsRequest) (sweep.SweepPlan, seedRowFunc) {
	plan := sweep.SeedlessMonteCarloPlan(req.Runs)
	if req.Seed != nil {
		plan = sweep.MonteCarloPlan(req.Runs, *req.Seed)
	}
	seedRow := func(rt *runtime.Context, bindings []sweep.SweepBinding) (int64, error) {
		i, ok := sweep.RunNumber(bindings)
		if !ok {
			return 0, fmt.Errorf("%w: the row numbers no run", sweep.ErrSweepRuns)
		}
		if req.Seed != nil {
			rt.SetModelSeed(runtime.RunSeed(*req.Seed, i))
		} else {
			rt.ClearModelSeed()
		}
		return i, nil
	}

	return plan, seedRow
}

// seedRowFunc seeds a run's own context from the request's seed and names the run.
type seedRowFunc func(rt *runtime.Context, bindings []sweep.SweepBinding) (int64, error)

// runAction makes the runs of an action and tables the observables they produced.
func (s *Service) runAction(ctx context.Context, v *verifyContext, req *pb.RunRunsRequest, plan sweep.SweepPlan, seedRow seedRowFunc) (*pb.RunRunsResponse, error) {
	// An inherited action is named by the subtype too (Sub::inherited).
	syms := v.cached.LookupNamedMember(req.ActionSymbolId)
	if len(syms) == 0 {
		return &pb.RunRunsResponse{Error: fmt.Sprintf("action not found: %s", req.ActionSymbolId),
			FailureReason: pb.FailureReason_FAILURE_REASON_UNSPECIFIED}, nil
	}
	action := syms[0]
	readInputs := func(rt *runtime.Context) (map[string]runtime.Value, error) {
		inputs := make(map[string]runtime.Value, len(req.Inputs))
		for name, pv := range req.Inputs {
			val, err := protoconv.ProtoToRuntimeValue(rt, pv, v.cached.Index, rt.Semantics())
			if err != nil {
				return nil, fmt.Errorf("input %q could not be read: %w", name, err)
			}
			inputs[name] = val
		}
		return inputs, nil
	}
	// The inputs and the performer are read once on the request's runtime, which
	// answers one that cannot be read, and again on each run's own context.
	if _, err := readInputs(v.runtime); err != nil {
		return &pb.RunRunsResponse{Error: err.Error(), FailureReason: pb.FailureReason_FAILURE_REASON_EVALUATION}, nil
	}
	if _, err := v.performer(req.PerformerSymbolId); err != nil {
		return &pb.RunRunsResponse{Error: err.Error(), FailureReason: failureReason(err)}, nil
	}
	run := func(rt *runtime.Context, bindings []sweep.SweepBinding) (sweep.SweepRunResult, error) {
		if _, err := seedRow(rt, bindings); err != nil {
			return sweep.SweepRunResult{}, err
		}
		inputs, err := readInputs(rt)
		if err != nil {
			return sweep.SweepRunResult{}, err
		}
		self, err := s.performer(v.cached, rt, req.PerformerSymbolId)
		if err != nil {
			return sweep.SweepRunResult{}, err
		}
		outcome, err := rt.ActionOutcomePerformedBy(action, self, inputs)
		if err != nil {
			return sweep.SweepRunResult{}, fmt.Errorf("action execution failed: %w", err)
		}
		return sweep.SweepRunResult{Outputs: analysis.Observe(rt, outcome, req.Observe)}, nil
	}
	answered, err := s.sweepRuns(ctx, v, req.ActionSymbolId, plan, run)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return runsFailure(err, s.standingOf(answered)), nil
	}
	table := answered.Result.Table()
	resp := v.runsResponse(table, s.standingOf(answered))
	if err := analysis.Unobserved(table, req.Observe); err != nil {
		resp.Error = err.Error()
		resp.FailureReason = failureReason(err)
	}
	return resp, nil
}

// runMonteCarloCase makes the runs of a Simulation::MonteCarlo case and concludes
// it over the sample, settling every row's verdicts by the conclusion.
// monteCarloSample is what a Monte Carlo case's runs made, for recording them.
type monteCarloSample struct {
	table       sweep.SweepTable
	completed   []*runtime.MonteCarloRun
	concluded   runtime.AnalysisResult
	answered    analysis.Plan
	concludeErr error
}

func (s *Service) runMonteCarloCase(ctx context.Context, v *verifyContext, req *pb.RunRunsRequest, plan sweep.SweepPlan, seedRow seedRowFunc) (*pb.RunRunsResponse, error) {
	resp, _, err := s.monteCarloRuns(ctx, v, req, plan, seedRow)
	return resp, err
}

// monteCarloRuns runs the case and answers the runs' report beside the sample
// they made, nil when none ran.
func (s *Service) monteCarloRuns(ctx context.Context, v *verifyContext, req *pb.RunRunsRequest, plan sweep.SweepPlan, seedRow seedRowFunc) (*pb.RunRunsResponse, *monteCarloSample, error) {
	sym, err := v.lookup(req.AnalysisSymbolId)
	if err != nil {
		return &pb.RunRunsResponse{Error: err.Error(), FailureReason: pb.FailureReason_FAILURE_REASON_UNSPECIFIED}, nil, nil
	}
	if err := v.runtime.RequireMonteCarloCase(sym); err != nil {
		return runsFailure(err, standing{}), nil, nil
	}
	// The case's arguments and subject read as a sweep's, so a case the two RPCs
	// both run reads them the same way.
	args := &pb.RunSweepRequest{SubjectSymbolId: req.SubjectSymbolId, Arguments: req.Arguments, NamedArguments: req.NamedArguments}
	if _, err := v.subject(req.SubjectSymbolId); err != nil {
		return &pb.RunRunsResponse{Error: err.Error(), FailureReason: failureReason(err)}, nil, nil
	}
	if _, _, resp, err := v.sweepArguments(args); err != nil {
		return nil, nil, err
	} else if resp != nil {
		return &pb.RunRunsResponse{Error: resp.Error, FailureReason: resp.FailureReason}, nil, nil
	}
	scope := v.declaringScope(sym)

	var mu sync.Mutex
	made := make(map[int64]*runtime.MonteCarloRun)
	run := func(rt *runtime.Context, bindings []sweep.SweepBinding) (sweep.SweepRunResult, error) {
		i, err := seedRow(rt, bindings)
		if err != nil {
			return sweep.SweepRunResult{}, err
		}
		row := v.on(rt)
		subject, err := row.subject(req.SubjectSymbolId)
		if err != nil {
			return sweep.SweepRunResult{}, err
		}
		positional, named, resp, err := row.sweepArguments(args)
		if err != nil {
			return sweep.SweepRunResult{}, err
		}
		if resp != nil {
			return sweep.SweepRunResult{}, errors.New(resp.Error)
		}
		observed, err := rt.ObserveMonteCarlo(sym, runtime.AnalysisArgs{Subject: subject, Positional: positional, Named: named}, scope, nil)
		if err != nil {
			return sweep.SweepRunResult{}, err
		}
		observed.Number = i
		mu.Lock()
		made[i] = observed
		mu.Unlock()
		return sweep.SweepRunResult{
			Outputs:  []runtime.CalcOutputValue{{Name: monteCarloObservable, Value: observed.Observed}},
			Verdicts: observed.Verdicts,
			Subject:  observed.Subject,
		}, nil
	}
	answered, err := s.sweepRuns(ctx, v, req.AnalysisSymbolId, plan, run)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return runsFailure(err, s.standingOf(answered)), nil, nil
	}
	table := answered.Result.Table()
	completed := make([]*runtime.MonteCarloRun, 0, len(made))
	for _, row := range table.Rows {
		if i, ok := sweep.RunNumber(row.Bindings); ok && made[i] != nil {
			completed = append(completed, made[i])
		}
	}
	st := s.standingOf(answered)
	name := sym.Name
	if len(completed) == 0 {
		resp := v.runsResponse(table, st)
		resp.Unconcluded = "no run completed, so the case is not concluded"
		resp.Lines = append(resp.Lines, fmt.Sprintf("? %s: %s", name, resp.Unconcluded))
		return resp, &monteCarloSample{table: table, answered: answered}, nil
	}
	stats, err := runtime.MonteCarloSample(completed)
	var concluded runtime.AnalysisResult
	var sample []bool
	if err == nil {
		sample, err = runtime.SettleMonteCarloRuns(completed, &stats)
	}
	last := completed[len(completed)-1]
	if err == nil {
		concluded, err = last.Conclude(stats, sample)
	}
	byNumber := make(map[int64]*runtime.MonteCarloRun, len(completed))
	for _, run := range completed {
		byNumber[run.Number] = run
	}
	for k, row := range table.Rows {
		if i, ok := sweep.RunNumber(row.Bindings); ok && byNumber[i] != nil {
			table.Rows[k].Verdicts = byNumber[i].Verdicts
		}
	}
	resp, ids := v.runsResponseIDs(table, st)
	in := v.on(last.Context())
	// The conclusion is drawn in the last run's context, so its objects are the
	// last row's: renumber it as that row was, and add only the objects it adds.
	tail := &pb.SweepRow{}
	for _, out := range concluded.Outputs {
		tail.Outputs = append(tail.Outputs, &pb.CalcOutput{Name: out.Name, Value: s.valueToProto(in.runtime, out.Value, v.cached.Index)})
	}
	mark := "✓"
	for j := range concluded.Verdicts {
		switch concluded.Verdicts[j].Status {
		case runtime.VerdictNotSatisfied:
			mark = "✗"
		case runtime.VerdictUndecided:
			if mark == "✓" {
				mark = "?"
			}
		}
		tail.Verdicts = append(tail.Verdicts, st.stamp(in.analysisVerdict(&concluded.Verdicts[j], concluded.Subject)))
	}
	conclusion := ids.lastRow()
	conclusion.row(tail, nil)
	resp.Outputs = append(resp.Outputs, tail.Outputs...)
	resp.Verdicts = append(resp.Verdicts, tail.Verdicts...)
	recorded := &monteCarloSample{table: table, completed: completed, concluded: concluded, answered: answered, concludeErr: err}
	if err != nil {
		resp.Unconcluded = err.Error()
		resp.Lines = append(resp.Lines, fmt.Sprintf("? %s: %s", name, err.Error()))
		return resp, recorded, nil
	}
	resp.Lines = append(resp.Lines, fmt.Sprintf("%s %s over %d run(s)", mark, name, stats.Runs))
	roots := in.instanceGraphs(in.runRoots(concluded.Subject, nil, concluded.Outputs, nil))
	conclusion = ids.lastRow()
	conclusion.row(&pb.SweepRow{}, roots)
	known := make(map[int64]bool, len(resp.Instances))
	for _, inst := range resp.Instances {
		known[inst.Id] = true
	}
	for _, inst := range roots {
		if !known[inst.Id] {
			known[inst.Id] = true
			resp.Instances = append(resp.Instances, inst)
		}
	}
	return resp, recorded, nil
}

// sweepRuns puts the runs to the engines as a sweep over the Monte Carlo plan,
// the draws and clock step the request set on the runtime going with them.
func (s *Service) sweepRuns(ctx context.Context, v *verifyContext, target string, plan sweep.SweepPlan, run sweep.SweepRun) (analysis.Plan, error) {
	schedule := v.runtime.Schedule()
	return s.engines.Sweep(ctx, analysis.Request{
		Model:     s.model(v.cached),
		Subject:   target,
		Schedule:  schedule,
		Draws:     analysis.DrawsOf(v.runtime),
		ClockStep: analysis.ClockStepOf(v.runtime),
		Budget:    analysis.BudgetOf(s.budgets, schedule, analysis.Sweep, s.jobs),
		Selection: v.engine,
	}, plan, run)
}

// runsFailure is the response of runs no run was made for, under the standing
// of whatever engine refused them.
func runsFailure(err error, st standing) *pb.RunRunsResponse {
	return &pb.RunRunsResponse{Error: err.Error(), FailureReason: failureReason(err),
		Engine: st.engine, Strength: st.strength, Bounds: st.bounds}
}

// runsResponse spells the table on the wire as a sweep's rows, each observable's
// distribution after them, with the distribution lines the CLI prints.
func (v *verifyContext) runsResponse(table sweep.SweepTable, st standing) *pb.RunRunsResponse {
	resp, _ := v.runsResponseIDs(table, st)
	return resp
}

// runsResponseIDs is runsResponse with the renumbering its rows ended on.
func (v *verifyContext) runsResponseIDs(table sweep.SweepTable, st standing) (*pb.RunRunsResponse, rowIDs) {
	swept, ids := v.sweepResponseIDs(table, st)
	resp := &pb.RunRunsResponse{
		Rows:        swept.Rows,
		Instances:   swept.Instances,
		Observables: analysis.OutputColumns(table),
		Engine:      st.engine,
		Strength:    st.strength,
		Bounds:      st.bounds,
	}
	if !table.Seedless {
		seed := table.Seed
		resp.Seed = &seed
	}
	for _, name := range resp.Observables {
		summary := analysis.Summarize(table, name, objectText)
		resp.Distributions = append(resp.Distributions, v.distribution(summary))
		resp.Lines = append(resp.Lines, summary.Lines()...)
	}
	return resp, ids
}

// objectText spells a value that is not a number: an object by the usage it
// occurs as and its id, anything else as the runtime formats it.
func objectText(ctx *runtime.Context, val runtime.Value) string {
	if val.Kind == runtime.ValInstance {
		if inst, ok := ctx.Instance(val.Instance); ok {
			if usage := ctx.OccurrenceUsage(inst); usage != "" {
				return fmt.Sprintf("%s (object #%d)", usage, inst.ID)
			}
		}
	}
	return runtime.FormatValue(val)
}

// distribution spells one observable's summary on the wire.
func (v *verifyContext) distribution(summary analysis.ObservableSummary) *pb.RunDistribution {
	out := &pb.RunDistribution{Name: summary.Name, Unit: summary.Unit}
	number := func(x semantics.Value) *pb.Value {
		return v.service.valueToProto(v.runtime, runtime.ConstValue(x), v.cached.Index)
	}
	if d := summary.Distribution; d != nil {
		out.Numeric = true
		out.Count = int64(d.Count)
		out.Min = number(d.Min)
		out.Max = number(d.Max)
		out.Mean = number(analysis.DrawnMean(d))
		out.Deviation = d.Deviation
		out.Percentiles = []*pb.RunPercentile{
			{Percentile: 50, Value: number(d.P50)},
			{Percentile: 90, Value: number(d.P90)},
		}
		for _, bin := range d.Histogram {
			out.Histogram = append(out.Histogram, &pb.RunHistogramBin{Low: number(bin.Lo), High: number(bin.Hi), Count: int64(bin.Count)})
		}
	}
	for _, c := range summary.Counted {
		out.Count += int64(c.Count)
		out.Values = append(out.Values, &pb.RunCountedValue{Value: &pb.Value{Kind: &pb.Value_StringValue{StringValue: c.Text}}, Count: int64(c.Count)})
	}
	return out
}
