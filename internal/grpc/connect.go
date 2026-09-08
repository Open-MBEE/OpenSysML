// Copyright 2025 Open‐MBEE Foundation. All rights reserved.
// Use of this source code is governed by the LICENSE file.

package grpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/api/proto/protoconnect"
)

// ConnectAdapter serves the same Service through the Connect handler
// signatures, so one implementation answers every protocol Connect speaks.
type ConnectAdapter struct {
	svc *Service
}

var _ protoconnect.SysMLServiceHandler = (*ConnectAdapter)(nil)

// NewConnectAdapter wraps a Service as a Connect handler.
func NewConnectAdapter(svc *Service) *ConnectAdapter {
	return &ConnectAdapter{svc: svc}
}

// connectCall runs one unary handler. The Service refuses calls with
// *connect.Error already, so the error needs no translation here.
func connectCall[Req, Res any](
	ctx context.Context,
	req *connect.Request[Req],
	handler func(context.Context, *Req) (*Res, error),
) (*connect.Response[Res], error) {
	res, err := handler(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(res), nil
}

// statusError is a refused call: a *connect.Error whose code numbers the same
// as the canonical gRPC status every transport answers with.
func statusError(code connect.Code, message string) *connect.Error {
	return connect.NewError(code, errors.New(message))
}

// statusErrorf is statusError with formatting.
func statusErrorf(code connect.Code, format string, args ...any) *connect.Error {
	return connect.NewError(code, fmt.Errorf(format, args...))
}

// GetServerInfo reports what this build of the service can do.
func (a *ConnectAdapter) GetServerInfo(ctx context.Context, req *connect.Request[pb.ServerInfoRequest]) (*connect.Response[pb.ServerInfoResponse], error) {
	return connectCall(ctx, req, a.svc.GetServerInfo)
}

// ParseFile parses a model and returns its hash.
func (a *ConnectAdapter) ParseFile(ctx context.Context, req *connect.Request[pb.ParseFileRequest]) (*connect.Response[pb.ParseFileResponse], error) {
	return connectCall(ctx, req, a.svc.ParseFile)
}

// ParseSources parses several documents as one model.
func (a *ConnectAdapter) ParseSources(ctx context.Context, req *connect.Request[pb.ParseSourcesRequest]) (*connect.Response[pb.ParseSourcesResponse], error) {
	return connectCall(ctx, req, a.svc.ParseSources)
}

// GetSymbol returns symbol information by qualified name.
func (a *ConnectAdapter) GetSymbol(ctx context.Context, req *connect.Request[pb.GetSymbolRequest]) (*connect.Response[pb.SymbolResponse], error) {
	return connectCall(ctx, req, a.svc.GetSymbol)
}

// GetDiagnostics returns the diagnostics of a parsed model.
func (a *ConnectAdapter) GetDiagnostics(ctx context.Context, req *connect.Request[pb.DiagnosticsRequest]) (*connect.Response[pb.DiagnosticsResponse], error) {
	return connectCall(ctx, req, a.svc.GetDiagnostics)
}

// Evaluate evaluates an expression against a parsed model.
func (a *ConnectAdapter) Evaluate(ctx context.Context, req *connect.Request[pb.EvaluateRequest]) (*connect.Response[pb.EvaluateResponse], error) {
	return connectCall(ctx, req, a.svc.Evaluate)
}

// Instantiate materializes an instance of a definition.
func (a *ConnectAdapter) Instantiate(ctx context.Context, req *connect.Request[pb.InstantiateRequest]) (*connect.Response[pb.InstantiateResponse], error) {
	return connectCall(ctx, req, a.svc.Instantiate)
}

// ExecuteAction runs an action to completion.
func (a *ConnectAdapter) ExecuteAction(ctx context.Context, req *connect.Request[pb.ExecuteActionRequest]) (*connect.Response[pb.ExecuteActionResponse], error) {
	return connectCall(ctx, req, a.svc.ExecuteAction)
}

// ExecuteState runs a state machine over the events given.
func (a *ConnectAdapter) ExecuteState(ctx context.Context, req *connect.Request[pb.ExecuteStateRequest]) (*connect.Response[pb.ExecuteStateResponse], error) {
	return connectCall(ctx, req, a.svc.ExecuteState)
}

// Convert writes a model out as SysML notation or RDF Turtle.
func (a *ConnectAdapter) Convert(ctx context.Context, req *connect.Request[pb.ConvertRequest]) (*connect.Response[pb.ConvertResponse], error) {
	return connectCall(ctx, req, a.svc.Convert)
}

// ApplyEdits edits a parsed model's own source.
func (a *ConnectAdapter) ApplyEdits(ctx context.Context, req *connect.Request[pb.ApplyEditsRequest]) (*connect.Response[pb.ApplyEditsResponse], error) {
	return connectCall(ctx, req, a.svc.ApplyEdits)
}

// VerifyConstraint evaluates a constraint and returns its verdict.
func (a *ConnectAdapter) VerifyConstraint(ctx context.Context, req *connect.Request[pb.VerifyConstraintRequest]) (*connect.Response[pb.VerifyConstraintResponse], error) {
	return connectCall(ctx, req, a.svc.VerifyConstraint)
}

// VerifyRequirement evaluates a requirement and returns its verdict.
func (a *ConnectAdapter) VerifyRequirement(ctx context.Context, req *connect.Request[pb.VerifyRequirementRequest]) (*connect.Response[pb.VerifyRequirementResponse], error) {
	return connectCall(ctx, req, a.svc.VerifyRequirement)
}

// VerifySatisfaction evaluates satisfaction relationships in a model.
func (a *ConnectAdapter) VerifySatisfaction(ctx context.Context, req *connect.Request[pb.VerifySatisfactionRequest]) (*connect.Response[pb.VerifySatisfactionResponse], error) {
	return connectCall(ctx, req, a.svc.VerifySatisfaction)
}

// EvaluateCalc evaluates a calculation with the arguments given.
func (a *ConnectAdapter) EvaluateCalc(ctx context.Context, req *connect.Request[pb.EvaluateCalcRequest]) (*connect.Response[pb.EvaluateCalcResponse], error) {
	return connectCall(ctx, req, a.svc.EvaluateCalc)
}

// RunAnalysis runs an analysis case with the subject and arguments given.
func (a *ConnectAdapter) RunAnalysis(ctx context.Context, req *connect.Request[pb.RunAnalysisRequest]) (*connect.Response[pb.RunAnalysisResponse], error) {
	return connectCall(ctx, req, a.svc.RunAnalysis)
}

// RunSweep runs one analysis case or calc once per row of a parameter sweep.
func (a *ConnectAdapter) RunSweep(ctx context.Context, req *connect.Request[pb.RunSweepRequest]) (*connect.Response[pb.RunSweepResponse], error) {
	return connectCall(ctx, req, a.svc.RunSweep)
}

// Query evaluates a SysML v2 API & Services Query over a parsed model.
func (a *ConnectAdapter) Query(ctx context.Context, req *connect.Request[pb.QueryRequest]) (*connect.Response[pb.QueryResponse], error) {
	return connectCall(ctx, req, a.svc.Query)
}

// RunDocumentQuery runs a named document query and answers with typed rows.
func (a *ConnectAdapter) RunDocumentQuery(ctx context.Context, req *connect.Request[pb.RunDocumentQueryRequest]) (*connect.Response[pb.RunDocumentQueryResponse], error) {
	return connectCall(ctx, req, a.svc.RunDocumentQuery)
}

// RenderDocument renders a named document to Markdown.
func (a *ConnectAdapter) RenderDocument(ctx context.Context, req *connect.Request[pb.RenderDocumentRequest]) (*connect.Response[pb.RenderDocumentResponse], error) {
	return connectCall(ctx, req, a.svc.RenderDocument)
}
