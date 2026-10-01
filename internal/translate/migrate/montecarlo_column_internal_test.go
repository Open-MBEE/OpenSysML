package migrate

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// A column over a MonteCarloAnalysis statistic is a member-path column on the
// row's 'Monte Carlo' analysis, captioned by the statistic's v1 name; a name
// the analysis records nothing under stays a note, as does a statistic no
// listed instance records.
func TestMonteCarloColumnIsAMemberPath(t *testing.T) {
	recorded := &sysmlv1.Element{Name: "a run"}
	m := &migration{mcRecordedDone: true, mcRecorded: []*sysmlv1.Element{recorded}}
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
		got := m.monteCarloColumn(c.stat, rowSet{whole: true})
		if got.key != c.key || got.caption != c.caption || got.path != c.path || got.why != c.why {
			t.Errorf("%s: %+v; want key %q caption %q path %v why %q",
				c.stat, got, c.key, c.caption, c.path, c.why)
		}
	}
}

// A statistic column reads only when an instance the rows admit records it;
// none recorded means the column would read nothing wherever it stands.
func TestMonteCarloColumnRequiresARecordedInstance(t *testing.T) {
	m := &migration{mcRecordedDone: true}
	got := m.monteCarloColumn(monteCarloRuns, rowSet{whole: true})
	want := "the column's MonteCarloAnalysis::N is recorded by no instance the table lists, so the column would read nothing"
	if got.path || got.why != want {
		t.Errorf("no recording individual: %+v; want the recorded-by-none note", got)
	}
}
