package lower

import (
	"slices"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/syntax/ast"
)

// A node's parameters and attributes are lowered as features of the node, so
// each performance of it holds them apart from the enclosing action's features.
func TestActionNodeDeclaresItsOwnFeatures(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute total : Integer = 0;
			first start;
			then action p {
				in a : Integer = 3;
				out v : Integer;
				attribute k : Integer = 1;
				action inner;
				assign v := a;
			}
			then done;
		}
	`)

	p := nodeNamed(t, graph, "p")
	features := graph.Features[p]
	if len(features) != 3 {
		t.Fatalf("p declares %d features, want 3: %+v", len(features), features)
	}
	want := []struct {
		name      string
		direction ast.FeatureDirection
		valued    bool
	}{
		{"a", ast.DirIn, true},
		{"v", ast.DirOut, false},
		{"k", ast.DirNone, true},
	}
	for i, w := range want {
		got := features[i]
		if got.Name != w.name || got.Direction != w.direction || (got.Value != nil) != w.valued {
			t.Errorf("feature %d = %s/%v valued=%v, want %s/%v valued=%v",
				i, got.Name, got.Direction, got.Value != nil, w.name, w.direction, w.valued)
		}
		if got.Node == nil {
			t.Errorf("feature %s carries no declaration", got.Name)
		}
	}
	if len(graph.Bodies[p]) != 1 {
		t.Errorf("p lowered %d statements, want 1", len(graph.Bodies[p]))
	}
}

// An accept's message payload is the node's output pin; a trigger (`accept when`,
// `accept after`) names a condition or a time, not a value the node produces.
func TestAcceptNodePayloadIsItsOutputPin(t *testing.T) {
	graph := actionGraphFor(t, `
		attribute def Ping { attribute level : Integer; }
		action test {
			attribute ready : Boolean = true;
			first start;
			then action hear accept msg : Ping;
			then action wait accept timer when ready;
			then action pause accept delay after 2;
			then done;
		}
	`)

	hear := graph.Features[nodeNamed(t, graph, "hear")]
	if len(hear) != 1 || hear[0].Name != "msg" || hear[0].Direction != ast.DirOut {
		t.Errorf("hear declares features %+v, want its payload pin msg", hear)
	}
	for _, name := range []string{"wait", "pause"} {
		if got := graph.Features[nodeNamed(t, graph, name)]; len(got) != 0 {
			t.Errorf("%s declares features %+v, want none for a trigger", name, got)
		}
	}
}

// A binding connector at a node's pin is lowered to the node and pin it
// addresses, with the other end kept as written; one with a node pin at each end
// is lowered once per end.
func TestActionBindingAtANodePin(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute x : Integer = 5;
			out attribute total : Integer;
			bind add.a = x;
			bind total = add.sum;
			bind add.sum = twice.n;
			bind total = x;
			first start;
			then action add { in a : Integer; out sum : Integer; }
			then action twice { in n : Integer; }
			then done;
		}
	`)

	add := nodeNamed(t, graph, "add")
	twice := nodeNamed(t, graph, "twice")
	if len(graph.Bindings) != 4 {
		t.Fatalf("lowered %d pin bindings, want 4: %+v", len(graph.Bindings), graph.Bindings)
	}
	want := []struct {
		node      ast.Node
		pin       string
		other     string
		otherNode ast.Node
		otherPin  string
	}{
		{add, "a", "x", nil, ""},
		{add, "sum", "total", nil, ""},
		{add, "sum", "twice.n", twice, "n"},
		{twice, "n", "add.sum", add, "sum"},
	}
	for i, w := range want {
		got := graph.Bindings[i]
		if got.Node != w.node || got.Pin != w.pin || FeaturePath(got.Other) != w.other ||
			got.OtherNode != w.otherNode || got.OtherPin != w.otherPin {
			t.Errorf("binding %d = %s.%s = %s (other node %v.%s), want %s = %s (other node %v.%s)",
				i, getNodeName(got.Node), got.Pin, FeaturePath(got.Other), got.OtherNode, got.OtherPin,
				w.pin, w.other, w.otherNode, w.otherPin)
		}
		if got.Decl == nil {
			t.Errorf("binding %d carries no declaration", i)
		}
	}
}

// A binding end that chains through an object (`holder.inner.mark`) carries the chain
// walked and the feature written, so the runtime writes the object it reaches; a plain
// name or a node's pin carries none.
func TestActionBindingAtANodePinThroughAChain(t *testing.T) {
	graph := actionGraphFor(t, `
		part def Cell { attribute mark : Integer; }
		part def Holder { attribute value : Integer; part inner : Cell; }
		action test {
			part holder : Holder;
			attribute x : Integer = 5;
			bind add.sum = holder.value;
			bind add.a = holder.inner.mark;
			bind add.sum = twice.n;
			bind add.a = x;
			first start;
			then action add { in a : Integer; out sum : Integer; }
			then action twice { in n : Integer; }
			then done;
		}
	`)

	if len(graph.Bindings) != 5 {
		t.Fatalf("lowered %d pin bindings, want 5: %+v", len(graph.Bindings), graph.Bindings)
	}
	want := []struct {
		pin     string
		base    string
		steps   []string
		feature string
	}{
		{"sum", "holder", nil, "value"},
		{"a", "holder", []string{"inner"}, "mark"},
		{"sum", "", nil, ""},
		{"n", "", nil, ""},
		{"a", "", nil, ""},
	}
	for i, w := range want {
		got := graph.Bindings[i]
		if got.Pin != w.pin {
			t.Errorf("binding %d is at pin %s, want %s", i, got.Pin, w.pin)
		}
		if w.base == "" {
			if got.OtherChain != nil || got.OtherFeature != "" {
				t.Errorf("binding %d = %s carries chain %+v.%s, want none", i, FeaturePath(got.Other), got.OtherChain, got.OtherFeature)
			}
			continue
		}
		if got.OtherChain == nil || FeaturePath(got.OtherChain.Base) != w.base ||
			!slices.Equal(got.OtherChain.Steps, w.steps) || got.OtherFeature != w.feature {
			t.Errorf("binding %d = %s carries chain %+v.%s, want %s.%v.%s",
				i, FeaturePath(got.Other), got.OtherChain, got.OtherFeature, w.base, w.steps, w.feature)
		}
	}
}

// A binding end naming a node without a pin identifies no feature to bind.
func TestActionBindingAtANodeWithoutAPinIsReported(t *testing.T) {
	_, err := actionGraphErr(t, `
		action test {
			attribute x : Integer = 5;
			bind add = x;
			first start;
			then action add { in a : Integer; }
			then done;
		}
	`)
	if err == nil {
		t.Fatal("binding a node itself lowered without error")
	}
}

// A binding end reaching into a node's own flow (`leg.inner.v`) keeps the whole path:
// the node of this flow, the nodes reached through it, and the pin of the last.
func TestActionBindingAtANestedNodePin(t *testing.T) {
	graph := actionGraphFor(t, `
		action test {
			attribute x : Integer = 5;
			out attribute seen : Integer;
			bind leg.inner.w = x;
			bind seen = leg.inner.v;
			bind leg.inner.v = leg.rest.n;
			first start;
			then action leg {
				out v : Integer;
				first start;
				then action inner { in w : Integer; out v : Integer; }
				then action rest { in n : Integer; }
				then done;
			}
			then done;
		}
	`)

	leg := nodeNamed(t, graph, "leg")
	sub := graph.Subflows[leg]
	if sub == nil || sub.Graph == nil {
		t.Fatalf("leg owns no flow: %+v", sub)
	}
	inner := nodeNamed(t, sub.Graph, "inner")
	rest := nodeNamed(t, sub.Graph, "rest")
	if len(graph.Bindings) != 4 {
		t.Fatalf("lowered %d pin bindings, want 4: %+v", len(graph.Bindings), graph.Bindings)
	}
	want := []struct {
		path      []ast.Node
		pin       string
		other     string
		otherPath []ast.Node
		otherPin  string
	}{
		{[]ast.Node{inner}, "w", "x", nil, ""},
		{[]ast.Node{inner}, "v", "seen", nil, ""},
		{[]ast.Node{inner}, "v", "leg.rest.n", []ast.Node{rest}, "n"},
		{[]ast.Node{rest}, "n", "leg.inner.v", []ast.Node{inner}, "v"},
	}
	for i, w := range want {
		got := graph.Bindings[i]
		if got.Node != leg || !slices.Equal(got.Path, w.path) || got.Pin != w.pin || FeaturePath(got.Other) != w.other {
			t.Errorf("binding %d = %s.%v.%s = %s, want leg.%v.%s = %s",
				i, getNodeName(got.Node), got.Path, got.Pin, FeaturePath(got.Other), w.path, w.pin, w.other)
		}
		if w.otherPath == nil {
			if got.OtherNode != nil {
				t.Errorf("binding %d other end addresses node %v, want none", i, got.OtherNode)
			}
			continue
		}
		if got.OtherNode != leg || !slices.Equal(got.OtherPath, w.otherPath) || got.OtherPin != w.otherPin {
			t.Errorf("binding %d other end = %v.%v.%s, want leg.%v.%s", i, got.OtherNode, got.OtherPath, got.OtherPin, w.otherPath, w.otherPin)
		}
	}
}

// A binding end reaching through a node that declares no such nested action, or that
// performs another action (whose nodes are that action's own), is reported when lowered.
func TestActionBindingReachingIntoANodeWithoutTheNestedNodeIsReported(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"undeclared", `
			action test {
				attribute x : Integer = 5;
				bind leg.nope.w = x;
				first start;
				then action leg { first start; then action inner { in w : Integer; } then done; }
				then done;
			}
		`, `binding end "leg.nope.w" reaches into leg, which declares no nested action nope; bind at a pin of leg itself`},
		{"typed", `
			action def Leg { first start; then action inner { in w : Integer; } then done; }
			action test {
				attribute x : Integer = 5;
				bind leg.inner.w = x;
				first start;
				then action leg : Leg;
				then done;
			}
		`, `binding end "leg.inner.w" reaches into leg, which performs an action of its own rather than declaring inner; bind at a pin of leg itself`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := actionGraphErr(t, tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

// A flow joins pins of the nodes of one flow; an end reaching into a node's own flow is reported.
func TestActionFlowReachingIntoANodesOwnFlowIsReported(t *testing.T) {
	_, err := actionGraphErr(t, `
		action test {
			first start;
			then action leg { first start; then action inner { out v : Integer; } then done; }
			then action q { in n : Integer; }
			then done;
			flow leg.inner.v to q.n;
		}
	`)
	want := `end "leg.inner.v" reaches into a node's own flow`
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want %q", err, want)
	}
}
