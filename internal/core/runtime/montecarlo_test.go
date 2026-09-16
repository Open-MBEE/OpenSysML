package runtime

import (
	"fmt"
	"math"
	"reflect"
	"testing"
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

// Distribute reports the extremes, the mean, and nearest-rank percentiles, each
// a value some run took, and bins the range into equal widths.
func TestDistributeSummarisesTheRuns(t *testing.T) {
	d := Distribute([]float64{5, 1, 4, 2, 3, 10, 6, 7, 8, 9}, false)
	if d.Count != 10 || d.Min != 1 || d.Max != 10 || d.Mean != 5.5 {
		t.Errorf("count %d min %v max %v mean %v", d.Count, d.Min, d.Max, d.Mean)
	}
	if d.P50 != 5 || d.P90 != 9 {
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
	if total != 10 || d.Histogram[0].Lo != 1 || d.Histogram[HistogramBins-1].Hi != 10 {
		t.Errorf("bins %v cover %d run(s), want all 10 from 1 to 10", d.Histogram, total)
	}
	if Distribute(nil, false) != nil {
		t.Error("no values have a distribution")
	}
}

// Whole numbers bin by whole widths, one number per bin when the range allows and
// both ends of a bin included; a single value is one bin.
func TestDistributeBinsWholeNumbersWhole(t *testing.T) {
	d := Distribute([]float64{1, 2, 2, 3, 6}, true)
	want := []HistogramBin{{1, 1, 1}, {2, 2, 2}, {3, 3, 1}, {4, 4, 0}, {5, 5, 0}, {6, 6, 1}}
	if !reflect.DeepEqual(d.Histogram, want) {
		t.Errorf("bins %v, want %v", d.Histogram, want)
	}
	wide := Distribute([]float64{0, 100}, true)
	if len(wide.Histogram) != 8 || wide.Histogram[0] != (HistogramBin{0, 12, 1}) || wide.Histogram[7] != (HistogramBin{91, 100, 1}) {
		t.Errorf("bins over 0..100: %v", wide.Histogram)
	}
	one := Distribute([]float64{4, 4, 4}, false)
	if len(one.Histogram) != 1 || one.Histogram[0] != (HistogramBin{4, 4, 3}) {
		t.Errorf("bins over one value: %v", one.Histogram)
	}
}
