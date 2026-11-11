using System;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// GetElementProperties through the typed API: the element and the property names asked for sent as given, every
/// fact of every property and every arm of a value read, an element the form does not know told apart from a model
/// with no form and from a model the service does not hold, and what no service would accept refused before sending.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelElementPropertiesTests
{
    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ElementPropertiesAsyncSendsTheNamesInOrderAndReadsEveryArm(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .On("GetElementProperties", Wire.GetElementPropertiesRequest.Parser, _ => CompleteWire.ElementProperties);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var read = await model.ElementPropertiesAsync("Demo::Pair", ["inheritedFeature", "literals", "connectorEnd"], TestContext.Current.CancellationToken);
        var every = await model.ElementPropertiesAsync("Demo::Pair", cancellationToken: TestContext.Current.CancellationToken);

        Assert.Equal(
            [
                new Wire.GetElementPropertiesRequest { ModelHash = "h", ElementId = "Demo::Pair", Properties = { "inheritedFeature", "literals", "connectorEnd" } },
                new Wire.GetElementPropertiesRequest { ModelHash = "h", ElementId = "Demo::Pair" },
            ],
            handler.RequestsFor("GetElementProperties").Select(request => request.Message(Wire.GetElementPropertiesRequest.Parser)));
        Assert.Equal(
            new ElementProperties(
                new ElementRef("e-pair", "Demo::Pair", "PartDefinition"),
                [
                    new ElementProperty(
                        "inheritedFeature",
                        "Type",
                        true,
                        true,
                        [
                            new ElementPropertyValue.ElementValue(new ElementRef("e-p", "Demo::Unit::p", "PortUsage")),
                            new ElementPropertyValue.ElementValue(new ElementRef("lib-7", string.Empty, string.Empty)),
                        ],
                        string.Empty),
                    new ElementProperty(
                        "literals",
                        "Element",
                        true,
                        false,
                        [
                            new ElementPropertyValue.TextValue("in"),
                            new ElementPropertyValue.BooleanValue(true),
                            new ElementPropertyValue.IntegerValue(-3),
                            new ElementPropertyValue.RealValue(2.5),
                            new ElementPropertyValue.NumberValue("123456789012345678901234567890"),
                        ],
                        string.Empty),
                    new ElementProperty("connectorEnd", string.Empty, false, false, [], "PartDefinition declares no property connectorEnd"),
                ]),
            read);
        Assert.Equal(read, every);
        var inherited = read.Find("inheritedFeature")!;
        Assert.Equal(["Demo::Unit::p", "lib-7"], inherited.Elements.Select(element => element.ToString()));
        Assert.Equal(new ElementPropertyValue.ElementValue(new ElementRef("e-p", "Demo::Unit::p", "PortUsage")), inherited.Value);
        Assert.True(inherited.IsSupported);
        Assert.False(read.Find("connectorEnd")!.IsSupported);
        Assert.Null(read.Find("connectorEnd")!.Value);
        Assert.Empty(read.Find("literals")!.Elements);
        Assert.Null(read.Find("isAbstract"));
        Assert.Equal(["in", "true", "-3", "2.5", "123456789012345678901234567890"], read.Find("literals")!.Values.Select(value => value.ToString()));
        Assert.Equal("inheritedFeature = [Demo::Unit::p, lib-7]", inherited.ToString());
        Assert.Equal("connectorEnd: PartDefinition declares no property connectorEnd", read.Find("connectorEnd")!.ToString());
        Assert.Equal("Demo::Pair (PartDefinition): inheritedFeature, literals, connectorEnd", read.ToString());
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnElementTheFormDoesNotKnowIsNotFoundAndAModelWithoutAFormFails(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .On("GetElementProperties", Wire.GetElementPropertiesRequest.Parser, request => request.ElementId switch
            {
                "Demo::Nope" => new Wire.GetElementPropertiesResponse { Error = "element not found: Demo::Nope" },
                "Demo::Broken" => new Wire.GetElementPropertiesResponse { Error = "a.sysml: the document has syntax errors" },
                _ => new Wire.GetElementPropertiesResponse(),
            });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var missing = await Assert.ThrowsAsync<SymbolNotFoundException>(() => model.ElementPropertiesAsync("Demo::Nope", cancellationToken: TestContext.Current.CancellationToken));
        var broken = await Assert.ThrowsAsync<ModelException>(() => model.ElementPropertiesAsync("Demo::Broken", cancellationToken: TestContext.Current.CancellationToken));
        var malformed = await Assert.ThrowsAsync<TransportException>(() => model.ElementPropertiesAsync("Demo::Empty", cancellationToken: TestContext.Current.CancellationToken));

        Assert.Equal(("Demo::Nope", "element not found: Demo::Nope", Model.GetElementPropertiesMethod), (missing.Name, missing.Message, missing.Method));
        Assert.True(missing.Suggestions.IsEmpty);
        Assert.IsNotType<SymbolNotFoundException>(broken);
        Assert.Equal("a.sysml: the document has syntax errors", broken.Message);
        Assert.Equal(StatusCode.Internal, malformed.StatusCode);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AValueNamingNoArmIsUnsupportedAtItsPath(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .On("GetElementProperties", Wire.GetElementPropertiesRequest.Parser, _ => new Wire.GetElementPropertiesResponse
            {
                Element = new Wire.ElementRef { Id = "x" },
                Properties = { new Wire.ElementProperty { Name = "p" }, new Wire.ElementProperty { Name = "q", Values = { new Wire.ElementPropertyValue { BoolValue = false }, new Wire.ElementPropertyValue() } } },
            });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var empty = await Assert.ThrowsAsync<UnsupportedValueException>(() => model.ElementPropertiesAsync("x", cancellationToken: TestContext.Current.CancellationToken));

        Assert.Equal(("properties[1].values[1]", Model.GetElementPropertiesMethod), (empty.Path, empty.Method));
        Assert.Equal("The service answered a property value at properties[1].values[1] that names no kind of value.", empty.Message);
        Assert.True(empty.ValueBytes.IsEmpty);
    }

    [Fact]
    public void AValueOfAnArmThisClientDoesNotKnowIsUnsupportedAndKeepsItsBytes()
    {
        byte[] unknownArm = [0x3a, 0x01, 0x78];
        var response = new Wire.GetElementPropertiesResponse
        {
            Element = new Wire.ElementRef { Id = "x" },
            Properties = { new Wire.ElementProperty { Name = "p", Values = { Wire.ElementPropertyValue.Parser.ParseFrom(unknownArm) } } },
        };

        var newer = Assert.Throws<UnsupportedValueException>(() => Internal.Mapping.ModelWire.ElementProperties(response, CompleteWire.Method));

        Assert.Equal(Wire.ElementPropertyValue.ValueOneofCase.None, response.Properties[0].Values[0].ValueCase);
        Assert.Equal("The service answered a property value at properties[0].values[0] that is of a kind this client does not know; a newer client may read it.", newer.Message);
        Assert.Equal(unknownArm, newer.ValueBytes.ToArray());
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task WhatNoServiceWouldAnswerIsRefusedBeforeAnythingIsSent(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);
        using var older = new FakeServiceHandler(wire) { Capabilities = [.. FakeModels.ModelCapabilities.Where(name => name != Capabilities.ElementProperties)] };
        await using var olderConnection = await FakeConnection.OpenAsync(older);
        var model = connection.ModelByHash("h");

        await Assert.ThrowsAsync<ArgumentNullException>(() => model.ElementPropertiesAsync(null!, cancellationToken: TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentException>(() => model.ElementPropertiesAsync(" ", cancellationToken: TestContext.Current.CancellationToken));
        var blank = await Assert.ThrowsAsync<ArgumentException>(() => model.ElementPropertiesAsync("Demo", ["name", " "], TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentException>(() => model.ElementPropertiesAsync("Demo", ["name", null!], TestContext.Current.CancellationToken));
        var refused = await Assert.ThrowsAsync<CapabilityException>(() => olderConnection.ModelByHash("h").ElementPropertiesAsync("Demo", cancellationToken: TestContext.Current.CancellationToken));

        Assert.Equal("properties", blank.ParamName);
        Assert.Equal(Capabilities.ElementProperties, refused.Capability);
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
        Assert.Equal(["GetServerInfo"], older.Requests.Select(request => request.Method));
    }

    [Fact]
    public void AnElementIsNamedByItsQualifiedNameOrElseItsId()
    {
        Assert.Equal("Demo::Pair", new ElementRef("e1", "Demo::Pair", "PartDefinition").ToString());
        Assert.Equal("e2", new ElementRef("e2", string.Empty, string.Empty).ToString());
        Assert.Equal("e2", new ElementProperties(new ElementRef("e2", string.Empty, string.Empty), []).ToString());
        Assert.Throws<ArgumentNullException>(() => new ElementRef(null!, string.Empty, string.Empty));
        Assert.Throws<ArgumentNullException>(() => new ElementProperty("p", string.Empty, false, false, [], null!));
        Assert.Throws<ArgumentException>(() => new ElementProperties(new ElementRef("e", string.Empty, string.Empty), [null!]));
    }
}
