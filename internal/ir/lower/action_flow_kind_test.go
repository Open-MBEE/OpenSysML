package lower

import "testing"

// A plain `flow` lowers as a streaming flow, which orders nothing: only a
// `succession flow` carries its kind into a succession edge from source to target.
func TestObjectFlowKindFollowsTheSpelling(t *testing.T) {
	for _, tc := range []struct {
		spelling string
		kind     FlowKind
		edges    int
	}{
		{"flow", FlowStreaming, 0},
		{"succession flow", FlowSuccession, 1},
	} {
		t.Run(tc.spelling, func(t *testing.T) {
			graph := actionGraphFor(t, `
				action test {
					action src { out v : Integer; }
					action dst { in v : Integer; }
					`+tc.spelling+` src.v to dst.v;
				}
			`)
			var src, dst = graph.Nodes[0], graph.Nodes[1]
			flows := graph.DataFlows[src]
			if len(flows) != 1 || flows[0].Kind != tc.kind || flows[0].Target != dst {
				t.Fatalf("DataFlows[src] = %#v, want one %s to dst", flows, tc.kind)
			}
			if got := len(graph.Edges[src]); got != tc.edges {
				t.Errorf("src has %d successions, want %d for a %s", got, tc.edges, tc.kind)
			}
		})
	}
}
