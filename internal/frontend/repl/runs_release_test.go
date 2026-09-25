package repl

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
)

// A caller that sets release gets a table no row of which keeps its context, the rows
// that failed included; a caller that does not keeps every row's.
func TestRunsTableReleasesFailedRowsToo(t *testing.T) {
	s := compareSession(t)
	seed := uint64(1)
	for _, release := range []bool{false, true} {
		inv, unresolved := s.resolveInvocation([]Behavior{{Name: "Cfg::'Group 0'"}}, nil, nil)
		if inv == nil {
			t.Fatalf("Group 0 does not resolve: %+v", unresolved)
		}
		s.state.Lock()
		_, table, err := s.runsTable(inv, 8, &seed, nil, runtime.DrawRandom, 0, release)
		s.state.Unlock()
		if err != nil {
			t.Fatalf("release=%v: runs of Group 0: %v", release, err)
		}
		failed, completed := 0, 0
		for i, row := range table.Rows {
			if row.Err != nil {
				failed++
			} else {
				completed++
			}
			if release && row.Context != nil {
				t.Errorf("release=%v: row %d (err %v) keeps its context", release, i, row.Err)
			}
			if !release && row.Context == nil {
				t.Errorf("release=%v: row %d (err %v) lost its context", release, i, row.Err)
			}
		}
		if failed == 0 || completed == 0 {
			t.Fatalf("release=%v: %d failed and %d completed of 8 runs, want some of each", release, failed, completed)
		}
	}
}
