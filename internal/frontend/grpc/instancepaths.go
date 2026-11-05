package grpc

import (
	"context"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/protoconv"
)

// maxInstancePaths bounds the paths InstancePaths lists for one object; past
// it the response says it is truncated.
const maxInstancePaths = 1000

// InstancePaths lists every root-relative path that reaches the object the
// request names, from each root the model's population holds, in traversal
// order (see runtime.Context.PathsTo).
func (s *Service) InstancePaths(ctx context.Context, req *pb.InstancePathsRequest) (*pb.InstancePathsResponse, error) {
	if err := s.requireCapability(CapabilityInstancePaths); err != nil {
		return nil, err
	}
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}
	object := req.GetObject()
	if object == nil {
		object = &pb.DocumentObject{}
	}
	held := s.objects(cached)
	defer held.lock(ctx)()
	target, _, err := held.resolve("object", object)
	if err != nil {
		return nil, err
	}

	labels := make(map[int64]string)
	for _, root := range held.roots() {
		labels[root.Object.ID] = root.Label
	}
	bounds := runtime.PathBounds{Depth: protoconv.DefaultGraphBounds().Depth, Paths: maxInstancePaths}
	resp := &pb.InstancePathsResponse{}
	for _, root := range held.rootObjects() {
		bounds.Paths = maxInstancePaths - len(resp.Paths)
		found := held.rt.PathsTo(root, target, bounds)
		resp.Truncated = resp.Truncated || found.Truncated
		pbRoot := &pb.InstanceRoot{InstanceId: root.ID}
		if label, named := labels[root.ID]; named && label[0] != '#' {
			pbRoot.SymbolId = label
		}
		for _, steps := range found.Steps {
			resp.Paths = append(resp.Paths, &pb.InstancePath{
				Root:  pbRoot,
				Path:  runtime.PathText(steps),
				Steps: protoconv.PathStepsToProto(steps, cached.Index),
			})
		}
	}
	return resp, nil
}
