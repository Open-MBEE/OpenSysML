package resolve

import "testing"

// A hidesBelow walk cut by a collision computation in progress is remembered,
// undecided, only while that computation runs; afterwards it settles.
func TestHidesBelowCutIsKeptOnlyWhileTheCollisionComputationRuns(t *testing.T) {
	const src = `package P1 { public import P2::*; public import P3::*; part def D1; }
package P2 { public import P1::*; public import P3::*; part def D2; }
package P3 { public import P1::*; public import P2::*; part def D3; }
package Use { public import P1::*; part x : D3; }`
	r := resolveDoc(t, "cycle.sysml", src)
	if len(r.Diagnostics) != 0 {
		t.Fatalf("got diagnostics %v, want none", r.Diagnostics)
	}
	if r.belowCut != nil {
		t.Fatalf("cut answers %v outlive the computations that cut them", r.belowCut)
	}
	p1 := r.idx.Declaring("P1").Scope
	p2 := r.idx.Declaring("P2").Scope
	delete(r.below, p1)
	r.colliding[p2] = true
	if !r.hidesBelow(p1) {
		t.Fatal("a walk cut by a computation in progress must answer true, undecided")
	}
	if !r.belowCut[p1] {
		t.Fatal("the cut walk is not remembered for the computation's duration")
	}
	if !r.hidesBelow(p1) {
		t.Fatal("a remembered cut must answer as the walk did")
	}
	if _, done := r.below[p1]; done {
		t.Fatal("an undecided answer must not be memoized as settled")
	}
	delete(r.colliding, p2)
	r.collisionCut = false
	if r.hidesBelow(p1) {
		t.Fatal("nothing is hidden in the cycle once its computations settle")
	}
	if r.belowCut != nil {
		t.Fatalf("cut answers %v survive the computations settling", r.belowCut)
	}
	if hides, done := r.below[p1]; !done || hides {
		t.Fatalf("settled answer (%v, %v), want (false, memoized)", hides, done)
	}
}
