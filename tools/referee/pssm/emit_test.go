package pssm

import (
	"errors"
	"strings"
	"testing"
)

func emitFixture(t *testing.T, connectionPoints, body string) (*Model, error) {
	t.Helper()
	s := readFixture(t, machineSuite(connectionPoints, body))
	noDiagnostics(t, s)
	if len(s.Tests) != 1 {
		t.Fatalf("tests = %d", len(s.Tests))
	}
	return Emit(s, s.Tests[0])
}

// TestEmitStandard translates entry, exit and effect traces, nested and
// parallel states, into a model the front end and lowerer accept.
func TestEmitStandard(t *testing.T) {
	m, err := emitFixture(t, "", `
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            `+traceCall("exit", "xS2exit", "S2(exit)")+`
            <region xmi:type="uml:Region" xmi:id="xS2r1" name="R1">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xS2i" name="I"/>
              <subvertex xmi:type="uml:State" xmi:id="xS21" name="S2.1"/>
              <transition xmi:type="uml:Transition" xmi:id="xS2t" source="xS2i" target="xS21"/>
            </region>
            <region xmi:type="uml:Region" xmi:id="xS2r2" name="R2">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xS2j" name="I"/>
              <subvertex xmi:type="uml:State" xmi:id="xS22" name="S2.2"/>
              <transition xmi:type="uml:Transition" xmi:id="xS2u" source="xS2j" target="xS22"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
            `+traceCall("effect", "xT3effect", "T3(effect)")+`
          </transition>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`attribute log : String = "";`,
		`state S1 {`,
		`"S1(entry)"`,
		`"S2(exit)"`,
		`"T3(effect)"`,
		`accept Continue`,
		`parallel`,
		`then done;`,
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("model lacks %q:\n%s", want, m.Text)
		}
	}
	if problems := Validate(m); len(problems) > 0 {
		t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
	}
	if len(m.Events) != 1 || m.Events[0].Signal != "Start" {
		t.Errorf("events = %v, want Start", m.Events)
	}
}

func TestEmitTransitionTargetInEnclosingRegion(t *testing.T) {
	m, err := emitFixture(t, "", `
          <subvertex xmi:type="uml:State" xmi:id="xOuter" name="Outer">
            <region xmi:type="uml:Region" xmi:id="xOuterRegion" name="R1">
              <subvertex xmi:type="uml:State" xmi:id="xInner" name="Inner">
                <region xmi:type="uml:Region" xmi:id="xInnerRegion" name="R1">
                  <subvertex xmi:type="uml:State" xmi:id="xA" name="A"/>
                  <transition xmi:type="uml:Transition" xmi:id="xInnerToH" source="xA" target="xH">
                    <trigger xmi:type="uml:Trigger" xmi:id="xInnerToHTrigger" event="evContinue"/>
                  </transition>
                </region>
              </subvertex>
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xH" name="H" kind="deepHistory"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xOuterToH" source="xOuter" target="xH"/>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"transition first Outer_Inner.Outer_Inner_A accept Continue then Outer_H;",
		"transition first Outer then Outer.Outer_H;",
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("model lacks %q:\n%s", want, m.Text)
		}
	}
	if problems := Validate(m); len(problems) > 0 {
		t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
	}
}

func TestEmitTransitionFromOrthogonalRegionUsesScopedPaths(t *testing.T) {
	m, err := emitFixture(t, "", `
          <subvertex xmi:type="uml:State" xmi:id="xOuter" name="Outer">
            <region xmi:type="uml:Region" xmi:id="xOuterRegion" name="R1">
              <subvertex xmi:type="uml:State" xmi:id="xParallel" name="Parallel">
                <region xmi:type="uml:Region" xmi:id="xRegion1" name="Region1">
                  <subvertex xmi:type="uml:Pseudostate" xmi:id="xInitial1" name="I"/>
                  <subvertex xmi:type="uml:State" xmi:id="xA" name="A"/>
                  <transition xmi:type="uml:Transition" xmi:id="xInitialToA" source="xInitial1" target="xA"/>
                  <transition xmi:type="uml:Transition" xmi:id="xAToJoin" source="xA" target="xJoin"/>
                </region>
                <region xmi:type="uml:Region" xmi:id="xRegion2" name="Region2">
                  <subvertex xmi:type="uml:Pseudostate" xmi:id="xInitial2" name="I"/>
                  <subvertex xmi:type="uml:State" xmi:id="xB" name="B"/>
                  <transition xmi:type="uml:Transition" xmi:id="xInitialToB" source="xInitial2" target="xB"/>
                  <transition xmi:type="uml:Transition" xmi:id="xBToJoin" source="xB" target="xJoin"/>
                </region>
              </subvertex>
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xJoin" name="J" kind="join"/>
              <transition xmi:type="uml:Transition" xmi:id="xJoinToSibling" source="xJoin" target="xSibling"/>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="xSibling" name="Sibling"/>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"transition first Outer_Parallel.'Outer.Parallel/Region1'.Outer_Parallel_A then Outer_J;",
		"transition first Outer_Parallel.'Outer.Parallel/Region2'.Outer_Parallel_B then Outer_J;",
		"transition first Outer.Outer_J then Sibling;",
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("model lacks %q:\n%s", want, m.Text)
		}
	}
	if problems := Validate(m); len(problems) > 0 {
		t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
	}
}

// TestEmitInitialIntoPseudostate pins that an initial junction route leaves
// the body's named entry action and the model lowers clean.
func TestEmitInitialIntoPseudostate(t *testing.T) {
	m, err := emitFixture(t, "", `
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            <region xmi:type="uml:Region" xmi:id="xS2r1" name="R1">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xS2i" name="I"/>
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xS2j" name="J2" kind="junction"/>
              <subvertex xmi:type="uml:State" xmi:id="xS21" name="S2.1"/>
              <transition xmi:type="uml:Transition" xmi:id="xS2t" source="xS2i" target="xS2j"/>
              <transition xmi:type="uml:Transition" xmi:id="xS2u" source="xS2j" target="xS21"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"junction S2_J2;",
		"entry action 'S2.initial';",
		"transition 'S2.initial' then S2_J2;",
		"transition first S2_J2 then S2_S2_1;",
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("model lacks %q:\n%s", want, m.Text)
		}
	}
	if problems := Validate(m); len(problems) > 0 {
		t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
	}
}

// TestEmitInitialWithEffect pins that an initial transition's effect leaves
// the body's named entry action, in a single and in a parallel region.
func TestEmitInitialWithEffect(t *testing.T) {
	m, err := emitFixture(t, "", `
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            `+traceCall("entry", "xS2entry", "S2(entry)")+`
            <region xmi:type="uml:Region" xmi:id="xS2r1" name="R1">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xS2i" name="I"/>
              <subvertex xmi:type="uml:State" xmi:id="xS21" name="S2.1"/>
              <transition xmi:type="uml:Transition" xmi:id="xS2t" name="T2.1" source="xS2i" target="xS21">
                `+traceCall("effect", "xS2teffect", "T2.1(effect)")+`
              </transition>
            </region>
          </subvertex>
          <subvertex xmi:type="uml:State" xmi:id="xS3" name="S3">
            <region xmi:type="uml:Region" xmi:id="xS3r1" name="R1">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xS3i" name="I"/>
              <subvertex xmi:type="uml:State" xmi:id="xS31" name="S3.1"/>
              <transition xmi:type="uml:Transition" xmi:id="xS3t" name="T3.1" source="xS3i" target="xS31">
                `+traceCall("effect", "xS3teffect", "T3.1(effect)")+`
              </transition>
            </region>
            <region xmi:type="uml:Region" xmi:id="xS3r2" name="R2">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xS3j" name="I"/>
              <subvertex xmi:type="uml:State" xmi:id="xS32" name="S3.2"/>
              <transition xmi:type="uml:Transition" xmi:id="xS3u" source="xS3j" target="xS32"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>
          <transition xmi:type="uml:Transition" xmi:id="xT4" name="T4" source="xS2" target="xS3"/>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"entry action 'S2.initial' {",
		`transition 'S2.initial' do {`,
		`"T2.1(effect)"`,
		"then S2_S2_1;",
		"entry action 'S3/R1.initial';",
		`transition 'S3/R1.initial' do {`,
		`"T3.1(effect)"`,
		"then S3_S3_1;",
		"entry; then S3_S3_2;",
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("model lacks %q:\n%s", want, m.Text)
		}
	}
	if appends := strings.Count(m.Text, `"T2.1(effect)";`) + strings.Count(m.Text, `"T3.1(effect)";`); appends != 2 {
		t.Errorf("initial effects are appended %d times, want once each:\n%s", appends, m.Text)
	}
	if strings.Contains(m.Text, "state S2_I_start;") || strings.Contains(m.Text, "state S3_I_start;") {
		t.Errorf("model emits a helper state for an initial effect:\n%s", m.Text)
	}
	if problems := Validate(m); len(problems) > 0 {
		t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
	}
}

// TestEmitInitialEntryActionKeepsItsName pins that an effect-bearing initial
// transition uses the region's entry-action name without replacing its target.
func TestEmitInitialEntryActionKeepsItsName(t *testing.T) {
	m, err := emitFixture(t, "", `
          <subvertex xmi:type="uml:State" xmi:id="xS2" name="S2">
            <region xmi:type="uml:Region" xmi:id="xS2r1" name="R1">
              <subvertex xmi:type="uml:Pseudostate" xmi:id="xS2i" name="I"/>
              <subvertex xmi:type="uml:State" xmi:id="xS22" name="I_start_2"/>
              <subvertex xmi:type="uml:State" xmi:id="xS21" name="I_start"/>
              <transition xmi:type="uml:Transition" xmi:id="xS2t" name="T2.1" source="xS2i" target="xS21">
                `+traceCall("effect", "xS2teffect", "T2.1(effect)")+`
              </transition>
              <transition xmi:type="uml:Transition" xmi:id="xS2u" name="T2.2" source="xS21" target="xS22"/>
            </region>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xS2">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"entry action 'S2/R1.initial';",
		`transition 'S2/R1.initial' do {`,
		"then S2_I_start;",
		"state S2_I_start;",
		"state S2_I_start_2;",
		"transition first S2_I_start then S2_I_start_2;",
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("model lacks %q:\n%s", want, m.Text)
		}
	}
	for _, decl := range []string{"state S2_I_start;", "state S2_I_start_2;"} {
		if n := strings.Count(m.Text, decl); n != 1 {
			t.Errorf("%q is declared %d times, want once:\n%s", decl, n, m.Text)
		}
	}
	if strings.Contains(m.Text, "state S2_I_start_3;") {
		t.Errorf("model emits a helper state:\n%s", m.Text)
	}
	if problems := Validate(m); len(problems) > 0 {
		t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
	}
}

// factoryTarget gives the target class an Integer attribute and a constructor
// activity `Area001_Test$factory` that creates the instance, runs the given
// initialization on it and returns it: `t = new Area001_Test(); <init>; return t;`.
func factoryTarget(init string) string {
	return `<generalization xmi:type="uml:Generalization" xmi:id="tgtXGen" general="clsTarget"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="tgtXValue" name="value">
        <type href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="tgtXFactory" name="Area001_Test$factory">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="tgtXFactoryRet" direction="return" type="tgtX"/>
        <node xmi:type="uml:ActivityParameterNode" xmi:id="tgtXFactoryOut" name="Return" parameter="tgtXFactoryRet"/>
        <node xmi:type="uml:CreateObjectAction" xmi:id="fCreate" name="Create" classifier="tgtX">
          <result xmi:type="uml:OutputPin" xmi:id="fCreateOut"/>
        </node>
        <node xmi:type="uml:StartObjectBehaviorAction" xmi:id="fStart" name="Start">
          <object xmi:type="uml:InputPin" xmi:id="fStartObj"/>
        </node>
        <node xmi:type="uml:ForkNode" xmi:id="fFork" name="Fork(t)"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="fE1" source="fCreateOut" target="fFork"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="fE2" source="fFork" target="fStartObj"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="fE3" source="fFork" target="tgtXFactoryOut"/>
        ` + init + `
      </ownedBehavior>`
}

// factoryWrite is the statement `t.<feature> = <literal>` on the created instance.
func factoryWrite(feature, literalType, value string) string {
	return `<node xmi:type="uml:ValueSpecificationAction" xmi:id="fVal" name="Value">
          <result xmi:type="uml:OutputPin" xmi:id="fValOut"/>
          <value xmi:type="` + literalType + `" xmi:id="fLit" value="` + value + `"/>
        </node>
        <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="fWrite" name="Write" structuralFeature="` + feature + `" isReplaceAll="true">
          <object xmi:type="uml:InputPin" xmi:id="fWriteObj"/>
          <value xmi:type="uml:InputPin" xmi:id="fWriteVal"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="fE4" source="fFork" target="fWriteObj"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="fE5" source="fValOut" target="fWriteVal"/>`
}

func emitWithTarget(t *testing.T, target string) (*Model, error) {
	t.Helper()
	src := strings.Replace(machineSuite("", ""),
		`<generalization xmi:type="uml:Generalization" xmi:id="tgtXGen" general="clsTarget"/>`, target, 1)
	s := readFixture(t, src)
	noDiagnostics(t, s)
	if len(s.Tests) != 1 {
		t.Fatalf("tests = %d", len(s.Tests))
	}
	return Emit(s, s.Tests[0])
}

// TestEmitFactoryInitializesAttributes pins that what the target's constructor
// writes on the new instance becomes the attribute's initial value, and that a
// constructor doing anything else is refused rather than dropped.
func TestEmitFactoryInitializesAttributes(t *testing.T) {
	m, err := emitWithTarget(t, factoryTarget(factoryWrite("tgtXValue", "uml:LiteralInteger", "15")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Text, "attribute value : Integer = 15;") {
		t.Errorf("model lacks the factory's initial value:\n%s", m.Text)
	}
	if problems := Validate(m); len(problems) > 0 {
		t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
	}

	m, err = emitWithTarget(t, factoryTarget(""))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Text, "attribute value : Integer;") {
		t.Errorf("a constructor writing nothing left a value:\n%s", m.Text)
	}

	_, err = emitWithTarget(t, factoryTarget(factoryWrite("testerTestable", "uml:LiteralInteger", "15")))
	var te *TranslateError
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "writes testable, which is not an attribute") {
		t.Errorf("writing another class's feature: err = %v", err)
	}

	onSelf := strings.Replace(factoryWrite("tgtXValue", "uml:LiteralInteger", "15"),
		`<edge xmi:type="uml:ObjectFlow" xmi:id="fE4" source="fFork" target="fWriteObj"/>`,
		`<node xmi:type="uml:ReadSelfAction" xmi:id="fSelf"><result xmi:type="uml:OutputPin" xmi:id="fSelfOut"/></node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="fE4" source="fSelfOut" target="fWriteObj"/>`, 1)
	_, err = emitWithTarget(t, factoryTarget(onSelf))
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "is not a literal initialization of the new instance") {
		t.Errorf("writing something other than the instance: err = %v", err)
	}

	// A write whose object pin nothing feeds never fires (fUML), so it initializes nothing.
	unread := strings.Replace(factoryWrite("tgtXValue", "uml:LiteralInteger", "15"),
		`<edge xmi:type="uml:ObjectFlow" xmi:id="fE4" source="fFork" target="fWriteObj"/>`, "", 1)
	m, err = emitWithTarget(t, factoryTarget(unread))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Text, "attribute value : Integer;") {
		t.Errorf("a write nothing feeds left a value:\n%s", m.Text)
	}

	// A nested class's own `value` shares the name, not the identity.
	foreign := `<nestedClassifier xmi:type="uml:Class" xmi:id="tgtXOther" name="Other">
        <ownedAttribute xmi:type="uml:Property" xmi:id="otherValue" name="value">
          <type href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedAttribute>
      </nestedClassifier>
      ` + factoryWrite("otherValue", "uml:LiteralInteger", "15")
	_, err = emitWithTarget(t, factoryTarget(foreign))
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "writes value, which is not an attribute") {
		t.Errorf("writing a same-named feature of another class: err = %v", err)
	}

	// The factory creates a second instance, writes the first, then starts and returns the second.
	second := factoryWrite("tgtXValue", "uml:LiteralInteger", "15") + `
        <node xmi:type="uml:CreateObjectAction" xmi:id="fCreate2" name="Create2" classifier="tgtX">
          <result xmi:type="uml:OutputPin" xmi:id="fCreate2Out"/>
        </node>
        <node xmi:type="uml:ForkNode" xmi:id="fFork2" name="Fork(u)"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="fE6" source="fCreate2Out" target="fFork2"/>`
	second = strings.NewReplacer(
		`<edge xmi:type="uml:ObjectFlow" xmi:id="fE2" source="fFork" target="fStartObj"/>`,
		`<edge xmi:type="uml:ObjectFlow" xmi:id="fE2" source="fFork2" target="fStartObj"/>`,
		`<edge xmi:type="uml:ObjectFlow" xmi:id="fE3" source="fFork" target="tgtXFactoryOut"/>`,
		`<edge xmi:type="uml:ObjectFlow" xmi:id="fE3" source="fFork2" target="tgtXFactoryOut"/>`,
	).Replace(factoryTarget(second))
	_, err = emitWithTarget(t, second)
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "returns something other than the new instance") {
		t.Errorf("returning a second instance after writing the first: err = %v", err)
	}

	// The factory returns an instance of another class.
	other := strings.Replace(factoryTarget(factoryWrite("tgtXValue", "uml:LiteralInteger", "15")),
		`<edge xmi:type="uml:ObjectFlow" xmi:id="fE3" source="fFork" target="tgtXFactoryOut"/>`,
		`<node xmi:type="uml:CreateObjectAction" xmi:id="fCreate2" name="Create2" classifier="semX">
          <result xmi:type="uml:OutputPin" xmi:id="fCreate2Out"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="fE3" source="fCreate2Out" target="tgtXFactoryOut"/>`, 1)
	_, err = emitWithTarget(t, other)
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "returns something other than the new instance") {
		t.Errorf("returning another instance: err = %v", err)
	}

	unreturned := strings.Replace(factoryTarget(factoryWrite("tgtXValue", "uml:LiteralInteger", "15")),
		`<edge xmi:type="uml:ObjectFlow" xmi:id="fE3" source="fFork" target="tgtXFactoryOut"/>`, "", 1)
	_, err = emitWithTarget(t, unreturned)
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "does not return the new instance") {
		t.Errorf("returning nothing: err = %v", err)
	}

	// The factory creates, writes and returns a nested class that shares the target's name.
	twin := `<nestedClassifier xmi:type="uml:Class" xmi:id="tgtXTwin" name="Area001_Test">
        <ownedAttribute xmi:type="uml:Property" xmi:id="twinValue" name="value">
          <type href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
        </ownedAttribute>
      </nestedClassifier>
      ` + strings.Replace(factoryTarget(factoryWrite("twinValue", "uml:LiteralInteger", "15")),
		`classifier="tgtX"`, `classifier="tgtXTwin"`, 1)
	_, err = emitWithTarget(t, twin)
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "the new instance") {
		t.Errorf("creating a same-named other class: err = %v", err)
	}

	// The factory is an opaque behavior, whose body the reader does not follow.
	opaque := `<generalization xmi:type="uml:Generalization" xmi:id="tgtXGen" general="clsTarget"/>
      <ownedAttribute xmi:type="uml:Property" xmi:id="tgtXValue" name="value">
        <type href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#Integer"/>
      </ownedAttribute>
      <ownedBehavior xmi:type="uml:OpaqueBehavior" xmi:id="tgtXFactory" name="Area001_Test$factory">
        <language>Alf</language>
        <body>this.value = 15;</body>
      </ownedBehavior>`
	_, err = emitWithTarget(t, opaque)
	if !errors.As(err, &te) || !strings.Contains(err.Error(), "is not an activity") {
		t.Errorf("an opaque factory: err = %v", err)
	}
}

// markOperation is a target operation `mark(s : String)` whose method traces
// the given segment: a literal, or the parameter itself when segment is "s".
func markOperation(id, segment string) string {
	source := id + `ValOut`
	value := `<node xmi:type="uml:ValueSpecificationAction" xmi:id="` + id + `Val">
          <result xmi:type="uml:OutputPin" xmi:id="` + id + `ValOut"/>
          <value xmi:type="uml:LiteralString" xmi:id="` + id + `Lit" value="` + segment + `"/>
        </node>`
	if segment == "s" {
		source = id + `ParamNode`
		value = `<node xmi:type="uml:ActivityParameterNode" xmi:id="` + id + `ParamNode" parameter="` + id + `MethodP"/>`
	}
	return `<ownedOperation xmi:type="uml:Operation" xmi:id="` + id + `" name="mark" method="` + id + `Method">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="` + id + `P" name="s">
          <type href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#String"/>
        </ownedParameter>
      </ownedOperation>
      <ownedBehavior xmi:type="uml:Activity" xmi:id="` + id + `Method" name="mark$method" specification="` + id + `">
        <ownedParameter xmi:type="uml:Parameter" xmi:id="` + id + `MethodP" name="s">
          <type href="http://www.omg.org/spec/UML/20131001/PrimitiveTypes.xmi#String"/>
        </ownedParameter>
        <node xmi:type="uml:ReadSelfAction" xmi:id="` + id + `Self"><result xmi:type="uml:OutputPin" xmi:id="` + id + `SelfOut"/></node>
        ` + value + `
        <node xmi:type="uml:CallOperationAction" xmi:id="` + id + `Trace" operation="opTrace">
          <target xmi:type="uml:InputPin" xmi:id="` + id + `TraceTarget"/>
          <argument xmi:type="uml:InputPin" xmi:id="` + id + `TraceArg"/>
        </node>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + id + `E1" source="` + id + `SelfOut" target="` + id + `TraceTarget"/>
        <edge xmi:type="uml:ObjectFlow" xmi:id="` + id + `E2" source="` + source + `" target="` + id + `TraceArg"/>
      </ownedBehavior>`
}

// TestEmitCallSelectsOperationByIdentity pins that a call inlines the method
// of the operation it names, not the first operation sharing its name and arity.
func TestEmitCallSelectsOperationByIdentity(t *testing.T) {
	target := `<generalization xmi:type="uml:Generalization" xmi:id="tgtXGen" general="clsTarget"/>
      ` + markOperation("opMarkFirst", "first") + `
      ` + markOperation("opMarkSecond", "s")
	entry := strings.Replace(traceCall("entry", "xS1entry", "second"), `operation="opTrace"`, `operation="opMarkSecond"`, 1)
	src := strings.NewReplacer(
		`<generalization xmi:type="uml:Generalization" xmi:id="tgtXGen" general="clsTarget"/>`, target,
		traceCall("entry", "xS1entry", "S1(entry)"), entry,
	).Replace(machineSuite("", ""))
	s := readFixture(t, src)
	noDiagnostics(t, s)
	if len(s.Tests) != 1 {
		t.Fatalf("tests = %d", len(s.Tests))
	}
	m, err := Emit(s, s.Tests[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.Text, `"second"`) || strings.Contains(m.Text, `"first"`) {
		t.Errorf("the call inlined the wrong overload:\n%s", m.Text)
	}
	if problems := Validate(m); len(problems) > 0 {
		t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
	}
}

// TestEmitRejects pins the typed error for constructs with no translation.
func TestEmitRejects(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"internal transition", `
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" kind="internal" source="xS1" target="xS1">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`, "no spelling"},
		{"guard with a side effect", guardWithSideEffect, "acts on the model"},
		{"guard writing inside a conditional", guardBehavior(`
            <node xmi:type="uml:ConditionalNode" xmi:id="xT3if" name="1:IfStatement">
              <node xmi:type="uml:AddStructuralFeatureValueAction" xmi:id="xT3write" structuralFeature="attrCounter"/>
            </node>
            <edge xmi:type="uml:ControlFlow" xmi:id="xT3e5" source="xT3if" target="xT3ret1"/>`), "acts on the model"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := emitFixture(t, "", tc.body)
			var te *TranslateError
			if !errors.As(err, &te) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want TranslateError containing %q", err, tc.want)
			}
		})
	}
}

// TestEmitSharesABufferAmongTriggersDeferringOneSignal keeps a signal two
// triggers of a state defer in one buffer with one accept loop: a loop per
// trigger would keep each occurrence twice and declare its actions twice.
func TestEmitSharesABufferAmongTriggersDeferringOneSignal(t *testing.T) {
	m, err := emitFixture(t, "", `
          <subvertex xmi:type="uml:State" xmi:id="xD" name="D">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="xDt1" event="evData"/>
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="xDt2" event="evData"/>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xD">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`)
	if err != nil {
		t.Fatal(err)
	}
	for want, n := range map[string]int{
		"attribute deferred : IntegerData[*] ordered;": 1,
		"action receive ":           1,
		"action keep ":              1,
		"accept kept : IntegerData": 1,
	} {
		if got := strings.Count(m.Text, want); got != n {
			t.Errorf("model has %d of %q, want %d:\n%s", got, want, n, m.Text)
		}
	}
	if problems := Validate(m); len(problems) > 0 {
		t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
	}
}

// A kept signal named as the encoding names one of its own members — `kept`,
// the payload an accept loop binds, or `buffer`, the do action keeping it — is
// not hidden by that member: the made-up name steps aside, so the encoding's
// references to the signal still name it.
func TestEmitKeptSignalNamedAsAnEncodingMember(t *testing.T) {
	for signal, wants := range map[string][]string{
		"kept": {
			"attribute deferred : kept[*] ordered;",
			"action receive accept kept_2 : kept;",
			"assign deferred := SequenceFunctions::including(deferred, receive.kept_2);",
			"for kept_2 in deferred { send kept_2 to self; }",
		},
		"buffer": {
			"attribute deferred : buffer[*] ordered;",
			"do action buffer_2 {",
			"action receive accept kept : buffer;",
		},
	} {
		t.Run(signal, func(t *testing.T) {
			src := machineSuite("", `
          <subvertex xmi:type="uml:State" xmi:id="xD" name="D">
            <deferrableTrigger xmi:type="uml:Trigger" xmi:id="xDt" event="evNamed"/>
          </subvertex>
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" source="xS1" target="xD">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`)
			src = strings.Replace(src, fixtureEvents, fixtureEvents+`  <packagedElement xmi:type="uml:SignalEvent" xmi:id="evNamed" name="NamedEvent" signal="sigNamed"/>
`, 1)
			src = strings.Replace(src, `<packagedElement xmi:type="uml:Signal" xmi:id="sigStart" name="Start"/>`,
				`<packagedElement xmi:type="uml:Signal" xmi:id="sigStart" name="Start"/>
    <packagedElement xmi:type="uml:Signal" xmi:id="sigNamed" name="`+signal+`"/>`, 1)
			s := readFixture(t, src)
			noDiagnostics(t, s)
			if len(s.Tests) != 1 {
				t.Fatalf("tests = %d", len(s.Tests))
			}
			m, err := Emit(s, s.Tests[0])
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range wants {
				if !strings.Contains(m.Text, want) {
					t.Errorf("model lacks %q:\n%s", want, m.Text)
				}
			}
			if problems := Validate(m); len(problems) > 0 {
				t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
			}
		})
	}
}
