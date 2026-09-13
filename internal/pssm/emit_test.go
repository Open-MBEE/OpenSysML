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

// TestEmitRejects pins the typed error for constructs with no translation.
func TestEmitRejects(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"internal transition", `
          <transition xmi:type="uml:Transition" xmi:id="xT3" name="T3" kind="internal" source="xS1" target="xS1">
            <trigger xmi:type="uml:Trigger" xmi:id="xT3trig" event="evContinue"/>
          </transition>`, "no spelling"},
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
