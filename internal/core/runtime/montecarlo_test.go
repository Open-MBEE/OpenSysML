package runtime

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
)

// The seed of each run is a function of the Monte Carlo's seed and the run's
// number alone: the same pair derives the same seed, and the runs of one Monte
// Carlo — and of neighbouring seeds — all differ.
func TestRunSeedIsDistinctPerRunAndReproducible(t *testing.T) {
	seen := make(map[uint64]string)
	for _, seed := range []uint64{0, 1, 2, math.MaxUint64} {
		for i := int64(1); i <= 1000; i++ {
			run := RunSeed(seed, i)
			if run != RunSeed(seed, i) {
				t.Fatalf("RunSeed(%d, %d) differs between calls", seed, i)
			}
			key := seen[run]
			if key != "" {
				t.Fatalf("RunSeed(%d, %d) = %d, which %s also derives", seed, i, run, key)
			}
			seen[run] = fmt.Sprintf("seed %d run %d", seed, i)
		}
	}
}

// A Monte Carlo plan makes one row per run, numbered from 1 in RunParam, and
// nothing above the run limit.
func TestMonteCarloPlanNumbersItsRuns(t *testing.T) {
	ctx, _ := analysisFixture(t, sweepRobustnessModel)
	rows, err := ctx.sweepBindings(MonteCarloPlan(3, 9), 10)
	if err != nil {
		t.Fatal(err)
	}
	var numbers []int64
	for _, row := range rows {
		n, ok := RunNumber(row)
		if !ok || len(row) != 1 {
			t.Fatalf("row %v numbers no run", row)
		}
		numbers = append(numbers, n)
	}
	if want := []int64{1, 2, 3}; !reflect.DeepEqual(numbers, want) {
		t.Errorf("runs numbered %v, want %v", numbers, want)
	}
	if _, err := ctx.sweepBindings(MonteCarloPlan(11, 9), 10); err == nil {
		t.Error("11 runs under a limit of 10 were planned")
	}
	if _, ok := RunNumber(nil); ok {
		t.Error("no bindings number a run")
	}
}

// reals and ints are values for Distribute.
func reals(xs ...float64) []semantics.Value {
	vs := make([]semantics.Value, len(xs))
	for i, x := range xs {
		vs[i] = drawnReal(x)
	}
	return vs
}

func ints(ns ...int64) []semantics.Value {
	vs := make([]semantics.Value, len(ns))
	for i, n := range ns {
		vs[i] = drawnInt(n)
	}
	return vs
}

// Distribute reports the extremes, the mean, and nearest-rank percentiles, each
// a value some run took, and bins the range into equal widths.
func TestDistributeSummarisesTheRuns(t *testing.T) {
	d := Distribute(reals(5, 1, 4, 2, 3, 10, 6, 7, 8, 9))
	if d.Count != 10 || d.Integral || d.Min != drawnReal(1) || d.Max != drawnReal(10) || d.Mean != 5.5 {
		t.Errorf("count %d integral %v min %v max %v mean %v", d.Count, d.Integral, d.Min, d.Max, d.Mean)
	}
	if d.P50 != drawnReal(5) || d.P90 != drawnReal(9) {
		t.Errorf("p50 %v p90 %v, want the 5th and the 9th smallest", d.P50, d.P90)
	}
	if len(d.Histogram) != HistogramBins {
		t.Fatalf("%d bins, want %d", len(d.Histogram), HistogramBins)
	}
	total := 0
	for i, bin := range d.Histogram {
		total += bin.Count
		if i > 0 && bin.Lo != d.Histogram[i-1].Hi {
			t.Errorf("bin %d starts at %v, the one before ends at %v", i, bin.Lo, d.Histogram[i-1].Hi)
		}
	}
	if total != 10 || d.Histogram[0].Lo != drawnReal(1) || d.Histogram[HistogramBins-1].Hi != drawnReal(10) {
		t.Errorf("bins %v cover %d run(s), want all 10 from 1 to 10", d.Histogram, total)
	}
	if Distribute(nil) != nil {
		t.Error("no values have a distribution")
	}
}

// Integers bin by whole widths, one number per bin when the range allows and
// both ends of a bin included; a single value is one bin; a Real among the
// Integers makes the distribution one over Reals.
func TestDistributeBinsIntegersWhole(t *testing.T) {
	d := Distribute(ints(1, 2, 2, 3, 6))
	if !d.Integral || d.Min != drawnInt(1) || d.Max != drawnInt(6) || d.P50 != drawnInt(2) || d.P90 != drawnInt(6) || d.Mean != 2.8 {
		t.Errorf("integral %v min %v max %v p50 %v p90 %v mean %v", d.Integral, d.Min, d.Max, d.P50, d.P90, d.Mean)
	}
	want := []HistogramBin{
		{drawnInt(1), drawnInt(1), 1}, {drawnInt(2), drawnInt(2), 2}, {drawnInt(3), drawnInt(3), 1},
		{drawnInt(4), drawnInt(4), 0}, {drawnInt(5), drawnInt(5), 0}, {drawnInt(6), drawnInt(6), 1},
	}
	if !reflect.DeepEqual(d.Histogram, want) {
		t.Errorf("bins %v, want %v", d.Histogram, want)
	}
	wide := Distribute(ints(0, 100))
	if len(wide.Histogram) != 8 || wide.Histogram[0] != (HistogramBin{drawnInt(0), drawnInt(12), 1}) || wide.Histogram[7] != (HistogramBin{drawnInt(91), drawnInt(100), 1}) {
		t.Errorf("bins over 0..100: %v", wide.Histogram)
	}
	one := Distribute(reals(4, 4, 4))
	if len(one.Histogram) != 1 || one.Histogram[0] != (HistogramBin{drawnReal(4), drawnReal(4), 3}) {
		t.Errorf("bins over one value: %v", one.Histogram)
	}
	mixed := Distribute([]semantics.Value{drawnInt(1), drawnReal(2.5)})
	if mixed.Integral || mixed.Min != drawnReal(1) || mixed.Max != drawnReal(2.5) {
		t.Errorf("a Real among Integers: integral %v min %v max %v", mixed.Integral, mixed.Min, mixed.Max)
	}
}

// Reals at the edges of the representable range are summarised as any others: the
// mean of finite observations is finite whatever their sum would be, and a span too
// narrow to divide into eight widths bins each distinct value on its own.
func TestDistributeRealsAtTheEdgesOfTheRange(t *testing.T) {
	huge := Distribute(reals(1e308, 1e308))
	if huge.Mean != 1e308 {
		t.Errorf("mean %v, want 1e308: the sum overflows, the mean does not", huge.Mean)
	}
	if mixed := Distribute(reals(math.MaxFloat64, -math.MaxFloat64, 1e308, -1e308)); mixed.Mean != 0 {
		t.Errorf("mean %v, want 0 over values of both signs", mixed.Mean)
	}
	if d := Distribute(reals(0.5, 0.25, 0.25)); d.Mean != 1.0/3 {
		t.Errorf("mean %v, want 1/3 rounded once", d.Mean)
	}
	if inf := Distribute(reals(1, math.Inf(1))); !math.IsInf(inf.Mean, 1) {
		t.Errorf("mean %v, want +Inf where an observation is", inf.Mean)
	}
	tiny := math.SmallestNonzeroFloat64
	narrow := Distribute(reals(0, tiny, tiny, 2*tiny))
	want := []HistogramBin{{drawnReal(0), drawnReal(0), 1}, {drawnReal(tiny), drawnReal(tiny), 2}, {drawnReal(2 * tiny), drawnReal(2 * tiny), 1}}
	if !reflect.DeepEqual(narrow.Histogram, want) {
		t.Errorf("bins %v, want one a distinct value %v", narrow.Histogram, want)
	}
	if narrow.Min != drawnReal(0) || narrow.Max != drawnReal(2*tiny) || narrow.Mean != tiny {
		t.Errorf("min %v mean %v max %v", narrow.Min, narrow.Mean, narrow.Max)
	}
	wide := Distribute(reals(-math.MaxFloat64, math.MaxFloat64, 0))
	if len(wide.Histogram) != 1 || wide.Histogram[0].Count != 3 || wide.Mean != 0 {
		t.Errorf("over the whole Real range: bins %v mean %v, want one bin and a mean of 0", wide.Histogram, wide.Mean)
	}
}

// Integers beyond 2^53, which a Real cannot tell apart, stay distinct in every
// statistic but the mean, and the whole Integer range bins without overflowing.
func TestDistributeKeepsLargeIntegersExact(t *testing.T) {
	const big = int64(1) << 53
	d := Distribute(ints(big+1, big))
	if d.Min != drawnInt(big) || d.Max != drawnInt(big+1) || d.P50 != drawnInt(big) || d.P90 != drawnInt(big+1) {
		t.Errorf("min %v max %v p50 %v p90 %v, want %d and %d apart", d.Min, d.Max, d.P50, d.P90, big, big+1)
	}
	if d.Mean != float64(big) {
		t.Errorf("mean %v, want the nearest Real to %d.5", d.Mean, big)
	}
	want := []HistogramBin{{drawnInt(big), drawnInt(big), 1}, {drawnInt(big + 1), drawnInt(big + 1), 1}}
	if !reflect.DeepEqual(d.Histogram, want) {
		t.Errorf("bins %v, want %v", d.Histogram, want)
	}
	whole := Distribute(ints(math.MaxInt64, math.MinInt64, 0))
	if whole.Mean != -1.0/3 || whole.Min != drawnInt(math.MinInt64) || whole.Max != drawnInt(math.MaxInt64) {
		t.Errorf("over the whole Integer range: min %v mean %v max %v, want a mean of -1/3", whole.Min, whole.Mean, whole.Max)
	}
	if len(whole.Histogram) != 1 || whole.Histogram[0].Count != 3 {
		t.Errorf("bins over the whole Integer range: %v", whole.Histogram)
	}
	extremes := Distribute(ints(math.MaxInt64, math.MinInt64+1))
	if n := len(extremes.Histogram); n != HistogramBins || extremes.Histogram[0].Lo != drawnInt(math.MinInt64+1) || extremes.Histogram[n-1].Hi != drawnInt(math.MaxInt64) {
		t.Errorf("bins over all but one Integer: %v", extremes.Histogram)
	}
	if extremes.Mean != 0 {
		t.Errorf("mean %v, want 0: the sum is exact before it rounds", extremes.Mean)
	}
}
