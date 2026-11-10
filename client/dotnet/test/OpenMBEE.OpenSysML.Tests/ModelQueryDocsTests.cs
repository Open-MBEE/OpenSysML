using System;
using System.Collections.Generic;
using System.Linq;
using System.Numerics;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Query, RunDocumentQuery, RenderDocument and RenderView through the typed API, over every wire: the exact request
/// each query, binding and option makes, every field of each answer read, an absent geometry, style, canvas or origin
/// kept absent rather than zero, a malformed answer refused, the gates checked in the service's order before anything
/// is sent, and a missing model, a missing declaration and a fault in the model's own declarations told apart.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelQueryDocsTests
{
    private const string Hash = "h";

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task QuerySendsTheStructuredQueryAndReadsEveryElement(ClientWire wire)
    {
        using var handler = Serving(wire).On("Query", Wire.QueryRequest.Parser, _ => CompleteQueryDocsWire.Query);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;
        var query = new Query
        {
            Scope = ["Demo::vehicle", "Demo::spare"],
            Select = ["name", "owner"],
            Where = QueryConstraint.And(
                QueryConstraint.Equal("@type", "PartUsage", "PartDefinition"),
                QueryConstraint.Or(QueryConstraint.Greater("multiplicityLower", "1").Inverted(), QueryConstraint.Less("multiplicityUpper", "4"))),
            OrderBy = ["name", "qualifiedName"],
        };

        var result = await model.QueryAsync(query, token);
        await model.QueryAsync(Query.All, token);

        Assert.Equal(
            [
                new Wire.QueryRequest
                {
                    ModelHash = Hash,
                    Query = new Wire.Query
                    {
                        Scope = { "Demo::vehicle", "Demo::spare" },
                        Select = { "name", "owner" },
                        Where = new Wire.Constraint
                        {
                            Composite = new Wire.CompositeConstraint
                            {
                                Operator = Wire.CompositeOperator.And,
                                Constraint =
                                {
                                    Primitive("@type", Wire.PrimitiveOperator.Equal, false, "PartUsage", "PartDefinition"),
                                    new Wire.Constraint
                                    {
                                        Composite = new Wire.CompositeConstraint
                                        {
                                            Operator = Wire.CompositeOperator.Or,
                                            Constraint =
                                            {
                                                Primitive("multiplicityLower", Wire.PrimitiveOperator.Greater, true, "1"),
                                                Primitive("multiplicityUpper", Wire.PrimitiveOperator.Less, false, "4"),
                                            },
                                        },
                                    },
                                },
                            },
                        },
                        OrderBy = { "name", "qualifiedName" },
                    },
                },
                new Wire.QueryRequest { ModelHash = Hash, Query = new Wire.Query() },
            ],
            handler.RequestsFor("Query").Select(sent => sent.Message(Wire.QueryRequest.Parser)));
        Assert.Equal(2, result.Elements.Count);
        var wheels = result.Elements[0];
        Assert.Equal(("Demo::vehicle::wheels", "PartUsage"), (wheels.Id, wheels.Type));
        Assert.Equal(new ValueDictionary<string>(new Dictionary<string, string> { ["documentation"] = string.Empty, ["name"] = "wheels", ["owner"] = "Demo::vehicle" }), wheels.Properties);
        Assert.Equal("Demo::vehicle::wheels (PartUsage)", wheels.ToString());
        Assert.Empty(result.Elements[1].Properties);
        Assert.Equal(new QueryResult([new QueryElement("Demo::Vehicle", "PartDefinition", [])]), new QueryResult([result.Elements[1]]));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task OslcTextIsSentAloneAndGatedBehindTheQueryCapability(ClientWire wire)
    {
        const string Text = "oslc.where=rdf:type=\"PartUsage\"&oslc.select=sysml:name";
        using var handler = Serving(wire).On("Query", Wire.QueryRequest.Parser, _ => new Wire.QueryResponse());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var token = TestContext.Current.CancellationToken;

        var none = await connection.ModelByHash(Hash).QueryOslcAsync(Text, token);

        Assert.Empty(none.Elements);
        Assert.Equal(new Wire.QueryRequest { ModelHash = Hash, OslcQuery = Text }, Assert.Single(handler.RequestsFor("Query")).Message(Wire.QueryRequest.Parser));
        Assert.Equal(Capabilities.Query, await RefusedAsync(wire, [Capabilities.Query, Capabilities.OslcQuery], model => model.QueryOslcAsync(Text, token)));
        Assert.Equal(Capabilities.OslcQuery, await RefusedAsync(wire, [Capabilities.OslcQuery], model => model.QueryOslcAsync(Text, token)));
        Assert.Equal(Capabilities.Query, await RefusedAsync(wire, [Capabilities.Query], model => model.QueryAsync(Query.All, token)));
        await Assert.ThrowsAsync<ArgumentException>(() => connection.ModelByHash(Hash).QueryOslcAsync(string.Empty, token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => connection.ModelByHash(Hash).QueryOslcAsync(null!, token));
        await Assert.ThrowsAsync<ArgumentNullException>(() => connection.ModelByHash(Hash).QueryAsync(null!, token));
        Assert.Single(handler.RequestsFor("Query"));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheOrderedMembershipAndMetaclassOperatorsAreSentAsTheStandardNamesThem(ClientWire wire)
    {
        using var handler = Serving(wire).On("Query", Wire.QueryRequest.Parser, _ => new Wire.QueryResponse());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;
        QueryConstraint[] sent =
        [
            QueryConstraint.GreaterOrEqual("multiplicityLower", "1"),
            QueryConstraint.LessOrEqual("multiplicityUpper", "4").Inverted(),
            QueryConstraint.In("name", "wheels", "spare"),
            QueryConstraint.InstanceOf("Usage", "Definition"),
        ];

        foreach (var where in sent)
        {
            await model.QueryAsync(new Query { Where = where }, token);
        }

        Assert.Equal(
            [
                Primitive("multiplicityLower", Wire.PrimitiveOperator.GreaterEqual, false, "1"),
                Primitive("multiplicityUpper", Wire.PrimitiveOperator.LessEqual, true, "4"),
                Primitive("name", Wire.PrimitiveOperator.In, false, "wheels", "spare"),
                Primitive("@type", Wire.PrimitiveOperator.InstanceOf, false, "Usage", "Definition"),
            ],
            handler.RequestsFor("Query").Select(request => request.Message(Wire.QueryRequest.Parser).Query.Where));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnOrderingIsGatedBehindItsCapabilityAndAQueryWithoutOneIsNot(ClientWire wire)
    {
        var token = TestContext.Current.CancellationToken;
        var ordered = new Query { OrderBy = ["name"] };
        using var handler = new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Except([Capabilities.QueryOrderBy])] }
            .On("Query", Wire.QueryRequest.Parser, _ => new Wire.QueryResponse());
        await using var connection = await FakeConnection.OpenAsync(handler);

        await connection.ModelByHash(Hash).QueryAsync(ordered with { OrderBy = [] }, token);

        Assert.Equal(Capabilities.QueryOrderBy, await RefusedAsync(wire, [Capabilities.QueryOrderBy], model => model.QueryAsync(ordered, token)));
        Assert.Equal(Capabilities.Query, await RefusedAsync(wire, [Capabilities.Query, Capabilities.QueryOrderBy], model => model.QueryAsync(ordered, token)));
        Assert.Empty(Assert.Single(handler.RequestsFor("Query")).Message(Wire.QueryRequest.Parser).Query.OrderBy);
    }

    [Fact]
    public void AConstraintCannotBeBuiltWithoutAnOperatorItsOperandsOrItsConstraints()
    {
        Assert.Throws<ArgumentOutOfRangeException>(() => new QueryConstraint.Comparison("@type", default, ["PartUsage"]));
        Assert.Throws<ArgumentOutOfRangeException>(() => new QueryConstraint.Comparison("@type", (ComparisonOperator)8, ["PartUsage"]));
        Assert.Throws<ArgumentException>(() => QueryConstraint.Equal("@type"));
        Assert.Throws<ArgumentException>(() => QueryConstraint.Equal(string.Empty, "x"));
        Assert.Throws<ArgumentNullException>(() => QueryConstraint.Equal(null!, "x"));
        Assert.Throws<ArgumentException>(() => QueryConstraint.Equal("name", [null!]));
        Assert.Throws<ArgumentException>(() => new QueryConstraint.Comparison("multiplicityLower", ComparisonOperator.Greater, ["1", "2"]));
        Assert.Throws<ArgumentException>(() => new QueryConstraint.Comparison("multiplicityLower", ComparisonOperator.Less, []));
        Assert.Throws<ArgumentException>(() => new QueryConstraint.Comparison("multiplicityLower", ComparisonOperator.GreaterOrEqual, ["1", "2"]));
        Assert.Throws<ArgumentException>(() => new QueryConstraint.Comparison("multiplicityLower", ComparisonOperator.LessOrEqual, ["1", "2"]));
        Assert.Throws<ArgumentException>(() => QueryConstraint.In("name"));
        Assert.Throws<ArgumentException>(() => QueryConstraint.InstanceOf());
        Assert.Throws<ArgumentException>(() => QueryConstraint.InstanceOf("Usage", string.Empty));
        Assert.Throws<ArgumentNullException>(() => QueryConstraint.InstanceOf(null!));
        Assert.Equal("property", Assert.Throws<ArgumentException>(() => new QueryConstraint.Comparison("name", ComparisonOperator.InstanceOf, ["Usage"])).ParamName);
        Assert.Throws<ArgumentException>(() => QueryConstraint.And());
        Assert.Throws<ArgumentException>(() => QueryConstraint.Or([null!]));
        Assert.Throws<ArgumentOutOfRangeException>(() => new QueryConstraint.Composite(default, [QueryConstraint.Equal("name", "x"), QueryConstraint.Equal("name", "y")]));
        Assert.Throws<ArgumentException>(() => new Query { Scope = [null!] });
        Assert.Throws<ArgumentNullException>(() => new Query { Select = null! });
        Assert.Throws<ArgumentNullException>(() => new Query { OrderBy = null! });
        Assert.Throws<ArgumentException>(() => new Query { OrderBy = [null!] });
    }

    [Fact]
    public void ACompositeOfOneConstraintIsRefusedAsTheStandardRefusesIt()
    {
        var one = QueryConstraint.Equal("name", "x");

        var and = Assert.Throws<ArgumentException>(() => QueryConstraint.And(one));
        var or = Assert.Throws<ArgumentException>(() => QueryConstraint.Or(one));
        var none = Assert.Throws<ArgumentException>(() => new QueryConstraint.Composite(CompositeOperator.And, []));

        Assert.StartsWith("A composite combines at least two constraints, got one; write the constraint on its own.", and.Message, StringComparison.Ordinal);
        Assert.Equal(and.Message, or.Message);
        Assert.StartsWith("A composite combines at least two constraints, got none.", none.Message, StringComparison.Ordinal);
        Assert.Equal(2, QueryConstraint.And(one, one).Constraints.Count);
    }

    [Fact]
    public void QueriesAndConstraintsAreImmutableValuesComparedByContent()
    {
        var scope = new List<string> { "Demo" };
        var query = new Query { Scope = scope, Where = QueryConstraint.Equal("name", "x") };
        scope.Add("Other");
        var greater = QueryConstraint.Greater("multiplicityLower", "1");

        Assert.Equal(["Demo"], query.Scope);
        Assert.Equal(new Query { Scope = ["Demo"], Where = QueryConstraint.Equal("name", "x") }, query);
        Assert.NotEqual(query, query with { Select = ["name"] });
        Assert.Equal(Query.All, new Query());
        Assert.Equal(new Query { OrderBy = ["name"] }, new Query { OrderBy = new List<string> { "name" } });
        Assert.NotEqual(query, query with { OrderBy = ["name"] });
        Assert.Equal(greater, greater.Inverted().Inverted());
        Assert.NotEqual<QueryConstraint>(greater, greater.Inverted());
        Assert.Equal(greater.GetHashCode(), QueryConstraint.Greater("multiplicityLower", "1").GetHashCode());
        Assert.NotEqual<QueryConstraint>(QueryConstraint.And(greater, greater.Inverted()), QueryConstraint.Or(greater, greater.Inverted()));
        Assert.True(QueryConstraint.And(greater, greater) == QueryConstraint.And(QueryConstraint.Greater("multiplicityLower", "1"), greater));
        Assert.True(greater != greater.Inverted());
        Assert.False(greater != QueryConstraint.Greater("multiplicityLower", "1"));
        Assert.Equal(QueryConstraint.Or(greater, greater).GetHashCode(), QueryConstraint.Or(QueryConstraint.Greater("multiplicityLower", "1"), greater).GetHashCode());
        Assert.NotEqual<QueryConstraint>(QueryConstraint.Equal("name", "x"), QueryConstraint.In("name", "x"));
        Assert.Equal("(not multiplicityLower > \"1\" or @type = (\"PartUsage\", \"PartDefinition\"))", QueryConstraint.Or(greater.Inverted(), QueryConstraint.Equal("@type", "PartUsage", "PartDefinition")).ToString());
        Assert.Equal(
            "(name < \"m\" and name <= \"n\" and multiplicityLower >= \"1\" and name in (\"m\") and not @type instanceOf (\"Usage\", \"Definition\"))",
            QueryConstraint.And(
                QueryConstraint.Less("name", "m"),
                QueryConstraint.LessOrEqual("name", "n"),
                QueryConstraint.GreaterOrEqual("multiplicityLower", "1"),
                QueryConstraint.In("name", "m"),
                QueryConstraint.InstanceOf("Usage", "Definition").Inverted()).ToString());
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnElementWithoutItsIdOrTypeIsAMalformedAnswer(ClientWire wire)
    {
        using var handler = Serving(wire).On("Query", Wire.QueryRequest.Parser, request => new Wire.QueryResponse
        {
            Elements = { request.OslcQuery == "id" ? new Wire.QueryResultElement { Type = "PartUsage" } : new Wire.QueryResultElement { Id = "Demo" } },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var id = await Assert.ThrowsAsync<TransportException>(() => model.QueryOslcAsync("id", token));
        var type = await Assert.ThrowsAsync<TransportException>(() => model.QueryOslcAsync("type", token));

        Assert.Equal(StatusCode.Internal, id.StatusCode);
        Assert.Contains("elements[0] with no id", id.Message, StringComparison.Ordinal);
        Assert.Contains("elements[0] with no type", type.Message, StringComparison.Ordinal);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task RunDocumentQuerySendsEveryBindingInOrderAndReadsEveryRow(ClientWire wire)
    {
        using var handler = Serving(wire).On("RunDocumentQuery", Wire.RunDocumentQueryRequest.Parser, _ => CompleteQueryDocsWire.DocumentQuery);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;
        var big = BigInteger.Pow(2, 64);
        var answered = new DocumentValue.ObjectValue(new DocumentObject(2, "Garage::car", new DocumentValue.ElementValue("Garage::Car", "PartDefinition")));
        DocumentBinding[] bindings =
        [
            new("root", DocumentValue.ElementRef("Garage::car"), new DocumentValue.ElementValue("Garage::Car", "PartDefinition")),
            new("objects", DocumentValue.ObjectRef("Garage::car.wheels[2]"), DocumentValue.ObjectRef(3), answered),
            new("scalars", new DocumentValue.TextValue("10"), new DocumentValue.BooleanValue(true), new DocumentValue.NumberValue(new Value.IntegerValue(-4))),
            new("numbers", new DocumentValue.NumberValue(new Value.BigIntegerValue(big)), new DocumentValue.NumberValue(new Value.RationalValue(new Rational(1, 3))), new DocumentValue.NumberValue(new Value.RealValue(-0.0))),
            new("mass", new DocumentValue.QuantityValue(new Quantity(new Value.RationalValue(new Rational(1, 4)), string.Empty, null))),
            new("root", DocumentValue.ElementRef("Garage::bike")),
            new("none"),
        ];

        var result = await model.RunDocumentQueryAsync("Garage::Checks", bindings, token);
        await model.RunDocumentQueryAsync("Garage::All", cancellationToken: token);

        Assert.Equal(
            [
                new Wire.RunDocumentQueryRequest
                {
                    ModelHash = Hash,
                    QueryId = "Garage::Checks",
                    Bindings =
                    {
                        Binding("root", new() { ElementId = "Garage::car" }, new() { ElementId = "Garage::Car", ElementType = "PartDefinition" }),
                        Binding(
                            "objects",
                            new() { Object = new Wire.DocumentObject { Path = "Garage::car.wheels[2]" } },
                            new() { Object = new Wire.DocumentObject { InstanceId = 3 } },
                            new() { Object = new Wire.DocumentObject { InstanceId = 2, Path = "Garage::car", Element = new Wire.DocumentValue { ElementId = "Garage::Car", ElementType = "PartDefinition" } } }),
                        Binding("scalars", new() { StringValue = "10" }, new() { BoolValue = true }, new() { IntValue = -4 }),
                        Binding("numbers", new() { BigIntValue = "18446744073709551616" }, new() { RationalValue = new Wire.Rational { Numerator = "1", Denominator = "3" } }, new() { RealValue = -0.0 }),
                        Binding("mass", new Wire.DocumentValue { Quantity = new Wire.Quantity { RationalMagnitude = new Wire.Rational { Numerator = "1", Denominator = "4" } } }),
                        Binding("root", new Wire.DocumentValue { ElementId = "Garage::bike" }),
                        Binding("none"),
                    },
                },
                new Wire.RunDocumentQueryRequest { ModelHash = Hash, QueryId = "Garage::All" },
            ],
            handler.RequestsFor("RunDocumentQuery").Select(sent => sent.Message(Wire.RunDocumentQueryRequest.Parser)));
        Assert.True(double.IsNegative(((Value.RealValue)((DocumentValue.NumberValue)bindings[3].Values[2]).Number).Value));
        Assert.Equal(["name", "values"], result.Columns);
        Assert.Equal(6, result.Rows.Count);
        var first = result.Rows[0];
        Assert.Equal(new DocumentValue.ElementValue("Garage::car", "PartUsage"), first.Element);
        Assert.Equal([new DocumentValue.TextValue("car")], first.Cells[0]);
        Assert.Equal(
            CompleteExecutionWire.DocumentValues.Select((value, i) => Internal.DocumentWire.Value(value, Model.RunDocumentQueryMethod, $"v{i}")),
            first.Cells[1]);
        Assert.Equal(new DocumentObject(2, "Garage::car", new DocumentValue.ElementValue("Garage::car", "PartUsage")), Assert.IsType<DocumentValue.ObjectValue>(result.Rows[1].Element).Value);
        Assert.Equal(2, result.Rows[1].Cells.Count);
        Assert.All(result.Rows[1].Cells, cell => Assert.Empty(cell));
        Assert.Equal("violated", Assert.IsType<DocumentValue.VerdictValue>(result.Rows[2].Element).Verdict.Verdict);
        Assert.Equal("on.dim", Assert.IsType<DocumentValue.StateValue>(result.Rows[3].Element).State.StatePath);
        Assert.Equal("send Switch to #3", Assert.IsType<DocumentValue.EventValue>(result.Rows[4].Element).Event.Text);
        Assert.Equal(new DocumentValue.ElementValue(null, "AssertConstraintUsage"), result.Rows[5].Element);
        Assert.Empty(result.Rows[2].Cells);
    }

    [Fact]
    public void ABindingRefusesWhatAQueryOnlyAnswersAndCopiesItsValues()
    {
        DocumentValue[] refused =
        [
            new DocumentValue.InfinityValue(),
            new DocumentValue.VerdictValue(new DocumentVerdict(null, "constraint", "t", "p", "holds", string.Empty, string.Empty, [])),
            new DocumentValue.StateValue(new DocumentState(null, "m", "s", "s", null, string.Empty, [])),
            new DocumentValue.EventValue(new DocumentEvent("entry", null, null, string.Empty, string.Empty, string.Empty, string.Empty, null, string.Empty, [], [], string.Empty, "entry", 0, 0, string.Empty, null)),
            new DocumentValue.ElementValue(null, "PartUsage"),
            new DocumentValue.ElementValue(string.Empty, string.Empty),
            new DocumentValue.ObjectValue(new DocumentObject(0, string.Empty, null)),
            new DocumentValue.ObjectValue(new DocumentObject(-1, "Garage::car", null)),
        ];
        var reasons = refused.Select(value => Assert.Throws<ArgumentException>(() => new DocumentBinding("p", value)).Message).ToList();
        var values = new List<DocumentValue> { DocumentValue.ElementRef("A") };
        var binding = new DocumentBinding("p", values);
        values.Add(DocumentValue.ElementRef("B"));

        Assert.All(reasons, reason => Assert.StartsWith("The value p[0] ", reason, StringComparison.Ordinal));
        Assert.Contains("infinity", reasons[0], StringComparison.Ordinal);
        Assert.Contains("ids count up from 1", reasons[6], StringComparison.Ordinal);
        Assert.Throws<ArgumentException>(() => new DocumentBinding(string.Empty, DocumentValue.ElementRef("A")));
        Assert.Throws<ArgumentNullException>(() => new DocumentBinding(null!, DocumentValue.ElementRef("A")));
        Assert.Throws<ArgumentException>(() => new DocumentBinding("p", [null!]));
        Assert.Throws<ArgumentException>(() => DocumentValue.ElementRef(string.Empty));
        Assert.Throws<ArgumentException>(() => DocumentValue.ObjectRef(string.Empty));
        Assert.Throws<ArgumentOutOfRangeException>(() => DocumentValue.ObjectRef(0));
        Assert.Equal([DocumentValue.ElementRef("A")], binding.Values);
        Assert.Equal(new DocumentBinding("p", DocumentValue.ElementRef("A")), binding);
        Assert.Equal(new DocumentValue.ElementValue("A", string.Empty), DocumentValue.ElementRef("A"));
        Assert.Equal(new DocumentObject(5, string.Empty, null), DocumentValue.ObjectRef(5).Value);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task DocumentQueryGatesAreCheckedInTheServicesOrderAndAnExactRationalIsSentAsTheSameReal(ClientWire wire)
    {
        var token = TestContext.Current.CancellationToken;
        var big = new DocumentValue.NumberValue(new Value.BigIntegerValue(BigInteger.Pow(2, 70)));
        var third = new DocumentValue.NumberValue(new Value.RationalValue(new Rational(1, 3)));
        var bigMass = new DocumentValue.QuantityValue(new Quantity(new Value.BigIntegerValue(BigInteger.Pow(2, 70)), string.Empty, null));
        var thirdMass = new DocumentValue.QuantityValue(new Quantity(new Value.RationalValue(new Rational(1, 3)), string.Empty, null));
        Task Run(Model model, params DocumentValue[] values) => model.RunDocumentQueryAsync("Q", [new DocumentBinding("p", values)], token);

        Assert.Equal(Capabilities.DocumentQuery, await RefusedAsync(wire, [Capabilities.DocumentQuery, Capabilities.BigIntValues], model => Run(model, big)));
        Assert.Equal(Capabilities.BigIntValues, await RefusedAsync(wire, [Capabilities.BigIntValues, Capabilities.RationalValues], model => Run(model, third, big)));
        Assert.Equal(Capabilities.RationalValues, await RefusedAsync(wire, [Capabilities.RationalValues], model => Run(model, third, big)));
        Assert.Equal(Capabilities.BigIntValues, await RefusedAsync(wire, [Capabilities.BigIntValues], model => Run(model, bigMass)));
        Assert.Equal(Capabilities.RationalValues, await RefusedAsync(wire, [Capabilities.RationalValues], model => Run(model, thirdMass)));

        using var handler = new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Except([Capabilities.RationalValues])] }
            .On("RunDocumentQuery", Wire.RunDocumentQueryRequest.Parser, _ => new Wire.RunDocumentQueryResponse());
        await using var connection = await FakeConnection.OpenAsync(handler);
        var quarter = new DocumentValue.NumberValue(new Value.RationalValue(new Rational(1, 4)));
        var quarterMass = new DocumentValue.QuantityValue(new Quantity(new Value.RationalValue(new Rational(-5, 2)), string.Empty, null));
        var unreduced = new DocumentValue.QuantityValue(new Quantity(new Value.IntegerValue(2), "kg", null));

        await Run(connection.ModelByHash(Hash), quarter, quarterMass);
        await Assert.ThrowsAsync<ArgumentException>(() => Run(connection.ModelByHash(Hash), unreduced));
        await Assert.ThrowsAsync<ArgumentException>(() => connection.ModelByHash(Hash).RunDocumentQueryAsync(string.Empty, cancellationToken: token));
        await Assert.ThrowsAsync<ArgumentException>(() => connection.ModelByHash(Hash).RunDocumentQueryAsync("Q", [null!], token));

        Assert.Equal(
            Binding("p", new() { RealValue = 0.25 }, new() { Quantity = new Wire.Quantity { RealMagnitude = -2.5 } }),
            Assert.Single(Assert.Single(handler.RequestsFor("RunDocumentQuery")).Message(Wire.RunDocumentQueryRequest.Parser).Bindings));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AMissingModelAMissingDeclarationAndAFaultInTheModelsDeclarationsAreToldApart(ClientWire wire)
    {
        using var handler = Serving(wire);
        handler.Respond = (sent, _) => Task.FromResult(sent.Method switch
        {
            "RunDocumentQuery" => sent.Message(Wire.RunDocumentQueryRequest.Parser).QueryId switch
            {
                "evicted" => handler.Refuse(StatusCode.NotFound, "model not found: h"),
                "other" => handler.Refuse(StatusCode.NotFound, "model not found: g"),
                "missing" => handler.Refuse(StatusCode.NotFound, "symbol not found: missing"),
                "broken" => handler.Refuse(StatusCode.FailedPrecondition, "unknown property \"nope\" (declared in m.sysml)"),
                _ => handler.Refuse(StatusCode.ResourceExhausted, "visit budget exhausted"),
            },
            "RenderDocument" => sent.Message(Wire.RenderDocumentRequest.Parser).DocumentId == "broken"
                ? handler.Refuse(StatusCode.FailedPrecondition, "a section names no query")
                : handler.Refuse(StatusCode.NotFound, "model not found: h"),
            _ => sent.Message(Wire.RenderViewRequest.Parser).View switch
            {
                "evicted" => handler.Refuse(StatusCode.NotFound, "model not found: h"),
                "missing" => handler.Refuse(StatusCode.NotFound, "no view named missing"),
                _ => handler.Refuse(StatusCode.FailedPrecondition, "precondition"),
            },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var evicted = await Assert.ThrowsAsync<ModelNotFoundException>(() => model.RunDocumentQueryAsync("evicted", cancellationToken: token));
        var other = await Assert.ThrowsAsync<ServiceException>(() => model.RunDocumentQueryAsync("other", cancellationToken: token));
        var missing = await Assert.ThrowsAsync<ServiceException>(() => model.RunDocumentQueryAsync("missing", cancellationToken: token));
        var broken = await Assert.ThrowsAsync<QueryException>(() => model.RunDocumentQueryAsync("broken", cancellationToken: token));
        var exhausted = await Assert.ThrowsAsync<ServiceException>(() => model.RunDocumentQueryAsync("big", cancellationToken: token));
        var document = await Assert.ThrowsAsync<QueryException>(() => model.RenderDocumentAsync("broken", cancellationToken: token));
        var gone = await Assert.ThrowsAsync<ModelNotFoundException>(() => model.RenderDocumentAsync("Observatory::MassReport", cancellationToken: token));
        var viewGone = await Assert.ThrowsAsync<ModelNotFoundException>(() => model.RenderViewAsync("evicted", cancellationToken: token));
        var noView = await Assert.ThrowsAsync<ServiceException>(() => model.RenderViewAsync("missing", cancellationToken: token));
        var precondition = await Assert.ThrowsAsync<ServiceException>(() => model.RenderViewAsync("other", cancellationToken: token));

        Assert.Equal((Model.RunDocumentQueryMethod, "model not found: h"), (evicted.Method, evicted.ServiceMessage));
        Assert.All([other, missing, exhausted, noView, precondition], error => Assert.Equal(typeof(ServiceException), error.GetType()));
        Assert.Equal((StatusCode.NotFound, "symbol not found: missing"), (missing.StatusCode, missing.ServiceMessage));
        Assert.Equal(StatusCode.ResourceExhausted, exhausted.StatusCode);
        Assert.IsAssignableFrom<ModelException>(broken);
        Assert.Equal((StatusCode.FailedPrecondition, "unknown property \"nope\" (declared in m.sysml)", Model.RunDocumentQueryMethod), (broken.StatusCode, broken.ServiceMessage, broken.Method));
        Assert.Equal(broken.ServiceMessage, broken.Message);
        Assert.Empty(broken.Diagnostics);
        Assert.Equal(wire == ClientWire.Grpc, broken.InnerException is Grpc.Core.RpcException);
        Assert.Equal(Model.RenderDocumentMethod, document.Method);
        Assert.Equal(Model.RenderDocumentMethod, gone.Method);
        Assert.Equal(Model.RenderViewMethod, viewGone.Method);
        Assert.Equal((StatusCode.NotFound, "no view named missing"), (noView.StatusCode, noView.ServiceMessage));
        Assert.Equal(StatusCode.FailedPrecondition, precondition.StatusCode);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ARowSelectingNoElementOrHoldingAnUnknownArmIsRefused(ClientWire wire)
    {
        using var handler = Serving(wire).On("RunDocumentQuery", Wire.RunDocumentQueryRequest.Parser, request => request.QueryId switch
        {
            "bare" => new Wire.RunDocumentQueryResponse { Rows = { new Wire.DocumentQueryRow() } },
            "malformed" => new Wire.RunDocumentQueryResponse
            {
                Rows = { new Wire.DocumentQueryRow { Element = CompleteExecutionWire.Element, Cells = { new Wire.DocumentQueryCell { Values = { new Wire.DocumentValue { Infinity = false } } } } } },
            },
            _ => new Wire.RunDocumentQueryResponse { Rows = { new Wire.DocumentQueryRow { Element = Unknown() } } },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var bare = await Assert.ThrowsAsync<TransportException>(() => model.RunDocumentQueryAsync("bare", cancellationToken: token));
        var malformed = await Assert.ThrowsAsync<TransportException>(() => model.RunDocumentQueryAsync("malformed", cancellationToken: token));
        Assert.Contains("rows[0]: it selects no element", bare.Message, StringComparison.Ordinal);
        Assert.Contains("rows[0].cells[0].values[0]", malformed.Message, StringComparison.Ordinal);
        if (wire == ClientWire.ConnectJson)
        {
            // JSON drops a field its schema does not declare, so the newer arm arrives as no arm at all.
            var row = Assert.Single((await model.RunDocumentQueryAsync("unknown", cancellationToken: token)).Rows);
            Assert.Equal(new DocumentValue.ElementValue(null, string.Empty), row.Element);
        }
        else
        {
            var unknown = await Assert.ThrowsAsync<UnsupportedValueException>(() => model.RunDocumentQueryAsync("unknown", cancellationToken: token));
            Assert.Equal(Model.RunDocumentQueryMethod, unknown.Method);
        }
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task RenderDocumentSendsTheFormAndReadsTheContentAndFileName(ClientWire wire)
    {
        using var handler = Serving(wire).On("RenderDocument", Wire.RenderDocumentRequest.Parser, request => request.DocumentId switch
        {
            "unnamed" => new Wire.RenderDocumentResponse { Markdown = "# T\n" },
            "both" => new Wire.RenderDocumentResponse { Markdown = "# T\n", Html = "<h1>T</h1>" },
            "also" => new Wire.RenderDocumentResponse { Asciidoc = "= T\n", Html = "<h1>T</h1>" },
            _ => request.Form switch
            {
                "html" => CompleteQueryDocsWire.Html,
                "asciidoc" => CompleteQueryDocsWire.AsciiDoc,
                _ => CompleteQueryDocsWire.Markdown,
            },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var markdown = await model.RenderDocumentAsync("Observatory::MassReport", cancellationToken: token);
        var html = await model.RenderDocumentAsync("Observatory::MassReport", DocumentForm.Html, token);
        var asciidoc = await model.RenderDocumentAsync("Observatory::MassReport", DocumentForm.AsciiDoc, token);
        var unnamed = await model.RenderDocumentAsync("unnamed", DocumentForm.Markdown, token);
        var both = await Assert.ThrowsAsync<TransportException>(() => model.RenderDocumentAsync("both", DocumentForm.Html, token));
        var also = await Assert.ThrowsAsync<TransportException>(() => model.RenderDocumentAsync("also", DocumentForm.AsciiDoc, token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.RenderDocumentAsync("Observatory::MassReport", (DocumentForm)3, token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.RenderDocumentAsync(string.Empty, cancellationToken: token));

        Assert.Equal(
            [
                new Wire.RenderDocumentRequest { ModelHash = Hash, DocumentId = "Observatory::MassReport" },
                new Wire.RenderDocumentRequest { ModelHash = Hash, DocumentId = "Observatory::MassReport", Form = "html" },
                new Wire.RenderDocumentRequest { ModelHash = Hash, DocumentId = "Observatory::MassReport", Form = "asciidoc" },
                new Wire.RenderDocumentRequest { ModelHash = Hash, DocumentId = "unnamed" },
                new Wire.RenderDocumentRequest { ModelHash = Hash, DocumentId = "both", Form = "html" },
                new Wire.RenderDocumentRequest { ModelHash = Hash, DocumentId = "also", Form = "asciidoc" },
            ],
            handler.RequestsFor("RenderDocument").Select(sent => sent.Message(Wire.RenderDocumentRequest.Parser)));
        Assert.Equal(new RenderedDocument(DocumentForm.Markdown, "# Telescope Mass Report\n", "Observatory-MassReport.md"), markdown);
        Assert.Equal(markdown.Content, markdown.ToString());
        Assert.Equal((DocumentForm.Html, "Observatory-MassReport.html"), (html.Form, html.FileName));
        Assert.StartsWith("<!DOCTYPE html>", html.Content, StringComparison.Ordinal);
        Assert.Equal(new RenderedDocument(DocumentForm.AsciiDoc, "= Telescope Mass Report\n", "Observatory-MassReport.adoc"), asciidoc);
        Assert.Null(unnamed.FileName);
        Assert.Equal((StatusCode.Internal, StatusCode.Internal), (both.StatusCode, also.StatusCode));
        Assert.Throws<ArgumentException>(() => new RenderedDocument(DocumentForm.Markdown, string.Empty, string.Empty));
        Assert.Equal(Capabilities.RenderDocument, await RefusedAsync(wire, [Capabilities.RenderDocument, Capabilities.RenderDocumentHtml], m => m.RenderDocumentAsync("D", DocumentForm.Html, token)));
        Assert.Equal(Capabilities.RenderDocumentHtml, await RefusedAsync(wire, [Capabilities.RenderDocumentHtml], m => m.RenderDocumentAsync("D", DocumentForm.Html, token)));
        Assert.Equal(Capabilities.RenderDocument, await RefusedAsync(wire, [Capabilities.RenderDocument, Capabilities.RenderDocumentAsciidoc], m => m.RenderDocumentAsync("D", DocumentForm.AsciiDoc, token)));
        Assert.Equal(Capabilities.RenderDocumentAsciidoc, await RefusedAsync(wire, [Capabilities.RenderDocumentAsciidoc], m => m.RenderDocumentAsync("D", DocumentForm.AsciiDoc, token)));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task RenderViewSendsEveryOptionAndReadsEveryFieldOfTheRendering(ClientWire wire)
    {
        using var handler = Serving(wire).On("RenderView", Wire.RenderViewRequest.Parser, _ => CompleteQueryDocsWire.View);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        var view = await model.RenderViewAsync(
            "PlantViews::placedView",
            new RenderViewOptions { Ports = RenderViewPorts.Full, Show = [ViewNotation.PortSymbols, ViewNotation.EdgeLabels], Depth = 2, Compartments = RenderViewCompartments.Most },
            token);
        await model.RenderViewAsync("#interconnection:Plant::Loop", cancellationToken: token);
        await model.RenderViewAsync("#interconnection:Plant::Loop", new RenderViewOptions { Compartments = RenderViewCompartments.Ports }, token);

        Assert.Equal(
            [
                new Wire.RenderViewRequest { ModelHash = Hash, View = "PlantViews::placedView", Ports = "full", Show = { "port-symbols", "edge-labels" }, Depth = 2, Compartments = "most" },
                new Wire.RenderViewRequest { ModelHash = Hash, View = "#interconnection:Plant::Loop" },
                new Wire.RenderViewRequest { ModelHash = Hash, View = "#interconnection:Plant::Loop", Compartments = "ports" },
            ],
            handler.RequestsFor("RenderView").Select(sent => sent.Message(Wire.RenderViewRequest.Parser)));
        Assert.Equal(("PlantViews::placedView", "interconnection", "asInterconnectionDiagram"), (view.View, view.Kind, view.Stated));
        var (loop, pump, tank, pressure) = (view.Nodes[0], view.Nodes[1], view.Nodes[2], view.Nodes[3]);
        Assert.Equal([loop], view.Roots);
        Assert.Equal([pump], view.ChildrenOf(loop));
        Assert.Equal([tank, pressure], view.ChildrenOf(pump));
        Assert.Empty(view.ChildrenOf(tank));
        Assert.Equal((string.Empty, false, string.Empty, false), (pump.Compartment, pump.Inherited, pump.Arrow, pump.Parallel));
        Assert.Equal(("right", false), (tank.Arrow, tank.Parallel));
        Assert.Equal((string.Empty, true), (loop.Arrow, loop.Parallel));
        Assert.Equal(("n4", "attribute", "pressure", "Real", "n2", "attributes", true), (pressure.Id, pressure.Kind, pressure.Name, pressure.Type, pressure.Parent, pressure.Compartment, pressure.Inherited));
        Assert.Same(pump, view.Node("n2"));
        Assert.Null(view.Node("n9"));
        Assert.Equal(("n1", "part def", "Plant::Loop", false, "already shown", string.Empty), (loop.Id, loop.Kind, loop.Name, loop.NameSynthesized, loop.Detail, loop.Parent));
        Assert.Equal(new SourceSpan("views.sysml", 10, 3, 11, 9), loop.Origin);
        Assert.Equal(new ViewGeometry(300, 40, 120, 60.5, HasSize: true, Collapsed: true, Unplaced: false), loop.Geometry);
        Assert.Equal(new ViewStyle("#FFE8BD", "#333333", "#000000", "Arial", 11, bold: true, italic: true), loop.Style);
        Assert.Equal((true, "Pump", "pump : Pump", true, "n1"), (pump.NameSynthesized, pump.Type, pump.Text, pump.StandIn, pump.Parent));
        Assert.Equal([new ViewPort("n2.0", "outlet", "FluidPort", "out", inherited: false), new ViewPort("n2.1", "vent", string.Empty, string.Empty, inherited: true)], pump.Ports);
        Assert.Equal(new SourceSpan("lib.sysml", 0, 0, 0, 0), pump.Origin);
        Assert.Equal(new ViewGeometry(0, 0, 0, 0, HasSize: false, Collapsed: true, Unplaced: true), pump.Geometry);
        Assert.Null(pump.Style);
        Assert.Equal((null, null, null), (tank.Origin, tank.Geometry, tank.Style));
        var supply = view.Edges[0];
        Assert.Equal(("n2", "n3", "n2.0", "n3.0", "supply", "supply", "connection"), (supply.From, supply.To, supply.FromPort, supply.ToPort, supply.Label, supply.Name, supply.Kind));
        Assert.Equal([new ViewPoint(400, 70), new ViewPoint(0.5, 120.25)], supply.Route);
        Assert.Equal(new ViewStyle(string.Empty, "#0000FF", string.Empty, string.Empty, 0, bold: false, italic: false), supply.Style);
        Assert.Equal(new SourceSpan("views.sysml", 20, 3, 21, 9), supply.Origin);
        Assert.Equal((null, null), (view.Edges[1].Origin, view.Edges[1].Style));
        Assert.Empty(view.Edges[1].Route);
        Assert.Equal(["name", "type"], view.Columns);
        Assert.Equal([new ViewRow(["pump", "Pump"], new SourceSpan("views.sysml", 12, 3, 13, 9)), new ViewRow(["tank", string.Empty], null)], view.Rows);
        Assert.Equal(new ViewCanvas("px", 1200, 800, hasSize: true), view.Canvas);
        Assert.Equal(new ViewNote("anchored", "n3", string.Empty, string.Empty, 650, 40, 100, 30, hasSize: true, new SourceSpan("views.sysml", 30, 3, 31, 9)), view.Notes[0]);
        Assert.Equal(new ViewNote("check pressure", string.Empty, "n2", "n3", 420, 140, 0, 0, hasSize: false, null), view.Notes[1]);
        Assert.Equal(["a picture was refused", "nothing more"], view.Notices);
        Assert.Equal(["port-symbols", "edge-labels"], view.Shown);
        Assert.Equal(new ViewFrame("placedView", "PlantViews::PlacedView", ["expose Plant::**;"], ["@SysML::PartUsage"], ["placed by hand", "reviewed"]), view.Frame);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AGeometryStyleCanvasOrOriginTheRenderingOmitsStaysAbsentRatherThanZero(ClientWire wire)
    {
        using var handler = Serving(wire).On("RenderView", Wire.RenderViewRequest.Parser, _ => CompleteQueryDocsWire.SparseView);
        await using var connection = await FakeConnection.OpenAsync(handler);

        var view = await connection.ModelByHash(Hash).RenderViewAsync("RenderViewDemo::connections", cancellationToken: TestContext.Current.CancellationToken);

        Assert.Null(view.Canvas);
        var node = Assert.Single(view.Nodes);
        Assert.Equal((null, null, null), (node.Origin, node.Geometry, node.Style));
        Assert.Empty(node.Ports);
        Assert.Equal((null, null), (view.Edges[0].Origin, view.Edges[0].Style));
        Assert.Null(Assert.Single(view.Rows).Origin);
        Assert.Null(Assert.Single(view.Notes).Origin);
        Assert.Equal((string.Empty, 0, 0, false), (view.Notes[0].Text, view.Notes[0].X, view.Notes[0].Width, view.Notes[0].HasSize));
        Assert.Empty(view.Notices);
        Assert.Empty(view.Shown);
        Assert.Empty(view.Columns);
        Assert.Null(view.Frame);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task InvalidViewOptionsAreRefusedBeforeSendingAndNodesOutOfOrderOrRowsWithoutABoxAreMalformed(ClientWire wire)
    {
        using var handler = Serving(wire).On("RenderView", Wire.RenderViewRequest.Parser, request => new Wire.RenderViewResponse
        {
            Nodes =
            {
                request.View switch
                {
                    "orphan" => [new Wire.RenderNode { Id = "n1", Parent = "n9" }],
                    "late" => [new Wire.RenderNode { Id = "n2", Parent = "n1" }, new Wire.RenderNode { Id = "n1" }],
                    "self" => [new Wire.RenderNode { Id = "n1", Parent = "n1" }],
                    "unhoused" => [new Wire.RenderNode { Id = "n1", Compartment = "parts" }],
                    "nested" => [new Wire.RenderNode { Id = "n1" }, new Wire.RenderNode { Id = "n2", Parent = "n1", Compartment = "parts" }, new Wire.RenderNode { Id = "n3", Parent = "n2", Compartment = "ports" }],
                    _ => new[] { new Wire.RenderNode { Id = "n1" }, new Wire.RenderNode { Id = "n1" } },
                },
            },
        });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = connection.ModelByHash(Hash);
        var token = TestContext.Current.CancellationToken;

        await Assert.ThrowsAsync<ArgumentException>(() => model.RenderViewAsync("V", new RenderViewOptions { Depth = 0 }, token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.RenderViewAsync("V", new RenderViewOptions { Depth = -1 }, token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.RenderViewAsync("V", new RenderViewOptions { Ports = (RenderViewPorts)2 }, token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.RenderViewAsync("V", new RenderViewOptions { Compartments = (RenderViewCompartments)3 }, token));
        await Assert.ThrowsAsync<ArgumentException>(() => model.RenderViewAsync(string.Empty, cancellationToken: token));
        Assert.Throws<ArgumentException>(() => new RenderViewOptions { Show = [null!] });
        Assert.Throws<ArgumentNullException>(() => new ViewFrame(null!, string.Empty, [], [], []));
        Assert.Throws<ArgumentNullException>(() => new ViewFrame("v", string.Empty, null!, [], []));
        Assert.Throws<ArgumentException>(() => new ViewFrame("v", string.Empty, [], [null!], []));
        Assert.Empty(handler.RequestsFor("RenderView"));
        string[] names = ["orphan", "late", "self", "twice", "unhoused", "nested"];
        var malformed = names
            .Select(name => Assert.ThrowsAsync<TransportException>(() => model.RenderViewAsync(name, cancellationToken: token)))
            .ToList();
        var messages = (await Task.WhenAll(malformed)).Select(error => error.Message).ToList();

        Assert.Contains("nodes[0] names \"n9\" as its parent", messages[0], StringComparison.Ordinal);
        Assert.Contains("nodes[0] names \"n1\" as its parent", messages[1], StringComparison.Ordinal);
        Assert.Contains("nodes[0] names \"n1\" as its parent", messages[2], StringComparison.Ordinal);
        Assert.Contains("nodes[1] repeats the id \"n1\"", messages[3], StringComparison.Ordinal);
        Assert.Contains("nodes[0] is a row of the compartment \"parts\" but names no parent whose box holds it", messages[4], StringComparison.Ordinal);
        Assert.Contains("nodes[2] is a row of the compartment \"ports\" of \"n2\", which is itself a row and has no box", messages[5], StringComparison.Ordinal);
        Assert.Equal(Capabilities.RenderView, await RefusedAsync(wire, [Capabilities.RenderView], m => m.RenderViewAsync("V", cancellationToken: token)));
    }

    private static FakeServiceHandler Serving(ClientWire wire) => new(wire) { Capabilities = FakeModels.ModelCapabilities };

    private static Wire.Constraint Primitive(string property, Wire.PrimitiveOperator op, bool inverse, params string[] values) =>
        new() { Primitive = new Wire.PrimitiveConstraint { Property = property, Operator = op, Inverse = inverse, Value = { values } } };

    private static Wire.DocumentQueryBinding Binding(string parameter, params Wire.DocumentValue[] values) => new() { Parameter = parameter, Values = { values } };

    // A value of an arm a newer schema adds: field 99, which this client's schema does not declare.
    private static Wire.DocumentValue Unknown() => Wire.DocumentValue.Parser.ParseFrom([0x9a, 0x06, 0x00]);

    // The capability a call is refused for when the service lacks the given ones, after checking nothing was sent.
    private static async Task<string> RefusedAsync(ClientWire wire, string[] withheld, Func<Model, Task> call)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Except(withheld)] };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var refused = await Assert.ThrowsAsync<CapabilityException>(() => call(connection.ModelByHash(Hash)));

        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
        return refused.Capability;
    }
}
