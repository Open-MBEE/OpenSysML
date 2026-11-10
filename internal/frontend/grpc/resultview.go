package grpc

import (
	"errors"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docir"
	"github.com/Open-MBEE/OpenSysML/internal/doc/docrender"
	"github.com/Open-MBEE/OpenSysML/internal/doc/queryexec"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
)

// CapabilityQueryResultView names the as_view fields of RunDocumentQueryRequest and QueryRequest,
// which write an answer as a QVTS::QueryResult view, and RenderView's header and form for one.
const CapabilityQueryResultView = "query_result_view"

// modelSource prefixes the hash a result view names a model by when its
// documents were parsed from content and have no file.
const modelSource = "model:"

// contentName is what a document parsed from request content is named, having no file.
const contentName = "<content>"

// resultViewSpec states a run over a cached model: its source the files the
// model was parsed from, or its hash when it has none, its provenance the hash.
func resultViewSpec(cached *CachedModel, hash, name, query string) queryexec.ResultView {
	var files []string
	for _, doc := range cached.Documents {
		if doc.Source != nil && doc.Source.Name() != "" && doc.Source.Name() != contentName {
			files = append(files, doc.Source.Name())
		}
	}
	origin := modelSource + hash
	if len(files) > 0 {
		origin = strings.Join(files, " ")
	}
	return queryexec.ResultView{Name: name, Query: query, Source: origin, Provenance: modelSource + hash}
}

// withResultView attaches a failed query's view to its status as a
// QueryResultView detail, beside the QueryOutcome it is written from.
func withResultView(status error, text string) error {
	var connErr *connect.Error
	if text == "" || !errors.As(status, &connErr) {
		return status
	}
	detail, err := connect.NewErrorDetail(&pb.QueryResultView{Text: text})
	if err != nil {
		return status
	}
	connErr.AddDetail(detail)
	return connErr
}

// failedResultView is the view of a run that failed with err, empty when err
// carries no path or model error.
func failedResultView(spec queryexec.ResultView, qctx queryexec.Context, err error) string {
	text, _ := spec.Write(qctx, nil, err)
	return text
}

// bindingsText writes a request's bindings as %run-query takes them,
// `<parameter>=<value>` in request order, after the query's name.
func bindingsText(name string, bindings []*pb.DocumentQueryBinding) string {
	parts := []string{name}
	for _, binding := range bindings {
		for _, value := range binding.GetValues() {
			parts = append(parts, binding.GetParameter()+"="+bindingValueText(value))
		}
	}
	return strings.Join(parts, " ")
}

func bindingValueText(value *pb.DocumentValue) string {
	switch kind := value.GetKind().(type) {
	case *pb.DocumentValue_ElementId:
		return kind.ElementId
	case *pb.DocumentValue_StringValue:
		return strconv.Quote(kind.StringValue)
	case *pb.DocumentValue_IntValue:
		return strconv.FormatInt(kind.IntValue, 10)
	case *pb.DocumentValue_RealValue:
		return strconv.FormatFloat(kind.RealValue, 'g', -1, 64)
	case *pb.DocumentValue_BoolValue:
		return strconv.FormatBool(kind.BoolValue)
	case *pb.DocumentValue_BigIntValue:
		return kind.BigIntValue
	case *pb.DocumentValue_RationalValue:
		return rationalText(kind.RationalValue)
	case *pb.DocumentValue_Quantity:
		return quantityText(kind.Quantity)
	case *pb.DocumentValue_Object:
		if kind.Object.GetPath() != "" {
			return kind.Object.GetPath()
		}
		return "#" + strconv.FormatInt(kind.Object.GetInstanceId(), 10)
	default:
		return "?"
	}
}

func rationalText(r *pb.Rational) string {
	return r.GetNumerator() + "/" + r.GetDenominator()
}

func quantityText(q *pb.Quantity) string {
	var magnitude string
	switch m := q.GetMagnitude().(type) {
	case *pb.Quantity_IntMagnitude:
		magnitude = strconv.FormatInt(m.IntMagnitude, 10)
	case *pb.Quantity_RealMagnitude:
		magnitude = strconv.FormatFloat(m.RealMagnitude, 'g', -1, 64)
	case *pb.Quantity_BigIntMagnitude:
		magnitude = m.BigIntMagnitude
	case *pb.Quantity_RationalMagnitude:
		magnitude = rationalText(m.RationalMagnitude)
	}
	return magnitude + " [" + q.GetUnit() + "]"
}

// apiQueryText writes a SysML v2 API Query as the text its view states:
// `scope=…; select=…; where=…; orderBy=…`, each part present when the query has it.
func apiQueryText(q *pb.Query) string {
	var parts []string
	if len(q.GetScope()) > 0 {
		parts = append(parts, "scope="+strings.Join(q.GetScope(), ","))
	}
	if len(q.GetSelect()) > 0 {
		parts = append(parts, "select="+strings.Join(q.GetSelect(), ","))
	}
	if q.GetWhere() != nil {
		parts = append(parts, "where="+constraintText(q.GetWhere()))
	}
	if len(q.GetOrderBy()) > 0 {
		parts = append(parts, "orderBy="+strings.Join(q.GetOrderBy(), ","))
	}
	return strings.Join(parts, "; ")
}

var primitiveOperatorText = map[pb.PrimitiveOperator]string{
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_EQUAL:         "=",
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER:       ">",
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS:          "<",
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_LESS_EQUAL:    "<=",
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_GREATER_EQUAL: ">=",
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_IN:            " in ",
	pb.PrimitiveOperator_PRIMITIVE_OPERATOR_INSTANCE_OF:   " instanceOf ",
}

func constraintText(c *pb.Constraint) string {
	if primitive := c.GetPrimitive(); primitive != nil {
		values := make([]string, len(primitive.GetValue()))
		for i, value := range primitive.GetValue() {
			values[i] = strconv.Quote(value)
		}
		operand := strings.Join(values, ",")
		if len(values) != 1 {
			operand = "[" + operand + "]"
		}
		text := primitive.GetProperty() + primitiveOperatorText[primitive.GetOperator()] + operand
		if primitive.GetInverse() {
			return "not(" + text + ")"
		}
		return text
	}
	composite := c.GetComposite()
	operator := " and "
	if composite.GetOperator() == pb.CompositeOperator_COMPOSITE_OPERATOR_OR {
		operator = " or "
	}
	terms := make([]string, len(composite.GetConstraint()))
	for i, term := range composite.GetConstraint() {
		terms[i] = constraintText(term)
	}
	return "(" + strings.Join(terms, operator) + ")"
}

// queryResultForm renders a QVTS::QueryResult view's rendering as a document in
// form into the response's field of that name.
func queryResultForm(response *pb.RenderViewResponse, rendering *view.Rendering, form string) error {
	document := docir.QueryResult(rendering)
	var err error
	switch form {
	case renderFormMarkdown:
		response.Markdown, err = docrender.Markdown(document, docrender.MarkdownOptions{})
	case renderFormHTML:
		response.Html, err = docrender.HTML(document, docrender.HTMLOptions{})
	case renderFormAsciiDoc:
		response.Asciidoc, err = docrender.AsciiDoc(document, docrender.AsciiDocOptions{})
	default:
		return statusErrorf(connect.CodeInvalidArgument,
			"form %q is not one this service renders: markdown, html or asciidoc", form)
	}
	if err != nil {
		return documentStatus(err)
	}
	return nil
}
