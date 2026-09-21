package pssm

import (
	"context"
	"fmt"
	"strings"
	"testing"

	oreport "github.com/Open-MBEE/OpenSysML/tools/oracle/report"
)

const fumlLibrary = "http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#PrimitiveBehaviors-"

// tracingLibrary is the suite's Util::Tracing package, as far as the machine's
// behaviors reference it.
const tracingLibrary = `  <packagedElement xmi:type="uml:Package" xmi:id="utilT" name="Util">
    <packagedElement xmi:type="uml:Package" xmi:id="utilTracing" name="Tracing">
      <packagedElement xmi:type="uml:Activity" xmi:id="fmtPV" name="formatParameterValue"/>
    </packagedElement>
  </packagedElement>
`

// param is one parameter of a fixture behavior: typ is a primitive type name
// or the id of a signal.
type param struct{ name, dir, typ string }

func (p param) decl(id string) string {
	typ, attr := `<type href="`+primitiveTypes+p.typ+`"/>`, ""
	if strings.HasPrefix(p.typ, "sig") {
		typ, attr = "", ` type="`+p.typ+`"`
	}
	return `  <ownedParameter xmi:type="uml:Parameter" xmi:id="` + id + `" name="` + p.name + `" direction="` + p.dir + `"` + attr + `>` + typ + `</ownedParameter>
  <node xmi:type="uml:ActivityParameterNode" xmi:id="` + id + `Node" parameter="` + id + `"/>
`
}

// behaviorBuilder accumulates the nodes and flows of a fixture activity.
type behaviorBuilder struct {
	id          string
	nodes, flow strings.Builder
	n, e        int
}

func (b *behaviorBuilder) fresh() string {
	b.n++
	return fmt.Sprintf("%sN%d", b.id, b.n)
}

func (b *behaviorBuilder) flows(src, dst string) {
	b.e++
	fmt.Fprintf(&b.flow, `  <edge xmi:type="uml:ObjectFlow" xmi:id="%sE%d" source="%s" target="%s"/>`+"\n", b.id, b.e, src, dst)
}

// literal adds a value specification and returns its result pin.
func (b *behaviorBuilder) literal(kind, value string) string {
	id := b.fresh()
	fmt.Fprintf(&b.nodes, `  <node xmi:type="uml:ValueSpecificationAction" xmi:id="%s"><result xmi:type="uml:OutputPin" xmi:id="%sOut"/><value xmi:type="%s" xmi:id="%sLit" value="%s"/></node>`+"\n", id, id, kind, id, value)
	return id + "Out"
}

// primitive adds a call of a fUML primitive over the given sources and
// returns its result pin.
func (b *behaviorBuilder) primitive(qualified string, args ...string) string {
	id := b.fresh()
	fmt.Fprintf(&b.nodes, `  <node xmi:type="uml:CallBehaviorAction" xmi:id="%s"><behavior href="%s%s"/>`, id, fumlLibrary, qualified)
	for i, a := range args {
		fmt.Fprintf(&b.nodes, `<argument xmi:type="uml:InputPin" xmi:id="%sA%d"/>`, id, i)
		b.flows(a, fmt.Sprintf("%sA%d", id, i))
	}
	fmt.Fprintf(&b.nodes, `<result xmi:type="uml:OutputPin" xmi:id="%sOut"/></node>`+"\n", id)
	return id + "Out"
}

// format adds formatParameterValue(<in>, value) and returns its result pin.
func (b *behaviorBuilder) format(in bool, value string) string {
	id := b.fresh()
	fmt.Fprintf(&b.nodes, `  <node xmi:type="uml:CallBehaviorAction" xmi:id="%s" behavior="fmtPV"><argument xmi:type="uml:InputPin" xmi:id="%sA0"/><argument xmi:type="uml:InputPin" xmi:id="%sA1"/><result xmi:type="uml:OutputPin" xmi:id="%sOut"/></node>`+"\n", id, id, id, id)
	b.flows(b.literal("uml:LiteralBoolean", fmt.Sprint(in)), id+"A0")
	b.flows(value, id+"A1")
	return id + "Out"
}

// trace adds this.trace(value).
func (b *behaviorBuilder) trace(value string) {
	id := b.fresh()
	fmt.Fprintf(&b.nodes, `  <node xmi:type="uml:ReadSelfAction" xmi:id="%sSelf"><result xmi:type="uml:OutputPin" xmi:id="%sSelfOut"/></node>
  <node xmi:type="uml:CallOperationAction" xmi:id="%s" operation="opTrace"><target xmi:type="uml:InputPin" xmi:id="%sTarget"/><argument xmi:type="uml:InputPin" xmi:id="%sArg"/></node>`+"\n", id, id, id, id, id)
	b.flows(id+"SelfOut", id+"Target")
	b.flows(value, id+"Arg")
}

// paramTrace writes an activity <tag> tracing `<segment>` and formatParameterValue of
// each parameter, every output the Or of the first two inputs (the suite's Event 019 shape).
func paramTrace(tag, id, segment string, params ...param) string {
	b := &behaviorBuilder{id: id}
	var decls strings.Builder
	var ins, values []string
	for i, p := range params {
		pid := fmt.Sprintf("%sP%d", id, i)
		decls.WriteString(p.decl(pid))
		if p.dir == "in" {
			ins = append(ins, pid+"Node")
			values = append(values, b.format(true, pid+"Node"))
		}
	}
	for i, p := range params {
		if p.dir == "in" {
			continue
		}
		pid := fmt.Sprintf("%sP%d", id, i)
		b.flows(b.primitive("BooleanFunctions-Or", ins[0], ins[1]), pid+"Node")
		values = append(values, b.format(false, b.primitive("BooleanFunctions-Or", ins[0], ins[1])))
	}
	acc := values[len(values)-1]
	for i := len(values) - 2; i >= 0; i-- {
		acc = b.primitive("StringFunctions-Concat", values[i], acc)
	}
	b.trace(b.primitive("StringFunctions-Concat", b.literal("uml:LiteralString", segment), acc))
	return `<` + tag + ` xmi:type="uml:Activity" xmi:id="` + id + `" name="` + id + `">
` + decls.String() + b.nodes.String() + b.flow.String() + `</` + tag + `>
`
}

// orOperation is `or(in left, in right, out result): Boolean`, the suite's
// Event 019 E operation, with its call event.
const orOperation = `      <ownedOperation xmi:type="uml:Operation" xmi:id="opOr" name="or">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="opOrL" name="left" direction="in"><type href="` + primitiveTypes + `Boolean"/></ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="opOrR" name="right" direction="in"><type href="` + primitiveTypes + `Boolean"/></ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="opOrO" name="result" direction="out"><type href="` + primitiveTypes + `Boolean"/></ownedParameter>
        <ownedParameter xmi:type="uml:Parameter" xmi:id="opOrRet" name="return" direction="return"><type href="` + primitiveTypes + `Boolean"/></ownedParameter>
      </ownedOperation>
`

var orParams = []param{{"left", "in", "Boolean"}, {"right", "in", "Boolean"}, {"result", "out", "Boolean"}, {"return", "return", "Boolean"}}

// parameterSuite is one test package: the target owns the operations, its machine
// goes Initial -> wait then the body, and the tester performs the steps after Start.
func parameterSuite(operations, body string, expected []string, steps ...testerStep) string {
	return fixtureHead + fixtureEvents + tracingLibrary +
		`  <packagedElement xmi:type="uml:CallEvent" xmi:id="evOr" operation="opOr"/>
  <packagedElement xmi:type="uml:Package" xmi:id="areaX" name="Area">
` + registration("Area", "semX", "Area 001", expected...) +
		`  <packagedElement xmi:type="uml:Package" xmi:id="pkgX" name="001">
    <packagedElement xmi:type="uml:Class" xmi:id="semX" name="Area001_SemanticTest">
      <generalization xmi:type="uml:Generalization" xmi:id="semXGen" general="clsSemanticTest"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="tgtX" name="Area001_Test" classifierBehavior="smX">
      <generalization xmi:type="uml:Generalization" xmi:id="tgtXGen" general="clsTarget"/>
` + operations + `      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="smX" name="Area001_Test">
        <region xmi:type="uml:Region" xmi:id="regX" name="Region1">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="xInit" name="Initial1"/>
          <subvertex xmi:type="uml:State" xmi:id="xWait" name="wait"/>
          <subvertex xmi:type="uml:FinalState" xmi:id="xFin" name="FinalState1"/>
          <transition xmi:type="uml:Transition" xmi:id="xT1" name="T1" source="xInit" target="xWait"/>
` + body + `
        </region>
      </ownedBehavior>
    </packagedElement>
` + testerWith("Area001_Tester", steps...) + `  </packagedElement>
  </packagedElement>
` + fixtureTail
}

func refereeParameterSuite(t *testing.T, operations, body string, expected []string, steps ...testerStep) (*Report, *Suite) {
	t.Helper()
	s := readFixture(t, parameterSuite(operations, body, expected, steps...))
	noDiagnostics(t, s)
	if len(s.Tests) != 1 {
		t.Fatalf("tests = %d", len(s.Tests))
	}
	report, err := Referee(context.Background(), s, Provenance{Document: "fixture", Tests: 1}, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Tests) != 1 {
		t.Fatalf("rows = %d", len(report.Tests))
	}
	return report, s
}

func wantPass(t *testing.T, row TestReport) {
	t.Helper()
	if row.Bucket != oreport.BucketPass {
		t.Fatalf("bucket %s, reasons %q, reached %q; want pass", row.Bucket, row.Reasons, row.Reached)
	}
}

var (
	integerData = param{"data", "in", "sigData"}
	sendData    = func(v string) testerStep {
		return testerStep{send: "sigData", args: []string{"uml:LiteralInteger=" + v}}
	}
	sendContinue = testerStep{send: "sigContinue"}
)

// A signal's payload binds the effect's parameter directly and the entries of the
// target state and its initial substate through the stored occurrence.
func TestParametersSignalBindsEffectAndEntries(t *testing.T) {
	body := `
          <subvertex xmi:type="uml:State" xmi:id="xS1" name="S1">
            ` + paramTrace("entry", "xS1entry", "S1(entry)", integerData) + `
            <region xmi:type="uml:Region" xmi:id="xS1r" name="R">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xS1i" name="I"/>
              <subvertex xmi:type="uml:State" xmi:id="xS11" name="S1.1">
                ` + paramTrace("entry", "xS11entry", "S1.1(entry)", integerData) + `
              </subvertex>
              <transition xmi:type="uml:Transition" xmi:id="xT11" name="T1.1" source="xS1i" target="xS11"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT2" name="T2" source="xWait" target="xS1">
            <trigger xmi:type="uml:Trigger" xmi:id="xT2trig" event="evData"/>
            ` + paramTrace("effect", "xT2effect", "T2(effect)", integerData) + `
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xFin">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`
	report, _ := refereeParameterSuite(t, "", body,
		[]string{"T2(effect)[in=5]::S1(entry)[in=5]::S1.1(entry)[in=5]"},
		sendData("5"), sendContinue)
	wantPass(t, report.Tests[0])
}

// Each occurrence binds afresh: a state re-entered by a later occurrence of
// the same signal traces the later value.
func TestParametersRebindPerOccurrence(t *testing.T) {
	body := `
          <subvertex xmi:type="uml:State" xmi:id="xS1" name="S1">
            ` + paramTrace("entry", "xS1entry", "S1(entry)", integerData) + `
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT2" name="T2" source="xWait" target="xS1">
            <trigger xmi:type="uml:Trigger" xmi:id="xT2trig" event="evData"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS1">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evData"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="xT4" name="T4" source="xS1" target="xFin">
            <trigger xmi:type="uml:Trigger" xmi:id="xT4trig" event="evContinue"/>
          </transition>`
	report, _ := refereeParameterSuite(t, "", body,
		[]string{"S1(entry)[in=5]::S1(entry)[in=7]"},
		sendData("5"), sendData("7"), sendContinue)
	wantPass(t, report.Tests[0])
}

// A call event binds the operation's inputs to the entry's, whose outputs return
// through the call and are traced by the tester after the step (Event 019 E and D).
func TestParametersCallBindsInputsAndReturnsOutputs(t *testing.T) {
	body := `
          <subvertex xmi:type="uml:State" xmi:id="xS1" name="S1">
            ` + paramTrace("entry", "xS1entry", "S1(entry)", orParams...) + `
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT2" name="T2" source="xWait" target="xS1">
            <trigger xmi:type="uml:Trigger" xmi:id="xT2trig" event="evOr"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xFin">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`
	call := testerStep{call: "opOr", args: []string{"uml:LiteralBoolean=true", "uml:LiteralBoolean=false"}}
	report, s := refereeParameterSuite(t, orOperation, body,
		[]string{"S1(entry)[in=true][in=false][out=true][out=true]::[out=true]"},
		call.tracingResult(), sendContinue)
	wantPass(t, report.Tests[0])
	m, err := Emit(s, s.Tests[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"accept 'or'(left, right) do {\n            assign trigger_v_or_left := left;\n            assign trigger_v_or_right := right;\n        } then S1;",
		"in left = trigger_v_or_left;",
		"in right = trigger_v_or_right;",
		"out result : Boolean;",
		"out 'return' : Boolean;",
		"assign 'return' := (left or right);",
		"inout log : String;",
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("emitted text lacks %q:\n%s", want, m.Text)
		}
	}
}

// A do activity's in parameters bind like an entry's; the trace admits the
// activity finishing or not before the next event, as PSSM does.
func TestParametersDoActivityBindsInputs(t *testing.T) {
	body := `
          <subvertex xmi:type="uml:State" xmi:id="xS1" name="S1">
            ` + paramTrace("entry", "xS1entry", "S1(entry)", integerData) + `
            ` + paramTrace("doActivity", "xS1do", "S1(doActivity)", integerData) + `
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT2" name="T2" source="xWait" target="xS1">
            <trigger xmi:type="uml:Trigger" xmi:id="xT2trig" event="evData"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xFin">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`
	report, _ := refereeParameterSuite(t, "", body,
		[]string{"S1(entry)[in=5]::S1(doActivity)[in=5]", "S1(entry)[in=5]"},
		sendData("5"), sendContinue)
	wantPass(t, report.Tests[0])
	if got := report.Tests[0].Reached; len(got) != 2 {
		t.Errorf("reached %q, want both admitted traces", got)
	}
}

// Behaviors whose parameters bind nothing the notation reaches stay refused,
// each with its reason, and the classifier names the state.
func TestParametersRefusals(t *testing.T) {
	bound := `
          <subvertex xmi:type="uml:State" xmi:id="xS1" name="S1">
            ` + paramTrace("entry", "xS1entry", "S1(entry)", integerData) + `
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT2" name="T2" source="xWait" target="xS1">
            <trigger xmi:type="uml:Trigger" xmi:id="xT2trig" event="evData"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="xT9" name="T9" source="xS2" target="xFin">
            <trigger xmi:type="uml:Trigger" xmi:id="xT9trig" event="evContinue"/>
          </transition>`
	cases := []struct {
		name, body, where, reason string
	}{
		{"completion", bound + `
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            ` + paramTrace("entry", "xS2entry", "S2(entry)", integerData) + `
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2"/>`,
			"S2", "reached by the completion of state S1, which carries no event"},
		{"exit", bound + `
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            ` + paramTrace("exit", "xS2exit", "S2(exit)", integerData) + `
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evData"/>
          </transition>`,
			"S2", "the exit runs before the effect of transition T9"},
		{"two events", bound + `
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            ` + paramTrace("entry", "xS2entry", "S2(entry)", integerData) + `
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evData"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="xT4" name="T4" source="xWait" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT4trig" event="evOr"/>
          </transition>`,
			"S2", "bound from IntegerData by one path and or() by another"},
		{"several triggers", bound + `
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            ` + paramTrace("entry", "xS2entry", "S2(entry)", integerData) + `
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evData"/>
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig2" event="evContinue"/>
          </transition>`,
			"S2", "transition T3 accepts several events"},
		{"wrong signature", bound + `
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            ` + paramTrace("entry", "xS2entry", "S2(entry)", param{"flag", "in", "Boolean"}) + `
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evData"/>
          </transition>`,
			"S2", "parameter flag is a Boolean but IntegerData carries a IntegerData there"},
		{"do with outputs", bound + `
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            ` + paramTrace("doActivity", "xS2do", "S2(doActivity)", orParams...) + `
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evOr"/>
          </transition>`,
			"S2", "a do activity's outputs return to nobody"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := readFixture(t, parameterSuite(orOperation, tc.body, []string{"S1(entry)[in=5]"}, sendData("5")))
			noDiagnostics(t, s)
			test := s.Tests[0]
			var refused *Refusal
			bindings := BindBehaviors(test.Machine)
			for i := range bindings.Refused {
				if bindings.Refused[i].Where == tc.where {
					refused = &bindings.Refused[i]
				}
			}
			if refused == nil || !strings.Contains(refused.Reason, tc.reason) {
				t.Fatalf("refused = %+v, want %q at %s", refused, tc.reason, tc.where)
			}
			if c := Classify(test); c.Reason() != "behavior parameter "+tc.where {
				t.Errorf("reason %q, want only behavior parameter %s", c.Reason(), tc.where)
			}
		})
	}
}

// tracingResult turns a call step into `this.trace(formatParameterValue(false,
// this.testable.<op>(args)))`, the suite's way of tracing a returned value.
func (s testerStep) tracingResult() testerStep {
	s.traceResult = true
	return s
}
