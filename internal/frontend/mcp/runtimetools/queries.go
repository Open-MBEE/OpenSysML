package runtimetools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/mcp/toolkit"
)

// QueryInput is a SysML v2 API & Services Query, or OSLC query text.
type QueryInput struct {
	ModelInput
	Query     map[string]any `json:"query,omitempty" jsonschema:"the standard's Query as protojson writes it: {\"scope\":[qualified names],\"select\":[properties],\"where\":{\"primitive\":{\"property\":\"@type\",\"operator\":\"PRIMITIVE_OPERATOR_EQUAL\",\"value\":[\"PartUsage\"]}},\"orderBy\":[properties]} or a composite {\"composite\":{\"operator\":\"COMPOSITE_OPERATOR_AND\",\"constraint\":[at least two]}}; operators are PRIMITIVE_OPERATOR_EQUAL, _LESS, _LESS_EQUAL, _GREATER, _GREATER_EQUAL, _IN (value is a list) and _INSTANCE_OF (on @type, value names metaclasses); orderBy sorts ascending by each property in turn; empty considers the whole model"`
	OSLCQuery string         `json:"oslcQuery,omitempty" jsonschema:"OSLC Query 3.0 parameter text, instead of query"`
}

// RunDocumentQueryInput runs a named document query.
type RunDocumentQueryInput struct {
	ModelInput
	QueryID  string           `json:"queryId" jsonschema:"qualified name of the document query, a calc def specializing DocumentQueries::Query"`
	Bindings []map[string]any `json:"bindings,omitempty" jsonschema:"entry-parameter bindings, each a protojson DocumentQueryBinding {\"parameter\":name,\"values\":[DocumentValue]} where a DocumentValue is {\"elementId\":qualified name}, {\"stringValue\":..}, {\"intValue\":\"..\"}, {\"realValue\":..}, {\"boolValue\":..} or {\"object\":{\"path\":\"Garage::car\"}}"`
}

// RenderDocumentInput renders a named document.
type RenderDocumentInput struct {
	ModelInput
	DocumentID string `json:"documentId" jsonschema:"qualified name of the document, a part def specializing DocumentQueries::Document"`
	Form       string `json:"form,omitempty" jsonschema:"markdown (the default), html or asciidoc"`
}

// RenderViewInput renders a view.
type RenderViewInput struct {
	ModelInput
	View         string   `json:"view" jsonschema:"a qualified view name, or a #<kind>:<qualified name> pseudo-view targeting an element (#interconnection:Demo::Vehicle, #tree:Demo, #state:Demo::Lamp, #action:Demo::Descend, #sequence:..., #case:..., #requirement:..., #mixed:...)"`
	Ports        string   `json:"ports,omitempty" jsonschema:"minimal (the default) or full: how many of a part's ports are drawn"`
	Show         []string `json:"show,omitempty" jsonschema:"optional notation elements to draw besides those the view's rendering shows, as sysml -render-show takes them: parameter-symbols, port-symbols, compartments, edge-labels, keywords, multiplicities, typings, qualifiers, or all"`
	Compartments string   `json:"compartments,omitempty" jsonschema:"how a tree rendering folds a node's owned members into compartment rows of its parent: positioned (the default), most or ports, as sysml -render-compartments takes them"`
	Depth        int      `json:"depth,omitempty" jsonschema:"how many levels of nested elements to draw below each exposed element, overriding the view's own depth; 0 or absent draws every level"`
}

// ExportGraphsInput names a behavior to export.
type ExportGraphsInput struct {
	ModelInput
	Subject string `json:"subject" jsonschema:"qualified name of an action or state machine, definition or usage"`
}

func (g *group) registerQueries(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name: "query",
		Description: "Select elements of a parsed model with a SysML v2 API & Services Query — scope, select, where, orderBy — or OSLC query " +
			"text (the Query RPC). Each matched element answers its qualified name, metamodel type and the properties select asked " +
			"for. A query the service cannot evaluate fails with INVALID_ARGUMENT rather than answering empty. The result is the " +
			"QueryResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Query model"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in QueryInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		req := &pb.QueryRequest{}
		if err := unmarshal(in, m.hash, req); err != nil {
			return nil, nil, err
		}
		if req.GetOslcQuery() == "" && req.GetQuery() == nil {
			req.Query = &pb.Query{}
		}
		resp, err := g.svc.Query(ctx, req)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return answer(resp, m, queryText(resp))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "run_document_query",
		Description: "Run a named document query with parameter bindings and answer its typed rows (the RunDocumentQuery RPC, the " +
			"REPL's %run-query). The query runs over the model and over the objects instantiate created for the same modelHash, which " +
			"a binding may name. The result is the RunDocumentQueryResponse as protojson writes it; the text is the table.",
		Annotations: toolkit.ReadOnly("Run document query"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in RunDocumentQueryInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		req := &pb.RunDocumentQueryRequest{}
		if err := unmarshal(in, m.hash, req); err != nil {
			return nil, nil, err
		}
		resp, err := g.svc.RunDocumentQuery(ctx, req)
		if err != nil {
			return nil, nil, toolError(err)
		}
		return answer(resp, m, documentQueryText(resp))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "render_model_document",
		Description: "Render a named document of a model given by modelHash, paths or sources (the workspace group's render_document renders by name over the root) — a part def specializing DocumentQueries::Document — to Markdown, HTML or AsciiDoc (the " +
			"RenderDocument RPC, the CLI's -render-document), byte for byte what the CLI writes. The text is the rendered document; " +
			"the result is the RenderDocumentResponse as protojson writes it.",
		Annotations: toolkit.ReadOnly("Render document"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in RenderDocumentInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		resp, err := g.svc.RenderDocument(ctx, &pb.RenderDocumentRequest{ModelHash: m.hash, DocumentId: in.DocumentID, Form: in.Form})
		if err != nil {
			return nil, nil, toolError(err)
		}
		text := resp.GetMarkdown()
		if text == "" {
			text = resp.GetHtml()
		}
		if text == "" {
			text = resp.GetAsciidoc()
		}
		return answer(resp, m, text)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "render_model_view",
		Description: "Render a declared view or a targeted pseudo-view of a model given by modelHash, paths or sources as diagram data (the RenderView RPC; the workspace group's render_view renders by name over the root): nodes flattened " +
			"parents first, edges with their ports, table columns and rows, canvas, notes and notices. The result is the " +
			"RenderViewResponse as protojson writes it; the text lists the nodes and edges.",
		Annotations: toolkit.ReadOnly("Render view"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in RenderViewInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		resp, err := g.svc.RenderView(ctx, &pb.RenderViewRequest{ModelHash: m.hash, View: in.View, Ports: in.Ports, Show: in.Show, Compartments: in.Compartments})
		if err != nil {
			return nil, nil, toolError(err)
		}
		return answer(resp, m, viewText(resp))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name: "export_graphs",
		Description: "Export the lowered graph of an action or state machine — the subject's and every behavior it performs — as " +
			"the canonical graphs:<version> JSON external analysis engines read (the ExportGraphs RPC). The result is the " +
			"ExportGraphsResponse as protojson writes it, the JSON in `content`; the text names the subject and version on its first " +
			"line and carries the JSON after it.",
		Annotations: toolkit.ReadOnly("Export graphs"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ExportGraphsInput) (*mcp.CallToolResult, Result, error) {
		m, err := g.resolve(ctx, in.ModelInput)
		if err != nil {
			return nil, nil, err
		}
		resp, err := g.svc.ExportGraphs(ctx, &pb.ExportGraphsRequest{ModelHash: m.hash, Subject: in.Subject})
		if err != nil {
			return nil, nil, toolError(err)
		}
		return answer(resp, m, fmt.Sprintf("graphs:%d of %s\n%s", resp.GetVersion(), resp.GetSubject(), resp.GetContent()))
	})
}

// queryText lists the elements a query matched, each with the properties selected.
func queryText(resp *pb.QueryResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d elements\n", len(resp.GetElements()))
	for _, e := range resp.GetElements() {
		fmt.Fprintf(&b, "%s (%s)", e.GetId(), e.GetType())
		names := make([]string, 0, len(e.GetProperties()))
		for name := range e.GetProperties() {
			names = append(names, name)
		}
		sortStrings(names)
		for _, name := range names {
			fmt.Fprintf(&b, " %s=%s", name, e.GetProperties()[name])
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// documentQueryText tables the rows a document query answered.
func documentQueryText(resp *pb.RunDocumentQueryResponse) string {
	var b strings.Builder
	columns := make([]string, 0, len(resp.GetColumns()))
	for _, c := range resp.GetColumns() {
		columns = append(columns, c.GetName())
	}
	fmt.Fprintf(&b, "%d rows; columns: element | %s\n", len(resp.GetRows()), strings.Join(columns, " | "))
	for _, row := range resp.GetRows() {
		cells := []string{documentValueText(row.GetElement())}
		for _, cell := range row.GetCells() {
			values := make([]string, 0, len(cell.GetValues()))
			for _, v := range cell.GetValues() {
				values = append(values, documentValueText(v))
			}
			cells = append(cells, strings.Join(values, ", "))
		}
		b.WriteString(strings.Join(cells, " | ") + "\n")
	}
	return b.String()
}

// viewText lists the nodes, edges, rows and notices of a rendered view.
func viewText(resp *pb.RenderViewResponse) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s view %s: %d nodes, %d edges", resp.GetKind(), resp.GetView(), len(resp.GetNodes()), len(resp.GetEdges()))
	if len(resp.GetRows()) > 0 {
		fmt.Fprintf(&b, ", %d rows", len(resp.GetRows()))
	}
	b.WriteByte('\n')
	for _, n := range resp.GetNodes() {
		fmt.Fprintf(&b, "node %s: %s %s", n.GetId(), n.GetKind(), n.GetName())
		if n.GetType() != "" {
			fmt.Fprintf(&b, " : %s", n.GetType())
		}
		if n.GetParent() != "" {
			fmt.Fprintf(&b, " in %s", n.GetParent())
		}
		b.WriteByte('\n')
	}
	for _, e := range resp.GetEdges() {
		fmt.Fprintf(&b, "edge %s -> %s", e.GetFrom(), e.GetTo())
		if e.GetKind() != "" {
			fmt.Fprintf(&b, " (%s)", e.GetKind())
		}
		if e.GetLabel() != "" {
			fmt.Fprintf(&b, " %q", e.GetLabel())
		}
		b.WriteByte('\n')
	}
	if len(resp.GetColumns()) > 0 {
		b.WriteString(strings.Join(resp.GetColumns(), " | ") + "\n")
	}
	for _, r := range resp.GetRows() {
		b.WriteString(strings.Join(r.GetCells(), " | ") + "\n")
	}
	for _, n := range resp.GetNotices() {
		b.WriteString("notice: " + n + "\n")
	}
	return b.String()
}

// documentValueText spells a document-query value as %run-query prints one.
func documentValueText(v *pb.DocumentValue) string {
	if v == nil {
		return ""
	}
	switch k := v.GetKind().(type) {
	case *pb.DocumentValue_ElementId:
		return k.ElementId
	case *pb.DocumentValue_StringValue:
		return fmt.Sprintf("%q", k.StringValue)
	case *pb.DocumentValue_IntValue:
		return fmt.Sprint(k.IntValue)
	case *pb.DocumentValue_RealValue:
		return fmt.Sprint(k.RealValue)
	case *pb.DocumentValue_BoolValue:
		return fmt.Sprint(k.BoolValue)
	case *pb.DocumentValue_Infinity:
		return "*"
	case *pb.DocumentValue_Quantity:
		return quantityText(k.Quantity)
	case *pb.DocumentValue_BigIntValue:
		return k.BigIntValue
	case *pb.DocumentValue_RationalValue:
		return k.RationalValue.GetNumerator() + "/" + k.RationalValue.GetDenominator()
	case *pb.DocumentValue_Verdict:
		return fmt.Sprintf("%s %s on %s: %s", k.Verdict.GetKind(), k.Verdict.GetText(), k.Verdict.GetPath(), k.Verdict.GetVerdict())
	case *pb.DocumentValue_Object:
		if k.Object.GetPath() != "" {
			return k.Object.GetPath()
		}
		return fmt.Sprintf("#%d", k.Object.GetInstanceId())
	case *pb.DocumentValue_State:
		return k.State.GetMachine() + " in " + k.State.GetStatePath()
	case *pb.DocumentValue_Event:
		return k.Event.GetText()
	}
	return ""
}
