package fuml

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/runtime"
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
// Reader reads and clears the features of an Item it is given; Holder's owned
// behavior Reflect reads self.
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
		"action 'Insert(xs)' { in object : Item; in value : Integer; in insertAt : Integer; out result : Item = object; assign object.xs := if insertAt < 0 ? including(object.xs, value) else if insertAt == 0 ? (value, object.xs) else includingAt(object.xs, value, insertAt); }",
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
