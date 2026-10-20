package grpc

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/simresults"
	"github.com/Open-MBEE/OpenSysML/internal/exec/sweep"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// errorLine prefixes the lines of a comparison that report a failure.
const errorLine = "error: "

// compareOptions shape the runs CompareResults makes, as the request asked.
type compareOptions struct {
	runs      int64
	seed      *uint64
	draws     *runtime.DrawPolicy
	clockStep *float64
	observe   []simresults.ObservablePair
}

// clockObservable names the clock column every run's outputs end with.
const clockObservable = "clock"

// CompareResults runs each configuration the sidecar indexes as its tool ran it
// and tables the tool's distribution of every observable beside the runs'.
func (s *Service) CompareResults(ctx context.Context, req *pb.CompareResultsRequest) (*pb.CompareResultsResponse, error) {
	if err := s.requireCapability(CapabilityCompareResults); err != nil {
		return nil, err
	}
	if req.Results == "" {
		return nil, statusError(connect.CodeInvalidArgument, "results is required: the JSON sysml -migration-results wrote")
	}
	results, err := simresults.Read(strings.NewReader(req.Results))
	if err != nil {
		return nil, statusErrorf(connect.CodeInvalidArgument, "results: %v", err)
	}
	if req.Runs < 0 {
		return nil, statusErrorf(connect.CodeInvalidArgument, "runs must not be negative, got %d", req.Runs)
	}
	opts := compareOptions{runs: req.Runs, seed: req.Seed}
	if req.Draws != "" {
		policy, err := runtime.ParseDrawPolicy(req.Draws)
		if err != nil {
			return nil, statusErrorf(connect.CodeInvalidArgument, "%v", err)
		}
		opts.draws = &policy
	}
	if req.ClockStep != nil {
		if err := runtime.CheckClockStep(*req.ClockStep); err != nil {
			return nil, statusErrorf(connect.CodeInvalidArgument, "clock_step: %v", err)
		}
		opts.clockStep = req.ClockStep
	}
	for _, pair := range req.Observe {
		if strings.TrimSpace(pair.GetStored()) == "" {
			return nil, statusError(connect.CodeInvalidArgument, "observe names no stored observable; name the observable, with the feature of the run that answers it")
		}
		opts.observe = append(opts.observe, simresults.ObservablePair{Stored: pair.GetStored(), Feature: pair.GetFeature()})
	}
	v, err := s.newVerifyContext(ctx, req.ModelHash, req.Engine)
	if err != nil {
		return nil, err
	}
	defer v.release()

	resp := &pb.CompareResultsResponse{Source: results.Source}
	selected, refused := results.Select(req.Actions)
	for i := range results.Configurations {
		if !selected[i] {
			continue
		}
		c, plan, err := s.compareConfiguration(ctx, v, &results.Configurations[i], results.Repeats(i), opts)
		if err != nil {
			return nil, err
		}
		if plan != nil {
			st := s.standingOf(*plan)
			resp.Engine, resp.Strength, resp.Bounds = st.engine, st.strength, st.bounds
		}
		resp.Configurations = append(resp.Configurations, c)
	}
	for _, r := range refused {
		resp.Configurations = append(resp.Configurations, &pb.ConfigurationComparison{
			Name: r.Name, Status: "unresolved", Unresolved: r.Reason, Lines: []string{errorLine + r.Reason}})
	}
	return resp, nil
}

// compareConfiguration runs one configuration on its target, for its run count,
// under its draw policy, and sets the tool's distributions beside the runs'. A
// configuration that cannot be run is answered unresolved; only a caller that
// went away is an error.
func (s *Service) compareConfiguration(ctx context.Context, v *verifyContext, cfg *simresults.ConfigurationResults, repeats []string, opts compareOptions) (*pb.ConfigurationComparison, *analysis.Plan, error) {
	label := "compare " + cfg.Name
	c := &pb.ConfigurationComparison{
		Id: cfg.ID, Name: cfg.Name, Behavior: cfg.Behavior, Target: cfg.Target, ResultLocation: cfg.Location,
		StoredRuns: cfg.StoredRuns(), StoredSnapshots: int64(len(cfg.Snapshots)),
	}
	unresolved := func(reason string) {
		c.Status, c.Unresolved = "unresolved", reason
		c.Lines = append(c.Lines, errorLine+reason)
	}
	notRun := func(why string) { unresolved("the configuration " + cfg.Name + " could not be run: " + why) }
	if cfg.Behavior == "" {
		reason := "the configuration " + cfg.Name + " performs no migrated behavior"
		if len(cfg.Notes) > 0 {
			reason += "; " + strings.Join(cfg.Notes, "; ")
		}
		unresolved(reason)
		return c, nil, nil
	}
	notes := slices.Clone(repeats)
	count := cfg.Runs
	if opts.runs > 0 {
		count = opts.runs
	}
	if count <= 0 {
		count = 1
		notes = append(notes, "the configuration states no numberOfRuns, so one run is made, as its tool makes without one; -runs <number> makes more")
	}
	policy := runtime.DrawRandom
	switch {
	case opts.draws != nil:
		policy = *opts.draws
	case cfg.Draws != "":
		parsed, err := runtime.ParseDrawPolicy(cfg.Draws)
		if err != nil {
			unresolved(fmt.Sprintf("the durationSimulationMode %q of the configuration %s is no draw policy", cfg.Draws, cfg.Name))
			return c, nil, nil
		}
		policy = parsed
	}
	step := cfg.ClockStep
	if opts.clockStep != nil {
		step = *opts.clockStep
	}
	if err := runtime.CheckClockStep(step); err != nil {
		unresolved("the clock step of the configuration " + cfg.Name + " cannot be run on: " + err.Error())
		return c, nil, nil
	}
	c.Runs, c.Draws, c.ClockStep = count, policy.String(), step
	syms := v.cached.LookupNamedMember(cfg.Name)
	if len(syms) == 0 {
		notRun("action not found: " + cfg.Name)
		return c, nil, nil
	}
	action := syms[0]
	plan := sweep.SeedlessMonteCarloPlan(count)
	switch {
	case opts.seed != nil:
		plan = sweep.MonteCarloPlan(count, *opts.seed)
	case !policy.Fixed():
		notRun("the runs draw at random; set seed, or fix the draws as min, max or average")
		return c, nil, nil
	}
	run := func(rt *runtime.Context, bindings []sweep.SweepBinding) (sweep.SweepRunResult, error) {
		i, ok := sweep.RunNumber(bindings)
		if !ok {
			return sweep.SweepRunResult{}, fmt.Errorf("%w: the row numbers no run", sweep.ErrSweepRuns)
		}
		if opts.seed != nil {
			rt.SetModelSeed(runtime.RunSeed(*opts.seed, i))
		} else {
			rt.ClearModelSeed()
		}
		rt.SetDrawPolicy(policy)
		if err := rt.SetClockStep(step); err != nil {
			return sweep.SweepRunResult{}, err
		}
		outcome, err := rt.ActionOutcomePerformedBy(action, nil, nil)
		if err != nil {
			return sweep.SweepRunResult{}, err
		}
		return sweep.SweepRunResult{Outputs: observedOutputs(rt, outcome)}, nil
	}
	schedule := v.runtime.Schedule()
	answered, err := s.engines.Sweep(ctx, analysis.Request{
		Model:     s.model(v.cached),
		Subject:   cfg.Name,
		Schedule:  schedule,
		Budget:    analysis.BudgetOf(s.budgets, schedule, analysis.Sweep, s.jobs),
		Selection: v.engine,
	}, plan, run)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		notRun(err.Error())
		return c, &answered, nil
	}
	table := answered.Result.Table()
	st := s.standingOf(answered)
	swept := v.sweepResponse(table, st)
	c.Rows, c.Instances = swept.Rows, swept.Instances
	if !table.Seedless {
		seed := table.Seed
		c.Seed = &seed
	}
	var failures []string
	for _, row := range table.Rows {
		if row.Err != nil {
			failures = append(failures, row.Err.Error())
			continue
		}
		c.Completed++
	}
	header := fmt.Sprintf("%s — %s in %s; %d run(s) by OpenSysML, draws %s", label, cfg.StoredRunsText(), orNoLocation(cfg.Location), c.Completed, policy)
	if !table.Seedless {
		header += fmt.Sprintf(", seed %d", table.Seed)
	}
	if step > 0 {
		header += ", clock step " + semantics.FormatReal(step) + " s"
	}
	comparison := simresults.Compare(cfg, table, opts.observe)
	c.Observables = comparisonToProto(comparison)
	c.Lines = append(c.Lines, header)
	c.Lines = append(c.Lines, comparison.Lines()...)
	c.Errors = dedupeSorted(failures)
	for _, f := range c.Errors {
		c.Lines = append(c.Lines, errorLine+f)
	}
	c.Notes = append(notes, cfg.Notes...)
	for _, n := range c.Notes {
		c.Lines = append(c.Lines, "note: "+n)
	}
	c.Status = "holds"
	if len(comparison.Observables) == 0 {
		// Nothing stood against anything: the tool stored no observable and the
		// runs produced no number, so there is nothing to hold.
		c.Status = "undecided"
		c.Notes = append(c.Notes, "no observable was compared")
		c.Lines = append(c.Lines, "note: no observable was compared")
	}
	for _, o := range comparison.Observables {
		// Compared alone: the runs may have produced no number for the observable,
		// or the tool stored none, and then nothing stood against anything.
		if !o.Compared || o.Unstored || o.Run == nil || o.Tool == nil {
			c.Status = "undecided"
		}
	}
	if len(failures) > 0 {
		c.Status = "fails"
	}
	return c, &answered, nil
}

// observedOutputs are every output a run produced, sorted by name, then the clock.
func observedOutputs(rt *runtime.Context, outcome runtime.Outcome) []runtime.CalcOutputValue {
	names := make([]string, 0, len(outcome.Outputs))
	for name := range outcome.Outputs {
		if name != clockObservable {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	outputs := make([]runtime.CalcOutputValue, 0, len(names)+1)
	for _, name := range names {
		outputs = append(outputs, runtime.CalcOutputValue{Name: name, Value: outcome.Outputs[name]})
	}
	return append(outputs, runtime.CalcOutputValue{Name: clockObservable, Value: rt.ClockValue()})
}

func orNoLocation(s string) string {
	if s == "" {
		return "no result location"
	}
	return s
}

func dedupeSorted(messages []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range messages {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out
}

func comparisonToProto(c simresults.Comparison) []*pb.ObservableComparison {
	out := make([]*pb.ObservableComparison, 0, len(c.Observables))
	for _, o := range c.Observables {
		p := &pb.ObservableComparison{
			Stored: o.Stored, Feature: o.Feature, Unit: o.Unit,
			Tool: distributionToProto(o.Tool), Pooled: o.Pooled, Unstored: o.Unstored,
			Run: distributionToProto(o.Run), Produced: int64(o.Produced), Compared: o.Compared,
			Notes: o.Notes,
		}
		if d := o.Difference; d != nil {
			p.Difference = &pb.StatisticDifference{Min: d.Min, Mean: d.Mean, P50: d.P50, P90: d.P90, Max: d.Max}
		}
		if cs := o.Declared; cs != nil {
			p.Declared = &pb.CaseStatistics{Analysis: cs.Analysis, Notes: cs.Notes}
			for _, row := range cs.Rows {
				p.Declared.Rows = append(p.Declared.Rows, &pb.CaseStatistic{
					Statistic: row.Statistic, Of: row.Of, Tool: row.Tool, Run: row.Run, Difference: row.Difference})
			}
		}
		out = append(out, p)
	}
	return out
}

func distributionToProto(d *runtime.Distribution) *pb.ComparedDistribution {
	if d == nil {
		return nil
	}
	return &pb.ComparedDistribution{
		Count: int64(d.Count), Min: simresults.RealOf(d.Min), Mean: d.Mean, P50: simresults.RealOf(d.P50),
		P90: simresults.RealOf(d.P90), Max: simresults.RealOf(d.Max), Deviation: d.Deviation,
	}
}
