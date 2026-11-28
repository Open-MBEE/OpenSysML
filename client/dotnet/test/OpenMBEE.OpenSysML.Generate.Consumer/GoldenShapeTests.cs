#pragma warning disable CS1591 // Test classes are public for xUnit; the generated goldens beside them are held to documentation.

using System;
using System.Linq;
using System.Reflection;
using OpenMBEE.OpenSysML.Generate.Tests;
using OpenMBEE.OpenSysML.Typed;
using Xunit;
using CaseModel = OpenSysML.Golden.Cases.Cases;
using CaseParts = OpenSysML.Golden.Cases.CaseParts;

namespace OpenMBEE.OpenSysML.Generate.Consumer;

/// <summary>
/// The committed goldens, as compiled here against the library alone, have the shape the generator promises,
/// and read hand-built instances as it promises: no service is needed.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class GoldenShapeTests
{
    private static readonly Type[] Generated = [.. typeof(GoldenShapeTests).Assembly.GetTypes().Where(type => type.Namespace?.StartsWith("OpenSysML.Golden.", StringComparison.Ordinal) == true)];

    [Fact]
    public void AClassInheritsItsFirstBaseAndImplementsEveryBaseInterface()
    {
        Assert.Equal(typeof(TypedObject), typeof(CaseModel.Vehicle).BaseType);
        Assert.Equal(typeof(CaseModel.Vehicle), typeof(CaseModel.Car).BaseType);
        Assert.Equal(typeof(CaseModel.Car), typeof(CaseModel.RegisteredCar).BaseType);
        Assert.True(typeof(CaseModel.IRegistered).IsAssignableFrom(typeof(CaseModel.RegisteredCar)));
        Assert.Equal(typeof(CaseModel.Leased), typeof(CaseModel.Pooled).BaseType);
        Assert.True(typeof(CaseModel.IOwned).IsAssignableFrom(typeof(CaseModel.Pooled)));
        Assert.Equal([typeof(CaseModel.ILeased), typeof(CaseModel.IOwned)], typeof(CaseModel.IPooled).GetInterfaces().Where(type => type.Namespace == typeof(CaseModel.IPooled).Namespace && type != typeof(CaseModel.IAsset)).OrderBy(type => type.Name, StringComparer.Ordinal));
    }

    [Fact]
    public void InterfacesOnlyReadAndClassesExposeNothingButTheirView()
    {
        var interfaces = Generated.Where(type => type.IsInterface).ToList();
        var classes = Generated.Where(type => type.IsClass && type.IsSubclassOf(typeof(TypedObject))).ToList();

        Assert.NotEmpty(interfaces);
        Assert.Equal(interfaces.Count, classes.Count);
        Assert.All(interfaces, type => Assert.All(type.GetProperties(), property => Assert.False(property.CanWrite, $"{type.Name}.{property.Name}")));
        Assert.All(classes, type => Assert.Equal(
            ["Instance", "Instantiation"],
            type.GetProperties(BindingFlags.Public | BindingFlags.Instance).Select(property => property.Name).Order(StringComparer.Ordinal)));
        Assert.All(classes, type => Assert.Empty(type.GetConstructors()));
    }

    [Fact]
    public void EachRegistryHoldsEveryGeneratedClassOfItsFile()
    {
        foreach (var (registry, root) in new[]
        {
            (global::OpenSysML.Golden.Vehicle.SysmlTypes.Registry, "OpenSysML.Golden.Vehicle"),
            (global::OpenSysML.Golden.Fleet.SysmlTypes.Registry, "OpenSysML.Golden.Fleet"),
            (global::OpenSysML.Golden.Cases.SysmlTypes.Registry, "OpenSysML.Golden.Cases"),
        })
        {
            var ids = Generated
                .Where(type => type.IsClass && type.IsSubclassOf(typeof(TypedObject)) && type.Namespace!.StartsWith(root + ".", StringComparison.Ordinal))
                .Select(type => (string)type.GetField("SysmlId")!.GetRawConstantValue()!)
                .Order(StringComparer.Ordinal);
            Assert.Equal(ids, registry.DefinitionIds.Order(StringComparer.Ordinal));
        }

        Assert.Equal("2", global::OpenSysML.Golden.Cases.SysmlTypes.GeneratorVersion);
        Assert.StartsWith("sha256:", global::OpenSysML.Golden.Cases.SysmlTypes.ModelHash, StringComparison.Ordinal);
    }

    [Fact]
    public void AnInheritedPropertyReadsTheFeatureThatRedefinesItUnderAnotherName()
    {
        var engine = new Instance(2, "CaseParts::BigEngine", [new("power", Single(new Value.RealValue(500))), new("cylinders", Single(new Value.IntegerValue(4)))]);
        var truck = new Instance(1, "Cases::Truck", [new("motor", Single(new Value.InstanceRefValue(2))), new("payload", Single(new Value.RealValue(10000)))]);
        var instantiation = new Instantiation(truck, [truck, engine], []);

        var view = CaseModel.Truck.FromInstance(instantiation);
        CaseModel.IVehicle asVehicle = view;

        Assert.Equal(500.0, view.Motor.Power);
        Assert.IsType<CaseParts.BigEngine>(asVehicle.Engine);
        Assert.Equal(view.Motor, asVehicle.Engine);
        Assert.Equal(10000.0, view.Payload);
    }

    [Fact]
    public void AViewIsOfTheMostSpecificGeneratedClassAndAnotherTypeIsRefused()
    {
        var truck = new Instance(1, "Cases::truck", []);

        Assert.IsType<CaseModel.Truck>(CaseModel.Vehicle.FromInstance(truck));
        Assert.IsType<CaseModel.Car>(CaseModel.Vehicle.FromInstance(new Instance(2, "Cases::RegisteredCar", [])), exactMatch: false);
        Assert.IsType<CaseModel.Vehicle>(CaseModel.Vehicle.FromInstance(new Instance(3, "Cases::someVehicle", [])));
        var refused = Assert.Throws<InstanceTypeException>(() => CaseParts.Engine.FromInstance(truck));
        Assert.Equal(("CaseParts::Engine", "Cases::truck"), (refused.Expected, refused.Actual));
        Assert.IsType<CaseParts.Engine>(CaseParts.Engine.Unchecked(truck));
    }

    private static FeatureValue Single(Value value) => new("f", value, [], true, string.Empty, 0);
}
