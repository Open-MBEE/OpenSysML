package grpc

import (
	"context"
	"sort"

	"connectrpc.com/connect"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/check/passes"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/resolve"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/selfcheck"
)

// SelfCheck applies the SysMLValidation constraints, and those of the rule
// packages named, reflectively to every element of a parsed model.
func (s *Service) SelfCheck(ctx context.Context, req *pb.SelfCheckRequest) (*pb.SelfCheckResponse, error) {
	if err := s.requireCapability(CapabilitySelfCheck); err != nil {
		return nil, err
	}
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, "model %s is no longer cached: parse it again before checking it", req.ModelHash)
	}
	groups, err := selfcheck.ResolvePackages(cached.Index, req.SelfCheckPackage)
	if err != nil {
		return &pb.SelfCheckResponse{Error: err.Error()}, nil
	}
	sources := make(map[string]*source.SourceFile, len(cached.Documents))
	for _, doc := range cached.Documents {
		sources[doc.Source.Name()] = doc.Source
	}
	// The check memoizes into the semantics it reads, so each request types the
	// model afresh rather than sharing the symbol context's, as Semantics does.
	resolver := resolve.New(cached.Index)
	sem := passes.NewTypedModel(resolver)
	sem.SetSourceText(cachedSourceText(cached))
	var release func()
	report := selfcheck.Run(selfcheck.Input{
		Index:   cached.Index,
		Model:   sem,
		Sources: sources,
		Runtime: func() (*runtime.Context, error) {
			rt, rel := s.newRuntime(ctx, cached)
			release = rel
			return rt, nil
		},
		Package:      "SysMLValidation",
		RulePackages: groups,
	})
	if release != nil {
		release()
	}
	if err := ctx.Err(); err != nil {
		return nil, statusError(connect.CodeCanceled, err.Error())
	}
	return selfCheckResponse(report), nil
}

func selfCheckResponse(report selfcheck.Report) *pb.SelfCheckResponse {
	counts := report.Counts
	resp := &pb.SelfCheckResponse{
		Status:           "holds",
		Elements:         int64(counts.Elements),
		Checks:           int64(counts.Checks),
		Violations:       int64(counts.Violations),
		Unevaluated:      int64(counts.Unevaluated),
		EvaluationErrors: int64(counts.EvaluationErrors),
		Summary:          counts.Summary(),
	}
	switch {
	case counts.Violations > 0:
		resp.Status = "fails"
	case counts.EvaluationErrors > 0:
		resp.Status = "unresolved"
	}
	for _, f := range report.Findings {
		finding := &pb.SelfCheckFinding{
			Status:     f.Status.String(),
			Subject:    f.Subject,
			Location:   f.Location,
			Constraint: f.Constraint,
			Element:    f.Element,
			Message:    f.Message,
			Line:       f.Line(),
		}
		if f.Err != nil {
			finding.Message = f.Err.Error()
		}
		resp.Findings = append(resp.Findings, finding)
	}
	names := make([]string, 0, len(counts.Applications))
	for name := range counts.Applications {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		resp.Applications = append(resp.Applications, &pb.SelfCheckApplication{
			Constraint: name, Count: int64(counts.Applications[name]),
		})
	}
	return resp
}
