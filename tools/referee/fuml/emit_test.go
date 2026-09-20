package fuml

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/semantics"
)

// flowModel exercises the translation rules found by hand: Pick decides on an
// input, joins two branches before a third value, and collects three values
// in order; Caller calls Pick twice.
const flowModel = `<?xml version="1.0" encoding="UTF-8"?>
<uml:Model xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="Flows">
  <packagedElement xmi:type="uml:Activity" xmi:id="pick" name="Pick">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="pickIn" name="x" direction="in">` + integerType + `</ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="pickLow" name="low" direction="out">` + integerType + `
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="lowLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="lowHi" value="1"/>
    </ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="pickHigh" name="high" direction="out">` + integerType + `
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="highLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="highHi" value="1"/>
    </ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="pickOut" name="picked" direction="out" isOrdered="true" isUnique="false">` + integerType + `
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="pickLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="pickHi" value="*"/>
    </ownedParameter>
    <node xmi:type="uml:InitialNode" xmi:id="init" name="Initial"/>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="pickInNode" name="Parameter(x)" parameter="pickIn"/>
    <node xmi:type="uml:DecisionNode" xmi:id="decide" name="Decision"/>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="lowNode" name="Parameter(low)" parameter="pickLow"/>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="highNode" name="Parameter(high)" parameter="pickHigh"/>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v10" name="Value(10)">
      <result xmi:type="uml:OutputPin" xmi:id="v10r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v10v" value="10"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v20" name="Value(20)">
      <result xmi:type="uml:OutputPin" xmi:id="v20r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v20v" value="20"/>
    </node>
    <node xmi:type="uml:JoinNode" xmi:id="join" name="Join"/>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v30" name="Value(30)">
      <result xmi:type="uml:OutputPin" xmi:id="v30r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v30v" value="30"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="pickOutNode" name="Parameter(picked)" parameter="pickOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="f1" source="pickInNode" target="decide"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="f2" source="decide" target="lowNode">
      <guard xmi:type="uml:LiteralInteger" xmi:id="g0"/>
    </edge>
    <edge xmi:type="uml:ObjectFlow" xmi:id="f3" source="decide" target="highNode">
      <guard xmi:type="uml:LiteralInteger" xmi:id="g1" value="1"/>
    </edge>
    <edge xmi:type="uml:ControlFlow" xmi:id="c1" source="init" target="v10"/>
    <edge xmi:type="uml:ControlFlow" xmi:id="c2" source="v10" target="v20"/>
    <edge xmi:type="uml:ControlFlow" xmi:id="c3" source="v10" target="join"/>
    <edge xmi:type="uml:ControlFlow" xmi:id="c4" source="v20" target="join"/>
    <edge xmi:type="uml:ControlFlow" xmi:id="c5" source="join" target="v30"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="f4" source="v10r" target="pickOutNode"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="f5" source="v20r" target="pickOutNode"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="f6" source="v30r" target="pickOutNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="caller" name="Caller">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="callerOut" name="all" direction="out" isUnique="false">` + integerType + `
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="callerLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="callerHi" value="*"/>
    </ownedParameter>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="cv0" name="Value(0)">
      <result xmi:type="uml:OutputPin" xmi:id="cv0r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="cv0v" value="0"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="cv1" name="Value(1)">
      <result xmi:type="uml:OutputPin" xmi:id="cv1r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="cv1v" value="1"/>
    </node>
    <node xmi:type="uml:CallBehaviorAction" xmi:id="callA" name="Pick A" behavior="pick">
      <argument xmi:type="uml:InputPin" xmi:id="callAx" name="x">` + integerType + `</argument>
      <result xmi:type="uml:OutputPin" xmi:id="callAl" name="low">` + integerType + `</result>
      <result xmi:type="uml:OutputPin" xmi:id="callAh" name="high">` + integerType + `</result>
      <result xmi:type="uml:OutputPin" xmi:id="callAp" name="picked">` + integerType + `</result>
    </node>
    <node xmi:type="uml:CallBehaviorAction" xmi:id="callB" name="Pick B" behavior="pick">
      <argument xmi:type="uml:InputPin" xmi:id="callBx" name="x">` + integerType + `</argument>
      <result xmi:type="uml:OutputPin" xmi:id="callBl" name="low">` + integerType + `</result>
      <result xmi:type="uml:OutputPin" xmi:id="callBh" name="high">` + integerType + `</result>
      <result xmi:type="uml:OutputPin" xmi:id="callBp" name="picked">` + integerType + `</result>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="callerOutNode" name="Parameter(all)" parameter="callerOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="k1" source="cv0r" target="callAx"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="k2" source="cv1r" target="callBx"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="k3" source="callAl" target="callerOutNode"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="k4" source="callBh" target="callerOutNode"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="k5" source="callAp" target="callerOutNode"/>
  </packagedElement>
</uml:Model>
`

func emitted(t *testing.T, s *Suite, name string) *Emitted {
	t.Helper()
	em, err := Emit(fixtureActivity(t, s, name))
	if err != nil {
		t.Fatalf("Emit(%s): %v", name, err)
	}
	if problems := Validate(em); len(problems) > 0 {
		t.Fatalf("Emit(%s) does not validate: %v\n%s", name, problems, em.Text)
	}
	return em
}

func wantLines(t *testing.T, em *Emitted, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(em.Text, want) {
			t.Errorf("%s lacks %q:\n%s", em.Name, want, em.Text)
		}
	}
}

// Parameters keep their direction, type and multiplicity; a multi-valued
// output is `[0..*] nonunique`, ordered when the parameter is, and starts
// empty so that an unfed one is absent rather than unbound.
func TestEmitParameters(t *testing.T) {
	s := fixtureSuite(t, flowModel)
	wantLines(t, emitted(t, s, "Pick"), "in x : Integer;", "out low : Integer[0..1] = ();", "out picked : Integer[0..*] ordered nonunique = ();")
	wantLines(t, emitted(t, s, "Caller"), "out 'all' : Integer[0..*] nonunique = ();")
	wantLines(t, emitted(t, fixtureSuite(t, fixtureModel), "Sum"), "out result : Integer;")
}

// An object flow whose target has no control predecessor gets an enabling
// succession beside it; one whose target has is left alone.
func TestEmitEnablesObjectFlowTargets(t *testing.T) {
	s := fixtureSuite(t, fixtureModel)
	em := emitted(t, s, "Sum")
	wantLines(t, em,
		"flow 'Value(3)'.result to 'Call(Plus)'.x;",
		"succession first 'Value(3)' then 'Call(Plus)';",
		"succession first 'Value(2)' then 'Call(Plus)';",
		"succession first 'Call(Plus)' then 'Parameter(result)';")
	if n := strings.Count(em.Text, "then 'Call(Plus)'"); n != 2 {
		t.Errorf("Call(Plus) enabled %d times, want once per feeding flow:\n%s", n, em.Text)
	}
	if n := strings.Count(em.Text, "then 'Parameter(result)'"); n != 1 {
		t.Errorf("Parameter(result) enabled %d times, want once:\n%s", n, em.Text)
	}
}

// Nodes with no predecessor start together from a fork on `start`; a node with
// several successors gets a fork; a decision guards its successions by the
// value decided on; a join's one outgoing succession is what follows it.
func TestEmitControlNodes(t *testing.T) {
	s := fixtureSuite(t, flowModel)
	em := emitted(t, s, "Pick")
	wantLines(t, em,
		"decide Decision;",
		"join Join;",
		"succession first Decision if 'Parameter(x)'.v == 0 then 'Parameter(low)';",
		"succession first Decision if 'Parameter(x)'.v == 1 then 'Parameter(high)';",
		"succession first 'Value(10) fork' then Join;",
		"succession first 'Value(20) fork' then Join;",
		"succession first Join then 'Value(30)';",
		"succession first start then")
	if n := strings.Count(em.Text, "succession first Join then"); n != 1 {
		t.Errorf("Join has %d outgoing successions, want 1:\n%s", n, em.Text)
	}
	if n := strings.Count(em.Text, "succession first start then"); n != 1 {
		t.Errorf("%d successions from start, want one (through a fork):\n%s", n, em.Text)
	}
}

// A called activity becomes its own action definition in the same package,
// referenced by qualified name; each call binds its pins to the callee's
// parameters by name and is enabled by the flows feeding it.
func TestEmitCalls(t *testing.T) {
	s := fixtureSuite(t, flowModel)
	em := emitted(t, s, "Caller")
	wantLines(t, em,
		"action def Pick {",
		"action def Caller {",
		"action 'Pick A' : fuml::Pick;",
		"action 'Pick B' : fuml::Pick;",
		"flow 'Value(0)'.result to 'Pick A'.x;",
		"flow 'Pick A'.picked to 'Parameter(all)'.v;",
		"succession first 'Value(0)' then 'Pick A';")
	if em.Qualified != "fuml::Caller" || em.Name != "Caller.sysml" {
		t.Errorf("Emitted = %q %q", em.Qualified, em.Name)
	}
	if got := strings.Join(em.Produces["Pick A"], ","); got != "Pick A.low,Pick A.high,Pick A.picked" {
		t.Errorf("Produces[Pick A] = %q", got)
	}
}

// A construct the pilot emitter does not spell is a TranslateError naming the
// activity, the place and the reason; the classifier still calls the activity
// expressible, and the referee files the error.
func TestEmitRefusesUntranslatedConstructs(t *testing.T) {
	s := fixtureSuite(t, fixtureModel)
	a := fixtureActivity(t, s, "Selfer")
	if c := Classify(a, nil); c.Class != Expressible {
		t.Fatalf("Selfer classified %s", c.Class)
	}
	_, err := Emit(a)
	var te *TranslateError
	if !errors.As(err, &te) || !IsTranslateError(err) {
		t.Fatalf("Emit(Selfer) = %v, want a TranslateError", err)
	}
	if te.Activity != "Selfer" || te.Where != "ReadSelf" || !strings.Contains(te.Reason, "no class owns") {
		t.Errorf("TranslateError = %+v", te)
	}
	if IsTranslateError(errors.New("other")) {
		t.Error("a plain error is a TranslateError")
	}
}

// Validate reports parser diagnostics and lowering errors of a translated
// model, and nothing for a well-formed one.
func TestValidate(t *testing.T) {
	s := fixtureSuite(t, fixtureModel)
	em := emitted(t, s, "Sum")
	broken := *em
	broken.Text = strings.Replace(em.Text, "then 'Call(Plus)';", "then 'Call(Minus)';", 1)
	problems := Validate(&broken)
	if len(problems) == 0 {
		t.Fatal("a succession to an undeclared node validates")
	}
	misnamed := *em
	misnamed.Qualified = "fuml::NoSuch"
	if problems := Validate(&misnamed); len(problems) != 1 || !strings.Contains(problems[0], "0 symbols named") {
		t.Errorf("misnamed: %v", problems)
	}
}

// The comparison is by value: a scalar must match, an unordered multi-valued
// output matches as a multiset, an ordered one in order, and an output the
// implementation left empty must be empty here. Every run within the budget
// must agree; the budget need not exhaust the schedules.
func TestExecuteCompares(t *testing.T) {
	s := fixtureSuite(t, flowModel)
	pick := fixtureActivity(t, s, "Pick")
	em := emitted(t, s, "Pick")
	run := func(x ExpectedActivity) *Execution {
		ex, err := Execute(context.Background(), em, &x, DefaultBudget, 2)
		if err != nil {
			t.Fatal(err)
		}
		return ex
	}
	// x defaults to 0, so the decision takes low; the control flow fixes the order.
	agreed := []ExpectedOutput{integers("low", 0), integers("picked", 10, 20, 30)}
	fired := []string{"Value(10)", "Value(20)", "Value(30)"}
	ex := run(executed(pick, agreed, fired...))
	if !ex.Passed() || ex.Fired != "" || ex.Runs == 0 || ex.Status == "" {
		t.Errorf("ordered agreement: %+v", ex)
	}
	if strings.Join(ex.Reached, "|") != "low = 0; high = -; picked = 10, 20, 30" {
		t.Errorf("reached %q", ex.Reached)
	}
	ex = run(executed(pick, []ExpectedOutput{integers("low", 0), integers("picked", 20, 10, 30)}, fired...))
	if ex.Passed() || strings.Join(ex.Reasons(), ";") != "outputs differ: low = 0; high = -; picked = 10, 20, 30" {
		t.Errorf("ordered disagreement: %+v", ex)
	}
	ex = run(executed(pick, []ExpectedOutput{integers("low", 0), integers("high", 0), integers("picked", 10, 20, 30)}, fired...))
	if ex.Passed() || strings.Join(ex.Expected, ";") != "low = 0;high = 0;picked = 10, 20, 30" {
		t.Errorf("present expected, absent here: %+v", ex)
	}
	ex = run(executed(pick, agreed, "Value(10)", "Value(30)"))
	if !ex.Passed() || !strings.Contains(ex.Fired, "produced a value here only: Value(20)") {
		t.Errorf("advisory firing: %+v", ex)
	}

	caller := fixtureActivity(t, s, "Caller")
	em = emitted(t, s, "Caller")
	x := executed(caller, []ExpectedOutput{integers("all", 30, 1, 20, 10, 0)})
	ex, err := Execute(context.Background(), em, &x, DefaultBudget, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.Passed() || strings.Join(ex.Reached, "|") != "all = 0, 1, 10, 20, 30" {
		t.Errorf("multiset agreement: %+v", ex)
	}
}

// sharedNameModel has Outer collect into `output` what Inner leaves in its own `output`.
const sharedNameModel = `<?xml version="1.0" encoding="UTF-8"?>
<uml:Model xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="Shared">
  <packagedElement xmi:type="uml:Activity" xmi:id="inner" name="Inner">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="innerIn" name="input" direction="in">` + integerType + `</ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="innerOut" name="output" direction="out">` + integerType + `</ownedParameter>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="innerInNode" name="Parameter(input)" parameter="innerIn"/>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="innerOutNode" name="Parameter(output)" parameter="innerOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="i1" source="innerInNode" target="innerOutNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="outer" name="Outer">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="outerOut" name="output" direction="out" isUnique="false">` + integerType + `
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="outerLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="outerHi" value="*"/>
    </ownedParameter>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v7" name="Value(7)">
      <result xmi:type="uml:OutputPin" xmi:id="v7r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v7v" value="7"/>
    </node>
    <node xmi:type="uml:CallBehaviorAction" xmi:id="call" name="Call(Inner)" behavior="inner">
      <argument xmi:type="uml:InputPin" xmi:id="callIn" name="input">` + integerType + `</argument>
      <result xmi:type="uml:OutputPin" xmi:id="callOut" name="output">` + integerType + `</result>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="outerOutNode" name="Parameter(output)" parameter="outerOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="o1" source="v7r" target="callIn"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="o2" source="callOut" target="outerOutNode"/>
  </packagedElement>
</uml:Model>
`

// A called activity's parameter named like the caller's is spelled apart, since the
// runtime returns a nested action's outputs to same-named enclosing features; the
// translated activity's own parameters keep the names the record compares by.
func TestEmitSpellsSharedParameterNamesApart(t *testing.T) {
	s := fixtureSuite(t, sharedNameModel)
	em := emitted(t, s, "Outer")
	wantLines(t, em,
		"action def Inner {", "in input : Integer;", "out Inner_output : Integer;",
		"assign Inner_output := v;",
		"action def Outer {", "out output : Integer[0..*] nonunique = ();",
		"flow 'Value(7)'.result to 'Call(Inner)'.input;",
		"flow 'Call(Inner)'.Inner_output to 'Parameter(output)'.v;")
	if strings.Contains(em.Text, "out output : Integer;") {
		t.Errorf("Inner keeps the shared name:\n%s", em.Text)
	}
	outer := fixtureActivity(t, s, "Outer")
	x := executed(outer, []ExpectedOutput{integers("output", 7)})
	ex, err := Execute(context.Background(), em, &x, DefaultBudget, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.Passed() || strings.Join(ex.Reached, "|") != "output = 7" {
		t.Errorf("shared name: %+v", ex)
	}
	wantLines(t, emitted(t, s, "Inner"), "out output : Integer;", "assign output := v;")
}

// A budget that samples the schedules passes when the sample agrees, the status
// naming the budget hit; a run ending in a typed error fails.
func TestExecuteBudgets(t *testing.T) {
	s := fixtureSuite(t, flowModel)
	caller := fixtureActivity(t, s, "Caller")
	em := emitted(t, s, "Caller")
	x := executed(caller, []ExpectedOutput{integers("all", 0, 1, 10, 20, 30)})
	ex, err := Execute(context.Background(), em, &x, runtime.ExploreBudget{Runs: 1, Depth: 64}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.Passed() || ex.Complete || ex.Runs != 1 || !strings.Contains(ex.Status, "runs budget 1 hit after 1 runs") {
		t.Errorf("sampled schedules: %+v", ex)
	}

	t.Setenv(runtime.MaxStepsEnvVar, "1")
	ex, err = Execute(context.Background(), em, &x, DefaultBudget, 1)
	if err != nil {
		t.Fatal(err)
	}
	if ex.Passed() || len(ex.Errors) == 0 || !strings.HasPrefix(ex.Reasons()[0], "run error: ") {
		t.Errorf("step budget: %+v", ex)
	}
}

// objectModel exercises the object rules: Item specializes Base and declares
// every multiplicity shape; Assemble creates one and edits each feature its way;
// Reader reads and clears the features of an Item it is given; Pairing collects
// two Items into an unordered output; Holder's owned behavior Reflect reads self.
const objectModel = `<?xml version="1.0" encoding="UTF-8"?>
<uml:Model xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="Objects">
  <packagedElement xmi:type="uml:Class" xmi:id="base" name="Base">
    <ownedAttribute xmi:type="uml:Property" xmi:id="n" name="n">` + integerType + `</ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Class" xmi:id="item" name="Item">
    <generalization xmi:type="uml:Generalization" xmi:id="gen" general="base"/>
    <ownedAttribute xmi:type="uml:Property" xmi:id="xs" name="xs" isOrdered="true" isUnique="false">` + integerType + `
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="xsLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="xsHi" value="*"/>
    </ownedAttribute>
    <ownedAttribute xmi:type="uml:Property" xmi:id="set" name="set">` + integerType + `
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="setLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="setHi" value="3"/>
    </ownedAttribute>
    <ownedAttribute xmi:type="uml:Property" xmi:id="opt" name="opt">
      <type xmi:type="uml:PrimitiveType" href="pathmap://UML_LIBRARIES/UMLPrimitiveTypes.library.uml#String"/>
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="optLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="optHi" value="1"/>
    </ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="assemble" name="Assemble">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="assembleOut" name="made" direction="out" type="item"/>
    <node xmi:type="uml:CreateObjectAction" xmi:id="create" name="Create(Item)" classifier="item">
      <result xmi:type="uml:OutputPin" xmi:id="createR" name="result" type="item"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v7" name="Value(7)">
      <result xmi:type="uml:OutputPin" xmi:id="v7r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v7v" value="7"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v1" name="Value(1)">
      <result xmi:type="uml:OutputPin" xmi:id="v1r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v1v" value="1"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v2" name="Value(2)">
      <result xmi:type="uml:OutputPin" xmi:id="v2r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v2v" value="2"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v9" name="Value(9)">
      <result xmi:type="uml:OutputPin" xmi:id="v9r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v9v" value="9"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v5" name="Value(5)">
      <result xmi:type="uml:OutputPin" xmi:id="v5r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v5v" value="5"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="writeN" name="Write(n)" structuralFeature="n" isReplaceAll="true">
      <object xmi:type="uml:InputPin" xmi:id="writeNo" name="object" type="item"/>
      <value xmi:type="uml:InputPin" xmi:id="writeNv" name="value">` + integerType + `</value>
      <result xmi:type="uml:OutputPin" xmi:id="writeNr" name="result" type="item"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="add1" name="Add(xs)-1" structuralFeature="xs">
      <object xmi:type="uml:InputPin" xmi:id="add1o" name="object" type="item"/>
      <value xmi:type="uml:InputPin" xmi:id="add1v" name="value">` + integerType + `</value>
      <result xmi:type="uml:OutputPin" xmi:id="add1r" name="result" type="item"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="add2" name="Add(xs)-2" structuralFeature="xs">
      <object xmi:type="uml:InputPin" xmi:id="add2o" name="object" type="item"/>
      <value xmi:type="uml:InputPin" xmi:id="add2v" name="value">` + integerType + `</value>
      <result xmi:type="uml:OutputPin" xmi:id="add2r" name="result" type="item"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="insert" name="Insert(xs)" structuralFeature="xs">
      <object xmi:type="uml:InputPin" xmi:id="inserto" name="object" type="item"/>
      <value xmi:type="uml:InputPin" xmi:id="insertv" name="value">` + integerType + `</value>
      <insertAt xmi:type="uml:InputPin" xmi:id="inserta" name="insertAt">` + integerType + `</insertAt>
      <result xmi:type="uml:OutputPin" xmi:id="insertr" name="result" type="item"/>
    </node>
    <node xmi:type="uml:RemoveStructuralFeatureValueAction" xmi:id="removeAt" name="RemoveAt(xs)" structuralFeature="xs">
      <object xmi:type="uml:InputPin" xmi:id="removeAto" name="object" type="item"/>
      <value xmi:type="uml:InputPin" xmi:id="removeAtv" name="value">` + integerType + `</value>
      <removeAt xmi:type="uml:InputPin" xmi:id="removeAta" name="removeAt">` + integerType + `</removeAt>
      <result xmi:type="uml:OutputPin" xmi:id="removeAtr" name="result" type="item"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="addSet1" name="Add(set)-1" structuralFeature="set">
      <object xmi:type="uml:InputPin" xmi:id="addSet1o" name="object" type="item"/>
      <value xmi:type="uml:InputPin" xmi:id="addSet1v" name="value">` + integerType + `</value>
      <result xmi:type="uml:OutputPin" xmi:id="addSet1r" name="result" type="item"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="addSet2" name="Add(set)-2" structuralFeature="set">
      <object xmi:type="uml:InputPin" xmi:id="addSet2o" name="object" type="item"/>
      <value xmi:type="uml:InputPin" xmi:id="addSet2v" name="value">` + integerType + `</value>
      <result xmi:type="uml:OutputPin" xmi:id="addSet2r" name="result" type="item"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="assembleOutNode" name="Parameter(made)" parameter="assembleOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a1" source="createR" target="writeNo"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a2" source="v7r" target="writeNv"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a3" source="writeNr" target="add1o"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a4" source="v1r" target="add1v"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a5" source="add1r" target="add2o"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a6" source="v2r" target="add2v"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a7" source="add2r" target="inserto"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a8" source="v9r" target="insertv"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a9" source="v2r" target="inserta"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a10" source="insertr" target="removeAto"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a11" source="v1r" target="removeAtv"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a12" source="v1r" target="removeAta"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a13" source="removeAtr" target="addSet1o"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a14" source="v5r" target="addSet1v"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a15" source="addSet1r" target="addSet2o"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a16" source="v5r" target="addSet2v"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="a17" source="addSet2r" target="assembleOutNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="reader" name="Reader">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="readerIn" name="given" direction="in" type="item"/>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="readerN" name="n" direction="out">` + integerType + `</ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="readerXs" name="xs" direction="out" isOrdered="true" isUnique="false">` + integerType + `
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="readerXsLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="readerXsHi" value="*"/>
    </ownedParameter>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="readerInNode" name="Parameter(given)" parameter="readerIn"/>
    <node xmi:type="uml:ForkNode" xmi:id="readerFork" name="Fork"/>
    <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="readN" name="Read(n)" structuralFeature="n">
      <object xmi:type="uml:InputPin" xmi:id="readNo" name="object" type="item"/>
      <result xmi:type="uml:OutputPin" xmi:id="readNr" name="result">` + integerType + `</result>
    </node>
    <node xmi:type="uml:ClearStructuralFeatureAction" xmi:id="clearXs" name="Clear(xs)" structuralFeature="xs">
      <object xmi:type="uml:InputPin" xmi:id="clearXso" name="object" type="item"/>
      <result xmi:type="uml:OutputPin" xmi:id="clearXsr" name="result" type="item"/>
    </node>
    <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="readXs" name="Read(xs)" structuralFeature="xs">
      <object xmi:type="uml:InputPin" xmi:id="readXso" name="object" type="item"/>
      <result xmi:type="uml:OutputPin" xmi:id="readXsr" name="result" isOrdered="true" isUnique="false">` + integerType + `
        <lowerValue xmi:type="uml:LiteralInteger" xmi:id="readXsrLo"/>
        <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="readXsrHi" value="*"/>
      </result>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="readerNNode" name="Parameter(n)" parameter="readerN"/>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="readerXsNode" name="Parameter(xs)" parameter="readerXs"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="r1" source="readerInNode" target="readerFork"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="r2" source="readerFork" target="readNo"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="r3" source="readerFork" target="clearXso"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="r4" source="readNr" target="readerNNode"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="r5" source="clearXsr" target="readXso"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="r6" source="readXsr" target="readerXsNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="pairing" name="Pairing">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="pairingOut" name="both" direction="out" type="item">
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="bothLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="bothHi" value="*"/>
    </ownedParameter>
    <node xmi:type="uml:CreateObjectAction" xmi:id="createA" name="Create(A)" classifier="item">
      <result xmi:type="uml:OutputPin" xmi:id="createAr" name="result" type="item"/>
    </node>
    <node xmi:type="uml:CreateObjectAction" xmi:id="createB" name="Create(B)" classifier="item">
      <result xmi:type="uml:OutputPin" xmi:id="createBr" name="result" type="item"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="pv1" name="Value(1)">
      <result xmi:type="uml:OutputPin" xmi:id="pv1r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="pv1v" value="1"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="pv2" name="Value(2)">
      <result xmi:type="uml:OutputPin" xmi:id="pv2r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="pv2v" value="2"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="writeA" name="Write(A.n)" structuralFeature="n" isReplaceAll="true">
      <object xmi:type="uml:InputPin" xmi:id="writeAo" name="object" type="item"/>
      <value xmi:type="uml:InputPin" xmi:id="writeAv" name="value">` + integerType + `</value>
      <result xmi:type="uml:OutputPin" xmi:id="writeAr" name="result" type="item"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="writeB" name="Write(B.n)" structuralFeature="n" isReplaceAll="true">
      <object xmi:type="uml:InputPin" xmi:id="writeBo" name="object" type="item"/>
      <value xmi:type="uml:InputPin" xmi:id="writeBv" name="value">` + integerType + `</value>
      <result xmi:type="uml:OutputPin" xmi:id="writeBr" name="result" type="item"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="pairingOutNode" name="Parameter(both)" parameter="pairingOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="p1" source="createAr" target="writeAo"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="p2" source="pv1r" target="writeAv"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="p3" source="createBr" target="writeBo"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="p4" source="pv2r" target="writeBv"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="p5" source="writeAr" target="pairingOutNode"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="p6" source="writeBr" target="pairingOutNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="twins" name="Twins">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="twinsBoth" name="both" direction="out" type="item">
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="twinsBothLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="twinsBothHi" value="*"/>
    </ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="twinsPick" name="pick" direction="out" type="item"/>
    <node xmi:type="uml:CreateObjectAction" xmi:id="twinA" name="Create(A)" classifier="item">
      <result xmi:type="uml:OutputPin" xmi:id="twinAr" name="result" type="item"/>
    </node>
    <node xmi:type="uml:CreateObjectAction" xmi:id="twinB" name="Create(B)" classifier="item">
      <result xmi:type="uml:OutputPin" xmi:id="twinBr" name="result" type="item"/>
    </node>
    <node xmi:type="uml:ForkNode" xmi:id="twinsFork" name="Fork"/>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="twinsBothNode" name="Parameter(both)" parameter="twinsBoth"/>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="twinsPickNode" name="Parameter(pick)" parameter="twinsPick"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="t1" source="twinAr" target="twinsBothNode"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="t2" source="twinBr" target="twinsFork"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="t3" source="twinsFork" target="twinsBothNode"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="t4" source="twinsFork" target="twinsPickNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Class" xmi:id="holder" name="Holder">
    <ownedBehavior xmi:type="uml:Activity" xmi:id="reflect" name="Reflect">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="reflectOut" name="me" direction="out" type="holder"/>
      <node xmi:type="uml:ReadSelfAction" xmi:id="readSelf" name="ReadSelf">
        <result xmi:type="uml:OutputPin" xmi:id="readSelfr" name="result" type="holder"/>
      </node>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="reflectOutNode" name="Parameter(me)" parameter="reflectOut"/>
      <edge xmi:type="uml:ObjectFlow" xmi:id="h1" source="readSelfr" target="reflectOutNode"/>
    </ownedBehavior>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="instantiator" name="Instantiator">
    <node xmi:type="uml:CreateObjectAction" xmi:id="createReader" name="Create(Reader)" classifier="reader">
      <result xmi:type="uml:OutputPin" xmi:id="createReaderr" name="result" type="reader"/>
    </node>
  </packagedElement>
</uml:Model>
`

// object is the implementation's record of an object of one type with the
// features named holding the values given.
func object(id, typeName string, features ...ExpectedFeature) ExpectedValue {
	return ExpectedValue{Kind: "Object", ID: id, Types: []string{typeName}, Features: features}
}

func feature(name string, values ...int) ExpectedFeature {
	return ExpectedFeature{Feature: name, Values: integers(name, values...).Values}
}

// A class is a part def with an attribute per property at its exact
// multiplicity, the default [1..1] unordered unique left unwritten, specializing
// its generals with `:>`; a class the activity's closure touches through a
// parameter, a pin, a created classifier or a feature's owner is declared once.
func TestEmitClasses(t *testing.T) {
	s := fixtureSuite(t, objectModel)
	em := emitted(t, s, "Assemble")
	wantLines(t, em,
		"\tpart def Base {\n\t\tattribute n : Integer;\n\t}\n",
		"\tpart def Item :> Base {\n\t\tattribute xs : Integer [0..*] ordered nonunique;\n\t\tattribute set : Integer [0..3];\n\t\tattribute opt : String [0..1];\n\t}\n",
		"out made : Item;")
	if n := strings.Count(em.Text, "part def "); n != 2 {
		t.Errorf("%d part defs, want Base and Item once each:\n%s", n, em.Text)
	}
	if strings.Contains(em.Text, "Holder") {
		t.Errorf("Holder declared though Assemble never touches it:\n%s", em.Text)
	}
}

// A create object action is `new T()` onto its result pin and starts no
// behavior; a structural feature action takes the object at a pin, hands it on
// through its result pin and assigns the feature as the reference implementation
// does: a replacing add takes the value, an add inserts first (dropping a unique
// feature's old copy), an indexed add inserts at insertAt, `*` appending; an
// indexed remove drops the value at removeAt when there is one.
func TestEmitObjectCreationAndFeatureWrites(t *testing.T) {
	s := fixtureSuite(t, objectModel)
	em := emitted(t, s, "Assemble")
	wantLines(t, em,
		"action 'Create(Item)' { out result : Item = new Item(); }",
		"action 'Write(n)' { in object : Item; in value : Integer; out result : Item = object; assign object.n := value; }",
		"action 'Add(xs)-1' { in object : Item; in value : Integer; out result : Item = object; assign object.xs := (value, object.xs); }",
		"action 'Insert(xs)' { in object : Item; in value : Integer; in insertAt : Integer; out result : Item = object; assign object.xs := if insertAt < 0 ? including(object.xs, value) else includingAt(object.xs, value, insertAt); }",
		"action 'RemoveAt(xs)' { in object : Item; in value : Integer; in removeAt : Integer; out result : Item = object; assign object.xs := if removeAt >= 1 and removeAt <= size(object.xs) ? excludingAt(object.xs, removeAt) else object.xs; }",
		"action 'Add(set)-1' { in object : Item; in value : Integer; out result : Item = object; assign object.set := (value, excluding(object.set, value)); }",
		"flow 'Create(Item)'.result to 'Write(n)'.object;",
		"flow 'Write(n)'.result to 'Add(xs)-1'.object;",
		"flow 'Value(2)'.result to 'Insert(xs)'.insertAt;",
		"flow 'Add(set)-2'.result to 'Parameter(made)'.v;")
	if strings.Contains(em.Text, "perform") || strings.Contains(em.Text, "start") && strings.Contains(em.Text, "Item.") {
		t.Errorf("creation starts a behavior:\n%s", em.Text)
	}
}

// A read structural feature action reads the object's feature onto its result
// pin at the feature's multiplicity; a clear empties it; an object flowing from a
// fork reaches every action fed by it.
func TestEmitFeatureReads(t *testing.T) {
	s := fixtureSuite(t, objectModel)
	em := emitted(t, s, "Reader")
	wantLines(t, em,
		"in given : Item;",
		"action 'Parameter(given)' { out v : Item = given; }",
		"action 'Read(n)' { in object : Item; out result : Integer[0..1] = object.n; }",
		"action 'Clear(xs)' { in object : Item; out result : Item = object; assign object.xs := (); }",
		"action 'Read(xs)' { in object : Item; out result : Integer[0..*] ordered nonunique = object.xs; }",
		"flow 'Parameter(given)'.v to 'Read(n)'.object;",
		"flow 'Parameter(given)'.v to 'Clear(xs)'.object;",
		"succession first Fork then 'Read(n)';",
		"flow 'Clear(xs)'.result to 'Read(xs)'.object;",
		"flow 'Read(n)'.result to 'Parameter(n)'.v;")
}

// A read self action outside any class is a TranslateError; a class's owned
// behavior, where `this` would be the object, is one too until owned behaviors
// are translated.
func TestEmitReadSelf(t *testing.T) {
	s := fixtureSuite(t, objectModel)
	var reflect *Activity
	for _, c := range s.Tests.Classes {
		for _, b := range c.Behaviors {
			if b.Name == "Reflect" {
				reflect = b
			}
		}
	}
	if reflect == nil || reflect.Owner == nil || reflect.Owner.Name != "Holder" {
		t.Fatalf("Holder owns no Reflect: %+v", reflect)
	}
	_, err := Emit(reflect)
	var te *TranslateError
	if !errors.As(err, &te) || te.Activity != "Reflect" || te.Where != "activity" || !strings.Contains(te.Reason, "owned behavior") {
		t.Errorf("Emit(Reflect) = %v, want a TranslateError on the owned behavior", err)
	}
}

// An activity instantiated as an object (a behavior is a class in UML) is
// refused naming the activity, apart from a classifier the model lacks.
func TestEmitRefusesAnActivityAsObject(t *testing.T) {
	s := fixtureSuite(t, objectModel)
	_, err := Emit(fixtureActivity(t, s, "Instantiator"))
	var te *TranslateError
	if !errors.As(err, &te) || te.Where != "Create(Reader)" || !strings.Contains(te.Reason, "object of the activity Reader") {
		t.Errorf("Emit(Instantiator) = %v, want a TranslateError on the activity created", err)
	}
}

// Assemble's object comes out with the features the reference implementation
// would leave: n written, xs inserted first then at 2 and removed at 1, the
// unique set holding one 5; Reader gets a default Item, whose n is 0 and whose
// cleared xs is empty. Objects render by type and feature, numbered by mention.
func TestExecuteObjects(t *testing.T) {
	s := fixtureSuite(t, objectModel)
	assemble := fixtureActivity(t, s, "Assemble")
	want := object("o1", "Item", feature("n", 7), feature("xs", 9, 1), feature("set", 5))
	x := executed(assemble, []ExpectedOutput{{Parameter: "made", Values: []ExpectedValue{want}}})
	ex, err := Execute(context.Background(), emitted(t, s, "Assemble"), &x, DefaultBudget, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.Passed() || strings.Join(ex.Reached, "|") != "made = Item#1{n = 7; opt = -; set = 5; xs = 9, 1}" {
		t.Errorf("Assemble: %+v", ex)
	}
	want = object("o1", "Item", feature("n", 7), feature("xs", 1, 9), feature("set", 5))
	x = executed(assemble, []ExpectedOutput{{Parameter: "made", Values: []ExpectedValue{want}}})
	if ex, err = Execute(context.Background(), emitted(t, s, "Assemble"), &x, DefaultBudget, 1); err != nil {
		t.Fatal(err)
	}
	if ex.Passed() || strings.Join(ex.Reasons(), ";") != "outputs differ: made = Item#1{n = 7; opt = -; set = 5; xs = 9, 1}" {
		t.Errorf("Assemble, order differing: %+v", ex)
	}

	reader := fixtureActivity(t, s, "Reader")
	x = executed(reader, []ExpectedOutput{integers("n", 0), integers("xs")})
	if ex, err = Execute(context.Background(), emitted(t, s, "Reader"), &x, DefaultBudget, 1); err != nil {
		t.Fatal(err)
	}
	if !ex.Passed() || strings.Join(ex.Reached, "|") != "n = 0; xs = -" {
		t.Errorf("Reader: %+v", ex)
	}
}

// An unordered output holding objects compares as a multiset: the objects are
// numbered in an order their spelling fixes, not the order they arrived in, so
// the record and every run agree whichever Item reached the parameter first.
func TestExecuteUnorderedObjects(t *testing.T) {
	s := fixtureSuite(t, objectModel)
	pairing := fixtureActivity(t, s, "Pairing")
	first := object("a", "Item", feature("n", 1))
	second := object("b", "Item", feature("n", 2))
	forward := executed(pairing, []ExpectedOutput{{Parameter: "both", Values: []ExpectedValue{first, second}}})
	backward := executed(pairing, []ExpectedOutput{{Parameter: "both", Values: []ExpectedValue{second, first}}})
	want := "both = Item#1{n = 1; opt = -; set = -; xs = -}, Item#2{n = 2; opt = -; set = -; xs = -}"
	if got := renderExpected(pairing, &backward); got != want {
		t.Errorf("renderExpected(backward) = %q, want %q", got, want)
	}
	for name, x := range map[string]ExpectedActivity{"forward": forward, "backward": backward} {
		ex, err := Execute(context.Background(), emitted(t, s, "Pairing"), &x, DefaultBudget, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !ex.Passed() || strings.Join(ex.Reached, "|") != want {
			t.Errorf("Pairing, %s record: %+v", name, ex)
		}
	}
}

// Two objects alike in every feature are told apart by their holders: the one a
// second parameter also holds numbers alike whichever order either side met them.
func TestExecuteIdenticalObjectsNumberByReference(t *testing.T) {
	s := fixtureSuite(t, objectModel)
	twins := fixtureActivity(t, s, "Twins")
	lone, picked := object("a", "Item"), object("b", "Item")
	record := func(both ...ExpectedValue) ExpectedActivity {
		return executed(twins, []ExpectedOutput{
			{Parameter: "both", Values: both},
			{Parameter: "pick", Values: []ExpectedValue{picked}},
		})
	}
	forward, backward := record(lone, picked), record(picked, lone)
	want := "both = Item#1{n = -; opt = -; set = -; xs = -}, Item#2{n = -; opt = -; set = -; xs = -}\npick = #1"
	for name, x := range map[string]ExpectedActivity{"forward": forward, "backward": backward} {
		if got := renderExpected(twins, &x); got != want {
			t.Errorf("renderExpected(%s) = %q, want %q", name, got, want)
		}
		ex, err := Execute(context.Background(), emitted(t, s, "Twins"), &x, DefaultBudget, 1)
		if err != nil {
			t.Fatal(err)
		}
		if !ex.Passed() || strings.Join(ex.Reached, "|") != strings.ReplaceAll(want, "\n", "; ") || ex.Runs < 2 {
			t.Errorf("Twins, %s record: %+v", name, ex)
		}
	}
}

// signalModel exercises the signal rules: Ping carries a level, Pong specializes
// it, Notifier sends a Pong to a Target it is given, Listener accepts a Ping onto
// its result pin and then a bare Pong, and Carrier's attribute is a signal.
const signalModel = `<?xml version="1.0" encoding="UTF-8"?>
<uml:Model xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="Signals">
  <packagedElement xmi:type="uml:Signal" xmi:id="ping" name="Ping">
    <ownedAttribute xmi:type="uml:Property" xmi:id="level" name="level">` + integerType + `</ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Signal" xmi:id="pong" name="Pong">
    <generalization xmi:type="uml:Generalization" xmi:id="pongGen" general="ping"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Signal" xmi:id="idle" name="Idle"/>
  <packagedElement xmi:type="uml:Signal" xmi:id="envelope" name="Envelope">
    <ownedAttribute xmi:type="uml:Property" xmi:id="payload" name="payload" type="message"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Signal" xmi:id="chain" name="Chain">
    <ownedAttribute xmi:type="uml:Property" xmi:id="next" name="next" type="chain">
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="nextLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="nextHi" value="1"/>
    </ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:SignalEvent" xmi:id="pingEvent" signal="ping"/>
  <packagedElement xmi:type="uml:SignalEvent" xmi:id="pongEvent" signal="pong"/>
  <packagedElement xmi:type="uml:SignalEvent" xmi:id="envelopeEvent" signal="envelope"/>
  <packagedElement xmi:type="uml:Class" xmi:id="target" name="Target"/>
  <packagedElement xmi:type="uml:Class" xmi:id="message" name="Message">
    <ownedAttribute xmi:type="uml:Property" xmi:id="body" name="body">` + integerType + `</ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Class" xmi:id="carrier" name="Carrier">
    <ownedAttribute xmi:type="uml:Property" xmi:id="last" name="last" type="ping">
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="lastLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="lastHi" value="1"/>
    </ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="notifier" name="Notifier">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="notifierTo" name="to" direction="in" type="target"/>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="notifierOut" name="sent" direction="out">` + integerType + `</ownedParameter>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="notifierToNode" name="Parameter(to)" parameter="notifierTo"/>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v4" name="Value(4)">
      <result xmi:type="uml:OutputPin" xmi:id="v4r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v4v" value="4"/>
    </node>
    <node xmi:type="uml:ForkNode" xmi:id="notifierFork" name="Fork"/>
    <node xmi:type="uml:SendSignalAction" xmi:id="sendPong" name="Send(Pong)" signal="pong">
      <target xmi:type="uml:InputPin" xmi:id="sendPongTarget" name="target" type="target"/>
      <argument xmi:type="uml:InputPin" xmi:id="sendPongLevel" name="level">` + integerType + `</argument>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="notifierOutNode" name="Parameter(sent)" parameter="notifierOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n1" source="notifierToNode" target="sendPongTarget"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n2" source="v4r" target="notifierFork"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n3" source="notifierFork" target="sendPongLevel"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n4" source="notifierFork" target="notifierOutNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="listener" name="Listener">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="listenerOut" name="heard" direction="out" type="ping">
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="heardLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="heardHi" value="1"/>
    </ownedParameter>
    <node xmi:type="uml:InitialNode" xmi:id="listenerInit" name="Initial"/>
    <node xmi:type="uml:AcceptEventAction" xmi:id="acceptPing" name="Accept(Ping)">
      <result xmi:type="uml:OutputPin" xmi:id="acceptPingR" name="signal" type="ping"/>
      <trigger xmi:type="uml:Trigger" xmi:id="pingTrigger" event="pingEvent"/>
    </node>
    <node xmi:type="uml:AcceptEventAction" xmi:id="acceptPong" name="Accept(Pong)">
      <trigger xmi:type="uml:Trigger" xmi:id="pongTrigger" event="pongEvent"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="listenerOutNode" name="Parameter(heard)" parameter="listenerOut"/>
    <edge xmi:type="uml:ControlFlow" xmi:id="l1" source="listenerInit" target="acceptPing"/>
    <edge xmi:type="uml:ControlFlow" xmi:id="l2" source="acceptPing" target="acceptPong"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="l3" source="acceptPingR" target="listenerOutNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="opener" name="Opener">
    <node xmi:type="uml:AcceptEventAction" xmi:id="acceptEnvelope" name="Accept(Envelope)">
      <trigger xmi:type="uml:Trigger" xmi:id="envelopeTrigger" event="envelopeEvent"/>
    </node>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="poster" name="Poster">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="posterTo" name="to" direction="in" type="target"/>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="posterOut" name="posted" direction="out" type="message"/>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="posterToNode" name="Parameter(to)" parameter="posterTo"/>
    <node xmi:type="uml:CreateObjectAction" xmi:id="createMessage" name="Create(Message)" classifier="message">
      <result xmi:type="uml:OutputPin" xmi:id="createMessageR" name="result" type="message"/>
    </node>
    <node xmi:type="uml:ForkNode" xmi:id="posterFork" name="Fork"/>
    <node xmi:type="uml:SendSignalAction" xmi:id="sendEnvelope" name="Send(Envelope)" signal="envelope">
      <target xmi:type="uml:InputPin" xmi:id="sendEnvelopeTarget" name="target" type="target"/>
      <argument xmi:type="uml:InputPin" xmi:id="sendEnvelopePayload" name="payload" type="message"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="posterOutNode" name="Parameter(posted)" parameter="posterOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="q1" source="posterToNode" target="sendEnvelopeTarget"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="q2" source="createMessageR" target="posterFork"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="q3" source="posterFork" target="sendEnvelopePayload"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="q4" source="posterFork" target="posterOutNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="linker" name="Linker">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="linkerOut" name="link" direction="out" type="chain">
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="linkLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="linkHi" value="1"/>
    </ownedParameter>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="unmarshaller" name="Unmarshaller">
    <node xmi:type="uml:AcceptEventAction" xmi:id="acceptUnmarshalled" name="Accept(Ping)" isUnmarshall="true">
      <result xmi:type="uml:OutputPin" xmi:id="acceptLevel" name="level">` + integerType + `</result>
      <trigger xmi:type="uml:Trigger" xmi:id="unmarshallTrigger" event="pingEvent"/>
    </node>
  </packagedElement>
</uml:Model>
`

// A signal is an attribute definition holding its attributes, specializing its
// generals with `:>`, declared before the classes and activities that name it;
// the closure follows the signals sent, accepted and typing parameters, pins and
// attributes, and their generals, and leaves the rest out.
func TestEmitSignals(t *testing.T) {
	s := fixtureSuite(t, signalModel)
	em := emitted(t, s, "Notifier")
	wantLines(t, em,
		"\tattribute def Ping {\n\t\tattribute level : Integer;\n\t}\n",
		"\tattribute def Pong :> Ping;\n",
		"\tpart def Target {\n\t}\n",
		"in 'to' : Target;")
	if strings.Index(em.Text, "attribute def Ping") > strings.Index(em.Text, "attribute def Pong") ||
		strings.Index(em.Text, "attribute def Pong") > strings.Index(em.Text, "part def Target") {
		t.Errorf("signals are not declared generals first, before the classes:\n%s", em.Text)
	}
	for _, stray := range []string{"Idle", "Carrier", "Envelope", "Message"} {
		if strings.Contains(em.Text, stray) {
			t.Errorf("%s declared though Notifier never names it:\n%s", stray, em.Text)
		}
	}
	em = emitted(t, s, "Listener")
	wantLines(t, em, "\tattribute def Ping {", "\tattribute def Pong :> Ping;\n", "out heard : Ping[0..1] = ();")
	if strings.Contains(em.Text, "Target") {
		t.Errorf("Target declared though Listener never names it:\n%s", em.Text)
	}
}

// redefinitionModel exercises redefined properties: Loud specializes Ping and
// redefines its level, Special specializes Base and redefines its n, Shouter
// sends a Loud with the one argument the effective signal takes, and Marker
// creates a Special and writes its redefining n.
const redefinitionModel = `<?xml version="1.0" encoding="UTF-8"?>
<uml:Model xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="Redefinitions">
  <packagedElement xmi:type="uml:Signal" xmi:id="ping" name="Ping">
    <ownedAttribute xmi:type="uml:Property" xmi:id="level" name="level">` + integerType + `</ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Signal" xmi:id="loud" name="Loud">
    <generalization xmi:type="uml:Generalization" xmi:id="loudGen" general="ping"/>
    <ownedAttribute xmi:type="uml:Property" xmi:id="loudLevel" name="level" redefinedProperty="level">` + integerType + `</ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Class" xmi:id="target" name="Target"/>
  <packagedElement xmi:type="uml:Class" xmi:id="base" name="Base">
    <ownedAttribute xmi:type="uml:Property" xmi:id="n" name="n">` + integerType + `</ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Class" xmi:id="special" name="Special">
    <generalization xmi:type="uml:Generalization" xmi:id="specialGen" general="base"/>
    <ownedAttribute xmi:type="uml:Property" xmi:id="specialN" name="n">` + integerType + `
      <redefinedProperty xmi:idref="n"/>
    </ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="shouter" name="Shouter">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="shouterTo" name="to" direction="in" type="target"/>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="shouterOut" name="sent" direction="out">` + integerType + `</ownedParameter>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="shouterToNode" name="Parameter(to)" parameter="shouterTo"/>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v8" name="Value(8)">
      <result xmi:type="uml:OutputPin" xmi:id="v8r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v8v" value="8"/>
    </node>
    <node xmi:type="uml:ForkNode" xmi:id="shouterFork" name="Fork"/>
    <node xmi:type="uml:SendSignalAction" xmi:id="sendLoud" name="Send(Loud)" signal="loud">
      <target xmi:type="uml:InputPin" xmi:id="sendLoudTarget" name="target" type="target"/>
      <argument xmi:type="uml:InputPin" xmi:id="sendLoudLevel" name="level">` + integerType + `</argument>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="shouterOutNode" name="Parameter(sent)" parameter="shouterOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="s1" source="shouterToNode" target="sendLoudTarget"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="s2" source="v8r" target="shouterFork"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="s3" source="shouterFork" target="sendLoudLevel"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="s4" source="shouterFork" target="shouterOutNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="marker" name="Marker">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="markerOut" name="made" direction="out" type="special"/>
    <node xmi:type="uml:CreateObjectAction" xmi:id="createSpecial" name="Create(Special)" classifier="special">
      <result xmi:type="uml:OutputPin" xmi:id="createSpecialR" name="result" type="special"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v3" name="Value(3)">
      <result xmi:type="uml:OutputPin" xmi:id="v3r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v3v" value="3"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="writeSpecialN" name="Write(n)" structuralFeature="specialN" isReplaceAll="true">
      <object xmi:type="uml:InputPin" xmi:id="writeSpecialNo" name="object" type="special"/>
      <value xmi:type="uml:InputPin" xmi:id="writeSpecialNv" name="value">` + integerType + `</value>
      <result xmi:type="uml:OutputPin" xmi:id="writeSpecialNr" name="result" type="special"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="markerOutNode" name="Parameter(made)" parameter="markerOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="k1" source="createSpecialR" target="writeSpecialNo"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="k2" source="v3r" target="writeSpecialNv"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="k3" source="writeSpecialNr" target="markerOutNode"/>
  </packagedElement>
</uml:Model>
`

// A property redefining an inherited one replaces it: the effective attributes of
// the class or signal hold the redefinition alone, so a send of the specialized
// signal takes one argument and the object of the specialized class is written
// and rendered through one feature; the redefinition is spelled `:>>`.
func TestEmitRedefinedProperties(t *testing.T) {
	s := fixtureSuite(t, redefinitionModel)
	loud := s.Tests.SignalOf(TypeRef{Name: "Loud"})
	special := s.Tests.ClassOf(TypeRef{Name: "Special"})
	if loud == nil || special == nil {
		t.Fatal("Loud or Special missing")
	}
	if attrs := loud.AllAttributes(); len(attrs) != 1 || attrs[0].ID != "loudLevel" {
		t.Errorf("Loud.AllAttributes() = %v, want the redefining level alone", attrs)
	}
	if attrs := special.AllAttributes(); len(attrs) != 1 || attrs[0].ID != "specialN" {
		t.Errorf("Special.AllAttributes() = %v, want the redefining n alone", attrs)
	}
	em := emitted(t, s, "Shouter")
	wantLines(t, em,
		"\tattribute def Loud :> Ping {\n\t\tattribute :>> level : Integer;\n\t}\n",
		"action 'Send(Loud)' { in target : Target; in level : Integer; send new Loud(level = level) to target; }")
	x := executed(fixtureActivity(t, s, "Shouter"), []ExpectedOutput{integers("sent", 8)})
	ex, err := Execute(context.Background(), em, &x, DefaultBudget, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.Passed() || strings.Join(ex.Reached, "|") != "sent = 8" {
		t.Errorf("Shouter: %+v", ex)
	}
	em = emitted(t, s, "Marker")
	wantLines(t, em,
		"\tpart def Special :> Base {\n\t\tattribute :>> n : Integer;\n\t}\n",
		"action 'Write(n)' { in object : Special; in value : Integer; out result : Special = object; assign object.n := value; }")
	x = executed(fixtureActivity(t, s, "Marker"), []ExpectedOutput{{Parameter: "made", Values: []ExpectedValue{object("sp", "Special", feature("n", 3))}}})
	if ex, err = Execute(context.Background(), em, &x, DefaultBudget, 1); err != nil {
		t.Fatal(err)
	}
	if !ex.Passed() || strings.Join(ex.Reached, "|") != "made = Special#1{n = 3}" {
		t.Errorf("Marker: %+v", ex)
	}
	renamed := &Property{Name: "loudness", Redefines: []*Property{{Name: "level"}}}
	if name, err := redefinedName(fixtureActivity(t, s, "Marker"), "attribute Loud.loudness", renamed); err != nil || name != "loudness :>> level" {
		t.Errorf("redefinedName(renaming) = %q, %v", name, err)
	}
}

// A signal typing a class's attribute is declared, and the attribute keeps its type.
func TestEmitSignalTypedAttribute(t *testing.T) {
	s := fixtureSuite(t, signalModel)
	c := s.Tests.ClassOf(TypeRef{Name: "Carrier"})
	if c == nil {
		t.Fatal("no class Carrier")
	}
	root := fixtureActivity(t, s, "Listener")
	text, err := emitClass(root, c)
	if err != nil {
		t.Fatal(err)
	}
	if text != "\tpart def Carrier {\n\t\tattribute last : Ping [0..1];\n\t}\n" {
		t.Errorf("Carrier:\n%s", text)
	}
	signals, err := signalClosure(root, nil, []*Class{c})
	if err != nil || len(signals) != 1 || signals[0].Name != "Ping" {
		t.Errorf("signalClosure(Carrier) = %v, %v; want Ping", signals, err)
	}
}

// A signal's attribute typed by a class is a reference to an object of it, and
// the class is declared even where the signal alone names it; a send carries the
// object and the sender still holds it afterwards.
func TestEmitClassTypedSignalAttribute(t *testing.T) {
	s := fixtureSuite(t, signalModel)
	em := emitted(t, s, "Opener")
	wantLines(t, em,
		"\tattribute def Envelope {\n\t\tref part payload : Message;\n\t}\n",
		"\tpart def Message {\n\t\tattribute body : Integer;\n\t}\n",
		"action 'Accept(Envelope)' accept Envelope;")
	if strings.Index(em.Text, "attribute def Envelope") > strings.Index(em.Text, "part def Message") {
		t.Errorf("the signal is not declared before the class it references:\n%s", em.Text)
	}
	poster := fixtureActivity(t, s, "Poster")
	x := executed(poster, []ExpectedOutput{{Parameter: "posted", Values: []ExpectedValue{object("m", "Message", feature("body"))}}})
	ex, err := Execute(context.Background(), emitted(t, s, "Poster"), &x, DefaultBudget, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.Passed() || strings.Join(ex.Reached, "|") != "posted = Message#1{body = -}" {
		t.Errorf("Poster: %+v", ex)
	}
}

// namesakeModel declares a class named Integer beside the primitive: Holder
// references one and holds an optional primitive Integer; Boxing creates a
// Holder, writes and positionally removes its optional value, and boxes an object.
const namesakeModel = `<?xml version="1.0" encoding="UTF-8"?>
<uml:Model xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="Namesakes">
  <packagedElement xmi:type="uml:Class" xmi:id="intclass" name="Integer">
    <ownedAttribute xmi:type="uml:Property" xmi:id="intn" name="n">` + integerType + `</ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Class" xmi:id="holder" name="Holder">
    <ownedAttribute xmi:type="uml:Property" xmi:id="hvalue" name="value" type="intclass">
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="hvalueLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="hvalueHi" value="1"/>
    </ownedAttribute>
    <ownedAttribute xmi:type="uml:Property" xmi:id="hopt" name="opt">` + integerType + `
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="hoptLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="hoptHi" value="1"/>
    </ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="boxing" name="Boxing">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="boxingOut" name="boxed" direction="out" type="holder"/>
    <node xmi:type="uml:CreateObjectAction" xmi:id="createHolder" name="Create(Holder)" classifier="holder">
      <result xmi:type="uml:OutputPin" xmi:id="createHolderR" name="result" type="holder"/>
    </node>
    <node xmi:type="uml:CreateObjectAction" xmi:id="createInt" name="Create(Integer)" classifier="intclass">
      <result xmi:type="uml:OutputPin" xmi:id="createIntR" name="result" type="intclass"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="b7" name="Value(7)">
      <result xmi:type="uml:OutputPin" xmi:id="b7r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="b7v" value="7"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="b9" name="Value(9)">
      <result xmi:type="uml:OutputPin" xmi:id="b9r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="b9v" value="9"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="b1" name="Value(1)">
      <result xmi:type="uml:OutputPin" xmi:id="b1r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="b1v" value="1"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="writeOpt" name="Write(opt)" structuralFeature="hopt" isReplaceAll="true">
      <object xmi:type="uml:InputPin" xmi:id="writeOpto" name="object" type="holder"/>
      <value xmi:type="uml:InputPin" xmi:id="writeOptv" name="value">` + integerType + `</value>
      <result xmi:type="uml:OutputPin" xmi:id="writeOptr" name="result" type="holder"/>
    </node>
    <node xmi:type="uml:RemoveStructuralFeatureValueAction" xmi:id="removeOpt" name="RemoveAt(opt)" structuralFeature="hopt">
      <object xmi:type="uml:InputPin" xmi:id="removeOpto" name="object" type="holder"/>
      <value xmi:type="uml:InputPin" xmi:id="removeOptv" name="value">` + integerType + `</value>
      <removeAt xmi:type="uml:InputPin" xmi:id="removeOpta" name="removeAt">` + integerType + `</removeAt>
      <result xmi:type="uml:OutputPin" xmi:id="removeOptr" name="result" type="holder"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="writeValue" name="Write(value)" structuralFeature="hvalue" isReplaceAll="true">
      <object xmi:type="uml:InputPin" xmi:id="writeValueo" name="object" type="holder"/>
      <value xmi:type="uml:InputPin" xmi:id="writeValuev" name="value" type="intclass"/>
      <result xmi:type="uml:OutputPin" xmi:id="writeValuer" name="result" type="holder"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="boxingOutNode" name="Parameter(boxed)" parameter="boxingOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n1" source="createHolderR" target="writeOpto"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n2" source="b7r" target="writeOptv"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n3" source="writeOptr" target="removeOpto"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n4" source="b9r" target="removeOptv"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n5" source="b1r" target="removeOpta"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n6" source="removeOptr" target="writeValueo"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n7" source="createIntR" target="writeValuev"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="n8" source="writeValuer" target="boxingOutNode"/>
  </packagedElement>
</uml:Model>
`

// A class named as a primitive is the class wherever the model references it,
// and the primitive is spelled qualified beside it so neither takes the other
// over; a positioned removal from a single-valued feature empties it at
// position 1 whatever value the pin holds, as the reference implementation does.
func TestEmitPrimitiveNamesakeAndPositionedScalarRemove(t *testing.T) {
	s := fixtureSuite(t, namesakeModel)
	em := emitted(t, s, "Boxing")
	wantLines(t, em,
		"\tpart def Integer {\n\t\tattribute n : ScalarValues::Integer;\n\t}\n",
		"\tpart def Holder {\n\t\tref part value : Integer [0..1];\n\t\tattribute opt : ScalarValues::Integer [0..1];\n\t}\n",
		"action 'Value(7)' { out result : ScalarValues::Integer = 7; }",
		"action 'Create(Integer)' { out result : Integer = new Integer(); }",
		"action 'RemoveAt(opt)' { in object : Holder; in value : ScalarValues::Integer; in removeAt : ScalarValues::Integer; out result : Holder = object; assign object.opt := if removeAt == 1 ? () else object.opt; }",
		"action 'Write(value)' { in object : Holder; in value : Integer; out result : Holder = object; assign object.value := value; }")
	boxing := fixtureActivity(t, s, "Boxing")
	boxed := object("h", "Holder", ExpectedFeature{Feature: "value", Values: []ExpectedValue{{Kind: "Reference", Referent: &ExpectedValue{Kind: "Object", ID: "i", Types: []string{"Integer"}, Features: []ExpectedFeature{{Feature: "n"}}}}}})
	boxed.Features = append(boxed.Features, ExpectedFeature{Feature: "opt"})
	x := executed(boxing, []ExpectedOutput{{Parameter: "boxed", Values: []ExpectedValue{boxed}}})
	ex, err := Execute(context.Background(), em, &x, DefaultBudget, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.Passed() || strings.Join(ex.Reached, "|") != "boxed = Holder#1{opt = -; value = Integer#2{n = -}}" {
		t.Errorf("Boxing: %+v", ex)
	}
	if got := featureUpdate(&Node{Kind: RemoveStructuralFeatureValueAction, RemoveDuplicates: true}, &Property{Name: "opt", Multiplicity: Multiplicity{Upper: 1}}, true); got != "if object.opt == value ? () else object.opt" {
		t.Errorf("a removeDuplicates removal from a scalar = %q, want the value compared", got)
	}
}

// idNamesakeModel gives a class the XMI id Integer, the fragment the UML
// primitive's href ends in: Sum adds two primitive Integers, and Wrap holds
// an object of the class in an Integer-typed parameter.
const idNamesakeModel = `<?xml version="1.0" encoding="UTF-8"?>
<uml:Model xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="IdNamesakes">
  <packagedElement xmi:type="uml:Class" xmi:id="Integer" name="Counter">
    <ownedAttribute xmi:type="uml:Property" xmi:id="cn" name="n">` + integerType + `</ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Signal" xmi:id="String" name="Note"/>
  <packagedElement xmi:type="uml:Activity" xmi:id="sum" name="Sum">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="sumA" name="a" direction="in">` + integerType + `</ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="sumOut" name="total" direction="out">` + integerType + `</ownedParameter>
    <ownedParameter xmi:type="uml:Parameter" xmi:id="sumLabel" name="label" direction="out">
      <type xmi:type="uml:PrimitiveType" href="pathmap://UML_LIBRARIES/UMLPrimitiveTypes.library.uml#String"/>
    </ownedParameter>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="sumANode" name="Parameter(a)" parameter="sumA"/>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="s2" name="Value(2)">
      <result xmi:type="uml:OutputPin" xmi:id="s2r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="s2v" value="2"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="sl" name="Value(ok)">
      <result xmi:type="uml:OutputPin" xmi:id="slr" name="result">
        <type xmi:type="uml:PrimitiveType" href="pathmap://UML_LIBRARIES/UMLPrimitiveTypes.library.uml#String"/>
      </result>
      <value xmi:type="uml:LiteralString" xmi:id="slv" value="ok"/>
    </node>
    <node xmi:type="uml:CallBehaviorAction" xmi:id="add" name="Add">
      <argument xmi:type="uml:InputPin" xmi:id="addX" name="x">` + integerType + `</argument>
      <argument xmi:type="uml:InputPin" xmi:id="addY" name="y">` + integerType + `</argument>
      <result xmi:type="uml:OutputPin" xmi:id="addR" name="result">` + integerType + `</result>
      <behavior xmi:type="uml:FunctionBehavior" href="fUML_Library.xmi#PrimitiveBehaviors-IntegerFunctions-plus"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="sumOutNode" name="Parameter(total)" parameter="sumOut"/>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="sumLabelNode" name="Parameter(label)" parameter="sumLabel"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="i1" source="sumANode" target="addX"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="i2" source="s2r" target="addY"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="i3" source="addR" target="sumOutNode"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="i4" source="slr" target="sumLabelNode"/>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="wrap" name="Wrap">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="wrapOut" name="made" direction="out" type="Integer"/>
    <node xmi:type="uml:CreateObjectAction" xmi:id="createCounter" name="Create(Counter)" classifier="Integer">
      <result xmi:type="uml:OutputPin" xmi:id="createCounterR" name="result" type="Integer"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="wrapOutNode" name="Parameter(made)" parameter="wrapOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="w1" source="createCounterR" target="wrapOutNode"/>
  </packagedElement>
</uml:Model>
`

// An external type reference names an element of another document, so its href
// fragment selects no class or signal of the model even when one carries it as
// its XMI id: the primitive stays a scalar, and a local reference by that id
// still finds the classifier.
func TestEmitExternalTypesNeverLocalClassifiers(t *testing.T) {
	s := fixtureSuite(t, idNamesakeModel)
	m := s.Tests
	if c := m.ClassOf(TypeRef{ID: "Integer", Name: "Integer", Kind: "PrimitiveType", External: true}); c != nil {
		t.Errorf("external Integer resolves to class %s", c.Name)
	}
	if sg := m.SignalOf(TypeRef{ID: "String", Name: "String", Kind: "PrimitiveType", External: true}); sg != nil {
		t.Errorf("external String resolves to signal %s", sg.Name)
	}
	if c := m.ClassOf(TypeRef{ID: "Integer", Name: "Counter", Kind: "Class"}); c == nil || c.Name != "Counter" {
		t.Errorf("local reference by id Integer = %v, want Counter", c)
	}
	if got := m.primitive(TypeRef{ID: "Integer", Name: "Integer", External: true}); got != "Integer" {
		t.Errorf("external Integer as a primitive = %q", got)
	}
	if got := m.primitive(TypeRef{ID: "Integer", Name: "Counter"}); got != "" {
		t.Errorf("the class with id Integer as a primitive = %q, want none", got)
	}
	em := emitted(t, s, "Sum")
	wantLines(t, em,
		"in a : Integer;",
		"out total : Integer;",
		"out label : String;",
		"action 'Value(2)' { out result : Integer = 2; }")
	if strings.Contains(em.Text, "part def") || strings.Contains(em.Text, "Counter") {
		t.Errorf("Sum declares a class it never touches:\n%s", em.Text)
	}
	sum := fixtureActivity(t, s, "Sum")
	x := executed(sum, []ExpectedOutput{integers("total", 2), {Parameter: "label", Values: []ExpectedValue{{Kind: "String", Value: json.RawMessage(`"ok"`)}}}})
	ex, err := Execute(context.Background(), em, &x, DefaultBudget, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.Passed() || strings.Join(ex.Reached, "|") != "total = 2; label = \"ok\"" {
		t.Errorf("Sum: %+v", ex)
	}
	wantLines(t, emitted(t, s, "Wrap"),
		"\tpart def Counter {\n\t\tattribute n : Integer;\n\t}\n",
		"out made : Counter;",
		"action 'Create(Counter)' { out result : Counter = new Counter(); }")
}

// zeroInsertModel inserts into an ordered feature at position 0, which no
// one-based position is.
const zeroInsertModel = `<?xml version="1.0" encoding="UTF-8"?>
<uml:Model xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.eclipse.org/uml2/5.0.0/UML" xmi:id="m" name="ZeroInsert">
  <packagedElement xmi:type="uml:Class" xmi:id="item" name="Item">
    <ownedAttribute xmi:type="uml:Property" xmi:id="xs" name="xs" isOrdered="true" isUnique="false">` + integerType + `
      <lowerValue xmi:type="uml:LiteralInteger" xmi:id="xsLo"/>
      <upperValue xmi:type="uml:LiteralUnlimitedNatural" xmi:id="xsHi" value="*"/>
    </ownedAttribute>
  </packagedElement>
  <packagedElement xmi:type="uml:Activity" xmi:id="zero" name="InsertAtZero">
    <ownedParameter xmi:type="uml:Parameter" xmi:id="zeroOut" name="made" direction="out" type="item"/>
    <node xmi:type="uml:CreateObjectAction" xmi:id="create" name="Create(Item)" classifier="item">
      <result xmi:type="uml:OutputPin" xmi:id="createR" name="result" type="item"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v9" name="Value(9)">
      <result xmi:type="uml:OutputPin" xmi:id="v9r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralInteger" xmi:id="v9v" value="9"/>
    </node>
    <node xmi:type="uml:ValueSpecificationAction" xmi:id="v0" name="Value(0)">
      <result xmi:type="uml:OutputPin" xmi:id="v0r" name="result">` + integerType + `</result>
      <value xmi:type="uml:LiteralUnlimitedNatural" xmi:id="v0v" value="0"/>
    </node>
    <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="insert" name="Insert(xs)" structuralFeature="xs">
      <object xmi:type="uml:InputPin" xmi:id="inserto" name="object" type="item"/>
      <value xmi:type="uml:InputPin" xmi:id="insertv" name="value">` + integerType + `</value>
      <insertAt xmi:type="uml:InputPin" xmi:id="inserta" name="insertAt">` + integerType + `</insertAt>
      <result xmi:type="uml:OutputPin" xmi:id="insertr" name="result" type="item"/>
    </node>
    <node xmi:type="uml:ActivityParameterNode" xmi:id="zeroOutNode" name="Parameter(made)" parameter="zeroOut"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="z1" source="createR" target="inserto"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="z2" source="v9r" target="insertv"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="z3" source="v0r" target="inserta"/>
    <edge xmi:type="uml:ObjectFlow" xmi:id="z4" source="insertr" target="zeroOutNode"/>
  </packagedElement>
</uml:Model>
`

// Every position but `*` reaches includingAt, so an insertion at 0 is the
// runtime's out-of-range error rather than a silent insertion first.
func TestExecuteInsertAtZeroIsOutOfRange(t *testing.T) {
	s := fixtureSuite(t, zeroInsertModel)
	em := emitted(t, s, "InsertAtZero")
	wantLines(t, em, "assign object.xs := if insertAt < 0 ? including(object.xs, value) else includingAt(object.xs, value, insertAt); }")
	zero := fixtureActivity(t, s, "InsertAtZero")
	x := executed(zero, []ExpectedOutput{{Parameter: "made", Values: []ExpectedValue{object("i", "Item", feature("xs", 9))}}})
	ex, err := Execute(context.Background(), em, &x, DefaultBudget, 1)
	if err != nil {
		t.Fatal(err)
	}
	if ex.Passed() || len(ex.Errors) != 1 || !strings.Contains(ex.Errors[0], runtime.ErrIndexOutOfRange.Error()) || !strings.Contains(ex.Errors[0], "insertion index 0") {
		t.Errorf("insertion at 0: %+v", ex)
	}
}

// A send signal action is an action taking the target and one value per
// attribute of the signal, generals' included, whose body sends a new instance
// to the target; it completes without waiting for a reply.
func TestEmitSendSignal(t *testing.T) {
	s := fixtureSuite(t, signalModel)
	em := emitted(t, s, "Notifier")
	wantLines(t, em,
		"action 'Send(Pong)' { in target : Target; in level : Integer; send new Pong(level = level) to target; }",
		"flow 'Parameter(to)'.v to 'Send(Pong)'.target;",
		"flow 'Value(4)'.result to 'Send(Pong)'.level;",
		"succession first Fork then 'Send(Pong)';")
	if strings.Contains(em.Text, "accept") {
		t.Errorf("a send waits:\n%s", em.Text)
	}
}

// An accept event action with a result pin is an accept node binding the signal
// received to the pin, typed by the trigger's signal; one without a pin is a
// bare accept; an unmarshalling accept is a TranslateError.
func TestEmitAcceptEvent(t *testing.T) {
	s := fixtureSuite(t, signalModel)
	em := emitted(t, s, "Listener")
	wantLines(t, em,
		"action 'Accept(Ping)' accept signal : Ping;",
		"action 'Accept(Pong)' accept Pong;",
		"succession first 'Accept(Ping)' then 'Accept(Ping) fork';",
		"succession first 'Accept(Ping) fork' then 'Accept(Pong)';",
		"flow 'Accept(Ping)'.signal to 'Parameter(heard)'.v;")
	_, err := Emit(fixtureActivity(t, s, "Unmarshaller"))
	var te *TranslateError
	if !errors.As(err, &te) || te.Where != "Accept(Ping)" || !strings.Contains(te.Reason, "unmarshalls") {
		t.Errorf("Emit(Unmarshaller) = %v, want a TranslateError on the unmarshalling accept", err)
	}
}

// A signal an output holds spells by the attributes its definition declares, as
// an object does, on the record's side and the run's alike.
func TestRenderSignalValues(t *testing.T) {
	s := fixtureSuite(t, signalModel)
	listener := fixtureActivity(t, s, "Listener")
	em := emitted(t, s, "Listener")
	heard := ExpectedValue{Kind: "Signal", Types: []string{"Ping"}, Features: []ExpectedFeature{feature("level", 8)}}
	x := executed(listener, []ExpectedOutput{{Parameter: "heard", Values: []ExpectedValue{heard}}})
	const want = "heard = Ping#1{level = 8}"
	if got := renderExpected(listener, &x); got != want {
		t.Errorf("expected side:\n%s\nwant\n%s", got, want)
	}
	budgets, err := runBudgets()
	if err != nil {
		t.Fatal(err)
	}
	action, _, fresh, err := build(em, budgets)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := fresh(0)
	if err != nil {
		t.Fatal(err)
	}
	ping, ok := action.OwnerScope.LookupLocal("Ping")
	if !ok {
		t.Fatal("the emitted model declares no Ping")
	}
	inst, err := ctx.InstantiateRead(ping, func(inst *runtime.Instance) error {
		return inst.SetFeatureValue(ctx, "level", runtime.Value{Kind: runtime.ValConst, Const: semantics.Value{Kind: semantics.ValInt, Int: 8}})
	})
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string]runtime.Value{"heard": {Kind: runtime.ValInstance, Instance: inst.ID}}
	if got := renderOutputs(listener, ctx, outputs); got != want {
		t.Errorf("run side:\n%s\nwant\n%s", got, want)
	}
	// A signal is a value: one instance two outputs hold spells whole in each,
	// as the record's two mentions do, never as an alias of the first.
	pingRef := TypeRef{ID: "Ping", Name: "Ping", Kind: "Signal"}
	twice := &Activity{Model: listener.Model, Parameters: []*Parameter{
		{Name: "first", Direction: Out, Type: pingRef, Multiplicity: Multiplicity{Lower: 1, Upper: 1, Unique: true}},
		{Name: "second", Direction: Out, Type: pingRef, Multiplicity: Multiplicity{Lower: 1, Upper: 1, Unique: true}},
	}}
	x = executed(twice, []ExpectedOutput{
		{Parameter: "first", Values: []ExpectedValue{heard}},
		{Parameter: "second", Values: []ExpectedValue{heard}},
	})
	const wantTwice = "first = Ping#1{level = 8}\nsecond = Ping#2{level = 8}"
	if got := renderExpected(twice, &x); got != wantTwice {
		t.Errorf("expected side, twice:\n%s\nwant\n%s", got, wantTwice)
	}
	shared := runtime.Value{Kind: runtime.ValInstance, Instance: inst.ID}
	if got := renderOutputs(twice, ctx, map[string]runtime.Value{"first": shared, "second": shared}); got != wantTwice {
		t.Errorf("run side, twice:\n%s\nwant\n%s", got, wantTwice)
	}
	// A signal reached again while its own features are read is that signal, not
	// a copy: a cycle through a feature spells as an alias of the one being filled.
	linker := fixtureActivity(t, s, "Linker")
	em = emitted(t, s, "Linker")
	action, _, fresh, err = build(em, budgets)
	if err != nil {
		t.Fatal(err)
	}
	if ctx, err = fresh(0); err != nil {
		t.Fatal(err)
	}
	chain, ok := action.OwnerScope.LookupLocal("Chain")
	if !ok {
		t.Fatal("the emitted model declares no Chain")
	}
	loop, err := ctx.InstantiateRead(chain, func(inst *runtime.Instance) error {
		return inst.SetFeatureValue(ctx, "next", runtime.Value{Kind: runtime.ValInstance, Instance: inst.ID})
	})
	if err != nil {
		t.Fatal(err)
	}
	const wantLoop = "link = Chain#1{next = #1}"
	if got := renderOutputs(linker, ctx, map[string]runtime.Value{"link": {Kind: runtime.ValInstance, Instance: loop.ID}}); got != wantLoop {
		t.Errorf("run side, cycle:\n%s\nwant\n%s", got, wantLoop)
	}
}

// A send to an object that runs no behavior completes, the message left
// pending; an accept nothing sends to is a run error the runtime types as an
// accept deadlock — a finding about the run, not a construct left untranslated.
func TestExecuteSignals(t *testing.T) {
	s := fixtureSuite(t, signalModel)
	notifier := fixtureActivity(t, s, "Notifier")
	x := executed(notifier, []ExpectedOutput{integers("sent", 4)})
	ex, err := Execute(context.Background(), emitted(t, s, "Notifier"), &x, DefaultBudget, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !ex.Passed() || strings.Join(ex.Reached, "|") != "sent = 4" {
		t.Errorf("Notifier: %+v", ex)
	}
	listener := fixtureActivity(t, s, "Listener")
	x = executed(listener, []ExpectedOutput{{Parameter: "heard"}})
	if ex, err = Execute(context.Background(), emitted(t, s, "Listener"), &x, DefaultBudget, 1); err != nil {
		t.Fatal(err)
	}
	if ex.Passed() || len(ex.Errors) != 1 || !strings.Contains(ex.Errors[0], runtime.ErrAcceptDeadlock.Error()) {
		t.Errorf("Listener: %+v", ex)
	}
}
