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
		{"orthogonal region without initial", "", `<subvertex xmi:type="uml:State" xmi:id="xO" name="O"><region xmi:type="uml:Region" xmi:id="xOr1" name="R1"><subvertex xmi:type="uml:Pseudostate" xmi:id="xOi" name="I"/><subvertex xmi:type="uml:State" xmi:id="xO1" name="O.1"/><transition xmi:type="uml:Transition" xmi:id="xOt" source="xOi" target="xO1"/></region><region xmi:type="uml:Region" xmi:id="xOr2" name="R2"><subvertex xmi:type="uml:State" xmi:id="xO2" name="O.2"/></region></subvertex>`, "orthogonal region without an initial state O/R2"},
		{"redefined state", "", `<subvertex xmi:type="uml:State" xmi:id="xR" name="R" redefinedState="xS1"/>`, "redefined state R"},
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
