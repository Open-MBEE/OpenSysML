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

// TestEmitInitialIntoPseudostate pins the rewrite of an initial transition
// into a junction: the region starts in a helper state whose completion
// transition reaches the junction, and the model lowers clean.
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
		"state S2_I_start;",
		"transition first S2_I_start then S2_J2;",
		"entry; then S2_I_start;",
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

// TestEmitInitialWithEffect pins that an initial transition's effect rides a
// helper state's completion transition, in a single and in a parallel region.
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
		"state S2_I_start;",
		`transition first S2_I_start do {`,
		`"T2.1(effect)"`,
		"then S2_S2_1;",
		"transition 'S2.initial' then S2_I_start;",
		"state S3_I_start;",
		`transition first S3_I_start do {`,
		`"T3.1(effect)"`,
		"then S3_S3_1;",
		"entry; then S3_I_start;",
		"entry; then S3_S3_2;",
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("model lacks %q:\n%s", want, m.Text)
		}
	}
	if appends := strings.Count(m.Text, `"T2.1(effect)";`) + strings.Count(m.Text, `"T3.1(effect)";`); appends != 2 {
		t.Errorf("initial effects are appended %d times, want once each:\n%s", appends, m.Text)
	}
	if strings.Contains(m.Text, "entry action 'S3/R1.initial'") {
		t.Errorf("model folds an initial effect into a region's entry action:\n%s", m.Text)
	}
	if problems := Validate(m); len(problems) > 0 {
		t.Errorf("%s\n%s", strings.Join(problems, "\n"), m.Text)
	}
}

// TestEmitInitialHelperNameIsUnique pins that the helper state an initial
// transition starts in shares the vertex name registry, and that the registry
// reserves final names: a state spelled like a suffixed name keeps it, and
// the next collision probes past it.
func TestEmitInitialHelperNameIsUnique(t *testing.T) {
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
		"state S2_I_start;",
		"state S2_I_start_2;",
		"state S2_I_start_3;",
		"then S2_I_start_3;",
		"transition first S2_I_start_3 then S2_I_start_2;",
		"entry; then S2_I_start;",
	} {
		if !strings.Contains(m.Text, want) {
			t.Errorf("model lacks %q:\n%s", want, m.Text)
		}
	}
	for _, decl := range []string{"state S2_I_start;", "state S2_I_start_2;", "state S2_I_start_3;"} {
		if n := strings.Count(m.Text, decl); n != 1 {
			t.Errorf("%q is declared %d times, want once:\n%s", decl, n, m.Text)
		}
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
