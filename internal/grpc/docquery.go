package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/core/docir"
	"github.com/Open-MBEE/OpenSysML/internal/core/docplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/core/libs"
	"github.com/Open-MBEE/OpenSysML/internal/core/model"
	"github.com/Open-MBEE/OpenSysML/internal/core/provenance"
	corequery "github.com/Open-MBEE/OpenSysML/internal/core/query"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/core/queryplan"
	"github.com/Open-MBEE/OpenSysML/internal/core/semantics"
	"github.com/Open-MBEE/OpenSysML/internal/core/source"
	"github.com/Open-MBEE/OpenSysML/internal/core/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/core/view"
)

// RunDocumentQuery runs a named document query with parameter bindings, the
// answer %run-query gives, as typed rows rather than formatted lines.
func (s *Service) RunDocumentQuery(ctx context.Context, req *pb.RunDocumentQueryRequest) (*pb.RunDocumentQueryResponse, error) {
	if err := s.requireCapability(CapabilityDocumentQuery); err != nil {
		return nil, err
	}
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}
	if req.QueryId == "" {
		return nil, statusError(connect.CodeInvalidArgument, "a document query to run must be named in query_id")
	}
	sc := cached.SymbolContext()
	defer sc.Lock()()
	sym, err := documentSymbol(sc.Index, req.QueryId)
	if err != nil {
		return nil, err
	}
	if !queryplan.IsQueryDefinition(sc.Index, sc.Semantics, sym) {
		return nil, statusErrorf(connect.CodeInvalidArgument,
			"%s is not a document query: one is a calc def specializing DocumentQueries::Query", req.QueryId)
	}
	program, err := queryplan.Compile(sc.Index, sc.Semantics, sc.Resolver, sym)
	if err != nil {
		return nil, documentStatus(err)
	}
	bindings, err := documentBindings(sc.Index, sc.Semantics, req.Bindings)
	if err != nil {
		return nil, err
	}
	result, err := queryexec.Execute(program,
		queryexec.Context{Index: sc.Index, Resolver: sc.Resolver, Model: sc.Semantics},
		bindings, queryexec.Options{})
	if err != nil {
		return nil, documentStatus(err)
	}
	return rowSetResponse(sc.Index, result), nil
}

// RenderDocument renders a named document to Markdown, as -render-document does.
func (s *Service) RenderDocument(ctx context.Context, req *pb.RenderDocumentRequest) (*pb.RenderDocumentResponse, error) {
	if err := s.requireCapability(CapabilityRenderDocument); err != nil {
		return nil, err
	}
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}
	if req.DocumentId == "" {
		return nil, statusError(connect.CodeInvalidArgument, "a document to render must be named in document_id")
	}
	sc := cached.SymbolContext()
	defer sc.Lock()()
	sym, err := documentSymbol(sc.Index, req.DocumentId)
	if err != nil {
		return nil, err
	}
	if !docplan.IsDocumentDefinition(sc.Index, sc.Semantics, sym) {
		return nil, statusErrorf(connect.CodeInvalidArgument,
			"%s is not a document: one is a part def specializing DocumentQueries::Document", req.DocumentId)
	}
	plan, err := docplan.Compile(sc.Index, sc.Semantics, sc.Resolver, sym)
	if err != nil {
		return nil, documentStatus(err)
	}
	document, err := docir.EvaluateLinked(plan,
		model.SiblingDocumentPlans(sc.Index, sc.Semantics, sc.Resolver, sym),
		queryexec.Context{Index: sc.Index, Resolver: sc.Resolver, Model: sc.Semantics},
		queryexec.Options{}, cachedSourceText(cached))
	if err != nil {
		return nil, documentStatus(err)
	}
	markdown, err := docrender.Markdown(document, docrender.MarkdownOptions{})
	if err != nil {
		return nil, documentStatus(err)
	}
	return &pb.RenderDocumentResponse{Markdown: markdown}, nil
}

// documentSymbol resolves the query or document a request names, failing with
// NOT_FOUND when the model does not declare it.
func documentSymbol(idx *symbols.Index, id string) (*symbols.Symbol, error) {
	syms := lookupNamed(idx, id)
	if len(syms) == 0 {
		return nil, statusErrorf(connect.CodeNotFound, "symbol not found: %s", id)
	}
	return syms[0], nil
}

// documentBindings converts a request's typed bindings into the engine's.
// Repeated parameters append, as %run-query's repeated bindings do.
func documentBindings(idx *symbols.Index, sem *semantics.Model, bindings []*pb.DocumentQueryBinding) (queryexec.Bindings, error) {
	if len(bindings) == 0 {
		return nil, nil
	}
	out := make(queryexec.Bindings, len(bindings))
	for _, binding := range bindings {
		if binding.GetParameter() == "" {
			return nil, statusError(connect.CodeInvalidArgument, "a binding must name the parameter it binds")
		}
		for _, value := range binding.GetValues() {
			bound, err := boundValue(idx, sem, binding.GetParameter(), value)
			if err != nil {
				return nil, err
			}
			out[binding.Parameter] = append(out[binding.Parameter], bound)
		}
	}
	return out, nil
}

// boundValue converts one request value. An element is bound by qualified name;
// infinity is only ever answered, so binding it is refused.
func boundValue(idx *symbols.Index, sem *semantics.Model, parameter string, value *pb.DocumentValue) (queryexec.Value, error) {
	switch kind := value.GetKind().(type) {
	case *pb.DocumentValue_ElementId:
		syms := lookupNamed(idx, kind.ElementId)
		if len(syms) == 0 {
			return queryexec.Value{}, statusErrorf(connect.CodeInvalidArgument,
				"binding %s names an element the model does not have: %q", parameter, kind.ElementId)
		}
		return queryexec.ElementValue(syms[0]), nil
	case *pb.DocumentValue_StringValue:
		return queryexec.StringValue(kind.StringValue), nil
	case *pb.DocumentValue_IntValue:
		return queryexec.IntegerValue(kind.IntValue), nil
	case *pb.DocumentValue_RealValue:
		return queryexec.RealValue(kind.RealValue), nil
	case *pb.DocumentValue_BoolValue:
		return queryexec.BooleanValue(kind.BoolValue), nil
	case *pb.DocumentValue_Infinity:
		return queryexec.Value{}, statusErrorf(connect.CodeInvalidArgument,
			"binding %s: infinity is answered by queries, not bound to them", parameter)
	case *pb.DocumentValue_Quantity:
		bound, err := ProtoToQuantity(kind.Quantity, idx, sem)
		if err != nil {
			return queryexec.Value{}, statusErrorf(connect.CodeInvalidArgument, "binding %s: %v", parameter, err)
		}
		quantity := bound.Quantity()
		if quantity == nil {
			return queryexec.Value{}, statusErrorf(connect.CodeInvalidArgument, "binding %s carries no quantity", parameter)
		}
		return queryexec.QuantityValue(*quantity), nil
	default:
		return queryexec.Value{}, statusErrorf(connect.CodeInvalidArgument,
			"binding %s carries no value", parameter)
	}
}

// rowSetResponse converts an executed row set, keeping the engine's order.
func rowSetResponse(idx *symbols.Index, result *queryexec.RowSet) *pb.RunDocumentQueryResponse {
	columns := result.Columns()
	response := &pb.RunDocumentQueryResponse{
		Columns: make([]*pb.DocumentQueryColumn, 0, len(columns)),
	}
	for _, column := range columns {
		response.Columns = append(response.Columns, &pb.DocumentQueryColumn{Name: column.Name()})
	}
	rows := result.Rows()
	response.Rows = make([]*pb.DocumentQueryRow, 0, len(rows))
	for _, row := range rows {
		cells := row.Cells()
		pbRow := &pb.DocumentQueryRow{
			Element: documentValue(idx, row.Element()),
			Cells:   make([]*pb.DocumentQueryCell, 0, len(cells)),
		}
		for _, cell := range cells {
			values := cell.Values()
			pbCell := &pb.DocumentQueryCell{Values: make([]*pb.DocumentValue, 0, len(values))}
			for _, value := range values {
				pbCell.Values = append(pbCell.Values, documentValue(idx, value))
			}
			pbRow.Cells = append(pbRow.Cells, pbCell)
		}
		response.Rows = append(response.Rows, pbRow)
	}
	return response
}

// documentValue converts one engine value, naming an element by qualified name
// and metamodel type.
func documentValue(idx *symbols.Index, value queryexec.Value) *pb.DocumentValue {
	switch value.Kind() {
	case queryexec.ValueElement:
		sym, ok := value.Element()
		if !ok {
			return &pb.DocumentValue{}
		}
		return &pb.DocumentValue{
			Kind:        &pb.DocumentValue_ElementId{ElementId: idx.GetFQN(sym)},
			ElementType: corequery.MetamodelTypeNameOf(sym),
		}
	case queryexec.ValueString:
		text, _ := value.String()
		return &pb.DocumentValue{Kind: &pb.DocumentValue_StringValue{StringValue: text}}
	case queryexec.ValueInteger:
		integer, _ := value.Integer()
		return &pb.DocumentValue{Kind: &pb.DocumentValue_IntValue{IntValue: integer}}
	case queryexec.ValueReal:
		realVal, _ := value.Real()
		return &pb.DocumentValue{Kind: &pb.DocumentValue_RealValue{RealValue: realVal}}
	case queryexec.ValueBoolean:
		boolean, _ := value.Boolean()
		return &pb.DocumentValue{Kind: &pb.DocumentValue_BoolValue{BoolValue: boolean}}
	case queryexec.ValueInfinity:
		return &pb.DocumentValue{Kind: &pb.DocumentValue_Infinity{Infinity: true}}
	case queryexec.ValueQuantity:
		quantity, _ := value.Quantity()
		return &pb.DocumentValue{Kind: &pb.DocumentValue_Quantity{Quantity: QuantityToProto(&quantity)}}
	default:
		return &pb.DocumentValue{}
	}
}

// documentStatus maps a typed engine failure onto the status code for it,
// keeping the engine's own message and appending the source it names.
func documentStatus(err error) error {
	var planErr *queryplan.Error
	if errors.As(err, &planErr) {
		return statusWithOrigin(queryPlanCode(planErr.Kind), err, planErr.Origin)
	}
	var execErr *queryexec.Error
	if errors.As(err, &execErr) {
		return statusWithOrigin(queryExecCode(execErr.Kind), err, execErr.Origin)
	}
	var docPlanErr *docplan.Error
	if errors.As(err, &docPlanErr) {
		return statusWithOrigin(docPlanCode(docPlanErr.Kind), err, docPlanErr.Origin)
	}
	var docIRErr *docir.Error
	if errors.As(err, &docIRErr) {
		return statusWithOrigin(docIRCode(docIRErr), err, docIRErr.Origin)
	}
	return connect.NewError(connect.CodeInternal, err)
}

// statusWithOrigin fails with the engine's message, naming the source
// declaration behind the failure when the engine reports one.
func statusWithOrigin(code connect.Code, err error, origin provenance.Origin) error {
	if !origin.Located() {
		return connect.NewError(code, err)
	}
	return statusErrorf(code, "%s (declared in %s)", err.Error(), origin.Doc)
}

// queryPlanCode is the status a query-planning failure reports as: a fault in
// the model's own query definitions is a failed precondition of the call.
func queryPlanCode(kind queryplan.ErrorKind) connect.Code {
	switch kind {
	case queryplan.ErrorNotQueryDefinition:
		return connect.CodeInvalidArgument
	case queryplan.ErrorLibraryUnavailable, queryplan.ErrorInvalidContext:
		return connect.CodeInternal
	default:
		return connect.CodeFailedPrecondition
	}
}

// queryExecCode is the status an execution failure reports as: a wrong binding
// is the caller's fault, an exhausted budget is a resource limit, and the rest
// are faults in the model's queries.
func queryExecCode(kind queryexec.ErrorKind) connect.Code {
	switch kind {
	case queryexec.ErrorUnknownBinding, queryexec.ErrorMissingBinding,
		queryexec.ErrorBindingType, queryexec.ErrorBindingMultiplicity:
		return connect.CodeInvalidArgument
	case queryexec.ErrorVisitBudget, queryexec.ErrorInvocationBudget,
		queryexec.ErrorInvocationDepth:
		return connect.CodeResourceExhausted
	case queryexec.ErrorInvalidContext:
		return connect.CodeInternal
	default:
		return connect.CodeFailedPrecondition
	}
}

// docPlanCode is the status a document-planning failure reports as.
func docPlanCode(kind docplan.ErrorKind) connect.Code {
	switch kind {
	case docplan.ErrorNotDocumentDefinition:
		return connect.CodeInvalidArgument
	case docplan.ErrorLibraryUnavailable, docplan.ErrorInvalidContext:
		return connect.CodeInternal
	default:
		return connect.CodeFailedPrecondition
	}
}

// cachedSourceText reads notation from whichever of the model's documents a span
// belongs to, and behind them from the library files its index holds, for the
// labels a diagram rendering takes verbatim.
func cachedSourceText(model *CachedModel) view.SourceText {
	sources := make(map[string]*source.SourceFile, len(model.Documents))
	for _, doc := range model.Documents {
		if doc.Source != nil {
			sources[doc.Source.Name()] = doc.Source
		}
	}
	if len(sources) == 0 && model.Library == nil {
		return nil
	}
	return source.TextOf(sources, libs.Text(model.Library))
}

// docIRCode is the status a document-evaluation failure reports as: a failed
// query keeps its execution mapping, and the rest are the evaluator's own.
func docIRCode(err *docir.Error) connect.Code {
	if err.Kind == docir.ErrorQueryExecution {
		var execErr *queryexec.Error
		if errors.As(err.Err, &execErr) {
			return queryExecCode(execErr.Kind)
		}
	}
	return connect.CodeInternal
}
