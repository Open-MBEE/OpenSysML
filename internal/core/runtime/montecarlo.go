package runtime

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// A Monte Carlo is a sweep over nothing but the seed: one ordinary run of a
// behavior per row, each drawing its modeled randomness from a seed of its own
// derived from the one given, and a table of what the runs observed.

// ErrSweepRuns reports a Monte Carlo that runs nothing or states a range too.
var ErrSweepRuns = errors.New("invalid run request")

// RunParam names the column a Monte Carlo table numbers its runs in.
const RunParam = "run"

// runSeedStride spaces the run seeds before mixing, an odd constant so every run
// of one Monte Carlo lands on a different input of the mixer.
const runSeedStride uint64 = 0x9E3779B97F4A7C15

// RunSeed is the model seed run i (from 1) of a Monte Carlo seeded from seed
// draws under: SplitMix64's finalizer over seed + i·stride, a bijection, so two
// runs of one Monte Carlo never share a seed, and run i is the same on every platform.
func RunSeed(seed uint64, i int64) uint64 {
	z := seed + unsignedInt(i)*runSeedStride
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// MonteCarloPlan is the plan of runs runs seeded from seed: one row per run and
// no ranges.
func MonteCarloPlan(runs int64, seed uint64) SweepPlan {
	return SweepPlan{Runs: runs, Seed: seed}
}

// runBindings is one row per run, binding RunParam to the run's number from 1;
// the run's seed is RunSeed of the plan's, which the run derives when it starts.
func (ctx *Context) runBindings(plan SweepPlan, limit int64) ([][]SweepBinding, error) {
	if len(plan.Ranges) > 0 {
		return nil, fmt.Errorf("%w: a Monte Carlo runs one behavior as declared; it states no range", ErrSweepRuns)
	}
	if plan.Sampled {
		return nil, fmt.Errorf("%w: a Monte Carlo draws its seeds from the seed given; it samples no range", ErrSweepRuns)
	}
	if plan.Runs < 1 {
		return nil, fmt.Errorf("%w: %d run(s) runs nothing; ask for at least one", ErrSweepRuns, plan.Runs)
	}
	if plan.Runs > limit {
		return nil, ctx.sweepBudgetError(limit)
	}
	rows := make([][]SweepBinding, 0, plan.Runs)
	for i := int64(1); i <= plan.Runs; i++ {
		number := Value{Kind: ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: i}}
		rows = append(rows, []SweepBinding{{Param: RunParam, Value: number}})
	}
	return rows, nil
}

// RunNumber is the number of the run a Monte Carlo row is, from its bindings.
func RunNumber(bindings []SweepBinding) (int64, bool) {
	for _, b := range bindings {
		if b.Param == RunParam && b.Value.Kind == ValConst && b.Value.Const.Kind == semantics.ValInt {
			return b.Value.Const.Int, true
		}
	}
	return 0, false
}

// Distribution summarises the numbers one observable took over the runs of a
// Monte Carlo: their extremes and mean, the nearest-rank median and 90th
// percentile, and a histogram of at most HistogramBins equal-width bins.
// Integral marks a distribution of whole numbers, whose bins cover whole numbers.
type Distribution struct {
	Count     int
	Integral  bool
	Min       float64
	Mean      float64
	Max       float64
	P50       float64
	P90       float64
	Histogram []HistogramBin
}

// HistogramBin is one bin of a histogram and how many runs fell in it: the
// numbers from Lo up to Hi, Hi included in the last bin and in every bin of an
// integral distribution.
type HistogramBin struct {
	Lo, Hi float64
	Count  int
}

// HistogramBins is the most bins a histogram has.
const HistogramBins = 8

// Distribute summarises values, whose order does not matter; nil for none. The
// percentiles are nearest-rank: the k-th smallest for k = ⌈p·n⌉, so each is a value
// some run took. A histogram over one value is one bin; over several its bins cut
// the range equally, the last taking the maximum. integral bins whole numbers by
// whole widths, one number per bin when the range allows.
func Distribute(values []float64, integral bool) *Distribution {
	if len(values) == 0 {
		return nil
	}
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	n := len(sorted)
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	d := &Distribution{
		Count:    n,
		Integral: integral,
		Min:      sorted[0],
		Mean:     sum / float64(n),
		Max:      sorted[n-1],
		P50:      nearestRank(sorted, 0.5),
		P90:      nearestRank(sorted, 0.9),
	}
	if integral {
		d.Histogram = integralHistogram(sorted)
	} else {
		d.Histogram = histogram(sorted)
	}
	return d
}

// nearestRank is the p-quantile of sorted by nearest rank.
func nearestRank(sorted []float64, p float64) float64 {
	k := int(math.Ceil(p * float64(len(sorted))))
	if k < 1 {
		k = 1
	}
	return sorted[k-1]
}

// histogram bins sorted into at most HistogramBins equal-width bins.
func histogram(sorted []float64) []HistogramBin {
	lo, hi := sorted[0], sorted[len(sorted)-1]
	if lo == hi || math.IsInf(hi-lo, 0) {
		return []HistogramBin{{Lo: lo, Hi: hi, Count: len(sorted)}}
	}
	bins := make([]HistogramBin, HistogramBins)
	width := (hi - lo) / HistogramBins
	for i := range bins {
		bins[i].Lo = lo + float64(i)*width
		bins[i].Hi = lo + float64(i+1)*width
	}
	bins[len(bins)-1].Hi = hi
	for _, v := range sorted {
		i := int((v - lo) / width)
		if i >= HistogramBins {
			i = HistogramBins - 1
		}
		bins[i].Count++
	}
	return bins
}

// integralHistogram bins sorted whole numbers into at most HistogramBins bins of
// one whole width each, both bounds of a bin included.
func integralHistogram(sorted []float64) []HistogramBin {
	lo, hi := sorted[0], sorted[len(sorted)-1]
	span := hi - lo + 1
	if math.IsInf(span, 0) || math.IsNaN(span) {
		return []HistogramBin{{Lo: lo, Hi: hi, Count: len(sorted)}}
	}
	width := math.Ceil(span / HistogramBins)
	count := int(math.Ceil(span / width))
	bins := make([]HistogramBin, count)
	for i := range bins {
		bins[i].Lo = lo + float64(i)*width
		bins[i].Hi = min(lo+float64(i+1)*width-1, hi)
	}
	for _, v := range sorted {
		i := int((v - lo) / width)
		if i >= count {
			i = count - 1
		}
		bins[i].Count++
	}
	return bins
}
