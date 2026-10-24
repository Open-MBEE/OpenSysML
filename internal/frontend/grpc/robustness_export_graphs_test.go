package grpc

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/modelform"
)

// TestGRPCRobustnessExportGraphs exports machines whose lowered graph carries
// vertices the form must handle without a declaring parent: the region owners
// of a parallel state definition, asked for as the definition and as a state
// usage exhibiting it. Each answers the form, never a panic.
func TestGRPCRobustnessExportGraphs(t *testing.T) {
	srv := mustNewService(t, 10)
	t.Cleanup(srv.Close)
	content, err := os.ReadFile(filepath.Join("..", "..", "exec", "analysis", "modelform", "testdata", "parallel_machine.sysml"))
	if err != nil {
		t.Fatal(err)
	}
	hash := mustParse(t, srv, string(content))

	for name, subject := range map[string]string{
		"parallel_definition":         "Orthogonal::Machine",
		"usage_exhibiting_a_parallel": "Orthogonal::Tug::modes",
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := srv.ExportGraphs(context.Background(), &pb.ExportGraphsRequest{ModelHash: hash, Subject: subject})
			if err != nil {
				t.Fatalf("ExportGraphs %s: %v", subject, err)
			}
			var graphs modelform.Graphs
			if err := json.Unmarshal([]byte(resp.Content), &graphs); err != nil {
				t.Fatalf("%s: content is not graphs JSON: %v", subject, err)
			}
			if len(graphs.States) != 1 || len(graphs.States[0].Regions) != 2 {
				t.Fatalf("%s: %d state machines, want one with two regions: %s", subject, len(graphs.States), resp.Content)
			}
			for _, v := range graphs.States[0].Vertices {
				if v.Name == "" {
					t.Errorf("%s: vertex %d (%s) has no name", subject, v.ID, v.Kind)
				}
			}
		})
	}
}
