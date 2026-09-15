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
	a := ExpectedActivity{ID: "outer", Name: "Outer", Events: []ExpectedEvent{
		{Kind: "Execute", Activity: "Outer", ID: "outer"},
		{Kind: "Fire", Activity: "Outer", Action: "call", ID: "n1"},
		{Kind: "Execute", Activity: "Inner", ID: "inner"},
		{Kind: "Fire", Activity: "Inner", Action: "step", ID: "n2"},
		{Kind: "Complete", Activity: "Inner", ID: "inner"},
		{Kind: "Fire", Activity: "Outer", Action: "call", ID: "n1"},
		{Kind: "Execute", Activity: "Inner", ID: "inner"},
		{Kind: "Fire", Activity: "Inner", Action: "step", ID: "n2"},
		{Kind: "Complete", Activity: "Inner", ID: "inner"},
		{Kind: "Complete", Activity: "Outer", ID: "outer"},
	}}
	if got := a.Refired(); len(got) != 1 || got[0] != "call" {
		t.Fatalf("Refired() = %v, want [call]", got)
	}
}

// Two nodes sharing a display name are two nodes; the implementation's trace
// cannot tell them apart, so it carries no id for them and they count as nothing.
func TestRefiredCountsByNodeIdentityNotName(t *testing.T) {
	a := ExpectedActivity{ID: "act", Name: "Act", Events: []ExpectedEvent{
		{Kind: "Execute", Activity: "Act", ID: "act"},
		{Kind: "Fire", Activity: "Act", Action: "this"},
		{Kind: "Fire", Activity: "Act", Action: "this"},
		{Kind: "Fire", Activity: "Act", Action: "Value(1)", ID: "v1"},
		{Kind: "Fire", Activity: "Act", Action: "Value(1)", ID: "v1"},
		{Kind: "Complete", Activity: "Act", ID: "act"},
	}}
	if got := a.Refired(); len(got) != 1 || got[0] != "Value(1)" {
		t.Fatalf("Refired() = %v, want [Value(1)]", got)
	}
}

// A recursive call suspends the caller: the callee's fires belong to its own
// activation, and the caller's count resumes, not restarts, when it completes.
func TestRefiredKeepsRecursiveActivationsApart(t *testing.T) {
	events := func(callerFiresAfter int) []ExpectedEvent {
		evs := []ExpectedEvent{
			{Kind: "Execute", Activity: "Rec", ID: "rec"},
			{Kind: "Fire", Activity: "Rec", Action: "step", ID: "s"},
			{Kind: "Fire", Activity: "Rec", Action: "call", ID: "c"},
			{Kind: "Execute", Activity: "Rec", ID: "rec"},
			{Kind: "Fire", Activity: "Rec", Action: "step", ID: "s"},
			{Kind: "Complete", Activity: "Rec", ID: "rec"},
		}
		for i := 0; i < callerFiresAfter; i++ {
			evs = append(evs, ExpectedEvent{Kind: "Fire", Activity: "Rec", Action: "step", ID: "s"})
		}
		return append(evs, ExpectedEvent{Kind: "Complete", Activity: "Rec", ID: "rec"})
	}
	once := ExpectedActivity{ID: "rec", Name: "Rec", Events: events(0)}
	if got := once.Refired(); len(got) != 0 {
		t.Fatalf("one step per activation refired %v, want none", got)
	}
	again := ExpectedActivity{ID: "rec", Name: "Rec", Events: events(1)}
	if got := again.Refired(); len(got) != 1 || got[0] != "step" {
		t.Fatalf("caller stepping again after the callee refired %v, want [step]", got)
	}
}

// The committed trace nests: every Execute has its Complete, in stack order,
// and every Fire under an activity whose name identifies one node carries its id.
func TestCommittedEventsNestAndIdentifyNodes(t *testing.T) {
	_, e := committedExpected(t)
	identified := 0
	for _, a := range e.Activities {
		var open []string
		for _, ev := range a.Events {
			switch ev.Kind {
			case "Execute":
				open = append(open, ev.Activity)
			case "Complete":
				if len(open) == 0 || open[len(open)-1] != ev.Activity {
					t.Fatalf("%s: Complete %s with %v open", a.Name, ev.Activity, open)
				}
				open = open[:len(open)-1]
			case "Fire":
				if ev.ID != "" {
					identified++
				}
			}
		}
		if len(open) != 0 && a.Error == "" {
			t.Errorf("%s: %v never completed", a.Name, open)
		}
		if a.Executed && a.ID != "" {
			if len(a.Events) == 0 || a.Events[0].ID != a.ID {
				t.Errorf("%s: first event %+v does not open the activity itself", a.Name, a.Events[:1])
			}
		}
	}
	if identified == 0 {
		t.Fatal("no Fire carries a node id")
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
