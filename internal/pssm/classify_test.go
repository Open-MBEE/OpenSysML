package pssm

import (
	"strings"
	"testing"
)

// machineSuite is one test package whose target machine has the given
// connection points and region body; the tester only sends Start.
func machineSuite(connectionPoints, body string) string {
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
        ` + connectionPoints + `
        <region xmi:type="uml:Region" xmi:id="regX" name="Region1">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="xInit" name="Initial1"/>
          <subvertex xmi:type="uml:State" xmi:id="xS1" name="S1">
            ` + traceCall("entry", "xS1entry", "S1(entry)") + `
          </subvertex>
          <subvertex xmi:type="uml:FinalState" xmi:id="xFin" name="FinalState1"/>
          <transition xmi:type="uml:Transition" xmi:id="xT1" name="T1" source="xInit" target="xS1"/>
          <transition xmi:type="uml:Transition" xmi:id="xT2" name="T2" source="xS1" target="xFin">
            <trigger xmi:type="uml:Trigger" xmi:id="xT2trig" event="evStart"/>
          </transition>
          ` + body + `
        </region>
      </ownedBehavior>
    </packagedElement>
` + tester("Area001_Tester") + `  </packagedElement>
  </packagedElement>
` + fixtureTail
}

func classifyFixture(t *testing.T, connectionPoints, body string) Classification {
	t.Helper()
	s := readFixture(t, machineSuite(connectionPoints, body))
	noDiagnostics(t, s)
	if len(s.Tests) != 1 {
		t.Fatalf("tests = %d", len(s.Tests))
	}
	return Classify(s.Tests[0])
}

func TestClassifyStandard(t *testing.T) {
	c := classifyFixture(t, "", `
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            <region xmi:type="uml:Region" xmi:id="xS2r1" name="R1">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xS2i" name="I"/>
              <subvertex xmi:type="uml:State" xmi:id="xS21" name="S2.1"/>
              <transition xmi:type="uml:Transition" xmi:id="xS2t" source="xS2i" target="xS21"/>
            </region>
            <region xmi:type="uml:Region" xmi:id="xS2r2" name="R2">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xS2i2" name="I"/>
              <subvertex xmi:type="uml:State" xmi:id="xS22" name="S2.2"/>
              <transition xmi:type="uml:Transition" xmi:id="xS2t2" source="xS2i2" target="xS22"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`)
	if c.Class != Standard || len(c.Uses) != 0 || c.Reason() != "standard notation only" {
		t.Errorf("standard machine classified %s %v (%s)", c.Class, c.Uses, c.Reason())
	}
	if !c.Class.Expressible() {
		t.Error("standard is not expressible")
	}
}

func TestClassifyExtensions(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"defer", `<subvertex xmi:type="uml:State" xmi:id="xD" name="D"><deferrableTrigger xmi:type="uml:Trigger" xmi:id="xDt" event="evData"/></subvertex>`, "defer D"},
		{"fork", `<subvertex xmi:type="uml:Pseudostate" xmi:id="xF" name="Fork1" kind="fork"/>`, "fork Fork1"},
		{"join", `<subvertex xmi:type="uml:Pseudostate" xmi:id="xJ" name="Join1" kind="join"/>`, "join Join1"},
		{"junction", `<subvertex xmi:type="uml:Pseudostate" xmi:id="xJn" name="Junction1" kind="junction"/>`, "junction Junction1"},
		{"choice", `<subvertex xmi:type="uml:Pseudostate" xmi:id="xC" name="Choice1" kind="choice"/>`, "choice Choice1"},
		{"shallow", `<subvertex xmi:type="uml:State" xmi:id="xH" name="H"><region xmi:type="uml:Region" xmi:id="xHr" name="R"><subvertex xmi:type="uml:Pseudostate" xmi:id="xHs" name="History1" kind="shallowHistory"/></region></subvertex>`, "shallow history H.History1"},
		{"deep", `<subvertex xmi:type="uml:Pseudostate" xmi:id="xDh" name="History1" kind="deepHistory"/>`, "deep history History1"},
		{"fork into regions without initial pseudostates", `<subvertex xmi:type="uml:Pseudostate" xmi:id="xF" name="Fork1" kind="fork"/><subvertex xmi:type="uml:State" xmi:id="xO" name="O"><region xmi:type="uml:Region" xmi:id="xOr1" name="R1"><subvertex xmi:type="uml:State" xmi:id="xO1" name="O.1"/></region><region xmi:type="uml:Region" xmi:id="xOr2" name="R2"><subvertex xmi:type="uml:State" xmi:id="xO2" name="O.2"/></region></subvertex><transition xmi:type="uml:Transition" xmi:id="xTf1" name="TF1" source="xF" target="xO1"/><transition xmi:type="uml:Transition" xmi:id="xTf2" name="TF2" source="xF" target="xO2"/>`, "fork Fork1"},
		{"fork into a state nested below a region without an initial pseudostate", `<subvertex xmi:type="uml:Pseudostate" xmi:id="xF" name="Fork1" kind="fork"/><subvertex xmi:type="uml:State" xmi:id="xO" name="O"><region xmi:type="uml:Region" xmi:id="xOr1" name="R1"><subvertex xmi:type="uml:State" xmi:id="xW" name="W"><region xmi:type="uml:Region" xmi:id="xWr" name="WR"><subvertex xmi:type="uml:Pseudostate" xmi:id="xWi" name="I"/><subvertex xmi:type="uml:State" xmi:id="xW0" name="W.0"/><subvertex xmi:type="uml:State" xmi:id="xO1" name="O.1"/><transition xmi:type="uml:Transition" xmi:id="xWt" source="xWi" target="xW0"/></region></subvertex></region><region xmi:type="uml:Region" xmi:id="xOr2" name="R2"><subvertex xmi:type="uml:State" xmi:id="xO2" name="O.2"/></region></subvertex><transition xmi:type="uml:Transition" xmi:id="xTf1" name="TF1" source="xF" target="xO1"/><transition xmi:type="uml:Transition" xmi:id="xTf2" name="TF2" source="xF" target="xO2"/>`, "fork Fork1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := classifyFixture(t, "", tc.body)
			if c.Class != Extension || c.Reason() != tc.want {
				t.Errorf("classified %s (%s), want extension (%s)", c.Class, c.Reason(), tc.want)
			}
			if !c.Class.Expressible() {
				t.Error("extension is not expressible")
			}
		})
	}
}

func TestClassifyTerminateOutranksExtensions(t *testing.T) {
	c := classifyFixture(t, "", `
          <subvertex xmi:type="uml:Pseudostate" xmi:id="xC" name="Choice1" kind="choice"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="xTm" name="Terminate1" kind="terminate"/>
          <transition xmi:type="uml:Transition" xmi:id="xT3" source="xS1" target="xTm">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`)
	if c.Class != TerminateGap || c.Reason() != "terminate Terminate1" {
		t.Errorf("classified %s (%s)", c.Class, c.Reason())
	}
	if c.Class.Expressible() {
		t.Error("terminate gap is expressible")
	}
	// The extension is still recorded, after the deciding use.
	if len(c.Uses) != 2 || c.Uses[0].Construct != ConstructTerminate || c.Uses[1].Construct != ConstructChoice {
		t.Errorf("uses = %v", c.Uses)
	}
}

func TestClassifyNoSpellingOutranksAll(t *testing.T) {
	cases := []struct {
		name, connectionPoints, body, want string
	}{
		{"machine entry point", `<connectionPoint xmi:type="uml:Pseudostate" xmi:id="xEP" name="EntryPoint1" kind="entryPoint"/>`, "", "entry point EntryPoint1"},
		{"machine exit point", `<connectionPoint xmi:type="uml:Pseudostate" xmi:id="xXP" name="ExitPoint1" kind="exitPoint"/>`, "", "exit point ExitPoint1"},
		{"state entry point", "", `<subvertex xmi:type="uml:State" xmi:id="xC" name="C"><connectionPoint xmi:type="uml:Pseudostate" xmi:id="xCep" name="EntryPoint1" kind="entryPoint"/></subvertex>`, "entry point EntryPoint1"},
		{"connection point reference", "", `<subvertex xmi:type="uml:State" xmi:id="xSub" name="Sub" submachine="smX"><connection xmi:type="uml:ConnectionPointReference" xmi:id="xSubc" name="ref"/></subvertex>`, "submachine state Sub; connection point reference Sub"},
		{"local transition", "", `<transition xmi:type="uml:Transition" xmi:id="xTl" name="TL" kind="local" source="xS1" target="xS1"/>`, "local transition TL"},
		{"internal transition", "", `<transition xmi:type="uml:Transition" xmi:id="xTi" name="TI" kind="internal" source="xS1" target="xS1"/>`, "internal transition TI"},
		{"extended region", "", `<subvertex xmi:type="uml:State" xmi:id="xE" name="E"><region xmi:type="uml:Region" xmi:id="xEr" name="R" extendedRegion="regX"/></subvertex>`, "extended region R"},
		{"orthogonal region nothing enters", "", `<subvertex xmi:type="uml:State" xmi:id="xO" name="O"><region xmi:type="uml:Region" xmi:id="xOr1" name="R1"><subvertex xmi:type="uml:Pseudostate" xmi:id="xOi" name="I"/><subvertex xmi:type="uml:State" xmi:id="xO1" name="O.1"/><transition xmi:type="uml:Transition" xmi:id="xOt" source="xOi" target="xO1"/></region><region xmi:type="uml:Region" xmi:id="xOr2" name="R2"><subvertex xmi:type="uml:State" xmi:id="xO2" name="O.2"/></region></subvertex>`, "lowerer refuses an orthogonal region with neither an entry transition nor a fork branch into it O/R2"},
		{"orthogonal region of a nested orthogonal state a fork passes through", "", `<subvertex xmi:type="uml:Pseudostate" xmi:id="xF" name="Fork1" kind="fork"/><subvertex xmi:type="uml:State" xmi:id="xO" name="O"><region xmi:type="uml:Region" xmi:id="xOr1" name="R1"><subvertex xmi:type="uml:State" xmi:id="xN" name="N"><region xmi:type="uml:Region" xmi:id="xNr1" name="P"><subvertex xmi:type="uml:State" xmi:id="xO1" name="O.1"/></region><region xmi:type="uml:Region" xmi:id="xNr2" name="Q"><subvertex xmi:type="uml:State" xmi:id="xQ1" name="Q.1"/></region></subvertex></region><region xmi:type="uml:Region" xmi:id="xOr2" name="R2"><subvertex xmi:type="uml:State" xmi:id="xO2" name="O.2"/></region></subvertex><transition xmi:type="uml:Transition" xmi:id="xTf1" name="TF1" source="xF" target="xO1"/><transition xmi:type="uml:Transition" xmi:id="xTf2" name="TF2" source="xF" target="xO2"/>`, "lowerer refuses an orthogonal region with neither an entry transition nor a fork branch into it O.N/P; lowerer refuses an orthogonal region with neither an entry transition nor a fork branch into it O.N/Q"},
		{"redefined state", "", `<subvertex xmi:type="uml:State" xmi:id="xR" name="R" redefinedState="xS1"/>`, "redefined state R"},
		{"guard side effect", "", guardWithSideEffect, "guard side effect T3"},
		{"redefined transition", "", `<transition xmi:type="uml:Transition" xmi:id="xTr" name="TR" source="xS1" target="xFin" redefinedTransition="xT2"/>`, "redefined transition TR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Every case also uses terminate and a choice, which must not win.
			body := tc.body + `
          <subvertex xmi:type="uml:Pseudostate" xmi:id="xCh" name="Choice1" kind="choice"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="xTm" name="Terminate1" kind="terminate"/>`
			c := classifyFixture(t, tc.connectionPoints, body)
			if c.Class != NotExpressible || c.Reason() != tc.want {
				t.Errorf("classified %s (%s), want not-expressible (%s)", c.Class, c.Reason(), tc.want)
			}
			if c.Class.Expressible() {
				t.Error("not-expressible is expressible")
			}
		})
	}
}

func TestClassifyStandaloneAndRedefinedMachine(t *testing.T) {
	src := strings.Replace(machineSuite("", ""),
		`<ownedBehavior xmi:type="uml:StateMachine" xmi:id="smX" name="Area001_Test">`,
		`<ownedBehavior xmi:type="uml:StateMachine" xmi:id="smX" name="Area001_Test" redefinedBehavior="smX">`, 1)
	s := readFixture(t, src)
	c := Classify(s.Tests[0])
	if c.Class != NotExpressible || c.Reason() != "redefined state machine Area001_Test" {
		t.Errorf("redefined machine classified %s (%s)", c.Class, c.Reason())
	}

	sa := &Test{
		Name:    "Standalone 001",
		Target:  &Class{Name: "SA_Test", Standalone: true},
		Machine: &StateMachine{Name: "SA_Test"},
	}
	c = Classify(sa)
	if c.Class != NotExpressible || c.Reason() != "standalone state machine SA_Test" {
		t.Errorf("standalone classified %s (%s)", c.Class, c.Reason())
	}

	c = Classify(&Test{Name: "none"})
	if c.Class != NotExpressible || c.Reason() != "no state machine" {
		t.Errorf("machineless classified %s (%s)", c.Class, c.Reason())
	}
}

func TestClassifyStrayConnectionPoint(t *testing.T) {
	// A join filed as a connection point that no transition reaches is
	// recorded but decides nothing; a reached one counts as a join.
	stray := `<connectionPoint xmi:type="uml:Pseudostate" xmi:id="xJ" name="Join1" kind="join"/>`
	c := classifyFixture(t, stray, "")
	if c.Class != Standard || len(c.Uses) != 1 || c.Uses[0].String() != "stray connection point join Join1" {
		t.Errorf("stray classified %s %v", c.Class, c.Uses)
	}
	c = classifyFixture(t, stray, `<transition xmi:type="uml:Transition" xmi:id="xT3" source="xJ" target="xFin"/>`)
	if c.Class != Extension || c.Reason() != "join Join1" {
		t.Errorf("reached connection point classified %s (%s)", c.Class, c.Reason())
	}
}

// guardBehavior writes a transition T3 from S1 to FinalState1 on evContinue whose
// guard `true` is an activity returning true, preceded by the given statements.
func guardBehavior(statements string, behaviors ...string) string {
	return `
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xFin" guard="xT3guard">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
            <ownedRule xmi:type="uml:Constraint" xmi:id="xT3guard">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="xT3spec" behavior="xT3act">
                <body>true</body>
                <language>Alf</language>
              </specification>
            </ownedRule>
          </transition>
        </region>
        <ownedBehavior xmi:type="uml:Activity" xmi:id="xT3act" name="T3_guard">
          <ownedParameter xmi:type="uml:Parameter" xmi:id="xT3ret" direction="return"/>
          <node xmi:type="uml:ActivityParameterNode" xmi:id="xT3retNode" name="Return" parameter="xT3ret"/>
          <node xmi:type="uml:StructuredActivityNode" xmi:id="xT3body" name="Body">
            ` + statements + `
            <node xmi:type="uml:StructuredActivityNode" xmi:id="xT3ret1" name="2:ReturnStatement">
              <node xmi:type="uml:ValueSpecificationAction" xmi:id="xT3true">
                <result xmi:type="uml:OutputPin" xmi:id="xT3trueOut"/>
                <value xmi:type="uml:LiteralBoolean" xmi:id="xT3trueLit" value="true"/>
              </node>
              <structuredNodeOutput xmi:type="uml:OutputPin" xmi:id="xT3ret1Out"/>
              <edge xmi:type="uml:ObjectFlow" xmi:id="xT3e1" source="xT3trueOut" target="xT3ret1Out"/>
            </node>
          </node>
          <edge xmi:type="uml:ObjectFlow" xmi:id="xT3e2" source="xT3ret1Out" target="xT3retNode"/>
        </ownedBehavior>` + strings.Join(behaviors, "") + `
        <region xmi:type="uml:Region" xmi:id="regX2" name="Region2">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="xInit2" name="Initial2"/>
          <subvertex xmi:type="uml:State" xmi:id="xS9" name="S9"/>
          <transition xmi:type="uml:Transition" xmi:id="xT9" name="T9" source="xInit2" target="xS9"/>`
}

// guardWithSideEffect is a guard whose behavior traces before it returns.
var guardWithSideEffect = guardBehavior(`
            <node xmi:type="uml:StructuredActivityNode" xmi:id="xT3stmt1" name="1:ExpressionStatement">
              <node xmi:type="uml:ReadSelfAction" xmi:id="xT3self"><result xmi:type="uml:OutputPin" xmi:id="xT3selfOut"/></node>
              <node xmi:type="uml:ValueSpecificationAction" xmi:id="xT3val">
                <result xmi:type="uml:OutputPin" xmi:id="xT3valOut"/>
                <value xmi:type="uml:LiteralString" xmi:id="xT3lit" value="T3(guard)"/>
              </node>
              <node xmi:type="uml:CallOperationAction" xmi:id="xT3call" operation="opTrace">
                <target xmi:type="uml:InputPin" xmi:id="xT3callTarget"/>
                <argument xmi:type="uml:InputPin" xmi:id="xT3callArg"/>
              </node>
              <edge xmi:type="uml:ObjectFlow" xmi:id="xT3e3" source="xT3selfOut" target="xT3callTarget"/>
              <edge xmi:type="uml:ObjectFlow" xmi:id="xT3e4" source="xT3valOut" target="xT3callArg"/>
            </node>
            <edge xmi:type="uml:ControlFlow" xmi:id="xT3e5" source="xT3stmt1" target="xT3ret1"/>`)

// helperActivity writes an activity xHelper of the test class holding the given nodes.
func helperActivity(nodes string) string {
	return `
        <ownedBehavior xmi:type="uml:Activity" xmi:id="xHelper" name="helper">
          ` + nodes + `
        </ownedBehavior>`
}

var (
	pureHelper    = helperActivity(`<node xmi:type="uml:ReadSelfAction" xmi:id="xHelperSelf"><result xmi:type="uml:OutputPin" xmi:id="xHelperSelfOut"/></node>`)
	writingHelper = helperActivity(`<node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="xHelperWrite" structuralFeature="attrCounter"/>`)
)

func TestClassifyGuardSideEffect(t *testing.T) {
	c := classifyFixture(t, "", guardWithSideEffect)
	if c.Class != NotExpressible || c.Reason() != "guard side effect T3" {
		t.Errorf("classified %s (%s), want not-expressible (guard side effect T3)", c.Class, c.Reason())
	}
	// A guard behavior that only computes its value is the expression it spells.
	c = classifyFixture(t, "", guardBehavior(""))
	if c.Class != Standard {
		t.Errorf("pure guard behavior classified %s (%s), want standard", c.Class, c.Reason())
	}
	// Control flow the reader does not evaluate is not a side effect either.
	c = classifyFixture(t, "", guardBehavior(`
            <node xmi:type="uml:ConditionalNode" xmi:id="xT3if" name="1:IfStatement"/>
            <edge xmi:type="uml:ControlFlow" xmi:id="xT3e5" source="xT3if" target="xT3ret1"/>`))
	if c.Class != Standard {
		t.Errorf("branching guard behavior classified %s (%s), want standard", c.Class, c.Reason())
	}
	// Nor is a call of a behavior that only computes: a library function or
	// an activity of the document whose nodes all read.
	c = classifyFixture(t, "", guardBehavior(`
            <node xmi:type="uml:CallBehaviorAction" xmi:id="xT3not" name="Call(!)">
              <behavior href="http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#PrimitiveBehaviors-BooleanFunctions-Not"/>
            </node>
            <node xmi:type="uml:CallBehaviorAction" xmi:id="xT3helper" name="Call(helper)" behavior="xHelper"/>
            <edge xmi:type="uml:ControlFlow" xmi:id="xT3e5" source="xT3not" target="xT3helper"/>
            <edge xmi:type="uml:ControlFlow" xmi:id="xT3e6" source="xT3helper" target="xT3ret1"/>`,
		pureHelper))
	if c.Class != Standard {
		t.Errorf("guard behavior calling pure behaviors classified %s (%s), want standard", c.Class, c.Reason())
	}
	// An action the reader does not express still acts on the model, whether
	// it stands alone, is nested in control flow the reader does not evaluate,
	// or is reached through a call.
	for name, statements := range map[string]string{
		"destroy": `
            <node xmi:type="uml:DestroyObjectAction" xmi:id="xT3destroy" name="Destroy"/>
            <edge xmi:type="uml:ControlFlow" xmi:id="xT3e5" source="xT3destroy" target="xT3ret1"/>`,
		"write inside a conditional": `
            <node xmi:type="uml:ConditionalNode" xmi:id="xT3if" name="1:IfStatement">
              <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="xT3write" structuralFeature="attrCounter"/>
            </node>
            <edge xmi:type="uml:ControlFlow" xmi:id="xT3e5" source="xT3if" target="xT3ret1"/>`,
		"library output": `
            <node xmi:type="uml:CallBehaviorAction" xmi:id="xT3write" name="Call(WriteLine)">
              <behavior href="http://www.omg.org/spec/FUML/20180501/fUML_Library.xmi#BasicInputOutput-WriteLine"/>
            </node>
            <edge xmi:type="uml:ControlFlow" xmi:id="xT3e5" source="xT3write" target="xT3ret1"/>`,
	} {
		c = classifyFixture(t, "", guardBehavior(statements))
		if c.Class != NotExpressible || c.Reason() != "guard side effect T3" {
			t.Errorf("%s: classified %s (%s), want not-expressible (guard side effect T3)", name, c.Class, c.Reason())
		}
	}
	callHelper := `
            <node xmi:type="uml:CallBehaviorAction" xmi:id="xT3helper" name="Call(helper)" behavior="xHelper"/>
            <edge xmi:type="uml:ControlFlow" xmi:id="xT3e5" source="xT3helper" target="xT3ret1"/>`
	opaqueHelper := `<ownedBehavior xmi:type="uml:OpaqueBehavior" xmi:id="xHelper" name="helper"><body>return true;</body><language>Alf</language></ownedBehavior>`
	for name, helper := range map[string]string{
		"write in a called behavior":     writingHelper,
		"write two calls down":           helperActivity(`<node xmi:type="uml:CallBehaviorAction" xmi:id="xHelperCall" behavior="xWriter"/>`) + strings.ReplaceAll(writingHelper, "xHelper", "xWriter"),
		"call of a behavior not defined": helperActivity(`<node xmi:type="uml:CallBehaviorAction" xmi:id="xHelperCall" behavior="xNowhere"/>`),
		"call of an opaque behavior":     opaqueHelper,
	} {
		c = classifyFixture(t, "", guardBehavior(callHelper, helper))
		if c.Class != NotExpressible || c.Reason() != "guard side effect T3" {
			t.Errorf("%s: classified %s (%s), want not-expressible (guard side effect T3)", name, c.Class, c.Reason())
		}
	}
	// A called function behavior does not act, by contract.
	c = classifyFixture(t, "", guardBehavior(callHelper, strings.Replace(opaqueHelper, "uml:OpaqueBehavior", "uml:FunctionBehavior", 1)))
	if c.Class != Standard {
		t.Errorf("guard behavior calling a function behavior classified %s (%s), want standard", c.Class, c.Reason())
	}
	// A guard behavior that is not an activity is not read, so whether it acts
	// is unknown; a function behavior does not act by contract.
	unread := func(kind string) string {
		return strings.Replace(guardBehavior(""), `<ownedBehavior xmi:type="uml:Activity" xmi:id="xT3act" name="T3_guard">`,
			`<ownedBehavior xmi:type="`+kind+`" xmi:id="xT3act" name="T3_guard"><body>return true;</body><language>Alf</language></ownedBehavior>
        <ownedBehavior xmi:type="uml:Activity" xmi:id="xT3unused" name="unused">`, 1)
	}
	c = classifyFixture(t, "", unread("uml:OpaqueBehavior"))
	if c.Class != NotExpressible || c.Reason() != "guard behavior not read T3" {
		t.Errorf("opaque guard behavior classified %s (%s), want not-expressible (guard behavior not read T3)", c.Class, c.Reason())
	}
	if c = classifyFixture(t, "", unread("uml:FunctionBehavior")); c.Class != Standard {
		t.Errorf("function guard behavior classified %s (%s), want standard", c.Class, c.Reason())
	}
	// A behavior calling itself is read once.
	c = classifyFixture(t, "", guardBehavior(callHelper, helperActivity(`<node xmi:type="uml:CallBehaviorAction" xmi:id="xHelperCall" behavior="xHelper"/>`)))
	if c.Class != Standard {
		t.Errorf("guard behavior calling a recursive pure behavior classified %s (%s), want standard", c.Class, c.Reason())
	}
	// An operation is what its method does, however the method is referenced.
	callOp := `
            <node xmi:type="uml:CallOperationAction" xmi:id="xT3op" operation="xOp"/>
            <edge xmi:type="uml:ControlFlow" xmi:id="xT3e5" source="xT3op" target="xT3ret1"/>`
	for _, op := range []string{
		`<ownedOperation xmi:type="uml:Operation" xmi:id="xOp" name="op" method="xHelper"/>`,
		`<ownedOperation xmi:type="uml:Operation" xmi:id="xOp" name="op"><method xmi:idref="xHelper"/></ownedOperation>`,
	} {
		if c = classifyFixture(t, "", guardBehavior(callOp, pureHelper, op)); c.Class != Standard {
			t.Errorf("guard behavior calling a pure operation classified %s (%s), want standard", c.Class, c.Reason())
		}
		if c = classifyFixture(t, "", guardBehavior(callOp, writingHelper, op)); c.Class != NotExpressible || c.Reason() != "guard side effect T3" {
			t.Errorf("guard behavior calling a writing operation classified %s (%s), want not-expressible (guard side effect T3)", c.Class, c.Reason())
		}
	}
}
