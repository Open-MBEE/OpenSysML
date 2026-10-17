package opensysml

import (
	"context"
	"fmt"
	"strings"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// Runs is what RunAction and RunMonteCarlo answer: one row per run, each
// observable's distribution over the completed runs and, for a Monte Carlo
// case, its conclusion over the sample.
type Runs struct {
	// Rows is the table, one per run in run order.
	Rows []RunRow
	// Observables names what every row reports, in column order.
	Observables []string
	// Distributions summarises each observable over the completed runs.
	Distributions []RunDistribution
	// Lines is the report as `sysml -runs` prints it.
	Lines []string
	// Outputs and Verdicts are a Monte Carlo case's conclusion over the sample;
	// Unconcluded says why it has none.
	Outputs     []CalcOutput
	Verdicts    []Verdict
	Unconcluded string
	Instances   []*Instance
	Standing    Standing
	// Seed is the seed the runs drew from, nil when the draws were fixed.
	Seed        *uint64
	Diagnostics []Diagnostic
}

// RunRow is one run: the run number it bound, what it observed, what it
// decided, and the error that stopped it where it failed.
type RunRow struct {
	Inputs   []CalcOutput
	Outputs  []CalcOutput
	Verdicts []Verdict
	Error    string
}

// RunDistribution summarises one observable over the completed runs: the
// extremes, mean, deviation, percentiles and histogram of a numeric one, or
// the count of each distinct value otherwise.
type RunDistribution struct {
	Name, Unit     string
	Count          int64
	Numeric        bool
	Min, Max, Mean Value
	Deviation      float64
	Percentiles    []RunPercentile
	Histogram      []RunHistogramBin
	Values         []RunCountedValue
}

// RunPercentile is the value a percentile of the runs fell at or below.
type RunPercentile struct {
	Percentile float64
	Value      Value
}

// RunHistogramBin counts the runs whose value fell in [Low, High).
type RunHistogramBin struct {
	Low, High Value
	Count     int64
}

// RunCountedValue counts the runs that observed one distinct value.
type RunCountedValue struct {
	Value Value
	Count int64
}

// RunsOption configures RunAction and RunMonteCarlo.
type RunsOption func(*runsOptions)

type runsOptions struct {
	inputs    map[string]Value
	performer string
	subject   string
	arguments []Value
	named     []namedArgument
	seed      *uint64
	draws     string
	clockStep float64
	observe   []string
	engine    string
}

// RunsInputs binds the action's input parameters before every run.
func RunsInputs(inputs map[string]Value) RunsOption {
	return func(o *runsOptions) { o.inputs = inputs }
}

// RunsPerformer names the part/usage, or a path from one, whose object performs the action.
func RunsPerformer(symbolID string) RunsOption {
	return func(o *runsOptions) { o.performer = symbolID }
}

// RunsSubject names the part/usage to instantiate as the case's subject.
func RunsSubject(symbolID string) RunsOption {
	return func(o *runsOptions) { o.subject = symbolID }
}

// RunsArguments binds the case's in parameters positionally.
func RunsArguments(values ...Value) RunsOption {
	return func(o *runsOptions) { o.arguments = append(o.arguments, values...) }
}

// RunsArgument binds one of the case's in parameters by name.
func RunsArgument(name string, value Value) RunsOption {
	return func(o *runsOptions) { o.named = append(o.named, namedArgument{name: name, value: value}) }
}

// RunsSeed seeds the runs' draws, one derived seed per run; required unless
// RunsDraws fixes them.
func RunsSeed(seed uint64) RunsOption {
	return func(o *runsOptions) { o.seed = &seed }
}

// RunsDraws sets the draw policy for the model's distributions: "random" (the
// default), "min", "max" or "average".
func RunsDraws(policy string) RunsOption {
	return func(o *runsOptions) { o.draws = policy }
}

// RunsClockStep sets the clock step of each run; zero keeps the default.
func RunsClockStep(step float64) RunsOption {
	return func(o *runsOptions) { o.clockStep = step }
}

// RunsObserve names the action's features each run reports, "clock" for the
// time it completed at; none reports every feature and the clock.
func RunsObserve(features ...string) RunsOption {
	return func(o *runsOptions) { o.observe = append(o.observe, features...) }
}

// RunsEngine selects the analysis engine asked; empty is auto.
func RunsEngine(name string) RunsOption {
	return func(o *runsOptions) { o.engine = name }
}

// RunAction runs an action count times, each in a context of its own, and
// tables what the runs observed. Requires the runs capability.
func (c *client) RunAction(ctx context.Context, model *Model, actionID string, count int64, opts ...RunsOption) (*Runs, error) {
	return c.runRuns(ctx, model, actionID, "", count, opts)
}

// RunMonteCarlo runs a Simulation::MonteCarlo case count times and concludes
// it over the sample. Requires the runs capability.
func (c *client) RunMonteCarlo(ctx context.Context, model *Model, caseID string, count int64, opts ...RunsOption) (*Runs, error) {
	return c.runRuns(ctx, model, "", caseID, count, opts)
}

func (c *client) runRuns(ctx context.Context, model *Model, actionID, caseID string, count int64, opts []RunsOption) (*Runs, error) {
	var options runsOptions
	for _, opt := range opts {
		opt(&options)
	}
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireCapabilities(ctx, CapabilityRuns); err != nil {
		return nil, err
	}
	if options.performer != "" {
		if err := c.requireCapabilities(ctx, CapabilityPerformer); err != nil {
			return nil, err
		}
	}
	inputs, err := valuesToProto(options.inputs)
	if err != nil {
		return nil, err
	}
	arguments, named, err := argumentsToProto(options.arguments, options.named)
	if err != nil {
		return nil, err
	}
	req := &pb.RunRunsRequest{
		ModelHash:         hash,
		ActionSymbolId:    actionID,
		AnalysisSymbolId:  caseID,
		Inputs:            inputs,
		PerformerSymbolId: options.performer,
		SubjectSymbolId:   options.subject,
		Arguments:         arguments,
		NamedArguments:    named,
		Runs:              count,
		Seed:              options.seed,
		Draws:             options.draws,
		ClockStep:         options.clockStep,
		Observe:           options.observe,
		Engine:            options.engine,
	}
	resp, err := c.caller.runRuns(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		op := "RunAction"
		if actionID == "" {
			op = "RunMonteCarlo"
		}
		return nil, &VerifyError{
			FailureError: FailureError{Op: op, Message: resp.Error, Diagnostics: diagnostics},
			Reason:       Reason(resp.FailureReason),
		}
	}
	return runsFromProto(resp, diagnostics), nil
}

// argumentsToProto converts a case's positional and named arguments.
func argumentsToProto(positional []Value, named []namedArgument) ([]*pb.Value, map[string]*pb.Value, error) {
	var arguments []*pb.Value
	for _, value := range positional {
		converted, err := valueToProto(value)
		if err != nil {
			return nil, nil, err
		}
		arguments = append(arguments, converted)
	}
	var byName map[string]*pb.Value
	if len(named) > 0 {
		byName = make(map[string]*pb.Value, len(named))
		for _, arg := range named {
			converted, err := valueToProto(arg.value)
			if err != nil {
				return nil, nil, err
			}
			byName[arg.name] = converted
		}
	}
	return arguments, byName, nil
}

func runsFromProto(resp *pb.RunRunsResponse, diagnostics []Diagnostic) *Runs {
	out := &Runs{
		Rows:        runRowsFromProto(resp.Rows),
		Observables: resp.Observables,
		Lines:       resp.Lines,
		Unconcluded: resp.Unconcluded,
		Instances:   instancesFromProto(resp.Instances),
		Standing:    standingFromProto(resp.Engine, resp.Strength, resp.Bounds),
		Seed:        resp.Seed,
		Diagnostics: diagnostics,
	}
	for _, d := range resp.Distributions {
		dist := RunDistribution{Name: d.Name, Unit: d.Unit, Count: d.Count, Numeric: d.Numeric, Deviation: d.Deviation}
		if d.Min != nil {
			dist.Min, dist.Max, dist.Mean = valueFromProto(d.Min), valueFromProto(d.Max), valueFromProto(d.Mean)
		}
		for _, p := range d.Percentiles {
			dist.Percentiles = append(dist.Percentiles, RunPercentile{Percentile: p.Percentile, Value: valueFromProto(p.Value)})
		}
		for _, b := range d.Histogram {
			dist.Histogram = append(dist.Histogram, RunHistogramBin{Low: valueFromProto(b.Low), High: valueFromProto(b.High), Count: b.Count})
		}
		for _, v := range d.Values {
			dist.Values = append(dist.Values, RunCountedValue{Value: valueFromProto(v.Value), Count: v.Count})
		}
		out.Distributions = append(out.Distributions, dist)
	}
	out.Outputs = calcOutputsFromProto(resp.Outputs)
	for _, verdict := range resp.Verdicts {
		if converted := verdictFromProto(verdict); converted != nil {
			out.Verdicts = append(out.Verdicts, *converted)
		}
	}
	return out
}

func calcOutputsFromProto(outputs []*pb.CalcOutput) []CalcOutput {
	var out []CalcOutput
	for _, output := range outputs {
		out = append(out, CalcOutput{Name: output.Name, Value: valueFromProto(output.Value)})
	}
	return out
}

func runRowsFromProto(rows []*pb.SweepRow) []RunRow {
	out := make([]RunRow, 0, len(rows))
	for _, row := range rows {
		converted := RunRow{Outputs: calcOutputsFromProto(row.Outputs), Error: row.Error}
		for _, in := range row.Inputs {
			converted.Inputs = append(converted.Inputs, CalcOutput{Name: in.Name, Value: valueFromProto(in.Value)})
		}
		for _, verdict := range row.Verdicts {
			if v := verdictFromProto(verdict); v != nil {
				converted.Verdicts = append(converted.Verdicts, *v)
			}
		}
		out = append(out, converted)
	}
	return out
}

// String is the report as `sysml -runs` prints it.
func (r *Runs) String() string { return strings.Join(r.Lines, "\n") }

// SelfCheck is what SelfCheck answers: every finding of the reflective
// validation check and the counts over them.
type SelfCheck struct {
	// Status is "holds", "fails" or "unresolved".
	Status                                                      string
	Findings                                                    []SelfCheckFinding
	Elements, Checks, Violations, Unevaluated, EvaluationErrors int64
	// Applications is how many elements each constraint was applied to.
	Applications map[string]int64
	// Summary is the one-line total `sysml -self-check` prints last.
	Summary     string
	Diagnostics []Diagnostic
}

// SelfCheckFinding is one violation, evaluation the runtime could not make,
// error or skipped rule constraint; Line is it as the CLI prints it.
type SelfCheckFinding struct {
	Status, Subject, Location, Constraint, Element, Message, Line string
}

// Holds reports whether the check found no violation and no evaluation error.
func (s *SelfCheck) Holds() bool { return s.Status == "holds" }

// SelfCheck applies the SysMLValidation constraints, and those of the rule
// packages named, reflectively to every element of the model. Requires the
// self_check capability.
func (c *client) SelfCheck(ctx context.Context, model *Model, rulePackages ...string) (*SelfCheck, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireCapabilities(ctx, CapabilitySelfCheck); err != nil {
		return nil, err
	}
	resp, err := c.caller.selfCheck(ctx, &pb.SelfCheckRequest{ModelHash: hash, SelfCheckPackage: rulePackages})
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &FailureError{Op: "SelfCheck", Message: resp.Error, Diagnostics: diagnostics}
	}
	out := &SelfCheck{
		Status: resp.Status, Elements: resp.Elements, Checks: resp.Checks, Violations: resp.Violations,
		Unevaluated: resp.Unevaluated, EvaluationErrors: resp.EvaluationErrors, Summary: resp.Summary,
		Applications: make(map[string]int64, len(resp.Applications)), Diagnostics: diagnostics,
	}
	for _, f := range resp.Findings {
		out.Findings = append(out.Findings, SelfCheckFinding{
			Status: f.Status, Subject: f.Subject, Location: f.Location, Constraint: f.Constraint,
			Element: f.Element, Message: f.Message, Line: f.Line,
		})
	}
	for _, a := range resp.Applications {
		out.Applications[a.Constraint] = a.Count
	}
	return out, nil
}

// Recording is what RecordRun answers: the runs' own report, what the model
// gained and the notation the records were written into.
type Recording struct {
	// Run, Sweep or Sample is the runs' own result, by how the case ran.
	Run    *Analysis
	Sweep  []RunRow
	Sample *Runs
	// Package, Definition and Records name what the model gained; Source is
	// the declarations generated.
	Package, Definition string
	Records             []string
	Source              string
	// Content is the recorded notation of a single-document model the records
	// merged into; Documents always carries the document they went into, or
	// the records as a document of their own.
	Content   string
	Documents []RecordedDocument
	// Skipped counts the rows or runs that failed and were not recorded;
	// Reasons says why runs were not recorded.
	Skipped int64
	Reasons []string
	// ModelHash names the recorded model, cached for the next request.
	ModelHash   string
	Diagnostics []Diagnostic
}

// RecordedDocument is one document the records were written into.
type RecordedDocument struct {
	Name, Content string
}

// RecordOption configures RecordRun.
type RecordOption func(*recordOptions)

type recordOptions struct {
	subject   string
	arguments []Value
	named     []namedArgument
	engine    string
	into      string
	ranges    []*pb.SweepRange
	runs      int64
	seed      *uint64
	// err is the first option that could not be encoded, refused before the call.
	err error
}

// RecordSubject names the part/usage to instantiate as the case's subject.
func RecordSubject(symbolID string) RecordOption {
	return func(o *recordOptions) { o.subject = symbolID }
}

// RecordArguments binds the case's in parameters positionally.
func RecordArguments(values ...Value) RecordOption {
	return func(o *recordOptions) { o.arguments = append(o.arguments, values...) }
}

// RecordArgument binds one of the case's in parameters by name.
func RecordArgument(name string, value Value) RecordOption {
	return func(o *recordOptions) { o.named = append(o.named, namedArgument{name: name, value: value}) }
}

// RecordEngine selects the analysis engine asked; empty is auto.
func RecordEngine(name string) RecordOption {
	return func(o *recordOptions) { o.engine = name }
}

// RecordInto names the package the records go into; empty is a Records
// package beside the package enclosing the case's own.
func RecordInto(pkg string) RecordOption {
	return func(o *recordOptions) { o.into = pkg }
}

// RecordRange runs the case once per value of a parameter stepped from start
// to end, as a sweep does, and records each completed row; several ranges
// make their product.
func RecordRange(parameter string, start, end, step Value) RecordOption {
	return func(o *recordOptions) {
		r := &pb.SweepRange{Parameter: parameter}
		var err error
		for _, bound := range []struct {
			into **pb.Value
			from Value
		}{{&r.Start, start}, {&r.End, end}, {&r.Step, step}} {
			if bound.from == nil {
				continue
			}
			if *bound.into, err = valueToProto(bound.from); err != nil && o.err == nil {
				o.err = fmt.Errorf("RecordRange %s: %w", parameter, err)
			}
		}
		o.ranges = append(o.ranges, r)
	}
}

// RecordSample runs the Simulation::MonteCarlo case count times from seed and
// records each completed run and the sample's conclusion.
func RecordSample(count int64, seed uint64) RecordOption {
	return func(o *recordOptions) { o.runs, o.seed = count, &seed }
}

// RecordRun runs an analysis case and records the runs into the model as
// AnalysisRecords declarations. Requires the record_run capability.
func (c *client) RecordRun(ctx context.Context, model *Model, caseID string, opts ...RecordOption) (*Recording, error) {
	var options recordOptions
	for _, opt := range opts {
		opt(&options)
	}
	if options.err != nil {
		return nil, options.err
	}
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireCapabilities(ctx, CapabilityRecordRun); err != nil {
		return nil, err
	}
	arguments, named, err := argumentsToProto(options.arguments, options.named)
	if err != nil {
		return nil, err
	}
	req := &pb.RecordRunRequest{
		ModelHash:        hash,
		AnalysisSymbolId: caseID,
		SubjectSymbolId:  options.subject,
		Arguments:        arguments,
		NamedArguments:   named,
		Engine:           options.engine,
		RecordInto:       options.into,
		Ranges:           options.ranges,
		Runs:             options.runs,
		Seed:             options.seed,
		AcceptDocuments:  true,
	}
	resp, err := c.caller.recordRun(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &VerifyError{
			FailureError: FailureError{Op: "RecordRun", Message: resp.Error, Diagnostics: diagnostics},
			Reason:       Reason(resp.FailureReason),
		}
	}
	out := &Recording{
		Package: resp.Package, Definition: resp.Definition, Records: resp.Records, Source: resp.Source,
		Content: resp.Content, Skipped: resp.Skipped, Reasons: resp.Reasons, ModelHash: resp.ModelHash,
		Diagnostics: diagnostics,
	}
	if resp.Run != nil {
		out.Run = analysisFromProto(resp.Run, diagnosticsFromProto(resp.Run.Diagnostics))
	}
	if resp.Sweep != nil {
		out.Sweep = runRowsFromProto(resp.Sweep.Rows)
	}
	if resp.Sample != nil {
		out.Sample = runsFromProto(resp.Sample, diagnosticsFromProto(resp.Sample.Diagnostics))
	}
	for _, doc := range resp.Documents {
		out.Documents = append(out.Documents, RecordedDocument{Name: doc.Name, Content: doc.Content})
	}
	return out, nil
}
