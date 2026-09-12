package lower

import (
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/core/ast"
	"github.com/Open-MBEE/OpenSysML/internal/core/parser"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
)

// scopedActionGraph lowers the named action definition with the scope tree of its
// document, so footprints resolve names to their declarations.
func scopedActionGraph(t *testing.T, src, name string) *ActionGraph {
	t.Helper()
	p := parser.New(source.New("test.sysml", []byte(src)))
	root := p.ParseFile()
	if len(p.Diagnostics) != 0 {
		t.Fatalf("parse diagnostics: %v", p.Diagnostics)
	}
	idx := symbols.NewIndexFromDoc("test.sysml", root)
	doc := idx.DocumentRoot("test.sysml")
	decl := actionDefinition(t, root, name)
	scope := doc.ChildFor(decl)
	if scope == nil {
		t.Fatalf("scope for %s missing", name)
	}
	graph, err := ToActionGraph(decl, scope)
	if err != nil {
		t.Fatalf("ToActionGraph: %v", err)
	}
	return graph
}

func footprintNamed(t *testing.T, graph *ActionGraph, name string) Footprint {
	t.Helper()
	node := namedActionNode(t, graph, name)
	fp, ok := graph.Footprints[node]
	if !ok {
		t.Fatalf("node %s has no footprint", name)
	}
	return fp
}

func placeNames(places []Place) []string {
	names := make([]string, 0, len(places))
	for _, p := range places {
		names = append(names, p.Name)
	}
	return names
}

func hasPlace(places []Place, name string) bool {
	for _, p := range places {
		if p.Name == name {
			return true
		}
	}
	return false
}

// Every node of a lowered action carries a footprint; a body's assignments are
// its writes and the values they assign its reads.
func TestFootprintsCoverEveryNode(t *testing.T) {
	graph := scopedActionGraph(t, `
		action def Clash {
			attribute x : Integer = 0;
			attribute y : Integer = 0;
			attribute leftRan : Boolean = false;
			first start;
			fork split;
			action left { assign x := y + 1; assign leftRan := true; }
			action right { assign y := 2; }
			join sync;
			done;
			succession first start then split;
			succession first split then left;
			succession first split then right;
			succession first left then sync;
			succession first right then sync;
			succession first sync then done;
		}
	`, "Clash")

	for _, node := range graph.Nodes {
		if _, ok := graph.Footprints[node]; !ok {
			t.Errorf("node %s has no footprint", getNodeName(node))
		}
	}
	left := footprintNamed(t, graph, "left")
	if got := placeNames(left.Writes); strings.Join(got, ",") != "x,leftRan" {
		t.Errorf("left writes = %v, want [x leftRan]", got)
	}
	if !hasPlace(left.Reads, "y") {
		t.Errorf("left reads = %v, want y among them", placeNames(left.Reads))
	}
	if left.Dynamic {
		t.Errorf("left is dynamic:\n%s", left)
	}
	for _, p := range left.Writes {
		if p.Sym == nil {
			t.Errorf("write of %s did not resolve to its declaration", p.Name)
		}
		if p.Local {
			t.Errorf("write of %s marked as a pin", p.Name)
		}
	}
}

// The dependence relation holds exactly where the note's clauses say it does.
func TestFootprintDependence(t *testing.T) {
	graph := scopedActionGraph(t, `
		action def Deps {
			attribute x : Integer = 0;
			attribute y : Integer = 0;
			attribute flag : Boolean = false;
			attribute temp : Integer = 0;
			first start;
			fork split;
			action writeX { assign x := 1; }
			action writeXAgain { assign x := 2; }
			action writeY { assign y := 1; }
			action readX { assign y := x; }
			action setFlag { assign flag := true; }
			action guarded;
			action after;
			action heat { assign temp := 40; }
			action awaitWarm accept when temp > 20;
			action sender send 1;
			action receiver accept n : Integer;
			action toJoinA;
			action toJoinB;
			join sync;
			done;
			succession first start then split;
			succession first split then writeX;
			succession first split then writeXAgain;
			succession first split then writeY;
			succession first split then readX;
			succession first split then setFlag;
			succession first split then guarded;
			succession first guarded if flag then after;
			succession first split then heat;
			succession first split then awaitWarm;
			succession first split then sender;
			succession first split then receiver;
			succession first split then toJoinA;
			succession first split then toJoinB;
			succession first toJoinA then sync;
			succession first toJoinB then sync;
			succession first sync then done;
		}
	`, "Deps")

	fp := func(name string) Footprint { return footprintNamed(t, graph, name) }
	cases := []struct {
		a, b      string
		dependent bool
		why       string
	}{
		{"writeX", "writeXAgain", true, "two writes of one feature"},
		{"writeX", "readX", true, "a write meets a read"},
		{"writeX", "writeY", false, "writes of different features"},
		{"writeY", "readX", true, "readX writes what writeY writes"},
		{"setFlag", "guarded", true, "a write to a feature a succession guard reads"},
		{"writeX", "guarded", false, "the guard reads another feature"},
		{"heat", "awaitWarm", true, "a write to a feature a parked trigger reads"},
		{"writeX", "awaitWarm", false, "the trigger reads another feature"},
		{"sender", "receiver", true, "a send may answer an accept"},
		{"sender", "writeX", false, "a send meets no accept"},
		{"toJoinA", "toJoinB", true, "both arrive at one join"},
		{"toJoinA", "writeX", false, "only one converges"},
	}
	for _, tc := range cases {
		got := fp(tc.a).Dependent(fp(tc.b))
		if got != tc.dependent {
			t.Errorf("%s ~ %s = %v, want %v (%s)\n%s\n%s", tc.a, tc.b, got, tc.dependent, tc.why, fp(tc.a), fp(tc.b))
		}
		if back := fp(tc.b).Dependent(fp(tc.a)); back != got {
			t.Errorf("%s ~ %s is not symmetric", tc.a, tc.b)
		}
	}
}

// A guard's read belongs to the node the guarded succession leaves, and a
// trigger's condition to the accept it parks.
func TestFootprintGuardAndTriggerReads(t *testing.T) {
	graph := scopedActionGraph(t, `
		action def Guards {
			attribute flag : Boolean = false;
			attribute temp : Integer = 0;
			first start;
			action guarded;
			action after;
			action awaitWarm accept when temp > 20;
			done;
			succession first start then guarded;
			succession first guarded if flag then after;
			succession first after then awaitWarm;
			succession first awaitWarm then done;
		}
	`, "Guards")

	if guarded := footprintNamed(t, graph, "guarded"); !hasPlace(guarded.Reads, "flag") || len(guarded.Writes) != 0 {
		t.Errorf("guarded footprint:\n%s\nwant a read of flag and no writes", guarded)
	}
	await := footprintNamed(t, graph, "awaitWarm")
	if !hasPlace(await.Reads, "temp") {
		t.Errorf("awaitWarm footprint:\n%s\nwant a read of temp", await)
	}
	// A trigger is answered by the clock or a condition, never from the message queue.
	if len(await.Accepts) != 0 || await.Dynamic {
		t.Errorf("awaitWarm footprint:\n%s\nwant no channel and no dynamic target", await)
	}
}

// A typed accept names its signal and port; its parameter is a pin it writes.
func TestFootprintAcceptChannel(t *testing.T) {
	graph := scopedActionGraph(t, `
		action def Listen {
			attribute a : Integer = 0;
			first start;
			action left accept x : Integer;
			action recorder { assign a := x; }
			done;
			succession first start then left;
			succession first left then recorder;
			succession first recorder then done;
		}
	`, "Listen")

	left := footprintNamed(t, graph, "left")
	if len(left.Accepts) != 1 || left.Accepts[0].Signal != "Integer" || left.Accepts[0].Port != "" {
		t.Errorf("left accepts = %v, want [Integer]", left.Accepts)
	}
	if !hasPlace(left.Writes, "x") {
		t.Errorf("left writes = %v, want the accept parameter", placeNames(left.Writes))
	}
	recorder := footprintNamed(t, graph, "recorder")
	if !hasPlace(recorder.Reads, "x") || !hasPlace(recorder.Writes, "a") {
		t.Errorf("recorder footprint:\n%s\nwant read x, write a", recorder)
	}
	if !left.Dependent(recorder) {
		t.Error("the accept and the reader of its parameter are independent")
	}
}

// Two nodes' own pins of one name are distinct places; a pin bound to an
// enclosing feature reaches that feature.
func TestFootprintPinsAndBindings(t *testing.T) {
	graph := scopedActionGraph(t, `
		action def Pins {
			attribute total : Integer = 0;
			attribute other : Integer = 0;
			first start;
			fork split;
			action addA { in n : Integer; out r : Integer; }
			action addB { in n : Integer; out r : Integer; }
			action afterA;
			action afterB;
			action bound { in n : Integer; }
			bind bound.n = total;
			join sync;
			done;
			succession first start then split;
			succession first split then addA;
			succession first split then addB;
			succession first split then bound;
			succession first addA then afterA;
			succession first addB then afterB;
			succession first afterA then sync;
			succession first afterB then sync;
			succession first bound then sync;
			succession first sync then done;
		}
	`, "Pins")

	addA, addB := footprintNamed(t, graph, "addA"), footprintNamed(t, graph, "addB")
	for _, fp := range []Footprint{addA, addB} {
		if got := placeNames(fp.Writes); strings.Join(got, ",") != "n,r" {
			t.Errorf("pin writes = %v, want [n r]", got)
		}
		for _, p := range fp.Writes {
			if !p.Local || p.Sym == nil {
				t.Errorf("pin %s: Local=%v Sym=%v, want a resolved pin", p.Name, p.Local, p.Sym)
			}
		}
	}
	// Each performance holds its own pins, so same-named pins of two nodes never meet.
	if addA.Dependent(addB) {
		t.Errorf("addA and addB dependent through their own pins:\n%s\n%s", addA, addB)
	}
	// The nodes feeding one join converge on it.
	afterA, afterB := footprintNamed(t, graph, "afterA"), footprintNamed(t, graph, "afterB")
	if !afterA.Dependent(afterB) || len(afterA.Control) != 1 {
		t.Errorf("afterA and afterB do not converge on sync:\n%s\n%s", afterA, afterB)
	}
	bound := footprintNamed(t, graph, "bound")
	if !hasPlace(bound.Reads, "total") || !hasPlace(bound.Writes, "total") {
		t.Errorf("bound footprint:\n%s\nwant total read and written through the binding", bound)
	}
	if hasPlace(bound.Reads, "other") {
		t.Errorf("bound reads other:\n%s", bound)
	}
}

// A `via` send, a chain from a computed base and a nested performance are dynamic
// targets, dependent on every other move.
func TestFootprintDynamicTargets(t *testing.T) {
	graph := scopedActionGraph(t, `
		action def Dynamic {
			attribute x : Integer = 0;
			port p;
			first start;
			fork split;
			action viaSend send 1 via p;
			action writeX { assign x := 1; }
			action typed : Add;
			action nested {
				action inner { assign x := 3; }
				first start then inner;
				then done;
			}
			join sync;
			done;
			succession first start then split;
			succession first split then viaSend;
			succession first split then writeX;
			succession first split then typed;
			succession first split then nested;
			succession first viaSend then sync;
			succession first writeX then sync;
			succession first typed then sync;
			succession first nested then sync;
			succession first sync then done;
		}
		action def Add { in a : Integer; out r : Integer; }
	`, "Dynamic")

	if typed := footprintNamed(t, graph, "typed"); !typed.Dynamic {
		t.Errorf("a node performing an action is not dynamic:\n%s", typed)
	}

	viaSend := footprintNamed(t, graph, "viaSend")
	if !viaSend.Dynamic {
		t.Errorf("via send footprint:\n%s\nwant dynamic", viaSend)
	}
	if len(viaSend.Sends) != 1 || viaSend.Sends[0].Port != "p" {
		t.Errorf("via send channels = %v, want [via p]", viaSend.Sends)
	}
	writeX := footprintNamed(t, graph, "writeX")
	if !viaSend.Dependent(writeX) || !writeX.Dependent(viaSend) {
		t.Error("a dynamic move is independent of a write")
	}
	nested := footprintNamed(t, graph, "nested")
	if nested.Dynamic {
		t.Errorf("a node stating its own flow is dynamic:\n%s", nested)
	}
	sub := graph.Subflows[namedActionNode(t, graph, "nested")]
	if sub == nil || sub.Graph == nil {
		t.Fatal("nested subflow missing")
	}
	inner := sub.Graph.Footprints[namedActionNode(t, sub.Graph, "inner")]
	if !hasPlace(inner.Writes, "x") {
		t.Errorf("inner footprint:\n%s\nwant a write of x", inner)
	}
}

// A statement run inside a block's own flow keeps the footprint of its
// statements; a chained assignment resolves to the feature it reaches.
func TestFootprintBlocksAndChains(t *testing.T) {
	graph := scopedActionGraph(t, `
		part def Counter { attribute count : Integer = 0; }
		action def Blocks {
			attribute c : Counter;
			attribute go : Boolean = true;
			attribute n : Integer = 0;
			first start;
			action step {
				if go {
					assign c.count := n;
					assign n := n + 1;
				}
			}
			done;
			succession first start then step;
			succession first step then done;
		}
	`, "Blocks")

	step := footprintNamed(t, graph, "step")
	if !hasPlace(step.Reads, "go") || !hasPlace(step.Reads, "n") || !hasPlace(step.Reads, "c") {
		t.Errorf("step reads = %v, want go, n and c", placeNames(step.Reads))
	}
	if !hasPlace(step.Writes, "count") || !hasPlace(step.Writes, "n") {
		t.Errorf("step writes = %v, want count and n", placeNames(step.Writes))
	}
	if step.Dynamic {
		t.Errorf("step is dynamic:\n%s", step)
	}
}

// Dependent joins two footprints only by the clauses of the note; String renders
// each clause once, sorted.
func TestFootprintStringAndClauses(t *testing.T) {
	join := &ast.JoinNode{Name: "sync"}
	fp := Footprint{
		Reads:   []Place{{Name: "b"}, {Name: "a"}},
		Writes:  []Place{{Name: "x"}},
		Sends:   []Channel{{Signal: "Go", Port: "p"}},
		Accepts: []Channel{{}},
		Control: []ast.Node{join},
		Dynamic: true,
	}
	want := "reads: a, b\nwrites: x\nsends: Go via p\naccepts: *\ncontrol: sync\ndynamic\n"
	if got := fp.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	empty := Footprint{}
	if empty.Dependent(Footprint{Writes: []Place{{Name: "x"}}}) {
		t.Error("an empty footprint depends on a write")
	}
	if !empty.Dependent(Footprint{Dynamic: true}) {
		t.Error("an empty footprint is independent of a dynamic move")
	}
	bus := Footprint{Accepts: []Channel{{Signal: "Go"}}}
	if !bus.Dependent(Footprint{Accepts: []Channel{{}}}) || !bus.Dependent(Footprint{Sends: []Channel{{Signal: "Stop"}}}) {
		t.Error("two moves on the bus are independent")
	}
	if !(Footprint{Control: []ast.Node{join}}).Dependent(Footprint{Control: []ast.Node{join}}) {
		t.Error("two arrivals at one join are independent")
	}
	if (Footprint{Control: []ast.Node{join}}).Dependent(Footprint{Control: []ast.Node{&ast.JoinNode{Name: "sync"}}}) {
		t.Error("arrivals at different joins are dependent")
	}
}

// One declaration under two names is one place: a write through the declared
// name meets a read through its short name, and a write through a redefining
// feature meets a read through the name it redefines. Distinct declarations
// that share a name meet by name unless both are pins.
func TestFootprintAliasesMeet(t *testing.T) {
	graph := scopedActionGraph(t, `
		action def Base {
			attribute base : Integer = 0;
		}
		action def Aliased :> Base {
			attribute <sx> x : Integer = 0;
			attribute y :>> base;
			attribute seenX : Integer = -1;
			attribute seenBase : Integer = -1;
			attribute other : Integer = 0;
			first start;
			fork split;
			action wx { assign x := 1; }
			action rx { assign seenX := sx; }
			action wy { assign y := 1; }
			action rb { assign seenBase := base; }
			action wo { assign other := 1; }
			action ax;
			action arx;
			action ay;
			action ab;
			action ao;
			join sync;
			done;
			succession first start then split;
			succession first split then wx;
			succession first split then rx;
			succession first split then wy;
			succession first split then rb;
			succession first split then wo;
			succession first wx then ax;
			succession first rx then arx;
			succession first wy then ay;
			succession first rb then ab;
			succession first wo then ao;
			succession first ax then sync;
			succession first arx then sync;
			succession first ay then sync;
			succession first ab then sync;
			succession first ao then sync;
			succession first sync then done;
		}
	`, "Aliased")

	fp := func(name string) Footprint { return footprintNamed(t, graph, name) }
	cases := []struct {
		a, b      string
		dependent bool
		why       string
	}{
		{"wx", "rx", true, "a write through the name meets a read through the short name"},
		{"wy", "rb", true, "a write through the redefining feature meets a read of the redefined one"},
		{"wx", "wy", false, "distinct features under distinct names"},
		{"rx", "wo", false, "the short name resolves to no feature the other writes"},
	}
	for _, tc := range cases {
		got := fp(tc.a).Dependent(fp(tc.b))
		if got != tc.dependent {
			t.Errorf("%s ~ %s = %v, want %v (%s)\n%s\n%s", tc.a, tc.b, got, tc.dependent, tc.why, fp(tc.a), fp(tc.b))
		}
	}
	for _, p := range fp("rx").Reads {
		if p.Name == "sx" && p.Sym == nil {
			t.Error("the short name did not resolve to its declaration")
		}
	}

	x := &symbols.Symbol{Name: "x"}
	cases2 := []struct {
		p, q      Place
		conflicts bool
		why       string
	}{
		{Place{Sym: x, Name: "x"}, Place{Sym: x, Name: "sx"}, true, "one declaration under two names"},
		{Place{Sym: x, Name: "x", Local: true}, Place{Sym: x, Name: "sx", Local: true}, true, "one pin under two names"},
		{Place{Sym: x, Name: "n", Local: true}, Place{Sym: &symbols.Symbol{Name: "n"}, Name: "n", Local: true}, false, "two pins named alike"},
		{Place{Sym: x, Name: "n"}, Place{Sym: &symbols.Symbol{Name: "n"}, Name: "n"}, true, "two features named alike"},
		{Place{Name: "x"}, Place{Sym: x, Name: "x"}, true, "an unresolved name meets the resolved one"},
		{Place{Name: "x"}, Place{Sym: x, Name: "sx"}, false, "an unresolved name meets only its spelling"},
	}
	for _, tc := range cases2 {
		if got := tc.p.Conflicts(tc.q); got != tc.conflicts {
			t.Errorf("%v ~ %v = %v, want %v (%s)", tc.p, tc.q, got, tc.conflicts, tc.why)
		}
		if back := tc.q.Conflicts(tc.p); back != tc.conflicts {
			t.Errorf("%v ~ %v is not symmetric", tc.p, tc.q)
		}
	}
}
