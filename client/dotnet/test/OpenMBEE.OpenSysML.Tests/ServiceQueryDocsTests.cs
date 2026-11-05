using System;
using System.Collections.Concurrent;
using System.IO;
using System.Linq;
using System.Net.Http;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Querying, running document queries, rendering documents and rendering views against a real sysml-grpc over every
/// client wire: structured and OSLC queries with every constraint form; element, object and scalar bindings answering
/// element, object, verdict, state and event rows; Markdown and HTML documents; views with ports, notation, depth,
/// layout, styles, notes and tables, nothing the view omits filled in; the refusals of a missing model, a missing
/// declaration and a fault in the model's declarations told apart; the answers of every conformance fixture and
/// every view the renderer's own tests declare read without a malformed-answer refusal; and, against a service
/// withholding these capabilities, refusals made before anything is sent.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceQueryDocsTests(ServiceFixture services) : IClassFixture<ServiceFixture>
{
    private const string Withheld = "query,oslc_query,document_query,render_document_html,render_view";

    private const string Lamps = """
        package Lamps {
            private import DocumentQueries::*;
            private import KerML::Root::Element;
            private import ScalarValues::*;

            attribute def Toggle;
            state def LampMachine {
                entry; then off;
                state off;
                transition off_on first off accept Toggle then on;
                state on;
            }
            part def Lamp { exhibit state lp : LampMachine; }
            part lamp : Lamp;

            calc def CurrentStates :> Query {
                in root : Element[1];
                Project(source = States(source = root), properties = ("machine", "statePath"))
            }
            calc def Nowhere :> Query {
                InState(name = "orbit")
            }
            calc def Steps :> Query {
                in root : Element[1];
                Events(source = root)
            }
            calc def Window :> Query {
                in root : Element[1];
                in s : Real[1];
                in b : Real[1];
                Events(source = root, since = s, before = b)
            }
        }
        """;

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task QueriesSelectWhatTheServiceEvaluatesInEveryConstraintForm(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = (await connection.LoadAsync(Fixture("query.sysml"), cancellationToken: Token)).RequireNoErrors();

        var all = await model.QueryAsync(Query.All, Token);
        var parts = await model.QueryAsync(new Query { Where = QueryConstraint.Equal("@type", "PartUsage") }, Token);
        var scoped = await model.QueryAsync(new Query { Scope = ["Demo::vehicle"], Select = ["name", "owner"] }, Token);
        var either = await model.QueryAsync(new Query { Where = QueryConstraint.Or(QueryConstraint.Equal("name", "spare"), QueryConstraint.Equal("name", "Wheel")) }, Token);
        var notParts = await model.QueryAsync(
            new Query { Scope = ["Demo::vehicle"], Where = new QueryConstraint.Comparison("@type", ComparisonOperator.Equal, ["PartUsage"], inverse: true) },
            Token);
        var many = await model.QueryAsync(new Query { Where = QueryConstraint.Greater("multiplicityLower", "1") }, Token);
        var few = await model.QueryAsync(new Query { Where = QueryConstraint.And(QueryConstraint.Equal("@type", "PartUsage"), QueryConstraint.Less("multiplicityUpper", "5")) }, Token);
        var atLeast = await model.QueryAsync(new Query { Where = QueryConstraint.GreaterOrEqual("multiplicityLower", "4") }, Token);
        var atMost = await model.QueryAsync(new Query { Where = QueryConstraint.LessOrEqual("multiplicityUpper", "4") }, Token);
        var below = await model.QueryAsync(new Query { Where = QueryConstraint.Less("multiplicityUpper", "4") }, Token);
        var member = await model.QueryAsync(new Query { Where = QueryConstraint.In("name", "spare", "Wheel") }, Token);
        var usages = await model.QueryAsync(new Query { Scope = ["Demo::vehicle"], Where = QueryConstraint.InstanceOf("Usage") }, Token);
        var definitions = await model.QueryAsync(new Query { Scope = ["Demo"], Where = QueryConstraint.InstanceOf("Definition") }, Token);
        var byName = await model.QueryAsync(new Query { Scope = ["Demo"], OrderBy = ["name"] }, Token);
        var byType = await model.QueryAsync(new Query { Where = QueryConstraint.Equal("@type", "PartUsage", "AttributeUsage"), OrderBy = ["@type"] }, Token);
        var oslc = await model.QueryOslcAsync("oslc.where=rdf:type=\"PartUsage\"&oslc.select=sysml:name", Token);
        var scope = await Assert.ThrowsAsync<ServiceException>(() => model.QueryAsync(new Query { Scope = ["Demo::Missing"] }, Token));
        var property = await Assert.ThrowsAsync<ServiceException>(() => model.QueryAsync(new Query { Where = QueryConstraint.Equal("nope", "x") }, Token));
        var metaclass = await Assert.ThrowsAsync<ServiceException>(() => model.QueryAsync(new Query { Where = QueryConstraint.InstanceOf("Nope") }, Token));
        var sortedBy = await Assert.ThrowsAsync<ServiceException>(() => model.QueryAsync(new Query { OrderBy = ["nope"] }, Token));
        var gone = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("0000").QueryAsync(Query.All, Token));

        Assert.Equal(
            ["Demo", "Demo::Vehicle", "Demo::Vehicle::mass", "Demo::Wheel", "Demo::vehicle", "Demo::vehicle::wheels", "Demo::vehicle::vin", "Demo::spare"],
            all.Elements.Select(element => element.Id).Where(id => id.StartsWith("Demo", StringComparison.Ordinal)));
        Assert.Equal(["Demo::vehicle", "Demo::vehicle::wheels", "Demo::spare"], parts.Elements.Select(element => element.Id));
        Assert.All(parts.Elements, element => Assert.Equal("PartUsage", element.Type));
        Assert.Equal(["Demo::vehicle", "Demo::vehicle::wheels", "Demo::vehicle::vin"], scoped.Elements.Select(element => element.Id));
        Assert.Equal(["name", "owner"], scoped.Elements[1].Properties.Keys);
        Assert.Equal(("wheels", "Demo::vehicle"), (scoped.Elements[1].Properties["name"], scoped.Elements[1].Properties["owner"]));
        Assert.Equal(["Demo::Wheel", "Demo::spare"], either.Elements.Select(element => element.Id));
        Assert.Equal(["Demo::vehicle::vin"], notParts.Elements.Select(element => element.Id));
        Assert.Equal(["Demo::vehicle::wheels"], many.Elements.Select(element => element.Id));
        Assert.Equal(["Demo::vehicle::wheels"], few.Elements.Select(element => element.Id));
        // wheels is [4]: both bounds meet the inclusive comparisons and miss the strict one.
        Assert.Equal(["Demo::vehicle::wheels"], atLeast.Elements.Select(element => element.Id));
        Assert.Equal(["Demo::vehicle::wheels"], atMost.Elements.Select(element => element.Id));
        Assert.Empty(below.Elements);
        Assert.Equal(["Demo::Wheel", "Demo::spare"], member.Elements.Select(element => element.Id));
        Assert.Equal(["Demo::vehicle", "Demo::vehicle::wheels", "Demo::vehicle::vin"], usages.Elements.Select(element => element.Id));
        Assert.Equal(["Demo::Vehicle", "Demo::Wheel"], definitions.Elements.Select(element => element.Id));
        // Names compare byte-wise, so capitals sort first.
        Assert.Equal(
            ["Demo", "Demo::Vehicle", "Demo::Wheel", "Demo::Vehicle::mass", "Demo::spare", "Demo::vehicle", "Demo::vehicle::vin", "Demo::vehicle::wheels"],
            byName.Elements.Select(element => element.Id));
        // Elements of one type keep the order the model declares them in.
        Assert.Equal(
            ["Demo::Vehicle::mass", "Demo::vehicle::vin", "Demo::vehicle", "Demo::vehicle::wheels", "Demo::spare"],
            byType.Elements.Select(element => element.Id));
        Assert.Equal(parts.Elements.Select(element => element.Id), oslc.Elements.Select(element => element.Id));
        Assert.Equal(
            [StatusCode.InvalidArgument, StatusCode.InvalidArgument, StatusCode.InvalidArgument, StatusCode.InvalidArgument],
            new[] { scope.StatusCode, property.StatusCode, metaclass.StatusCode, sortedBy.StatusCode });
        Assert.Equal(StatusCode.NotFound, gone.StatusCode);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task EveryElementOfEveryConformanceFixtureIsAnsweredWithItsIdAndType(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var fixtures = Directory.GetFiles(Repository.PathOf("conformance", "fixtures"), "*.sysml").Order(StringComparer.Ordinal).ToList();
        var selected = 0;

        foreach (var path in fixtures)
        {
            var model = await connection.LoadAsync(path, cancellationToken: Token);
            selected += (await model.QueryAsync(new Query { Select = ["name"] }, Token)).Elements.Count;
        }

        Assert.True(fixtures.Count > 30, $"only {fixtures.Count} fixtures were found");
        Assert.True(selected > 400, $"only {selected} elements were selected");
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task DocumentQueriesAnswerTypedRowsOverElementsObjectsVerdictsStatesAndEvents(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var observatory = (await connection.LoadAsync(Fixture("document.sysml"), cancellationToken: Token)).RequireNoErrors();
        var garage = (await connection.LoadAsync(Fixture("verdicts.sysml"), cancellationToken: Token)).RequireNoErrors();
        var objects = (await connection.LoadAsync(Repository.PathOf("internal", "doc", "docrender", "testdata", "object_report.sysml"), cancellationToken: Token)).RequireNoErrors();
        var lamps = (await connection.LoadContentAsync(Lamps, cancellationToken: Token)).RequireNoErrors();
        var telescope = new DocumentBinding("root", DocumentValue.ElementRef("Observatory::telescope"));

        var table = await observatory.RunDocumentQueryAsync("Observatory::SubsystemTable", [telescope], Token);
        var heavy = await observatory.RunDocumentQueryAsync("Observatory::HeavySubsystemNames", [telescope, new DocumentBinding("threshold", new DocumentValue.TextValue("10"))], Token);
        var missing = await observatory.RunDocumentQueryAsync("Observatory::MissingSubsystems", [telescope], Token);
        var verdicts = await garage.RunDocumentQueryAsync("Garage::Checks", [new DocumentBinding("root", DocumentValue.ElementRef("Garage::car"))], Token);
        var car = (await objects.InstantiateAsync("Garage::car", Token)).Root;
        var byPath = await objects.RunDocumentQueryAsync("Garage::Parts", [new DocumentBinding("root", DocumentValue.ObjectRef("Garage::car"))], Token);
        var byId = await objects.RunDocumentQueryAsync("Garage::Parts", [new DocumentBinding("root", DocumentValue.ObjectRef(car.Id))], Token);
        var rebound = await objects.RunDocumentQueryAsync("Garage::Parts", [new DocumentBinding("root", byPath.Rows[0].Element)], Token);
        var lamp = (await lamps.InstantiateAsync("Lamps::lamp", Token)).Root;
        var states = await lamps.RunDocumentQueryAsync("Lamps::CurrentStates", [new DocumentBinding("root", DocumentValue.ObjectRef(lamp.Id))], Token);
        var events = await lamps.RunDocumentQueryAsync("Lamps::Steps", [new DocumentBinding("root", DocumentValue.ObjectRef(lamp.Id))], Token);

        Assert.Equal(["name", "mass"], table.Columns);
        Assert.Equal(
            ["baffle|shroud *tricky*", "mount", "optics", "segmentControl"],
            table.Rows.Select(row => Assert.IsType<DocumentValue.TextValue>(Assert.Single(row.Cells[0])).Text));
        Assert.Equal(new DocumentValue.ElementValue("Observatory::telescope::mount", "PartUsage"), table.Rows[1].Element);
        Assert.Equal(new DocumentValue.NumberValue(new Value.RealValue(15.0)), Assert.Single(table.Rows[1].Cells[1]));
        Assert.Equal(["mount", "segmentControl"], heavy.Rows.Select(row => Assert.Single(row.Cells[0]).ToString()));
        Assert.Equal((2, 0), (missing.Columns.Count, missing.Rows.Count));
        Assert.Equal(["path", "name", "verdict", "reason"], verdicts.Columns);
        Assert.Equal(7, verdicts.Rows.Count);
        var holds = Assert.IsType<DocumentValue.VerdictValue>(verdicts.Rows[0].Element).Verdict;
        Assert.Equal((new DocumentValue.ElementValue("Garage::Car::massOk", "ConstraintUsage"), "constraint", "holds"), (holds.Assertion, holds.Kind, holds.Verdict));
        Assert.Equal(["Garage::car.engine", "Garage::car.wheels[1]", "Garage::car.wheels[2]"], byPath.Rows.Select(row => Assert.IsType<DocumentValue.ObjectValue>(row.Element).Value.Path));
        Assert.Equal(new DocumentValue.ElementValue("Garage::Car::wheels", "PartUsage"), ((DocumentValue.ObjectValue)byPath.Rows[1].Element).Value.Element);
        Assert.Equal(new DocumentValue.NumberValue(new Value.IntegerValue(30)), Assert.Single(byPath.Rows[1].Cells[2]));
        Assert.Equal(byPath.Rows.Select(row => ((DocumentValue.ObjectValue)row.Element).Value.InstanceId), byId.Rows.Select(row => ((DocumentValue.ObjectValue)row.Element).Value.InstanceId));
        Assert.Empty(rebound.Rows);
        var state = Assert.IsType<DocumentValue.StateValue>(Assert.Single(states.Rows).Element).State;
        Assert.Equal(("lp", "off", lamp.Id), (state.Machine, state.StatePath, state.Subject!.InstanceId));
        Assert.Equal("entry", Assert.IsType<DocumentValue.EventValue>(Assert.Single(events.Rows).Element).Event.Kind);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AMissingModelAMissingDeclarationAWrongBindingAndAFaultInTheModelAreToldApart(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var observatory = (await connection.LoadAsync(Fixture("document.sysml"), cancellationToken: Token)).RequireNoErrors();
        var lamps = (await connection.LoadContentAsync(Lamps, cancellationToken: Token)).RequireNoErrors();
        var lamp = (await lamps.InstantiateAsync("Lamps::lamp", Token)).Root;
        var root = new DocumentBinding("root", DocumentValue.ElementRef("Observatory::telescope"));

        var gone = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("0000").RunDocumentQueryAsync("Q", cancellationToken: Token));
        var unknown = await Assert.ThrowsAsync<ServiceException>(() => observatory.RunDocumentQueryAsync("Observatory::NoSuchQuery", cancellationToken: Token));
        var notQuery = await Assert.ThrowsAsync<ServiceException>(() => observatory.RunDocumentQueryAsync("Observatory::MassReport", [root], Token));
        var unbound = await Assert.ThrowsAsync<ServiceException>(() => observatory.RunDocumentQueryAsync("Observatory::SubsystemTable", cancellationToken: Token));
        var noObjects = await Assert.ThrowsAsync<ServiceException>(() =>
            observatory.RunDocumentQueryAsync("Observatory::SubsystemTable", [new DocumentBinding("root", DocumentValue.ObjectRef("telescope"))], Token));
        var nowhere = await Assert.ThrowsAsync<QueryException>(() => lamps.RunDocumentQueryAsync("Lamps::Nowhere", cancellationToken: Token));
        var empty = await Assert.ThrowsAsync<QueryException>(() => lamps.RunDocumentQueryAsync(
            "Lamps::Window",
            [
                new DocumentBinding("root", DocumentValue.ObjectRef(lamp.Id)),
                new DocumentBinding("s", new DocumentValue.NumberValue(new Value.RealValue(1))),
                new DocumentBinding("b", new DocumentValue.NumberValue(new Value.RealValue(1))),
            ],
            Token));
        var noDocument = await Assert.ThrowsAsync<ServiceException>(() => observatory.RenderDocumentAsync("Observatory::NoSuchDocument", cancellationToken: Token));
        var notDocument = await Assert.ThrowsAsync<ServiceException>(() => observatory.RenderDocumentAsync("Observatory::SubsystemTable", cancellationToken: Token));
        var documentGone = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("0000").RenderDocumentAsync("D", cancellationToken: Token));
        var noView = await Assert.ThrowsAsync<ServiceException>(() => observatory.RenderViewAsync("Observatory::NoSuchView", cancellationToken: Token));
        var viewGone = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("0000").RenderViewAsync("V", cancellationToken: Token));

        Assert.All([unknown, noObjects, noDocument, noView], error => Assert.Equal((typeof(ServiceException), StatusCode.NotFound), (error.GetType(), error.StatusCode)));
        Assert.All([notQuery, unbound, notDocument], error => Assert.Equal(StatusCode.InvalidArgument, error.StatusCode));
        Assert.Contains("holds no objects", noObjects.ServiceMessage, StringComparison.Ordinal);
        Assert.Contains("no view named", noView.ServiceMessage, StringComparison.Ordinal);
        Assert.All([nowhere, empty], error => Assert.Equal(StatusCode.FailedPrecondition, error.StatusCode));
        Assert.Contains("orbit", nowhere.Message, StringComparison.Ordinal);
        Assert.All([gone, documentGone, viewGone], error => Assert.StartsWith("model not found: 0000", error.ServiceMessage, StringComparison.Ordinal));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ADocumentRendersToMarkdownHtmlAndAsciiDocNamingTheFileARenderedSetWrites(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var observatory = (await connection.LoadAsync(Fixture("document.sysml"), cancellationToken: Token)).RequireNoErrors();

        var markdown = await observatory.RenderDocumentAsync("Observatory::MassReport", cancellationToken: Token);
        var html = await observatory.RenderDocumentAsync("Observatory::MassReport", DocumentForm.Html, Token);
        var asciidoc = await observatory.RenderDocumentAsync("Observatory::MassReport", DocumentForm.AsciiDoc, Token);

        Assert.Equal(DocumentForm.Markdown, markdown.Form);
        Assert.StartsWith("# Telescope Mass Report", markdown.Content, StringComparison.Ordinal);
        Assert.Contains("| mount |", markdown.Content, StringComparison.Ordinal);
        Assert.Equal(DocumentForm.Html, html.Form);
        Assert.Contains("<h1", html.Content, StringComparison.Ordinal);
        Assert.Contains("Telescope Mass Report", html.Content, StringComparison.Ordinal);
        Assert.EndsWith(".md", markdown.FileName, StringComparison.Ordinal);
        Assert.EndsWith(".html", html.FileName, StringComparison.Ordinal);
        Assert.Equal(Path.GetFileNameWithoutExtension(markdown.FileName), Path.GetFileNameWithoutExtension(html.FileName));
        Assert.Equal(DocumentForm.AsciiDoc, asciidoc.Form);
        Assert.StartsWith("= Telescope Mass Report", asciidoc.Content, StringComparison.Ordinal);
        Assert.Contains("mount", asciidoc.Content, StringComparison.Ordinal);
        Assert.EndsWith(".adoc", asciidoc.FileName, StringComparison.Ordinal);
        Assert.Equal(Path.GetFileNameWithoutExtension(markdown.FileName), Path.GetFileNameWithoutExtension(asciidoc.FileName));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AViewRendersWithItsPortsNotationAndDepthAndNothingItOmitsFilledIn(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var views = (await connection.LoadAsync(Fixture("views.sysml"), cancellationToken: Token)).RequireNoErrors();
        var tree = await connection.LoadAsync(ViewTestdata("depth-tree.sysml"), cancellationToken: Token);

        var minimal = await views.RenderViewAsync("RenderViewDemo::connections", cancellationToken: Token);
        var full = await views.RenderViewAsync("RenderViewDemo::connections", new RenderViewOptions { Ports = RenderViewPorts.Full, Show = [ViewNotation.EdgeLabels] }, Token);
        var all = await views.RenderViewAsync("RenderViewDemo::connections", new RenderViewOptions { Show = [ViewNotation.All] }, Token);
        var pseudo = await views.RenderViewAsync("#tree:RenderViewDemo::Assembly", cancellationToken: Token);
        var unlimited = await tree.RenderViewAsync("DepthViews::unlimited", cancellationToken: Token);
        var shallow = await tree.RenderViewAsync("DepthViews::unlimited", new RenderViewOptions { Depth = 1 }, Token);
        var stated = await tree.RenderViewAsync("DepthViews::depth1", cancellationToken: Token);
        var deeper = await tree.RenderViewAsync("DepthViews::depth1", new RenderViewOptions { Depth = 3 }, Token);
        var notation = await Assert.ThrowsAsync<ServiceException>(() => views.RenderViewAsync("RenderViewDemo::connections", new RenderViewOptions { Show = ["sparkles"] }, Token));

        Assert.Equal(("RenderViewDemo::connections", "interconnection"), (minimal.View, minimal.Kind));
        Assert.Equal(["api"], minimal.Nodes[1].Ports.Select(port => port.Name));
        Assert.Equal(["api", "spare"], full.Nodes[1].Ports.Select(port => port.Name));
        Assert.Equal(("n1.0", "n2.0"), (minimal.Edges[0].FromPort, minimal.Edges[0].ToPort));
        Assert.Contains(ViewNotation.EdgeLabels, full.Shown);
        Assert.DoesNotContain(ViewNotation.EdgeLabels, minimal.Shown);
        Assert.True(all.Shown.Count > full.Shown.Count);
        Assert.Equal("tree", pseudo.Kind);
        Assert.All([minimal, full, pseudo], view => Assert.Null(view.Canvas));
        Assert.All(minimal.Nodes, node => Assert.Equal((null, null), (node.Geometry, node.Style)));
        Assert.All(minimal.Nodes, node => Assert.Equal("views.sysml", Path.GetFileName(node.Origin?.File)));
        Assert.Equal(minimal.Nodes.Count, minimal.Roots.Count() + minimal.Nodes.Count(node => node.Parent.Length > 0));
        Assert.Contains(minimal.Roots, root => minimal.ChildrenOf(root).Any());
        Assert.True(unlimited.Nodes.Count > shallow.Nodes.Count, $"{unlimited.Nodes.Count} nodes at every level, {shallow.Nodes.Count} at one");
        Assert.Equal(stated.Nodes.Select(node => node.Id), shallow.Nodes.Select(node => node.Id));
        Assert.True(deeper.Nodes.Count > stated.Nodes.Count, $"{deeper.Nodes.Count} nodes at three levels, {stated.Nodes.Count} at the view's one");
        Assert.Equal(StatusCode.InvalidArgument, notation.StatusCode);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task CompartmentsFoldMembersIntoRowsOfTheirOwnersBoxAndWhatAnOwnerHasFromItsTypeIsMarkedInherited(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var views = (await connection.LoadAsync(Fixture("views.sysml"), cancellationToken: Token)).RequireNoErrors();
        var tree = (await connection.LoadAsync(ViewTestdata("depth-tree.sysml"), cancellationToken: Token)).RequireNoErrors();
        var inheriting = (await connection.LoadAsync(ViewTestdata("inherited-prefix.sysml"), cancellationToken: Token)).RequireNoErrors();

        var positioned = await tree.RenderViewAsync("DepthViews::depth1", cancellationToken: Token);
        var most = await tree.RenderViewAsync("DepthViews::depth1", new RenderViewOptions { Compartments = RenderViewCompartments.Most }, Token);
        var ports = await tree.RenderViewAsync("DepthViews::depth1", new RenderViewOptions { Compartments = RenderViewCompartments.Ports }, Token);
        var typed = await inheriting.RenderViewAsync("InheritedPrefixViews::part1Tree", new RenderViewOptions { Compartments = RenderViewCompartments.Most }, Token);
        var linked = await views.RenderViewAsync("RenderViewDemo::connections", new RenderViewOptions { Ports = RenderViewPorts.Full }, Token);

        // The tree no Layout positions folds nothing by default.
        Assert.All(positioned.Nodes, node => Assert.Empty(node.Compartment));
        var vehicle = Assert.Single(most.Roots);
        Assert.Equal(
            [("mass", "attributes"), ("wheel", "parts"), ("engine", "parts")],
            most.ChildrenOf(vehicle).Select(row => (row.Name, row.Compartment)));
        var engine = Assert.Single(ports.Nodes, node => node.Name == "engine");
        Assert.Equal(string.Empty, engine.Compartment);
        Assert.Equal([("fuelIn", "ports")], ports.ChildrenOf(engine).Select(row => (row.Name, row.Compartment)));
        Assert.Equal(1, ports.Nodes.Count(node => node.Compartment.Length > 0));

        // part1 : APart declares nothing of its own, so every member it lists is APart's.
        var part1 = Assert.Single(typed.Roots);
        Assert.False(part1.Inherited);
        Assert.Equal(
            [("mass", "attributes", true), ("engine", "parts", true), ("aPort", "ports", true)],
            typed.ChildrenOf(part1).Select(row => (row.Name, row.Compartment, row.Inherited)));

        // sender : Sender and receiver : Receiver have their ports from their types.
        Assert.Equal(
            [("sender", "api", true), ("sender", "spare", true), ("receiver", "api", true)],
            linked.Nodes.SelectMany(node => node.Ports.Select(port => (node.Name, port.Name, port.Inherited))));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AViewsLayoutStylesNotesCanvasAndTableAreReadAsTheViewStatesThem(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var plant = await connection.LoadAsync(ViewTestdata("layout.sysml"), cancellationToken: Token);
        var fleet = (await connection.LoadAsync(ViewTestdata("table.sysml"), cancellationToken: Token)).RequireNoErrors();
        var folded = await connection.LoadAsync(ViewTestdata("layout-collapsed.sysml"), cancellationToken: Token);

        var placed = await plant.RenderViewAsync("PlantViews::placedView", cancellationToken: Token);
        var plain = await plant.RenderViewAsync("PlantViews::plainView", cancellationToken: Token);
        var table = await fleet.RenderViewAsync("TableViews::partsTable", cancellationToken: Token);
        var foldOnly = await folded.RenderViewAsync("FoldedViews::foldOnly", cancellationToken: Token);

        Assert.Equal(("PlantViews::placedView", "interconnection", "render asInterconnectionDiagram"), (placed.View, placed.Kind, placed.Stated));
        Assert.Equal(new ViewCanvas("px", 1200, 800, hasSize: true), placed.Canvas);
        var pump = placed.Nodes.Single(node => node.Name == "pump");
        var tank = placed.Nodes.Single(node => node.Name == "tank");
        Assert.Equal(new ViewGeometry(300, 40, 0, 0, HasSize: false, Collapsed: true, Unplaced: false), pump.Geometry);
        Assert.Equal(new ViewGeometry(500, 40, 120, 60, HasSize: true, Collapsed: false, Unplaced: false), tank.Geometry);
        Assert.Equal(new ViewStyle("#FFE8BD", "#333333", string.Empty, "Arial", 11, bold: true, italic: false), pump.Style);
        Assert.Null(tank.Style);
        Assert.Equal([pump, tank], placed.ChildrenOf(Assert.Single(placed.Roots)));
        var supply = Assert.Single(placed.Edges);
        Assert.Equal((pump.Id, tank.Id, pump.Ports[0].Id, tank.Ports[0].Id, "connection"), (supply.From, supply.To, supply.FromPort, supply.ToPort, supply.Kind));
        Assert.Equal([new ViewPoint(400, 70), new ViewPoint(450, 120), new ViewPoint(500, 70)], supply.Route);
        Assert.Equal(new ViewStyle(string.Empty, "#0000FF", string.Empty, string.Empty, 0, bold: false, italic: false), supply.Style);
        Assert.Equal(["always", "anchored", "check pressure", "free"], placed.Notes.Select(note => note.Text));
        Assert.All(placed.Notes, note => Assert.NotNull(note.Origin));
        Assert.Equal((tank.Id, 650.0, 40.0, 100.0, 30.0, true), (placed.Notes[1].Anchor, placed.Notes[1].X, placed.Notes[1].Y, placed.Notes[1].Width, placed.Notes[1].Height, placed.Notes[1].HasSize));
        Assert.Equal((string.Empty, pump.Id, tank.Id, false), (placed.Notes[2].Anchor, placed.Notes[2].EdgeFrom, placed.Notes[2].EdgeTo, placed.Notes[2].HasSize));
        Assert.Equal((string.Empty, string.Empty, 0.0, 700.0), (placed.Notes[3].Anchor, placed.Notes[3].EdgeFrom, placed.Notes[3].X, placed.Notes[3].Y));
        Assert.Empty(placed.Columns);
        Assert.Empty(placed.Notices);

        Assert.Null(plain.Canvas);
        Assert.Equal(new ViewGeometry(10, 20, 100, 50, HasSize: true, Collapsed: false, Unplaced: false), plain.Nodes.Single(node => node.Name == "pump").Geometry);
        Assert.Equal("#FFFFDC", plain.Nodes.Single(node => node.Name == "pump").Style!.Fill);
        Assert.Equal((null, null), (plain.Nodes.Single(node => node.Name == "tank").Geometry, plain.Nodes.Single(node => node.Name == "tank").Style));

        Assert.Equal("table", table.Kind);
        Assert.Equal(["Element", "Kind", "Type", "Declared in"], table.Columns);
        Assert.Equal(["Fleet::Truck", "engine", "wheels"], table.Rows.Select(row => row.Cells[0]));
        Assert.Equal(["engine", "part", "Engine", "Fleet::Truck"], table.Rows[1].Cells);
        Assert.All(table.Rows, row => Assert.NotNull(row.Origin));
        Assert.Empty(table.Nodes);

        var engine = foldOnly.Nodes.Single(node => node.Name == "engine");
        Assert.Equal(new ViewGeometry(0, 0, 0, 0, HasSize: false, Collapsed: true, Unplaced: true), engine.Geometry);
        var wheel = foldOnly.Nodes.Single(node => node.Name == "wheel");
        Assert.Equal(["hub"], foldOnly.ChildrenOf(wheel).Select(node => node.Name));
        Assert.Equal(wheel.Id, foldOnly.Node(foldOnly.ChildrenOf(wheel).Single().Parent)!.Id);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task EveryViewTheRenderersOwnTestsDeclareIsReadWithItsNodesInOrder(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var rendered = 0;

        foreach (var path in Directory.GetFiles(Repository.PathOf("internal", "ir", "view", "testdata"), "*.sysml").Order(StringComparer.Ordinal))
        {
            var model = await connection.LoadAsync(path, cancellationToken: Token);
            if (model.HasErrors)
            {
                continue;
            }

            foreach (var view in (await model.QueryAsync(new Query { Where = QueryConstraint.Equal("@type", "ViewUsage") }, Token)).Elements)
            {
                try
                {
                    var rendering = await model.RenderViewAsync(view.Id, new RenderViewOptions { Ports = RenderViewPorts.Full, Show = [ViewNotation.All] }, Token);
                    Assert.All(rendering.Nodes.Where(node => node.Parent.Length > 0), node => Assert.NotNull(rendering.Node(node.Parent)));
                    rendered++;
                }
                catch (ServiceException refused) when (refused.StatusCode == StatusCode.InvalidArgument)
                {
                    // A view the renderer refuses to select or draw is refused in its own words, not read.
                }
            }
        }

        Assert.True(rendered > 100, $"only {rendered} views were rendered");
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AWithheldCapabilityIsRefusedBeforeAnythingIsSent(ClientWire wire)
    {
        var withheld = await services.ServiceAsync("connect", Withheld);
        using var counting = new CountingHandler();
        await using var connection = await Connection.OpenAsync(
            new ConnectionOptions { Address = withheld.Address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), HttpMessageHandler = counting },
            Token);
        var model = connection.ModelByHash("h");
        var root = new DocumentBinding("root", DocumentValue.ElementRef("A"));

        var refused = new[]
        {
            await Assert.ThrowsAsync<CapabilityException>(() => model.QueryAsync(Query.All, Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.QueryOslcAsync("oslc.where=rdf:type=\"PartUsage\"", Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.RunDocumentQueryAsync("Q", [root], Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.RenderDocumentAsync("D", DocumentForm.Html, Token)),
            await Assert.ThrowsAsync<CapabilityException>(() => model.RenderViewAsync("V", cancellationToken: Token)),
        };

        Assert.Equal(
            [Capabilities.Query, Capabilities.Query, Capabilities.DocumentQuery, Capabilities.RenderDocumentHtml, Capabilities.RenderView],
            refused.Select(error => error.Capability));
        Assert.Equal(["GetServerInfo"], counting.Methods);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AWithheldOrderingOrAsciiDocIsRefusedBeforeAnythingIsSentWhileTheRestIsServed(ClientWire wire)
    {
        var withheld = await services.ServiceAsync("connect", "query_order_by,render_document_asciidoc");
        using var counting = new CountingHandler();
        await using var connection = await Connection.OpenAsync(
            new ConnectionOptions { Address = withheld.Address, Protocol = wire.Protocol(), Encoding = wire.Encoding(), HttpMessageHandler = counting },
            Token);
        var model = (await connection.LoadAsync(Fixture("document.sysml"), cancellationToken: Token)).RequireNoErrors();

        var ordering = await Assert.ThrowsAsync<CapabilityException>(() => model.QueryAsync(new Query { OrderBy = ["name"] }, Token));
        var asciidoc = await Assert.ThrowsAsync<CapabilityException>(() => model.RenderDocumentAsync("Observatory::MassReport", DocumentForm.AsciiDoc, Token));
        await model.QueryAsync(Query.All, Token);
        await model.RenderDocumentAsync("Observatory::MassReport", DocumentForm.Html, Token);

        Assert.Equal([Capabilities.QueryOrderBy, Capabilities.RenderDocumentAsciidoc], new[] { ordering.Capability, asciidoc.Capability });
        Assert.Equal(["GetServerInfo", "ParseFile", "Query", "RenderDocument"], counting.Methods);
    }

    private static string Fixture(string name) => Repository.PathOf("conformance", "fixtures", name);

    private static string ViewTestdata(string name) => Repository.PathOf("internal", "ir", "view", "testdata", name);

    private async Task<Connection> OpenAsync(ClientWire wire)
    {
        var service = await services.ServiceAsync("connect");
        return await Connection.OpenAsync(wire.To(service.Address), Token);
    }

    /// <summary>A borrowed handler that records the method of every request it carries to the service.</summary>
    private sealed class CountingHandler() : DelegatingHandler(new SocketsHttpHandler())
    {
        private readonly ConcurrentQueue<string> methods = new();

        internal string[] Methods => [.. methods];

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            methods.Enqueue(request.RequestUri!.AbsolutePath.Split('/')[^1]);
            return base.SendAsync(request, cancellationToken);
        }
    }
}
