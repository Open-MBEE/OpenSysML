package migrate

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/xmi/sysmlv1"
)

// A column over a MonteCarloAnalysis statistic is a member-path column on the
// row's 'Monte Carlo' analysis, captioned by the statistic's v1 name; a name
// the analysis records nothing under stays a note.
func TestMonteCarloColumnIsAMemberPath(t *testing.T) {
	m := &migration{mcRecordedDone: true, mcRecorded: []*monteCarloCase{{}}}
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
		got := m.monteCarloColumn(c.stat, nil)
		if got.key != c.key || got.caption != c.caption || got.path != c.path || got.why != c.why {
			t.Errorf("%s: %+v; want key %q caption %q path %v why %q",
				c.stat, got, c.key, c.caption, c.path, c.why)
		}
	}
}

// A statistic column reads only when an instance the rows admit records it:
// a recorded analysis whose block is a row classifier, or any recorded
// analysis when the rows admit no particular classifier.
func TestMonteCarloColumnRequiresARecordedInstance(t *testing.T) {
	block := &sysmlv1.Element{Name: "Sweep Analysis"}
	other := &sysmlv1.Element{Name: "Other Analysis"}
	cases := []struct {
		name        string
		recorded    []*monteCarloCase
		classifiers []*sysmlv1.Element
		want        columnSource
	}{
		{"the block is a row classifier",
			[]*monteCarloCase{{block: block}}, []*sysmlv1.Element{block},
			columnSource{key: "'Monte Carlo'.runs", caption: monteCarloRuns, path: true}},
		{"any recorded case when rows admit no particular classifier",
			[]*monteCarloCase{{block: block}}, nil,
			columnSource{key: "'Monte Carlo'.runs", caption: monteCarloRuns, path: true}},
		{"an unrelated row classifier",
			[]*monteCarloCase{{block: block}}, []*sysmlv1.Element{other},
			columnSource{why: "the column's MonteCarloAnalysis::N is recorded by no instance the table lists, so the column would read nothing"}},
		{"no instance records anything",
			nil, []*sysmlv1.Element{block},
			columnSource{why: "the column's MonteCarloAnalysis::N is recorded by no instance the table lists, so the column would read nothing"}},
	}
	for _, c := range cases {
		m := &migration{mcRecordedDone: true, mcRecorded: c.recorded}
		if got := m.monteCarloColumn(monteCarloRuns, c.classifiers); got != c.want {
			t.Errorf("%s: %+v; want %+v", c.name, got, c.want)
		}
	}
}
