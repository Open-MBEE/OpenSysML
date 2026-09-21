package simresults

import (
	"reflect"
	"strings"
	"testing"
)

// mixed stores t run by run in two snapshots and summarised over eight in a
// third, whose t is the mean; u is stored run by run alone, in the summary too.
func mixed() ConfigurationResults {
	return ConfigurationResults{
		ID: "_c", Name: "'Group 0'", Analysis: "t", Observables: []string{"t", "u"},
		Snapshots: []Snapshot{
			{ID: "_r1", Values: map[string]float64{"t": 2, "u": 1}},
			{ID: "_r2", Values: map[string]float64{"t": 4}},
			{ID: "_sum", Values: map[string]float64{"t": 3.5, "u": 7}, Statistics: &Statistics{Observable: "t", Runs: 8, Mean: 3.5, Deviation: 0.5}},
		},
	}
}

// The values of an observable are those stored run by run: a summary's mean of
// it is no run's and is left out, while the summary's other values are runs'.
func TestValuesLeaveOutTheSummarisedMean(t *testing.T) {
	c := mixed()
	if got := c.Values("t"); !reflect.DeepEqual(got, []float64{2, 4}) {
		t.Errorf("Values(t) = %v, want the two stored run by run", got)
	}
	if got := c.Values("u"); !reflect.DeepEqual(got, []float64{1, 7}) {
		t.Errorf("Values(u) = %v, want the summary's u among them: it summarises t alone", got)
	}
	if got := c.Values("v"); got != nil {
		t.Errorf("Values(v) = %v, want none", got)
	}
}

// The summarised snapshots of an observable are those whose statistics are of it.
func TestSummarisedAreTheSnapshotsWithStatisticsOfTheObservable(t *testing.T) {
	c := mixed()
	if got := c.Summarised("t"); len(got) != 1 || got[0].ID != "_sum" {
		t.Errorf("Summarised(t) = %+v, want the summary alone", got)
	}
	if got := c.Summarised("u"); got != nil {
		t.Errorf("Summarised(u) = %+v, want none", got)
	}
}

// The runs a configuration's snapshots stand for count each summary's runs and
// every other snapshot once; the summary spells the difference from the
// snapshot count when there is one.
func TestStoredRunsCountSummarisedRuns(t *testing.T) {
	c := mixed()
	if got := c.StoredRuns(); got != 10 {
		t.Errorf("StoredRuns = %d, want 10", got)
	}
	r := Results{Source: "m.xmi", Configurations: []ConfigurationResults{c, {ID: "_d", Name: "'Group 1'"}}}
	if got, want := r.Summary(), "results of 2 run configuration(s): 1 with 3 stored snapshot(s) standing for 10 run(s)"; got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}
	c.Snapshots = c.Snapshots[:2]
	r.Configurations[0] = c
	if got, want := r.Summary(), "results of 2 run configuration(s): 1 with 2 stored snapshot(s)"; got != want {
		t.Errorf("Summary without a summary = %q, want %q", got, want)
	}
}

// The sidecar round-trips through JSON with its statistics.
func TestReadKeepsStatistics(t *testing.T) {
	const doc = `{"source":"m.xmi","configurations":[{"id":"_c","name":"'Group 0'","analysis":"t","observables":["t"],
	  "snapshots":[{"id":"_s","values":{"t":3.5},"statistics":{"observable":"t","runs":8,"mean":3.5,"deviation":0.5}}]}]}`
	r, err := Read(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	c := r.Configurations[0]
	want := &Statistics{Observable: "t", Runs: 8, Mean: 3.5, Deviation: 0.5}
	if c.Analysis != "t" || !reflect.DeepEqual(c.Snapshots[0].Statistics, want) {
		t.Errorf("read %+v, want statistics %+v of t", c, want)
	}
	if got := c.StoredRuns(); got != 8 {
		t.Errorf("StoredRuns = %d, want 8", got)
	}
}

// A summarising snapshot another configuration stores under the same name with
// the same statistics is noted as a likely copy; one differing in a number both
// hold, in its statistics, or storing runs one by one, is not.
func TestRepeatsNoteSummariesStoredTwice(t *testing.T) {
	summary := func(name string, values map[string]float64, dev float64) Snapshot {
		return Snapshot{ID: "_" + name, Name: name, Values: values, Statistics: &Statistics{Observable: "t", Runs: 5, Mean: 3.5, Deviation: dev}}
	}
	r := Results{Source: "m.xmi", Configurations: []ConfigurationResults{
		{ID: "_a", Name: "'Group 0'", Location: "'Results A'", Snapshots: []Snapshot{summary("g0", map[string]float64{"t": 3.5, "p": 1}, 0.5)}},
		{ID: "_b", Name: "'Group 0 again'", Location: "'Results B'", Snapshots: []Snapshot{summary("g0", map[string]float64{"t": 3.5, "p": 1, "q": 2}, 0.5)}},
		{ID: "_c", Name: "'Group 1'", Location: "'Results C'", Snapshots: []Snapshot{summary("g0", map[string]float64{"t": 3.5, "p": 0}, 0.5)}},
		{ID: "_d", Name: "'Group 2'", Location: "'Results D'", Snapshots: []Snapshot{summary("g0", map[string]float64{"t": 3.5}, 0.25)}},
		{ID: "_e", Name: "'Group 3'", Location: "'Results E'", Snapshots: []Snapshot{{ID: "_run", Name: "g0", Values: map[string]float64{"t": 3.5}}}},
	}}
	want := []string{`the snapshot "g0" bears the name and the statistics of t of a snapshot of the configuration 'Group 0 again' (in 'Results B'), so one may be a copy of the other`}
	if got := r.Repeats(0); !reflect.DeepEqual(got, want) {
		t.Errorf("Repeats(0) = %q, want %q", got, want)
	}
	for i := 2; i < len(r.Configurations); i++ {
		if got := r.Repeats(i); got != nil {
			t.Errorf("Repeats(%d) = %q, want none: %s", i, got, r.Configurations[i].Name)
		}
	}
}

// Summaries of one observable whose means lie more than three standard errors
// apart disagree, so they cannot be of one and the same model; ones within
// sampling error of each other, or of a single run, do not.
func TestDisagreeingSummaries(t *testing.T) {
	summary := func(name string, runs int64, mean, dev float64) Snapshot {
		return Snapshot{ID: "_" + name, Name: name, Values: map[string]float64{"t": mean}, Statistics: &Statistics{Observable: "t", Runs: runs, Mean: mean, Deviation: dev}}
	}
	c := ConfigurationResults{Snapshots: []Snapshot{
		summary("a", 1000, 13.26, 8.28),
		summary("b", 1000, 13.5, 8.5),
		summary("c", 1000, 21.3, 14.5),
		summary("d", 1, 40.0, 0),
		{ID: "_run", Values: map[string]float64{"t": 50}},
	}}
	var got []string
	for _, s := range c.Disagreeing("t") {
		got = append(got, s.Name)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Disagreeing = %q, want %q", got, want)
	}
	if got := c.Disagreeing("p"); got != nil {
		t.Errorf("Disagreeing of an observable not summarised = %v, want none", got)
	}
	c.Snapshots = c.Snapshots[:2]
	if got := c.Disagreeing("t"); got != nil {
		t.Errorf("summaries within sampling error disagree: %v", got)
	}
	if !(Statistics{Runs: 3, Mean: 1}).Disagrees(Statistics{Runs: 3, Mean: 2}) {
		t.Error("constant runs of different values do not disagree")
	}
}
