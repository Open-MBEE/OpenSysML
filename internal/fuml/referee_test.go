package fuml

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/xmi"
)

// fixtureLibrary is the slice of the foundational library the fixture calls.
const fixtureLibrary = `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20110701" xmlns:uml="http://www.omg.org/spec/UML/20110701">
  <uml:Model xmi:type="uml:Model" xmi:id="_0" name="FoundationalModelLibrary">
    <packagedElement xmi:type="uml:Package" xmi:id="PrimitiveBehaviors" name="PrimitiveBehaviors">
      <packagedElement xmi:type="uml:Package" xmi:id="PrimitiveBehaviors-IntegerFunctions" name="IntegerFunctions">
        <packagedElement xmi:type="uml:FunctionBehavior" xmi:id="PrimitiveBehaviors-IntegerFunctions-plus" name="+"/>
      </packagedElement>
    </packagedElement>
  </uml:Model>
</xmi:XMI>
`

const integerType = `<type xmi:type="uml:PrimitiveType" href="pathmap://UML_LIBRARIES/UMLPrimitiveTypes.library.uml#Integer"/>`

// fixtureModel is a test model of the suite's shape: Sum adds two literals into
// an output, Twice collects two literals into a multi-valued output, Creator
// creates an object the pilot emitter does not translate, and Extent reads a
// classifier extent SysML v2 cannot spell.
const fixtureModel = `<?xml version="1.0" encoding="UTF-8"?>
<uml:Model xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="Fixture">
  <packagedElement xmi:type="uml:Activity" xmi:id="sum" name="Sum">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="sumOut" name="result" direction="out">` + integerType + `</ownedParameter>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v3" name="Value(3)">
      <result xmi:type="uml:OutputPin" xmi:id="v3r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v3v" value="3"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v2" name="Value(2)">
      <result xmi:type="uml:OutputPin" xmi:id="v2r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v2v" value="2"/>
    </node>
    <node xmi:type="uml:CallBehaviorAction" xmi:id="plus" name="Call(Plus)">
      <argument xmi:type="uml:InputPin" xmi:id="plusX" name="x">` + integerType + `</argument>
      <argument xmi:type="uml:InputPin" xmi:id="plusY" name="y">` + integerType + `</argument>
      <result xmi:type="uml:OutputPin" xmi:id="plusR" name="result">` + integerType + `</result>
      <behavior xmi:type="uml:FunctionBehavior" href="fUML_Library.xmi#PrimitiveBehaviors-IntegerFunctions-plus"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="sumOutNode" name="Parameter(result)" parameter="sumOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="s1" source="v3r" target="plusX"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="s2" source="v2r" target="plusY"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="s3" source="plusR" target="sumOutNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="twice" name="Twice">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="twiceOut" name="values" direction="out" isUnique="false">` + integerType + `
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="twiceLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="twiceHi" value="*"/>
    </ownedParameter>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="t1" name="Value(1)">
      <result xmi:type="uml:OutputPin" xmi:id="t1r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="t1v" value="1"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="t2" name="Value(2)">
      <result xmi:type="uml:OutputPin" xmi:id="t2r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="t2v" value="2"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="twiceOutNode" name="Parameter(values)" parameter="twiceOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="w1" source="t1r" target="twiceOutNode"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="w2" source="t2r" target="twiceOutNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Class" xmi:id="k" name="K"/>
  <packagedElement xmi:type="uml:Activity" xmi:id="creator" name="Creator">
    <node xmi:type="uml:CreateObjectAction" xmi:id="createK" name="Create(K)" classifier="k">
      <result xmi:type="uml:OutputPin" xmi:id="createKr" name="result" type="k"/>
    </node>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="extent" name="Extent">
    <node xmi:type="uml:ReadExtentAction" xmi:id="readK" name="ReadExtent(K)" classifier="k">
      <result xmi:type="uml:OutputPin" xmi:id="readKr" name="result"/>
    </node>
  </packagedElement>
</uml:Model>
`

// exceptionFixture is an exception-test model of one activity.
const exceptionFixture = `<?xml version="1.0" encoding="UTF-8"?>
<uml:Model xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="x" name="Exceptions">
  <packagedElement xmi:type="uml:Activity" xmi:id="raiser" name="Raiser">
    <node xmi:type="uml:RaiseExceptionAction" xmi:id="raise" name="Raise">
      <exception xmi:type="uml:InputPin" xmi:id="raiseIn" name="exception"/>
    </node>
  </packagedElement>
</uml:Model>
`

// fixtureSuite reads the fixture library and the two fixture models as a Suite.
func fixtureSuite(t *testing.T, tests string) *Suite {
	t.Helper()
	doc, err := xmi.Parse(strings.NewReader(fixtureLibrary))
	if err != nil {
		t.Fatal(err)
	}
	lib := &Library{doc: doc}
	s := &Suite{Library: lib}
	for _, m := range []struct {
		file, text string
		into       **Model
	}{{TestsFile, tests, &s.Tests}, {ExceptionTestsFile, exceptionFixture, &s.Exception}} {
		if *m.into, err = ReadModel(strings.NewReader(m.text), lib); err != nil {
			t.Fatal(err)
		}
		(*m.into).File = m.file
	}
	return s
}

func fixtureActivity(t *testing.T, s *Suite, name string) *Activity {
	t.Helper()
	for _, a := range s.Tests.Activities {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("no activity %s", name)
	return nil
}

// integers is one output parameter's recorded values.
func integers(parameter string, values ...int) ExpectedOutput {
	o := ExpectedOutput{Parameter: parameter}
	for _, v := range values {
		raw, _ := json.Marshal(v)
		o.Values = append(o.Values, ExpectedValue{Kind: "Integer", Value: raw})
	}
	return o
}

// executed is the implementation's record of one execution of a: its outputs
// and a trace firing each named action once, in order.
func executed(a *Activity, outputs []ExpectedOutput, fired ...string) ExpectedActivity {
	x := ExpectedActivity{Model: a.Model.File, ID: a.ID, Name: a.Name, Executed: true, Outputs: outputs}
	x.Events = append(x.Events, ExpectedEvent{Kind: "Execute", Activity: a.Name, ID: a.ID})
	for _, f := range fired {
		ev := ExpectedEvent{Kind: "Fire", Activity: a.Name, Action: f, ID: f}
		if n := a.NodeNamed(f); n != nil {
			ev.ID = n.ID
		}
		x.Events = append(x.Events, ev)
	}
	x.Events = append(x.Events, ExpectedEvent{Kind: "Complete", Activity: a.Name, ID: a.ID})
	return x
}

func fixtureExpected(t *testing.T, s *Suite) *Expected {
	t.Helper()
	sum, twice := fixtureActivity(t, s, "Sum"), fixtureActivity(t, s, "Twice")
	return &Expected{Activities: []ExpectedActivity{
		executed(sum, []ExpectedOutput{integers("result", 5)}, "Value(3)", "Value(2)", "Call(Plus)"),
		executed(twice, []ExpectedOutput{integers("values", 2, 1)}, "Value(2)", "Value(1)"),
	}}
}

func refereed(t *testing.T, s *Suite, x *Expected, opts Options) *Report {
	t.Helper()
	report, err := Referee(context.Background(), s, x, Provenance{RITag: "fixture", Activities: s.Activities()}, opts)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func row(t *testing.T, r *Report, name string) ActivityReport {
	t.Helper()
	for _, a := range r.Activities {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("no row %s in %+v", name, r.Activities)
	return ActivityReport{}
}

func wantReasons(t *testing.T, row ActivityReport, wants ...string) {
	t.Helper()
	for _, want := range wants {
		found := false
		for _, r := range row.Reasons {
			if strings.Contains(r, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: reasons %q lack %q", row.Name, row.Reasons, want)
		}
	}
}

// Every activity lands in the bucket its class and its run decide: a run that
// agrees passes, a construct the emitter declines is not-expressible naming it
// with the class kept expressible, a not-expressible activity is filed with the
// classifier's reason and never run, and an expressible activity the
// implementation did not execute fails.
func TestRefereeBuckets(t *testing.T) {
	s := fixtureSuite(t, fixtureModel)
	x := fixtureExpected(t, s)
	x.Activities = append(x.Activities, ExpectedActivity{Model: TestsFile, ID: "creator", Name: "Creator", Skipped: "no test executes it"})
	r := refereed(t, s, x, Options{})
	if r.Buckets[BucketPass] != 2 || r.Buckets[BucketFail] != 0 || r.Buckets[BucketNotExpressible] != 3 || r.Buckets[BucketDiffersByDesign] != 0 {
		t.Errorf("buckets = %v", r.Buckets)
	}
	sum := row(t, r, "Sum")
	if sum.Bucket != BucketPass || len(sum.Reasons) != 0 || sum.Fired != "" || sum.Runs == 0 || !strings.HasPrefix(sum.Status, "complete") {
		t.Errorf("Sum = %+v", sum)
	}
	if strings.Join(sum.Expected, ";") != "result = 5" || strings.Join(sum.Reached, ";") != "result = 5" {
		t.Errorf("Sum outputs = %q / %q", sum.Expected, sum.Reached)
	}
	twice := row(t, r, "Twice")
	if twice.Bucket != BucketPass || strings.Join(twice.Reached, ";") != "values = 1, 2" {
		t.Errorf("Twice = %+v", twice)
	}
	creator := row(t, r, "Creator")
	if creator.Bucket != BucketNotExpressible || creator.Class != "expressible" || creator.Runs != 0 || len(creator.Reasons) != 1 {
		t.Errorf("Creator = %+v", creator)
	}
	wantReasons(t, creator, "not yet translated: Create(K): CreateObjectAction")
	extent := row(t, r, "Extent")
	if extent.Bucket != BucketNotExpressible || extent.Runs != 0 || len(extent.Reasons) != 1 {
		t.Errorf("Extent = %+v", extent)
	}
	wantReasons(t, extent, "ReadExtentAction", "no classifier extent")
	raiser := row(t, r, "Raiser")
	if raiser.Bucket != BucketNotExpressible || raiser.Model != ExceptionTestsFile {
		t.Errorf("Raiser = %+v", raiser)
	}
	wantReasons(t, raiser, "exception-test model")
}

// An expressible activity without an execution in the record cannot be
// compared and fails saying so, with the driver's reason where it left one.
func TestRefereeNoRecord(t *testing.T) {
	s := fixtureSuite(t, fixtureModel)
	x := &Expected{Activities: []ExpectedActivity{{Model: TestsFile, ID: "twice", Name: "Twice", Skipped: "the test harness has no case for it"}}}
	r := refereed(t, s, x, Options{Filter: "Sum"})
	if r.Buckets[BucketFail] != 1 || len(r.Activities) != 1 {
		t.Fatalf("report = %+v", r)
	}
	wantReasons(t, r.Activities[0], "record has no execution of it")
	r = refereed(t, s, x, Options{Filter: "Twice"})
	wantReasons(t, r.Activities[0], "record has no execution of it: the test harness has no case for it")
}

// A run whose outputs differ from the record fails naming both; the differing
// values, and the actions that fired in one but produced no value in the other,
// are reported.
func TestRefereeDisagreement(t *testing.T) {
	s := fixtureSuite(t, fixtureModel)
	sum := fixtureActivity(t, s, "Sum")
	x := &Expected{Activities: []ExpectedActivity{executed(sum, []ExpectedOutput{integers("result", 6)}, "Value(3)")}}
	r := refereed(t, s, x, Options{Filter: "Sum"})
	got := r.Activities[0]
	if got.Bucket != BucketFail || strings.Join(got.Expected, ";") != "result = 6" {
		t.Errorf("Sum = %+v", got)
	}
	wantReasons(t, got, "outputs differ: result = 5")
	if !strings.Contains(got.Fired, "produced a value here only: Call(Plus), Value(2)") {
		t.Errorf("fired = %q", got.Fired)
	}

	x = &Expected{Activities: []ExpectedActivity{executed(sum, nil, "Value(3)", "Value(2)", "Call(Plus)", "Absent")}}
	r = refereed(t, s, x, Options{Filter: "Sum"})
	got = r.Activities[0]
	if strings.Join(got.Expected, ";") != "result = -" || got.Bucket != BucketFail {
		t.Errorf("absent output: %+v", got)
	}
}

// The classifier's differs-by-design is never inferred from a run: an activity
// whose trace re-fires an object-fed action is filed there whether or not the
// run agrees,
// with the classifier's reason first.
func TestRefereeDiffersByDesignIsFixed(t *testing.T) {
	s := fixtureSuite(t, fixtureModel)
	sum := fixtureActivity(t, s, "Sum")
	agreeing := executed(sum, []ExpectedOutput{integers("result", 5)}, "Value(3)", "Value(2)", "Call(Plus)", "Call(Plus)")
	disagreeing := executed(sum, []ExpectedOutput{integers("result", 7)}, "Value(3)", "Value(2)", "Call(Plus)", "Call(Plus)")
	for _, x := range []ExpectedActivity{agreeing, disagreeing} {
		r := refereed(t, s, &Expected{Activities: []ExpectedActivity{x}}, Options{Filter: "Sum"})
		got := r.Activities[0]
		if got.Bucket != BucketDiffersByDesign || got.Class != "differs-by-design" || got.Runs == 0 {
			t.Errorf("Sum = %+v", got)
		}
		if !strings.Contains(got.Reasons[0], "per-token") {
			t.Errorf("reasons open with %q", got.Reasons[0])
		}
	}
}

// The encoded report is byte-identical for any -jobs.
func TestRefereeDeterministic(t *testing.T) {
	s := fixtureSuite(t, fixtureModel)
	x := fixtureExpected(t, s)
	var first []byte
	for _, jobs := range []int{1, 3, 8} {
		encoded, err := refereed(t, s, x, Options{Jobs: jobs}).Encode()
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

// Filtering keeps the named activities; the summary leads with the meaning of
// a pass and lists every bucket's rows but pass's.
func TestRefereeFilterAndSummary(t *testing.T) {
	s := fixtureSuite(t, fixtureModel)
	x := fixtureExpected(t, s)
	if r := refereed(t, s, x, Options{Filter: "no such activity"}); len(r.Activities) != 0 || len(r.Buckets) != 4 {
		t.Errorf("filtered report = %+v", r)
	}
	summary := refereed(t, s, x, Options{}).Summary()
	if !strings.HasPrefix(summary, Meaning+"\n") {
		t.Errorf("summary does not open with the meaning:\n%s", summary)
	}
	for _, want := range []string{"pass                 2", "fail                 0", "not-expressible      3", "differs-by-design    0", "\nnot-expressible:\n  Creator\n    not yet translated", "\n  Extent\n    ReadExtentAction"} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary lacks %q:\n%s", want, summary)
		}
	}
	if strings.Contains(summary, "  Sum\n") {
		t.Errorf("summary lists a pass:\n%s", summary)
	}
}

// A suite the reader could not read whole is refused, so a count is always
// the whole suite's.
func TestRefereeRefusesASuiteReadInPart(t *testing.T) {
	broken := strings.Replace(fixtureModel, `target="sumOutNode"`, `target="nowhere"`, 1)
	s := fixtureSuite(t, broken)
	_, err := Referee(context.Background(), s, fixtureExpected(t, s), Provenance{}, Options{})
	if err == nil || !strings.Contains(err.Error(), "not read whole") || !strings.Contains(err.Error(), TestsFile+": ") {
		t.Errorf("partial suite: %v", err)
	}
}

// The gate is by count: equal counts reproduce whatever the rows say, a moved
// count fails naming the moved activity, and a different suite fails on provenance.
func TestReproducesByCount(t *testing.T) {
	s := fixtureSuite(t, fixtureModel)
	x := fixtureExpected(t, s)
	was := refereed(t, s, x, Options{})
	was.Provenance.Recorded, was.Provenance.Develop = "2000-01-01", "abc"
	if err := Reproduces(was, refereed(t, s, x, Options{})); err != nil {
		t.Errorf("same run: %v", err)
	}
	rowsOnly := refereed(t, s, x, Options{})
	rowsOnly.Activities[0].Reasons = []string{"rows are not the gate"}
	if err := Reproduces(was, rowsOnly); err != nil {
		t.Errorf("rows differ, counts do not: %v", err)
	}

	sum := fixtureActivity(t, s, "Sum")
	moved := refereed(t, s, &Expected{Activities: []ExpectedActivity{x.Activities[1], executed(sum, []ExpectedOutput{integers("result", 6)})}}, Options{})
	err := Reproduces(was, moved)
	if err == nil {
		t.Fatal("moved count reproduces")
	}
	for _, want := range []string{"pass: baseline 2, this run 1", "fail: baseline 0, this run 1", "Sum pass -> fail", "-update"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%v\nlacks %q", err, want)
		}
	}

	other := refereed(t, s, x, Options{})
	other.Provenance.JarDigest = "different"
	err = Reproduces(was, other)
	if err == nil || !strings.Contains(err.Error(), "provenance") || !strings.Contains(err.Error(), "provisioning") {
		t.Errorf("other suite: %v", err)
	}
}
