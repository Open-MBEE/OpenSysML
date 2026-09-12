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
	path, err := Locate("../../" + DefaultRoot)
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

// TestSuiteClassification pins the classifier area by area. It differs from
// the alignment note's hand count (37/33/3/30) by nine tests the notation
// cannot spell: six Event tests and Deferred 007 use parameterised
// entry/exit/do behaviors, operation results or a tester-side trace, and
// Fork 002 and Join 001 fork into orthogonal regions with no initial state.
func TestSuiteClassification(t *testing.T) {
	s := loadSuite(t)
	type row struct{ std, ext, gap, none int }
	want := map[string]row{
		"Behavior": {4, 0, 0, 1}, "Transition": {8, 1, 0, 6}, "Event": {10, 0, 0, 6},
		"Entering": {4, 0, 0, 1}, "Exiting": {4, 0, 0, 1}, "Entry": {0, 0, 0, 6},
		"Exit": {0, 0, 0, 3}, "Choice": {0, 5, 0, 0}, "Junction": {0, 5, 0, 1},
		"Fork": {0, 0, 0, 2}, "Join": {0, 2, 0, 1}, "Final": {1, 0, 0, 0},
		"Terminate": {0, 0, 3, 0}, "History": {0, 8, 0, 0}, "Deferred": {0, 9, 0, 1},
		"Redefinition": {0, 0, 0, 6}, "Standalone": {0, 0, 0, 3}, "Other": {0, 0, 0, 1},
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
		case TerminateGap:
			r.gap++
			total.gap++
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
	if total != (row{31, 30, 3, 39}) {
		t.Errorf("total = %+v, want {31 30 3 39}", total)
	}
}
