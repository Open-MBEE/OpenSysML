package pssm

import (
	"strings"
	"testing"
)

// fixtureHead opens a document in the suite's shape: the shared architecture
// classes and signals every test package specializes and sends.
const fixtureHead = `<?xml version="1.0" encoding="UTF-8"?>
<xmi:XMI xmi:version="20131001" xmlns:xmi="http://www.omg.org/spec/XMI/20131001" xmlns:uml="http://www.omg.org/spec/UML/20131001">
<uml:Model xmi:id="model" name="Fixture">
  <packagedElement xmi:type="uml:Package" xmi:id="util" name="Util">
    <packagedElement xmi:type="uml:Signal" xmi:id="sigStart" name="Start"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="sigContinue" name="Continue"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="sigData" name="IntegerData">
      <ownedAttribute xmi:type="uml:Property" xmi:id="sigDataValue" name="value">
        <type href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="clsTarget" name="Target">
      <ownedOperation xmi:type="uml:Operation" xmi:id="opTrace" name="trace">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="opTraceP" name="segment">
          <type href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#String"/>
        </ownedParameter>
      </ownedOperation>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="clsTester" name="Tester">
      <ownedAttribute xmi:type="uml:Property" xmi:id="testerTestable" name="testable" type="clsTarget"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="clsSemanticTest" name="SemanticTest">
      <ownedAttribute xmi:type="uml:Property" xmi:id="stName" name="name"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="stExpected" name="expectedTraces"/>
    </packagedElement>
  </packagedElement>
`

const fixtureTail = `</uml:Model>
</xmi:XMI>
`

// registration writes a suite-style registration statement list for one test:
// `t = new Sem(); t.name = "<name>"; t.expectedTraces->add("<trace>")...`.
func registration(id, semID, name string, traces ...string) string {
	var b strings.Builder
	b.WriteString(`  <packagedElement xmi:type="uml:Activity" xmi:id="` + id + `Tests" name="` + id + `Tests">
    <node xmi:type="uml:StructuredActivityNode" xmi:id="` + id + `Body" name="Body">
      <node xmi:type="uml:StructuredActivityNode" xmi:id="` + id + `S1" name="1:Expression Statement">
        <node xmi:type="uml:CreateObjectAction" xmi:id="` + id + `Create" name="Create" classifier="` + semID + `">
          <result xmi:type="uml:OutputPin" xmi:id="` + id + `CreateOut"/>
        </node>
        <node xmi:type="uml:ForkNode" xmi:id="` + id + `Fork" name="Fork(t)"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + id + `E1" source="` + id + `CreateOut" target="` + id + `Fork"/>
      </node>
`)
	write := func(n int, feature, value string) {
		s := id + "W" + string(rune('a'+n))
		b.WriteString(`      <node xmi:type="uml:StructuredActivityNode" xmi:id="` + s + `" name="` + itoa(n+2) + `:Expression Statement">
        <node xmi:type="uml:ValueSpecificationAction" xmi:id="` + s + `V" name="Value">
          <result xmi:type="uml:OutputPin" xmi:id="` + s + `VOut"/>
          <value xmi:type="uml:LiteralString" xmi:id="` + s + `Lit" value="` + value + `"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="` + s + `Write" name="Write" structuralFeature="` + feature + `">
          <object xmi:type="uml:InputPin" xmi:id="` + s + `Obj"/>
          <value xmi:type="uml:InputPin" xmi:id="` + s + `Val"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E" source="` + s + `VOut" target="` + s + `Val"/>
      </node>
      <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `Feed" source="` + id + `Fork" target="` + s + `Obj"/>
`)
	}
	write(0, "stName", name)
	for i, tr := range traces {
		write(i+1, "stExpected", tr)
	}
	b.WriteString("    </node>\n  </packagedElement>\n")
	return b.String()
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + itoa(n%10)
}

// tester writes a Tester class whose classifier behavior accepts Start and
// then sends the given signals to `this.testable`.
func tester(id string, sends ...string) string {
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
	for i, sig := range sends {
		s := id + "Send" + itoa(i)
		b.WriteString(`        <node xmi:type="uml:ReadSelfAction" xmi:id="` + s + `Self"><result xmi:type="uml:OutputPin" xmi:id="` + s + `SelfOut"/></node>
        <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="` + s + `Read" structuralFeature="testerTestable">
          <object xmi:type="uml:InputPin" xmi:id="` + s + `ReadObj"/>
          <result xmi:type="uml:OutputPin" xmi:id="` + s + `ReadOut"/>
        </node>
        <node xmi:type="uml:SendSignalAction" xmi:id="` + s + `" name="Send" signal="` + sig + `">
          <target xmi:type="uml:InputPin" xmi:id="` + s + `Target"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E1" source="` + s + `SelfOut" target="` + s + `ReadObj"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + s + `E2" source="` + s + `ReadOut" target="` + s + `Target"/>
        <edge xmi:type="uml:ControlFlow" xmi:id="` + s + `E3" source="` + prev + `" target="` + s + `"/>
`)
		prev = s
	}
	b.WriteString("      </ownedBehavior>\n    </packagedElement>\n")
	return b.String()
}

// traceCall writes an activity whose one statement is `this.trace("<segment>")`.
func traceCall(tag, id, segment string) string {
	return `<` + tag + ` xmi:type="uml:Activity" xmi:id="` + id + `" name="` + id + `">
  <node xmi:type="uml:ReadSelfAction" xmi:id="` + id + `Self"><result xmi:type="uml:OutputPin" xmi:id="` + id + `SelfOut"/></node>
  <node xmi:type="uml:ValueSpecificationAction" xmi:id="` + id + `Val">
    <result xmi:type="uml:OutputPin" xmi:id="` + id + `ValOut"/>
    <value xmi:type="uml:LiteralString" xmi:id="` + id + `Lit" value="` + segment + `"/>
  </node>
  <node xmi:type="uml:CallOperationAction" xmi:id="` + id + `Call" operation="opTrace">
    <target xmi:type="uml:InputPin" xmi:id="` + id + `CallTarget"/>
    <argument xmi:type="uml:InputPin" xmi:id="` + id + `CallArg"/>
  </node>
  <edge xmi:type="uml:ObjectFlow" xmi:id="` + id + `E1" source="` + id + `SelfOut" target="` + id + `CallTarget"/>
  <edge xmi:type="uml:ObjectFlow" xmi:id="` + id + `E2" source="` + id + `ValOut" target="` + id + `CallArg"/>
</` + tag + `>
`
}

const fixtureEvents = `  <packagedElement xmi:type="uml:SignalEvent" xmi:id="evStart" name="StartEvent" signal="sigStart"/>
  <packagedElement xmi:type="uml:SignalEvent" xmi:id="evContinue" name="ContinueEvent" signal="sigContinue"/>
  <packagedElement xmi:type="uml:SignalEvent" xmi:id="evData" name="DataEvent" signal="sigData"/>
`

// simpleSuite is one test package: a target whose machine goes wait -> S1 on
// Start, S1 -> S2 on Continue with a guard and an effect, S2 -> final; a
// tester sending Continue; two expected traces.
func simpleSuite() string {
	return fixtureHead + fixtureEvents +
		`  <packagedElement xmi:type="uml:Package" xmi:id="areaBehavior" name="Behavior">
` + registration("Behavior", "semBehavior001", "Behavior 001", "S1(entry)::T2(effect)", "S1(entry)") +
		`  <packagedElement xmi:type="uml:Package" xmi:id="pkgBehavior001" name="001">
    <ownedComment xmi:type="uml:Comment" xmi:id="pkgComment" body="RTC steps: Start, Continue"/>
    <packagedElement xmi:type="uml:Class" xmi:id="semBehavior001" name="Behavior001_SemanticTest">
      <generalization xmi:type="uml:Generalization" xmi:id="semGen" general="clsSemanticTest"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="tgtBehavior001" name="Behavior001_Test" classifierBehavior="smBehavior001">
      <generalization xmi:type="uml:Generalization" xmi:id="tgtGen" general="clsTarget"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="tgtValue" name="value">
        <type href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        <defaultValue xmi:type="uml:LiteralInteger" xmi:id="tgtValueDefault" value="5"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="smBehavior001" name="Behavior001_Test">
        <region xmi:type="uml:Region" xmi:id="reg1" name="Region1">
          <ownedComment xmi:type="uml:Comment" xmi:id="regComment" body="The machine under test."/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="init1" name="Initial1"/>
          <subvertex xmi:type="uml:State" xmi:id="stWait" name="wait"/>
          <subvertex xmi:type="uml:State" xmi:id="stS1" name="S1">
            ` + traceCall("entry", "s1entry", "S1(entry)") + `
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="s1defer" event="evData"/>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="stS2" name="S2">
            <region xmi:type="uml:Region" xmi:id="reg2" name="Region2">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="init2" name="Initial2" kind="initial"/>
              <subvertex xmi:type="uml:State" xmi:id="stS21" name="S2.1"/>
              <subvertex xmi:type="uml:Pseudostate" xmi:id="hist" name="History" kind="shallowHistory"/>
              <transition xmi:type="uml:Transition" xmi:id="t21" name="T2.1" source="init2" target="stS21"/>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:FinalState" xmi:id="fin1" name="FinalState1"/>
          <subvertex xmi:type="uml:Pseudostate" xmi:id="term" name="Terminate1" kind="terminate"/>
          <transition xmi:type="uml:Transition" xmi:id="t1" name="T1" source="init1" target="stWait"/>
          <transition xmi:type="uml:Transition" xmi:id="t2" name="T2" source="stWait" target="stS1">
            <trigger xmi:type="uml:Trigger" xmi:id="t2trig" event="evStart"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="t3" name="T3" source="stS1" target="stS2" guard="t3guard">
            <trigger xmi:type="uml:Trigger" xmi:id="t3trig" event="evContinue"/>
            <ownedRule xmi:type="uml:Constraint" xmi:id="t3guard" name="isFive">
              <specification xmi:type="uml:OpaqueExpression" xmi:id="t3spec">
                <body>this.value == 5</body>
                <language>Alf</language>
              </specification>
            </ownedRule>
            ` + traceCall("effect", "t3effect", "T2(effect)") + `
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="t4" name="T4" source="stS2" target="fin1"/>
          <transition xmi:type="uml:Transition" xmi:id="t5" name="T5" kind="local" source="stS2" target="stS21" guard="t5guard">
            <ownedRule xmi:type="uml:Constraint" xmi:id="t5guard">
              <specification xmi:type="uml:Expression" xmi:id="t5spec" symbol="else"/>
            </ownedRule>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="t6" name="T6" kind="internal" source="stS2" target="stS2" guard="t6guard">
            <ownedRule xmi:type="uml:Constraint" xmi:id="t6guard">
              <specification xmi:type="uml:LiteralBoolean" xmi:id="t6spec" value="true"/>
            </ownedRule>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="t7" name="T7" source="stS1" target="term"/>
        </region>
      </ownedBehavior>
    </packagedElement>
` + tester("Behavior001_Tester", "sigContinue", "sigData") + `  </packagedElement>
  </packagedElement>
` + fixtureTail
}

func readFixture(t *testing.T, src string) *Suite {
	t.Helper()
	s, err := Read(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return s
}

func noDiagnostics(t *testing.T, s *Suite) {
	t.Helper()
	for _, d := range s.Diagnostics {
		t.Errorf("suite diagnostic: %s", d)
	}
	for _, tt := range s.Tests {
		for _, d := range tt.Diagnostics {
			t.Errorf("%s diagnostic: %s", tt.Name, d)
		}
	}
}

func findVertex(t *testing.T, sm *StateMachine, name string) *Vertex {
	t.Helper()
	var found *Vertex
	var walk func([]*Region)
	walk = func(regs []*Region) {
		for _, r := range regs {
			for _, v := range r.Vertices {
				if v.Name == name {
					found = v
				}
				walk(v.Regions)
			}
		}
	}
	walk(sm.Regions)
	if found == nil {
		t.Fatalf("vertex %q not found", name)
	}
	return found
}

func findTransition(t *testing.T, sm *StateMachine, name string) *Transition {
	t.Helper()
	var found *Transition
	var walk func([]*Region)
	walk = func(regs []*Region) {
		for _, r := range regs {
			for _, tr := range r.Transitions {
				if tr.Name == name {
					found = tr
				}
			}
			for _, v := range r.Vertices {
				walk(v.Regions)
			}
		}
	}
	walk(sm.Regions)
	if found == nil {
		t.Fatalf("transition %q not found", name)
	}
	return found
}

func TestReadRegistration(t *testing.T) {
	s := readFixture(t, simpleSuite())
	noDiagnostics(t, s)
	if len(s.Tests) != 1 {
		t.Fatalf("tests = %d, want 1", len(s.Tests))
	}
	tt := s.Tests[0]
	if tt.Area != "Behavior" || tt.Name != "Behavior 001" || tt.ID != "Behavior001_SemanticTest" {
		t.Errorf("registration = %q/%q/%q", tt.Area, tt.Name, tt.ID)
	}
	if got := strings.Join(tt.Expected, "|"); got != "S1(entry)::T2(effect)|S1(entry)" {
		t.Errorf("expected traces = %q", got)
	}
	if tt.Target == nil || tt.Target.Name != "Behavior001_Test" || tt.Tester == nil || tt.Tester.Name != "Behavior001_Tester" {
		t.Fatalf("target/tester not attached: %+v", tt)
	}
	if tt.Machine == nil || tt.Machine.Owner != "Behavior001_Test" {
		t.Fatalf("machine not attached")
	}
	if len(tt.Target.Attributes) != 1 || tt.Target.Attributes[0].Type != "Integer" || tt.Target.Attributes[0].Default.String() != "5" {
		t.Errorf("attributes = %+v", tt.Target.Attributes)
	}
	if len(tt.Notes) != 1 || tt.Notes[0] != "The machine under test." {
		t.Errorf("notes = %q", tt.Notes)
	}
	if len(s.Signals) != 3 || s.Signals["IntegerData"].Attributes[0].Name != "value" {
		t.Errorf("signals = %v", s.Signals)
	}
}

func TestReadStimulation(t *testing.T) {
	s := readFixture(t, simpleSuite())
	stim := s.Tests[0].Stimulation
	if stim == nil {
		t.Fatal("no stimulation")
	}
	var got []string
	for _, st := range stim.Statements {
		got = append(got, st.String())
	}
	want := "accept Start; send Continue() to this.testable; send IntegerData() to this.testable"
	if strings.Join(got, "; ") != want {
		t.Errorf("stimulation = %q\nwant %q", strings.Join(got, "; "), want)
	}
	if len(stim.Unsupported) != 0 {
		t.Errorf("unsupported = %q", stim.Unsupported)
	}
}

func TestReadMachine(t *testing.T) {
	s := readFixture(t, simpleSuite())
	sm := s.Tests[0].Machine
	if len(sm.Regions) != 1 || sm.Regions[0].Name != "Region1" || sm.Regions[0].Owner() != nil {
		t.Fatalf("regions = %+v", sm.Regions)
	}
	kinds := map[string]VertexKind{
		"Initial1": VertexInitial, "wait": VertexState, "S1": VertexState, "S2": VertexState,
		"FinalState1": VertexFinal, "Terminate1": VertexTerminate, "Initial2": VertexInitial,
		"S2.1": VertexState, "History": VertexShallowHistory,
	}
	for name, kind := range kinds {
		if v := findVertex(t, sm, name); v.Kind != kind {
			t.Errorf("%s kind = %s, want %s", name, v.Kind, kind)
		}
	}
	s21 := findVertex(t, sm, "S2.1")
	if s21.Path() != "S2.S2.1" || s21.Region.Owner().Name != "S2" {
		t.Errorf("S2.1 path = %q owner = %v", s21.Path(), s21.Region.Owner())
	}
	s1 := findVertex(t, sm, "S1")
	if s1.Entry == nil || len(s1.Entry.Body.Statements) != 1 || s1.Entry.Body.Statements[0].String() != `this.trace("S1(entry)")` {
		t.Errorf("S1 entry = %+v", s1.Entry)
	}
	if len(s1.Deferred) != 1 || s1.Deferred[0].Event.Describe() != "IntegerData" {
		t.Errorf("S1 deferred = %+v", s1.Deferred)
	}
	if s1.Exit != nil || s1.Do != nil {
		t.Errorf("S1 has exit/do behaviors it does not declare")
	}
}

func TestReadTransitions(t *testing.T) {
	s := readFixture(t, simpleSuite())
	sm := s.Tests[0].Machine
	cases := map[string]string{
		"T1":   "transition T1 initial Initial1 -> state wait",
		"T2":   "transition T2 state wait -> state S1 on Start",
		"T3":   "transition T3 state S1 -> state S2 on Continue [Alf: this.value == 5]",
		"T4":   "transition T4 state S2 -> final FinalState1",
		"T5":   "local transition T5 state S2 -> state S2.S2.1 [else]",
		"T6":   "internal transition T6 state S2 -> state S2 [true]",
		"T7":   "transition T7 state S1 -> terminate Terminate1",
		"T2.1": "transition T2.1 initial S2.Initial2 -> state S2.S2.1",
	}
	for name, want := range cases {
		if got := findTransition(t, sm, name).Describe(); got != want {
			t.Errorf("%s = %q\nwant %q", name, got, want)
		}
	}
	t3 := findTransition(t, sm, "T3")
	if t3.Guard.Kind != GuardOpaque || t3.Guard.Name != "isFive" || t3.Guard.Behavior != nil {
		t.Errorf("T3 guard = %+v", t3.Guard)
	}
	if t3.Effect == nil || t3.Effect.Body.Statements[0].String() != `this.trace("T2(effect)")` {
		t.Errorf("T3 effect = %+v", t3.Effect)
	}
	if findTransition(t, sm, "T5").Guard.Kind != GuardElse || findTransition(t, sm, "T6").Guard.Kind != GuardLiteral {
		t.Errorf("guard kinds misread")
	}
	if findTransition(t, sm, "T2.1").Region.Owner().Name != "S2" {
		t.Errorf("T2.1 is not owned by S2's region")
	}
	if findTransition(t, sm, "T1").Guard != nil {
		t.Errorf("T1 has a guard it does not declare")
	}
}

func TestReadParseErrors(t *testing.T) {
	cases := map[string]string{
		"empty":       "",
		"unbalanced":  `<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"><a>`,
		"two roots":   `<a/><b/>`,
		"dup id":      `<xmi:XMI xmlns:xmi="http://www.omg.org/spec/XMI/20131001"><a xmi:id="x"/><b xmi:id="x"/></xmi:XMI>`,
		"not xml":     `hello`,
		"bad closing": `<a></b>`,
	}
	for name, src := range cases {
		if _, err := Read(strings.NewReader(src)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// oddSuite is a well-formed document the reader can only read in part: an
// unknown pseudostate and transition kind, an unresolved target and event, a
// test with no expected trace and no tester.
func oddSuite() string {
	return fixtureHead + fixtureEvents +
		`  <packagedElement xmi:type="uml:Package" xmi:id="areaX" name="Odd">
` + registration("Odd", "semOdd", "Odd 001") +
		`  <packagedElement xmi:type="uml:Package" xmi:id="pkgOdd" name="001">
    <packagedElement xmi:type="uml:Class" xmi:id="semOdd" name="Odd_SemanticTest">
      <generalization xmi:type="uml:Generalization" xmi:id="semOddGen" general="clsSemanticTest"/>
    </packagedElement>
    <packagedElement xmi:type="uml:Class" xmi:id="tgtOdd" name="Odd_Test" classifierBehavior="smOdd">
      <generalization xmi:type="uml:Generalization" xmi:id="tgtOddGen" general="clsTarget"/>
      <ownedBehavior xmi:type="uml:StateMachine" xmi:id="smOdd" name="Odd_Test">
        <region xmi:type="uml:Region" xmi:id="regOdd" name="Region1">
          <subvertex xmi:type="uml:Pseudostate" xmi:id="oddPs" name="P" kind="sideways"/>
          <subvertex xmi:type="uml:State" xmi:id="oddS" name="S"/>
          <transition xmi:type="uml:Transition" xmi:id="oddT" name="T" source="oddPs" target="missing" kind="diagonal">
            <trigger xmi:type="uml:Trigger" xmi:id="oddTrig" event="noSuchEvent"/>
          </transition>
        </region>
      </ownedBehavior>
    </packagedElement>
  </packagedElement>
  </packagedElement>
` + fixtureTail
}

func TestReadDiagnostics(t *testing.T) {
	s := readFixture(t, oddSuite())
	if len(s.Tests) != 1 {
		t.Fatalf("tests = %d", len(s.Tests))
	}
	joined := func(ds []Diagnostic) string {
		parts := make([]string, len(ds))
		for i, d := range ds {
			parts[i] = d.String()
		}
		return strings.Join(parts, "\n")
	}
	suite := joined(s.Diagnostics)
	for _, want := range []string{`kind "sideways"`, `kind "diagonal"`, "target missing is not a vertex", "event noSuchEvent is not in the document"} {
		if !strings.Contains(suite, want) {
			t.Errorf("suite diagnostics lack %q:\n%s", want, suite)
		}
	}
	test := joined(s.Tests[0].Diagnostics)
	for _, want := range []string{"no expected trace is registered", "no class specializing Tester"} {
		if !strings.Contains(test, want) {
			t.Errorf("test diagnostics lack %q:\n%s", want, test)
		}
	}
	if v := findVertex(t, s.Tests[0].Machine, "P"); v.Kind != VertexUnknown {
		t.Errorf("unknown pseudostate kind read as %s", v.Kind)
	}
	// The unresolved transition must still describe itself without panicking.
	tr := findTransition(t, s.Tests[0].Machine, "T")
	if got := tr.Describe(); got != "transition T unknown P -> <no vertex> on unresolved noSuchEvent" {
		t.Errorf("Describe = %q", got)
	}
	var none *Transition
	if none.Describe() != "<no transition>" || (*Vertex)(nil).Describe() != "<no vertex>" {
		t.Errorf("nil Describe not guarded")
	}
}

func TestReadStandaloneMachine(t *testing.T) {
	src := fixtureHead + fixtureEvents +
		`  <packagedElement xmi:type="uml:Package" xmi:id="areaSA" name="Standalone">
` + registration("Standalone", "semSA", "Standalone 001", "T1(effect)") +
		`  <packagedElement xmi:type="uml:Package" xmi:id="pkgSA" name="001">
    <packagedElement xmi:type="uml:Class" xmi:id="semSA" name="SA_SemanticTest">
      <generalization xmi:type="uml:Generalization" xmi:id="semSAGen" general="clsSemanticTest"/>
    </packagedElement>
    <packagedElement xmi:type="uml:StateMachine" xmi:id="smSA" name="SA_Test">
      <generalization xmi:type="uml:Generalization" xmi:id="smSAGen" general="clsTarget"/>
      <region xmi:type="uml:Region" xmi:id="regSA" name="Region1">
        <subvertex xmi:type="uml:Pseudostate" xmi:id="saInit" name="Initial1"/>
        <subvertex xmi:type="uml:State" xmi:id="saS" name="S"/>
        <transition xmi:type="uml:Transition" xmi:id="saT" name="T1" source="saInit" target="saS"/>
      </region>
    </packagedElement>
` + tester("SA_Tester") + `  </packagedElement>
  </packagedElement>
` + fixtureTail
	s := readFixture(t, src)
	noDiagnostics(t, s)
	tt := s.Tests[0]
	if tt.Target == nil || !tt.Target.Standalone || tt.Machine == nil || tt.Machine.Name != "SA_Test" || tt.Machine.Owner != "" {
		t.Fatalf("standalone machine misread: %+v", tt.Target)
	}
	if len(tt.Stimulation.Statements) != 1 || tt.Stimulation.Statements[0].Kind != StmtAccept {
		t.Errorf("stimulation = %+v", tt.Stimulation)
	}
}

func TestReadActivityExpressions(t *testing.T) {
	// A hand-compiled guard method in the shape Alf produces: numbered
	// statements, a fork for a local, a library call, a return parameter.
	src := fixtureHead + fixtureEvents +
		`  <packagedElement xmi:type="uml:Class" xmi:id="clsX" name="X">
    <ownedAttribute xmi:type="uml:Property" xmi:id="xValue" name="value"/>
    <ownedOperation xmi:type="uml:Operation" xmi:id="opBig" name="big" method="actBig">
      <ownedParameter xmi:type="uml:Parameter" xmi:id="opBigRet" direction="return"/>
    </ownedOperation>
    <ownedBehavior xmi:type="uml:Activity" xmi:id="actBig" name="big$method$1" specification="opBig">
      <ownedComment xmi:type="uml:Comment" xmi:id="actBigDoc" body="activity 'big$method$1'() : Boolean { return this.value &gt; 3; }"/>
      <ownedParameter xmi:type="uml:Parameter" xmi:id="actBigRet" direction="return"/>
      <node xmi:type="uml:ActivityParameterNode" xmi:id="retNode" name="Return" parameter="actBigRet"/>
      <node xmi:type="uml:StructuredActivityNode" xmi:id="bigBody" name="Body">
        <node xmi:type="uml:StructuredActivityNode" xmi:id="stmt1" name="1:ReturnStatement">
          <node xmi:type="uml:ReadSelfAction" xmi:id="rs"><result xmi:type="uml:OutputPin" xmi:id="rsOut"/></node>
          <node xmi:type="uml:ForkNode" xmi:id="rsFork"/>
          <node xmi:type="uml:ReadStructuralFeatureAction" xmi:id="rd" structuralFeature="xValue">
            <object xmi:type="uml:InputPin" xmi:id="rdObj"/>
            <result xmi:type="uml:OutputPin" xmi:id="rdOut"/>
          </node>
          <node xmi:type="uml:ValueSpecificationAction" xmi:id="v3">
            <result xmi:type="uml:OutputPin" xmi:id="v3Out"/>
            <value xmi:type="uml:LiteralInteger" xmi:id="v3Lit" value="3"/>
          </node>
          <node xmi:type="uml:CallBehaviorAction" xmi:id="gt" name="Call('&gt;')">
            <behavior href="http://www.omg.org/spec/ALF/20170201/Alf-Library.xmi#Alf-Library-PrimitiveBehaviors-IntegerFunctions-gt"/>
            <argument xmi:type="uml:InputPin" xmi:id="gtA"/>
            <argument xmi:type="uml:InputPin" xmi:id="gtB"/>
            <result xmi:type="uml:OutputPin" xmi:id="gtOut"/>
          </node>
          <structuredNodeOutput xmi:type="uml:OutputPin" xmi:id="stmt1Out"/>
          <edge xmi:type="uml:ObjectFlow" xmi:id="e1" source="rsOut" target="rsFork"/>
          <edge xmi:type="uml:ObjectFlow" xmi:id="e2" source="rsFork" target="rdObj"/>
          <edge xmi:type="uml:ObjectFlow" xmi:id="e3" source="rdOut" target="gtA"/>
          <edge xmi:type="uml:ObjectFlow" xmi:id="e4" source="v3Out" target="gtB"/>
          <edge xmi:type="uml:ObjectFlow" xmi:id="e5" source="gtOut" target="stmt1Out"/>
        </node>
      </node>
      <edge xmi:type="uml:ObjectFlow" xmi:id="e6" source="stmt1Out" target="retNode"/>
    </ownedBehavior>
  </packagedElement>
` + fixtureTail
	doc, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ReadDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	noDiagnostics(t, s)
	r := &reader{doc: doc, suite: &Suite{}, ops: map[string]*Operation{}, behaviors: map[string]*Behavior{}, events: map[string]*Event{}}
	body := r.readActivity(doc.ByID("actBig"))
	if len(body.Statements) != 1 || body.Statements[0].String() != "return gt(this.value, 3)" {
		t.Errorf("body = %+v", body.Statements)
	}
	if len(body.Unsupported) != 0 {
		t.Errorf("unsupported = %q", body.Unsupported)
	}
	b := r.behavior(doc.ByID("actBig"))
	if !strings.Contains(b.Source, "return this.value > 3") {
		t.Errorf("source = %q", b.Source)
	}
}

func TestReadUnsupportedNodes(t *testing.T) {
	src := fixtureHead +
		`  <packagedElement xmi:type="uml:Activity" xmi:id="actOdd" name="odd">
    <node xmi:type="uml:LoopNode" xmi:id="loop" name="while"/>
    <node xmi:type="uml:CreateLinkAction" xmi:id="link" name="link"/>
    <node xmi:type="uml:Something" xmi:id="odd" name="odd"/>
    <node xmi:type="uml:ForkNode" xmi:id="cycA"/>
    <node xmi:type="uml:ForkNode" xmi:id="cycB"/>
    <edge xmi:type="uml:ControlFlow" xmi:id="c1" source="cycA" target="cycB"/>
    <edge xmi:type="uml:ControlFlow" xmi:id="c2" source="cycB" target="cycA"/>
  </packagedElement>
` + fixtureTail
	doc, err := Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	r := &reader{doc: doc, suite: &Suite{}, ops: map[string]*Operation{}, behaviors: map[string]*Behavior{}, events: map[string]*Event{}}
	body := r.readActivity(doc.ByID("actOdd"))
	if len(body.Statements) != 0 {
		t.Errorf("statements = %+v", body.Statements)
	}
	joined := strings.Join(body.Unsupported, "\n")
	for _, want := range []string{"branches or loops", "manipulates objects", "node kind the reader does not know", "flow cycle"} {
		if !strings.Contains(joined, want) {
			t.Errorf("unsupported lacks %q:\n%s", want, joined)
		}
	}
}
