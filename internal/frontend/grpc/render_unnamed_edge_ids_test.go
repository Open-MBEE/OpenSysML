package grpc

import (
	"context"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

// unnamedEdgeIDsModel has several unnamed relationships under one owner in an
// action and a state machine: the index names them all after that owner alone.
const unnamedEdgeIDsModel = `package Beh {
	private import Actions::*;
	private import States::*;
	item def Go;
	item def Ping;
	action def Drive {
		action a; action b; action c; action d;
		then a; then b; then c; then d;
	}
	part def Modes {
		exhibit state states {
			entry; then idle;
			state idle; state done;
			transition idle accept Go then done;
			transition idle accept Ping then done;
		}
	}
}
`

// TestRenderViewUnnamedEdgesHaveDistinctResolvableIDs pins a resolvable id per
// unnamed edge, which a shared owner id could not give a canvas or lookup.
func TestRenderViewUnnamedEdgesHaveDistinctResolvableIDs(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	parsed, _ := parseContent(t, srv, unnamedEdgeIDsModel)

	for _, view := range []string{"#action:Beh::Drive", "#state:Beh::Modes::states"} {
		t.Run(view, func(t *testing.T) {
			rendered, err := srv.RenderView(context.Background(), &pb.RenderViewRequest{
				ModelHash: parsed.ModelHash, View: view,
			})
			if err != nil {
				t.Fatalf("RenderView: %v", err)
			}
			if len(rendered.Edges) < 2 {
				t.Fatalf("rendered %d edges, want at least two unnamed ones", len(rendered.Edges))
			}
			seen := make(map[string]bool)
			for _, edge := range rendered.Edges {
				if edge.ElementId == "" {
					t.Fatalf("edge of kind %q has no element id", edge.Kind)
				}
				if seen[edge.ElementId] {
					t.Fatalf("two unnamed edges share the element id %q", edge.ElementId)
				}
				seen[edge.ElementId] = true

				props, err := srv.GetElementProperties(context.Background(), &pb.GetElementPropertiesRequest{
					ModelHash: parsed.ModelHash, ElementId: edge.ElementId, Properties: []string{"name"},
				})
				if err != nil {
					t.Fatalf("GetElementProperties(%s): %v", edge.ElementId, err)
				}
				if props.Error != "" {
					t.Errorf("GetElementProperties(%s): %s", edge.ElementId, props.Error)
				} else if props.Element == nil {
					t.Errorf("GetElementProperties(%s) answered no element", edge.ElementId)
				}
			}
		})
	}
}
