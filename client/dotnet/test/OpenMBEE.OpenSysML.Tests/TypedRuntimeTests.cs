using System;
using System.Collections.Generic;
using System.Linq;
using System.Numerics;
using OpenMBEE.OpenSysML.Typed;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The runtime generated typed classes read through: the decoders, the accessors per multiplicity, and the
/// registry that resolves a view to the most specific generated class, over hand-built instances.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class TypedRuntimeTests
{
    private const string VehicleId = "Demo::Vehicle";
    private const string CarId = "Demo::Car";
    private const string EngineId = "Demo::Engine";

    private static readonly TypedRegistry Registry = new(
        [
            new(VehicleId, static (instance, instantiation) => new Vehicle(instance, instantiation)),
            new(CarId, static (instance, instantiation) => new Car(instance, instantiation)),
            new(EngineId, static (instance, instantiation) => new Engine(instance, instantiation)),
        ],
        [new("Demo::sedan", CarId)]);

    private interface IVehicle : ITypedObject;

    private interface ICar : IVehicle;

    private interface IEngine : ITypedObject;

    [Fact]
    public void ScalarDecodersAcceptTheirArmsAndWidenWithoutLoss()
    {
        Assert.True(FeatureDecoders.AsBoolean("b", new Value.BooleanValue(true), null));
        Assert.Equal("text", FeatureDecoders.AsString("s", new Value.TextValue("text"), null));
        Assert.Equal(new BigInteger(7), FeatureDecoders.AsInteger("i", new Value.IntegerValue(7), null));
        Assert.Equal(BigInteger.Pow(10, 30), FeatureDecoders.AsInteger("i", new Value.BigIntegerValue(BigInteger.Pow(10, 30)), null));
        Assert.Equal(new Rational(1, 3), FeatureDecoders.AsRational("q", new Value.RationalValue(new Rational(1, 3)), null));
        Assert.Equal(new Rational(5, 1), FeatureDecoders.AsRational("q", new Value.IntegerValue(5), null));
        Assert.Equal(new Rational(1, 4), FeatureDecoders.AsRational("q", new Value.RealValue(0.25), null));
        Assert.Equal(2.5, FeatureDecoders.AsReal("r", new Value.RealValue(2.5), null));
        Assert.Equal(3.0, FeatureDecoders.AsReal("r", new Value.IntegerValue(3), null));
        Assert.Equal(0.5, FeatureDecoders.AsReal("r", new Value.RationalValue(new Rational(1, 2)), null));
        Assert.Equal(new Complex(1, 2), FeatureDecoders.AsComplex("c", new Value.ComplexValue(1, 2), null));
        Assert.Equal(new Complex(4, 0), FeatureDecoders.AsComplex("c", new Value.IntegerValue(4), null));
        var literal = new Value.EnumLiteralValue("D::Color::red", "D::Color", "red");
        Assert.Same(literal, FeatureDecoders.AsEnumLiteral("e", literal, null));
        var unset = new Value.UnsetValue();
        Assert.Same(unset, FeatureDecoders.AsValue("v", unset, null));
    }

    [Fact]
    public void ADecoderReportsAValueOfAnotherTypeNamingTheFeature()
    {
        var mismatch = Assert.Throws<TypeMismatchException>(() => FeatureDecoders.AsReal("mass", new Value.TextValue("heavy"), null));

        Assert.Equal(("mass", "double"), (mismatch.FeatureName, mismatch.Expected));
        Assert.Equal(new Value.TextValue("heavy"), mismatch.Actual);
        Assert.Equal("feature value 'mass': expected double, got \"heavy\"", mismatch.Message);
        Assert.Throws<TypeMismatchException>(() => FeatureDecoders.AsBoolean("b", new Value.IntegerValue(1), null));
        Assert.Throws<TypeMismatchException>(() => FeatureDecoders.AsInteger("i", new Value.RealValue(1.0), null));
        Assert.Throws<TypeMismatchException>(() => FeatureDecoders.AsRational("q", new Value.RealValue(double.NaN), null));
        Assert.Throws<TypeMismatchException>(() => FeatureDecoders.AsReal("r", new Value.UnsetValue(), null));
    }

    [Fact]
    public void ARequiredFeatureReadsExactlyOneValue()
    {
        var vehicle = View(("mass", Single(new Value.RealValue(1500))), ("many", Many(new Value.RealValue(1), new Value.RealValue(2))), ("none", Single(new Value.NullValue())));

        Assert.Equal(1500.0, TypedFeatures.Required(vehicle, "mass", FeatureDecoders.AsReal));
        Assert.Equal("a value", Assert.Throws<TypeMismatchException>(() => TypedFeatures.Required(vehicle, "absent", FeatureDecoders.AsReal)).Expected);
        Assert.Equal("a single value", Assert.Throws<TypeMismatchException>(() => TypedFeatures.Required(vehicle, "many", FeatureDecoders.AsReal)).Expected);
        Assert.Equal("a value", Assert.Throws<TypeMismatchException>(() => TypedFeatures.Required(vehicle, "none", FeatureDecoders.AsReal)).Expected);
    }

    [Fact]
    public void ARequiredFallbackValueRejectsNullAndUnsetBeforeDecoding()
    {
        var unset = new Value.UnsetValue();
        var none = new Value.NullValue();
        var present = new Value.TextValue("present");
        var vehicle = View(("unset", Single(unset)), ("none", Single(none)), ("present", Single(present)));

        foreach (var name in new[] { "unset", "none" })
        {
            var mismatch = Assert.Throws<TypeMismatchException>(() => TypedFeatures.Required(vehicle, name, FeatureDecoders.AsValue));
            Assert.Equal((name, "a value"), (mismatch.FeatureName, mismatch.Expected));
            Assert.Same(name == "unset" ? (Value)unset : none, mismatch.Actual);
        }

        Assert.Same(present, TypedFeatures.Required(vehicle, "present", FeatureDecoders.AsValue));
    }

    [Fact]
    public void AnOptionalFeatureReadsNullForNoValue()
    {
        var vehicle = View(
            ("one", Single(new Value.RealValue(1))),
            ("unset", Single(new Value.UnsetValue())),
            ("null", Single(new Value.NullValue())),
            ("empty", Many()),
            ("oneOfMany", Many(new Value.TextValue("x"))),
            ("two", Many(new Value.RealValue(1), new Value.RealValue(2))));

        Assert.Equal(1.0, TypedFeatures.OptionalValue(vehicle, "one", FeatureDecoders.AsReal));
        Assert.Null(TypedFeatures.OptionalValue(vehicle, "unset", FeatureDecoders.AsReal));
        Assert.Null(TypedFeatures.OptionalValue(vehicle, "null", FeatureDecoders.AsReal));
        Assert.Null(TypedFeatures.OptionalValue(vehicle, "empty", FeatureDecoders.AsReal));
        Assert.Null(TypedFeatures.OptionalValue(vehicle, "absent", FeatureDecoders.AsReal));
        Assert.Equal("x", TypedFeatures.Optional(vehicle, "oneOfMany", FeatureDecoders.AsString));
        Assert.Equal("at most one value", Assert.Throws<TypeMismatchException>(() => TypedFeatures.OptionalValue(vehicle, "two", FeatureDecoders.AsReal)).Expected);
    }

    [Fact]
    public void AListFeatureReadsEveryValueAndASequenceAsSeveral()
    {
        var vehicle = View(
            ("many", Many(new Value.IntegerValue(1), new Value.IntegerValue(2))),
            ("sequence", Single(new Value.SequenceValue([new Value.IntegerValue(3), new Value.IntegerValue(4)]))),
            ("one", Single(new Value.IntegerValue(5))),
            ("unset", Single(new Value.UnsetValue())));

        Assert.Equal([1, 2], TypedFeatures.List(vehicle, "many", FeatureDecoders.AsInteger));
        Assert.Equal([3, 4], TypedFeatures.List(vehicle, "sequence", FeatureDecoders.AsInteger));
        Assert.Equal([5], TypedFeatures.List(vehicle, "one", FeatureDecoders.AsInteger));
        Assert.Empty(TypedFeatures.List(vehicle, "unset", FeatureDecoders.AsInteger));
        Assert.Empty(TypedFeatures.List(vehicle, "absent", FeatureDecoders.AsInteger));
    }

    [Fact]
    public void AFeatureThatFailedOrWasNeverMaterializedRaisesFromEveryAccessor()
    {
        var vehicle = View(
            ("failed", new FeatureValue("failed", null, [], true, "cyclic dependency", 0)),
            ("unmaterialized", new FeatureValue("unmaterialized", null, [], false, string.Empty, 0)),
            ("unsupported", Single(new Value.NullValue("a value arm this client does not know"))));

        var failed = Assert.Throws<FeatureValueException>(() => TypedFeatures.Required(vehicle, "failed", FeatureDecoders.AsReal));
        Assert.Equal(("failed", "cyclic dependency"), (failed.FeatureName, failed.Reason));
        Assert.Equal("feature value 'failed': cyclic dependency", failed.Message);
        Assert.Throws<FeatureValueException>(() => TypedFeatures.List(vehicle, "unmaterialized", FeatureDecoders.AsReal));
        Assert.Throws<FeatureValueException>(() => TypedFeatures.OptionalValue(vehicle, "unsupported", FeatureDecoders.AsReal));
    }

    [Fact]
    public void TheRegistryViewsAnInstanceAsItsMostSpecificGeneratedClass()
    {
        var car = Registry.FromInstance<IVehicle>(new Instance(1, CarId, []), null, VehicleId);
        var sedan = Registry.FromInstance<IVehicle>(new Instance(2, "Demo::sedan", []), null, VehicleId);
        var unknown = Registry.FromInstance<IVehicle>(new Instance(3, "Demo::myVehicle", []), null, VehicleId);

        Assert.IsType<Car>(car);
        Assert.IsType<Car>(sedan);
        Assert.IsType<Vehicle>(unknown);
        Assert.Equal(CarId, Registry.DefinitionOf("Demo::sedan"));
        Assert.Null(Registry.DefinitionOf("Demo::myVehicle"));
        Assert.Equal([CarId, EngineId, VehicleId], [.. Registry.DefinitionIds.Order(StringComparer.Ordinal)]);
    }

    [Fact]
    public void TheRegistryRefusesAnInstanceAnotherGeneratedClassDescribes()
    {
        var error = Assert.Throws<InstanceTypeException>(() => Registry.FromInstance<IEngine>(new Instance(1, CarId, []), null, EngineId));

        Assert.Equal((EngineId, CarId), (error.Expected, error.Actual));
        Assert.Contains("Unchecked(instance)", error.Message, StringComparison.Ordinal);
        Assert.Throws<InstanceTypeException>(() => Registry.FromInstance<ICar>(new Instance(1, VehicleId, []), null, CarId));
    }

    [Fact]
    public void AnObjectFeatureResolvesThroughTheInstantiation()
    {
        var engine = new Instance(2, EngineId, []);
        var root = new Instance(1, VehicleId, [new("engine", Single(new Value.InstanceRefValue(2))), new("ghost", Single(new Value.InstanceRefValue(9)))]);
        var instantiation = new Instantiation(root, [root, engine], []);
        var vehicle = Registry.FromInstance<IVehicle>(root, instantiation, VehicleId);

        var decoded = TypedFeatures.Required(vehicle, "engine", Registry.Decoder<IEngine>(EngineId));
        Assert.IsType<Engine>(decoded);
        Assert.Same(engine, decoded.Instance);
        Assert.Same(instantiation, decoded.Instantiation);
        Assert.Throws<TypeMismatchException>(() => TypedFeatures.Required(vehicle, "ghost", Registry.Decoder<IEngine>(EngineId)));
        Assert.Throws<TypeMismatchException>(() => TypedFeatures.Required(Registry.FromInstance<IVehicle>(root, null, VehicleId), "engine", Registry.Decoder<IEngine>(EngineId)));
    }

    [Fact]
    public void TheRegistryRefusesAnInconsistentTable()
    {
        TypedFactory factory = static (instance, instantiation) => new Vehicle(instance, instantiation);

        Assert.Throws<ArgumentException>(() => new TypedRegistry([new(VehicleId, factory), new(VehicleId, factory)], []));
        Assert.Throws<ArgumentException>(() => new TypedRegistry([new(VehicleId, factory)], [new("Demo::car", CarId)]));
        Assert.Throws<ArgumentException>(() => new TypedRegistry([new(VehicleId, factory)], [new(VehicleId, VehicleId)]));
        Assert.Throws<ArgumentException>(() => Registry.FromInstance<IVehicle>(new Instance(1, VehicleId, []), null, "Demo::Unknown"));
        Assert.Throws<ArgumentException>(() => Registry.Decoder<IVehicle>("Demo::Unknown"));
        Assert.Throws<ArgumentException>(() => Registry.FromInstance<IEngine>(new Instance(1, VehicleId, []), null, VehicleId));
    }

    [Fact]
    public void ViewsAreEqualOverEqualInstancesOfOneClass()
    {
        var instance = new Instance(1, VehicleId, [new("mass", Single(new Value.RealValue(1)))]);
        var same = new Instance(1, VehicleId, [new("mass", Single(new Value.RealValue(1)))]);

        Assert.Equal<TypedObject>(new Vehicle(instance, null), new Vehicle(same, null));
        Assert.Equal(new Vehicle(instance, null).GetHashCode(), new Vehicle(same, null).GetHashCode());
        Assert.NotEqual<TypedObject>(new Vehicle(instance, null), new Car(instance, null));
        Assert.NotEqual<TypedObject>(new Vehicle(instance, null), new Vehicle(new Instance(2, VehicleId, []), null));
        Assert.Equal("Vehicle(Demo::Vehicle#1)", new Vehicle(instance, null).ToString());
        Assert.Throws<ArgumentNullException>(() => new Vehicle(null!, null));
    }

    private static Vehicle View(params (string Name, FeatureValue Value)[] features) =>
        new(new Instance(1, VehicleId, features.Select(feature => new KeyValuePair<string, FeatureValue>(feature.Name, feature.Value)).ToArray()), null);

    private static FeatureValue Single(Value value) => new("f", value, [], true, string.Empty, 0);

    private static FeatureValue Many(params Value[] values) => new("f", null, values, true, string.Empty, 0);

    private class Vehicle(Instance instance, Instantiation? instantiation) : TypedObject(instance, instantiation), IVehicle;

    private sealed class Car(Instance instance, Instantiation? instantiation) : Vehicle(instance, instantiation), ICar;

    private sealed class Engine(Instance instance, Instantiation? instantiation) : TypedObject(instance, instantiation), IEngine;
}
