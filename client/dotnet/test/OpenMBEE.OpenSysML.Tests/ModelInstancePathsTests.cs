using System;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// How an instantiated object was reached and every path to it, through the typed API: <c>reached_by</c> read on
/// every object and resolved to its holder, InstancePaths asked by id and by path with every fact of a path read, an
/// object the population lacks told apart from a model the service does not hold, and what no service would accept
/// refused before sending.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class ModelInstancePathsTests
{
    public static TheoryData<ClientWire> Wires => ClientWires.All;

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AnInstantiatedObjectSaysHowItWasReached(ClientWire wire)
    {
        var reach = new Wire.InstanceReach { InstanceId = 1, FeatureName = "wheels", FeatureSymbolId = "Fuel::vehicle::wheels", Index = 2, Path = "wheels[2]" };
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .On("Instantiate", Wire.InstantiateRequest.Parser, _ => new Wire.InstantiateResponse
            {
                Instance = new Wire.Instance { Id = 1, TypeSymbolId = "Fuel::vehicle" },
                Instances =
                {
                    new Wire.Instance { Id = 1, TypeSymbolId = "Fuel::vehicle" },
                    new Wire.Instance { Id = 3, TypeSymbolId = "Fuel::Wheel", ReachedBy = reach },
                    new Wire.Instance { Id = 4, TypeSymbolId = "Fuel::Hub", ReachedBy = new Wire.InstanceReach { InstanceId = 7, FeatureName = "hub", Path = "elsewhere.hub" } },
                },
            });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var built = await model.InstantiateAsync("Fuel::vehicle", TestContext.Current.CancellationToken);

        var wheel = built.FindInstance(3)!;
        var hub = built.FindInstance(4)!;
        Assert.Null(built.Root.ReachedBy);
        Assert.Equal(string.Empty, built.Root.Path);
        Assert.Null(built.FindOwner(built.Root));
        Assert.Equal(new InstanceReach(1, "wheels", "Fuel::vehicle::wheels", 2, "wheels[2]"), wheel.ReachedBy);
        Assert.Equal("wheels[2]", wheel.Path);
        Assert.Equal("wheels[2]", wheel.ReachedBy!.ToString());
        Assert.Same(built.Root, built.FindOwner(wheel));
        Assert.Null(built.FindOwner(hub));
        Assert.NotEqual(wheel, new Instance(wheel.Id, wheel.TypeSymbolId, wheel.FeatureValues));
        Assert.Throws<ArgumentNullException>(() => built.FindOwner(null!));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ThePathsToAnObjectAreAskedByIdOrByPathAndReadWhole(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .On("InstancePaths", Wire.InstancePathsRequest.Parser, _ => CompleteWire.InstancePaths);
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();

        var byId = await model.InstancePathsAsync(6, TestContext.Current.CancellationToken);
        var byPath = await model.InstancePathsAtAsync("Fuel::vehicle.wheels[2].hub", TestContext.Current.CancellationToken);

        Assert.Equal(
            [
                new Wire.InstancePathsRequest { ModelHash = "h", Object = new Wire.DocumentObject { InstanceId = 6 } },
                new Wire.InstancePathsRequest { ModelHash = "h", Object = new Wire.DocumentObject { Path = "Fuel::vehicle.wheels[2].hub" } },
            ],
            handler.RequestsFor("InstancePaths").Select(request => request.Message(Wire.InstancePathsRequest.Parser)));
        Assert.Equal(
            new InstancePaths(
                [
                    new InstancePath(
                        1,
                        "Fuel::vehicle",
                        "wheels[2].hub",
                        [new InstancePathStep("wheels", "Fuel::vehicle::wheels", 2, 4), new InstancePathStep("hub", "Fuel::Wheel::hub", 0, 6)]),
                    new InstancePath(9, string.Empty, string.Empty, []),
                ],
                true),
            byId);
        Assert.Equal(byId, byPath);
        Assert.True(byId.IsTruncated);
        Assert.Equal(["Fuel::vehicle.wheels[2].hub", "#9"], byId.Paths.Select(path => path.ToString()));
        Assert.Equal(["wheels[2] -> #4", "hub -> #6"], byId.Paths[0].Steps.Select(step => step.ToString()));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task AFailureIsReportedWithItsDiagnosticsAndAnUnknownObjectIsNotTheModel(ClientWire wire)
    {
        using var handler = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .On("InstancePaths", Wire.InstancePathsRequest.Parser, request => request.Object.InstanceId switch
            {
                1 => new Wire.InstancePathsResponse { Error = "the population cannot be walked", Diagnostics = { FakeModels.Diagnostic("error", "cyclic") } },
                2 => new Wire.InstancePathsResponse { Paths = { new Wire.InstancePath { Path = "rootless" } } },
                _ => throw new InvalidOperationException("unexpected request"),
            });
        await using var connection = await FakeConnection.OpenAsync(handler);
        var model = await connection.LoadAsync();
        using var refusing = new FakeServiceHandler(wire) { Capabilities = FakeModels.ModelCapabilities }
            .Parsing("h", FakeModels.Symbol("r", string.Empty, "RootNamespace"))
            .Refusing("InstancePaths", StatusCode.NotFound, "binding object: no object #99");
        await using var refusingConnection = await FakeConnection.OpenAsync(refusing);
        var refusingModel = await refusingConnection.LoadAsync();

        var failed = await Assert.ThrowsAsync<ModelException>(() => model.InstancePathsAsync(1, TestContext.Current.CancellationToken));
        var rootless = await Assert.ThrowsAsync<TransportException>(() => model.InstancePathsAsync(2, TestContext.Current.CancellationToken));
        var absent = await Assert.ThrowsAsync<ServiceException>(() => refusingModel.InstancePathsAsync(99, TestContext.Current.CancellationToken));

        Assert.Equal(("the population cannot be walked", Model.InstancePathsMethod), (failed.Message, failed.Method));
        Assert.Equal(["cyclic"], failed.Diagnostics.Select(diagnostic => diagnostic.Message));
        Assert.Equal(StatusCode.Internal, rootless.StatusCode);
        Assert.Contains("paths[0]", rootless.Message, StringComparison.Ordinal);
        Assert.Equal(typeof(ServiceException), absent.GetType());
        Assert.Equal((StatusCode.NotFound, "binding object: no object #99"), (absent.StatusCode, absent.ServiceMessage));
    }

    [Fact]
    public async Task WhatNoServiceWouldAnswerIsRefusedBeforeAnythingIsSent()
    {
        using var handler = new FakeServiceHandler() { Capabilities = FakeModels.ModelCapabilities };
        await using var connection = await FakeConnection.OpenAsync(handler);
        using var older = new FakeServiceHandler() { Capabilities = [.. FakeModels.ModelCapabilities.Where(name => name != Capabilities.InstancePaths)] };
        await using var olderConnection = await FakeConnection.OpenAsync(older);
        var model = connection.ModelByHash("h");

        await Assert.ThrowsAsync<ArgumentOutOfRangeException>(() => model.InstancePathsAsync(0, TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentOutOfRangeException>(() => model.InstancePathsAsync(-1, TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentNullException>(() => model.InstancePathsAtAsync(null!, TestContext.Current.CancellationToken));
        await Assert.ThrowsAsync<ArgumentException>(() => model.InstancePathsAtAsync(string.Empty, TestContext.Current.CancellationToken));
        var byId = await Assert.ThrowsAsync<CapabilityException>(() => olderConnection.ModelByHash("h").InstancePathsAsync(1, TestContext.Current.CancellationToken));
        var byPath = await Assert.ThrowsAsync<CapabilityException>(() => olderConnection.ModelByHash("h").InstancePathsAtAsync("#1", TestContext.Current.CancellationToken));

        Assert.All([byId, byPath], error => Assert.Equal(Capabilities.InstancePaths, error.Capability));
        Assert.Equal(["GetServerInfo"], handler.Requests.Select(request => request.Method));
        Assert.Equal(["GetServerInfo"], older.Requests.Select(request => request.Method));
    }

    [Fact]
    public void APathIsAddressedFromItsRootsLabelOrElseItsId()
    {
        Assert.Equal("Garage::car", new InstancePath(2, "Garage::car", string.Empty, []).ToString());
        Assert.Equal("#2.wheels[1]", new InstancePath(2, string.Empty, "wheels[1]", [new InstancePathStep("wheels", "Garage::Car::wheels", 1, 5)]).ToString());
        Assert.Equal("engine -> #3", new InstancePathStep("engine", "Garage::Car::engine", 0, 3).ToString());
        Assert.Throws<ArgumentNullException>(() => new InstancePath(1, null!, string.Empty, []));
        Assert.Throws<ArgumentException>(() => new InstancePaths([null!], false));
        Assert.Throws<ArgumentNullException>(() => new InstanceReach(1, "f", "F::f", 0, null!));
    }
}
