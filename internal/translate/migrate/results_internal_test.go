package migrate

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
)

// A summary's statistics carry the deviation and out-of-specification count only
// when the snapshot records them, and refuse a count that is no count of the runs.
func TestMonteCarloStatisticsCarryWhatIsRecorded(t *testing.T) {
	values := map[string]float64{"t": 3.5}
	cases := []struct {
		name    string
		summary map[string]float64
		want    *simresults.Statistics
		note    string
	}{
		{"whole", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5, monteCarloDeviation: 0.5, monteCarloOutOfSpec: 1},
			&simresults.Statistics{Observable: "t", Runs: 4, Mean: 3.5, Deviation: simresults.Real(0.5), OutOfSpec: simresults.Count(1)}, ""},
		{"zero deviation", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5, monteCarloDeviation: 0},
			&simresults.Statistics{Observable: "t", Runs: 4, Mean: 3.5, Deviation: simresults.Real(0)}, ""},
		{"no deviation", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5},
			&simresults.Statistics{Observable: "t", Runs: 4, Mean: 3.5}, ""},
		{"none out of specification", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5, monteCarloOutOfSpec: 0},
			&simresults.Statistics{Observable: "t", Runs: 4, Mean: 3.5, OutOfSpec: simresults.Count(0)}, ""},
		{"more out of specification than runs", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5, monteCarloOutOfSpec: 5},
			nil, "record a MonteCarloAnalysis::OutOfSpec of 5 over 4 runs, which is no count of them, so they hold no statistics"},
		{"fractional out of specification", map[string]float64{monteCarloRuns: 4, monteCarloMean: 3.5, monteCarloOutOfSpec: 1.5},
			nil, "record a MonteCarloAnalysis::OutOfSpec of 1.5 over 4 runs, which is no count of them, so they hold no statistics"},
	}
	for _, c := range cases {
		got, note, foreign := monteCarloStatistics("t", c.summary, false, values)
		if !reflect.DeepEqual(got, c.want) || note != c.note || foreign {
			t.Errorf("%s: statistics %+v, note %q, foreign %v; want %+v, %q", c.name, got, note, foreign, c.want, c.note)
		}
	}
}
