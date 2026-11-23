package resolve

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

// A read that depends on its cuts alone must not copy the namespaces entered
// since its mark: that record grows with the lookup in progress.
func TestRouteCutsSinceDoesNotCopyEnteredNamespaces(t *testing.T) {
	excluded, other := &symbols.Scope{}, &symbols.Scope{}
	r := &Resolver{importPath: []*symbols.Scope{excluded}}
	for range 1024 {
		r.importSeen = append(r.importSeen, other)
	}

	allocs := testing.AllocsPerRun(100, func() {
		if cut := r.routeCutsSince(r.markRoute(), nil); len(cut) != 0 {
			t.Fatalf("cut = %d namespaces, want none", len(cut))
		}
	})
	if allocs != 0 {
		t.Errorf("an uncut read allocated %v times, want 0", allocs)
	}
	if r.importMarks != 0 {
		t.Errorf("importMarks = %d after closing every mark, want 0", r.importMarks)
	}

	mark := r.markRoute()
	if r.entersNamespace(excluded) {
		t.Fatal("entered a namespace that is being read")
	}
	r.entersNamespace(excluded)
	if cut := r.routeCutsSince(mark, nil); len(cut) != 1 || cut[0] != excluded {
		t.Errorf("cut = %v, want the excluded namespace once", cut)
	}
	mark = r.markRoute()
	r.entersNamespace(excluded)
	if cut := r.routeCutsSince(mark, excluded); len(cut) != 0 {
		t.Errorf("cut = %v, want none: the read's own namespace is not a dependency", cut)
	}
}

// routeDepsSince reports the same cuts as routeCutsSince, with the entries.
func TestRouteDepsSinceReportsCutsAndEntries(t *testing.T) {
	excluded, a, b := &symbols.Scope{}, &symbols.Scope{}, &symbols.Scope{}
	r := &Resolver{importPath: []*symbols.Scope{excluded}}
	mark := r.markRoute()
	r.entersNamespace(a)
	r.entersNamespace(excluded)
	r.entersNamespace(b)
	r.entersNamespace(a)
	deps := r.routeDepsSince(mark, nil)
	if len(deps.cut) != 1 || deps.cut[0] != excluded {
		t.Errorf("cut = %v, want the excluded namespace", deps.cut)
	}
	if len(deps.entered) != 2 || deps.entered[0] != a || deps.entered[1] != b {
		t.Errorf("entered = %v, want a then b", deps.entered)
	}
	if r.importMarks != 0 {
		t.Errorf("importMarks = %d, want 0", r.importMarks)
	}
}
