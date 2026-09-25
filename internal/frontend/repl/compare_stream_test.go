package repl

import (
	"reflect"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/simresults"
)

// CompareResultsEach reports each verdict as it is decided, in the order
// CompareResults lists them: configurations in index order, refused names after.
func TestCompareResultsEachStreamsVerdictsInOrder(t *testing.T) {
	s := compareSession(t)
	seed := uint64(1)
	results := compareResults("'Group 1'", 2)
	refused := results.Configurations[0]
	refused.ID, refused.Name, refused.Behavior = "_d", "Cfg::Idle", ""
	again := results.Configurations[0]
	again.ID, again.Name, again.Runs = "_e", "Cfg::'Sub::Group'", 1
	results.Configurations = append(results.Configurations, refused, again)
	opts := CompareOptions{Seed: &seed, Only: []string{"Cfg::'Group 1'", "Cfg::Idle", "_e", "Nowhere"}}

	var streamed []Verdict
	var subjects []string
	s.CompareResultsEach(results, opts, func(v Verdict) {
		streamed = append(streamed, v)
		subjects = append(subjects, v.Subject)
	})
	want := []string{"compare Cfg::'Group 1'", "compare Cfg::Idle", "compare Cfg::'Sub::Group'", "compare Nowhere"}
	if !reflect.DeepEqual(subjects, want) {
		t.Fatalf("verdicts streamed as %v, want %v", subjects, want)
	}
	if !streamed[0].Holds() || streamed[1].Status != VerdictUnresolved || !streamed[2].Holds() || streamed[3].Status != VerdictUnresolved {
		t.Errorf("verdict statuses = %v %v %v %v", streamed[0].Status, streamed[1].Status, streamed[2].Status, streamed[3].Status)
	}
	for i, v := range s.CompareResults(results, opts) {
		if v.Subject != streamed[i].Subject || v.Status != streamed[i].Status || !reflect.DeepEqual(v.Lines, streamed[i].Lines) {
			t.Errorf("CompareResults collects\n%+v\nwhere CompareResultsEach streamed\n%+v", v, streamed[i])
		}
	}

	var none []Verdict
	s.CompareResultsEach(&simresults.Results{Source: results.Source}, opts, func(v Verdict) { none = append(none, v) })
	if len(none) != 1 || none[0].Status != VerdictUnresolved || none[0].Subject != "compare" {
		t.Errorf("an empty index streams %+v, want one refusal", none)
	}
}
