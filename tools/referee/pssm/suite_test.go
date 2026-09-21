package pssm

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// loadSuite reads the pinned suite, skipping when it is absent unless
// RequireEnv is set, in which case absence is a failure as CI demands.
func loadSuite(t *testing.T) *Suite {
	t.Helper()
	path, err := Locate("../../../" + DefaultRoot)
	if err != nil {
		if errors.Is(err, ErrSuiteAbsent) && !Required() {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	s, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TestSuiteRead pins the shape of the pinned suite: 103 registered tests,
// their per-area counts, and a reader that has no unresolved reference.
func TestSuiteRead(t *testing.T) {
	s := loadSuite(t)
	if len(s.Tests) != 103 {
		t.Errorf("tests = %d, want 103", len(s.Tests))
	}
	for _, d := range s.Diagnostics {
		t.Errorf("suite diagnostic: %s", d)
	}
	areas := map[string]int{}
	for _, tt := range s.Tests {
		areas[tt.Area]++
		if len(tt.Expected) == 0 {
			t.Errorf("%s: no expected trace", tt.Name)
		}
		if tt.Machine == nil {
			t.Errorf("%s: no state machine", tt.Name)
		}
		if tt.Stimulation == nil {
			t.Errorf("%s: no stimulation", tt.Name)
		}
		for _, d := range tt.Diagnostics {
			t.Errorf("%s: %s", tt.Name, d)
		}
	}
	var got []string
	for area, n := range areas {
		got = append(got, fmt.Sprintf("%s %d", area, n))
	}
	sort.Strings(got)
	want := strings.Fields(`Behavior_5 Choice_5 Deferred_10 Entering_5 Entry_6 Event_16 Exit_3 Exiting_5
		Final_1 Fork_2 History_8 Join_3 Junction_6 Other_1 Redefinition_6 Standalone_3 Terminate_3 Transition_15`)
	for i := range want {
		want[i] = strings.ReplaceAll(want[i], "_", " ")
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("areas = %v\nwant    %v", got, want)
	}
}

// TestSuiteClassification pins the classifier area by area; the alignment
// note's test-suite section and docs/project/pssm-referee.md record the seven
// tests it moves out of the original hand count (37/33/3/30) and why, and the
// three terminate tests that are standard now that terminate executes.
func TestSuiteClassification(t *testing.T) {
	s := loadSuite(t)
	type row struct{ std, ext, none int }
	want := map[string]row{
		"Behavior": {4, 0, 1}, "Transition": {8, 1, 6}, "Event": {11, 0, 5},
		"Entering": {4, 0, 1}, "Exiting": {4, 0, 1}, "Entry": {0, 0, 6},
		"Exit": {0, 0, 3}, "Choice": {0, 4, 1}, "Junction": {0, 5, 1},
		"Fork": {0, 1, 1}, "Join": {0, 3, 0}, "Final": {1, 0, 0},
		"Terminate": {3, 0, 0}, "History": {0, 8, 0}, "Deferred": {0, 9, 1},
		"Redefinition": {0, 0, 6}, "Standalone": {0, 0, 3}, "Other": {0, 0, 1},
	}
	got := map[string]row{}
	var total row
	var strays []string
	for _, tt := range s.Tests {
		c := Classify(tt)
		r := got[tt.Area]
		switch c.Class {
		case Standard:
			r.std++
			total.std++
		case Extension:
			r.ext++
			total.ext++
		case NotExpressible:
			r.none++
			total.none++
		}
		got[tt.Area] = r
		for _, u := range c.Uses {
			if constructClass[u.Construct] == Standard {
				strays = append(strays, tt.Name+": "+u.String())
			} else if c.Class == Standard {
				t.Errorf("%s: standard with use %v", tt.Name, u)
			}
		}
		if c.Class != Standard && c.Reason() == "" {
			t.Errorf("%s: %s without a reason", tt.Name, c.Class)
		}
	}
	// The suite files two unreferenced pseudostates as connection points; they
	// are recorded, not counted.
	wantStrays := []string{
		"Join001: stray connection point join Join1",
		"Deferred 004 B: stray connection point initial Initial1",
	}
	if strings.Join(strays, "\n") != strings.Join(wantStrays, "\n") {
		t.Errorf("strays = %q, want %q", strays, wantStrays)
	}
	for area, w := range want {
		if got[area] != w {
			t.Errorf("%s = %+v, want %+v", area, got[area], w)
		}
	}
	if total != (row{35, 31, 37}) {
		t.Errorf("total = %+v, want {35 31 37}", total)
	}
}

// TestSuiteNoTranslationReasons pins the reasons left on the tests the driver
// and reader lifted a construct from: a tester trace and a standalone machine
// are no refusals, the rest of each list is byte-identical.
func TestSuiteNoTranslationReasons(t *testing.T) {
	s := loadSuite(t)
	want := map[string]string{
		"Event 019 A":    "standard notation only",
		"Event 019 D":    "operation result T2",
		"Event 019 E":    "behavior parameter S1.S1.1; behavior parameter S1.S2.1.S2.1.1; operation result T2",
		"Deferred 007":   "operation result T4",
		"Standalone 001": "exit point ExitPoint1; exit point ExitPoint1; entry point EntryPoint1",
		"Standalone 002": "exit point ExitPoint1; entry point EntryPoint1; behavior parameter S2; behavior parameter S2; behavior parameter S2.S2.1; behavior parameter S2.S2.2",
		"Standalone 003": "behavior parameter S1.S1.1; behavior parameter S1.S2.1.S2.1.1; operation result T2",
		"Entry 002 F":    "behavior parameter S1; entry point EntryPoint1; behavior parameter S1.S1.1; behavior parameter S1.S1.2; local transition T1.1; local transition T1.2",
	}
	for _, tt := range s.Tests {
		reason, ok := want[tt.Name]
		if !ok {
			continue
		}
		delete(want, tt.Name)
		c := Classify(tt)
		if c.Reason() != reason {
			t.Errorf("%s: reason %q, want %q", tt.Name, c.Reason(), reason)
		}
		class := NotExpressible
		if reason == "standard notation only" {
			class = Standard
		}
		if c.Class != class {
			t.Errorf("%s: %s, want %s", tt.Name, c.Class, class)
		}
	}
	for name := range want {
		t.Errorf("%s: not in the suite", name)
	}
}
