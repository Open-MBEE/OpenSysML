package wasm

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/frontend/engine"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/jsonrpc"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func TestEngineRenderView(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "render-view.sysml"))
	if err != nil {
		t.Fatalf("reading RenderView fixture: %v", err)
	}
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	parsedBody := mustCall(t, eng, "ParseSources", renderParamsJSON(t, map[string]any{
		"documents": []map[string]string{{"name": "render-view.sysml", "content": string(src)}},
	}))
	var parsed engine.JParseSourcesResponse
	if err := json.Unmarshal(parsedBody, &parsed); err != nil {
		t.Fatalf("decoding ParseSources: %v\n%s", err, parsedBody)
	}
	if parsed.ModelHash == "" || len(parsed.Diagnostics) != 0 {
		t.Fatalf("ParseSources = %+v, want a hash and no diagnostics", parsed)
	}

	declared := engineRenderView(t, eng, parsed.ModelHash, "RenderViews::link", "", "edge-labels")
	if declared.Kind != "interconnection" {
		t.Errorf("declared view kind = %q, want interconnection", declared.Kind)
	}
	if len(declared.Notices) != 0 {
		t.Errorf("notices = %#v, want none", declared.Notices)
	}
	var root engine.JRenderNode
	parts := map[string]engine.JRenderNode{}
	for _, node := range declared.Nodes {
		if strings.HasSuffix(node.Name, "::Link") || node.Name == "Link" {
			root = node
		}
		if node.Name == "sender" || node.Name == "receiver" {
			parts[node.Name] = node
		}
	}
	if root.ID == "" || root.Parent != "" {
		t.Fatalf("Link root = %+v, want a root node", root)
	}
	if len(parts) != 2 || parts["sender"].Parent != root.ID || parts["receiver"].Parent != root.ID {
		t.Errorf("part parents: root=%+v parts=%+v, want sender and receiver under Link", root, parts)
	}
	if len(declared.Edges) != 1 {
		t.Fatalf("declared edges = %+v, want the wire connector", declared.Edges)
	}
	edge := declared.Edges[0]
	if edge.Label != "wire" || edge.Kind != "connection" ||
		edge.From != parts["sender"].ID || edge.To != parts["receiver"].ID {
		t.Errorf("declared edge = %+v, want sender to receiver labelled wire", edge)
	}

	pseudo := engineRenderView(t, eng, parsed.ModelHash, "#interconnection:RenderDemo::Link", "")
	if pseudo.Kind != "interconnection" ||
		pseudo.Stated != "no view declared; rendering RenderDemo::Link directly" {
		t.Errorf("targeted interconnection = kind %q, stated %q", pseudo.Kind, pseudo.Stated)
	}
	tree := engineRenderView(t, eng, parsed.ModelHash, "#tree:RenderDemo::Link", "")
	if tree.Kind != "tree" {
		t.Errorf("#tree kind = %q, want tree", tree.Kind)
	}
	matrix := engineRenderView(t, eng, parsed.ModelHash, "RenderViews::relationshipMatrix", "")
	if matrix.Kind != string(view.KindMatrix) || len(matrix.Columns) < 2 || len(matrix.Rows) == 0 {
		t.Fatalf("declared matrix = kind %q, columns %v, rows %v", matrix.Kind, matrix.Columns, matrix.Rows)
	}
	if len(matrix.Rows[0].Cells) != len(matrix.Columns) {
		t.Fatalf("matrix first row has %d cells, want %d columns", len(matrix.Rows[0].Cells), len(matrix.Columns))
	}
	hasConnect := false
	for _, row := range matrix.Rows {
		for _, cell := range row.Cells {
			if cell == "connect" {
				hasConnect = true
			}
		}
	}
	if !hasConnect {
		t.Fatalf("matrix rows contain no connect relationship: %v", matrix.Rows)
	}
	pseudoMatrix := engineRenderView(t, eng, parsed.ModelHash, "#matrix:RenderDemo::Link", "")
	if pseudoMatrix.Kind != string(view.KindMatrix) || len(pseudoMatrix.Columns) < 2 || len(pseudoMatrix.Rows) == 0 {
		t.Fatalf("matrix pseudo-view = kind %q, columns %v, rows %v", pseudoMatrix.Kind, pseudoMatrix.Columns, pseudoMatrix.Rows)
	}

	ws := model.NewWorkspace()
	ws.Open("render-view.sysml", src, 1)
	for _, viewName := range []string{
		"RenderViews::link", "#interconnection:RenderDemo::Link",
		"RenderViews::relationshipMatrix", "#matrix:RenderDemo::Link",
	} {
		for _, display := range []string{"minimal", "full"} {
			rendering, _, err := ws.RenderView("render-view.sysml", viewName)
			if err != nil {
				t.Fatalf("Workspace.RenderView(%q): %v", viewName, err)
			}
			data := rendering.DataWith(view.Options{Ports: view.Ports(display)})
			got := engineRenderView(t, eng, parsed.ModelHash, viewName, display)
			assertRenderDataParity(t, got, data)
		}
	}

	for _, test := range []struct {
		name      string
		modelHash string
		view      string
		ports     string
		code      uint32
		message   string
	}{
		{name: "unknown model hash", modelHash: "missing", view: "RenderViews::link", code: jsonrpc.CodeNotFound},
		{name: "empty view", modelHash: parsed.ModelHash, code: jsonrpc.CodeInvalidArgument},
		{name: "unknown view", modelHash: parsed.ModelHash, view: "RenderViews::missing", code: jsonrpc.CodeNotFound},
		{name: "non-view symbol", modelHash: parsed.ModelHash, view: "RenderDemo::Link", code: jsonrpc.CodeInvalidArgument},
		{name: "unsupported rendering", modelHash: parsed.ModelHash, view: "RenderViews::geometryView", code: jsonrpc.CodeInvalidArgument},
		{name: "invalid pseudo-view", modelHash: parsed.ModelHash, view: "#unknown:RenderDemo::Link", code: jsonrpc.CodeInvalidArgument, message: "#interconnection"},
		{name: "untargeted pseudo-view", modelHash: parsed.ModelHash, view: "#interconnection", code: jsonrpc.CodeInvalidArgument, message: "name an element"},
		{name: "unknown pseudo-view target", modelHash: parsed.ModelHash, view: "#tree:RenderDemo::Missing", code: jsonrpc.CodeNotFound},
		{name: "unknown port display", modelHash: parsed.ModelHash, view: "RenderViews::link", ports: "all", code: jsonrpc.CodeInvalidArgument, message: `unknown port display "all"; the displays are minimal, full`},
	} {
		t.Run(test.name, func(t *testing.T) {
			params := map[string]string{"modelHash": test.modelHash, "view": test.view, "ports": test.ports}
			_, err := eng.Call(context.Background(), "RenderView", []byte(renderParamsJSON(t, params)))
			var status *engine.Error
			if !errors.As(err, &status) {
				t.Fatalf("RenderView error = %v, want status error %d", err, test.code)
			}
			if status.Code != test.code {
				t.Errorf("status code = %d, want %d (%s)", status.Code, test.code, status.Message)
			}
			if test.message != "" && !strings.Contains(status.Message, test.message) {
				t.Errorf("status message = %q, want it to contain %q", status.Message, test.message)
			}
		})
	}
}

func engineRenderView(t *testing.T, eng *engine.Engine, hash, viewName, ports string, show ...string) engine.JRenderViewResponse {
	t.Helper()
	params := map[string]any{"modelHash": hash, "view": viewName, "ports": ports, "show": show}
	body := mustCall(t, eng, "RenderView", renderParamsJSON(t, params))
	var response engine.JRenderViewResponse
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decoding RenderView: %v\n%s", err, body)
	}
	return response
}

// assertRenderDataParity checks the engine's reply against the workspace's
// rendering of the same view, field for field but for declaration origins,
// which the service parity test compares against the gRPC answer.
func assertRenderDataParity(t *testing.T, got engine.JRenderViewResponse, want view.Data) {
	t.Helper()
	if got.View != want.View || got.Kind != string(want.Kind) || got.Stated != want.Stated {
		t.Errorf("render metadata = %q/%q/%q, workspace has %q/%q/%q",
			got.View, got.Kind, got.Stated, want.View, want.Kind, want.Stated)
	}
	if !reflect.DeepEqual(got.Columns, want.Columns) {
		t.Errorf("engine columns differ from workspace data: engine=%v workspace=%v", got.Columns, want.Columns)
	}
	var rows []engine.JRenderRow
	for _, row := range want.Rows {
		rows = append(rows, engine.JRenderRow{Cells: row.Cells})
	}
	for i := range got.Rows {
		got.Rows[i].Origin = nil
	}
	if !reflect.DeepEqual(got.Rows, rows) {
		t.Errorf("engine rows differ from workspace data: engine=%+v workspace=%+v", got.Rows, rows)
	}
	var nodes []engine.JRenderNode
	for _, node := range want.Nodes {
		item := engine.JRenderNode{
			ID: node.ID, Kind: node.Kind, Name: node.Name, NameSynthesized: node.NameSynthesized,
			Type: node.Type, Detail: node.Detail, Text: node.Text, StandIn: node.StandIn, Parent: node.Parent,
			Style: renderStyleJSON(node.Style), Inherited: node.Inherited,
		}
		if node.Geometry != nil {
			item.Geometry = &engine.JRenderGeometry{
				X: node.Geometry.X, Y: node.Geometry.Y, Width: node.Geometry.Width, Height: node.Geometry.Height,
				HasSize: node.Geometry.HasSize, Collapsed: node.Geometry.Collapsed,
			}
		}
		for _, port := range node.Ports {
			item.Ports = append(item.Ports, engine.JRenderPort{
				ID: port.ID, Name: port.Name, Type: port.Type, Direction: port.Direction.String(), Inherited: port.Inherited,
			})
		}
		nodes = append(nodes, item)
	}
	for i := range got.Nodes {
		got.Nodes[i].Origin = nil
	}
	if !reflect.DeepEqual(got.Nodes, nodes) {
		t.Errorf("engine nodes differ from workspace data:\nengine: %+v\nworkspace: %+v", got.Nodes, nodes)
	}
	var edges []engine.JRenderEdge
	for _, edge := range want.Edges {
		item := engine.JRenderEdge{
			From: edge.From, To: edge.To, FromPort: edge.FromPort, ToPort: edge.ToPort,
			Label: edge.Text(), Name: edge.Name, Kind: edge.Kind.String(), Style: renderStyleJSON(edge.Style),
		}
		for _, point := range edge.Route {
			item.Route = append(item.Route, engine.JRenderPoint{X: point.X, Y: point.Y})
		}
		edges = append(edges, item)
	}
	for i := range got.Edges {
		got.Edges[i].Origin = nil
	}
	if !reflect.DeepEqual(got.Edges, edges) {
		t.Errorf("engine edges differ from workspace data:\nengine: %+v\nworkspace: %+v", got.Edges, edges)
	}
}

func renderStyleJSON(style *view.Style) *engine.JRenderStyle {
	if style == nil {
		return nil
	}
	return &engine.JRenderStyle{
		Fill: style.Fill, Line: style.Line, Text: style.Text, Font: style.Font,
		FontSize: style.FontSize, Bold: style.Bold, Italic: style.Italic,
	}
}

func renderParamsJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("encoding JSON: %v", err)
	}
	return string(data)
}
