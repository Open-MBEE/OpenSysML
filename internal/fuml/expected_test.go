package fuml

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const repoRoot = "../.."

func clearPinEnv(t *testing.T) {
	t.Helper()
	for _, env := range []string{TagEnv, CommitEnv, TestsSHA256Env, ExceptionSHA2Env, LibrarySHA256Env, JarSHA256Env} {
		t.Setenv(env, "")
	}
}

func committedExpected(t *testing.T) (Pin, *Expected) {
	t.Helper()
	clearPinEnv(t)
	pin, err := ReadPin(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	e, err := ReadExpected(filepath.Join(repoRoot, filepath.FromSlash(ExpectedPath)))
	if err != nil {
		t.Fatal(err)
	}
	return pin, e
}

func TestCommittedExpectedMatchesThePin(t *testing.T) {
	pin, e := committedExpected(t)
	if err := pin.Check(e); err != nil {
		t.Fatal(err)
	}
	if e.Provenance.LibraryDigest == "" || e.Provenance.LibraryResource == "" {
		t.Fatalf("provenance names no foundational library: %+v", e.Provenance)
	}
}

// The record must hold every activity the pinned models declare, executed
// where the JUnit suite executes it directly, and no execution may have failed.
func TestCommittedExpectedCoversBothModels(t *testing.T) {
	_, e := committedExpected(t)
	declared := map[string]int{}
	executed := map[string]int{}
	ids := map[string]bool{}
	for _, a := range e.Activities {
		declared[a.Model]++
		key := a.Model + "#" + a.ID
		if ids[key] {
			t.Errorf("%s recorded twice", key)
		}
		ids[key] = true
		if a.ID == "" || a.Name == "" {
			t.Errorf("%s has an unnamed activity %+v", a.Model, a)
		}
		if a.Error != "" {
			t.Errorf("%s %s: the implementation failed: %s", a.Model, a.Name, a.Error)
		}
		if a.Executed {
			executed[a.Model]++
			if a.Skipped != "" {
				t.Errorf("%s %s both executed and skipped", a.Model, a.Name)
			}
			if len(a.Events) == 0 || a.Events[0].Kind != "Execute" || a.Events[0].Activity != a.Name {
				t.Errorf("%s %s: trace does not open with its own Execute: %+v", a.Model, a.Name, a.Events)
			}
		} else if a.Skipped == "" {
			t.Errorf("%s %s neither executed nor skipped", a.Model, a.Name)
		}
	}
	want := map[string][2]int{TestsFile: {43, 42}, ExceptionTestsFile: {12, 9}}
	for model, counts := range want {
		if declared[model] != counts[0] || executed[model] != counts[1] {
			t.Errorf("%s: %d declared, %d executed; want %d and %d", model, declared[model], executed[model], counts[0], counts[1])
		}
	}
}

// Spot checks against the assertions of the implementation's own JUnit tests.
func TestCommittedExpectedAgreesWithTheJUnitSuite(t *testing.T) {
	_, e := committedExpected(t)
	cases := []struct {
		id, name, parameter string
		values              []string
	}{
		{"_15_5_1_1a900482_1225493265234_644298_826", "Copier", "output", []string{"0"}},
		{"_15_5_1_1a900482_1225497525562_682640_935", "CopierCaller", "output", []string{"888"}},
		{"_15_5_1_1a900482_1225498182468_409301_1087", "SimpleDecision", "output_0", []string{"0"}},
		{"_15_5_1_1a900482_1225498182468_409301_1087", "SimpleDecision", "output_1", nil},
		{"_15_5_1_1a900482_1225499009421_139168_1510", "DecisionJoin", "output", []string{"0", "1"}},
		{"_15_5_1_1a900482_1225507220781_95467_2449", "ForkMerge", "output", []string{"0", "0"}},
		{"_15_5_1_1a900482_1225507524468_320008_2683", "ForkMergeData", "output", []string{"0", "0"}},
		{"_18_4_1_12e503d9_1483307657460_746223_7722", "NodeEnabler", "output", []string{"0"}},
		{"_18_4_1_12e503d9_1483308204258_480645_8357", "TestNodeEnabler", "output", []string{"1"}},
		{"_15_5_1_1a900482_1227279618546_338430_844", "TestIntegerFunctions", "NegResult", []string{"-3"}},
		{"_15_5_1_1a900482_1225670181734_47413_418", "TestSimpleActivities", "CopierCaller.output", []string{"888"}},
	}
	for _, c := range cases {
		a := e.Activity(TestsFile, c.id)
		if a == nil {
			t.Errorf("%s (%s) is not recorded", c.id, c.name)
			continue
		}
		if a.Name != c.name {
			t.Errorf("%s is %q, want %q", c.id, a.Name, c.name)
		}
		var got []string
		found := false
		for _, o := range a.Outputs {
			if o.Parameter != c.parameter {
				continue
			}
			found = true
			for _, v := range o.Values {
				got = append(got, string(v.Value))
			}
		}
		if !found {
			t.Errorf("%s has no output %s", c.name, c.parameter)
		}
		if strings.Join(got, ",") != strings.Join(c.values, ",") {
			t.Errorf("%s.%s = %v, want %v", c.name, c.parameter, got, c.values)
		}
	}
}

func TestRefiredDetectsPerTokenRefiring(t *testing.T) {
	_, e := committedExpected(t)
	refires := map[string][]string{
		"DecisionJoin":  {"Action_A"},
		"ForkMergeData": {"Action_B"},
		"ForkMerge":     {"Value(0)"},
		"Copier":        nil,
		"ForkJoin":      nil,
		"NodeEnabler":   nil,
	}
	for name, want := range refires {
		var a *ExpectedActivity
		for i := range e.Activities {
			if e.Activities[i].Model == TestsFile && e.Activities[i].Name == name {
				a = &e.Activities[i]
			}
		}
		if a == nil {
			t.Fatalf("%s not recorded", name)
		}
		if got := a.Refired(); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%s refired %v, want %v", name, got, want)
		}
	}
}

func TestRefiredIgnoresTheActionsOfCalledActivities(t *testing.T) {
	a := ExpectedActivity{Name: "Outer", Events: []ExpectedEvent{
		{Kind: "Execute", Activity: "Outer"},
		{Kind: "Fire", Activity: "Outer", Action: "call"},
		{Kind: "Execute", Activity: "Inner"},
		{Kind: "Fire", Activity: "Inner", Action: "step"},
		{Kind: "Fire", Activity: "Outer", Action: "call"},
		{Kind: "Execute", Activity: "Inner"},
		{Kind: "Fire", Activity: "Inner", Action: "step"},
	}}
	if got := a.Refired(); len(got) != 1 || got[0] != "call" {
		t.Fatalf("Refired() = %v, want [call]", got)
	}
}

func TestCheckRejectsARecordFromAnotherPin(t *testing.T) {
	pin, e := committedExpected(t)
	stale := *e
	stale.Provenance.RICommit = strings.Repeat("0", 40)
	stale.Provenance.Models = append([]ExpectedModel(nil), e.Provenance.Models...)
	stale.Provenance.Models[0].Digest = strings.Repeat("1", 64)
	err := pin.Check(&stale)
	if !errors.Is(err, ErrStaleExpected) {
		t.Fatalf("err = %v, want ErrStaleExpected", err)
	}
	for _, want := range []string{"riCommit", TestsFile, "make fuml-expected"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err %q does not name %s", err, want)
		}
	}
	missing := *e
	missing.Provenance.Models = e.Provenance.Models[:1]
	if err := pin.Check(&missing); err == nil || !strings.Contains(err.Error(), ExceptionTestsFile+" was not run") {
		t.Fatalf("err = %v, want one naming the model that was not run", err)
	}
}

func TestReadExpectedRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "expected.json")
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadExpected(path); err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("err = %v, want a parse error", err)
	}
	if _, err := ReadExpected(filepath.Join(t.TempDir(), "absent.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want ErrNotExist", err)
	}
}
