package behavior_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes/behavior"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// wantOnlyAccepterRule fails unless every tier together reports the accepter
// rule once, at the accepter, and nothing else about the model.
func wantOnlyAccepterRule(t *testing.T, src, accepter string) {
	t.Helper()
	got := analyzeTransitions(t, src)
	if len(got) != 1 {
		t.Fatalf("got %+v, want exactly one diagnostic", got)
	}
	d := got[0]
	if d.Severity != diag.SeverityError || d.Code != behavior.CodeAccepterSourceNotState ||
		d.Message != behavior.MsgAccepterSourceNotState {
		t.Fatalf("got %+v, want the accepter rule", d)
	}
	if at := src[d.Span.Offset : d.Span.Offset+d.Span.Len]; at != accepter {
		t.Fatalf("reported at %q, want the accepter %q", at, accepter)
	}
}

// validateTransitionUsageTriggerActions (SysML v2.1 §8.3.18.9): a source that is
// an ActionUsage of any kind but a StateUsage admits no accepter.
func TestAccepterSourceIsAnyActionThatIsNoState(t *testing.T) {
	for name, source := range map[string]string{
		"entry action":      `entry action a;`,
		"do action":         `do action a;`,
		"exit action":       `exit action a;`,
		"action usage":      `action a;`,
		"reference action":  `ref action a : Act;`,
		"performed action":  `perform action a : Act;`,
		"accept action":     `action a accept Sig;`,
		"send action":       `action a send new Sig() to self;`,
		"terminate action":  `action a terminate;`,
		"calculation":       `calc a { 1 }`,
		"use case":          `use case a;`,
		"analysis case":     `analysis a;`,
		"verification case": `verification a;`,
	} {
		t.Run(name, func(t *testing.T) {
			wantOnlyAccepterRule(t, `package test {
	item def Sig;
	action def Act;
	state def M {
		entry; then idle;
		state idle;
		`+source+`
		transition first a accept Sig then busy;
		state busy;
	}
}`, "accept Sig")
		})
	}
}

// The source is judged as resolved: inherited, reached through a chain or a
// qualified name, in a state usage's body, and with a guard and an effect beside the accepter.
func TestAccepterSourceIsJudgedWhereverItResolves(t *testing.T) {
	for name, src := range map[string]string{
		"inherited": `package test {
	item def Sig;
	state def Base { action a; }
	state def M :> Base {
		entry; then idle;
		state idle;
		transition first a accept Sig then idle;
	}
}`,
		"inherited entry action": `package test {
	item def Sig;
	state def Base { entry action a; state idle; }
	state def M :> Base {
		transition first a accept Sig then idle;
	}
}`,
		"feature chain": `package test {
	item def Sig;
	state def M {
		entry; then idle;
		state idle { action a; }
		transition first idle.a accept Sig then idle;
	}
}`,
		"qualified name": `package test {
	item def Sig;
	state def M {
		entry; then idle;
		state idle { do action a; }
		transition first idle::a accept Sig then idle;
	}
}`,
		"nested entry action": `package test {
	item def Sig;
	state def M {
		entry; then idle;
		state idle { entry action a; }
		transition first idle.a accept Sig then idle;
	}
}`,
		"state usage with guard and effect": `package test {
	item def Sig;
	part def P {
		state m {
			entry; then idle;
			state idle;
			action a;
			transition t first a accept Sig if true do action e { } then idle;
		}
	}
}`,
		"composite state body": `package test {
	item def Sig;
	state def M {
		entry; then outer;
		state outer {
			entry; then idle;
			state idle;
			action a;
			transition first a accept Sig then idle;
		}
	}
}`,
	} {
		t.Run(name, func(t *testing.T) {
			wantOnlyAccepterRule(t, src, "accept Sig")
		})
	}
}

// A transition written without a source leaves the member before it (SysML v2.1
// §7.18.3), so an accepter after an action that is no state breaks the same rule.
func TestAccepterAfterAnActionThatIsNoState(t *testing.T) {
	for name, source := range map[string]string{
		"action usage":     `action a;`,
		"performed action": `perform action a : Act;`,
		"calculation":      `calc a { 1 }`,
	} {
		t.Run(name, func(t *testing.T) {
			wantOnlyAccepterRule(t, `package test {
	item def Sig;
	action def Act;
	state def M {
		entry; then idle;
		state idle;
		`+source+`
		accept Sig then busy;
		state busy;
	}
}`, "accept Sig")
		})
	}
}

// A state usage of any spelling is a source an accepter may wait in.
func TestAccepterSourceStateUsageIsLegal(t *testing.T) {
	for name, source := range map[string]string{
		"state":           `state a;`,
		"composite state": `state a { entry action init; do action work; }`,
		"exhibited state": `exhibit state a : St;`,
		"reference state": `ref state a : St;`,
	} {
		t.Run(name, func(t *testing.T) {
			if got := analyzeTransitions(t, `package test {
	item def Sig;
	state def St;
	state def M {
		entry; then idle;
		state idle;
		`+source+`
		accept Sig then idle;
		transition first a accept Sig then idle;
	}
}`); len(got) != 0 {
				t.Fatalf("got %+v, want no diagnostics", got)
			}
		})
	}
}

// A source that is no action usage at all has no TransitionUsage::source for the
// accepter rule to read, so it stays the endpoint rule's, reported once.
func TestAccepterSourceThatIsNoActionKeepsTheEndpointRule(t *testing.T) {
	for name, source := range map[string]string{
		"part":      `part a;`,
		"attribute": `attribute a = 1;`,
		"item":      `item a;`,
	} {
		t.Run(name, func(t *testing.T) {
			got := analyzeTransitions(t, `package test {
	item def Sig;
	state def M {
		entry; then idle;
		state idle;
		`+source+`
		transition first a accept Sig then idle;
	}
}`)
			if len(got) != 1 || got[0].Code != resolve.CodeNotAVertex {
				t.Fatalf("got %+v, want one %s", got, resolve.CodeNotAVertex)
			}
		})
	}
}

// A transition without an accepter is not this rule's, whatever it leaves.
func TestTransitionWithoutAccepterIsNotTheAccepterRules(t *testing.T) {
	got := analyzeTransitions(t, `package test {
	state def M {
		entry; then idle;
		state idle;
		action a;
		transition first a if true then idle;
	}
}`)
	for _, d := range got {
		if d.Code == behavior.CodeAccepterSourceNotState {
			t.Fatalf("got %+v, want no accepter diagnostic", d)
		}
	}
}

// An action body's `transition` is the same TransitionUsage, so its accepter
// needs a state as its source there too, at any depth.
func TestAccepterSourceInAnActionBody(t *testing.T) {
	for name, src := range map[string]string{
		"action definition": `package test {
	item def Sig;
	action def X {
		action a;
		action b;
		transition first a accept Sig then b;
	}
}`,
		"action usage": `package test {
	item def Sig;
	part def P {
		action x {
			action a;
			action b;
			transition first a accept Sig then b;
		}
	}
}`,
		"do action of a state": `package test {
	item def Sig;
	state def M {
		entry; then idle;
		state idle {
			do action work {
				action a;
				action b;
				transition first a accept Sig then b;
			}
		}
	}
}`,
		"state usage in an action of a state": `package test {
	item def Sig;
	state def M {
		entry; then idle;
		state idle {
			do action work {
				state inner {
					entry; then ready;
					state ready;
					action a;
					transition first a accept Sig then ready;
				}
			}
		}
	}
}`,
	} {
		t.Run(name, func(t *testing.T) {
			wantOnlyAccepterRule(t, src, "accept Sig")
		})
	}
	if got := analyzeTransitions(t, `package test {
	action def X {
		action a;
		action b;
		transition first a if true then b;
	}
}`); len(got) != 0 {
		t.Fatalf("got %+v, want a guarded transition between actions clean", got)
	}
}

// The rule reads no vertex of the machine, so it holds in a machine whose
// vertices do not collect, which the endpoint checks leave to lowering.
func TestAccepterSourceInAMachineLoweringRejects(t *testing.T) {
	wantOnlyAccepterRule(t, `package test {
	item def Sig;
	state def Base {
		action prepare;
	}
	state def M {
		entry; then idle;
		state idle : Base;
		transition first idle.prepare accept Sig then idle;
	}
}`, "accept Sig")
}

// Where no library declares the metaclasses, as under a partial library, an
// action usage is still known as one by its own declaration.
func TestAccepterSourceWithoutTheMetaclassLibrary(t *testing.T) {
	const src = `package test {
	item def Sig;
	state def M {
		entry action init;
		transition init accept Sig then idle;
		state idle;
		action work;
		transition first work accept Sig then idle;
		transition first idle accept Sig then idle;
	}
}`
	p := parser.New(source.New("t.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("t.sysml", root)
	got := behavior.StateTransitionPass{}.Run(passes.NewContext("t.sysml", idx, nil), "t.sysml", root)
	if len(got) != 2 {
		t.Fatalf("got %+v, want the two action sources reported", got)
	}
	for _, d := range got {
		if d.Code != behavior.CodeAccepterSourceNotState {
			t.Fatalf("got %+v, want the accepter rule", d)
		}
	}
}
