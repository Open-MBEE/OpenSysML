package behavior_test

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes"
	"github.com/Open-MBEE/OpenSysML/internal/core/passes/behavior"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/diag"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/parser"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// controlNodeDiags runs the pass alone on a model that must parse clean.
func controlNodeDiags(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	sf := source.New("t.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	idx := newTestIndexFromDoc("t.sysml", root)
	return behavior.ControlNodeSuccessionPass{}.Run(passes.NewContext("t.sysml", idx, nil), "t.sysml", root)
}

func wantControlNodesClean(t *testing.T, src string) {
	t.Helper()
	if got := controlNodeDiags(t, src); len(got) != 0 {
		t.Fatalf("got %+v, want no diagnostics", got)
	}
}

// wantControlNodeErrors fails unless the pass reports exactly the given codes, in
// order, each at the given 1-based line with a message containing the text.
func wantControlNodeErrors(t *testing.T, src string, want ...controlNodeWant) []diag.Diagnostic {
	t.Helper()
	got := controlNodeDiags(t, src)
	if len(got) != len(want) {
		t.Fatalf("got %d diagnostics %+v, want %d", len(got), got, len(want))
	}
	sf := source.New("t.sysml", []byte(src))
	for i, d := range got {
		w := want[i]
		line := sf.Lines().PosAt(d.Span.Offset).Line
		if d.Severity != diag.SeverityError || d.Source != "control-node" || d.Code != w.code {
			t.Fatalf("diagnostic %d: got %+v, want severity=error source=control-node code=%s", i, d, w.code)
		}
		if line != w.line {
			t.Fatalf("diagnostic %d: got line %d (%q), want line %d", i, line, d.Message, w.line)
		}
		if !strings.Contains(d.Message, w.text) {
			t.Fatalf("diagnostic %d: got message %q, want it to contain %q", i, d.Message, w.text)
		}
	}
	return got
}

type controlNodeWant struct {
	code string
	line int
	text string
}

// A well-formed graph in every succession notation is silent: a fork with one
// incoming, a join and merge with one outgoing, a decision with one incoming.
func TestControlNodeWellFormedGraphIsSilent(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def A {
		action a;
		then fork f;
		first f then b;
		first f then c;
		action b;
		action c;
		join j;
		succession s1 first b then j;
		succession s2 first c then j;
		then merge m;
		then decide d;
		if true then e;
		else g;
		action e;
		action g;
	}
}`)
}

// The specification's merge and decision examples (SysML v2 8.4.13.4) carry the
// required multiplicities on every succession end, written as the notation
// places them: before the end they constrain.
func TestControlNodeSpecificationExamplesAreSilent(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def A3 {
		action a1;
		action a2;
		succession s1 first [0..1] a1 then [1..1] m;
		succession s2 first [0..1] a2 then [1..1] m;
		merge m;
		succession s3 first [1..1] m then a3;
		action a3;
	}
	action def A4 {
		action a1;
		succession s1 first a1 then [1..1] d;
		decide d;
		succession s2 first [1..1] d then [0..1] a2;
		succession s3 first [1..1] d then [0..1] a3;
		action a2;
		action a3;
	}
	action def A2 {
		action a1;
		succession s1 first a1 then [1] f;
		fork f;
		succession s2 first [1] f then [1..1] a2;
		succession s3 first [1] f then [1] a3;
		action a2;
		action a3;
	}
}`)
}

// Only successions count: a connection or binding to a control node is not a
// succession into it.
func TestControlNodeOtherConnectorsDoNotCount(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def A {
		action a;
		action b;
		fork f;
		first a then f;
		connect b to f;
		bind b = f;
		first f then b;
	}
}`)
}

// Control nodes live in every action body shape: nested usages, loop and branch
// bodies, a control node's own body, and the action kinds that specialize action.
func TestControlNodeInEveryActionBodyIsSilent(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def A {
		action a { fork f; action x; action y; first f then x; first f then y; }
		while true { join j; action p; first p then j; }
		if true { merge m; }
		then fork g { decide d; }
	}
	part def Q {
		action c { fork r; }
		perform action p { join s; }
	}
	state def S { entry action { fork t; } }
	calc def K { fork kk; }
	case def C { fork cc; }
}`)
}

func TestForkWithTwoIncomingSuccessions(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"shorthand", `action a; then fork f; action b; then f; first f then c; action c;`},
		{"first then", `action a; action b; fork f; first a then f; first b then f; first f then a;`},
		{"succession usage", `action a; action b; fork f; succession s1 first a then f; succession s2 first b then f;`},
		{"mixed", `action a; action b; fork f; first a then f; succession s2 first b then f;`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantControlNodeErrors(t, "package P {\n\taction def A {\n\t\t"+tc.body+"\n\t}\n}",
				controlNodeWant{behavior.CodeForkIncomingSuccessions, 3, "fork f has 2 incoming successions; a fork node may have at most one"})
		})
	}
}

func TestJoinWithTwoOutgoingSuccessions(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a;
		action b;
		action c;
		join j;
		first a then j;
		first j then b;
		first j then c;
	}
}`, controlNodeWant{behavior.CodeJoinOutgoingSuccessions, 6, "join j has 2 outgoing successions; a join node may have at most one"})
}

func TestMergeWithTwoOutgoingSuccessions(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a;
		action b;
		action c;
		merge m;
		first a then m;
		succession s1 first m then b;
		succession s2 first m then c;
	}
}`, controlNodeWant{behavior.CodeMergeOutgoingSuccessions, 6, "merge m has 2 outgoing successions; a merge node may have at most one"})
}

func TestDecisionWithTwoIncomingSuccessions(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a;
		action b;
		decide d;
		first a then d;
		first b then d;
		if true then a;
		else b;
	}
}`, controlNodeWant{behavior.CodeDecisionIncomingSuccessions, 5, "decide d has 2 incoming successions; a decision node may have at most one"})
}

// A named guarded succession (`succession s first a if g then b`) is a
// transition usage owning a succession, and counts on both of its ends.
func TestControlNodeGuardedSuccessionsCount(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action x; action y; action z;
		fork f;
		succession s1 first x if true then f;
		succession s2 first y if true then f;
		first f then z;
	}
	action def B {
		action x; action y; action z;
		join j;
		first x then j; first y then j;
		succession s3 first j if true then z;
		first j then z;
	}
	action def C {
		action x; action y;
		merge m;
		first x then m; first m then y;
		succession s4 first m if false then x;
	}
	action def D {
		action x; action y;
		decide d;
		first x then d;
		succession s5 first y if true then d;
		first d then y;
	}
}`,
		controlNodeWant{behavior.CodeForkIncomingSuccessions, 4, "fork f has 2 incoming successions"},
		controlNodeWant{behavior.CodeJoinOutgoingSuccessions, 11, "join j has 2 outgoing successions"},
		controlNodeWant{behavior.CodeMergeOutgoingSuccessions, 18, "merge m has 2 outgoing successions"},
		controlNodeWant{behavior.CodeDecisionIncomingSuccessions, 24, "decide d has 2 incoming successions"})
	wantControlNodesClean(t, `package P {
	action def A {
		action x; action y; action z;
		decide d;
		first x then d;
		succession s1 first d if true then y;
		succession s2 first d if false then z;
		merge m;
		succession s3 first y if true then m;
		succession s4 first z if true then m;
		first m then x;
	}
}`)
}

// A `succession flow` is a Succession as well as a Flow, so it counts into and
// out of every control-node kind; a plain `flow` beside it does not.
func TestControlNodeSuccessionFlowsCount(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action x { out item v; } action y { out item v; } action z { in item v; }
		fork f { in item v; out item w; }
		succession flow from x.v to f.v;
		succession flow from y.v to f.v;
		succession flow from f.w to z.v;
		flow from x.v to z.v;
	}
	action def B {
		action x { out item v; } action y { in item v; } action z { in item v; }
		join j { in item v; out item w; }
		succession flow from x.v to j.v;
		succession flow s1 from j.w to y.v;
		succession flow j.w to z.v;
	}
	action def C {
		action x { out item v; } action y { in item v; }
		merge m { in item v; out item w; }
		succession flow from x.v to m.v;
		succession flow from m.w to y.v;
		succession flow from m.w to y.v;
	}
	action def D {
		action x { out item v; } action y { out item v; } action z { in item v; }
		decide d { in item v; out item w; }
		succession flow from x.v to d.v;
		succession flow from y.v to d.v;
		succession flow from d.w to z.v;
		flow from y.v to d.v;
	}
}`,
		controlNodeWant{behavior.CodeForkIncomingSuccessions, 4, "fork f has 2 incoming successions"},
		controlNodeWant{behavior.CodeJoinOutgoingSuccessions, 12, "join j has 2 outgoing successions"},
		controlNodeWant{behavior.CodeMergeOutgoingSuccessions, 19, "merge m has 2 outgoing successions"},
		controlNodeWant{behavior.CodeDecisionIncomingSuccessions, 26, "decide d has 2 incoming successions"})
	wantControlNodesClean(t, `package P {
	action def A {
		action x { out item v; } action y { out item v; in item u; } action z { in item v; }
		fork f { in item v; out item w; }
		succession flow from x.v to f.v;
		succession flow from f.w to y.u;
		succession flow from f.w to z.v;
		join j { in item v; out item w; }
		succession flow from y.v to j.v;
		succession flow from x.v to j.v;
		succession flow from j.w to z.v;
		merge m { in item v; out item w; }
		succession flow from y.v to m.v;
		succession flow from x.v to m.v;
		succession flow from m.w to z.v;
		decide d { in item v; out item w; }
		succession flow from z.v to d.v;
		succession flow from d.w to x.v;
		succession flow from d.w to y.u;
	}
}`)
}

// A succession flow's ends relate what their dot notation names ahead of the
// payload feature; an end written without one relates nothing (the flow-end
// rule reports it), and a flow inherited from a general action counts.
func TestControlNodeSuccessionFlowEnds(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def A {
		action x; action y { out item v; } action z { in item v; }
		fork f { in item v; }
		succession flow from x to f;
		succession flow from y.v to f.v;
		first f then z;
	}
}`)
	wantControlNodeErrors(t, `package P {
	action def G {
		action x { out item v; } action z { in item v; }
		fork f { in item v; out item w; }
		succession flow from x.v to f.v;
		succession flow from f.w to z.v;
	}
	action def A :> G {
		action y { out item v; }
		succession flow from y.v to f.v;
	}
	action def B {
		action x { out item v; } action z { in item v; }
		part p { action w { in item v; } }
		join j { in item v; out item w; }
		succession flow from x.v to j.v;
		succession flow from j.w to z.v;
		succession flow from j.w to p.w.v;
	}
}`,
		controlNodeWant{behavior.CodeForkIncomingSuccessions, 10, "fork f has 2 incoming successions"},
		controlNodeWant{behavior.CodeJoinOutgoingSuccessions, 15, "join j has 2 outgoing successions"})
}

// The bounded side is the only one bounded: three successions out of a fork or
// decision, or into a join or merge, are legal.
func TestControlNodeUnboundedSideIsSilent(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def A {
		action a; action b; action c; action e;
		fork f;
		first a then f;
		first f then b; first f then c; first f then e;
		join j;
		first b then j; first c then j; first e then j;
		merge m;
		first j then m; first a then m; first b then m;
		decide d;
		succession first m then d;
		if true then a; if false then b; else c;
	}
}`)
}

func TestMergeIncomingSourceMultiplicity(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a; action b; action c;
		succession s1 first [1..1] a then [1] m;
		succession s2 first [0..1] b then [1] m;
		merge m;
		succession s3 first [1] m then c;
	}
}`, controlNodeWant{behavior.CodeMergeIncomingMultiplicity, 4,
		"succession into merge m has source multiplicity [1]; successions into a merge node must have source multiplicity [0..1]"})
}

func TestDecisionOutgoingTargetMultiplicity(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a; action b; action c;
		succession s1 first a then [1] d;
		decide d;
		succession s2 first [1] d then [1] b;
		succession s3 first [1] d then [0..*] c;
	}
}`, controlNodeWant{behavior.CodeDecisionOutgoingMultiplicity, 6,
		"succession out of decide d has target multiplicity [1]; successions out of a decision node must have target multiplicity [0..1]"},
		controlNodeWant{behavior.CodeDecisionOutgoingMultiplicity, 7,
			"succession out of decide d has target multiplicity [0..*]; successions out of a decision node must have target multiplicity [0..1]"})
}

// Every control node kind requires 1..1 on the target end of an incoming
// succession and on the source end of an outgoing one.
func TestControlNodeEndMultiplicities(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a; action b; action c;
		succession s1 first a then [0..1] f;
		fork f;
		succession s2 first [0..1] f then b;
		succession s3 first [2..3] f then c;
		join j;
		succession s4 first b then [*] j;
		succession s5 first c then [1] j;
		succession s6 first [0..1] j then a;
	}
}`, controlNodeWant{behavior.CodeControlNodeIncomingMultiplicity, 4,
		"succession into fork f has target multiplicity [0..1]; successions into a fork node must have target multiplicity [1]"},
		controlNodeWant{behavior.CodeControlNodeOutgoingMultiplicity, 6,
			"succession out of fork f has source multiplicity [0..1]; successions out of a fork node must have source multiplicity [1]"},
		controlNodeWant{behavior.CodeControlNodeOutgoingMultiplicity, 7, "source multiplicity [2..3]"},
		controlNodeWant{behavior.CodeControlNodeIncomingMultiplicity, 9,
			"succession into join j has target multiplicity [0..*]; successions into a join node must have target multiplicity [1]"},
		controlNodeWant{behavior.CodeControlNodeOutgoingMultiplicity, 11,
			"succession out of join j has source multiplicity [0..1]; successions out of a join node must have source multiplicity [1]"})
}

// A specialized action inherits the successions of its general action. One it
// adds into an inherited fork is reported once, at the succession it adds; the
// general action, whose graph is well formed, stays silent.
func TestControlNodeInheritedSuccessionsCount(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def Base {
		action a; action b; action c;
		fork f;
		succession s first a then f;
		first f then b;
		first f then c;
	}
	action def Derived :> Base {
		action d;
		first d then f;
	}
	action def Derived2 :> Base {
		action d;
		succession s2 first d then f;
	}
	action usage : Base {
		action d;
		then f;
	}
}`, controlNodeWant{behavior.CodeForkIncomingSuccessions, 11, "fork f has 2 incoming successions"},
		controlNodeWant{behavior.CodeForkIncomingSuccessions, 15, "fork f has 2 incoming successions"},
		controlNodeWant{behavior.CodeForkIncomingSuccessions, 19, "fork f has 2 incoming successions"})
}

// Redefining an inherited succession replaces it, so the count stays at one;
// redefining one of its ends does not drop it.
func TestControlNodeRedefinedSuccessionReplacesInherited(t *testing.T) {
	wantControlNodesClean(t, `package P {
	action def Base {
		action a; action b; action c;
		fork f;
		succession s first a then f;
		first f then b;
		first f then c;
	}
	action def Derived :> Base {
		succession :>> s first b then f;
	}
	action def Derived2 :> Base {
		action :>> a;
	}
}`)
	wantControlNodeErrors(t, `package P {
	action def Base {
		action a; action b; action c;
		fork f;
		succession s first a then f;
		first f then b;
	}
	action def Derived :> Base {
		action :>> a;
		first c then f;
	}
}`, controlNodeWant{behavior.CodeForkIncomingSuccessions, 10, "fork f has 2 incoming successions"})
}

// A violation the general action declares is reported there, once, not again in
// every action that specializes it.
func TestControlNodeInheritedViolationReportedOnce(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def Base {
		action a; action b;
		join j;
		first a then j;
		first j then a;
		first j then b;
	}
	action def Derived :> Base {
		action c;
		first j then c;
	}
	action def Derived2 :> Base {
		action c;
	}
}`, controlNodeWant{behavior.CodeJoinOutgoingSuccessions, 4, "join j has 2 outgoing successions"},
		controlNodeWant{behavior.CodeJoinOutgoingSuccessions, 11, "join j has 3 outgoing successions"})
}

// A control node's owner must be an action definition or usage; the grammar
// admits one in a constraint body (a calculation body), and the parser also in
// an occurrence body and in a succession's body, where it is not.
func TestControlNodeOwningType(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	occurrence def O {
		fork f;
	}
	action def A {
		action a; action b;
		first a then b { decide d; }
	}
	constraint def C {
		join j;
		true
	}
	part def Q {
		constraint c {
			merge m;
		}
	}
}`, controlNodeWant{behavior.CodeControlNodeOwner, 3,
		"fork f is declared in occurrence def O, which is not an action; declare it in the body of an action definition or usage"},
		controlNodeWant{behavior.CodeControlNodeOwner, 7,
			"decide d is declared in the body of a succession, which is not an action"},
		controlNodeWant{behavior.CodeControlNodeOwner, 10,
			"join j is declared in constraint def C, which is not an action"},
		controlNodeWant{behavior.CodeControlNodeOwner, 15,
			"merge m is declared in constraint c, which is not an action"})
}

// A constraint stated through a nested body (`require constraint { … }`,
// `assert constraint { … }`) owns the control nodes written in it just as a
// constraint declared with a body does, and it is not an action either.
func TestControlNodeInNestedConstraintBody(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	requirement def R {
		require constraint q { merge m; }
		assume constraint { fork f; }
	}
	constraint def C {
		assert constraint c2 { join j; }
	}
	part def Q {
		assert constraint c { decide d; }
	}
}`, controlNodeWant{behavior.CodeControlNodeOwner, 3,
		"merge m is declared in constraint q, which is not an action"},
		controlNodeWant{behavior.CodeControlNodeOwner, 4,
			"fork f is declared in an unnamed constraint, which is not an action"},
		controlNodeWant{behavior.CodeControlNodeOwner, 7,
			"join j is declared in constraint c2, which is not an action"},
		controlNodeWant{behavior.CodeControlNodeOwner, 10,
			"decide d is declared in constraint c, which is not an action"})
}

// A constraint body carries the statements of a calculation body, so a control
// node nested in an `if` or `while` body there is reached and counted; the
// statement's body is an action, so the node has an owner of the right kind.
func TestControlNodeInConstraintBodyStatements(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	attribute x : ScalarValues::Integer;
	constraint def C {
		in attribute y : ScalarValues::Integer;
		if y > 0 { action a; action b; action c; fork f; first a then f; first b then f; first f then c; }
		while y > 0 { action a; action b; action c; join j; first a then j; first j then b; first j then c; }
		y > 0
	}
	part def Q {
		assert constraint c { if x > 0 { action a; action b; action c; decide d; first a then d; first b then d; first d then c; } x > 0 }
		constraint c2 { if x > 0 ? true else false }
	}
	requirement def R {
		require constraint q { if x > 0 { action a; action b; action c; merge m; first a then m; first m then b; first m then c; } x > 0 }
		assume constraint { while x > 0 { action a; action b; action c; fork f; first a then f; first b then f; first f then c; } }
		assume constraint { if x > 0 ? true else false }
	}
}`, controlNodeWant{behavior.CodeForkIncomingSuccessions, 5, "fork f has 2 incoming successions"},
		controlNodeWant{behavior.CodeJoinOutgoingSuccessions, 6, "join j has 2 outgoing successions"},
		controlNodeWant{behavior.CodeDecisionIncomingSuccessions, 10, "decide d has 2 incoming successions"},
		controlNodeWant{behavior.CodeMergeOutgoingSuccessions, 14, "merge m has 2 outgoing successions"},
		controlNodeWant{behavior.CodeForkIncomingSuccessions, 15, "fork f has 2 incoming successions"})
}

// A nested constraint body declares parameters and features before its
// statements and condition; the nodes among them are reached all the same.
func TestControlNodeAfterNestedConstraintDeclarations(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	requirement def R {
		require constraint {
			in attribute x : ScalarValues::Real;
			attribute y : ScalarValues::Real = x + 1.0;
			if y > 0.0 { action a; action b; action c; fork f; first a then f; first b then f; first f then c; }
			merge m;
			y > 0.0
		}
		assume constraint c {
			in attribute z : ScalarValues::Real;
			assert constraint inner { in attribute w : ScalarValues::Real; decide d; w > z }
			z > 0.0
		}
	}
}`, controlNodeWant{behavior.CodeForkIncomingSuccessions, 6, "fork f has 2 incoming successions"},
		controlNodeWant{behavior.CodeControlNodeOwner, 7, "merge m is declared in an unnamed constraint"},
		controlNodeWant{behavior.CodeControlNodeOwner, 12, "decide d is declared in constraint inner"})
}

// The action body a transition, a send, a guarded succession, or a state's
// entry/do/exit block ends in is an action of its own, whose successions are
// counted as a unit even when the body has no name.
func TestControlNodeInAnonymousActionBodies(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []controlNodeWant
	}{
		{"named transition", `package P {
	state def S {
		state a; state b;
		transition t first a then b {
			action x; action y; action z; fork f;
			first x then f; first y then f; first f then z;
		}
	}
}`, []controlNodeWant{{behavior.CodeForkIncomingSuccessions, 5, "fork f has 2 incoming successions"}}},
		{"anonymous transition", `package P {
	state def S {
		state a; state b;
		transition first a then b {
			action x; action y; action z; join j;
			first x then j; first j then y; first j then z;
		}
	}
}`, []controlNodeWant{{behavior.CodeJoinOutgoingSuccessions, 5, "join j has 2 outgoing successions"}}},
		{"transition end multiplicity", `package P {
	state def S {
		state a; state b;
		transition first a then b {
			action x; action y; merge m;
			succession s first [1..1] x then m;
			first m then y;
		}
	}
}`, []controlNodeWant{{behavior.CodeMergeIncomingMultiplicity, 6, "source multiplicity [1]; successions into a merge node must have source multiplicity [0..1]"}}},
		{"effect action of named transition", `package P {
	state def S {
		state a; state b;
		transition t first a do action { action x; action y; action z; fork f; first x then f; first y then f; first f then z; } then b;
	}
}`, []controlNodeWant{{behavior.CodeForkIncomingSuccessions, 4, "fork f has 2 incoming successions"}}},
		{"effect action of anonymous transition", `package P {
	state def S {
		state a; state b;
		transition first b do action e { action x; action y; action z; join j; first x then j; first j then y; first j then z; } then a;
	}
}`, []controlNodeWant{{behavior.CodeJoinOutgoingSuccessions, 4, "join j has 2 outgoing successions"}}},
		{"braced effect", `package P {
	state def S {
		state a; state b;
		transition u first a do { action x; action y; action z; merge m; first x then m; first m then y; first m then z; } then b;
	}
}`, []controlNodeWant{{behavior.CodeMergeOutgoingSuccessions, 4, "merge m has 2 outgoing successions"}}},
		{"effect send", `package P {
	state def S {
		state a; state b;
		transition first b do send 1 to a { action x; action y; decide d; first x then d; first y then d; } then a;
	}
}`, []controlNodeWant{{behavior.CodeDecisionIncomingSuccessions, 4, "decide d has 2 incoming successions"}}},
		{"triggered transition body", `package P {
	attribute def Sig;
	state def S {
		state a; state b;
		transition first a accept e : Sig then b {
			action x; action y; action z; fork f;
			first x then f; first y then f; first f then z;
		}
	}
}`, []controlNodeWant{{behavior.CodeForkIncomingSuccessions, 6, "fork f has 2 incoming successions"}}},
		{"triggered transition braced effect", `package P {
	attribute def Sig;
	state def S {
		state a; state b;
		transition t first a accept e : Sig do { action x; action y; merge m; succession s first [1..1] x then m; first m then y; } then b;
	}
}`, []controlNodeWant{{behavior.CodeMergeIncomingMultiplicity, 5, "source multiplicity [1]; successions into a merge node must have source multiplicity [0..1]"}}},
		{"triggered transition effect action", `package P {
	attribute def Sig;
	state def S {
		state a; state b;
		transition first a accept e : Sig do action { action x; action y; action z; join j; first x then j; first j then y; first j then z; } then b;
	}
}`, []controlNodeWant{{behavior.CodeJoinOutgoingSuccessions, 5, "join j has 2 outgoing successions"}}},
		{"do block", `package P {
	state def S {
		state c {
			do { action x; action y; action z; merge m; first x then m; first m then y; first m then z; }
		}
	}
}`, []controlNodeWant{{behavior.CodeMergeOutgoingSuccessions, 4, "merge m has 2 outgoing successions"}}},
		{"entry action", `package P {
	state def S {
		state c {
			entry action { action x; action y; decide d; first x then d; first y then d; }
		}
	}
}`, []controlNodeWant{{behavior.CodeDecisionIncomingSuccessions, 4, "decide d has 2 incoming successions"}}},
		{"exit block", `package P {
	state def S {
		state c {
			exit { action x; action y; fork f; first x then f; first y then f; }
		}
	}
}`, []controlNodeWant{{behavior.CodeForkIncomingSuccessions, 4, "fork f has 2 incoming successions"}}},
		{"send body", `package P {
	action def A {
		action p;
		send 1 to p { action x; action y; action z; fork f; first x then f; first y then f; first f then z; }
	}
}`, []controlNodeWant{{behavior.CodeForkIncomingSuccessions, 4, "fork f has 2 incoming successions"}}},
		{"guarded succession body", `package P {
	action def A {
		action p; action q;
		first p if true then q { action x; action y; action z; join j; first x then j; first j then y; first j then z; }
	}
}`, []controlNodeWant{{behavior.CodeJoinOutgoingSuccessions, 4, "join j has 2 outgoing successions"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantControlNodeErrors(t, tc.src, tc.want...)
		})
	}
}

// The same bodies are silent when their graphs are well formed, and a control
// node in a state's entry/do/exit block is owned by that anonymous action.
func TestControlNodeInAnonymousActionBodiesIsSilent(t *testing.T) {
	wantControlNodesClean(t, `package P {
	attribute def Sig;
	state def S {
		state a; state b;
		transition t first a then b { action x; action y; fork f; first x then f; first f then y; }
		transition first b then a { action x; action y; join j; first x then j; first j then y; }
		transition v first a do action g { action x; action y; fork f; first x then f; first f then y; } then b;
		transition first b do { action x; action y; merge m; first x then m; first m then y; } then a;
		transition first a accept e : Sig then b { action x; action y; fork f; first x then f; first f then y; }
		transition first b accept e : Sig do action { action x; action y; join j; first x then j; first j then y; } then a;
		state c {
			entry { fork e; }
			do { action x; action y; merge m; first x then m; first m then y; }
			exit action { decide d; }
		}
	}
	action def A {
		action p; action q;
		send 1 to p { action x; action y; fork f; first x then f; first f then y; }
		first p if true then q { join j; }
	}
}`)
}

// A control node's own body is an action body: the nodes and successions it
// declares are checked as a unit whether or not the node has a name.
func TestControlNodeInUnnamedControlNodeBodies(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      []controlNodeWant
	}{
		{"fork body", `package P {
	action def A {
		fork { action x; action y; action z; fork f; first x then f; first y then f; first f then z; }
	}
}`, []controlNodeWant{{behavior.CodeForkIncomingSuccessions, 3, "fork f has 2 incoming successions"}}},
		{"join body", `package P {
	action def A {
		join { action x; action y; action z; merge m; first x then m; first m then y; first m then z; }
	}
}`, []controlNodeWant{{behavior.CodeMergeOutgoingSuccessions, 3, "merge m has 2 outgoing successions"}}},
		{"merge body", `package P {
	action def A {
		merge { action x; action y; decide d; first x then d; first y then d; }
	}
}`, []controlNodeWant{{behavior.CodeDecisionIncomingSuccessions, 3, "decide d has 2 incoming successions"}}},
		{"decide body", `package P {
	action def A {
		decide { action x; action y; action z; join j; first x then j; first j then y; first j then z; }
	}
}`, []controlNodeWant{{behavior.CodeJoinOutgoingSuccessions, 3, "join j has 2 outgoing successions"}}},
		{"body end multiplicity", `package P {
	action def A {
		fork { action x; action y; merge m; succession s first [1..1] x then m; first m then y; }
	}
}`, []controlNodeWant{{behavior.CodeMergeIncomingMultiplicity, 3, "source multiplicity [1]; successions into a merge node must have source multiplicity [0..1]"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wantControlNodeErrors(t, tc.src, tc.want...)
		})
	}
	wantControlNodesClean(t, `package P {
	action def A {
		fork { action x; action y; fork f; first x then f; first f then y; }
		join { action x; action y; join j; first x then j; first j then y; }
		merge { action x; action y; merge m; first x then m; first m then y; }
		decide { action x; action y; decide d; first x then d; first d then y; }
		fork named { action x; action y; fork f; first x then f; first f then y; }
	}
}`)
}

// The runtime rejects a join or merge with several successors when it runs
// them; the static rule reports the same graphs before anything runs.
func TestControlNodeStaticRuleAgreesWithRuntime(t *testing.T) {
	wantControlNodeErrors(t, `package P {
	action def A {
		action a; action b; action c;
		join j;
		first a then j;
		first j then b;
		first j then c;
		merge m;
		first b then m;
		first m then a;
		first m then c;
	}
}`, controlNodeWant{behavior.CodeJoinOutgoingSuccessions, 4, "join j has 2 outgoing successions"},
		controlNodeWant{behavior.CodeMergeOutgoingSuccessions, 8, "merge m has 2 outgoing successions"})
}

// analyzedControlNodeCodes runs the full registry and returns the codes of the
// control-node diagnostics beside the count of lower-tier ones.
func analyzedControlNodeCodes(t *testing.T, src string) (codes []string, lower int) {
	t.Helper()
	sf := source.New("t.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	for _, d := range passes.Analyze("t.sysml", root, nil, newTestIndexFromDoc("t.sysml", root)) {
		if d.Source == "control-node" {
			codes = append(codes, d.Code)
		} else if d.Severity == diag.SeverityError {
			lower++
		}
	}
	return codes, lower
}

// A lower-tier fault in a node's or succession's body, guard or effect is
// beside the point of the rules, which rest on the node's kind and the
// successions' end references; only an end whose own reference is faulty is
// left out of the count.
func TestControlNodeUnrelatedFaultsKeepDiagnostics(t *testing.T) {
	codes, lower := analyzedControlNodeCodes(t, `package P {
	occurrence def O {
		fork f { action a : Missing; }
	}
	action def A {
		action a; action b; action c;
		fork f;
		first a then f { action z : Missing; }
		first b if nope then f;
		first f then c;
		join j;
		first a then j; first b then j;
		succession s1 first j then a { action z : Missing; }
		succession s2 first j if nope then b;
	}
}`)
	if lower == 0 {
		t.Fatal("the unresolved references were not reported")
	}
	want := []string{behavior.CodeControlNodeOwner, behavior.CodeForkIncomingSuccessions, behavior.CodeJoinOutgoingSuccessions}
	if strings.Join(codes, ",") != strings.Join(want, ",") {
		t.Fatalf("got control-node codes %v, want %v", codes, want)
	}

	// A succession whose target is unresolved still leaves its source, and
	// reaches no node through the faulty end.
	codes, lower = analyzedControlNodeCodes(t, `package P {
	action def A {
		action a; action b;
		join j;
		first a then j;
		first j then b;
		succession s first j then Missing;
		fork f;
		first a then f;
		first b then Missing::f;
	}
}`)
	if lower == 0 {
		t.Fatal("the unresolved references were not reported")
	}
	if want := []string{behavior.CodeJoinOutgoingSuccessions}; strings.Join(codes, ",") != strings.Join(want, ",") {
		t.Fatalf("got control-node codes %v, want %v", codes, want)
	}
}

// A lower-tier fault is a span in the document being checked; an inherited node
// or succession declared in another document is not gated by it, however its
// own offsets happen to line up.
func TestControlNodeOtherDocumentFaultsDoNotGateInheritedNodes(t *testing.T) {
	const base = `package Gen {
	action def A {
		action a; action b;
		fork forkNode;
		first a then forkNode;
		first forkNode then b;
	}
}`
	// alignedTo pads the derived document so unresolved `Missing8` occupies the
	// same byte range as text in the base document.
	alignedTo := func(text string) string {
		offset := strings.Index(base, text)
		if offset < 0 {
			t.Fatalf("%q not in base", text)
		}
		head := "package D { action def B :> Gen::A { attribute pad :"
		if offset < len(head)+1 {
			t.Fatalf("%q too early in base to align with", text)
		}
		return head + strings.Repeat(" ", offset-len(head)) + "Missing8; action x; first x then forkNode; } }"
	}
	for _, text := range []string{"fork for", "forkNode;\n\t\tfirst forkNode"} {
		src := alignedTo(text)
		if strings.Index(src, "Missing8") != strings.Index(base, text) {
			t.Fatalf("Missing8 at %d, want %d", strings.Index(src, "Missing8"), strings.Index(base, text))
		}
		idx := libs.NewModelIndex()
		baseRoot := parser.New(source.New("base.sysml", []byte(base))).ParseFile()
		idx.AddDocument("base.sysml", baseRoot)
		p := parser.New(source.New("d.sysml", []byte(src)))
		root := p.ParseFile()
		if len(p.Diagnostics) != 0 {
			t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
		}
		idx.AddDocument("d.sysml", root)
		var codes []string
		lower := 0
		for _, d := range passes.Analyze("d.sysml", root, nil, idx) {
			if d.Source == "control-node" {
				codes = append(codes, d.Code)
			} else if d.Severity == diag.SeverityError {
				lower++
			}
		}
		if lower == 0 {
			t.Fatalf("aligned with %q: the unresolved reference was not reported", text)
		}
		if want := []string{behavior.CodeForkIncomingSuccessions}; strings.Join(codes, ",") != strings.Join(want, ",") {
			t.Fatalf("aligned with %q: got control-node codes %v, want %v", text, codes, want)
		}
	}
}

// The rule reaches the CLI and LSP through the shared registry.
func TestControlNodeRuleIsRegistered(t *testing.T) {
	src := `package P {
	action def A {
		action a; action b;
		fork f;
		first a then f;
		first b then f;
	}
}`
	sf := source.New("t.sysml", []byte(src))
	p := parser.New(sf)
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("unexpected parse diagnostics: %+v", p.Diagnostics)
	}
	for _, d := range passes.Analyze("t.sysml", root, nil, newTestIndexFromDoc("t.sysml", root)) {
		if d.Code == behavior.CodeForkIncomingSuccessions {
			return
		}
	}
	t.Fatal("Analyze did not report the fork's incoming successions")
}
