package simresults

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/sweep"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// ObservablePair is a stored observable and the feature of the run that answers
// it; an empty Feature is the target's feature of the observable's own name.
type ObservablePair struct {
	Stored  string
	Feature string
}

// Comparison sets the tool's distribution of every compared observable of one
// configuration beside the distribution the runs came to, observable by observable.
type Comparison struct {
	Observables []ObservableComparison
}

// ObservableComparison is one observable's comparison: the tool's distribution
// (pooled from summaries when it stored any), the runs' distribution when the
// completed runs all produced a number for the feature in one unit, and the
// relative difference of each statistic when both are there.
type ObservableComparison struct {
	Stored  string
	Feature string
	// Unit is the unit the runs' numbers came in, "" for none.
	Unit string
	// Tool is the tool's distribution, nil when it stored no number; Pooled is
	// whether it was pooled from summaries, so only Count and Mean are known.
	Tool   *runtime.Distribution
	Pooled bool
	// Unstored is whether the tool stored no result at all to compare against.
	Unstored bool
	// Run is the distribution of the completed runs, nil when they are not
	// compared; Produced is how many runs produced a number for the feature.
	Run      *runtime.Distribution
	Produced int
	// Compared is whether the runs were made and are reported, false when the
	// tool stored no observable of the name or no number for it.
	Compared bool
	// Difference is the relative difference of each statistic, nil without
	// both distributions; a pooled tool distribution compares the mean alone.
	Difference *Difference
	// Declared is the per-statistic comparison of a Simulation::MonteCarlo case
	// the configuration's observable is analysed by, nil when none is.
	Declared *CaseStatistics
	// Notes explain what was pooled, not compared, or compared with reservations.
	Notes []string
}

// Difference is the relative difference of each statistic, "" where it is not
// compared.
type Difference struct {
	Min, Mean, P50, P90, Max string
}

// CaseStatistics compares the statistics a Simulation::MonteCarlo case declares
// of an observable with the tool's pooled values, one row per statistic.
type CaseStatistics struct {
	Analysis string
	Rows     []CaseStatistic
	Notes    []string
}

// CaseStatistic is one statistic of the case: what it is, where the case
// declares it, the tool's value, the runs' value and their relative difference.
type CaseStatistic struct {
	Statistic, Of, Tool, Run, Difference string
}

// comparison is what the tool stored of an observable, as runRows compares against.
type comparison struct {
	stored   *runtime.Distribution
	pooled   bool
	declared *declared
}

// declared are the statistics of a Simulation::MonteCarlo case of the observable it
// analyses: the returns the case declares, then the outputs of the case the tool
// stored a value of without a return; with the tool's pooled value of each.
type declared struct {
	analysis   string
	statistics []string
	returned   int
	deviation  *float64
	outOfSpec  *int64
}

// caseOutputs are the outputs of Simulation::MonteCarlo by the tool's statistic each
// is bound from, in the order the case declares them.
var caseOutputs = []struct{ statistic, output string }{
	{StatisticRuns, runtime.MonteCarloRunsOutput},
	{StatisticMean, runtime.MonteCarloMeanOutput},
	{StatisticDeviation, runtime.MonteCarloDeviationOutput},
	{StatisticOutOfSpec, runtime.MonteCarloOutOfSpecOutput},
}

// caseOutput is the output of Simulation::MonteCarlo the tool's statistic is bound from.
func caseOutput(statistic string) string {
	for _, o := range caseOutputs {
		if o.statistic == statistic {
			return o.output
		}
	}
	return ""
}

// Compare sets the tool's results of each compared observable of cfg beside the
// runs of table, in the order ComparedObservables lists them. Numbers are read
// as they are; nothing is scaled or tuned.
func Compare(cfg *ConfigurationResults, table sweep.SweepTable, observe []ObservablePair) Comparison {
	var c Comparison
	for _, pair := range ComparedObservables(cfg, table, observe) {
		c.Observables = append(c.Observables, compareObservable(cfg, table, pair))
	}
	return c
}

func compareObservable(cfg *ConfigurationResults, table sweep.SweepTable, pair ObservablePair) ObservableComparison {
	o := ObservableComparison{Stored: pair.Stored, Feature: pair.Feature}
	name := pair.Stored
	if len(cfg.Snapshots) == 0 {
		o.Unstored = true
		runRows(&o, table, nil)
		return o
	}
	if !slices.Contains(cfg.Observables, name) {
		o.Notes = append(o.Notes, fmt.Sprintf("note: the tool stored no observable named %s, which %s was to answer", name, pair.Feature))
		return o
	}
	stored, pooled := StoredDistribution(cfg, name)
	if stored == nil {
		o.Notes = append(o.Notes, fmt.Sprintf("note: no snapshot holds a number for %s", name))
		return o
	}
	o.Tool, o.Pooled = stored, pooled
	if pooled {
		o.Notes = append(o.Notes, pooledNotes(cfg, name)...)
	}
	tool := &comparison{stored: stored, pooled: pooled}
	if pooled && name == cfg.Analysis {
		tool.declared = declaredStatistics(cfg, name)
	}
	runRows(&o, table, tool)
	return o
}

// pooledNotes say which summaries the tool's pooled distribution was pooled from,
// and whether they could have come from one model.
func pooledNotes(cfg *ConfigurationResults, name string) []string {
	var notes []string
	for _, snap := range cfg.Summarised(name) {
		st := snap.Statistics
		note := fmt.Sprintf("note: %s summarises %d run(s) of %s: mean %s", orUnnamed(snap.Name), st.Runs, name, Spell(st.Mean))
		if st.Deviation != nil {
			note += ", deviation " + Spell(*st.Deviation)
		}
		if st.OutOfSpec != nil {
			note += fmt.Sprintf(", %d out of specification", *st.OutOfSpec)
		}
		notes = append(notes, note)
	}
	if apart := cfg.Disagreeing(name); len(apart) > 0 {
		names := make([]string, len(apart))
		for i, snap := range apart {
			names[i] = orUnnamed(snap.Name)
		}
		notes = append(notes, fmt.Sprintf("note: the summaries %s of %s lie more than three standard errors apart, so they cannot be of runs of one and the same model, and the tool's mean of %s blends them", strings.Join(names, ", "), name, name))
	}
	return notes
}

// runRows sets the runs' distribution of the feature beside the tool's, or says
// why the runs are not compared.
func runRows(o *ObservableComparison, table sweep.SweepTable, tool *comparison) {
	name, feature := o.Stored, o.Feature
	ran, units, other, missing := runValues(table, feature)
	d := runtime.Distribute(ran)
	o.Compared = true
	o.Produced = len(ran)
	switch {
	case len(ran) == 0 && other == 0:
		o.Notes = append(o.Notes, fmt.Sprintf("note: no completed run produced %s, which answers %s", feature, name))
		return
	case missing > 0:
		note := fmt.Sprintf("note: %s was produced by %d of the %d completed run(s)", feature, len(ran)+other, len(ran)+other+missing)
		if other > 0 {
			note += fmt.Sprintf(" and holds no number in %d of those", other)
		}
		o.Notes = append(o.Notes, note+fmt.Sprintf(", so %s is not compared", name))
		return
	case d == nil:
		o.Produced = 0
		o.Notes = append(o.Notes, fmt.Sprintf("note: %s holds no number in any completed run, so %s is not compared", feature, name))
		return
	case other > 0:
		o.Notes = append(o.Notes, fmt.Sprintf("note: %s holds no number in %d of the %d completed run(s) that produced it, so %s is not compared", feature, other, other+len(ran), name))
		return
	case len(units) > 1:
		o.Notes = append(o.Notes, fmt.Sprintf("note: %s came to numbers in more than one unit (%s) over the completed runs, so %s is not compared", feature, unitList(units), name))
		return
	}
	o.Run, o.Unit = d, units[0]
	switch {
	case tool == nil:
	case tool.pooled:
		o.Difference = &Difference{Mean: Relative(DrawnMean(tool.stored), DrawnMean(d))}
		switch {
		case tool.declared != nil:
			o.Declared = caseStatistics(name, feature, tool, d)
		case d.Count < 2:
			o.Notes = append(o.Notes, fmt.Sprintf("note: %s came to no deviation over the %d completed run(s): fewer than two define none", feature, d.Count))
		default:
			o.Notes = append(o.Notes, fmt.Sprintf("note: %s came to a deviation of %s over the %d completed run(s)", feature, Spell(d.Deviation), d.Count))
		}
	default:
		o.Difference = &Difference{
			Min: Relative(tool.stored.Min, d.Min), Mean: Relative(DrawnMean(tool.stored), DrawnMean(d)),
			P50: Relative(tool.stored.P50, d.P50), P90: Relative(tool.stored.P90, d.P90), Max: Relative(tool.stored.Max, d.Max),
		}
	}
}

// caseStatistics compares each statistic the case declares of the observable with
// the tool's pooled value: the mean and deviation by relative difference, the run
// count as is, and the out-of-specification count not at all.
func caseStatistics(name, feature string, tool *comparison, d *runtime.Distribution) *CaseStatistics {
	cs := &CaseStatistics{Analysis: tool.declared.analysis}
	for i, stat := range tool.declared.statistics {
		of := "out " + caseOutput(stat)
		if i < tool.declared.returned {
			of = "return " + stat
		}
		row := CaseStatistic{Statistic: stat, Of: of}
		switch stat {
		case StatisticMean:
			row.Tool, row.Run, row.Difference = Spell(tool.stored.Mean), Spell(d.Mean), Relative(DrawnMean(tool.stored), DrawnMean(d))
		case StatisticDeviation:
			if tool.declared.deviation != nil {
				row.Tool = Spell(*tool.declared.deviation)
			}
			switch {
			case d.Count < 2:
				cs.Notes = append(cs.Notes, fmt.Sprintf("note: %s came to no deviation over the %d completed run(s): fewer than two define none, so %s is not compared", feature, d.Count, stat))
			case tool.declared.deviation == nil:
				row.Run = Spell(d.Deviation)
				cs.Notes = append(cs.Notes, fmt.Sprintf("note: a summary of %s kept no deviation, so the tool's is not pooled and %s is not compared", name, stat))
			default:
				row.Run, row.Difference = Spell(d.Deviation), Relative(RealValue(*tool.declared.deviation), RealValue(d.Deviation))
			}
		case StatisticRuns:
			row.Tool, row.Run = fmt.Sprint(tool.stored.Count), fmt.Sprint(d.Count)
		case StatisticOutOfSpec:
			if tool.declared.outOfSpec != nil {
				row.Tool = fmt.Sprint(*tool.declared.outOfSpec)
			}
			cs.Notes = append(cs.Notes, fmt.Sprintf("note: %s counts the runs the tool found out of specification by its own criterion, which no migrated check evaluates, so it is not compared", stat))
		}
		cs.Rows = append(cs.Rows, row)
	}
	return cs
}

// declaredStatistics are the statistics of the analysis case written for cfg's
// target, nil when none was: its declared returns, then the case's other outputs
// the tool's summaries of observable stored, with the tool's values pooled.
func declaredStatistics(cfg *ConfigurationResults, observable string) *declared {
	if cfg.AnalysisCase == "" {
		return nil
	}
	d := &declared{analysis: cfg.AnalysisCase, statistics: slices.Clone(cfg.Statistics), returned: len(cfg.Statistics)}
	if dev, ok := StoredDeviation(cfg, observable); ok {
		d.deviation = &dev
	}
	var out int64
	counted := false
	for _, s := range cfg.Summarised(observable) {
		if s.Statistics.OutOfSpec != nil {
			out += *s.Statistics.OutOfSpec
			counted = true
		}
	}
	if counted {
		d.outOfSpec = &out
	}
	for _, o := range caseOutputs {
		if slices.Contains(d.statistics, o.statistic) || !storedStatistic(cfg, observable, o.statistic) {
			continue
		}
		d.statistics = append(d.statistics, o.statistic)
	}
	return d
}

// storedStatistic is whether a summary of the observable holds the statistic.
func storedStatistic(cfg *ConfigurationResults, observable, statistic string) bool {
	for _, s := range cfg.Summarised(observable) {
		switch statistic {
		case StatisticRuns, StatisticMean:
			return true
		case StatisticDeviation:
			if s.Statistics.Deviation != nil {
				return true
			}
		case StatisticOutOfSpec:
			if s.Statistics.OutOfSpec != nil {
				return true
			}
		}
	}
	return false
}

// StoredDistribution is the tool's distribution of the observable: over its
// stored values when it summarised none, else pooled from values and summaries,
// which fixes the count and the mean alone.
func StoredDistribution(cfg *ConfigurationResults, observable string) (d *runtime.Distribution, pooled bool) {
	values := cfg.Values(observable)
	summarised := cfg.Summarised(observable)
	if len(summarised) == 0 {
		return runtime.Distribute(reals(values)), false
	}
	weighted := make([]runtime.Counted, 0, len(values)+len(summarised))
	for _, v := range values {
		weighted = append(weighted, runtime.Counted{Count: 1, Value: v})
	}
	for _, s := range summarised {
		weighted = append(weighted, runtime.Counted{Count: s.Statistics.Runs, Value: s.Statistics.Mean})
	}
	mean, runs := runtime.WeightedMean(weighted)
	if runs == 0 {
		return nil, true
	}
	return &runtime.Distribution{Count: int(runs), Mean: mean}, true
}

// StoredDeviation pools the deviation of the observable over the tool's values and
// summaries, false when a summary kept none or fewer than two runs define one.
func StoredDeviation(cfg *ConfigurationResults, observable string) (float64, bool) {
	pooled, _ := StoredDistribution(cfg, observable)
	if pooled == nil || pooled.Count < 2 {
		return 0, false
	}
	var spreads []runtime.Spread
	for _, v := range cfg.Values(observable) {
		spreads = append(spreads, runtime.Spread{Weight: 1, Value: v, About: true})
	}
	for _, s := range cfg.Summarised(observable) {
		st := s.Statistics
		if st.Deviation == nil {
			return 0, false
		}
		n := float64(st.Runs)
		spreads = append(spreads, runtime.Spread{Weight: n - 1, Value: *st.Deviation}, runtime.Spread{Weight: n, Value: st.Mean, About: true})
	}
	return runtime.PooledDeviation(pooled.Mean, spreads, pooled.Count-1), true
}

// ComparedObservables are the pairs asked for, or every stored observable when
// none was — the observable the configuration's analysis summarises, else every
// feature the completed runs produced a number for, when the tool stored none;
// one naming no feature is read from the feature of its own name that the
// target object holds — `target.Time_Total` — or the bare name when the
// configuration runs on no target.
func ComparedObservables(cfg *ConfigurationResults, table sweep.SweepTable, observe []ObservablePair) []ObservablePair {
	pairs := make([]ObservablePair, 0, max(len(observe), len(cfg.Observables)))
	switch {
	case len(observe) > 0:
		pairs = append(pairs, observe...)
	case len(cfg.Snapshots) == 0 && cfg.Analysis != "":
		pairs = append(pairs, ObservablePair{Stored: cfg.Analysis})
	case len(cfg.Snapshots) == 0:
		for _, feature := range numericOutputs(table) {
			pairs = append(pairs, ObservablePair{Stored: strings.TrimPrefix(feature, cfg.Target+"."), Feature: feature})
		}
	default:
		for _, name := range cfg.Observables {
			pairs = append(pairs, ObservablePair{Stored: name})
		}
	}
	for i := range pairs {
		if pairs[i].Feature != "" {
			continue
		}
		pairs[i].Feature = pairs[i].Stored
		if cfg.Target != "" {
			pairs[i].Feature = cfg.Target + "." + pairs[i].Stored
		}
	}
	return pairs
}

// numericOutputs names the outputs a completed run produced a number for, sorted.
func numericOutputs(table sweep.SweepTable) []string {
	seen := map[string]bool{}
	var names []string
	for _, row := range table.Rows {
		if row.Err != nil {
			continue
		}
		for _, out := range row.Outputs {
			if _, ok := runtime.MagnitudeValue(out.Value); ok && !seen[out.Name] {
				seen[out.Name] = true
				names = append(names, out.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// runValues are the numbers the completed runs produced for the feature, the
// units they came in, how many runs produced something else, and how many
// produced nothing for it.
func runValues(table sweep.SweepTable, feature string) (numbers []semantics.Value, units []string, other, missing int) {
	for _, row := range table.Rows {
		if row.Err != nil {
			continue
		}
		produced := false
		for _, out := range row.Outputs {
			if out.Name != feature {
				continue
			}
			produced = true
			n, unit, ok := runtime.Magnitude(out.Value)
			if !ok {
				other++
				continue
			}
			numbers = append(numbers, n)
			if !slices.Contains(units, unit) {
				units = append(units, unit)
			}
		}
		if !produced {
			missing++
		}
	}
	return numbers, units, other, missing
}

func unitList(units []string) string {
	names := make([]string, len(units))
	for i, u := range units {
		if u == "" {
			u = "none"
		}
		names[i] = u
	}
	return strings.Join(names, ", ")
}

func reals(values []float64) []semantics.Value {
	out := make([]semantics.Value, len(values))
	for i, v := range values {
		out[i] = RealValue(v)
	}
	return out
}

// Relative spells the relative difference of ran from stored as a signed
// percentage, or the absolute difference when stored is zero.
func Relative(stored, ran semantics.Value) string {
	a, b := RealOf(stored), RealOf(ran)
	if a == 0 {
		if b == 0 {
			return "+0.0%"
		}
		return fmt.Sprintf("%+.4g (of 0)", b)
	}
	return fmt.Sprintf("%+.1f%%", (b-a)/math.Abs(a)*100)
}

// RealOf is a statistic's number, an Integer read as the Real it is.
func RealOf(v semantics.Value) float64 {
	if v.Kind == semantics.ValInt {
		return v.AsReal()
	}
	return v.Real
}

// DrawnMean is a distribution's mean as the Real the runs drew.
func DrawnMean(d *runtime.Distribution) semantics.Value {
	return semantics.Value{Kind: semantics.ValReal, Real: d.Mean}
}

// Spell spells a statistic as the Real the tool wrote it as.
func Spell(v float64) string {
	return semantics.FormatConst(RealValue(v))
}

// RealValue wraps a statistic as the Real the tool wrote it as.
func RealValue(v float64) semantics.Value {
	return semantics.Value{Kind: semantics.ValReal, Real: v}
}

func orUnnamed(name string) string {
	if name == "" {
		return "an unnamed snapshot"
	}
	return strconv.Quote(name)
}

// Refusal is a configuration name Select could not resolve, and why.
type Refusal struct {
	Name   string
	Reason string
}

// Select marks the configurations the names pick, each by its id, its qualified
// name or, failing an exact match, its simple name; a name matching none or
// several is refused. Every configuration is selected when no name is given.
func (r *Results) Select(names []string) (selected []bool, refused []Refusal) {
	cfgs := r.Configurations
	selected = make([]bool, len(cfgs))
	if len(names) == 0 {
		for i := range selected {
			selected[i] = true
		}
		return selected, nil
	}
	for _, name := range names {
		var exact, simple []int
		for i := range cfgs {
			switch {
			case name == cfgs[i].ID || name == cfgs[i].Name:
				exact = append(exact, i)
			case SameName(name, cfgs[i].Name):
				simple = append(simple, i)
			}
		}
		found := exact
		if len(found) == 0 {
			found = simple
		}
		switch len(found) {
		case 1:
			selected[found[0]] = true
		case 0:
			refused = append(refused, Refusal{Name: name, Reason: fmt.Sprintf("no configuration is named %s", name)})
		default:
			spelled := make([]string, len(found))
			for j, i := range found {
				spelled[j] = cfgs[i].Name
			}
			refused = append(refused, Refusal{Name: name,
				Reason: fmt.Sprintf("%d configurations are named %s (%s); name one by its qualified name", len(found), name, strings.Join(spelled, ", "))})
		}
	}
	return selected, refused
}

// SameName is whether name names the configuration qualified: as the qualified
// name itself, as its text unquoted, or as its last segment.
func SameName(name, qualified string) bool {
	if name == qualified {
		return true
	}
	segments, ok := source.QualifiedNameSegments(qualified)
	if !ok {
		return plainName(name) == plainName(qualified)
	}
	want := plainName(name)
	return want == strings.Join(segments, "::") || want == segments[len(segments)-1]
}

// plainName is the text of a single, possibly quoted, name; anything else as is.
func plainName(name string) string {
	if segments, ok := source.QualifiedNameSegments(name); ok && len(segments) == 1 {
		return segments[0]
	}
	return name
}

// OpenSysMLLabel prefixes the runs' row of a comparison table, before the feature.
const OpenSysMLLabel = "OpenSysML ("

// Lines table the comparison as `sysml -compare-results` prints it: the tool's
// distribution of every observable beside the runs', with the relative difference
// of each statistic, then the notes on what was pooled or not compared.
func (c Comparison) Lines() []string {
	cells := [][]string{{"observable", "source", "runs", "min", "mean", "p50", "p90", "max"}}
	var notes []string
	for _, o := range c.Observables {
		label := OpenSysMLLabel + o.Feature + ")"
		switch {
		case o.Unstored:
			cells = append(cells, []string{o.Stored, "tool (no stored result to compare)", "0", "", "", "", "", ""})
		case o.Tool != nil && o.Pooled:
			cells = append(cells, []string{o.Stored, "tool", fmt.Sprint(o.Tool.Count), "", withUnit(DrawnMean(o.Tool), ""), "", "", ""})
		case o.Tool != nil:
			cells = append(cells, statisticsRow(o.Stored, "tool", o.Tool, ""))
		}
		switch {
		case !o.Compared:
		case o.Run != nil:
			cells = append(cells, statisticsRow("", label, o.Run, o.Unit))
		default:
			cells = append(cells, []string{"", label, fmt.Sprint(o.Produced), "", "", "", "", ""})
		}
		if d := o.Difference; d != nil {
			cells = append(cells, []string{"", "difference", "", d.Min, d.Mean, d.P50, d.P90, d.Max})
		}
		notes = append(notes, o.Notes...)
		if o.Declared != nil {
			notes = append(notes, o.Declared.lines(o.Stored, o.Feature)...)
		}
	}
	return append(renderTable(cells), notes...)
}

// lines table the case's statistics, the tool's beside the runs'.
func (cs *CaseStatistics) lines(name, feature string) []string {
	cells := [][]string{{"statistic", "of the case", "tool", OpenSysMLLabel + feature + ")", "difference"}}
	for _, row := range cs.Rows {
		cells = append(cells, []string{row.Statistic, row.Of, row.Tool, row.Run, row.Difference})
	}
	lines := append([]string{fmt.Sprintf("statistics of %s by %s:", name, cs.Analysis)}, renderTable(cells)...)
	return append(lines, cs.Notes...)
}

func statisticsRow(observable, source string, d *runtime.Distribution, unit string) []string {
	return []string{observable, source, fmt.Sprint(d.Count),
		withUnit(d.Min, unit), withUnit(DrawnMean(d), unit), withUnit(d.P50, unit), withUnit(d.P90, unit), withUnit(d.Max, unit)}
}

func withUnit(x semantics.Value, unit string) string {
	text := semantics.FormatConst(x)
	if unit == "" {
		return text
	}
	return text + " [" + unit + "]"
}

// renderTable renders a header row, a rule and the rows, the columns aligned and
// separated as the sweep tables are.
func renderTable(cells [][]string) []string {
	widths := make([]int, len(cells[0]))
	for _, row := range cells {
		for i, cell := range row {
			widths[i] = max(widths[i], len([]rune(cell)))
		}
	}
	render := func(row []string) string {
		padded := make([]string, len(row))
		for i, cell := range row {
			padded[i] = cell + strings.Repeat(" ", widths[i]-len([]rune(cell)))
		}
		return strings.TrimRight(strings.Join(padded, " | "), " ")
	}
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strings.Repeat("-", w)
	}
	lines := []string{render(cells[0]), strings.Join(parts, "-+-")}
	for _, row := range cells[1:] {
		lines = append(lines, render(row))
	}
	return lines
}

// StoredRunsText spells how many runs the tool stored of the configuration, and
// over how many snapshots when one summarises several.
func (c *ConfigurationResults) StoredRunsText() string {
	runs := c.StoredRuns()
	if len(c.Snapshots) == 0 {
		return "no stored run"
	}
	if runs == int64(len(c.Snapshots)) {
		return fmt.Sprintf("%d stored run(s)", runs)
	}
	return fmt.Sprintf("%d stored run(s) over %d snapshot(s)", runs, len(c.Snapshots))
}
