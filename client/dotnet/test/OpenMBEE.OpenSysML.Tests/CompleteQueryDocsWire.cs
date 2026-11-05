using System.Linq;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>Answers that set every field the query, document query, document and view responses can reach.</summary>
internal static class CompleteQueryDocsWire
{
    /// <summary>Two selected elements, one reporting properties and one none.</summary>
    internal static Wire.QueryResponse Query => new()
    {
        Elements =
        {
            new Wire.QueryResultElement { Id = "Demo::vehicle::wheels", Type = "PartUsage", Properties = { ["name"] = "wheels", ["owner"] = "Demo::vehicle", ["documentation"] = string.Empty } },
            new Wire.QueryResultElement { Id = "Demo::Vehicle", Type = "PartDefinition" },
        },
    };

    /// <summary>Rows of every kind a query selects, with a cell of every value arm and an empty cell.</summary>
    internal static Wire.RunDocumentQueryResponse DocumentQuery => new()
    {
        Columns = { new Wire.DocumentQueryColumn { Name = "name" }, new Wire.DocumentQueryColumn { Name = "values" } },
        Rows =
        {
            new Wire.DocumentQueryRow
            {
                Element = CompleteExecutionWire.Element,
                Cells =
                {
                    new Wire.DocumentQueryCell { Values = { new Wire.DocumentValue { StringValue = "car" } } },
                    new Wire.DocumentQueryCell { Values = { CompleteExecutionWire.DocumentValues } },
                },
            },
            new Wire.DocumentQueryRow { Element = new Wire.DocumentValue { Object = CompleteExecutionWire.Object }, Cells = { new Wire.DocumentQueryCell(), new Wire.DocumentQueryCell() } },
            new Wire.DocumentQueryRow { Element = new Wire.DocumentValue { Verdict = CompleteExecutionWire.Verdict } },
            new Wire.DocumentQueryRow { Element = new Wire.DocumentValue { State = CompleteExecutionWire.State } },
            new Wire.DocumentQueryRow { Element = new Wire.DocumentValue { Event = CompleteExecutionWire.Event } },
            new Wire.DocumentQueryRow { Element = new Wire.DocumentValue { ElementType = "AssertConstraintUsage" } },
        },
    };

    /// <summary>A document rendered to Markdown, with the file a rendered set writes it to.</summary>
    internal static Wire.RenderDocumentResponse Markdown => new() { Markdown = "# Telescope Mass Report\n", FileName = "Observatory-MassReport.md" };

    /// <summary>A document rendered to HTML, with the file a rendered set writes it to.</summary>
    internal static Wire.RenderDocumentResponse Html => new() { Html = "<!DOCTYPE html>\n<h1>Telescope Mass Report</h1>\n", FileName = "Observatory-MassReport.html" };

    /// <summary>A document rendered to AsciiDoc, with the file a rendered set writes it to.</summary>
    internal static Wire.RenderDocumentResponse AsciiDoc => new() { Asciidoc = "= Telescope Mass Report\n", FileName = "Observatory-MassReport.adoc" };

    /// <summary>A located span.</summary>
    internal static Wire.Span Origin(int line) => new() { File = "views.sysml", StartLine = line, StartCol = 3, EndLine = line + 1, EndCol = 9 };

    /// <summary>A rendering setting every field: nested nodes with ports, geometry and style, a routed styled edge, rows, a canvas, notes and a frame.</summary>
    internal static Wire.RenderViewResponse View => new()
    {
        View = "PlantViews::placedView",
        Kind = "interconnection",
        Stated = "asInterconnectionDiagram",
        Nodes =
        {
            new Wire.RenderNode
            {
                Id = "n1",
                Kind = "part def",
                Name = "Plant::Loop",
                Detail = "already shown",
                Origin = Origin(10),
                Geometry = new Wire.RenderGeometry { X = 300, Y = 40, Width = 120, Height = 60.5, HasSize = true, Collapsed = true },
                Style = new Wire.RenderStyle { Fill = "#FFE8BD", Line = "#333333", Text = "#000000", Font = "Arial", FontSize = 11, Bold = true, Italic = true },
                Parallel = true,
            },
            new Wire.RenderNode
            {
                Id = "n2",
                Kind = "part",
                Name = "pump",
                NameSynthesized = true,
                Type = "Pump",
                Text = "pump : Pump",
                StandIn = true,
                Parent = "n1",
                Ports =
                {
                    new Wire.RenderPort { Id = "n2.0", Name = "outlet", Type = "FluidPort", Direction = "out" },
                    new Wire.RenderPort { Id = "n2.1", Name = "vent", Inherited = true },
                },
                Origin = new Wire.Span { File = "lib.sysml" },
                Geometry = new Wire.RenderGeometry { Collapsed = true, Unplaced = true },
            },
            new Wire.RenderNode { Id = "n3", Kind = "part", Name = "tank", Parent = "n2", Arrow = "right" },
            new Wire.RenderNode { Id = "n4", Kind = "attribute", Name = "pressure", Type = "Real", Parent = "n2", Compartment = "attributes", Inherited = true },
        },
        Edges =
        {
            new Wire.RenderEdge
            {
                From = "n2",
                To = "n3",
                FromPort = "n2.0",
                ToPort = "n3.0",
                Label = "supply",
                Name = "supply",
                Kind = "connection",
                Origin = Origin(20),
                Route = { new Wire.RenderPoint { X = 400, Y = 70 }, new Wire.RenderPoint { X = 0.5, Y = 120.25 } },
                Style = new Wire.RenderStyle { Line = "#0000FF" },
            },
            new Wire.RenderEdge { From = "n1", To = "n3", Kind = "composition" },
        },
        Columns = { "name", "type" },
        Rows = { new Wire.RenderRow { Cells = { "pump", "Pump" }, Origin = Origin(12) }, new Wire.RenderRow { Cells = { "tank", string.Empty } } },
        Canvas = new Wire.RenderCanvas { Unit = "px", Width = 1200, Height = 800, HasSize = true },
        Notes =
        {
            new Wire.RenderNote { Text = "anchored", Anchor = "n3", X = 650, Y = 40, Width = 100, Height = 30, HasSize = true, Origin = Origin(30) },
            new Wire.RenderNote { Text = "check pressure", EdgeFrom = "n2", EdgeTo = "n3", X = 420, Y = 140 },
        },
        Notices = { "a picture was refused", "nothing more" },
        Shown = { "port-symbols", "edge-labels" },
        Frame = new Wire.RenderFrame
        {
            Name = "placedView",
            Type = "PlantViews::PlacedView",
            Exposes = { "expose Plant::**;" },
            Filters = { "@SysML::PartUsage" },
            Annotations = { "placed by hand", "reviewed" },
        },
    };

    /// <summary>A rendering stating nothing optional: no canvas, and a node with no origin, geometry or style.</summary>
    internal static Wire.RenderViewResponse SparseView => new()
    {
        View = "RenderViewDemo::connections",
        Kind = "tree",
        Nodes = { new Wire.RenderNode { Id = "n1", Kind = "part" } },
        Edges = { new Wire.RenderEdge { From = "n1", To = "n1" } },
        Rows = { new Wire.RenderRow() },
        Notes = { new Wire.RenderNote() },
    };

    /// <summary>Every sample, for a test proving they set every reachable field.</summary>
    internal static Google.Protobuf.IMessage[] All =>
        [Query, DocumentQuery, Markdown, Html, AsciiDoc, View, .. CompleteExecutionWire.DocumentValues.Select(value => (Google.Protobuf.IMessage)value)];
}
