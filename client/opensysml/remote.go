package opensysml

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
)

// DialOption configures Dial.
type DialOption func(*dialOptions)

type dialOptions struct {
	httpClient *http.Client
	json       bool
	token      string
}

// TokenEnv is the environment variable Dial reads a bearer token from when
// WithToken names none; the Python, Node, Java and Rust clients read the same.
const TokenEnv = "OPENSYSML_GRPC_TOKEN" // #nosec G101 -- a variable name, not a credential

// WithToken sends the bearer token a sysml-grpc serving an address other
// machines reach requires, as Authorization: Bearer TOKEN on every call.
// Without it Dial uses $OPENSYSML_GRPC_TOKEN when that is set.
func WithToken(token string) DialOption {
	return func(o *dialOptions) { o.token = token }
}

// WithHTTPClient names the HTTP client the connection uses, for callers with
// their own transport, TLS or timeout policy. The default is
// http.DefaultClient.
func WithHTTPClient(httpClient *http.Client) DialOption {
	return func(o *dialOptions) { o.httpClient = httpClient }
}

// WithJSONBody sends requests as Connect JSON instead of protobuf. JSON is a
// debugging affordance — legible in curl and proxies — and costs an order of
// magnitude in encoding time on large responses, which is why protobuf is the
// default.
func WithJSONBody() DialOption {
	return func(o *dialOptions) { o.json = true }
}

// Dial is the remote implementation: the same interface as New, answered by a
// service someone else runs. The address is explicit — this package never
// starts a service — and names a sysml-grpc serving the Connect transport
// ("localhost:50051" or "https://sysml.example.com"); an address without a
// scheme is dialed as http. Requests cross as Connect protobuf unless
// WithJSONBody says otherwise. Closing leaves the service running: it was not
// this package's to stop.
func Dial(address string, opts ...DialOption) (Client, error) {
	if strings.TrimSpace(address) == "" {
		return nil, errors.New("opensysml: Dial needs an address")
	}
	var config dialOptions
	for _, opt := range opts {
		opt(&config)
	}
	if config.httpClient == nil {
		config.httpClient = http.DefaultClient
	}
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	var connectOpts []connect.ClientOption
	if config.json {
		connectOpts = append(connectOpts, connect.WithProtoJSON())
	}
	if config.token == "" {
		config.token = os.Getenv(TokenEnv)
	}
	if config.token != "" {
		connectOpts = append(connectOpts, connect.WithInterceptors(bearer(config.token)))
	}
	return &client{caller: &remote{
		rpc: protoconnect.NewSysMLServiceClient(config.httpClient, address, connectOpts...),
	}}, nil
}

// bearer carries the token on every request a connection sends.
type bearer string

func (b bearer) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set("Authorization", "Bearer "+string(b))
		return next(ctx, req)
	}
}

func (b bearer) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("Authorization", "Bearer "+string(b))
		return conn
	}
}

func (b bearer) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

// remote answers through a Connect client. Errors cross as StatusError with
// the equivalent code, so a refusal reads the same as in process.
type remote struct {
	rpc protoconnect.SysMLServiceClient
}

func (r *remote) serverInfo(ctx context.Context) (*pb.ServerInfoResponse, error) {
	resp, err := r.rpc.GetServerInfo(ctx, connect.NewRequest(&pb.ServerInfoRequest{}))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) listEngines(ctx context.Context, req *pb.ListEnginesRequest) (*pb.ListEnginesResponse, error) {
	resp, err := r.rpc.ListEngines(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) parseFile(ctx context.Context, req *pb.ParseFileRequest) (*pb.ParseFileResponse, error) {
	resp, err := r.rpc.ParseFile(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) parseSources(ctx context.Context, req *pb.ParseSourcesRequest) (*pb.ParseSourcesResponse, error) {
	resp, err := r.rpc.ParseSources(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) getSymbol(ctx context.Context, req *pb.GetSymbolRequest) (*pb.SymbolResponse, error) {
	resp, err := r.rpc.GetSymbol(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) getDiagnostics(ctx context.Context, req *pb.DiagnosticsRequest) (*pb.DiagnosticsResponse, error) {
	resp, err := r.rpc.GetDiagnostics(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) evaluate(ctx context.Context, req *pb.EvaluateRequest) (*pb.EvaluateResponse, error) {
	resp, err := r.rpc.Evaluate(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) instantiate(ctx context.Context, req *pb.InstantiateRequest) (*pb.InstantiateResponse, error) {
	resp, err := r.rpc.Instantiate(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) executeAction(ctx context.Context, req *pb.ExecuteActionRequest) (*pb.ExecuteActionResponse, error) {
	resp, err := r.rpc.ExecuteAction(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) executeState(ctx context.Context, req *pb.ExecuteStateRequest) (*pb.ExecuteStateResponse, error) {
	resp, err := r.rpc.ExecuteState(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) verifyConstraint(
	ctx context.Context,
	req *pb.VerifyConstraintRequest,
) (*pb.VerifyConstraintResponse, error) {
	resp, err := r.rpc.VerifyConstraint(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) verifyRequirement(
	ctx context.Context,
	req *pb.VerifyRequirementRequest,
) (*pb.VerifyRequirementResponse, error) {
	resp, err := r.rpc.VerifyRequirement(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) verifySatisfaction(
	ctx context.Context,
	req *pb.VerifySatisfactionRequest,
) (*pb.VerifySatisfactionResponse, error) {
	resp, err := r.rpc.VerifySatisfaction(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) validateInstance(
	ctx context.Context,
	req *pb.ValidateInstanceRequest,
) (*pb.ValidateInstanceResponse, error) {
	resp, err := r.rpc.ValidateInstance(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) evaluateCalc(ctx context.Context, req *pb.EvaluateCalcRequest) (*pb.EvaluateCalcResponse, error) {
	resp, err := r.rpc.EvaluateCalc(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) runAnalysis(ctx context.Context, req *pb.RunAnalysisRequest) (*pb.RunAnalysisResponse, error) {
	resp, err := r.rpc.RunAnalysis(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) runRuns(ctx context.Context, req *pb.RunRunsRequest) (*pb.RunRunsResponse, error) {
	resp, err := r.rpc.RunRuns(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) selfCheck(ctx context.Context, req *pb.SelfCheckRequest) (*pb.SelfCheckResponse, error) {
	resp, err := r.rpc.SelfCheck(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) recordRun(ctx context.Context, req *pb.RecordRunRequest) (*pb.RecordRunResponse, error) {
	resp, err := r.rpc.RecordRun(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) query(ctx context.Context, req *pb.QueryRequest) (*pb.QueryResponse, error) {
	resp, err := r.rpc.Query(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) runDocumentQuery(
	ctx context.Context,
	req *pb.RunDocumentQueryRequest,
) (*pb.RunDocumentQueryResponse, error) {
	resp, err := r.rpc.RunDocumentQuery(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) renderDocument(ctx context.Context, req *pb.RenderDocumentRequest) (*pb.RenderDocumentResponse, error) {
	resp, err := r.rpc.RenderDocument(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) renderView(ctx context.Context, req *pb.RenderViewRequest) (*pb.RenderViewResponse, error) {
	resp, err := r.rpc.RenderView(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) exportGraphs(ctx context.Context, req *pb.ExportGraphsRequest) (*pb.ExportGraphsResponse, error) {
	resp, err := r.rpc.ExportGraphs(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) checkBehavior(ctx context.Context, req *pb.CheckBehaviorRequest) (*pb.CheckBehaviorResponse, error) {
	resp, err := r.rpc.CheckBehavior(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) convert(ctx context.Context, req *pb.ConvertRequest) (*pb.ConvertResponse, error) {
	resp, err := r.rpc.Convert(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) migrate(ctx context.Context, req *pb.MigrateRequest) (*pb.MigrateResponse, error) {
	resp, err := r.rpc.Migrate(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) applyEdits(ctx context.Context, req *pb.ApplyEditsRequest) (*pb.ApplyEditsResponse, error) {
	resp, err := r.rpc.ApplyEdits(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) close() error {
	return nil
}

// connectToError is the documented status mapping for the remote path: a
// Connect error code numbers the same as its gRPC status. A transport failure
// that never reached the service is CodeUnavailable.
func connectToError(err error) error {
	var connectErr *connect.Error
	if errors.As(err, &connectErr) {
		return &StatusError{Code: Code(connectErr.Code()), Message: connectErr.Message()}
	}
	return &StatusError{Code: CodeUnavailable, Message: err.Error()}
}

func (r *remote) importData(ctx context.Context, req *pb.ImportDataRequest) (*pb.ImportDataResponse, error) {
	resp, err := r.rpc.ImportData(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}

func (r *remote) compareResults(ctx context.Context, req *pb.CompareResultsRequest) (*pb.CompareResultsResponse, error) {
	resp, err := r.rpc.CompareResults(ctx, connect.NewRequest(req))
	if err != nil {
		return nil, connectToError(err)
	}
	return resp.Msg, nil
}
