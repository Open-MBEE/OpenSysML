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
