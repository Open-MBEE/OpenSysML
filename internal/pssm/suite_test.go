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
