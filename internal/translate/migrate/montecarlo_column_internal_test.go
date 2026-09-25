package migrate

import "testing"

// A column over a MonteCarloAnalysis statistic is a member-path column on the
// row's 'Monte Carlo' analysis, captioned by the statistic's v1 name; a name
// the analysis records nothing under stays a note.
func TestMonteCarloColumnIsAMemberPath(t *testing.T) {
	cases := []struct {
		stat, key, caption, why string
		path                    bool
	}{
		{monteCarloRuns, "'Monte Carlo'.runs", monteCarloRuns, "", true},
		{monteCarloMean, "'Monte Carlo'.mean", monteCarloMean, "", true},
		{monteCarloDeviation, "'Monte Carlo'.deviation", monteCarloDeviation, "", true},
		{monteCarloOutOfSpec, "'Monte Carlo'.outOfSpec", monteCarloOutOfSpec, "", true},
		{"Efficiency", "", "",
			"the column's MonteCarloAnalysis::Efficiency is no statistic the analysis records", false},
	}
	for _, c := range cases {
		got := monteCarloColumn(c.stat)
		if got.key != c.key || got.caption != c.caption || got.path != c.path || got.why != c.why {
			t.Errorf("%s: %+v; want key %q caption %q path %v why %q",
				c.stat, got, c.key, c.caption, c.path, c.why)
		}
	}
}
