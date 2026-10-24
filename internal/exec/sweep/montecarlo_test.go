package sweep

import (
	"fmt"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
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
			run := runtime.RunSeed(seed, i)
			if run != runtime.RunSeed(seed, i) {
				t.Fatalf("runtime.RunSeed(%d, %d) differs between calls", seed, i)
			}
			key := seen[run]
			if key != "" {
				t.Fatalf("runtime.RunSeed(%d, %d) = %d, which %s also derives", seed, i, run, key)
			}
			seen[run] = fmt.Sprintf("seed %d run %d", seed, i)
		}
	}
}

// A Monte Carlo plan makes one row per run, numbered from 1 in RunParam, and
// nothing above the run limit.
func TestMonteCarloPlanNumbersItsRuns(t *testing.T) {
	ctx, _ := analysisFixture(t, sweepRobustnessModel)
	rows, err := sweepBindings(ctx, MonteCarloPlan(3, 9), 10)
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
	if _, err := sweepBindings(ctx, MonteCarloPlan(11, 9), 10); err == nil {
		t.Error("11 runs under a limit of 10 were planned")
	}
	if _, ok := RunNumber(nil); ok {
		t.Error("no bindings number a run")
	}
}
