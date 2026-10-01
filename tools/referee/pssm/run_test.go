package pssm

import (
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
	oreport "github.com/Open-MBEE/OpenSysML/tools/oracle/report"
)

// testerStep is one statement of a tester's behavior after Start: a send, a call
// (args as `uml:LiteralInteger=5`; traceResult traces its result) or a trace.
type testerStep struct {
	send, call, trace string
	args              []string
	traceResult       bool
}

// testerWith writes a Tester class whose classifier behavior accepts Start and
// then performs the steps on `this.testable`, in order.
func testerWith(id string, steps ...testerStep) string {
	var b strings.Builder
	b.WriteString(`    <packagedElement xmi:type="uml:Class" xmi:id="` + id + `" name="` + id + `" classifierBehavior="` + id + `Beh">
      <generalization xmi:type="uml:Generalization" xmi:id="` + id + `Gen" general="clsTester"/>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="` + id + `Beh" name="` + id + `Behavior">
        <node xmi:type="uml:InitialNode" xmi:id="` + id + `Init"/>
        <node xmi:type="uml:AcceptEventAction" xmi:id="` + id + `Accept" name="AcceptStart">
          <trigger xmi:type="uml:Trigger" xmi:id="` + id + `Trig" event="evStart"/>
        </node>
        <edge xmi:type="uml:ControlFlow" xmi:id="` + id + `E0" source="` + id + `Init" target="` + id + `Accept"/>
`)
	prev := id + "Accept"
	for i, step := range steps {
		s := id + "Step" + itoa(i)
		b.WriteString(`        <node xmi:type="uml:ReadSelfAction" xmi:id="` + s + `Self"><result xmi:type="uml:OutputPin" xmi:id="` + s + `SelfOut"/></node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="` + s + `Read" structuralFeature="testerTestable">
          <object xmi:type="uml:InputPin" xmi:id="` + s + `ReadObj"/>
          <result xmi:type="uml:OutputPin" xmi:id="` + s + `ReadOut"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E1" source="` + s + `SelfOut" target="` + s + `ReadObj"/>
`)
		switch {
		case step.send != "":
			b.WriteString(`        <node xmi:type="uml:SendSignalAction" xmi:id="` + s + `" name="Send" signal="` + step.send + `">
          <target xmi:type="uml:InputPin" xmi:id="` + s + `Target"/>
` + argumentPins(s, step.args) + `        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E2" source="` + s + `ReadOut" target="` + s + `Target"/>
` + literalArguments(s, step.args))
		case step.call != "" && step.traceResult:
			b.WriteString(`        <node xmi:type="uml:CallOperationAction" xmi:id="` + s + `Call" name="Call" operation="` + step.call + `">
          <target xmi:type="uml:InputPin" xmi:id="` + s + `Target"/>
` + argumentPins(s, step.args) + `          <result xmi:type="uml:OutputPin" xmi:id="` + s + `Result"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E2" source="` + s + `ReadOut" target="` + s + `Target"/>
` + literalArguments(s, step.args) + `        <node xmi:type="uml:ValueSpecificationAction" xmi:id="` + s + `Dir">
          <result xmi:type="uml:OutputPin" xmi:id="` + s + `DirOut"/>
          <value xmi:type="uml:LiteralBoolean" xmi:id="` + s + `DirLit" value="false"/>
        </node>
        <node xmi:type="uml:CallBehaviorAction" xmi:id="` + s + `Fmt" behavior="fmtPV">
          <argument xmi:type="uml:InputPin" xmi:id="` + s + `FmtA0"/>
          <argument xmi:type="uml:InputPin" xmi:id="` + s + `FmtA1"/>
          <result xmi:type="uml:OutputPin" xmi:id="` + s + `FmtOut"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E5" source="` + s + `DirOut" target="` + s + `FmtA0"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E6" source="` + s + `Result" target="` + s + `FmtA1"/>
        <node xmi:type="uml:ReadSelfAction" xmi:id="` + s + `Self2"><result xmi:type="uml:OutputPin" xmi:id="` + s + `Self2Out"/></node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="` + s + `Read2" structuralFeature="testerTestable">
          <object xmi:type="uml:InputPin" xmi:id="` + s + `Read2Obj"/>
          <result xmi:type="uml:OutputPin" xmi:id="` + s + `Read2Out"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E7" source="` + s + `Self2Out" target="` + s + `Read2Obj"/>
        <node xmi:type="uml:CallOperationAction" xmi:id="` + s + `" name="Trace" operation="opTrace">
          <target xmi:type="uml:InputPin" xmi:id="` + s + `Target2"/>
          <argument xmi:type="uml:InputPin" xmi:id="` + s + `Arg"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E8" source="` + s + `Read2Out" target="` + s + `Target2"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E9" source="` + s + `FmtOut" target="` + s + `Arg"/>
`)
		case step.call != "":
			b.WriteString(`        <node xmi:type="uml:CallOperationAction" xmi:id="` + s + `" name="Call" operation="` + step.call + `">
          <target xmi:type="uml:InputPin" xmi:id="` + s + `Target"/>
` + argumentPins(s, step.args) + `        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E2" source="` + s + `ReadOut" target="` + s + `Target"/>
` + literalArguments(s, step.args))
		default:
			b.WriteString(`        <node xmi:type="uml:ValueSpecificationAction" xmi:id="` + s + `Val">
          <result xmi:type="uml:OutputPin" xmi:id="` + s + `ValOut"/>
          <value xmi:type="uml:LiteralString" xmi:id="` + s + `Lit" value="` + step.trace + `"/>
        </node>
        <node xmi:type="uml:CallOperationAction" xmi:id="` + s + `" name="Trace" operation="opTrace">
          <target xmi:type="uml:InputPin" xmi:id="` + s + `Target"/>
          <argument xmi:type="uml:InputPin" xmi:id="` + s + `Arg"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E2" source="` + s + `ReadOut" target="` + s + `Target"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E4" source="` + s + `ValOut" target="` + s + `Arg"/>
`)
		}
		b.WriteString(`        <edge xmi:type="uml:ControlFlow" xmi:id="` + s + `E3" source="` + prev + `" target="` + s + `"/>
`)
		prev = s
	}
	b.WriteString("      </ownedBehavior>\n    </packagedElement>\n")
	return b.String()
}

// argumentPins writes one argument pin per literal argument of action s.
func argumentPins(s string, args []string) string {
	var b strings.Builder
	for i := range args {
		b.WriteString(`          <argument xmi:type="uml:InputPin" xmi:id="` + s + `Arg` + itoa(i) + `"/>
`)
	}
	return b.String()
}

// literalArguments writes the value specification feeding each argument pin of
// action s.
func literalArguments(s string, args []string) string {
	var b strings.Builder
	for i, arg := range args {
		typ, value, _ := strings.Cut(arg, "=")
		v := s + "Lit" + itoa(i)
		b.WriteString(`        <node xmi:type="uml:ValueSpecificationAction" xmi:id="` + v + `">
          <result xmi:type="uml:OutputPin" xmi:id="` + v + `Out"/>
          <value xmi:type="` + typ + `" xmi:id="` + v + `Val" value="` + value + `"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + v + `E" source="` + v + `Out" target="` + s + `Arg` + itoa(i) + `"/>
`)
	}
	return b.String()
}

// callSuite is one test package whose target has an operation `op` the machine
// takes as a call event: wait -> S1 on Start; S1 -> S2 on op, S1's exit and
// the effect traced; S2 -> final on Continue, S2's exit traced. The tester
// performs the given steps after Start.
func callSuite(expected string, steps ...testerStep) string {
	return fixtureHead + fixtureEvents +
		`  <packagedElement xmi:type="uml:CallEvent" xmi:id="evOp" operation="opOp"/>
  <packagedElement xmi:type="uml:Package" xmi:id="areaX" name="Area">
` + registration("Area", "semX", "Area 001", expected) +
		`  <packagedElement xmi:type="uml:Package" xmi:id="pkgX" name="001">
    <packagedElement xmi:type="uml:Class" xmi:id="semX" name="Area001_SemanticTest">
      <generalization xmi:type="uml:Generalization" xmi:id="semXGen" general="clsSemanticTest"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="tgtX" name="Area001_Test" classifierBehavior="smX">
      <generalization xmi:type="uml:Generalization" xmi:id="tgtXGen" general="clsTarget"/>
      <ownedOperation xmi:type="uml:Operation" xmi:id="opOp" name="op"/>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="smX" name="Area001_Test">
        <region xmi:type="uml:Region" xmi:id="regX" name="Region1">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="xInit" name="Initial1"/>
          <subvertex xmi:type="uml:State" xmi:id="xWait" name="wait"/>
          <subvertex xmi:type="uml:State" xmi:id="xS1" name="S1">
            ` + traceCall("exit", "xS1exit", "S1(exit)") + `
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            ` + traceCall("exit", "xS2exit", "S2(exit)") + `
          </subvertex>
          <subvertex xmi:type="uml:FinalState" xmi:id="xFin" name="FinalState1"/>
          <transition xmi:type="uml:Transition" xmi:id="xT1" name="T1" source="xInit" target="xWait"/>
          <transition xmi:type="uml:Transition" xmi:id="xT2" name="T2" source="xWait" target="xS1">
            <trigger xmi:type="uml:Trigger" xmi:id="xT2trig" event="evStart"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evOp"/>
            ` + traceCall("effect", "xT3effect", "Call(op)") + `
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="xT4" name="T4" source="xS2" target="xFin">
            <trigger xmi:type="uml:Trigger" xmi:id="xT4trig" event="evContinue"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>
` + testerWith("Area001_Tester", steps...) + `  </packagedElement>
  </packagedElement>
` + fixtureTail
}

// refereeCallSuite reads the call suite and referees it.
func refereeCallSuite(t *testing.T, expected string, steps ...testerStep) (*Report, *Suite) {
	t.Helper()
	s := readFixture(t, callSuite(expected, steps...))
	noDiagnostics(t, s)
	if len(s.Tests) != 1 {
		t.Fatalf("tests = %d", len(s.Tests))
	}
	report, err := Referee(t.Context(), s, Provenance{Document: "fixture", Tests: 1}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tests) != 1 {
		t.Fatalf("rows = %d", len(report.Tests))
	}
	return report, s
}

// The tester's trace of a returned call lands in the log after the run-to-
// completion step that handled the call and before the next stimulus: between
// the effect the call fired and the exit the later Continue fires.
func TestDriveTesterTraceAfterCallReturns(t *testing.T) {
	report, s := refereeCallSuite(t, "S1(exit)::Call(op)::End::S2(exit)",
		testerStep{call: "opOp"}, testerStep{trace: "End"}, testerStep{send: "sigContinue"})
	row := report.Tests[0]
	if row.Bucket != oreport.BucketPass || len(row.Reasons) != 0 {
		t.Fatalf("bucket %s, reasons %q; want pass", row.Bucket, row.Reasons)
	}
	if strings.Join(row.Reached, ",") != "S1(exit)::Call(op)::End::S2(exit)" {
		t.Errorf("reached %q", row.Reached)
	}
	stimuli, err := Stimulation(s, s.Tests[0])
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Start", "op()", `trace("End")`, "Continue"}
	if len(stimuli) != len(want) {
		t.Fatalf("stimuli = %v", stimuli)
	}
	for i, st := range stimuli {
		if st.String() != want[i] {
			t.Errorf("stimulus %d = %s, want %s", i, st, want[i])
		}
	}
	if c := Classify(s.Tests[0]); c.Class != Standard {
		t.Errorf("classified %s (%s)", c.Class, c.Reason())
	}
}

// A trace the tester makes right after a send, or first of all, may run before
// or after the machine's own step: the referee refuses it rather than pick an
// order, and the classifier names the same statement.
func TestDriveRefusesATraceWhileTheMachineMayRun(t *testing.T) {
	cases := []struct {
		name  string
		steps []testerStep
	}{
		{"after a send", []testerStep{{send: "sigContinue"}, {trace: "End"}}},
		{"first", []testerStep{{trace: "End"}, {call: "opOp"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := readFixture(t, callSuite("S1(exit)", tc.steps...))
			noDiagnostics(t, s)
			_, err := Stimulation(s, s.Tests[0])
			var te *TranslateError
			if !errors.As(err, &te) || te.Reason != `this.testable.trace("End") traces while the machine may still be running` {
				t.Errorf("Stimulation err = %v", err)
			}
			c := Classify(s.Tests[0])
			if c.Class != NotExpressible || c.Reason() != `tester trace this.testable.trace("End")` {
				t.Errorf("classified %s (%s)", c.Class, c.Reason())
			}
		})
	}
}

// A call the machine leaves queued never returns to its caller: the run is an
// error and the test fails, named. An unhandled call in a running machine is
// discarded and returns, as UML does.
func TestDriveCallNotReturnedFails(t *testing.T) {
	report, _ := refereeCallSuite(t, "S1(exit)::Call(op)::End::S2(exit)",
		testerStep{call: "opOp"}, testerStep{call: "opOp"}, testerStep{trace: "End"}, testerStep{send: "sigContinue"})
	if row := report.Tests[0]; row.Bucket != oreport.BucketPass {
		t.Fatalf("discarded call: bucket %s, reasons %q; want pass", row.Bucket, row.Reasons)
	}

	report, _ = refereeCallSuite(t, "S1(exit)::Call(op)::S2(exit)::End",
		testerStep{call: "opOp"}, testerStep{send: "sigContinue"}, testerStep{call: "opOp"}, testerStep{trace: "End"})
	row := report.Tests[0]
	if row.Bucket != oreport.BucketFail {
		t.Fatalf("bucket %s, reasons %q; want fail", row.Bucket, row.Reasons)
	}
	wantReasons(t, row, "op(): ", "call not returned")
}

// countingCaller returns fixed outputs and counts the calls made of each operation.
type countingCaller struct {
	made    map[string]int
	outputs map[string]runtime.Value
}

func (c *countingCaller) Call(operation string, _ map[string]runtime.Value) (map[string]runtime.Value, error) {
	c.made[operation]++
	return c.outputs, nil
}

// One call action whose outputs the tester reads twice is one call of the
// machine: both reads come from the outputs that call returned.
func TestTracerMakesEachCallOnce(t *testing.T) {
	boolean := func(b bool) runtime.Value {
		return runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValBool, Bool: b}}
	}
	exec := &countingCaller{
		made:    map[string]int{},
		outputs: map[string]runtime.Value{"result": boolean(false), "return": boolean(true)},
	}
	format := &LibraryBehavior{Qualified: "Util::Tracing::formatParameterValue"}
	concat := &LibraryBehavior{Qualified: "StringFunctions::Concat"}
	read := func(result string) Expr {
		return Expr{Kind: ExprApply, Name: "formatParameterValue", Library: format, Args: []Expr{
			{Kind: ExprLiteral, Literal: &Literal{Kind: LiteralBoolean, Text: "false", Present: true}},
			{Kind: ExprCall, Name: "or", ID: "call-or", Result: result},
		}}
	}
	x := &Expr{Kind: ExprApply, Name: "Concat", Library: concat, Args: []Expr{read("return"), read("result")}}
	tr := &tracer{exec: exec, calls: map[string]runtime.QueuedEvent{"call-or": {Call: "or"}}, outputs: map[string]map[string]runtime.Value{}}
	v, err := tr.value(x)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != runtime.ValString || v.Str() != "[out=true][out=false]" {
		t.Errorf("traced %v, want [out=true][out=false]", v)
	}
	if exec.made["or"] != 1 {
		t.Errorf("or called %d times, want once", exec.made["or"])
	}

	tr = &tracer{exec: exec, calls: map[string]runtime.QueuedEvent{}, outputs: map[string]map[string]runtime.Value{}}
	if _, err := tr.value(x); err == nil || !strings.Contains(err.Error(), "not a call the stimulation bound") {
		t.Errorf("unbound call err = %v", err)
	}
}
