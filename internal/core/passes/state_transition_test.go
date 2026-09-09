package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
)

// transitionDiags runs the pass alone, so what it reports is not mixed with what
// another tier reports about the same model.
func transitionDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	sf := source.New("t.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	idx := newTestIndexFromDoc("t.sysml", root)
	return StateTransitionPass{}.Run(NewContext("t.sysml", idx, nil), "t.sysml", root)
}

// analyzeTransitions runs every tier, for a verdict another tier reports.
func analyzeTransitions(t *testing.T, src string) []Diagnostic {
	t.Helper()
	sf := source.New("t.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	var out []Diagnostic
	for _, d := range Analyze("t.sysml", root, nil, newTestIndexFromDoc("t.sysml", root)) {
		// The models here are written in our own state notation, which
		// NonstandardNotationPass warns about; the verdict under test is another
		// tier's.
		if d.Code == CodeNonstandardNotation {
			continue
		}
		out = append(out, d)
	}
	return out
}

// wantClean fails when the pass reports anything about a legal model, which is
// the failure mode that breaks models a modeller wrote correctly.
func wantClean(t *testing.T, src string) {
	t.Helper()
	if got := transitionDiags(t, src); len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// wantOneError fails unless the pass reports exactly the expected diagnostic.
func wantOneError(t *testing.T, src, code, messagePart string) Diagnostic {
	t.Helper()
	got := transitionDiags(t, src)
	if len(got) != 1 {
		t.Fatalf("got %+v, want exactly one diagnostic", got)
	}
	d := got[0]
	if d.Severity != SeverityError || d.Source != "state-transition" || d.Code != code {
		t.Fatalf("got %+v, want severity=error source=state-transition code=%s", d, code)
	}
	if !strings.Contains(d.Message, messagePart) {
		t.Fatalf("got message %q, want it to contain %q", d.Message, messagePart)
	}
	return d
}

// A vertex of a sibling orthogonal region belongs to the same state machine, so
// a transition crossing regions is legal (UML 2.5.1 §14.2.3.9).
func TestTransitionTargetInSiblingRegionIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		state a parallel {
			state r1 {
				entry; then i1;
				state i1;
				state x;
				succession first i1 then x;
				transition first x then y;
			}
			state r2 {
				entry; then i2;
				state i2;
				state y;
				succession first i2 then y;
			}
		}
	}
}`)
}

// The same crossing written with the `region` keyword, the spelling the runtime
// conformance cases use, is legal too.
func TestTransitionTargetInSiblingRegionKeywordIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		attribute crossed;
		entry; then start;
		state start;
		state running parallel {
			state left {
				entry; then lstart;
				state lstart;
				state lidle;

				succession first lstart then lidle;
				transition first lidle then rtarget;
			}
			state right {
				entry; then rstart;
				state rstart;
				state ridle;
				state rtarget;

				succession first rstart then ridle;
			}
		}

		succession first start then running;
	}
}`)
}

// A vertex of another state machine is not a vertex of this one, so naming it is
// illegal however well the name resolves.
func TestTransitionTargetInUnrelatedMachineIsIllegal(t *testing.T) {
	wantOneError(t, `package test {
	state def Other { entry; then s; state s; state running; succession first s then running; }
	state def M {
		entry; then i;
		state i;
		state busy;
		succession first i then busy;
		transition first busy then Other::running;
	}
}`, CodeEndpointNotOfMachine, "Other::running")
}

// The same endpoint written as a succession is the same violation.
func TestSuccessionTargetInUnrelatedMachineIsIllegal(t *testing.T) {
	wantOneError(t, `package test {
	state def Other { entry; then s; state s; state running; succession first s then running; }
	state def M {
		entry; then i;
		state i;
		state busy;
		succession first i then busy;
		succession first busy then Other::running;
	}
}`, CodeEndpointNotOfMachine, "Other::running")
}

// An entry point is a pseudostate the composite state owns, so a transition to
// it is legal (UML 2.5.1 §14.2.3.8).
func TestTransitionToEntryPointIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		entry; then i;
		state i;
		state comp {
			entry point ep;
			entry; then ci;
			state ci;
			state cs;
			succession first ci then cs;
		}
		succession first i then comp;
		transition first comp then comp::ep;
	}
}`)
}

// An attribute is no vertex, so a transition reaching one has no target vertex.
// Endpoint resolution reports this one, so the pass must not report it twice.
func TestTransitionTargetResolvingToNonVertexIsIllegal(t *testing.T) {
	src := `package test {
	state def M {
		attribute count;
		entry; then i;
		state i;
		state busy;
		succession first i then busy;
		transition first busy then count;
	}
}`
	got := analyzeTransitions(t, src)
	if len(got) != 1 {
		t.Fatalf("got %+v, want exactly one diagnostic", got)
	}
	if got[0].Severity != SeverityError || got[0].Code != "not-a-vertex" {
		t.Fatalf("got %+v, want an error coded not-a-vertex", got[0])
	}
	if len(transitionDiags(t, src)) != 0 {
		t.Fatalf("the pass reported an endpoint name resolution already reported")
	}
}

// A sourceless `accept … then` leaves the state declared before it in the same
// body (SysML v2 §7.18.3), so it has a source, and reporting it would break a
// legal model.
func TestSourcelessAcceptTransitionIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		entry; then i;
		state i;
		state busy;
		state done;
		succession first i then busy;
		state idle;
		accept go then done;
	}
}`)
}

// Several sourceless transitions after one state all leave it, and the succession
// `then state wait;` declares is looked past, as is a preceding sourceless
// transition; the shorthand is legal in a composite state's body too.
func TestSourcelessTransitionChainAndSuccessionAreLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		entry; then start;
		state start;
		then state normal;
		accept go then maintenance;
		if ready then degraded;
		state maintenance;
		accept after 5 then normal;
		state degraded {
			entry; then low;
			state low;
			accept go then high;
			state high;
		}
	}
}`)
}

// The shorthand's source is the member before it (SysML v2 §7.18.3), so as the
// first member of its body it has none, and the check reports it where the
// endpoint diagnostics report.
func TestSourcelessTransitionWithNothingBeforeIsReported(t *testing.T) {
	d := wantOneError(t, `package test {
	state def M {
		accept go then active;
		entry; then init;
		state init;
		state active;
	}
}`, CodeNoTransitionSource, "has no member before it to leave")
	if d.Message != lower.NoTransitionSourceMessage {
		t.Fatalf("got message %q, want %q", d.Message, lower.NoTransitionSourceMessage)
	}
}

// A guarded shorthand right after the body's entry action is the guarded entry
// transition of SysML v2 §7.18.3 (`EntryTransitionMember`), which the pilot
// accepts, an unguarded default among them included.
func TestGuardedEntryTransitionIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		in attribute isInitOff : Boolean;
		entry;
		if isInitOff then off;
		if not isInitOff then on;
		state off;
		state on;
	}
}`)
	wantClean(t, `package test {
	state def M {
		in attribute isInitOff : Boolean;
		entry; then off;
		if isInitOff then on;
		state off;
		state on;
	}
}`)
	wantClean(t, `package test {
	state def M {
		in attribute isInitOff : Boolean;
		entry action boot { }
		if isInitOff then off;
		then on;
		state off;
		state on;
	}
}`)
}

// An entry transition chooses the starting state by its guard alone (SysML v2
// §7.18.3): a trigger or an effect on it, or a target that is no state, is
// reported where the endpoint diagnostics report.
func TestEntryTransitionShapeIsReported(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		entry action boot { }
		accept go then active;
		state active;
	}
}`, CodeEntryTransitionShape, "carries a trigger")
	wantOneError(t, `package test {
	state def M {
		entry; then init;
		accept go then active;
		state init;
		state active;
	}
}`, CodeEntryTransitionShape, "carries a trigger")
	wantOneError(t, `package test {
	state def M {
		entry;
		if true do action mark then active;
		state active;
	}
}`, CodeEntryTransitionShape, "carries an effect")
	wantOneError(t, `package test {
	state def M {
		entry;
		if true then pick;
		choice pick;
		transition first pick then active;
		state active;
	}
}`, CodeEntryTransitionTarget, "reaches the choice pick")
}

// The member before the shorthand is an entry action or an attribute rather than
// a vertex, which the check names in modelling terms.
func TestSourcelessTransitionAfterANonVertexIsReported(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		entry; then init;
		state init;
		attribute count : Integer = 0;
		accept go then active;
		state active;
	}
}`, CodeTransitionSourceNotVertex, "leaves the attribute usage count, the member declared before it")
	wantOneError(t, `package test {
	state def M {
		entry; then init;
		state init;
		first init then active;
		accept go then done;
		state active;
	}
}`, CodeTransitionSourceNotVertex, "leaves the succession from init, the member declared before it, which is not a state")
	// The pilot's grammar chains the shorthand straight off the usage it leaves: a
	// parameter, a written succession or documentation between them is what it leaves.
	wantOneError(t, `package test {
	part def V;
	state def M {
		entry; then init;
		state init;
		in v : V;
		accept go then active;
		state active;
	}
}`, CodeTransitionSourceNotVertex, "leaves the in parameter v, the member declared before it")
	wantOneError(t, `package test {
	state def M {
		entry; then init;
		state init;
		succession first init then active;
		accept go then done;
		state active;
	}
}`, CodeTransitionSourceNotVertex, "leaves an unnamed succession usage, the member declared before it")
	// A pseudostate is a vertex, but not the state usage the rule names; only a
	// transition naming it as `first` leaves it, whatever the shorthand carries.
	wantOneError(t, `package test {
	state def M {
		entry; then init;
		state init;
		transition first init then pick;
		choice pick;
		accept go then active;
		transition first pick then active;
		state active;
	}
}`, CodeTransitionSourceNotVertex,
		"leaves the choice pick, the member declared before it, which is a pseudostate rather than a state (SysML v2 7.18.3): write `transition first pick … then …;` to leave it")
	wantOneError(t, `package test {
	state def M {
		attribute ready : Boolean = true;
		entry; then init;
		state init;
		transition first init then sync;
		join sync;
		if ready then active;
		transition first sync then active;
		state active;
	}
}`, CodeTransitionSourceNotVertex, "leaves the join sync, the member declared before it, which is a pseudostate rather than a state")
	wantOneError(t, `package test {
	state def M {
		entry; then init;
		state init;
		doc /* Serviced, then back to normal. */
		accept go then active;
		state active;
	}
}`, CodeTransitionSourceNotVertex, "leaves the documentation, the member declared before it")
}

// A region of a parallel state before the shorthand is not a state the machine
// can leave, and the diagnostic says which kind of member it is.
func TestSourcelessTransitionAfterARegionIsReported(t *testing.T) {
	wantOneError(t, `package test {
	attribute def Go;
	state def M {
		entry; then p;
		state p parallel {
			state r1 {
				entry; then a;
				state a;
			}
			accept Go then r2.b;
			state r2 {
				entry; then b;
				state b;
			}
		}
	}
}`, CodeTransitionSourceNotVertex,
		"leaves the state usage r1, the member declared before it, which is an orthogonal region of a parallel state")
}

// A junction no transition leaves routes a transition reaching it nowhere, which
// no cycle check finds, since the chain reaching it is acyclic.
func TestJunctionChainTerminatingNowhereIsIllegal(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		entry; then i;
		state i;
		state busy;
		junction j;
		succession first i then busy;
		transition first busy then j;
	}
}`, CodeNoOutgoingTransition, "junction j has no outgoing transition")
}

// A junction a transition does leave routes onward, so it is legal — the check
// above must rest on the missing transition, not on the junction itself.
func TestJunctionWithOutgoingTransitionIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		entry; then i;
		state i;
		state busy;
		state done;
		junction j;
		succession first i then busy;
		transition first busy then j;
		transition first j then done;
	}
}`)
}

// Sibling regions may declare same-named junctions, so a dead end is reported by
// the declaration it is: one region's `pick then …` says nothing about the other's.
func TestDeadEndJunctionIsReportedBesideASameNamedOneThatRoutes(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		state both parallel {
			state left {
				state lidle;
				state ldone;
				junction pick;
				succession first lidle then pick;
				succession first pick then ldone;
			}
			state right {
				state ridle;
				junction pick;
				succession first ridle then pick;
			}
		}
	}
}`, CodeNoOutgoingTransition, "junction pick has no outgoing transition")
}

// A succession is an outgoing transition too, so a junction one leaves is legal.
func TestJunctionLeftBySuccessionIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		entry; then i;
		state i;
		state busy;
		state done;
		junction j;
		succession first i then busy;
		transition first busy then j;
		succession first j then done;
	}
}`)
}

// Handed over from #205: a named `first`/`then` marker is not a vertex, and UML
// 2.5.1 §15.7.18 gives the initial pseudostate it stands for no incoming
// transition — so this reports at check time rather than at executor construction.
func TestTransitionToFirstMarkerIsIllegal(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		entry; then i;
		state i;
		state busy;
		state other;
		first marker then other;
		succession first i then busy;
		transition first busy then marker;
	}
}`, CodeEndpointNotOfMachine, "marker")
}

// A final state is a vertex, so a transition to one is legal.
func TestTransitionToFinalStateIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		entry; then i;
		state i;
		state busy;
		succession first i then busy;
		transition first busy then done;
	}
}`)
}

// A machine written as a usage is checked like one written as a definition.
func TestStateUsageMachineIsChecked(t *testing.T) {
	wantOneError(t, `package test {
	state def Other { entry; then s; state s; state running; succession first s then running; }
	state machine {
		entry; then i;
		state i;
		state busy;
		succession first i then busy;
		transition first busy then Other::running;
	}
}`, CodeEndpointNotOfMachine, "Other::running")
}

// The machine's entry action stands in for a start pseudostate, so a transition
// naming it as a source is legal (an ordinary action is rejected by name
// resolution, see resolve.TestResolveEndpointOrdinaryActionIsNotAVertex).
func TestTransitionOutOfEntryActionIsLegal(t *testing.T) {
	wantClean(t, `package test {
	state def M {
		entry action start { }
		transition start then busy;
		state busy;
		state done;
		transition first busy then done;
	}
}`)
}

// An entry action stands in for a start pseudostate in the completion shape,
// guarded or not: a triggered transition is an edge between two vertices, and
// an entry action is none. A trigger names the accepter rule, which is the
// specific reading of the same rejection.
func TestTriggeredTransitionOutOfEntryActionIsNotAVertex(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		entry action begin { }
		transition begin accept Warning then busy;
		state busy;
	}
}`, CodeAccepterSourceNotState, "must have a state as its source")
	wantClean(t, `package test {
	state def M {
		in attribute c : Boolean;
		entry action begin { }
		transition begin if c then busy;
		transition begin if not c then idle;
		state busy;
		state idle;
	}
}`)
}

// Nothing transitions into an entry action.
func TestTransitionIntoEntryActionIsNotAVertex(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		entry action begin { }
		transition begin then busy;
		state busy;
		transition busy then begin;
	}
}`, CodeEndpointNotOfMachine, "begin")
}

// Only the body a transition is written in lends it an entry action to leave: a
// name reaching another state's is not a start designation, and lowering agrees.
func TestTransitionOutOfAnotherStatesEntryActionIsNotAVertex(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		state a { entry action begin { } }
		state b;
		transition begin then b;
	}
}`, CodeEndpointNotOfMachine, "begin")
}

// A start designation names the state the machine starts in, so a transition out
// of an entry action into a pseudostate is not one.
func TestEntryActionTransitionIntoPseudostateIsNotAVertex(t *testing.T) {
	wantOneError(t, `package test {
	state def M {
		entry action begin { }
		transition begin then j;
		junction j;
		state b;
		transition j then b;
	}
}`, CodeEndpointNotOfMachine, "begin")
}
