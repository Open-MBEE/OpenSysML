package analysis

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/exec/sweep"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// ClockObservable names the observable every run of a Monte Carlo reports
// without being asked: the clock reading the run completed at.
const ClockObservable = "clock"

// OutputColumns names a table's outputs in the order its rows first produced them.
func OutputColumns(table sweep.SweepTable) []string {
	var outputs []string
	seen := make(map[string]bool)
	for _, row := range table.Rows {
		for _, out := range row.Outputs {
			if !seen[out.Name] {
				seen[out.Name] = true
				outputs = append(outputs, out.Name)
			}
		}
	}
	return outputs
}

// Observe is what one run reports: the observables named, in that order, from
// the clock and the outcome's features; every feature in name order and the
// clock when none are named. The clock's name is reserved, so a feature called
// clock is never reported; a feature the run does not hold is left out.
func Observe(ctx *runtime.Context, outcome runtime.Outcome, observables []string) []runtime.CalcOutputValue {
	names := observables
	if len(names) == 0 {
		names = make([]string, 0, len(outcome.Outputs)+1)
		for name := range outcome.Outputs {
			if name != ClockObservable {
				names = append(names, name)
			}
		}
		slices.Sort(names)
		names = append(names, ClockObservable)
	}
	outputs := make([]runtime.CalcOutputValue, 0, len(names))
	for _, name := range names {
		if name == ClockObservable {
			outputs = append(outputs, runtime.CalcOutputValue{Name: name, Value: ctx.ClockValue()})
			continue
		}
		if value, ok := outcome.Outputs[name]; ok {
			outputs = append(outputs, runtime.CalcOutputValue{Name: name, Value: value})
		}
	}
	return outputs
}

// Unobserved is the error of an observable no completed run produced: the
// action holds no such feature, or holds one no run gave a value. Nothing is
// held against a table whose every run failed.
func Unobserved(table sweep.SweepTable, observables []string) error {
	produced := make(map[string]bool)
	completed := 0
	for _, row := range table.Rows {
		if row.Err != nil {
			continue
		}
		completed++
		for _, out := range row.Outputs {
			produced[out.Name] = true
		}
	}
	if completed == 0 {
		return nil
	}
	for _, name := range observables {
		if !produced[name] {
			return fmt.Errorf("%w: no completed run of %s produced a value named %s (the clock is observed as %s)",
				sweep.ErrSweepRuns, table.Target, name, ClockObservable)
		}
	}
	return nil
}

// DistributionLines summarise each observable over the completed runs: its
// extremes, mean and percentiles where the runs produced numbers, the count of
// each distinct value otherwise, and a histogram of the numbers.
func DistributionLines(table sweep.SweepTable, text func(*runtime.Context, runtime.Value) string) []string {
	var lines []string
	for _, name := range OutputColumns(table) {
		lines = append(lines, Summarize(table, name, text).Lines()...)
	}
	return lines
}

// CountedText is one distinct non-numeric value an observable took, spelled
// through its run's context, and how many completed runs produced it.
type CountedText struct {
	Text  string
	Count int
}

// ObservableSummary is one observable over a table's completed runs: the
// distribution of the numbers it produced, in Unit, and the count of each
// distinct value that is not a number. MixedUnits reports numbers in more than
// one unit, which have no distribution.
type ObservableSummary struct {
	Name         string
	Unit         string
	Numbers      int
	MixedUnits   bool
	Distribution *runtime.Distribution
	Counted      []CountedText
}

// Summarize reads one observable off the completed rows; text spells a value
// that is not a number through the row's context.
func Summarize(table sweep.SweepTable, name string, text func(*runtime.Context, runtime.Value) string) ObservableSummary {
	var numbers []semantics.Value
	var unit string
	units := make(map[string]bool)
	counts := make(map[string]int)
	var order []string
	for _, row := range table.Rows {
		if row.Err != nil {
			continue
		}
		for _, out := range row.Outputs {
			if out.Name != name {
				continue
			}
			if n, u, ok := runtime.Magnitude(out.Value); ok {
				numbers = append(numbers, n)
				unit = u
				units[unit] = true
				continue
			}
			spelled := text(row.Context, out.Value)
			if counts[spelled] == 0 {
				order = append(order, spelled)
			}
			counts[spelled]++
		}
	}
	summary := ObservableSummary{Name: name, Unit: unit, Numbers: len(numbers)}
	if len(units) > 1 {
		summary.MixedUnits = true
		summary.Unit = ""
		return summary
	}
	summary.Distribution = runtime.Distribute(numbers)
	slices.Sort(order)
	for _, spelled := range order {
		summary.Counted = append(summary.Counted, CountedText{Text: spelled, Count: counts[spelled]})
	}
	return summary
}

// Lines is the summary as the CLI prints it: the distribution line and its
// histogram, then the distinct values with their counts.
func (s ObservableSummary) Lines() []string {
	if s.MixedUnits {
		return []string{fmt.Sprintf("%s: %d run(s) produced numbers in more than one unit; no distribution", s.Name, s.Numbers)}
	}
	var lines []string
	if d := s.Distribution; d != nil {
		lines = append(lines, distributionLine(s.Name, d, s.Unit))
		lines = append(lines, histogramLines(d, s.Unit)...)
	}
	if len(s.Counted) > 0 {
		parts := make([]string, len(s.Counted))
		for i, c := range s.Counted {
			parts[i] = fmt.Sprintf("%s ×%d", c.Text, c.Count)
		}
		lines = append(lines, fmt.Sprintf("%s: %s", s.Name, strings.Join(parts, ", ")))
	}
	return lines
}

// distributionLine is the one-line summary of a distribution, its numbers in unit.
func distributionLine(name string, d *runtime.Distribution, unit string) string {
	return fmt.Sprintf("%s: %d run(s), min %s, mean %s, max %s, p50 %s, p90 %s", name, d.Count,
		WithUnit(d.Min, unit), WithUnit(DrawnMean(d), unit), WithUnit(d.Max, unit), WithUnit(d.P50, unit), WithUnit(d.P90, unit))
}

// DrawnMean is a distribution's mean as the Real it is.
func DrawnMean(d *runtime.Distribution) semantics.Value {
	return semantics.Value{Kind: semantics.ValReal, Real: d.Mean}
}

// histogramLines render a histogram one bin per line: the bin's bounds to four
// significant digits (one number when they meet), a bar scaled to the runs,
// and the count, the columns aligned.
func histogramLines(d *runtime.Distribution, unit string) []string {
	const barWidth = 20
	labels := make([]string, len(d.Histogram))
	width := 0
	for i, bin := range d.Histogram {
		labels[i] = binLabel(bin, unit)
		width = max(width, len([]rune(labels[i])))
	}
	lines := make([]string, len(d.Histogram))
	for i, bin := range d.Histogram {
		bar := strings.Repeat("#", (bin.Count*barWidth+d.Count-1)/d.Count)
		lines[i] = fmt.Sprintf("  %-*s %-*s %d", width, labels[i], barWidth, bar, bin.Count)
	}
	return lines
}

// binLabel spells a bin's bounds, `lo..hi` in the unit, or the one number both are.
func binLabel(bin runtime.HistogramBin, unit string) string {
	if bin.Lo == bin.Hi {
		return WithUnit(bin.Lo, unit)
	}
	label := compactNumber(bin.Lo) + ".." + compactNumber(bin.Hi)
	if unit != "" {
		label += " [" + unit + "]"
	}
	return label
}

// compactNumber spells a bin bound: an Integer in full, a Real to four significant digits.
func compactNumber(x semantics.Value) string {
	if x.Kind == semantics.ValInt {
		return x.FormatInt()
	}
	return strconv.FormatFloat(x.Real, 'g', 4, 64)
}

// WithUnit spells a number as the table spells it, in unit, bare when it has none.
func WithUnit(x semantics.Value, unit string) string {
	text := semantics.FormatConst(x)
	if unit == "" {
		return text
	}
	return text + " [" + unit + "]"
}
