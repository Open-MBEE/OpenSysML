package corpus

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/tests/testutil/corpussubset"
)

// A recorded file that is gone is reported by name, and one the run skipped is counted.
func TestRecordedCoverage(t *testing.T) {
	recorded := []string{"a.sysml", "b.sysml", "c.sysml", "d.sysml"}
	present := map[string]bool{"a.sysml": true, "b.sysml": true, "d.sysml": true}
	checked := map[string]bool{"a.sysml": true}

	gone, unchecked := recordedCoverage(recorded, present, checked)
	if want := []string{"c.sysml"}; !reflect.DeepEqual(gone, want) {
		t.Errorf("gone = %v, want %v", gone, want)
	}
	if unchecked != 2 {
		t.Errorf("unchecked = %d, want 2", unchecked)
	}

	gone, unchecked = recordedCoverage(recorded[:1], present, checked)
	if len(gone) != 0 || unchecked != 0 {
		t.Errorf("a fully checked record reports gone=%v unchecked=%d", gone, unchecked)
	}
}

func TestUncheckedBanner(t *testing.T) {
	t.Setenv(corpussubset.EnvVar, corpussubset.ConstructCover)
	subset := corpussubset.Must(t, "../..")
	got := uncheckedBanner("some gate", subset, 7, 9, "files")
	for _, want := range []string{"RATCHET INCOMPLETE", "some gate", "7 of 9 recorded files unchecked", corpussubset.EnvVar + "=" + corpussubset.ConstructCover} {
		if !strings.Contains(got, want) {
			t.Errorf("banner %q lacks %q", got, want)
		}
	}
}

// subsetScoped names every test file here that reads the subset and how it accounts for
// what the subset leaves out, so a new subset-scoped gate cannot be added silently.
var subsetScoped = map[string]string{
	"construct_cover_test.go":              "corpussubset.ReadList(", // checks the list itself against the whole corpus
	"element_properties_agreement_test.go": "announceUnchecked(t, agreementGate, subset,",
	"examples_sweep_test.go":               "check(t, subset,", // the round-trip ratchets announce in roundTripGate.check
	"pilot_corpora_test.go":                "checkRecorded(t, subset,",
	"roundtrip_test.go":                    "announceUnchecked(t, g.name, subset,",
	"spec_examples_test.go":                "checkRecorded(t, subset,",
}

func TestSubsetScopedGatesAnnounce(t *testing.T) {
	sources, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	var scoped []string
	for _, name := range sources {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(data)
		if name == "ratchet_coverage_test.go" || !strings.Contains(src, "corpussubset.Subset") && !strings.Contains(src, "corpussubset.Must(") {
			continue
		}
		if name == "corpus_gate_test.go" {
			continue // the shared helpers, exercised through the gates listed
		}
		scoped = append(scoped, name)
		accounting, ok := subsetScoped[name]
		switch {
		case !ok:
			t.Errorf("%s reads %s but is not listed in subsetScoped; say how it announces what it leaves unchecked", name, corpussubset.EnvVar)
		case !strings.Contains(src, accounting):
			t.Errorf("%s no longer contains %q, its accounting for the files a subset leaves unchecked", name, accounting)
		}
	}
	listed := make([]string, 0, len(subsetScoped))
	for name := range subsetScoped {
		listed = append(listed, name)
	}
	sort.Strings(listed)
	if !reflect.DeepEqual(scoped, listed) {
		t.Errorf("subset-scoped files are %v, subsetScoped lists %v", scoped, listed)
	}
}
