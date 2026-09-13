package pssm

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
)

// interleaving is a parallel state, entered on S1's completion, whose two
// regions each fire a traced transition on one Continue, in either order.
func interleaving() string {
	return `
          <subvertex xmi:type="uml:State" xmi:id="xP" name="P">
            <region xmi:type="uml:Region" xmi:id="xPr1" name="R1">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xPi1" name="I"/>
              <subvertex xmi:type="uml:State" xmi:id="xPa1" name="A1"/>
              <subvertex xmi:type="uml:State" xmi:id="xPa2" name="A2"/>
              <transition xmi:type="uml:Transition" xmi:id="xPt1" source="xPi1" target="xPa1"/>
              <transition xmi:type="uml:Transition" xmi:id="xTA" name="TA" source="xPa1" target="xPa2">
                <trigger xmi:type="uml:Trigger" xmi:id="xTAtrig" event="evContinue"/>
                ` + traceCall("effect", "xTAeffect", "TA(effect)") + `
              </transition>
            </region>
            <region xmi:type="uml:Region" xmi:id="xPr2" name="R2">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xPi2" name="I"/>
              <subvertex xmi:type="uml:State" xmi:id="xPb1" name="B1"/>
              <subvertex xmi:type="uml:State" xmi:id="xPb2" name="B2"/>
              <transition xmi:type="uml:Transition" xmi:id="xPt2" source="xPi2" target="xPb1"/>
              <transition xmi:type="uml:Transition" xmi:id="xTB" name="TB" source="xPb1" target="xPb2">
                <trigger xmi:type="uml:Trigger" xmi:id="xTBtrig" event="evContinue"/>
                ` + traceCall("effect", "xTBeffect", "TB(effect)") + `
              </transition>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xP"/>`
}

var interleavings = []string{"S1(entry)::TA(effect)::TB(effect)", "S1(entry)::TB(effect)::TA(effect)"}

// refereeSuite is one test package: wait -> S1 on Start, S1 traces its entry,
// body hangs off S1, and the tester sends Continue after Start.
func refereeSuite(body string) string {
	return fixtureHead + fixtureEvents +
		`  <packagedElement xmi:type="uml:Package" xmi:id="areaX" name="Area">
` + registration("Area", "semX", "Area 001", "S1(entry)") +
		`  <packagedElement xmi:type="uml:Package" xmi:id="pkgX" name="001">
    <packagedElement xmi:type="uml:Class" xmi:id="semX" name="Area001_SemanticTest">
      <generalization xmi:type="uml:Generalization" xmi:id="semXGen" general="clsSemanticTest"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="tgtX" name="Area001_Test" classifierBehavior="smX">
      <generalization xmi:type="uml:Generalization" xmi:id="tgtXGen" general="clsTarget"/>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="smX" name="Area001_Test">
        <region xmi:type="uml:Region" xmi:id="regX" name="Region1">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="xInit" name="Initial1"/>
          <subvertex xmi:type="uml:State" xmi:id="xWait" name="wait"/>
          <subvertex xmi:type="uml:State" xmi:id="xS1" name="S1">
            ` + traceCall("entry", "xS1entry", "S1(entry)") + `
          </subvertex>
          <subvertex xmi:type="uml:FinalState" xmi:id="xFin" name="FinalState1"/>
          <transition xmi:type="uml:Transition" xmi:id="xT1" name="T1" source="xInit" target="xWait"/>
          <transition xmi:type="uml:Transition" xmi:id="xT2" name="T2" source="xWait" target="xS1">
            <trigger xmi:type="uml:Trigger" xmi:id="xT2trig" event="evStart"/>
          </transition>
          ` + body + `
        </region>
      </ownedBehavior>
    </packagedElement>
` + tester("Area001_Tester", "sigContinue") + `  </packagedElement>
  </packagedElement>
` + fixtureTail
}

// refereeFixture reads the one-test referee suite, sets its admitted traces and
// its name, and referees it.
func refereeFixture(t *testing.T, body, name string, expected []string, opts Options) (*Report, *Suite) {
	t.Helper()
	s := readFixture(t, refereeSuite(body))
	noDiagnostics(t, s)
	if len(s.Tests) != 1 {
		t.Fatalf("tests = %d", len(s.Tests))
	}
	if name != "" {
		s.Tests[0].Name = name
	}
	if expected != nil {
		s.Tests[0].Expected = expected
	}
	report, err := Referee(context.Background(), s, Provenance{Document: "fixture", Tests: 1}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tests) != 1 {
		t.Fatalf("rows = %d", len(report.Tests))
	}
	return report, s
}

func wantReasons(t *testing.T, row TestReport, wants ...string) {
	t.Helper()
	for _, want := range wants {
		found := false
		for _, r := range row.Reasons {
			if strings.Contains(r, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("reasons %q lack %q", row.Reasons, want)
		}
	}
}

// A machine whose one reachable trace is the one admitted passes, with no reason.
func TestRefereePass(t *testing.T) {
	report, _ := refereeFixture(t, "", "", nil, Options{})
	row := report.Tests[0]
	if row.Bucket != BucketPass || len(row.Reasons) != 0 {
		t.Fatalf("bucket %s, reasons %q; want pass", row.Bucket, row.Reasons)
	}
	if report.Buckets[BucketPass] != 1 || len(report.Buckets) != len(Buckets) {
		t.Errorf("buckets = %v", report.Buckets)
	}
	if got := []string{"S1(entry)"}; strings.Join(row.Reached, ",") != strings.Join(got, ",") {
		t.Errorf("reached %q", row.Reached)
	}
}

// Set equality both ways: a reached trace the suite does not admit and an
// admitted trace not reached each fail the test, named.
func TestRefereeTraceSetDisagreement(t *testing.T) {
	report, _ := refereeFixture(t, "", "", []string{"S1(entry)::T9(effect)"}, Options{})
	row := report.Tests[0]
	if row.Bucket != BucketFail {
		t.Fatalf("bucket %s, want fail", row.Bucket)
	}
	wantReasons(t, row,
		"reached a trace the suite does not admit: S1(entry)",
		"admitted trace not reached: S1(entry)::T9(effect)")

	report, _ = refereeFixture(t, interleaving(), "", interleavings[:1], Options{})
	row = report.Tests[0]
	if row.Bucket != BucketFail {
		t.Fatalf("bucket %s, want fail", row.Bucket)
	}
	wantReasons(t, row, "reached a trace the suite does not admit: "+interleavings[1])
	if len(row.Reached) != 2 {
		t.Errorf("reached %q, want both interleavings", row.Reached)
	}

	report, _ = refereeFixture(t, interleaving(), "", interleavings, Options{})
	if row := report.Tests[0]; row.Bucket != BucketPass {
		t.Errorf("both admitted: bucket %s, reasons %q", row.Bucket, row.Reasons)
	}
}

// A run that ends in a typed runtime error fails naming it, and is never a pass
// however the traces compare.
func TestRefereeRunError(t *testing.T) {
	body := `
          <subvertex xmi:type="uml:Pseudostate" xmi:id="xJ" name="J" kind="junction"/>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xJ">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="xT4" name="T4" source="xJ" target="xFin" guard="xT4guard">
            <ownedRule xmi:type="uml:Constraint" xmi:id="xT4guard">
              <specification xmi:type="uml:LiteralBoolean" xmi:id="xT4spec" value="false"/>
            </ownedRule>
          </transition>`
	report, _ := refereeFixture(t, body, "", []string{"S1(entry)"}, Options{})
	row := report.Tests[0]
	if row.Bucket != BucketFail {
		t.Fatalf("bucket %s, want fail", row.Bucket)
	}
	wantReasons(t, row, "run error: ", "no guard evaluated to true")
}

// An exploration that runs out of budget fails, since the reachable set is unknown.
func TestRefereeBudgetExhausted(t *testing.T) {
	report, _ := refereeFixture(t, interleaving(), "", interleavings, Options{Budget: runtime.ExploreBudget{Runs: 1, Depth: 64}})
	row := report.Tests[0]
	if row.Bucket != BucketFail {
		t.Fatalf("bucket %s, want fail", row.Bucket)
	}
	wantReasons(t, row, "exploration ")
}

// The step budget is the referee's own unless the environment names another,
// which is then the one a run is bounded by and the one its error reports.
func TestRefereeStepBudgetFromEnvironment(t *testing.T) {
	t.Setenv(runtime.MaxStepsEnvVar, "")
	budgets, err := runBudgets()
	if err != nil {
		t.Fatal(err)
	}
	if budgets.MaxSteps != MaxSteps {
		t.Fatalf("default step budget %d, want %d", budgets.MaxSteps, MaxSteps)
	}
	if budgets.MaxStateEvents != runtime.DefaultMaxStateEvents {
		t.Fatalf("event budget %d, want the runtime's default %d", budgets.MaxStateEvents, runtime.DefaultMaxStateEvents)
	}
	t.Setenv(runtime.MaxStepsEnvVar, "3")
	report, _ := refereeFixture(t, "", "", nil, Options{})
	row := report.Tests[0]
	if row.Bucket != BucketFail {
		t.Fatalf("bucket %s, want fail under a three-step budget", row.Bucket)
	}
	wantReasons(t, row, "run error: ", "evaluation step limit exceeded (3 steps")
}

// A budget the runtime would reject is an error of the run, not a fixture failure.
func TestRefereeRejectsAMalformedStepBudget(t *testing.T) {
	t.Setenv(runtime.MaxStepsEnvVar, "plenty")
	s := readFixture(t, refereeSuite(""))
	noDiagnostics(t, s)
	_, err := Referee(context.Background(), s, Provenance{Document: "fixture", Tests: 1}, Options{})
	if err == nil || !strings.Contains(err.Error(), runtime.MaxStepsEnvVar) {
		t.Fatalf("err = %v, want one naming %s", err, runtime.MaxStepsEnvVar)
	}
}

// A suite the reader could only read in part is refused, naming every
// diagnostic, rather than measured as if its partial models were the tests.
func TestRefereeRefusesASuiteReadInPart(t *testing.T) {
	s := readFixture(t, oddSuite())
	if len(s.Problems()) == 0 {
		t.Fatal("fixture reads clean; it must not")
	}
	_, err := Referee(context.Background(), s, Provenance{Document: "fixture", Tests: 1}, Options{})
	if err == nil {
		t.Fatal("Referee measured a suite read in part")
	}
	for _, want := range []string{"not read whole", `kind "sideways"`, "target missing is not a vertex", "Odd 001: Odd_SemanticTest: no expected trace is registered"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q:\n%v", want, err)
		}
	}
}

// differs-by-design comes only from the committed table: an unmapped failure
// stays a failure; a failure mapped to a tool-choice row stays one, citing the
// row; only a failure mapped to a "differs because v2 differs" row moves.
func TestRefereeDiffersByDesignIsNotInferred(t *testing.T) {
	broken := []string{"S1(entry)::nowhere"}
	report, _ := refereeFixture(t, "", "", broken, Options{})
	if row := report.Tests[0]; row.Bucket != BucketFail || row.Row != "" {
		t.Errorf("unmapped: bucket %s row %q, want fail", row.Bucket, row.Row)
	}

	toolChoice := ""
	for name, id := range TestRows {
		if Rows[id].Kind == RowToolChoice {
			toolChoice = name
			break
		}
	}
	report, _ = refereeFixture(t, "", toolChoice, broken, Options{})
	row := report.Tests[0]
	if row.Bucket != BucketFail || row.Row != TestRows[toolChoice] {
		t.Errorf("%s: bucket %s row %q, want fail on %s", toolChoice, row.Bucket, row.Row, TestRows[toolChoice])
	}
	wantReasons(t, row, "reports on "+TestRows[toolChoice], string(RowToolChoice))

	byDesign := ""
	for name, id := range TestRows {
		if Rows[id].Kind == RowDiffersByDesign {
			byDesign = name
			break
		}
	}
	report, _ = refereeFixture(t, "", byDesign, broken, Options{})
	row = report.Tests[0]
	if row.Bucket != BucketDiffersByDesign || row.Row != TestRows[byDesign] {
		t.Errorf("%s: bucket %s row %q, want differs-by-design on %s", byDesign, row.Bucket, row.Row, TestRows[byDesign])
	}
	wantReasons(t, row, "admitted trace not reached", "reports on "+TestRows[byDesign], string(RowDiffersByDesign))

	// A mapped test that passes is a pass, whatever its row's kind.
	report, _ = refereeFixture(t, "", byDesign, nil, Options{})
	if row := report.Tests[0]; row.Bucket != BucketPass || row.Row != TestRows[byDesign] {
		t.Errorf("%s passing: bucket %s row %q, want pass on %s", byDesign, row.Bucket, row.Row, TestRows[byDesign])
	}
}

// Every row of the table names a row of the note and, when the suite is present,
// a test of the suite.
func TestRefereeRowsAreWellFormed(t *testing.T) {
	for name, id := range TestRows {
		if _, ok := Rows[id]; !ok {
			t.Errorf("%s maps to %s, which is not a row", name, id)
		}
	}
	s := loadSuite(t)
	names := map[string]bool{}
	for _, tt := range s.Tests {
		names[tt.Name] = true
	}
	for name := range TestRows {
		if !names[name] {
			t.Errorf("%s is not a test of the suite", name)
		}
	}
}

// Not-expressible and terminate-gap tests are filed without being translated
// or run, with the classifier's reason.
func TestRefereeUnrunnableBuckets(t *testing.T) {
	report, _ := refereeFixture(t, `<subvertex xmi:type="uml:Pseudostate" xmi:id="xTerm" name="Terminate1" kind="terminate"/>
          <transition xmi:type="uml:Transition" xmi:id="xT3" source="xS1" target="xTerm">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`, "", nil, Options{})
	if row := report.Tests[0]; row.Bucket != BucketTerminateGap || len(row.Reached) != 0 || row.Runs != 0 {
		t.Errorf("terminate: %+v", row)
	}

	report, _ = refereeFixture(t, `<transition xmi:type="uml:Transition" xmi:id="xT3" kind="internal" source="xS1" target="xS1">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`, "", nil, Options{})
	row := report.Tests[0]
	if row.Bucket != BucketNotExpressible || len(row.Reached) != 0 {
		t.Errorf("internal transition: %+v", row)
	}
	wantReasons(t, row, "internal transition")
}

// The encoded report is byte-identical for any -jobs.
func TestRefereeDeterministic(t *testing.T) {
	var first []byte
	for _, jobs := range []int{1, 3, 8} {
		report, _ := refereeFixture(t, interleaving(), "", interleavings[:1], Options{Jobs: jobs})
		encoded, err := report.Encode()
		if err != nil {
			t.Fatal(err)
		}
		if first == nil {
			first = encoded
		} else if !bytes.Equal(first, encoded) {
			t.Errorf("jobs=%d differs:\n%s\n---\n%s", jobs, first, encoded)
		}
	}
}

// Filtering keeps the named tests; the summary leads with the meaning of a pass.
func TestRefereeFilterAndSummary(t *testing.T) {
	s := readFixture(t, simpleSuite())
	report, err := Referee(context.Background(), s, Provenance{}, Options{Filter: "no such test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tests) != 0 {
		t.Errorf("filtered rows = %d", len(report.Tests))
	}
	if !strings.HasPrefix(report.Summary(), Meaning+"\n") {
		t.Errorf("summary does not open with the meaning:\n%s", report.Summary())
	}
}

// The gate is by count: equal counts reproduce whatever the rows say, a moved
// count fails naming the moved test, and a different suite fails on provenance.
func TestReproducesByCount(t *testing.T) {
	was, _ := refereeFixture(t, "", "", nil, Options{})
	was.Provenance.Recorded = "2000-01-01"
	same, _ := refereeFixture(t, "", "", nil, Options{})
	if err := Reproduces(was, same); err != nil {
		t.Errorf("same run: %v", err)
	}
	rowsOnly, _ := refereeFixture(t, "", "", nil, Options{})
	rowsOnly.Tests[0].Reasons = []string{"rows are not the gate"}
	if err := Reproduces(was, rowsOnly); err != nil {
		t.Errorf("rows differ, counts do not: %v", err)
	}

	moved, _ := refereeFixture(t, "", "", []string{"other"}, Options{})
	err := Reproduces(was, moved)
	if err == nil {
		t.Fatal("moved count reproduces")
	}
	for _, want := range []string{"pass: baseline 1, this run 0", "fail: baseline 0, this run 1", "Area 001 pass -> fail", "-update"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%v\nlacks %q", err, want)
		}
	}

	other, _ := refereeFixture(t, "", "", nil, Options{})
	other.Provenance.Digest = "different"
	err = Reproduces(was, other)
	if err == nil || !strings.Contains(err.Error(), "provenance") || !strings.Contains(err.Error(), "provisioning") {
		t.Errorf("other suite: %v", err)
	}
}
