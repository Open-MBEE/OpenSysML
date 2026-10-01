// Package instancegraph registers %features json: an object graph written as
// the API's Instantiate response, so a client reads one shape whether it asked
// the service or the REPL.
package instancegraph

import (
	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/repl/replext"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

func init() { replext.RegisterGraph(writer{}) }

type writer struct{}

// DefaultInstances is the count the API serializes for one object.
func (writer) DefaultInstances() int { return protoconv.DefaultGraphBounds().Instances }

func (writer) Write(ctx *runtime.Context, inst *runtime.Instance, index *symbols.Index, depth, instances int, warning func() string) ([]byte, []error, error) {
	bounds := protoconv.GraphBounds{Depth: depth, Instances: instances}
	graph := protoconv.InstanceGraphToProtoWithin(ctx, inst, index, bounds)
	resp := &pb.InstantiateResponse{Instance: graph.Root, Instances: graph.All}
	if graph.Truncated {
		resp.Diagnostics = append(resp.Diagnostics, &pb.Diagnostic{Severity: "warning", Message: warning()})
	}
	out, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(resp)
	return out, graph.Errors, err
}
