package opensysml

import (
	"context"
	"time"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// DataImport is one data file ImportData sets the feature values of: a CSV,
// TSV, JSON or JSON Lines text, one row per element, named so its extension
// names its format unless Format does.
type DataImport struct {
	Name    string
	Content string
	// Format reads the content as csv, tsv, json or jsonl rather than as the
	// name's extension says.
	Format string
	// Map is the JSON mapping onto elements and features the content is read
	// by; "" for the content's own header.
	Map string
}

// ImportOption shapes an ImportData call.
type ImportOption func(*importOptions)

type importOptions struct {
	dryRun          bool
	acceptDocuments bool
}

// ImportDryRun reports the values the imports would set, changing nothing.
func ImportDryRun() ImportOption { return func(o *importOptions) { o.dryRun = true } }

// ImportAcceptDocuments accepts every edited document of a model of several.
func ImportAcceptDocuments() ImportOption { return func(o *importOptions) { o.acceptDocuments = true } }

// ImportResult is what ImportData answers: what each file set, and the model
// it made.
type ImportResult struct {
	Files []ImportedFile
	// Content is the edited content of a one-document model; Documents every
	// edited document. Both are empty on a dry run.
	Content   string
	Documents []EditedDocument
	// ModelHash is the imported model's, parsed and cached; "" on a dry run.
	ModelHash   string
	Diagnostics []Diagnostic
}

// ImportedFile is what one data file set.
type ImportedFile struct {
	Name             string
	Values, Elements int64
	Assignments      []ImportedValue
}

// ImportedValue is one assignment: the feature of the element set to the value,
// redefining the inherited feature Redefines when the element did not declare
// it, and where in the data it came from.
type ImportedValue struct {
	Element, Feature, Value, Redefines, Where, Description string
}

// ImportData sets the feature values the data files assign, one edit of the
// model per file, as `sysml -import` does (import_data capability).
func (c *client) ImportData(ctx context.Context, model *Model, imports []DataImport, opts ...ImportOption) (*ImportResult, error) {
	var o importOptions
	for _, opt := range opts {
		opt(&o)
	}
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireCapabilities(ctx, CapabilityImportData); err != nil {
		return nil, err
	}
	req := &pb.ImportDataRequest{ModelHash: hash, ImportDryRun: o.dryRun, AcceptDocuments: o.acceptDocuments}
	for _, in := range imports {
		req.Imports = append(req.Imports, &pb.DataImport{Name: in.Name, Content: in.Content, ImportFormat: in.Format, ImportMap: in.Map})
	}
	resp, err := c.caller.importData(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &FailureError{Op: "ImportData", Message: resp.Error, Diagnostics: diagnostics}
	}
	out := &ImportResult{Content: resp.Content, ModelHash: resp.ModelHash, Diagnostics: diagnostics}
	for _, f := range resp.Files {
		file := ImportedFile{Name: f.Name, Values: f.Values, Elements: f.Elements}
		for _, a := range f.Assignments {
			file.Assignments = append(file.Assignments, ImportedValue{
				Element: a.Element, Feature: a.Feature, Value: a.Value, Redefines: a.Redefines, Where: a.Where, Description: a.Description,
			})
		}
		out.Files = append(out.Files, file)
	}
	for _, d := range resp.Documents {
		out.Documents = append(out.Documents, EditedDocument{Name: d.Name, Content: d.Content})
	}
	return out, nil
}

// CompareOption shapes a CompareResults call.
type CompareOption func(*pb.CompareResultsRequest)

// CompareActions names the configurations compared, by qualified or simple name.
func CompareActions(names ...string) CompareOption {
	return func(r *pb.CompareResultsRequest) { r.Actions = append(r.Actions, names...) }
}

// CompareRuns makes count runs of every configuration instead of its own numberOfRuns.
func CompareRuns(count int64) CompareOption {
	return func(r *pb.CompareResultsRequest) { r.Runs = count }
}

// CompareSeed seeds the runs; without one, runs that draw at random are refused
// unless CompareDraws fixes them.
func CompareSeed(seed uint64) CompareOption {
	return func(r *pb.CompareResultsRequest) { r.Seed = &seed }
}

// CompareDraws replaces every configuration's duration mode: random, min, max or average.
func CompareDraws(policy string) CompareOption {
	return func(r *pb.CompareResultsRequest) { r.Draws = policy }
}

// CompareClockStep replaces every configuration's clock step, in seconds; 0 is continuous.
func CompareClockStep(step float64) CompareOption {
	return func(r *pb.CompareResultsRequest) { r.ClockStep = &step }
}

// CompareObserve compares the stored observable with the run's feature; an
// empty feature is the target's feature of the observable's own name.
func CompareObserve(stored, feature string) CompareOption {
	return func(r *pb.CompareResultsRequest) {
		r.Observe = append(r.Observe, &pb.ObservablePair{Stored: stored, Feature: feature})
	}
}

// CompareEngine selects the engine, as ListEngines names them.
func CompareEngine(engine string) CompareOption {
	return func(r *pb.CompareResultsRequest) { r.Engine = engine }
}

// Comparison is what CompareResults answers: one configuration after another,
// in the order the sidecar indexes them.
type Comparison struct {
	Source         string
	Configurations []ConfigurationComparison
	Standing       Standing
	Diagnostics    []Diagnostic
}

// ConfigurationComparison is one configuration's runs beside the tool's results.
type ConfigurationComparison struct {
	ID, Name, Behavior, Target, ResultLocation string
	// Status is holds, undecided (an observable was not compared), fails (a run
	// failed) or unresolved (the configuration could not be run, see Unresolved).
	Status, Unresolved          string
	StoredRuns, StoredSnapshots int64
	Runs, Completed             int64
	Draws                       string
	Seed                        *uint64
	ClockStep                   float64
	Rows                        []CompareRow
	// Instances are the objects the rows' values refer to, by id.
	Instances            []*Instance
	Observables          []ObservableComparison
	Errors, Notes, Lines []string
}

// CompareRow is one run: the run number it bound, what it produced, and the
// error that stopped it where it failed.
type CompareRow struct {
	Inputs, Outputs map[string]Value
	// Verdicts are the row's objective and assertion verdicts, Evaluations each
	// calc application the row made, as a sweep row reports them.
	Verdicts    []*Verdict
	Evaluations []Evaluation
	Elapsed     time.Duration
	Error       string
	Reason      Reason
}

// ObservableComparison is one observable: the tool's distribution beside the
// runs', or the notes on why they are not compared.
type ObservableComparison struct {
	Stored, Feature, Unit string
	Tool                  *ComparedDistribution
	Pooled, Unstored      bool
	Run                   *ComparedDistribution
	Produced              int64
	Compared              bool
	Difference            *StatisticDifference
	Declared              *CaseStatistics
	Notes                 []string
}

// ComparedDistribution is a distribution by its statistics; a pooled one knows
// its count and mean alone.
type ComparedDistribution struct {
	Count                               int64
	Min, Mean, P50, P90, Max, Deviation float64
}

// StatisticDifference is the relative difference of each statistic as a signed
// percentage, "" where it is not compared.
type StatisticDifference struct {
	Min, Mean, P50, P90, Max string
}

// CaseStatistics compares the statistics a Simulation::MonteCarlo case declares
// with the tool's, one row per statistic.
type CaseStatistics struct {
	Analysis string
	Rows     []CaseStatistic
	Notes    []string
}

// CaseStatistic is one statistic: which, where the case declares it, the
// tool's value, the runs' and their relative difference.
type CaseStatistic struct {
	Statistic, Of, Tool, Run, Difference string
}

// Holds is whether every configuration ran and compared.
func (c *Comparison) Holds() bool {
	for _, cfg := range c.Configurations {
		if cfg.Status != "holds" {
			return false
		}
	}
	return true
}

// CompareResults runs the configurations a tool's results sidecar (the JSON
// `sysml -migration-results` writes) indexes and tables the tool's distribution
// of every observable beside the runs' (compare_results capability).
func (c *client) CompareResults(ctx context.Context, model *Model, results string, opts ...CompareOption) (*Comparison, error) {
	hash, err := c.call(model)
	if err != nil {
		return nil, err
	}
	if err := c.requireCapabilities(ctx, CapabilityCompareResults); err != nil {
		return nil, err
	}
	req := &pb.CompareResultsRequest{ModelHash: hash, Results: results}
	for _, opt := range opts {
		opt(req)
	}
	resp, err := c.caller.compareResults(ctx, req)
	if err != nil {
		return nil, err
	}
	diagnostics := diagnosticsFromProto(resp.Diagnostics)
	if resp.Error != "" {
		return nil, &FailureError{Op: "CompareResults", Message: resp.Error, Diagnostics: diagnostics}
	}
	out := &Comparison{Source: resp.Source, Diagnostics: diagnostics, Standing: standingFromProto(resp.Engine, resp.Strength, resp.Bounds)}
	for _, cfg := range resp.Configurations {
		out.Configurations = append(out.Configurations, configurationComparisonFromProto(cfg))
	}
	return out, nil
}

func configurationComparisonFromProto(cfg *pb.ConfigurationComparison) ConfigurationComparison {
	out := ConfigurationComparison{
		ID: cfg.Id, Name: cfg.Name, Behavior: cfg.Behavior, Target: cfg.Target, ResultLocation: cfg.ResultLocation,
		Status: cfg.Status, Unresolved: cfg.Unresolved, StoredRuns: cfg.StoredRuns, StoredSnapshots: cfg.StoredSnapshots,
		Runs: cfg.Runs, Completed: cfg.Completed, Draws: cfg.Draws, Seed: cfg.Seed, ClockStep: cfg.ClockStep,
		Errors: cfg.Errors, Notes: cfg.Notes, Lines: cfg.Lines, Instances: instancesFromProto(cfg.Instances),
	}
	for _, row := range cfg.Rows {
		r := CompareRow{
			Inputs: make(map[string]Value, len(row.Inputs)), Outputs: make(map[string]Value, len(row.Outputs)),
			Evaluations: evaluationsFromProto(row.Evaluations), Elapsed: time.Duration(row.ElapsedMicros) * time.Microsecond,
			Error: row.Error, Reason: Reason(row.FailureReason),
		}
		for _, verdict := range row.Verdicts {
			r.Verdicts = append(r.Verdicts, verdictFromProto(verdict))
		}
		for _, in := range row.Inputs {
			r.Inputs[in.Name] = valueFromProto(in.Value)
		}
		for _, o := range row.Outputs {
			r.Outputs[o.Name] = valueFromProto(o.Value)
		}
		out.Rows = append(out.Rows, r)
	}
	for _, o := range cfg.Observables {
		obs := ObservableComparison{
			Stored: o.Stored, Feature: o.Feature, Unit: o.Unit, Tool: distributionFromProto(o.Tool), Pooled: o.Pooled, Unstored: o.Unstored,
			Run: distributionFromProto(o.Run), Produced: o.Produced, Compared: o.Compared, Notes: o.Notes,
		}
		if d := o.Difference; d != nil {
			obs.Difference = &StatisticDifference{Min: d.Min, Mean: d.Mean, P50: d.P50, P90: d.P90, Max: d.Max}
		}
		if cs := o.Declared; cs != nil {
			obs.Declared = &CaseStatistics{Analysis: cs.Analysis, Notes: cs.Notes}
			for _, row := range cs.Rows {
				obs.Declared.Rows = append(obs.Declared.Rows, CaseStatistic{Statistic: row.Statistic, Of: row.Of, Tool: row.Tool, Run: row.Run, Difference: row.Difference})
			}
		}
		out.Observables = append(out.Observables, obs)
	}
	return out
}

func distributionFromProto(d *pb.ComparedDistribution) *ComparedDistribution {
	if d == nil {
		return nil
	}
	return &ComparedDistribution{Count: d.Count, Min: d.Min, Mean: d.Mean, P50: d.P50, P90: d.P90, Max: d.Max, Deviation: d.Deviation}
}
