using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Text.RegularExpressions;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Holds the values to the schema, the inventory and the service: one sealed arm class and one arm
/// test per kind, every arm the schema declares read as its inventory kind and sent under its gate,
/// the gate predicates and their order read from the service's own source, and no derivation from
/// outside the library.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed partial class ValueInventoryTests
{
    private static readonly ParityInventory Committed = ParityInventory.LoadCommitted();

    private static readonly IReadOnlyDictionary<string, Wire.Value> Samples = new Dictionary<string, Wire.Value>(StringComparer.Ordinal)
    {
        ["int_value"] = new() { IntValue = 1 },
        ["real_value"] = new() { RealValue = 1.5 },
        ["bool_value"] = new() { BoolValue = true },
        ["string_value"] = new() { StringValue = "s" },
        ["instance_id"] = new() { InstanceId = 2 },
        ["sequence"] = new() { Sequence = new Wire.ValueSequence { Elements = { new Wire.Value { IntValue = 1 } } } },
        ["null"] = new() { Null = string.Empty },
        ["quantity"] = new() { Quantity = ValueArmTests.WireMetres(2) },
        ["enum_literal"] = new() { EnumLiteral = new Wire.EnumLiteral { LiteralId = "D::C::x", EnumerationId = "D::C", Name = "C::x" } },
        ["unset"] = new() { Unset = true },
        ["complex"] = new() { Complex = new Wire.Complex { Real = 1, Imaginary = 2 } },
        ["array"] = new() { Array = new Wire.Array { Dimensions = { 1 }, Elements = { new Wire.Value { BoolValue = false } } } },
        ["vector"] = new() { Vector = new Wire.Vector { Components = { new Wire.Value { RealValue = 3 } } } },
        ["vector_quantity"] = new() { VectorQuantity = new Wire.VectorQuantity { Components = { ValueArmTests.WireMetres(1) } } },
        ["measurement_ref"] = new() { MeasurementRef = new Wire.MeasurementRef { Unit = "m", UnitTerm = ValueArmTests.WireMetres(1).UnitTerm, UnitId = "SI::metre" } },
        ["infinity"] = new() { Infinity = true },
        ["function"] = new() { Function = new Wire.Function { CalcId = "F::Sq" } },
        ["set"] = new() { Set = new Wire.ValueSet { Elements = { new Wire.Value { StringValue = "m" } } } },
        ["tensor_quantity"] = new() { TensorQuantity = new Wire.TensorQuantity { Dimensions = { 1 }, Components = { ValueArmTests.WireMetres(1) } } },
        ["metaobject"] = new() { Metaobject = new Wire.Metaobject { ElementId = "V::seatBelt", MetaclassId = "SysML::Systems::PartUsage" } },
        ["undetermined"] = new() { Undetermined = new Wire.Undetermined { Reason = "open", Count = new Wire.MultiplicityInfo { Lower = "1", Upper = "*" } } },
        ["big_int_value"] = new() { BigIntValue = "-9223372036854775809" },
        ["rational_value"] = new() { RationalValue = new Wire.Rational { Numerator = "2", Denominator = "3" } },
    };

    [Fact]
    public void EveryKindHasOneSealedArmClassOneArmTestAndOneImplementedInventoryArm()
    {
        var kinds = Enum.GetNames<ValueKind>();
        var nested = typeof(Value).GetNestedTypes();
        var arms = nested.Where(type => !type.IsAbstract).ToList();
        var armTests = typeof(ValueArmTests).GetMethods(BindingFlags.Public | BindingFlags.Instance | BindingFlags.DeclaredOnly)
            .Where(method => method.IsDefined(typeof(FactAttribute)))
            .Select(method => method.Name);

        Assert.Equal(kinds.Select(kind => kind + "Value").Order(StringComparer.Ordinal), arms.Select(type => type.Name).Order(StringComparer.Ordinal));
        Assert.All(arms, type => Assert.True(type.IsSealed && type.IsSubclassOf(typeof(Value)), type.Name));
        Assert.Equal([typeof(Value.NumberValue)], nested.Where(type => type.IsAbstract));
        Assert.Equal(
            ["BigIntegerValue", "IntegerValue", "RationalValue", "RealValue"],
            arms.Where(type => type.IsSubclassOf(typeof(Value.NumberValue))).Select(type => type.Name).Order(StringComparer.Ordinal));
        Assert.Equal(kinds.Select(kind => kind + "Arm").Order(StringComparer.Ordinal), armTests.Order(StringComparer.Ordinal));
        Assert.Equal(kinds.Order(StringComparer.Ordinal), Committed.Values.Arms.Values.Select(arm => arm.Kind).Order(StringComparer.Ordinal));
        Assert.All(Committed.Values.Arms.Values, arm => Assert.Equal(("implemented", $"ValueArmTests.{arm.Kind}Arm"), (arm.Status, arm.Test)));
    }

    [Fact]
    public void OnlyTheLibraryCanCallTheConstructorsAnArmDerivesFrom()
    {
        const BindingFlags Constructors = BindingFlags.Instance | BindingFlags.Public | BindingFlags.NonPublic;

        Assert.All(typeof(Value).GetConstructors(Constructors), constructor => Assert.True(constructor.IsPrivate));
        Assert.All(typeof(Value.NumberValue).GetConstructors(Constructors), constructor => Assert.True(constructor.IsFamilyAndAssembly));
    }

    [Fact]
    public void EveryArmTheSchemaDeclaresIsReadAsItsInventoryKindAndSentUnderItsGate()
    {
        var oneof = Wire.Value.Descriptor.Oneofs.Single(oneof => oneof.Name == "kind");

        Assert.Equal(oneof.Fields.Select(field => field.Name).Order(StringComparer.Ordinal), Samples.Keys.Order(StringComparer.Ordinal));
        foreach (var field in oneof.Fields)
        {
            var arm = Committed.Values.Arms[field.Name];
            var sample = Samples[field.Name];
            Assert.Equal(field.FieldNumber, (int)sample.KindCase);

            var read = ValueWire.FromWire(sample, ValueArmTests.Method, field.Name);

            Assert.Equal(arm.Kind, read.Kind.ToString());
            Assert.Equal($"{arm.Kind}Value", read.GetType().Name);
            if (arm.Send == "response-only")
            {
                Assert.Throws<InvalidRequestException>(() => ValueWire.ToWire(read, field.Name));
                continue;
            }

            var sent = ValueWire.ToWire(read, field.Name);
            Assert.Equal(sample, sent);
            if (arm.Send == "ungated")
            {
                Assert.Empty(ValueWire.RequiredCapabilities(sent));
                ValueWire.Fit(sent, Capabilities.None);
            }
            else
            {
                Assert.Equal([arm.Send], ValueWire.RequiredCapabilities(sent));
                Assert.Equal(arm.Send, Assert.Throws<CapabilityException>(() => ValueWire.Fit(sent, Capabilities.None)).Capability);
                ValueWire.Fit(sent, new Capabilities([arm.Send]));
            }
        }
    }

    [Fact]
    public void EveryFieldOfTheValueMessagesIsTyped()
    {
        var messages = ParityChecker.Reachable([Wire.Value.Descriptor]).Select(message => message.FullName).Order(StringComparer.Ordinal).ToList();

        Assert.Equal(18, messages.Count);
        Assert.All(messages, message => Assert.All(
            Committed.Fields[message],
            field => Assert.True(field.Value.Status == "typed" && field.Value.Test is not null, $"{message}.{field.Key} is {field.Value.Status}")));
    }

    [Fact]
    public void TheSendGatesAreTheServicesPredicatesInTheServicesOrder()
    {
        var gate = GoFunction(Repository.PathOf("internal", "frontend", "grpc", "service.go"), "func (s *Service) requireValueCapabilities(");
        var constants = Directory.GetFiles(Repository.PathOf("internal", "frontend", "grpc"), "*.go")
            .Where(path => !path.EndsWith("_test.go", StringComparison.Ordinal))
            .SelectMany(path => GoCapability().Matches(File.ReadAllText(path)))
            .ToDictionary(match => match.Groups[1].Value, match => match.Groups[2].Value, StringComparer.Ordinal);
        var checks = GoGate().Matches(gate).Select(match => (Predicate: match.Groups[1].Value, Capability: constants[match.Groups[2].Value])).ToList();

        Assert.Equal(ValueWire.SendGateOrder, checks.Select(check => check.Capability));
        Assert.Equal(Committed.Values.SendGateOrder, ValueWire.SendGateOrder);
        Assert.Equal(
            ["Complex", "Structured", "MeasurementRef", "Function", "Infinity", "Set", "Tensor", "Metaobject", "BigInt", "Rational"],
            checks.Select(check => check.Predicate));
    }

    [Fact]
    public void TheServiceNestsAndHoldsValuesWhereThisClientLooksForThem()
    {
        var convert = Repository.PathOf("internal", "frontend", "protoconv", "convert.go");

        Assert.Equal(["Sequence", "Set", "Array", "Vector"], Cases(GoFunction(convert, "func NestedValues(")));
        Assert.Equal(["BigIntValue", "Vector", "Quantity", "VectorQuantity", "TensorQuantity", "EnumLiteral"], Cases(GoFunction(convert, "func ValueHoldsBigInt(")));
        Assert.Equal(["RationalValue", "Vector", "Quantity", "VectorQuantity", "TensorQuantity", "EnumLiteral"], Cases(GoFunction(convert, "func ValueHoldsRational(")));
        Assert.Equal(["RationalValue", "Quantity", "VectorQuantity", "TensorQuantity", "EnumLiteral"], Cases(GoFunction(convert, "func RationalsAsReals(")));
        Assert.Equal(["Array", "Vector", "VectorQuantity"], GoKinds().Matches(GoFunction(convert, "func ValueCarriesStructured(")).Select(match => match.Groups[1].Value));
    }

    [Fact]
    public void ACompilationOutsideTheLibraryCannotDeriveFromAValue()
    {
        const string Members = """
            public override ValueKind Kind => ValueKind.Null;
            public override bool Equals(Value? other) => false;
            public override int GetHashCode() => 0;
            public override string ToString() => string.Empty;
            """;

        Assert.Empty(Errors("public static class Use { public static Value Make() => new Value.SequenceValue([new Value.IntegerValue(1)]); }"));
        var outsideValue = $"public sealed class Outside : Value {{ {Members} }}";
        var outsideNumber = $"public sealed class Outside : Value.NumberValue {{ {Members} public override double ToDouble() => 0; }}";
        Assert.Equal(["CS1729"], Errors(outsideValue));
        Assert.Equal(["CS0122"], Errors(outsideValue, MetadataImportOptions.All));
        Assert.Equal(["CS0122"], Errors(outsideNumber));
        Assert.Equal(["CS0122"], Errors(outsideNumber, MetadataImportOptions.All));

        Assert.Contains("CS8864", Errors("public sealed record Outside : Value;"));
        foreach (var arm in typeof(Value).GetNestedTypes().Where(type => type.IsSealed))
        {
            Assert.Contains("CS0509", Errors($"public sealed class Outside : Value.{arm.Name} {{ }}"));
        }
    }

    // A consumer imports only what the library makes visible; importing all of it shows why a constructor is unusable.
    internal static List<string> Errors(string source, MetadataImportOptions imported = MetadataImportOptions.Public)
    {
        var library = typeof(Value).Assembly.Location;
        var references = ((string)AppContext.GetData("TRUSTED_PLATFORM_ASSEMBLIES")!)
            .Split(Path.PathSeparator)
            .Where(path => !string.Equals(Path.GetFileName(path), Path.GetFileName(library), StringComparison.Ordinal))
            .Append(library)
            .Select(path => MetadataReference.CreateFromFile(path));
        var compilation = CSharpCompilation.Create(
            "Consumer",
            [CSharpSyntaxTree.ParseText($"#nullable enable\nusing OpenMBEE.OpenSysML;\n{source}", new CSharpParseOptions(LanguageVersion.Latest))],
            references,
            new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary).WithMetadataImportOptions(imported));

        return compilation.GetDiagnostics()
            .Where(diagnostic => diagnostic.Severity == Microsoft.CodeAnalysis.DiagnosticSeverity.Error)
            .Select(diagnostic => diagnostic.Id)
            .Distinct()
            .ToList();
    }

    private static string GoFunction(string path, string signature)
    {
        var source = File.ReadAllText(path);
        var start = source.IndexOf(signature, StringComparison.Ordinal);
        Assert.True(start >= 0, $"{path} has no {signature}");
        var end = source.IndexOf("\n}\n", start, StringComparison.Ordinal);
        return source[start..end];
    }

    private static List<string> Cases(string body) => GoCase().Matches(body).Select(match => match.Groups[1].Value).ToList();

    [GeneratedRegex("""^const Capability(\w+) = "([a-z_]+)"$""", RegexOptions.Multiline)]
    private static partial Regex GoCapability();

    [GeneratedRegex("""protoconv\.ValueCarries(\w+)\(pv\)\s*\{\s*(?:if err := )?(?:return )?s\.requireCapability\(Capability(\w+)\)""")]
    private static partial Regex GoGate();

    [GeneratedRegex("""^\tcase \*pb\.Value_(\w+):""", RegexOptions.Multiline)]
    private static partial Regex GoCase();

    [GeneratedRegex("""\*pb\.Value_(\w+)""")]
    private static partial Regex GoKinds();
}
