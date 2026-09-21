package migrate

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
)

// A summary's statistics carry the deviation and out-of-specification count only
// when the snapshot records them, and refuse a count that is no count of the runs.
// Another value equal to the Mean, beside the observable holding it or instead of it,
// neither reads as statistics of that value nor tells the snapshot apart.
func TestMonteCarloStatisticsCarryWhatIsRecorded(t *testing.T) {
	values := map[string]float64{"t": 3.5, "u": 3.5}
	cases := []struct {
		name    string
		summary map[string]float64
		want    *simresults.Statistics
		note    string
	}{
		{"whole", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5, monteCarloDeviation: 0.5, monteCarloOutOfSpec: 1},
			&simresults.Statistics{Observable: "t", Runs: 4, Mean: 3.5, Deviation: simresults.Real(0.5), OutOfSpec: simresults.Count(1)}, ""},
		{"mean the observable does not hold", map[string]float64{monteCarloRuns: 4, monteCarloMean: 4},
			nil, "record MonteCarloAnalysis statistics whose Mean no value of t holds, though the analysis binds the two, so the statistics are not read"},
		{"zero deviation", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5, monteCarloDeviation: 0},
			&simresults.Statistics{Observable: "t", Runs: 4, Mean: 3.5, Deviation: simresults.Real(0)}, ""},
		{"no deviation", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5},
			&simresults.Statistics{Observable: "t", Runs: 4, Mean: 3.5}, ""},
		{"negative deviation", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5, monteCarloDeviation: -0.5},
			nil, "record a MonteCarloAnalysis::Deviation of -0.5, which is no standard deviation, so they hold no statistics"},
		{"none out of specification", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5, monteCarloOutOfSpec: 0},
			&simresults.Statistics{Observable: "t", Runs: 4, Mean: 3.5, OutOfSpec: simresults.Count(0)}, ""},
		{"more out of specification than runs", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5, monteCarloOutOfSpec: 5},
			nil, "record a MonteCarloAnalysis::OutOfSpec of 5 over 4 runs, which is no count of them, so they hold no statistics"},
		{"fractional out of specification", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5, monteCarloOutOfSpec: 1.5},
			nil, "record a MonteCarloAnalysis::OutOfSpec of 1.5 over 4 runs, which is no count of them, so they hold no statistics"},
	}
	for _, c := range cases {
		got, note := monteCarloStatistics("t", c.summary, false, values)
		if !reflect.DeepEqual(got, c.want) || note != c.note {
			t.Errorf("%s: statistics %+v, note %q; want %+v, %q", c.name, got, note, c.want, c.note)
		}
	}
	got, note := monteCarloStatistics("t", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5}, false, map[string]float64{"u": 3.5})
	if want := "record MonteCarloAnalysis statistics whose Mean no value of t holds, though the analysis binds the two, so the statistics are not read"; got != nil || note != want {
		t.Errorf("another value alone holding the Mean: statistics %+v, note %q; want none, %q", got, note, want)
	}
}
