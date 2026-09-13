package passes

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/lower"
	"github.com/Open-MBEE/OpenSysML/internal/core/resolve"
)

// endpointDiags runs the name-resolution tier over src, the tier an endpoint
// name is resolved at.
func endpointDiags(t *testing.T, src string) []Diagnostic {
	t.Helper()
	ctx, root := nameresCtx(t, "a.sysml", src)
	return NameResolutionPass{}.Run(ctx, "a.sysml", root)
}

// An endpoint naming a member the body does not declare is reported where the
// name is written, for every spelling the notation admits, rather than only when
// the model is executed.
func TestEndpointNamingNoMemberIsReported(t *testing.T) {
	cases := map[string]string{
		"succession": `package P { action def A {
			first start;
			action a;
			done;
			succession first start then zzz;
		} }`,
		"guarded succession": `package P { action def A {
			first start;
			action a;
			done;
			succession first start if true then zzz;
		} }`,
		"implicit source": `package P { action def A {
			first start;
			action a;
			then zzz;
		} }`,
		"decision branch": `package P { action def A {
			first start;
			action pick;
			if true then zzz;
		} }`,
		"default decision branch": `package P { action def A {
			first start;
			action pick;
			if true then pick;
			else zzz;
		} }`,
		"transition": `package P { state def M {
			entry; then idle;
			state idle;
			transition first idle then zzz;
		} }`,
		"transition source": `package P { state def M {
			entry; then idle;
			state idle;
			transition first zzz then idle;
		} }`,
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			got := endpointDiags(t, src)
			if len(got) != 1 {
				t.Fatalf("expected one diagnostic for the endpoint, got %+v", got)
			}
			d := got[0]
			if d.Severity != SeverityError {
				t.Errorf("expected an error, got %v", d.Severity)
			}
			if d.Code != "unresolved" || d.Source != "name-resolution" {
				t.Errorf("got code %q source %q, want unresolved at the name-resolution tier", d.Code, d.Source)
			}
			if !strings.Contains(d.Message, "zzz") {
				t.Errorf("expected the message to name zzz, got %q", d.Message)
			}
			if got := strings.TrimSpace(src[d.Span.Offset : d.Span.Offset+d.Span.Len]); got != "zzz" {
				t.Errorf("expected the span to cover the endpoint name, it covers %q", got)
			}
		})
	}
}

// `then final;` is a reference to a member named final now that the keyword is
// gone, so a body which declares no such member is reported rather than parsed
// as a final node and accepted.
func TestEndpointNamingRemovedFinalKeywordIsReported(t *testing.T) {
	src := `package P { action def A {
	first start;
	action a;
	then final;
} }`
	got := endpointDiags(t, src)
	if len(got) != 1 || !strings.Contains(got[0].Message, "final") {
		t.Fatalf("expected one diagnostic naming final, got %+v", got)
	}
	if covered := src[got[0].Span.Offset : got[0].Span.Offset+got[0].Span.Len]; covered != "final" {
		t.Errorf("expected the span to cover the endpoint name, it covers %q", covered)
	}
}

// Every endpoint the notation admits and the lowerer resolves stays silent: a
// declared member, the implicit `start` and `done` the standard library declares
// on Action, and the ends a member-attached `then` binds by position.
func TestEndpointsThatResolveStaySilent(t *testing.T) {
	cases := map[string]string{
		"declared members": `package P { action def A {
			action a;
			action b;
			succession first a then b;
		} }`,
		"implicit start": `package P { action def A {
			action a;
			succession first start then a;
		} }`,
		"initial node supplies start": `package P { action def A {
			first start;
			action a;
			done;
			succession first start then a;
		} }`,
		"implicit done": `package P { action def A {
			first start;
			action a;
			then done;
		} }`,
		"declared final node": `package P { action def A {
			first start;
			action a;
			done;
			succession first a then done;
		} }`,
		"guarded branches": `package P { action def A {
			first start;
			action a;
			action b;
			succession first a if true then b;
		} }`,
		"positional send end": `package P { action def A {
			attribute x;
			first start;
			action a;
			then send x() to self;
			then done;
		} }`,
		"positional loop end": `package P { action def A {
			first start;
			then loop action { action b; } until true;
			then done;
		} }`,
		"nested vertex of a machine": `package P { state def M {
			entry; then alpha;
			state alpha { state work; }
			state beta;
			then work;
		} }`,
		"end outside a machine or an action body": `package P { part def D {
			action def A { action x; }
			perform action p;
			then p;
		} }`,
		"transition ends": `package P { state def M {
			entry; then idle;
			state idle;
			state busy;
			attribute ready;
			transition first idle if ready then busy;
		} }`,
		"transition with a trigger and an effect": `package P {
			attribute def Ping;
			state def M {
				entry; then idle;
				state idle;
				state busy;
				action log;
				transition first idle accept Ping do action log then busy;
			}
		}`,
		"feature-chain action node": `package P { action def A {
			action source { attribute pin; }
			action target;
			succession first source.pin then target;
		} }`,
	}

	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			if got := endpointDiags(t, src); len(got) != 0 {
				t.Fatalf("expected no diagnostics, got %+v", got)
			}
		})
	}
}

func TestActionEndpointPassAcceptsLoweredNodes(t *testing.T) {
	cases := map[string]string{
		"implicit start": `package P { action def A {
			action a;
			succession first start then a;
		} }`,
		"implicit done": `package P { action def A {
			first start;
			action a;
			then done;
		} }`,
		"declared final": `package P { action def A {
			first start;
			action a;
			done;
			succession first a then done;
		} }`,
		"positional send": `package P { action def A {
			attribute x;
			first start;
			then send x() to self;
			then done;
		} }`,
		"positional loop": `package P { action def A {
			first start;
			then loop action { action tick; } until true;
			then done;
		} }`,
		"implied end": `package P { action def A {
			action a;
			then a;
		} }`,
		"control nodes": `package P { action def A {
			first start;
			fork split;
			join sync;
			merge gate;
			decide choose;
			succession first start then split;
			succession first split then sync;
			succession first sync then gate;
			succession first gate then choose;
		} }`,
		"nested action usage": `package P { action def A {
			action child;
			action sibling;
			succession first child then sibling;
		} }`,
		"sequenced if": `package P { action def A {
			action a;
			if true then a;
		} }`,
		"sequenced statements": `package P { action def A {
			attribute x;
			first start;
			then while true { terminate; }
			then assign x := 1;
			then send x() to self;
			then terminate;
		} }`,
		"feature-chain root": `package P { action def A {
			action source { attribute pin; }
			action target;
			succession first source.pin then target;
		} }`,
		"unresolved endpoint": `package P { action def A {
			action a;
			succession first a then missing;
		} }`,
		"nested action definition": `package P { action def Outer {
			action def Inner {
				action leaf;
				succession first start then leaf;
			}
			action next;
			succession first start then next;
		} }`,
		"state body": `package P { state def M {
			attribute mode = 0;
			entry; then idle;
			state idle;
			succession first idle then mode;
		} }`,
		"performed action in part": `package P { part def D {
			action def A { action x; }
			perform action p;
			then p;
		} }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, root := nameresCtx(t, "a.sysml", src)
			if got := (ActionEndpointPass{}).Run(ctx, "a.sysml", root); len(got) != 0 {
				t.Fatalf("expected no action endpoint diagnostics, got %+v", got)
			}
		})
	}
}

func TestStateSuccessionEndpointSpellingsAcceptVertices(t *testing.T) {
	targets := map[string]string{
		"choice":   `choice pick;`,
		"junction": `junction pick;`,
		"fork":     `fork pick;`,
		"join":     `join pick;`,
		"nested":   `state outer { state pick; }`,
	}
	for _, spelling := range []string{"succession first idle then pick;", "then pick;"} {
		for name, declaration := range targets {
			t.Run(spelling+"/"+name, func(t *testing.T) {
				src := `package P { state def M {
					entry; then idle;
					state idle;
					` + declaration + `
					` + spelling + `
				} }`
				if got := endpointDiags(t, src); len(got) != 0 {
					t.Fatalf("expected no diagnostics for a legal vertex endpoint, got %+v", got)
				}
			})
		}
	}

	cases := map[string]string{
		"region-local": `package P { state def M {
			state outer parallel {
				state left {
					state idle;
					state pick;
					succession first idle then pick;
				}
				state right { state other; }
			}
		} }`,
		"cross-region": `package P { state def M {
			state outer parallel {
				state left { state idle; }
				state right { state pick; }
			}
			succession first idle then pick;
		} }`,
		"inherited": `package P {
			state def Base { state idle; }
			state def Derived :> Base {
				state busy;
				EDGE
			}
		}`,
		"entry action source": `package P { state def M {
			entry action begin { }
			state idle;
			EDGE
		} }`,
		"first marker source": `package P { state def M {
			first marker then other;
			state other;
			state idle;
			EDGE
		} }`,
	}
	for name, src := range cases {
		for _, spelling := range []string{"succession first idle then idle;", "then idle;"} {
			t.Run(name+"/"+spelling, func(t *testing.T) {
				model := strings.Replace(src, "EDGE", spelling, 1)
				if name == "entry action source" {
					model = strings.Replace(model, "first idle", "first begin", 1)
				}
				if got := endpointDiags(t, model); len(got) != 0 {
					t.Fatalf("expected no diagnostics for a legal vertex endpoint, got %+v", got)
				}
			})
		}
	}
}

func TestActionEndpointPassFindsNestedActionDefinitions(t *testing.T) {
	cases := map[string]string{
		"state substate": `package P { state def M {
			state outer {
				action def Inner {
					attribute flag = 0;
					action leaf;
					succession first leaf then flag;
				}
			}
		} }`,
		"state region": `package P { state def M {
			state outer parallel {
				state left {
					action def Inner {
						attribute flag = 0;
						action leaf;
						succession first leaf then flag;
					}
				}
			}
		} }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, root := nameresCtx(t, "a.sysml", src)
			got := ActionEndpointPass{}.Run(ctx, "a.sysml", root)
			if len(got) != 1 || got[0].Code != CodeEndpointNotANode {
				t.Fatalf("expected one nested action endpoint diagnostic, got %+v", got)
			}
		})
	}
}

func TestResolvedStateEndpointNotVertexIsReported(t *testing.T) {
	cases := map[string]string{
		"machine end naming no vertex": `package P { state def M {
			attribute t = 0;
			entry; then s1;
			state s1;
			then t;
		} }`,
		"succession": `package P { state def M {
			attribute mode = 0;
			entry; then idle;
			state idle;
			succession first idle then mode;
		} }`,
		"then": `package P { state def M {
			attribute mode = 0;
			entry; then idle;
			state idle;
			then mode;
		} }`,
		"transition": `package P { state def M {
			attribute mode = 0;
			entry; then idle;
			state idle;
			transition first idle then mode;
		} }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			got := endpointDiags(t, src)
			if len(got) != 1 {
				t.Fatalf("expected one diagnostic for the endpoint, got %+v", got)
			}
			if got[0].Code != resolve.CodeNotAVertex {
				t.Fatalf("got code %q, want %q", got[0].Code, resolve.CodeNotAVertex)
			}
			covered := src[got[0].Span.Offset : got[0].Span.Offset+got[0].Span.Len]
			if covered != "mode" && covered != "t" {
				t.Fatalf("expected the span to cover the endpoint name, it covers %q", covered)
			}
		})
	}
}

func TestActionEndpointPassReportsResolvedNonNodes(t *testing.T) {
	cases := map[string]string{
		"succession": `package P { action def A {
			attribute flag = 0;
			action a;
			succession first a then flag;
		} }`,
		"then": `package P { action def A {
			attribute flag = 0;
			action a;
			first a;
			then flag;
		} }`,
		"transition": `package P { action def A {
			attribute flag = 0;
			action a;
			transition first a then flag;
		} }`,
		"decision branch": `package P { action def A {
			first start;
			action a;
			attribute flag = 0;
			if true then flag;
		} }`,
		"default decision branch": `package P { action def A {
			first start;
			action a;
			attribute flag = 0;
			if true then a;
			else flag;
		} }`,
		"transition source": `package P { action def A {
			attribute flag = 0;
			action a;
			transition first flag then a;
		} }`,
		"first marker": `package P { action def A {
			attribute flag = 0;
			action a;
			first a then flag;
		} }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, root := nameresCtx(t, "a.sysml", src)
			got := ActionEndpointPass{}.Run(ctx, "a.sysml", root)
			if len(got) != 1 {
				t.Fatalf("expected one diagnostic for the endpoint, got %+v", got)
			}
			if got[0].Code != CodeEndpointNotANode || got[0].Source != "action-endpoint" {
				t.Fatalf("got code %q source %q, want action endpoint diagnostic", got[0].Code, got[0].Source)
			}
			if covered := strings.TrimSpace(src[got[0].Span.Offset : got[0].Span.Offset+got[0].Span.Len]); covered != "flag" {
				t.Fatalf("expected the span to cover flag, it covers %q", covered)
			}
		})
	}
}

func TestActionEndpointPassDiagnosticAgreesWithLowering(t *testing.T) {
	cases := map[string]struct {
		src      string
		fails    bool
		endpoint string
	}{
		"accepted action node": {
			src: `package P { action def A {
				action a;
				succession first start then a;
			} }`,
		},
		"undefined target node": {
			src: `package P { action def A {
				attribute flag = 0;
				action a;
				succession first a then flag;
			} }`,
			fails:    true,
			endpoint: "flag",
		},
		"undefined source node": {
			src: `package P { action def A {
				attribute flag = 0;
				action a;
				transition first flag then a;
			} }`,
			fails:    true,
			endpoint: "flag",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			decl := actionDefIn(t, parseDoc(t, "a.sysml", c.src))
			_, err := lower.ToActionGraph(decl, nil)
			ctx, root := nameresCtx(t, "a.sysml", c.src)
			diags := ActionEndpointPass{}.Run(ctx, "a.sysml", root)
			if c.fails != (err != nil) {
				t.Fatalf("lowering error = %v, want %v", err, c.fails)
			}
			if !c.fails {
				if len(diags) != 0 {
					t.Fatalf("lowering accepts the endpoint but pass reports %+v", diags)
				}
				return
			}
			if len(diags) != 1 || diags[0].Code != CodeEndpointNotANode {
				t.Fatalf("lowering rejects the endpoint but pass reports %+v", diags)
			}
			if covered := strings.TrimSpace(c.src[diags[0].Span.Offset : diags[0].Span.Offset+diags[0].Span.Len]); covered != c.endpoint {
				t.Fatalf("expected the span to cover %s, it covers %q", c.endpoint, covered)
			}
		})
	}
}

func TestActionEndpointPassInheritedNodes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "inherited action node",
			src: `action def Base { action a; }
				action def Derived :> Base { first start then a; }`,
		},
		{
			name: "inherited non-node",
			src: `action def Base { attribute a; }
				action def Derived :> Base { first start then a; }`,
			want: 1,
		},
		{
			name: "unknown supertype",
			src:  `action def Derived :> Missing { first start then a; }`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, root := nameresCtx(t, "a.sysml", test.src)
			ctx.Model()
			got := ActionEndpointPass{}.Run(ctx, "a.sysml", root)
			if len(got) != test.want {
				t.Fatalf("got %d diagnostics %+v, want %d", len(got), got, test.want)
			}
			if test.want == 1 && got[0].Code != CodeEndpointNotANode {
				t.Fatalf("got code %q, want %q", got[0].Code, CodeEndpointNotANode)
			}
		})
	}
}

func TestActionEndpointPassIgnoresFeatureChainsThroughNonNodes(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want int
	}{
		{
			name: "typed part chain",
			src: `package P {
				part def Vehicle { action doorClosed; }
				part def Driver { action driverReady; }
				action def Startup {
					part vehicle : Vehicle;
					part driver : Driver;
					first vehicle.doorClosed then driver.driverReady;
				}
			}`,
		},
		{
			name: "plain non-node remains diagnosed",
			src: `package P {
				part def Vehicle { action doorClosed; }
				action def Startup {
					part vehicle : Vehicle;
					attribute flag = 0;
					first vehicle.doorClosed then flag;
				}
			}`,
			want: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, root := nameresCtx(t, "a.sysml", test.src)
			got := ActionEndpointPass{}.Run(ctx, "a.sysml", root)
			if len(got) != test.want {
				t.Fatalf("got %d diagnostics %+v, want %d", len(got), got, test.want)
			}
			if test.want == 1 && !strings.Contains(got[0].Message, "flag") {
				t.Fatalf("diagnostic does not name flag: %+v", got[0])
			}
		})
	}
}

// The tier agrees with the lowerer about which endpoint names are implicit: a
// name lowering resolves is silent here, and a name lowering fails on is
// reported here, so the diagnostic arrives at validation time instead.
func TestEndpointDiagnosticAgreesWithLowering(t *testing.T) {
	cases := map[string]struct {
		src      string
		resolves bool
	}{
		"start": {`package P { action def A { action a; succession first start then a; } }`, true},
		"done":  {`package P { action def A { first start; action a; then done; } }`, true},
		"declared final": {`package P { action def A {
			first start;
			action a;
			done;
			succession first a then done;
		} }`, true},
		"undeclared": {`package P { action def A {
			first start;
			action a;
			done;
			succession first start then zzz;
		} }`, false},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := lower.ToActionGraph(actionDefIn(t, parseDoc(t, "a.sysml", c.src)), nil)
			diags := endpointDiags(t, c.src)
			if c.resolves {
				if err != nil {
					t.Fatalf("expected lowering to resolve the endpoint, got %v", err)
				}
				if len(diags) != 0 {
					t.Fatalf("lowering resolves the endpoint but the tier reports %+v", diags)
				}
				return
			}
			if err == nil {
				t.Fatal("expected lowering to fail on the endpoint")
			}
			if len(diags) != 1 {
				t.Fatalf("lowering fails on the endpoint but the tier reports %+v", diags)
			}
		})
	}
}

// In a machine an end names a vertex the way a transition's does, so a nested or
// region-local one lowering reaches must stay silent here too.
func TestStateMachineEndpointAgreesWithLowering(t *testing.T) {
	const src = `package P { state def M {
		entry; then alpha;
		state alpha { state work; }
		state beta;
		then work;
	} }`

	ctx, root := nameresCtx(t, "a.sysml", src)
	if got := (NameResolutionPass{}).Run(ctx, "a.sysml", root); len(got) != 0 {
		t.Fatalf("expected no diagnostics for a nested vertex, got %+v", got)
	}

	machine := ctx.Index.LookupQualified("P::M")
	if len(machine) != 1 {
		t.Fatalf("looked up %d symbols for P::M, want the machine", len(machine))
	}
	sym := machine[0]
	scope := sym.Scope
	if scope == nil {
		scope = sym.OwnerScope
	}
	graph, err := lower.ToStateGraphWithEndpoints(sym.Decl, scope, ctx.Resolver())
	if err != nil {
		t.Fatalf("lowering the machine: %v", err)
	}
	var found bool
	for _, transitions := range graph.Transitions {
		for _, trans := range transitions {
			if state, ok := trans.Target.(*ast.StateNode); ok && state.Name == "work" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("lowering resolved no transition into the nested work, so the case does not test agreement")
	}
}

// actionDefIn returns the first action definition declared in root, the
// declaration lowering builds a graph from.
func actionDefIn(t *testing.T, root *ast.RootNamespace) ast.Node {
	t.Helper()
	var walk func(members []ast.Node) ast.Node
	walk = func(members []ast.Node) ast.Node {
		for _, member := range members {
			if ms, ok := member.(*ast.Membership); ok {
				member = ms.Member
			}
			switch n := member.(type) {
			case *ast.Definition:
				if n.Kind == ast.DefAction {
					return n
				}
				if found := walk(n.Members); found != nil {
					return found
				}
			case *ast.Package:
				if found := walk(n.Members); found != nil {
					return found
				}
			case *ast.Namespace:
				if found := walk(n.Members); found != nil {
					return found
				}
			}
		}
		return nil
	}
	def := walk(root.Members)
	if def == nil {
		t.Fatal("no action definition found")
	}
	return def
}
