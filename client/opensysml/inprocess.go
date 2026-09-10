package opensysml

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	sysmlgrpc "github.com/Open-MBEE/OpenSysML/internal/grpc"
)

// defaultCacheSize matches the sysml-grpc default, so the two implementations
// keep the same number of parses warm.
const defaultCacheSize = 100

// Option configures New.
type Option func(*options)

type options struct {
	cacheSize int
}

// WithCacheSize bounds how many parsed models the client keeps, evicting the
// least recently used beyond it. The default is 100; New refuses a size that
// is not positive.
func WithCacheSize(size int) Option {
	return func(o *options) { o.cacheSize = size }
}

// New is the in-process implementation: the engine linked into this binary,
// behind the same interface and semantics as the service — same cache keying,
// same capability list, same in-band failures — with no port, no child process
// and no serialization. Runtime budgets and library prewarming are read from
// the environment, as the service reads them. Close releases the standard
// library index the client holds.
func New(opts ...Option) (Client, error) {
	config := options{cacheSize: defaultCacheSize}
	for _, opt := range opts {
		opt(&config)
	}
	svc, err := sysmlgrpc.NewService(config.cacheSize, buildVersion())
	if err != nil {
		return nil, err
	}
	svc.Prewarm()
	return &client{caller: &inprocess{svc: svc}}, nil
}

// modulePath is the module buildVersion looks for among the binary's deps.
const modulePath = "github.com/Open-MBEE/OpenSysML"

// buildVersion is the version of OpenSysML linked into this binary — the
// dependency's version in an importing program, the main module's when this
// repository is the program. Informational only.
func buildVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if info.Main.Path == modulePath {
		return released(info.Main.Version)
	}
	for _, dep := range info.Deps {
		if dep.Path != modulePath {
			continue
		}
		if dep.Replace != nil {
			if version := released(dep.Replace.Version); version != "dev" {
				return version
			}
		}
		return released(dep.Version)
	}
	return "dev"
}

// released reports a module version, or "dev" for one the toolchain leaves
// unstamped: an unversioned build or a directory replacement.
func released(version string) string {
	if version == "" || version == "(devel)" {
		return "dev"
	}
	return version
}

// inprocess calls the service implementation directly. Errors cross as
// StatusError, and a panic is caught at this boundary rather than crossing it.
type inprocess struct {
	svc *sysmlgrpc.Service
}

func (p *inprocess) serverInfo(ctx context.Context) (*pb.ServerInfoResponse, error) {
	return answer(ctx, &pb.ServerInfoRequest{}, p.svc.GetServerInfo)
}

func (p *inprocess) listEngines(ctx context.Context, req *pb.ListEnginesRequest) (*pb.ListEnginesResponse, error) {
	return answer(ctx, req, p.svc.ListEngines)
}

func (p *inprocess) parseFile(ctx context.Context, req *pb.ParseFileRequest) (*pb.ParseFileResponse, error) {
	return answer(ctx, req, p.svc.ParseFile)
}

func (p *inprocess) parseSources(
	ctx context.Context,
	req *pb.ParseSourcesRequest,
) (*pb.ParseSourcesResponse, error) {
	return answer(ctx, req, p.svc.ParseSources)
}

func (p *inprocess) getSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error) {
	return answer(ctx, req, p.svc.GetSymbol)
}

func (p *inprocess) getDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.DiagnosticsResponse, error) {
	return answer(ctx, req, p.svc.GetDiagnostics)
}

func (p *inprocess) evaluate(ctx context.Context, req *pb.EvaluateRequest) (*pb.EvaluateResponse, error) {
	return answer(ctx, req, p.svc.Evaluate)
}

func (p *inprocess) instantiate(ctx context.Context, req *pb.InstantiateRequest) (*pb.InstantiateResponse, error) {
	return answer(ctx, req, p.svc.Instantiate)
}

func (p *inprocess) executeAction(
	ctx context.Context,
	req *pb.ExecuteActionRequest,
) (*pb.ExecuteActionResponse, error) {
	return answer(ctx, req, p.svc.ExecuteAction)
}

func (p *inprocess) executeState(ctx context.Context, req *pb.ExecuteStateRequest) (*pb.ExecuteStateResponse, error) {
	return answer(ctx, req, p.svc.ExecuteState)
}

func (p *inprocess) verifyConstraint(
	ctx context.Context,
	req *pb.VerifyConstraintRequest,
) (*pb.VerifyConstraintResponse, error) {
	return answer(ctx, req, p.svc.VerifyConstraint)
}

func (p *inprocess) verifyRequirement(
	ctx context.Context,
	req *pb.VerifyRequirementRequest,
) (*pb.VerifyRequirementResponse, error) {
	return answer(ctx, req, p.svc.VerifyRequirement)
}

func (p *inprocess) verifySatisfaction(
	ctx context.Context,
	req *pb.VerifySatisfactionRequest,
) (*pb.VerifySatisfactionResponse, error) {
	return answer(ctx, req, p.svc.VerifySatisfaction)
}

func (p *inprocess) evaluateCalc(ctx context.Context, req *pb.EvaluateCalcRequest) (*pb.EvaluateCalcResponse, error) {
	return answer(ctx, req, p.svc.EvaluateCalc)
}

func (p *inprocess) runAnalysis(ctx context.Context, req *pb.RunAnalysisRequest) (*pb.RunAnalysisResponse, error) {
	return answer(ctx, req, p.svc.RunAnalysis)
}

func (p *inprocess) query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	return answer(ctx, req, p.svc.Query)
}

func (p *inprocess) runDocumentQuery(
	ctx context.Context,
	req *pb.RunDocumentQueryRequest,
) (*pb.RunDocumentQueryResponse, error) {
	return answer(ctx, req, p.svc.RunDocumentQuery)
}

func (p *inprocess) renderDocument(
	ctx context.Context,
	req *pb.RenderDocumentRequest,
) (*pb.RenderDocumentResponse, error) {
	return answer(ctx, req, p.svc.RenderDocument)
}

func (p *inprocess) convert(ctx context.Context, req *pb.ConvertRequest) (*pb.ConvertResponse, error) {
	return answer(ctx, req, p.svc.Convert)
}

func (p *inprocess) applyEdits(ctx context.Context, req *pb.ApplyEditsRequest) (*pb.ApplyEditsResponse, error) {
	return answer(ctx, req, p.svc.ApplyEdits)
}

// answer runs one handler with the guarantees this boundary makes: the
// context decides whether an answer is delivered, as it does on the wire, and
// a panic becomes CodeInternal instead of crossing.
func answer[Req, Resp any](ctx context.Context, req Req, handler func(context.Context, Req) (Resp, error)) (resp Resp, err error) {
	defer recoverToError(&err)
	var zero Resp
	if err := contextStatus(ctx); err != nil {
		return zero, err
	}
	resp, callErr := handler(ctx, req)
	// The engine runs a call to completion; a done context withholds whatever it
	// produced, answer or error, as the wire does.
	if err := contextStatus(ctx); err != nil {
		return zero, err
	}
	if callErr != nil {
		return zero, statusToError(callErr)
	}
	return resp, nil
}

// contextStatus is the status a done context produces, matching the code a
// remote caller sees for the same context.
func contextStatus(ctx context.Context) error {
	switch err := ctx.Err(); {
	case err == nil:
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return &StatusError{Code: CodeDeadlineExceeded, Message: err.Error()}
	default:
		return &StatusError{Code: CodeCanceled, Message: err.Error()}
	}
}

func (p *inprocess) close() error {
	p.svc.Close()
	return nil
}

// statusToError is the documented status mapping: the status a handler refuses
// with becomes a StatusError carrying the same code. Connect error codes number
// the same as the canonical gRPC status codes.
func statusToError(err error) error {
	if err == nil {
		return nil
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		return &StatusError{Code: Code(ce.Code()), Message: ce.Message()}
	}
	return &StatusError{Code: CodeUnknown, Message: err.Error()}
}

// recoverToError keeps a panic from crossing the public boundary: it becomes
// CodeInternal, the status the wire would answer for a crashed handler.
func recoverToError(err *error) {
	if recovered := recover(); recovered != nil {
		*err = &StatusError{Code: CodeInternal, Message: fmt.Sprintf("panic: %v", recovered)}
	}
}
