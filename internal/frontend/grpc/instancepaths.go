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
	found, truncated := pathsFromRoots(held.rt, held.rootObjects(), target, protoconv.DefaultGraphBounds().Depth, maxInstancePaths)
	resp := &pb.InstancePathsResponse{Truncated: truncated}
	for _, rp := range found {
		pbRoot := &pb.InstanceRoot{InstanceId: rp.root.ID}
		if label, named := labels[rp.root.ID]; named && label[0] != '#' {
			pbRoot.SymbolId = label
		}
		resp.Paths = append(resp.Paths, &pb.InstancePath{
			Root:  pbRoot,
			Path:  runtime.PathText(rp.steps),
			Steps: protoconv.PathStepsToProto(rp.steps, cached.Index),
		})
	}
	return resp, nil
}

// rootPath is one path to an object, with the root it starts from.
type rootPath struct {
	root  *runtime.Instance
	steps []runtime.PathStep
}

// pathsFromRoots lists the paths to target from each root in turn, at most
// limit in all: once the limit is spent, a later root that still reaches the
// target makes the answer truncated rather than longer.
func pathsFromRoots(rt *runtime.Context, roots []*runtime.Instance, target *runtime.Instance, depth, limit int) ([]rootPath, bool) {
	var paths []rootPath
	truncated := false
	for _, root := range roots {
		remaining := limit - len(paths)
		if remaining <= 0 {
			more := rt.PathsTo(root, target, runtime.PathBounds{Depth: depth, Paths: 1})
			if len(more.Steps) > 0 || more.Truncated {
				truncated = true
				break
			}
			continue
		}
		found := rt.PathsTo(root, target, runtime.PathBounds{Depth: depth, Paths: remaining})
		truncated = truncated || found.Truncated
		for _, steps := range found.Steps {
			paths = append(paths, rootPath{root: root, steps: steps})
		}
	}
	return paths, truncated
}
