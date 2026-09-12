package main

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/client/opensysml"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// pkgClient runs scenarios through the public Go API, client/opensysml, so the
// suite holds the API to the wire contract rather than holding raw stubs to
// it. The "pkg" protocol answers in process; "pkg-connect" dials the started
// service through opensysml.Dial.
type pkgClient struct {
	name   string
	api    opensysml.Client
	models map[string]*opensysml.Model // hash → the handle later calls take
}

func newPkgClient(name string, api opensysml.Client) *pkgClient {
	return &pkgClient{name: name, api: api, models: map[string]*opensysml.Model{}}
}

// uncoveredError marks a call the public API's v1 surface cannot express,
// which the runner reports as a skip rather than hiding.
type uncoveredError struct{ reason string }

func (e *uncoveredError) Error() string { return e.reason }

// errNonNumericVector is the uncoveredError for a vector argument whose
// component is not a number.
var errNonNumericVector = &uncoveredError{reason: "the public Go API cannot send a vector with a non-numeric component"}

func (c *pkgClient) protocol() string { return c.name }

func (c *pkgClient) close() { _ = c.api.Close() }

func (c *pkgClient) call(ctx context.Context, method string, request protoreflect.Message) (protoreflect.Message, error) {
	response, err := c.dispatch(ctx, method, request)
	if err != nil {
		return nil, err
	}
	return response.ProtoReflect(), nil
}

// dispatch makes the call through the public API and rebuilds the wire
// response from what the API returned, so the comparison sees exactly what a
// Go caller was given.
func (c *pkgClient) dispatch(ctx context.Context, method string, request protoreflect.Message) (proto.Message, error) {
	switch method {
	case "GetServerInfo":
		return c.serverInfo(ctx)
	case "ListEngines":
		return c.listEngines(ctx)
	case "ParseFile":
		return c.parseFile(ctx, request)
	case "ParseSources":
		return c.parseSources(ctx, request)
	case "GetSymbol":
		return c.getSymbol(ctx, request)
	case "GetDiagnostics":
		return c.getDiagnostics(ctx, request)
	case "Evaluate":
		return c.evaluate(ctx, request)
	case "Instantiate":
		return c.instantiate(ctx, request)
	case "ExecuteAction":
		return c.executeAction(ctx, request)
	case "ExecuteState":
		return c.executeState(ctx, request)
	case "VerifyConstraint":
		return c.verifyConstraint(ctx, request)
	case "VerifyRequirement":
		return c.verifyRequirement(ctx, request)
	case "VerifySatisfaction":
		return c.verifySatisfaction(ctx, request)
	case "EvaluateCalc":
		return c.evaluateCalc(ctx, request)
	case "RunAnalysis":
		return c.runAnalysis(ctx, request)
	case "Query":
		return c.query(ctx, request)
	case "RunDocumentQuery":
		return c.runDocumentQuery(ctx, request)
	case "RenderDocument":
		return c.renderDocument(ctx, request)
	case "Convert":
		return c.convert(ctx, request)
	case "ApplyEdits":
		return c.applyEdits(ctx, request)
	default:
		return nil, &uncoveredError{reason: fmt.Sprintf("the public Go API does not cover %s", method)}
	}
}

func (c *pkgClient) serverInfo(ctx context.Context) (proto.Message, error) {
	info, err := c.api.ServerInfo(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.ServerInfoResponse{Version: info.Version, Capabilities: info.Capabilities}, nil
}

func (c *pkgClient) listEngines(ctx context.Context) (proto.Message, error) {
	engines, err := c.api.ListEngines(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	response := &pb.ListEnginesResponse{}
	for _, engine := range engines {
		response.Engines = append(response.Engines, &pb.EngineInfo{
			Name:         engine.Name,
			Authority:    engine.Authority,
			Answers:      engine.Answers,
			Bounds:       engine.Bounds,
			Process:      engine.Process,
			ProcessFound: engine.ProcessFound,
			Ready:        engine.Ready,
			Unavailable:  engine.Unavailable,
		})
	}
	return response, nil
}

func (c *pkgClient) parseFile(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.ParseFileRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	var opts []opensysml.ParseOption
	if req.Language != "" {
		opts = append(opts, opensysml.WithLanguage(opensysml.Language(req.Language)))
	}
	if req.StrictConformance {
		opts = append(opts, opensysml.WithStrictConformance())
	}
	var model *opensysml.Model
	var err error
	switch source := req.Source.(type) {
	case *pb.ParseFileRequest_FilePath:
		model, err = c.api.ParseFile(ctx, source.FilePath, opts...)
	case *pb.ParseFileRequest_Content:
		model, err = c.api.ParseSource(ctx, source.Content, opts...)
	default:
		return nil, &uncoveredError{reason: "the public Go API cannot send a parse request naming no source"}
	}
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.ParseFileResponse{Error: failure.Message, Diagnostics: diagnosticsToProto(failure.Diagnostics)}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	c.models[model.Hash] = model
	return &pb.ParseFileResponse{
		ModelHash:   model.Hash,
		Root:        symbolToProto(model.Root),
		Diagnostics: diagnosticsToProto(model.Diagnostics),
	}, nil
}

func (c *pkgClient) parseSources(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.ParseSourcesRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	documents := make([]opensysml.Document, 0, len(req.Documents))
	for _, document := range req.Documents {
		converted := opensysml.Document{Name: document.Name, Language: opensysml.Language(document.Language)}
		switch source := document.Source.(type) {
		case *pb.SourceDocument_FilePath:
			converted.Path = source.FilePath
		case *pb.SourceDocument_Content:
			converted.Content = source.Content
		}
		documents = append(documents, converted)
	}
	var opts []opensysml.ParseOption
	if req.StrictConformance {
		opts = append(opts, opensysml.WithStrictConformance())
	}
	model, err := c.api.ParseDocuments(ctx, documents, opts...)
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.ParseSourcesResponse{
			Error:       failure.Message,
			Diagnostics: diagnosticsToProto(failure.Diagnostics),
		}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	c.models[model.Hash] = model
	response := &pb.ParseSourcesResponse{
		ModelHash:   model.Hash,
		Diagnostics: diagnosticsToProto(model.Diagnostics),
	}
	for _, root := range model.Roots {
		response.Roots = append(response.Roots, symbolToProto(root))
	}
	return response, nil
}

func (c *pkgClient) getSymbol(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.GetSymbolRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	symbol, err := c.api.LookupSymbol(ctx, c.model(req.ModelHash), req.SymbolId)
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.SymbolResponse{Error: failure.Message}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.SymbolResponse{Symbol: symbolToProto(symbol)}, nil
}

func (c *pkgClient) getDiagnostics(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.DiagnosticsRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	diagnostics, err := c.api.Diagnostics(ctx, c.model(req.ModelHash))
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.DiagnosticsResponse{Error: failure.Message}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.DiagnosticsResponse{Diagnostics: diagnosticsToProto(diagnostics)}, nil
}

func (c *pkgClient) evaluate(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.EvaluateRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	var opts []opensysml.EvaluateOption
	if req.ContextSymbolId != "" {
		opts = append(opts, opensysml.WithContextSymbol(req.ContextSymbolId))
	}
	if req.SubjectSymbolId != "" {
		opts = append(opts, opensysml.WithSubject(req.SubjectSymbolId))
	}
	value, err := c.api.Evaluate(ctx, c.model(req.ModelHash), req.Expression, opts...)
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.EvaluateResponse{Error: failure.Message, Diagnostics: diagnosticsToProto(failure.Diagnostics)}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.EvaluateResponse{Result: valueToProto(value)}, nil
}

func (c *pkgClient) instantiate(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.InstantiateRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	instantiation, err := c.api.Instantiate(ctx, c.model(req.ModelHash), req.SymbolId)
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.InstantiateResponse{Error: failure.Message, Diagnostics: diagnosticsToProto(failure.Diagnostics)}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	response := &pb.InstantiateResponse{
		Instance:    instanceToProto(instantiation.Root),
		Diagnostics: diagnosticsToProto(instantiation.Diagnostics),
	}
	for _, instance := range instantiation.Instances {
		response.Instances = append(response.Instances, instanceToProto(instance))
	}
	return response, nil
}

func (c *pkgClient) executeAction(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.ExecuteActionRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	inputs := make(map[string]opensysml.Value, len(req.Inputs))
	for name, value := range req.Inputs {
		input, converted := valueFromProto(value)
		if !converted {
			return nil, errNonNumericVector
		}
		inputs[name] = input
	}
	run, err := c.api.ExecuteAction(ctx, c.model(req.ModelHash), req.ActionSymbolId, inputs,
		opensysml.WithSchedule(req.Schedule))
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.ExecuteActionResponse{
			Error:       failure.Message,
			Diagnostics: diagnosticsToProto(failure.Diagnostics),
		}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.ExecuteActionResponse{
		Outputs:     valuesToProto(run.Outputs),
		Diagnostics: diagnosticsToProto(run.Diagnostics),
	}, nil
}

func (c *pkgClient) executeState(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.ExecuteStateRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	run, err := c.api.ExecuteState(ctx, c.model(req.ModelHash), req.StateMachineSymbolId, req.Events,
		opensysml.WithSchedule(req.Schedule))
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.ExecuteStateResponse{
			Error:       failure.Message,
			Diagnostics: diagnosticsToProto(failure.Diagnostics),
		}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.ExecuteStateResponse{
		StatesVisited: run.Visited,
		FinalContext:  valuesToProto(run.Context),
		Diagnostics:   diagnosticsToProto(run.Diagnostics),
	}, nil
}

func (c *pkgClient) verifyConstraint(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.VerifyConstraintRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	verification, err := c.api.VerifyConstraint(ctx, c.model(req.ModelHash), req.SymbolId,
		opensysml.Against(req.SubjectSymbolId), opensysml.WithEngine(req.Engine))
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.VerifyConstraintResponse{
			Error:       failure.Message,
			Diagnostics: diagnosticsToProto(failure.Diagnostics),
		}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.VerifyConstraintResponse{
		Verdict:     verdictToProto(verification.Verdict),
		Instances:   instancesToProto(verification.Instances),
		Diagnostics: diagnosticsToProto(verification.Diagnostics),
	}, nil
}

func (c *pkgClient) verifyRequirement(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.VerifyRequirementRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	verification, err := c.api.VerifyRequirement(ctx, c.model(req.ModelHash), req.SymbolId,
		opensysml.Against(req.SubjectSymbolId), opensysml.WithEngine(req.Engine))
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.VerifyRequirementResponse{
			Error:       failure.Message,
			Diagnostics: diagnosticsToProto(failure.Diagnostics),
		}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.VerifyRequirementResponse{
		Verdict:     verdictToProto(verification.Verdict),
		Instances:   instancesToProto(verification.Instances),
		Diagnostics: diagnosticsToProto(verification.Diagnostics),
	}, nil
}

func (c *pkgClient) verifySatisfaction(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.VerifySatisfactionRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	satisfaction, err := c.api.VerifySatisfaction(ctx, c.model(req.ModelHash), req.SymbolId,
		opensysml.WithEngine(req.Engine))
	var verifyErr *opensysml.VerifyError
	if errors.As(err, &verifyErr) {
		return &pb.VerifySatisfactionResponse{
			Error:         verifyErr.Message,
			FailureReason: pb.FailureReason(verifyErr.Reason),
			Diagnostics:   diagnosticsToProto(verifyErr.Diagnostics),
		}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	response := &pb.VerifySatisfactionResponse{
		Instances:   instancesToProto(satisfaction.Instances),
		Diagnostics: diagnosticsToProto(satisfaction.Diagnostics),
	}
	for i := range satisfaction.Verdicts {
		response.Verdicts = append(response.Verdicts, verdictToProto(&satisfaction.Verdicts[i]))
	}
	return response, nil
}

func (c *pkgClient) evaluateCalc(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.EvaluateCalcRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	arguments := make([]opensysml.Value, 0, len(req.Arguments))
	for _, argument := range req.Arguments {
		converted, ok := valueFromProto(argument)
		if !ok {
			return nil, errNonNumericVector
		}
		arguments = append(arguments, converted)
	}
	calculation, err := c.api.Calculate(ctx, c.model(req.ModelHash), req.SymbolId,
		opensysml.CalcArguments(arguments...), opensysml.CalcEngine(req.Engine))
	var verifyErr *opensysml.VerifyError
	if errors.As(err, &verifyErr) {
		return &pb.EvaluateCalcResponse{
			Error:         verifyErr.Message,
			FailureReason: pb.FailureReason(verifyErr.Reason),
			Diagnostics:   diagnosticsToProto(verifyErr.Diagnostics),
		}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	response := &pb.EvaluateCalcResponse{
		Result:      valueToProto(calculation.Result),
		Diagnostics: diagnosticsToProto(calculation.Diagnostics),
		Engine:      calculation.Standing.Engine,
		Strength:    calculation.Standing.Strength,
		Bounds:      boundsToProto(calculation.Standing.Bounds),
	}
	for _, output := range calculation.Outputs {
		response.Outputs = append(response.Outputs, &pb.CalcOutput{
			Name:  output.Name,
			Value: valueToProto(output.Value),
		})
	}
	return response, nil
}

func (c *pkgClient) runAnalysis(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.RunAnalysisRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	opts := []opensysml.AnalysisOption{
		opensysml.Subject(req.SubjectSymbolId), opensysml.Schedule(req.Schedule), opensysml.Engine(req.Engine),
	}
	for _, argument := range req.Arguments {
		converted, ok := valueFromProto(argument)
		if !ok {
			return nil, errNonNumericVector
		}
		opts = append(opts, opensysml.Arguments(converted))
	}
	for _, name := range slices.Sorted(maps.Keys(req.NamedArguments)) {
		converted, ok := valueFromProto(req.NamedArguments[name])
		if !ok {
			return nil, errNonNumericVector
		}
		opts = append(opts, opensysml.Argument(name, converted))
	}
	analysis, err := c.api.RunAnalysis(ctx, c.model(req.ModelHash), req.SymbolId, opts...)
	var verifyErr *opensysml.VerifyError
	if errors.As(err, &verifyErr) {
		return &pb.RunAnalysisResponse{
			Error:         verifyErr.Message,
			FailureReason: pb.FailureReason(verifyErr.Reason),
			Diagnostics:   diagnosticsToProto(verifyErr.Diagnostics),
		}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	response := &pb.RunAnalysisResponse{
		Instances:   instancesToProto(analysis.Instances),
		Diagnostics: diagnosticsToProto(analysis.Diagnostics),
		Engine:      analysis.Standing.Engine,
		Strength:    analysis.Standing.Strength,
		Bounds:      boundsToProto(analysis.Standing.Bounds),
	}
	for _, output := range analysis.Outputs {
		response.Outputs = append(response.Outputs, &pb.CalcOutput{
			Name:  output.Name,
			Value: valueToProto(output.Value),
		})
	}
	for i := range analysis.Verdicts {
		response.Verdicts = append(response.Verdicts, verdictToProto(&analysis.Verdicts[i]))
	}
	return response, nil
}

func (c *pkgClient) query(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.QueryRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	model := c.model(req.ModelHash)
	var elements []opensysml.QueryElement
	var err error
	switch {
	case req.OslcQuery != "":
		elements, err = c.api.QueryOSLC(ctx, model, req.OslcQuery)
	default:
		query, converted := queryFromProto(req.Query)
		if !converted {
			return nil, &uncoveredError{reason: "the public Go API cannot send a query with no comparison operator"}
		}
		elements, err = c.api.Query(ctx, model, query)
	}
	if err != nil {
		return nil, apiError(err)
	}
	response := &pb.QueryResponse{}
	for _, element := range elements {
		response.Elements = append(response.Elements, &pb.QueryResultElement{
			Id:         element.ID,
			Type:       element.Type,
			Properties: element.Properties,
		})
	}
	return response, nil
}

func (c *pkgClient) runDocumentQuery(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.RunDocumentQueryRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	bindings := make([]opensysml.Binding, 0, len(req.Bindings))
	for _, binding := range req.Bindings {
		converted := opensysml.Binding{Parameter: binding.Parameter}
		for _, value := range binding.Values {
			converted.Values = append(converted.Values, cellFromProto(value))
		}
		bindings = append(bindings, converted)
	}
	rows, err := c.api.RunDocumentQuery(ctx, c.model(req.ModelHash), req.QueryId, bindings...)
	if err != nil {
		return nil, apiError(err)
	}
	response := &pb.RunDocumentQueryResponse{}
	for _, column := range rows.Columns {
		response.Columns = append(response.Columns, &pb.DocumentQueryColumn{Name: column})
	}
	for _, row := range rows.Rows {
		converted := &pb.DocumentQueryRow{Element: cellToProto(row.Element)}
		for _, cell := range row.Cells {
			values := &pb.DocumentQueryCell{}
			for _, value := range cell {
				values.Values = append(values.Values, cellToProto(value))
			}
			converted.Cells = append(converted.Cells, values)
		}
		response.Rows = append(response.Rows, converted)
	}
	return response, nil
}

func (c *pkgClient) renderDocument(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.RenderDocumentRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	markdown, err := c.api.RenderDocument(ctx, c.model(req.ModelHash), req.DocumentId)
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.RenderDocumentResponse{Markdown: markdown}, nil
}

func (c *pkgClient) convert(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.ConvertRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	var opts []opensysml.ConvertOption
	if req.FromFormat != "" {
		opts = append(opts, opensysml.WithFromFormat(opensysml.Format(req.FromFormat)))
	}
	if req.TolerateSyntaxErrors {
		opts = append(opts, opensysml.WithTolerateSyntaxErrors())
	}
	var conversion *opensysml.Conversion
	var err error
	switch source := req.Source.(type) {
	case *pb.ConvertRequest_ModelHash:
		if req.FromFormat != "" {
			return nil, &uncoveredError{
				reason: "the public Go API reads a parsed model as the notation the parse read",
			}
		}
		conversion, err = c.api.Convert(ctx, c.model(source.ModelHash), opensysml.Format(req.ToFormat), opts...)
	case *pb.ConvertRequest_FilePath:
		conversion, err = c.api.ConvertFile(ctx, source.FilePath, opensysml.Format(req.ToFormat), opts...)
	case *pb.ConvertRequest_Content:
		conversion, err = c.api.ConvertSource(ctx, source.Content, opensysml.Format(req.ToFormat), opts...)
	default:
		return nil, &uncoveredError{reason: "the public Go API cannot send a conversion naming no source"}
	}
	var failure *opensysml.FailureError
	if errors.As(err, &failure) {
		return &pb.ConvertResponse{
			Error:       failure.Message,
			Diagnostics: diagnosticsToProto(failure.Diagnostics),
		}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	return &pb.ConvertResponse{
		Content:            conversion.Content,
		FromFormat:         string(conversion.From),
		ToFormat:           string(conversion.To),
		Experimental:       conversion.Experimental,
		ExperimentalNotice: conversion.ExperimentalNotice,
		Diagnostics:        diagnosticsToProto(conversion.Diagnostics),
	}, nil
}

func (c *pkgClient) applyEdits(ctx context.Context, request protoreflect.Message) (proto.Message, error) {
	req := &pb.ApplyEditsRequest{}
	if err := retype(request, req); err != nil {
		return nil, err
	}
	edits := make([]opensysml.Edit, 0, len(req.Operations))
	for _, operation := range req.Operations {
		edit, converted := editFromProto(operation)
		if !converted {
			return nil, &uncoveredError{reason: "the public Go API cannot send an edit naming no operation"}
		}
		edits = append(edits, edit)
	}
	result, err := c.api.ApplyEdits(ctx, c.model(req.ModelHash), edits...)
	var editErr *opensysml.EditError
	if errors.As(err, &editErr) {
		return &pb.ApplyEditsResponse{
			Error:             editErr.Message,
			Failure:           pb.EditFailure(editErr.Failure),
			ReferringElements: editErr.Referring,
			Diagnostics:       diagnosticsToProto(editErr.Diagnostics),
		}, nil
	}
	if err != nil {
		return nil, apiError(err)
	}
	response := &pb.ApplyEditsResponse{
		Content:     result.Content,
		Diagnostics: diagnosticsToProto(result.Diagnostics),
	}
	for _, applied := range result.Applied {
		// #nosec G115 -- source offsets and lengths fit in int32.
		response.Applied = append(response.Applied, &pb.AppliedEdit{
			OperationIndex: int32(applied.Index),
			Target:         applied.Target,
			Offset:         int32(applied.Offset),
			Length:         int32(applied.Length),
			OldText:        applied.OldText,
			NewText:        applied.NewText,
		})
	}
	return response, nil
}

// model answers the handle a hash names: the parse this client made, or a bare
// handle so an unknown hash reaches the service and is refused there.
func (c *pkgClient) model(hash string) *opensysml.Model {
	if model, ok := c.models[hash]; ok {
		return model
	}
	return &opensysml.Model{Hash: hash}
}

// retype copies a dynamic request into its generated type.
func retype(request protoreflect.Message, into proto.Message) error {
	if request == nil {
		return nil
	}
	data, err := proto.Marshal(request.Interface())
	if err != nil {
		return err
	}
	return proto.Unmarshal(data, into)
}

// apiError maps the public API's StatusError back to the gRPC status the
// runner compares, so the documented mapping is itself under test.
func apiError(err error) error {
	var statusErr *opensysml.StatusError
	if errors.As(err, &statusErr) {
		return status.Error(codes.Code(statusErr.Code), statusErr.Message)
	}
	return err
}

// The rebuilds from public types to wire types. Their fidelity is part of what
// the suite checks: a public type that dropped a field would fail comparison.

func symbolToProto(symbol *opensysml.Symbol) *pb.SymbolInfo {
	if symbol == nil {
		return nil
	}
	out := &pb.SymbolInfo{
		Id:                        symbol.ID,
		Name:                      symbol.Name,
		Kind:                      symbol.Kind,
		Metadata:                  symbol.Metadata,
		ChildIds:                  symbol.ChildIDs,
		WithheldLibraryAttributes: int32(symbol.WithheldLibraryAttributes), // #nosec G115 -- attribute counts are small
	}
	for _, attribute := range symbol.Attributes {
		out.Attributes = append(out.Attributes, &pb.AttributeInfo{
			Name:  attribute.Name,
			Type:  attribute.Type,
			Value: valueToProto(attribute.Value),
			Unit:  attribute.Unit,
		})
	}
	if symbol.Type != nil {
		out.TypeInfo = &pb.TypeInfo{
			Declared:        symbol.Type.Declared,
			ResolvedId:      symbol.Type.ResolvedID,
			ResolvedKind:    symbol.Type.ResolvedKind,
			Primitive:       symbol.Type.Primitive,
			PrimitiveSource: symbol.Type.PrimitiveSource,
			Quantity:        symbol.Type.Quantity,
			Unit:            symbol.Type.Unit,
		}
	}
	if symbol.Multiplicity != nil {
		out.Multiplicity = &pb.MultiplicityInfo{Lower: symbol.Multiplicity.Lower, Upper: symbol.Multiplicity.Upper}
	}
	for _, specialization := range symbol.Specializations {
		out.Specializations = append(out.Specializations, &pb.Specialization{
			Kind:       specialization.Kind,
			Declared:   specialization.Declared,
			TargetId:   specialization.TargetID,
			TargetKind: specialization.TargetKind,
		})
	}
	return out
}

func diagnosticsToProto(diagnostics []opensysml.Diagnostic) []*pb.Diagnostic {
	var out []*pb.Diagnostic
	for _, diagnostic := range diagnostics {
		converted := &pb.Diagnostic{Severity: diagnostic.Severity, Message: diagnostic.Message, Code: diagnostic.Code}
		if diagnostic.Span != nil {
			// #nosec G115 -- line and column numbers fit in int32.
			converted.Span = &pb.Span{
				File:      diagnostic.Span.File,
				StartLine: int32(diagnostic.Span.StartLine),
				StartCol:  int32(diagnostic.Span.StartCol),
				EndLine:   int32(diagnostic.Span.EndLine),
				EndCol:    int32(diagnostic.Span.EndCol),
			}
		}
		out = append(out, converted)
	}
	return out
}

func valueToProto(value opensysml.Value) *pb.Value {
	switch v := value.(type) {
	case nil:
		return nil
	case opensysml.Int:
		return &pb.Value{Kind: &pb.Value_IntValue{IntValue: int64(v)}}
	case opensysml.Real:
		return &pb.Value{Kind: &pb.Value_RealValue{RealValue: float64(v)}}
	case opensysml.Complex:
		return &pb.Value{Kind: &pb.Value_Complex{Complex: &pb.Complex{Real: real(v), Imaginary: imag(v)}}}
	case opensysml.Bool:
		return &pb.Value{Kind: &pb.Value_BoolValue{BoolValue: bool(v)}}
	case opensysml.String:
		return &pb.Value{Kind: &pb.Value_StringValue{StringValue: string(v)}}
	case opensysml.InstanceID:
		return &pb.Value{Kind: &pb.Value_InstanceId{InstanceId: int64(v)}}
	case opensysml.Sequence:
		sequence := &pb.ValueSequence{}
		for _, element := range v {
			sequence.Elements = append(sequence.Elements, valueToProto(element))
		}
		return &pb.Value{Kind: &pb.Value_Sequence{Sequence: sequence}}
	case opensysml.Null:
		return &pb.Value{Kind: &pb.Value_Null{Null: string(v)}}
	case opensysml.Unset:
		return &pb.Value{Kind: &pb.Value_Unset{Unset: true}}
	case opensysml.Undetermined:
		return &pb.Value{Kind: &pb.Value_Undetermined{Undetermined: &pb.Undetermined{
			Reason: v.Reason,
			Count:  &pb.MultiplicityInfo{Lower: v.CountLower, Upper: v.CountUpper},
		}}}
	case opensysml.Quantity:
		return &pb.Value{Kind: &pb.Value_Quantity{Quantity: quantityToProto(v)}}
	case opensysml.EnumLiteral:
		return &pb.Value{Kind: &pb.Value_EnumLiteral{EnumLiteral: &pb.EnumLiteral{
			LiteralId:     v.LiteralID,
			EnumerationId: v.EnumerationID,
			Name:          v.Name,
		}}}
	case opensysml.Array:
		array := &pb.Array{Dimensions: append([]int64(nil), v.Dimensions...)}
		for _, element := range v.Elements {
			array.Elements = append(array.Elements, valueToProto(element))
		}
		return &pb.Value{Kind: &pb.Value_Array{Array: array}}
	case opensysml.Vector:
		vector := &pb.Vector{}
		for _, component := range v {
			vector.Components = append(vector.Components, valueToProto(component))
		}
		return &pb.Value{Kind: &pb.Value_Vector{Vector: vector}}
	case opensysml.VectorQuantity:
		vq := &pb.VectorQuantity{}
		for _, component := range v {
			vq.Components = append(vq.Components, quantityToProto(component))
		}
		return &pb.Value{Kind: &pb.Value_VectorQuantity{VectorQuantity: vq}}
	case opensysml.Set:
		set := &pb.ValueSet{}
		for _, element := range v {
			set.Elements = append(set.Elements, valueToProto(element))
		}
		return &pb.Value{Kind: &pb.Value_Set{Set: set}}
	case opensysml.TensorQuantity:
		tensor := &pb.TensorQuantity{Dimensions: append([]int64(nil), v.Dimensions...)}
		for _, component := range v.Components {
			tensor.Components = append(tensor.Components, quantityToProto(component))
		}
		return &pb.Value{Kind: &pb.Value_TensorQuantity{TensorQuantity: tensor}}
	case opensysml.MeasurementRef:
		return &pb.Value{Kind: &pb.Value_MeasurementRef{MeasurementRef: &pb.MeasurementRef{
			Unit:     v.Unit,
			UnitTerm: unitTermToProto(v.Term),
			UnitId:   v.UnitID,
		}}}
	case opensysml.Function:
		return &pb.Value{Kind: &pb.Value_Function{Function: &pb.Function{
			CalcId: v.CalcID,
			SelfId: int64(v.Self),
		}}}
	case opensysml.Metaobject:
		return &pb.Value{Kind: &pb.Value_Metaobject{Metaobject: &pb.Metaobject{
			ElementId:   v.ElementID,
			MetaclassId: v.MetaclassID,
		}}}
	default:
		return nil
	}
}

func valuesToProto(values map[string]opensysml.Value) map[string]*pb.Value {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]*pb.Value, len(values))
	for name, value := range values {
		out[name] = valueToProto(value)
	}
	return out
}

// valueFromProto reads a scenario's request value into the public type a call
// takes, so a request the suite states reaches the API as the API states it.
// It reports false for a value the public types cannot state, such as a vector
// with a non-numeric component.
func valueFromProto(value *pb.Value) (opensysml.Value, bool) {
	switch kind := value.GetKind().(type) {
	case *pb.Value_IntValue:
		return opensysml.Int(kind.IntValue), true
	case *pb.Value_RealValue:
		return opensysml.Real(kind.RealValue), true
	case *pb.Value_Complex:
		return opensysml.Complex(complex(kind.Complex.GetReal(), kind.Complex.GetImaginary())), true
	case *pb.Value_BoolValue:
		return opensysml.Bool(kind.BoolValue), true
	case *pb.Value_StringValue:
		return opensysml.String(kind.StringValue), true
	case *pb.Value_InstanceId:
		return opensysml.InstanceID(kind.InstanceId), true
	case *pb.Value_Null:
		return opensysml.Null(kind.Null), true
	case *pb.Value_Unset:
		return opensysml.Unset{}, true
	case *pb.Value_Undetermined:
		return opensysml.Undetermined{
			Reason:     kind.Undetermined.GetReason(),
			CountLower: kind.Undetermined.GetCount().GetLower(),
			CountUpper: kind.Undetermined.GetCount().GetUpper(),
		}, true
	case *pb.Value_Sequence:
		sequence := make(opensysml.Sequence, 0, len(kind.Sequence.GetElements()))
		for _, element := range kind.Sequence.GetElements() {
			converted, ok := valueFromProto(element)
			if !ok {
				return nil, false
			}
			sequence = append(sequence, converted)
		}
		return sequence, true
	case *pb.Value_Quantity:
		return quantityFromProto(kind.Quantity), true
	case *pb.Value_EnumLiteral:
		return opensysml.EnumLiteral{
			LiteralID:     kind.EnumLiteral.GetLiteralId(),
			EnumerationID: kind.EnumLiteral.GetEnumerationId(),
			Name:          kind.EnumLiteral.GetName(),
		}, true
	case *pb.Value_Array:
		array := opensysml.Array{Dimensions: append([]int64(nil), kind.Array.GetDimensions()...)}
		for _, element := range kind.Array.GetElements() {
			converted, ok := valueFromProto(element)
			if !ok {
				return nil, false
			}
			array.Elements = append(array.Elements, converted)
		}
		return array, true
	case *pb.Value_Vector:
		vector := make(opensysml.Vector, 0, len(kind.Vector.GetComponents()))
		for _, component := range kind.Vector.GetComponents() {
			converted, ok := valueFromProto(component)
			number, numeric := converted.(opensysml.Number)
			if !ok || !numeric {
				return nil, false
			}
			vector = append(vector, number)
		}
		return vector, true
	case *pb.Value_VectorQuantity:
		vq := make(opensysml.VectorQuantity, 0, len(kind.VectorQuantity.GetComponents()))
		for _, component := range kind.VectorQuantity.GetComponents() {
			vq = append(vq, quantityFromProto(component))
		}
		return vq, true
	case *pb.Value_Set:
		set := make(opensysml.Set, 0, len(kind.Set.GetElements()))
		for _, element := range kind.Set.GetElements() {
			converted, ok := valueFromProto(element)
			if !ok {
				return nil, false
			}
			set = append(set, converted)
		}
		return set, true
	case *pb.Value_TensorQuantity:
		tensor := opensysml.TensorQuantity{Dimensions: append([]int64(nil), kind.TensorQuantity.GetDimensions()...)}
		for _, component := range kind.TensorQuantity.GetComponents() {
			tensor.Components = append(tensor.Components, quantityFromProto(component))
		}
		return tensor, true
	case *pb.Value_MeasurementRef:
		return opensysml.MeasurementRef{
			Unit:   kind.MeasurementRef.GetUnit(),
			Term:   unitTermFromProto(kind.MeasurementRef.GetUnitTerm()),
			UnitID: kind.MeasurementRef.GetUnitId(),
		}, true
	case *pb.Value_Function:
		return opensysml.Function{
			CalcID: kind.Function.GetCalcId(),
			Self:   opensysml.InstanceID(kind.Function.GetSelfId()),
		}, true
	case *pb.Value_Metaobject:
		return opensysml.Metaobject{
			ElementID:   kind.Metaobject.GetElementId(),
			MetaclassID: kind.Metaobject.GetMetaclassId(),
		}, true
	default:
		return nil, true
	}
}

func quantityFromProto(quantity *pb.Quantity) opensysml.Quantity {
	out := opensysml.Quantity{Unit: quantity.GetUnit(), Term: unitTermFromProto(quantity.GetUnitTerm())}
	switch magnitude := quantity.GetMagnitude().(type) {
	case *pb.Quantity_IntMagnitude:
		out.Magnitude = opensysml.Int(magnitude.IntMagnitude)
	case *pb.Quantity_RealMagnitude:
		out.Magnitude = opensysml.Real(magnitude.RealMagnitude)
	}
	return out
}

func unitTermFromProto(term *pb.UnitTerm) *opensysml.UnitTerm {
	if term == nil {
		return nil
	}
	converted := &opensysml.UnitTerm{ScaleNum: term.GetScaleNum(), ScaleDen: term.GetScaleDen()}
	for _, factor := range term.GetFactors() {
		converted.Factors = append(converted.Factors, opensysml.UnitFactor{
			UnitID:   factor.GetUnitId(),
			Exponent: factor.GetExponent(),
		})
	}
	return converted
}

func instancesToProto(instances []*opensysml.Instance) []*pb.Instance {
	var out []*pb.Instance
	for _, instance := range instances {
		out = append(out, instanceToProto(instance))
	}
	return out
}

func verdictToProto(verdict *opensysml.Verdict) *pb.Verdict {
	if verdict == nil {
		return nil
	}
	return &pb.Verdict{
		Kind:           verdict.Kind,
		ElementId:      verdict.ElementID,
		Element:        verdict.Element,
		Holds:          verdict.Holds,
		Condition:      verdict.Condition,
		InstanceId:     verdict.InstanceID,
		InstanceTypeId: verdict.InstanceTypeID,
		Error:          verdict.Error,
		FailureReason:  pb.FailureReason(verdict.Reason),
		Engine:         verdict.Standing.Engine,
		Strength:       verdict.Standing.Strength,
		Bounds:         boundsToProto(verdict.Standing.Bounds),
	}
}

func boundsToProto(bounds []opensysml.Bound) []*pb.Bound {
	if len(bounds) == 0 {
		return nil
	}
	out := make([]*pb.Bound, 0, len(bounds))
	for _, bound := range bounds {
		out = append(out, &pb.Bound{Name: bound.Name, Limit: bound.Limit, Reached: bound.Reached})
	}
	return out
}

// queryFromProto reads a scenario's query into the public builders. It reports
// false for a query the builders cannot state, such as one naming no operator.
func queryFromProto(query *pb.Query) (opensysml.Query, bool) {
	out := opensysml.Query{Scope: query.GetScope(), Select: query.GetSelect()}
	if query.GetWhere() == nil {
		return out, true
	}
	where, ok := conditionFromProto(query.GetWhere())
	if !ok {
		return out, false
	}
	out.Where = where
	return out, true
}

func conditionFromProto(constraint *pb.Constraint) (*opensysml.Condition, bool) {
	switch kind := constraint.GetConstraint().(type) {
	case *pb.Constraint_Primitive:
		primitive := kind.Primitive
		var condition *opensysml.Condition
		switch primitive.GetOperator() {
		case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL:
			condition = opensysml.Equals(primitive.GetProperty(), primitive.GetValue()...)
		case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER:
			condition = opensysml.Greater(primitive.GetProperty(), firstValue(primitive.GetValue()))
		case pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS:
			condition = opensysml.Less(primitive.GetProperty(), firstValue(primitive.GetValue()))
		default:
			return nil, false
		}
		if primitive.GetInverse() {
			condition = condition.Not()
		}
		return condition, true
	case *pb.Constraint_Composite:
		operands := make([]*opensysml.Condition, 0, len(kind.Composite.GetConstraint()))
		for _, operand := range kind.Composite.GetConstraint() {
			converted, ok := conditionFromProto(operand)
			if !ok {
				return nil, false
			}
			operands = append(operands, converted)
		}
		switch kind.Composite.GetOperator() {
		case pb.CompositeOperator_COMPOSITE_OPERATOR_AND:
			return opensysml.All(operands...), true
		case pb.CompositeOperator_COMPOSITE_OPERATOR_OR:
			return opensysml.Any(operands...), true
		default:
			return nil, false
		}
	default:
		return nil, false
	}
}

func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func cellFromProto(value *pb.DocumentValue) opensysml.Cell {
	switch kind := value.GetKind().(type) {
	case *pb.DocumentValue_ElementId:
		return opensysml.Element{ID: kind.ElementId, Type: value.GetElementType()}
	case *pb.DocumentValue_StringValue:
		return opensysml.String(kind.StringValue)
	case *pb.DocumentValue_IntValue:
		return opensysml.Int(kind.IntValue)
	case *pb.DocumentValue_RealValue:
		return opensysml.Real(kind.RealValue)
	case *pb.DocumentValue_BoolValue:
		return opensysml.Bool(kind.BoolValue)
	case *pb.DocumentValue_Infinity:
		return opensysml.Infinity{}
	default:
		return nil
	}
}

func cellToProto(cell opensysml.Cell) *pb.DocumentValue {
	switch value := cell.(type) {
	case opensysml.Element:
		return &pb.DocumentValue{
			Kind:        &pb.DocumentValue_ElementId{ElementId: value.ID},
			ElementType: value.Type,
		}
	case opensysml.String:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_StringValue{StringValue: string(value)}}
	case opensysml.Int:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_IntValue{IntValue: int64(value)}}
	case opensysml.Real:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_RealValue{RealValue: float64(value)}}
	case opensysml.Bool:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_BoolValue{BoolValue: bool(value)}}
	case opensysml.Infinity:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_Infinity{Infinity: true}}
	default:
		return nil
	}
}

func editFromProto(operation *pb.EditOperation) (opensysml.Edit, bool) {
	switch kind := operation.GetOperation().(type) {
	case *pb.EditOperation_SetValue:
		return opensysml.SetValue{Target: kind.SetValue.GetTarget(), Value: kind.SetValue.GetValue()}, true
	case *pb.EditOperation_Rename:
		return opensysml.Rename{Target: kind.Rename.GetTarget(), NewName: kind.Rename.GetNewName()}, true
	case *pb.EditOperation_AddMember:
		return opensysml.AddMember{
			Owner:        kind.AddMember.GetOwner(),
			Kind:         kind.AddMember.GetKind(),
			Name:         kind.AddMember.GetName(),
			Type:         kind.AddMember.GetType(),
			Multiplicity: kind.AddMember.GetMultiplicity(),
			Value:        kind.AddMember.GetValue(),
			Specializes:  kind.AddMember.GetSpecializes(),
		}, true
	case *pb.EditOperation_Delete:
		return opensysml.Delete{Target: kind.Delete.GetTarget(), Cascade: kind.Delete.GetCascade()}, true
	default:
		return nil, false
	}
}

func quantityToProto(quantity opensysml.Quantity) *pb.Quantity {
	out := &pb.Quantity{Unit: quantity.Unit}
	switch magnitude := quantity.Magnitude.(type) {
	case opensysml.Int:
		out.Magnitude = &pb.Quantity_IntMagnitude{IntMagnitude: int64(magnitude)}
	case opensysml.Real:
		out.Magnitude = &pb.Quantity_RealMagnitude{RealMagnitude: float64(magnitude)}
	}
	out.UnitTerm = unitTermToProto(quantity.Term)
	return out
}

func unitTermToProto(term *opensysml.UnitTerm) *pb.UnitTerm {
	if term == nil {
		return nil
	}
	out := &pb.UnitTerm{ScaleNum: term.ScaleNum, ScaleDen: term.ScaleDen}
	for _, factor := range term.Factors {
		out.Factors = append(out.Factors, &pb.UnitFactor{UnitId: factor.UnitID, Exponent: factor.Exponent})
	}
	return out
}

func instanceToProto(instance *opensysml.Instance) *pb.Instance {
	if instance == nil {
		return nil
	}
	out := &pb.Instance{Id: instance.ID, TypeSymbolId: instance.TypeSymbolID}
	if len(instance.FeatureValues) > 0 {
		out.FeatureValues = make(map[string]*pb.FeatureValue, len(instance.FeatureValues))
		for name, featureValue := range instance.FeatureValues {
			converted := &pb.FeatureValue{
				FeatureName:  featureValue.FeatureName,
				Value:        valueToProto(featureValue.Value),
				Materialized: featureValue.Materialized,
				Error:        featureValue.Error,
			}
			for _, element := range featureValue.Values {
				converted.Values = append(converted.Values, valueToProto(element))
			}
			out.FeatureValues[name] = converted
		}
	}
	return out
}
