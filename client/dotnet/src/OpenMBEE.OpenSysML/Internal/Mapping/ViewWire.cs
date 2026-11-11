using System.Collections.Generic;
using System.Linq;

namespace OpenMBEE.OpenSysML.Internal.Mapping;

/// <summary>Reads a <c>RenderViewResponse</c> into a <see cref="RenderedView"/>, keeping every field and an absent message absent.</summary>
internal static class ViewWire
{
    /// <summary>Reads a rendering.</summary>
    /// <exception cref="TransportException">
    /// A node repeats an id, names as its parent no node before it, or is a compartment row with no parent or whose parent
    /// is itself a row.
    /// </exception>
    internal static RenderedView Rendered(Wire.RenderViewResponse response, string method)
    {
        var seen = new Dictionary<string, Wire.RenderNode>(System.StringComparer.Ordinal);
        var nodes = new List<ViewNode>(response.Nodes.Count);
        foreach (var (node, i) in response.Nodes.Select((node, i) => (node, i)))
        {
            Wire.RenderNode? parent = null;
            if (node.Parent.Length > 0 && !seen.TryGetValue(node.Parent, out parent))
            {
                throw Malformed(method, $"nodes[{i}] names {ValueText.Quoted(node.Parent)} as its parent, which no node before it is");
            }

            if (node.Compartment.Length > 0)
            {
                if (parent is null)
                {
                    throw Malformed(method, $"nodes[{i}] is a row of the compartment {ValueText.Quoted(node.Compartment)} but names no parent whose box holds it");
                }

                if (parent.Compartment.Length > 0)
                {
                    throw Malformed(method, $"nodes[{i}] is a row of the compartment {ValueText.Quoted(node.Compartment)} of {ValueText.Quoted(node.Parent)}, which is itself a row and has no box");
                }
            }

            if (!seen.TryAdd(node.Id, node))
            {
                throw Malformed(method, $"nodes[{i}] repeats the id {ValueText.Quoted(node.Id)}");
            }

            nodes.Add(new ViewNode(
                node.Id,
                node.Kind,
                node.Name,
                node.NameSynthesized,
                node.Type,
                node.Detail,
                node.Text,
                node.StandIn,
                node.Parent,
                node.Ports.Select(port => new ViewPort(port.Id, port.Name, port.Type, port.Direction, port.Inherited)),
                Span(node.Origin),
                node.Geometry is { } geometry
                    ? new ViewGeometry(geometry.X, geometry.Y, geometry.Width, geometry.Height, geometry.HasSize, geometry.Collapsed, geometry.Unplaced)
                    : null,
                Style(node.Style),
                node.Compartment,
                node.Inherited,
                node.Arrow,
                node.Parallel));
        }

        return new RenderedView(
            response.View,
            response.Kind,
            response.Stated,
            nodes,
            response.Edges.Select(edge => new ViewEdge(
                edge.From,
                edge.To,
                edge.FromPort,
                edge.ToPort,
                edge.Label,
                edge.Name,
                edge.Kind,
                Span(edge.Origin),
                edge.Route.Select(point => new ViewPoint(point.X, point.Y)),
                Style(edge.Style))),
            response.Columns,
            response.Rows.Select(row => new ViewRow(row.Cells, Span(row.Origin))),
            response.Canvas is { } canvas ? new ViewCanvas(canvas.Unit, canvas.Width, canvas.Height, canvas.HasSize) : null,
            response.Notes.Select(note => new ViewNote(note.Text, note.Anchor, note.EdgeFrom, note.EdgeTo, note.X, note.Y, note.Width, note.Height, note.HasSize, Span(note.Origin))),
            response.Notices,
            response.Shown,
            response.Frame is { } frame ? new ViewFrame(frame.Name, frame.Type, frame.Exposes, frame.Filters, frame.Annotations) : null);
    }

    private static SourceSpan? Span(Wire.Span? span) =>
        span is null ? null : new SourceSpan(span.File, span.StartLine, span.StartCol, span.EndLine, span.EndCol);

    private static ViewStyle? Style(Wire.RenderStyle? style) =>
        style is null ? null : new ViewStyle(style.Fill, style.Line, style.Text, style.Font, style.FontSize, style.Bold, style.Italic);

    private static TransportException Malformed(string method, string reason) =>
        TransportException.Malformed(method, $"a malformed rendering: {reason}");
}
