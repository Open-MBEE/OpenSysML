using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.RegularExpressions;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Holds the values to the service: the gate predicates and their order, and where values nest, read from the
/// service's own source.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed partial class ValueInventoryTests
{
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
