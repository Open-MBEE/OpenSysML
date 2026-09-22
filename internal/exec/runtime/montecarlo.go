package runtime

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"slices"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
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
	return SweepPlan{Runs: runs, Seed: seed, MonteCarlo: true}
}

// SeedlessMonteCarloPlan is the plan of runs runs seeded from nothing: a fixed draw
// policy resolves every RandomFunctions call, and a weighted decision is an unseeded draw.
func SeedlessMonteCarloPlan(runs int64) SweepPlan {
	return SweepPlan{Runs: runs, MonteCarlo: true, Seedless: true}
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

// Distribution summarises one observable over the runs of a Monte Carlo: extremes,
// mean, sample standard deviation, nearest-rank p50/p90 and a histogram, exact on
// Integers save the Real mean and deviation.
type Distribution struct {
	Count    int
	Integral bool
	Min      semantics.Value
	Mean     float64
	// Deviation is the sample standard deviation about Mean, 0 for fewer than two runs.
	Deviation float64
	Max       semantics.Value
	P50       semantics.Value
	P90       semantics.Value
	Histogram []HistogramBin
}

// HistogramBin is one bin of a histogram and how many runs fell in it: the
// numbers from Lo up to Hi, Hi included in the last bin and in every bin of an
// integral distribution.
type HistogramBin struct {
	Lo, Hi semantics.Value
	Count  int
}

// HistogramBins is the most bins a histogram has.
const HistogramBins = 8

// Distribute summarises numbers in any order, nil for none: all Integers make an
// integral distribution computed on int64, a Real among them one over Reals.
func Distribute(numbers []semantics.Value) *Distribution {
	if len(numbers) == 0 {
		return nil
	}
	ints := make([]int64, 0, len(numbers))
	for _, v := range numbers {
		if v.Kind != semantics.ValInt {
			return distributeReals(numbers)
		}
		ints = append(ints, v.Int)
	}
	return distributeInts(ints)
}

// distributeInts summarises Integers on the Integers themselves; only the mean
// rounds, once, after an exact sum.
func distributeInts(values []int64) *Distribution {
	sorted := slices.Clone(values)
	slices.Sort(sorted)
	n := len(sorted)
	sum := new(big.Int)
	for _, v := range sorted {
		sum.Add(sum, big.NewInt(v))
	}
	mean, _ := new(big.Rat).SetFrac(sum, big.NewInt(int64(n))).Float64()
	reals := make([]float64, n)
	for i, v := range sorted {
		reals[i] = float64(v)
	}
	return &Distribution{
		Count:     n,
		Integral:  true,
		Min:       drawnInt(sorted[0]),
		Mean:      mean,
		Deviation: deviationOf(reals, mean),
		Max:       drawnInt(sorted[n-1]),
		P50:       drawnInt(nearestRank(sorted, 0.5)),
		P90:       drawnInt(nearestRank(sorted, 0.9)),
		Histogram: integralHistogram(sorted),
	}
}

// distributeReals summarises numbers as Reals; the mean rounds once, after an exact
// sum, so finite observations never overflow it.
func distributeReals(numbers []semantics.Value) *Distribution {
	sorted := make([]float64, len(numbers))
	for i, v := range numbers {
		sorted[i] = v.AsReal()
	}
	slices.Sort(sorted)
	n := len(sorted)
	mean := meanOf(sorted)
	return &Distribution{
		Count:     n,
		Min:       drawnReal(sorted[0]),
		Mean:      mean,
		Deviation: deviationOf(sorted, mean),
		Max:       drawnReal(sorted[n-1]),
		P50:       drawnReal(nearestRank(sorted, 0.5)),
		P90:       drawnReal(nearestRank(sorted, 0.9)),
		Histogram: histogram(sorted),
	}
}

// meanOf is the mean of values, exact until it rounds to a Real; a value that is
// no finite number carries into the mean as Real arithmetic would carry it.
func meanOf(values []float64) float64 {
	sum := new(big.Rat)
	for _, v := range values {
		if math.IsInf(v, 0) || math.IsNaN(v) {
			return realSum(values) / float64(len(values))
		}
		sum.Add(sum, new(big.Rat).SetFloat64(v))
	}
	mean, _ := sum.Quo(sum, big.NewRat(int64(len(values)), 1)).Float64()
	return mean
}

// deviationOf is the sample standard deviation of values about their mean, 0 for fewer than two.
func deviationOf(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	var sum float64
	for _, v := range values {
		d := v - mean
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(values)-1))
}

// realSum is the sum of values in Real arithmetic.
func realSum(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum
}

// drawnInt is n as an Integer value.
func drawnInt(n int64) semantics.Value {
	return semantics.Value{Kind: semantics.ValInt, Int: n}
}

// nearestRank is the p-quantile of sorted by nearest rank.
func nearestRank[T any](sorted []T, p float64) T {
	k := int(math.Ceil(p * float64(len(sorted))))
	if k < 1 {
		k = 1
	}
	return sorted[k-1]
}

// histogram bins sorted Reals into at most HistogramBins equal-width bins; a span too
// narrow to divide into Real widths bins each distinct value on its own.
func histogram(sorted []float64) []HistogramBin {
	lo, hi := sorted[0], sorted[len(sorted)-1]
	if lo == hi || math.IsInf(hi-lo, 0) {
		return []HistogramBin{{Lo: drawnReal(lo), Hi: drawnReal(hi), Count: len(sorted)}}
	}
	width := (hi - lo) / HistogramBins
	if width == 0 {
		return distinctHistogram(sorted)
	}
	bins := make([]HistogramBin, HistogramBins)
	for i := range bins {
		bins[i].Lo = drawnReal(lo + float64(i)*width)
		bins[i].Hi = drawnReal(lo + float64(i+1)*width)
	}
	bins[len(bins)-1].Hi = drawnReal(hi)
	for _, v := range sorted {
		bins[min(int((v-lo)/width), HistogramBins-1)].Count++
	}
	return bins
}

// distinctHistogram bins sorted Reals one distinct value to a bin.
func distinctHistogram(sorted []float64) []HistogramBin {
	var bins []HistogramBin
	for _, v := range sorted {
		if n := len(bins); n > 0 && bins[n-1].Lo == drawnReal(v) {
			bins[n-1].Count++
			continue
		}
		bins = append(bins, HistogramBin{Lo: drawnReal(v), Hi: drawnReal(v), Count: 1})
	}
	return bins
}

// integralHistogram bins sorted Integers by whole widths, both bounds included;
// offsets from the minimum are unsigned so the whole int64 range stays exact.
func integralHistogram(sorted []int64) []HistogramBin {
	lo, hi := sorted[0], sorted[len(sorted)-1]
	span := unsignedInt(hi) - unsignedInt(lo)
	if span == math.MaxUint64 {
		return []HistogramBin{{Lo: drawnInt(lo), Hi: drawnInt(hi), Count: len(sorted)}}
	}
	width := ceilDiv(span+1, HistogramBins)
	var bins []HistogramBin
	for from := uint64(0); from <= span; from += width {
		to := span
		if span-from >= width {
			to = from + width - 1
		}
		bins = append(bins, HistogramBin{
			Lo: drawnInt(signedInt(unsignedInt(lo) + from)),
			Hi: drawnInt(signedInt(unsignedInt(lo) + to)),
		})
		if to == span {
			break
		}
	}
	for _, v := range sorted {
		bin := min((unsignedInt(v)-unsignedInt(lo))/width, HistogramBins-1)
		bins[bin].Count++
	}
	return bins
}

// ceilDiv is ⌈n / d⌉ for d > 0, without overflowing near the top of uint64.
func ceilDiv(n, d uint64) uint64 {
	q := n / d
	if n%d != 0 {
		q++
	}
	return q
}
