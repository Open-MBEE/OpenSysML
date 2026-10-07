package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/analysis/modelform"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
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
	subject, err := graphsSubject(cached.Index, req.Subject)
	if err != nil {
		return nil, err
	}

	worker, release := cached.worker()
	defer release()
	graphs, err := modelform.GraphsOf(worker.Model, subject)
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

// graphsSubject is the one declaration a qualified name denotes. A declaration
// of the model's shadows the library's of the same name, as every RPC reads a
// name; a name the model declares more than once denotes none of them.
func graphsSubject(idx *symbols.Index, name string) (*symbols.Symbol, error) {
	found := symbolfacts.LookupNamed(idx, name)
	if len(found) == 0 {
		return nil, statusErrorf(connect.CodeNotFound, "symbol not found: %s", name)
	}
	// LookupNamed puts the model's declarations before the library's.
	declared := 0
	for declared < len(found) && !idx.Library(found[declared]) {
		declared++
	}
	switch {
	case declared == 1, declared == 0 && len(found) == 1:
		return found[0], nil
	case declared == 0:
		return nil, statusErrorf(connect.CodeInvalidArgument, "%s is ambiguous: the library declares %d elements under that name", name, len(found))
	}
	return nil, statusErrorf(connect.CodeInvalidArgument, "%s is ambiguous: the model declares %d elements under that name", name, declared)
}
