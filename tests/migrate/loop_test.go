package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/translate/migrate"
)

// testdata/xmi/loops.xmi, activity Looping: a tested-first loop with a
// decider is written as a v2 loop — setup once, test before body, the loop
// variables and results carried by private features and pin defaults.
func TestLoopNodeTestedFirstFixture(t *testing.T) {
	r := migrateFixtureFile(t, "loops")
	wantClean(t, "loops.xmi", r)

	wantLine(t, r.Notation, "action 'loop' {")
	wantLine(t, r.Notation, "in iIn : ScalarValues::Integer[1];")
	wantLine(t, r.Notation, "out sOut : ScalarValues::Integer[1];")
	wantLine(t, r.Notation, "private attribute i : ScalarValues::Integer := iIn;")
	wantLine(t, r.Notation, "private attribute s : ScalarValues::Integer := sIn;")
	wantLine(t, r.Notation, "private attribute ended : ScalarValues::Boolean := false;")
	wantLine(t, r.Notation, "first start then setup;")
	wantLine(t, r.Notation, "loop {")
	wantLine(t, r.Notation, "} until ended;")
	wantNote(t, r, "_loop", migrate.Mapped, "")
	wantNote(t, r, "_lvI", migrate.Mapped, "")
	wantNote(t, r, "_lvS", migrate.Mapped, "")

	// The test part's pins take the loop variables and the decider names the
	// test's pin; a setup output feeds a test pin through a pin default, and
	// a test output feeds a body pin the same way.
	wantLine(t, r.Notation, "in x : ScalarValues::Integer[1] = i;")
	wantLine(t, r.Notation, "in y : ScalarValues::Integer[1] = s;")
	wantLine(t, r.Notation, "in z : ScalarValues::Integer[1] = setup.zero.r;")
	wantLine(t, r.Notation, "then if not test.le.r {")
	wantLine(t, r.Notation, "assign ended := true;")
	wantLine(t, r.Notation, "in x : ScalarValues::Integer[1] = s;")
	wantLine(t, r.Notation, "in y : ScalarValues::Integer[1] = test.le.k;")
	wantLine(t, r.Notation, "assign i := body.inc.r;")
	wantLine(t, r.Notation, "then assign s := body.plus.r;")
	wantLine(t, r.Notation, "assign iOut := i;")
	wantLine(t, r.Notation, "then assign sOut := s;")
	wantNote(t, r, "_ze1", migrate.Unmapped, "the loop variable 's' takes its value from its loop-variable input")
	wantNote(t, r, "_ze2", migrate.Mapped, "the pin takes 'r', computed by an earlier part of the loop")
	wantNote(t, r, "_lfe2", migrate.Mapped, "the pin takes the loop variable 'i'")
	wantNote(t, r, "_lfe5", migrate.Mapped, "the pin takes the loop variable 's'")
	wantNote(t, r, "_lfe6", migrate.Mapped, "the pin takes 'k', computed by an earlier part of the loop")

	// A loop-level fork routes the loop variable to every pin it leads to;
	// a flow from the body back to the test cannot be written.
	wantNote(t, r, "_fork", migrate.Approximated, "the node routes data only")
	wantNote(t, r, "_back", migrate.Unmapped, "the flow runs from a later part of the loop to an earlier one; nothing carries the value back")

	// A cross-part flow listed before a same-part flow into the same pin
	// loses to it: the pin is already fed inside its part, so it takes no
	// default.
	wantNote(t, r, "_ordX", migrate.Unmapped, "the pin 'v' is already fed inside its part")
}

// Post-tested, over an untyped multi-valued loop variable: body before test.
func TestLoopNodePostTestedFixture(t *testing.T) {
	r := migrateFixtureFile(t, "loops")
	wantClean(t, "loops.xmi", r)

	wantNote(t, r, "_post", migrate.Mapped, "")
	wantLine(t, r.Notation, "in xIn[0..*] nonunique;")
	wantLine(t, r.Notation, "private ref x[0..*] nonunique := xIn;")
	wantLine(t, r.Notation, "first start then iterate;")
	wantLine(t, r.Notation, "in v[0..*] nonunique = x;")
	wantLine(t, r.Notation, "assign x := body.grow.r;")
	wantLine(t, r.Notation, "then action test {")
	wantLine(t, r.Notation, "then if not test.lt.r {")
	wantLine(t, r.Notation, "assign xOut := x;")
	wantNote(t, r, "_lvX", migrate.Mapped, "")
	wantNote(t, r, "_pf1", migrate.Mapped, "the pin takes the loop variable 'x'")
	wantNote(t, r, "_pf2", migrate.Mapped, "the pin takes the loop variable 'x'")
}

// The loop shapes that cannot be written keep the body-once notation with a
// note naming why: no decider, a decider nested deeper than the test part, and
// mismatched pin counts.
func TestLoopNodeFallbackFixture(t *testing.T) {
	r := migrateFixtureFile(t, "loops")
	wantClean(t, "loops.xmi", r)

	wantNote(t, r, "_plain", migrate.Approximated, "the loop names no decider, so its test cannot be written: the body is written once")
	wantLine(t, r.Notation, "action plain {")
	wantLine(t, r.Notation, "action qset {")
	wantLine(t, r.Notation, "action qbody {")

	wantNote(t, r, "_deep", migrate.Approximated, "the decider is not an output pin of a node in the test part: the body is written once")
	wantLine(t, r.Notation, "action deep {")
	wantLine(t, r.Notation, "action dtest {")

	wantNote(t, r, "_mismatch", migrate.Approximated, "the loop variables, inputs, body outputs and results differ in count: the body is written once")
	wantLine(t, r.Notation, "action mismatch {")
	wantLine(t, r.Notation, "action mb {")

	// Loop-level routing is only a fork's: a decision node falls back, as
	// does a fork two loop variables feed.
	wantNote(t, r, "_decide", migrate.Approximated, "'choose' routes data by a choice a pin default cannot make: the body is written once")
	wantNote(t, r, "_twofork", migrate.Approximated, "'joiner' carries values from more than one loop variable: the body is written once")
}

// A LoopNode's own variables are declared once, at the loop action level, so
// a write the setup or the body makes reaches the read the test makes on the
// next pass — the activity VarLoop's loop owns total.
func TestLoopNodeOwnedVariableFixture(t *testing.T) {
	r := migrateFixtureFile(t, "loops")
	wantClean(t, "loops.xmi", r)

	wantNote(t, r, "_vloop", migrate.Mapped, "")
	wantNote(t, r, "_vTotal", migrate.Mapped, "")
	if n := strings.Count(string(r.Notation), "private attribute total"); n != 1 {
		t.Errorf("expected exactly one private attribute total declaration, found %d", n)
	}
	wantLine(t, r.Notation, "private attribute total : ScalarValues::Integer;")
	wantLine(t, r.Notation, "assign total := 0;")
	wantLine(t, r.Notation, "assign total := total + x;")
	wantLine(t, r.Notation, "out r : ScalarValues::Integer[1] = total;")
}

// Every member of the loop action takes a distinct name: the pins keep the
// source names, the loop variables and the made-up members avoid them.
func TestLoopNodeNameCollisionsFixture(t *testing.T) {
	r := migrateFixtureFile(t, "loops")
	wantClean(t, "loops.xmi", r)

	wantLine(t, r.Notation, "in p : ScalarValues::Integer[1];")
	wantLine(t, r.Notation, "in test : ScalarValues::Integer[1];")
	wantLine(t, r.Notation, "out ended : ScalarValues::Integer[1];")
	wantLine(t, r.Notation, "private attribute p2 : ScalarValues::Integer := test;")
	wantLine(t, r.Notation, "private attribute ended2 : ScalarValues::Boolean := false;")
	wantLine(t, r.Notation, "action test2 {")
	wantLine(t, r.Notation, "} until ended2;")
	wantLine(t, r.Notation, "assign ended := p2;")
}
