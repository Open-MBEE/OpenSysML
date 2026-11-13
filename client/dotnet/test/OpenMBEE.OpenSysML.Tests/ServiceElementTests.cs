using System;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// What a real sysml-grpc answers of an element and of an object, over every client wire: a symbol's members with
/// their provenance and the standard-library ones on request, a feature's derived types and reference subsetting, an
/// element's metamodel properties by specification name walked through the ids they answer, how an instantiated
/// object was reached and every path to it, and, against a service withholding these capabilities, neither members nor
/// how an object was reached, and the reads that need them refused.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceElementTests(ServiceFixture services) : IClassFixture<ServiceFixture>
{
    private const string Withheld = "symbol_members,element_properties,instance_paths";

    // One port object held under several paths: the tank's fuelOut, the interface end supplier referencing it, and
    // the reference pump to the tank.
    private const string Fuel = """
        package Fuel {
            port def FuelPort { out item fuel; }
            interface def FuelLine { end supplier : FuelPort; end consumer : ~FuelPort; }
            part def Tank { port fuelOut : FuelPort; }
            part def Engine { port fuelIn : ~FuelPort; }
            part def Powertrain { part tank : Tank; part engine : Engine; }
            part def Wheel;
            part vehicle {
                part powertrain : Powertrain;
                part wheels : Wheel[2];
                ref part pump : Tank = powertrain.tank;
                interface fuelLine : FuelLine connect supplier ::> powertrain.tank.fuelOut to consumer ::> powertrain.engine.fuelIn;
            }
        }
        """;

    private const string Typed = """
        package Typed {
            part def T;
            part def V;
            part def Holder {
                part q : T;
                part pBoth : V :> q;
                ref part r ::> q;
            }
        }
        """;

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task MembersAreListedWithTheirProvenanceAndLibraryOnesOnRequest(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, File.ReadAllText(Fixture("members.sysml")));

        var m = await model.SymbolAsync("Demo::m", cancellationToken: Token);
        var root = await model.SymbolAsync("Demo::Base", cancellationToken: Token);
        var full = await model.SymbolAsync("Demo::m", new SymbolOptions { IncludeLibraryMembers = true }, Token);

        Assert.Empty(m.ChildIds);
        Assert.Empty(m.OwnMembers);
        Assert.Equal(new SymbolMember("Demo::Base::p", "p", "partUsage", true, "Demo::Base", false), m.InheritedMembers[0]);
        Assert.All(m.Members, member => Assert.False(member.IsLibrary));
        Assert.True(m.WithheldLibraryMembers > 0);
        Assert.Equal([new SymbolMember("Demo::Base::p", "p", "partUsage", false, string.Empty, false)], root.OwnMembers);
        Assert.Equal(["Demo::Base::p"], root.ChildIds);
        Assert.Equal(0, full.WithheldLibraryMembers);
        Assert.Equal(m.Members, full.Members.Take(m.Members.Count));
        var library = full.Members.Where(member => member.IsLibrary).ToList();
        Assert.Equal(m.WithheldLibraryMembers, library.Count);
        Assert.All(library, member => Assert.True(member.IsInherited && member.InheritedFrom.Length > 0, member.ToString()));
        Assert.Equal(full, await model.FindSymbolAsync("Demo::m", new SymbolOptions { IncludeLibraryMembers = true }, Token));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AFeaturesDerivedTypesAndReferenceSubsettingAreReported(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, Typed);

        var both = await model.SymbolAsync("Typed::Holder::pBoth", cancellationToken: Token);
        var referencing = await model.SymbolAsync("Typed::Holder::r", cancellationToken: Token);
        var definition = await model.SymbolAsync("Typed::V", cancellationToken: Token);

        Assert.Equal(("V", "Typed::V"), (both.TypeFacts!.Declared, both.TypeFacts.ResolvedId));
        Assert.Equal([new TypeRef("Typed::V", "partDef"), new TypeRef("Typed::T", "partDef")], both.TypeFacts.Types);
        Assert.Equal((string.Empty, string.Empty), (referencing.TypeFacts!.Declared, referencing.TypeFacts.ResolvedId));
        Assert.Equal([new TypeRef("Typed::T", "partDef")], referencing.TypeFacts.Types);
        Assert.Contains(new Specialization("references", "q", "Typed::Holder::q", "partUsage"), referencing.Specializations);
        Assert.Empty(definition.TypeFacts?.Types ?? []);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ElementPropertiesAreReadBySpecificationNameAndWalkedByTheirIds(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, File.ReadAllText(Fixture("element_properties.sysml")));

        var pair = await model.ElementPropertiesAsync("Demo::Pair", ["inheritedFeature", "isAbstract", "connectorEnd"], Token);
        var connector = await model.ElementPropertiesAsync("Demo::Pair::@2", ["relatedFeature", "connectorEnd"], Token);
        var related = connector.Find("relatedFeature")!.Elements;
        var walked = await model.ElementPropertiesAsync(related[0].Id, ["name"], Token);
        var every = await model.ElementPropertiesAsync("Demo::CmdPort::cmd", cancellationToken: Token);
        var missing = await Assert.ThrowsAsync<SymbolNotFoundException>(() => model.ElementPropertiesAsync("Demo::Nope", ["name"], Token));
        var unheld = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("no-such-model").ElementPropertiesAsync("Demo", cancellationToken: Token));

        Assert.Equal(("Demo::Pair", "PartDefinition"), (pair.Element.QualifiedName, pair.Element.Metaclass));
        Assert.NotEmpty(pair.Element.Id);
        Assert.Equal(["inheritedFeature", "isAbstract", "connectorEnd"], pair.Properties.Select(property => property.Name));
        var inherited = pair.Find("inheritedFeature")!;
        Assert.Equal(("Type", true, true), (inherited.DefiningClass, inherited.IsMany, inherited.IsDerived));
        Assert.Contains("Demo::Unit::p", inherited.Elements.Select(element => element.QualifiedName));
        Assert.Contains("Parts::Part::ownedPorts", inherited.Elements.Select(element => element.QualifiedName));
        Assert.Equal(new ElementPropertyValue.BooleanValue(false), pair.Find("isAbstract")!.Value);
        Assert.False(pair.Find("connectorEnd")!.IsSupported);
        Assert.Contains("declares no property connectorEnd", pair.Find("connectorEnd")!.Unsupported, StringComparison.Ordinal);
        Assert.Equal((string.Empty, "ConnectionUsage"), (connector.Element.QualifiedName, connector.Element.Metaclass));
        Assert.Equal([("Demo::Pair::u1", "PartUsage"), ("Demo::Pair::u2", "PartUsage")], related.Select(element => (element.QualifiedName, element.Metaclass)));
        Assert.Equal(2, connector.Find("connectorEnd")!.Values.Count);
        Assert.Equal(related[0], walked.Element);
        Assert.Equal(new ElementPropertyValue.TextValue("u1"), walked.Find("name")!.Value);
        Assert.True(every.Properties.Count >= 10);
        Assert.Contains("direction", every.Properties.Select(property => property.Name));
        Assert.Contains("type", every.Properties.Select(property => property.Name));
        Assert.Equal(("Demo::Nope", Model.GetElementPropertiesMethod), (missing.Name, missing.Method));
        Assert.Contains("element not found", missing.Message, StringComparison.Ordinal);
        Assert.Equal(StatusCode.NotFound, unheld.StatusCode);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnObjectSaysHowItWasReachedAndEveryPathToItIsListed(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var model = await LoadAsync(connection, Fuel);

        var vehicle = await model.InstantiateAsync("Fuel::vehicle", Token);
        var byPath = vehicle.Reachable.Where(instance => instance.ReachedBy is not null).ToDictionary(instance => instance.Path, StringComparer.Ordinal);
        var supplier = byPath["fuelLine.supplier"];
        var paths = await model.InstancePathsAsync(supplier.Id, Token);
        var throughPump = await model.InstancePathsAtAsync("Fuel::vehicle.pump.fuelOut", Token);
        var fromRoot = await model.InstancePathsAsync(vehicle.Root.Id, Token);
        var wheel = await model.InstancePathsAtAsync("Fuel::vehicle.wheels[2]", Token);
        var absent = await Assert.ThrowsAsync<ServiceException>(() => model.InstancePathsAsync(999_999, Token));
        var unreadable = await Assert.ThrowsAsync<ServiceException>(() => model.InstancePathsAtAsync("Fuel::vehicle.", Token));
        var unheld = await Assert.ThrowsAsync<ModelNotFoundException>(() => connection.ModelByHash("no-such-model").InstancePathsAsync(1, Token));

        Assert.Null(vehicle.Root.ReachedBy);
        Assert.Equal(
            ["fuelLine", "fuelLine.consumer", "fuelLine.consumer.fuel", "fuelLine.supplier", "fuelLine.supplier.fuel", "powertrain", "powertrain.engine", "powertrain.tank", "wheels[1]", "wheels[2]"],
            byPath.Keys.Order(StringComparer.Ordinal));
        Assert.Equal(new InstanceReach(byPath["fuelLine"].Id, "supplier", "Fuel::FuelLine::supplier", 0, "fuelLine.supplier"), supplier.ReachedBy);
        Assert.Same(byPath["fuelLine"], vehicle.FindOwner(supplier));
        Assert.Equal((vehicle.Root.Id, "wheels", 2), (byPath["wheels[2]"].ReachedBy!.InstanceId, byPath["wheels[2]"].ReachedBy!.FeatureName, byPath["wheels[2]"].ReachedBy!.Index));
        Assert.Equal(
            new Value.InstanceRefValue(supplier.Id),
            byPath["powertrain.tank"].FeatureValues["fuelOut"].Value);

        Assert.False(paths.IsTruncated);
        Assert.Equal(["Fuel::vehicle.fuelLine.supplier", "Fuel::vehicle.powertrain.tank.fuelOut", "Fuel::vehicle.pump.fuelOut"], paths.Paths.Select(path => path.ToString()));
        Assert.All(paths.Paths, path => Assert.Equal((vehicle.Root.Id, "Fuel::vehicle"), (path.RootInstanceId, path.RootSymbolId)));
        Assert.Equal(
            [
                new InstancePathStep("powertrain", "Fuel::vehicle::powertrain", 0, byPath["powertrain"].Id),
                new InstancePathStep("tank", "Fuel::Powertrain::tank", 0, byPath["powertrain.tank"].Id),
                new InstancePathStep("fuelOut", "Fuel::Tank::fuelOut", 0, supplier.Id),
            ],
            paths.Paths[1].Steps);
        Assert.Equal(paths, throughPump);
        foreach (var path in paths.Paths)
        {
            Assert.Equal(paths, await model.InstancePathsAtAsync(path.ToString(), Token));
        }

        Assert.Equal([new InstancePath(vehicle.Root.Id, "Fuel::vehicle", string.Empty, [])], fromRoot.Paths);
        Assert.Equal(2, Assert.Single(Assert.Single(wheel.Paths).Steps).Index);
        Assert.Equal(typeof(ServiceException), absent.GetType());
        Assert.Equal(StatusCode.NotFound, absent.StatusCode);
        Assert.Equal(StatusCode.InvalidArgument, unreadable.StatusCode);
        Assert.Equal(Model.InstancePathsMethod, unheld.Method);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AServiceWithoutTheCapabilitiesReportsNeitherMembersNorReachAndTheReadsAreRefused(ClientWire wire)
    {
        var service = await services.ServiceAsync("connect", Withheld);
        await using var connection = await Connection.OpenAsync(wire.To(service.Address), Token);
        var model = await LoadAsync(connection, Fuel);

        var symbol = await model.SymbolAsync("Fuel::vehicle", cancellationToken: Token);
        var built = await model.InstantiateAsync("Fuel::vehicle", Token);
        var library = await Assert.ThrowsAsync<CapabilityException>(() => model.SymbolAsync("Fuel::vehicle", new SymbolOptions { IncludeLibraryMembers = true }, Token));
        var properties = await Assert.ThrowsAsync<CapabilityException>(() => model.ElementPropertiesAsync("Fuel::vehicle", cancellationToken: Token));
        var paths = await Assert.ThrowsAsync<CapabilityException>(() => model.InstancePathsAsync(built.Root.Id, Token));

        Assert.Empty(symbol.Members);
        Assert.Equal(0, symbol.WithheldLibraryMembers);
        Assert.All(built.Reachable, instance => Assert.Null(instance.ReachedBy));
        Assert.Equal(
            [Capabilities.SymbolMembers, Capabilities.ElementProperties, Capabilities.InstancePaths],
            new[] { library, properties, paths }.Select(error => error.Capability));
    }

    private static string Fixture(string name) => Repository.PathOf("conformance", "fixtures", name);

    private static async Task<Model> LoadAsync(Connection connection, string content) =>
        (await connection.LoadContentAsync(content, cancellationToken: Token)).RequireNoErrors();

    private async Task<Connection> OpenAsync(ClientWire wire)
    {
        var service = await services.ServiceAsync("connect");
        return await Connection.OpenAsync(wire.To(service.Address), Token);
    }
}
