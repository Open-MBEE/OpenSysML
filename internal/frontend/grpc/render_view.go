package grpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/frontend/symbolfacts"
	"github.com/Open-MBEE/OpenSysML/internal/ir/view"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
	"github.com/Open-MBEE/OpenSysML/internal/syntax/source"
)

// RenderView renders a declared view or targeted pseudo-view as diagram data.
func (s *Service) RenderView(ctx context.Context, req *pb.RenderViewRequest) (*pb.RenderViewResponse, error) {
	if err := s.requireCapability(CapabilityRenderView); err != nil {
		return nil, err
	}
	cached, ok := s.cache.Get(req.ModelHash)
	if !ok {
		return nil, statusErrorf(connect.CodeNotFound, msgModelNotFound, req.ModelHash)
	}
	if req.View == "" {
		return nil, statusError(connect.CodeInvalidArgument, "view is required")
	}
	ports, ok := view.ParsePorts(req.Ports)
	if !ok {
		return nil, statusError(connect.CodeInvalidArgument, (&view.UnknownPortsError{Name: req.Ports}).Error())
	}

	worker, release := cached.worker()
	defer release()
	renderer := view.NewRenderer(worker.Model.Semantics(), worker.Model.Resolver(), cachedSourceText(cached))
	rendering, err := view.RenderNamed(renderer, req.View, func(name string) []*symbols.Symbol {
		return symbolfacts.LookupNamed(cached.Index, name)
	})
	if err != nil {
		var selectionErr *view.SelectionError
		if errors.As(err, &selectionErr) && selectionErr.Kind == view.SelectionNotFound {
			return nil, statusError(connect.CodeNotFound, err.Error())
		}
		return nil, statusError(connect.CodeInvalidArgument, err.Error())
	}
	return renderViewData(cached, rendering.DataFor(ports)), nil
}

type renderViewOrigins struct {
	model   *CachedModel
	sources map[string]*source.SourceFile
}

func (o *renderViewOrigins) span(origin view.Origin) *pb.Span {
	if !origin.Located() {
		return nil
	}
	sf := o.sources[origin.Doc]
	if sf == nil {
		if o.model.Library != nil {
			if content, err := o.model.Library.Read(origin.Doc); err == nil {
				sf = source.New(origin.Doc, content)
				o.sources[origin.Doc] = sf
			}
		}
	}
	if sf == nil {
		return &pb.Span{File: origin.Doc}
	}
	return sourceSpanToProto(sf, origin.Span)
}

func renderViewData(model *CachedModel, data view.Data) *pb.RenderViewResponse {
	origins := &renderViewOrigins{
		model:   model,
		sources: make(map[string]*source.SourceFile, len(model.Documents)),
	}
	for _, document := range model.Documents {
		if document.Source != nil {
			origins.sources[document.Source.Name()] = document.Source
		}
	}
	response := &pb.RenderViewResponse{
		View:    data.View,
		Kind:    string(data.Kind),
		Stated:  data.Stated,
		Columns: data.Columns,
		Notices: data.Notices,
		Nodes:   make([]*pb.RenderNode, 0, len(data.Nodes)),
		Edges:   make([]*pb.RenderEdge, 0, len(data.Edges)),
		Rows:    make([]*pb.RenderRow, 0, len(data.Rows)),
		Notes:   make([]*pb.RenderNote, 0, len(data.Notes)),
	}
	if data.Canvas != nil {
		response.Canvas = &pb.RenderCanvas{
			Unit: data.Canvas.Unit, Width: data.Canvas.Width, Height: data.Canvas.Height, HasSize: data.Canvas.HasSize,
		}
	}
	for _, node := range data.Nodes {
		item := &pb.RenderNode{
			Id: node.ID, Kind: node.Kind, Name: node.Name, NameSynthesized: node.NameSynthesized,
			Type: node.Type, Detail: node.Detail, Text: node.Text, StandIn: node.StandIn,
			Parent: node.Parent, Origin: origins.span(node.Origin),
		}
		for _, port := range node.Ports {
			item.Ports = append(item.Ports, &pb.RenderPort{
				Id: port.ID, Name: port.Name, Type: port.Type, Direction: port.Direction.String(),
			})
		}
		if node.Geometry != nil {
			item.Geometry = &pb.RenderGeometry{
				X: node.Geometry.X, Y: node.Geometry.Y, Width: node.Geometry.Width, Height: node.Geometry.Height,
				HasSize: node.Geometry.HasSize, Collapsed: node.Geometry.Collapsed,
			}
		}
		if node.Style != nil {
			item.Style = renderStyleData(node.Style)
		}
		response.Nodes = append(response.Nodes, item)
	}
	for _, edge := range data.Edges {
		item := &pb.RenderEdge{
			From: edge.From, To: edge.To, FromPort: edge.FromPort, ToPort: edge.ToPort,
			Label: edge.Label, Name: edge.Name, Kind: edge.Kind.String(), Origin: origins.span(edge.Origin),
		}
		for _, point := range edge.Route {
			item.Route = append(item.Route, &pb.RenderPoint{X: point.X, Y: point.Y})
		}
		if edge.Style != nil {
			item.Style = renderStyleData(edge.Style)
		}
		response.Edges = append(response.Edges, item)
	}
	for _, row := range data.Rows {
		response.Rows = append(response.Rows, &pb.RenderRow{Cells: row.Cells, Origin: origins.span(row.Origin)})
	}
	for _, note := range data.Notes {
		response.Notes = append(response.Notes, &pb.RenderNote{
			Text: note.Text, Anchor: note.Anchor, EdgeFrom: note.EdgeFrom, EdgeTo: note.EdgeTo,
			X: note.X, Y: note.Y, Width: note.Width, Height: note.Height, HasSize: note.HasSize,
			Origin: origins.span(note.Origin),
		})
	}
	if response.Notices == nil {
		response.Notices = []string{}
	}
	return response
}

func renderStyleData(style *view.Style) *pb.RenderStyle {
	if style == nil {
		return nil
	}
	return &pb.RenderStyle{
		Fill: style.Fill, Line: style.Line, Text: style.Text, Font: style.Font, FontSize: style.FontSize,
		Bold: style.Bold, Italic: style.Italic,
	}
}
