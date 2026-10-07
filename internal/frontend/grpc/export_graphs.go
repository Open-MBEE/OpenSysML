package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/modelform"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
)

// ExportGraphs exports the lowered graph of an action or state machine, and of
// every behavior it performs, as the canonical `graphs:1` JSON an external
// engine is sent. An unknown subject is NOT_FOUND; one that is no behavior, or
// a name several elements share, is INVALID_ARGUMENT.
func (s *Service) ExportGraphs(ctx context.Context, req *pb.ExportGraphsRequest) (*pb.ExportGraphsResponse, error) {
	if err := s.requireCapability(CapabilityExportGraphs); err != nil {
		return nil, err
	}
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}
	if req.Subject == "" {
		return nil, statusError(connect.CodeInvalidArgument, "subject is required")
	}
	found := symbolfacts.LookupNamed(cached.Index, req.Subject)
	if len(found) == 0 {
		return nil, statusErrorf(connect.CodeNotFound, "no element named %s", req.Subject)
	}

	worker, release := cached.worker()
	defer release()
	graphs, err := modelform.GraphsOf(worker.Model, found[0])
	if err != nil {
		if errors.Is(err, modelform.ErrGraphsSubject) {
			return nil, statusErrorf(connect.CodeInvalidArgument, "%s: %s", req.Subject, err)
		}
		return nil, statusErrorf(connect.CodeFailedPrecondition, "%s: %s", req.Subject, err)
	}
	raw, err := modelform.MarshalGraphs(graphs)
	if err != nil {
		return nil, statusError(connect.CodeInternal, err.Error())
	}
	return &pb.ExportGraphsResponse{
		Content: string(raw),
		Version: int32(graphs.Version), // #nosec G115 -- the form version is a small constant
		Subject: graphs.Subject,
	}, nil
}
