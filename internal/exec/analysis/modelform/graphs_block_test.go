package modelform

import (
	"errors"
	"slices"
	"testing"
)

// A node a block of a body declares is a node of the block's flow: the graph
// numbers it, names it and resolves what it performs, and holds nothing else of it.
func TestGraphsNumberTheNodesABlockDeclares(t *testing.T) {
	model, idx := graphsModelOf(t, `package test {
	private import ScalarValues::*;
	action def Work { }
	action run {
		attribute n : Integer = 1;
		first start;
		action choose {
			if n > 0 { action inner : Work; }
		}
		done;
		succession first start then choose;
		succession first choose then done;
	}
}
`)
	g, _ := exportGraphs(t, model, idx, "test::run")
	var run *ActionForm
	for _, a := range g.Actions {
		if a.Name == "test::run" {
			run = a
		}
	}
	if run == nil {
		t.Fatalf("no graph of test::run among %d", len(g.Actions))
	}
	var choose, inner *NodeForm
	for i := range run.Nodes {
		switch run.Nodes[i].Name {
		case "choose":
			choose = &run.Nodes[i]
		case "inner":
			inner = &run.Nodes[i]
		}
	}
	if choose == nil || inner == nil {
		t.Fatalf("nodes choose %v, inner %v: want both", choose, inner)
	}
	if !slices.Equal(choose.Block, []int{inner.ID}) {
		t.Errorf("choose.Block = %v, want [%d]", choose.Block, inner.ID)
	}
	if inner.Kind != "action usage" || !slices.Equal(inner.Performs, []string{"test::Work"}) {
		t.Errorf("inner is %q performing %v, want an action usage performing test::Work", inner.Kind, inner.Performs)
	}
	if inner.Span.Len == 0 || inner.Span.Document != "graphs.sysml" {
		t.Errorf("inner.Span = %+v, want its place in graphs.sysml", inner.Span)
	}
	if inner.Scope != "" || inner.Features != nil || inner.Body != nil || inner.Footprint != nil || inner.Subflow != nil {
		t.Errorf("inner carries %+v, want its identity alone", *inner)
	}
	if !slices.ContainsFunc(g.Actions, func(a *ActionForm) bool { return a.Name == "test::Work" }) {
		t.Errorf("test::Work, which inner performs, is not exported")
	}
}

// A block flow whose fork and join are its own converges on nodes the enclosing
// graph does not number, so the graph is refused rather than written without them.
func TestGraphsRefuseABlockFlowConvergingOnItsOwnNodes(t *testing.T) {
	model, idx := graphsModelOf(t, `package test {
	private import ScalarValues::*;
	action run {
		attribute total : Integer = 0;
		first start;
		action iterate {
			for i in 1..2 {
				action b { out v : Integer; assign v := i * 10; }
				action a { out v : Integer; assign v := i; }
				first f;
				fork f;
				succession f then a;
				succession f then b;
				join j;
				succession a then j;
				succession b then j;
				succession j then c;
				action c { assign total := total + a.v + b.v; }
			}
		}
		done;
		succession first start then iterate;
		succession first iterate then done;
	}
}
`)
	_, err := GraphsOf(model, lookupOne(t, idx, "test::run"))
	if !errors.Is(err, ErrGraphsOrder) {
		t.Fatalf("GraphsOf(test::run) = %v, want ErrGraphsOrder", err)
	}
	const want = "test::run: graphs: elements of one graph cannot be ordered: a vertex is reached only while it is written"
	if err.Error() != want {
		t.Errorf("GraphsOf(test::run) = %q, want %q", err, want)
	}
}
