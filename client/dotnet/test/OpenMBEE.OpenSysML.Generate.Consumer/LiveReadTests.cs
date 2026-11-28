#pragma warning disable CS1591 // Test classes are public for xUnit; the generated goldens beside them are held to documentation.

using System;
using System.IO;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Generate.Tests;
using OpenMBEE.OpenSysML.Tests;
using Xunit;
using CaseModel = OpenSysML.Golden.Cases.Cases;
using CaseParts = OpenSysML.Golden.Cases.CaseParts;
using FleetTop = OpenSysML.Golden.Fleet.FleetTop;
using VehicleDemo = OpenSysML.Golden.Vehicle.Demo;

namespace OpenMBEE.OpenSysML.Generate.Consumer;

/// <summary>
/// The committed goldens read live instances of the models they were generated from, over every wire: each
/// feature as the type its property declares, objects as the most specific generated class, and an inherited
/// property from the feature that redefines it.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class LiveReadTests(LiveServices services) : IClassFixture<LiveServices>
{
    private static readonly string[] FleetDocuments = ["lib.sysml", "top.sysml", "units.kerml"];
    private static readonly string[] CaseDocuments = ["parts.sysml", "vehicles.sysml", "extensions.sysml"];

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    private static CancellationToken Token => TestContext.Current.CancellationToken;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheVehicleGoldenReadsTheModelEveryClientGeneratesFrom(ClientWire wire)
    {
        await using var connection = await services.OpenAsync(wire, Token);
        var model = (await connection.LoadAsync(Repository.PathOf("internal", "frontend", "repl", "testdata", "vehicle_package.sysml"), cancellationToken: Token)).RequireNoErrors();

        var vehicle = VehicleDemo.Vehicle.FromInstance(await model.InstantiateAsync("Demo::Vehicle", Token));

        Assert.IsType<VehicleDemo.Vehicle>(vehicle);
        Assert.Equal(new Rational(1500, 1), vehicle.Mass);
        Assert.Equal(new Rational(300, 1), vehicle.Engine.Power);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheFleetGoldenReadsTypesDeclaredInAnotherDocument(ClientWire wire)
    {
        await using var connection = await services.OpenAsync(wire, Token);
        var directory = Repository.PathOf("client", "python", "tests", "fixtures", "multifile");
        var model = (await connection.ParseSourcesAsync(
            FleetDocuments.Select(name => SourceDocument.FromFile(Path.Combine(directory, name))),
            cancellationToken: Token)).RequireNoErrors();

        var car = FleetTop.Car.FromInstance(await model.InstantiateAsync("FleetTop::car", Token));

        Assert.IsType<FleetTop.Car>(car);
        Assert.Equal(120.0, car.Engine.Power);
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheCasesGoldenReadsEveryScalarMultiplicityAndValueKind(ClientWire wire)
    {
        await using var connection = await services.OpenAsync(wire, Token);
        var model = await CasesAsync(connection);

        var car = CaseModel.Vehicle.FromInstance(await model.InstantiateAsync("Cases::car", Token));

        Assert.IsType<CaseModel.Car>(car);
        Assert.Equal(1500.0, car.Mass);
        Assert.Equal("vehicle", car.Label);
        Assert.Equal(7, car.Serial);
        Assert.Equal(new Rational(1, 4), car.Ratio);
        Assert.False(car.Electric);
        Assert.Null(car.Impedance);
        Assert.Equal("CaseParts::Fuel", car.Fuel.EnumerationId);
        Assert.EndsWith("petrol", car.Fuel.LiteralId, StringComparison.Ordinal);
        Assert.Equal("SI::m", car.Wheelbase.Unit);
        Assert.Equal(new Value.RationalValue(new Rational(27, 10)), car.Wheelbase.Magnitude);
        Assert.Null(car.Size);
        Assert.Empty(car.Tags);
        Assert.Null(car.Nickname);
        Assert.Null(car.Engine);
        Assert.Equal(4, car.Wheels.Count);
        Assert.All(car.Wheels, wheel => Assert.Equal(0.3, wheel.Radius));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheCasesGoldenFollowsRedefinitionsAndEveryBase(ClientWire wire)
    {
        await using var connection = await services.OpenAsync(wire, Token);
        var model = await CasesAsync(connection);

        var truck = CaseModel.Truck.FromInstance(await model.InstantiateAsync("Cases::truck", Token));
        var registered = CaseModel.RegisteredCar.FromInstance(await model.InstantiateAsync("Cases::registered", Token));
        var pool = CaseModel.Pooled.FromInstance(await model.InstantiateAsync("Cases::pool", Token));
        var keyword = CaseModel.@event.@class.FromInstance(await model.InstantiateAsync("Cases::event::class", Token));

        Assert.IsType<CaseParts.BigEngine>(truck.Motor);
        Assert.Equal((500.0, 4), (truck.Motor.Power, (int)truck.Motor.Cylinders));
        Assert.Equal(truck.Motor, ((CaseModel.IVehicle)truck).Engine);
        Assert.Equal(500.0, ((CaseParts.IEngine)truck.Motor).Power);
        Assert.Equal(10000.0, truck.Payload);
        Assert.Equal("UNREGISTERED", registered.Plate);
        Assert.Equal(1500.0, registered.Mass);
        Assert.Equal(2, pool.AssetId);
        Assert.Equal(2, ((CaseModel.IAsset)pool).AssetId);
        Assert.True(pool.Shared);
        Assert.Equal("keyword", keyword.String);
        Assert.Throws<InstanceTypeException>(() => CaseParts.Engine.FromInstance(truck.Instance, truck.Instantiation));
    }

    private static async Task<Model> CasesAsync(Connection connection)
    {
        var directory = Repository.PathOf("client", "dotnet", "test", "testdata", "generate", "cases");
        return (await connection.ParseSourcesAsync(
            CaseDocuments.Select(name => SourceDocument.FromFile(Path.Combine(directory, name))),
            cancellationToken: Token)).RequireNoErrors();
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ReopenedPackagesReadDynamicMultiplicityAndIntersectedRedefinitions(ClientWire wire)
    {
        await using var connection = await services.OpenAsync(wire, Token);
        var model = await CasesAsync(connection);

        var readings = CaseModel.DynamicReadings.FromInstance(await model.InstantiateAsync("Cases::DynamicReadings", Token));
        var joined = CaseModel.JoinedReading.FromInstance(await model.InstantiateAsync("Cases::JoinedReading", Token));

        Assert.Equal([1.0, 2.0, 3.0], readings.Readings);
        Assert.Null(joined.Reading);
        Assert.Null(((CaseModel.INarrowReading)joined).Reading);
        Assert.Empty(((CaseModel.IBroadReading)joined).Reading);
    }
}
