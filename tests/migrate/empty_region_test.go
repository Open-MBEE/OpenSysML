package migrate_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/migrate"
)

// A composite state with two regions, one of them holding no vertex; the
// machine's own region is populated.
const emptyRegionMachine = `
    <packagedElement xmi:type="uml:Class" xmi:id="_dev" name="Device" classifierBehavior="_sm">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm" name="Life">
        <region xmi:type="uml:Region" xmi:id="_r0" name="main">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="_init0"/>
          <subvertex xmi:type="uml:State" xmi:id="_on" name="On">
            <region xmi:type="uml:Region" xmi:id="_r1" name="work">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="_init1"/>
              <subvertex xmi:type="uml:State" xmi:id="_run" name="Run"/>
              <transition xmi:type="uml:Transition" xmi:id="_t1i" source="_init1" target="_run"/>
            </region>
            <region xmi:type="uml:Region" xmi:id="_r2" name="spare"/>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="_t0" source="_init0" target="_on"/>
        </region>
      </ownedBehavior>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="_shell" name="Shell" classifierBehavior="_sm2">
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="_sm2" name="Idle">
        <region xmi:type="uml:Region" xmi:id="_r3"/>
      </ownedBehavior>
    </packagedElement>`

const emptyRegionApplications = `
  <sysml:Block xmi:id="_s1" base_Class="_dev"/>
  <sysml:Block xmi:id="_s2" base_Class="_shell"/>`

// A region with no vertex is not written: a state for it would be a region
// nothing enters, which the runtime refuses. The populated sibling region is
// then the state's only region and is written inline, and the machine runs.
func TestEmptyRegionsAreNotWritten(t *testing.T) {
	r := migrateDocument(t, emptyRegionMachine, emptyRegionApplications)
	for _, line := range []string{
		"state def Life {",
		"entry; then On;",
		"state On {",
		"entry; then Run;",
		"state def Idle {",
		"/* the StateMachine has no region with a vertex */",
	} {
		wantLine(t, r.Notation, line)
	}
	for _, line := range []string{"parallel", "state spare"} {
		if strings.Contains(string(r.Notation), line) {
			t.Errorf("notation writes %q for an empty region:\n%s", line, r.Notation)
		}
	}
	wantNote(t, r, "_r2", migrate.Approximated, "the region has no vertex and is not written: nothing would enter it")
	wantNote(t, r, "_r3", migrate.Approximated, "the region has no vertex and is not written: nothing would enter it")
	wantNote(t, r, "_r1", migrate.Mapped, "the one region is written as the body of its owner")

	s := session(t, r)
	meta(t, s, "%instantiate Device")
	meta(t, s, "%state Device::Life")
	if out := meta(t, s, "%current"); !strings.Contains(out, "Current state: Run") {
		t.Errorf("the machine did not enter On::Run:\n%s", out)
	}
}
