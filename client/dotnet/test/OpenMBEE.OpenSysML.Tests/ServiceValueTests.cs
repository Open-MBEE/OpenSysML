using System.Collections.Generic;
using System.Numerics;
using System.Threading.Tasks;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// Values against a real sysml-grpc over every client wire: each kind a request can carry is sent to
/// a calc that returns its argument and read back equal, and the kinds only the service sends are read
/// from its answers. The service is the one judge of what this client writes.
/// </summary>
[Trait(Category.Name, Category.Integration)]
public sealed class ServiceValueTests(ServiceFixture services) : IClassFixture<ServiceFixture>
{
    private const string Model = """
        package Echo {
            private import ScalarValues::*;
            calc def Same { in x; return r[1] = x; }
            calc same : Same;
            calc def Many { in x[0..*]; return r[0..*] = x; }
            calc many : Many;
            attribute s : Collections::Set { :>> elements = (3, 1, 2, 2, 3); }
            calc def Sq { in v : Real; return : Real = v * v; }
            enum def Color { red; green; }
            part seatBelt;
            part def Q {
                attribute d : Real;
                attribute k : Real = 2.0;
            }
        }
        """;

    private static readonly UnitTerm Metre = new(1, 1, [new UnitFactor("SI::metre", 1)]);

    public static TheoryData<ClientWire> Wires => ClientWires.All;

    public static IEnumerable<Value> Echoed =>
    [
        new Value.IntegerValue(7),
        new Value.IntegerValue(long.MinValue),
        new Value.BigIntegerValue(BigInteger.One << 70),
        new Value.BigIntegerValue(-(BigInteger.One << 64)),
        new Value.RealValue(2.5),
        new Value.RationalValue(new Rational(1, 3)),
        new Value.RationalValue(new Rational(-22, 7)),
        new Value.ComplexValue(1.5, -2.0),
        new Value.BooleanValue(true),
        new Value.TextValue("h\u00e9llo \"x\" \\"),
        new Value.InfinityValue(),
        new Value.SequenceValue([new Value.IntegerValue(1), new Value.TextValue("a"), new Value.RealValue(0.5)]),
        new Value.ArrayValue([2, 2], [new Value.IntegerValue(1), new Value.IntegerValue(2), new Value.IntegerValue(3), new Value.IntegerValue(4)]),
        new Value.VectorValue([new Value.IntegerValue(3), new Value.RealValue(4.0)]),
        new Value.QuantityValue(new Quantity(new Value.RealValue(3.5), "m", Metre)),
        new Value.VectorQuantityValue([new Quantity(new Value.RealValue(3.0), "m", Metre), new Quantity(new Value.RealValue(4.0), "m", Metre)]),
        new Value.TensorQuantityValue([2], [new Quantity(new Value.RealValue(1.0), "m", Metre), new Quantity(new Value.RealValue(2.0), "m", Metre)]),
        new Value.MeasurementRefValue("m", Metre, "SI::metre"),
        new Value.FunctionValue("Echo::Sq"),
        new Value.MetaobjectValue("Echo::seatBelt", "SysML::Systems::PartUsage"),
        new Value.EnumLiteralValue("Echo::Color::red", "Echo::Color", "Color::red"),
    ];

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task EveryKindARequestCarriesComesBackEqualFromTheService(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var hash = await ParseAsync(connection);

        foreach (var value in Echoed)
        {
            var sent = ValueWire.ToWire(value, "arguments[0]");
            ValueWire.Fit(sent, connection.Capabilities);
            var calc = value is Value.SequenceValue ? "Echo::many" : "Echo::same";
            var answer = await connection.CallAsync<Wire.EvaluateCalcRequest, Wire.EvaluateCalcResponse>(
                "EvaluateCalc", new Wire.EvaluateCalcRequest { ModelHash = hash, SymbolId = calc, Arguments = { sent } }, TestContext.Current.CancellationToken);

            Assert.True(answer.Error.Length == 0, $"{value}: {answer.Error}");
            Assert.Equal(value, ValueWire.FromWire(answer.Result, "sysml.SysMLService/EvaluateCalc", "result"));
        }
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheKindsOnlyTheServiceSendsAreReadFromItsAnswers(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var hash = await ParseAsync(connection);

        var open = await connection.CallAsync<Wire.EvaluateRequest, Wire.EvaluateResponse>(
            "Evaluate", new Wire.EvaluateRequest { ModelHash = hash, Expression = "d + 1.0", ContextSymbolId = "Echo::Q" }, TestContext.Current.CancellationToken);
        var literal = await connection.CallAsync<Wire.EvaluateRequest, Wire.EvaluateResponse>(
            "Evaluate", new Wire.EvaluateRequest { ModelHash = hash, Expression = "Echo::Color::green" }, TestContext.Current.CancellationToken);
        var instance = await connection.CallAsync<Wire.InstantiateRequest, Wire.InstantiateResponse>(
            "Instantiate", new Wire.InstantiateRequest { ModelHash = hash, SymbolId = "Echo::Q" }, TestContext.Current.CancellationToken);

        var undetermined = Assert.IsType<Value.UndeterminedValue>(ValueWire.FromWire(open.Result, "sysml.SysMLService/Evaluate", "result"));
        Assert.Equal(("1", "1"), (undetermined.CountLower, undetermined.CountUpper));
        Assert.Contains("d has no value", undetermined.Reason, System.StringComparison.Ordinal);
        Assert.Equal(new Value.EnumLiteralValue("Echo::Color::green", "Echo::Color", "Color::green"), ValueWire.FromWire(literal.Result, "sysml.SysMLService/Evaluate", "result"));
        Assert.Equal("Echo::Color", ((Value.EnumLiteralValue)ValueWire.FromWire(literal.Result, "sysml.SysMLService/Evaluate", "result")).EnumerationId);
        Assert.Equal(new Value.UnsetValue(), ValueWire.FromWire(instance.Instance.FeatureValues["d"].Value, "sysml.SysMLService/Instantiate", "instance.feature_values[\"d\"].value"));
        Assert.Equal(new Value.RealValue(2.0), ValueWire.FromWire(instance.Instance.FeatureValues["k"].Value, "sysml.SysMLService/Instantiate", "instance.feature_values[\"k\"].value"));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task ASetIsReadFromTheServiceAndReachesItAsItsMembers(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var hash = await ParseAsync(connection);
        var members = new Value.SetValue([new Value.IntegerValue(3), new Value.IntegerValue(1), new Value.IntegerValue(2)]);
        var sent = ValueWire.ToWire(members, "arguments[0]");
        ValueWire.Fit(sent, connection.Capabilities);

        var read = await connection.CallAsync<Wire.EvaluateRequest, Wire.EvaluateResponse>(
            "Evaluate", new Wire.EvaluateRequest { ModelHash = hash, Expression = "Echo::s.elements" }, TestContext.Current.CancellationToken);
        var echoed = await connection.CallAsync<Wire.EvaluateCalcRequest, Wire.EvaluateCalcResponse>(
            "EvaluateCalc", new Wire.EvaluateCalcRequest { ModelHash = hash, SymbolId = "Echo::many", Arguments = { sent } }, TestContext.Current.CancellationToken);

        Assert.Equal(members, ValueWire.FromWire(read.Result, "sysml.SysMLService/Evaluate", "result"));
        Assert.True(echoed.Error.Length == 0, echoed.Error);
        var reached = Assert.IsType<Value.SequenceValue>(ValueWire.FromWire(echoed.Result, "sysml.SysMLService/EvaluateCalc", "result"));
        Assert.Equal(members, new Value.SetValue(reached.Elements));
    }

    [Theory]
    [MemberData(nameof(Wires))]
    public async Task TheServiceRefusesWhatThisClientRefusesToSend(ClientWire wire)
    {
        await using var connection = await OpenAsync(wire);
        var hash = await ParseAsync(connection);

        var closing = await connection.CallAsync<Wire.EvaluateCalcRequest, Wire.EvaluateCalcResponse>(
            "EvaluateCalc",
            new Wire.EvaluateCalcRequest { ModelHash = hash, SymbolId = "Echo::same", Arguments = { new Wire.Value { Function = new Wire.Function { CalcId = "Echo::Sq", SelfId = 3 } } } },
            TestContext.Current.CancellationToken);
        var unset = await connection.CallAsync<Wire.EvaluateCalcRequest, Wire.EvaluateCalcResponse>(
            "EvaluateCalc",
            new Wire.EvaluateCalcRequest { ModelHash = hash, SymbolId = "Echo::same", Arguments = { new Wire.Value { Unset = true } } },
            TestContext.Current.CancellationToken);

        Assert.Throws<InvalidRequestException>(() => ValueWire.ToWire(new Value.FunctionValue("Echo::Sq", 3), "arguments[0]"));
        Assert.Throws<InvalidRequestException>(() => ValueWire.ToWire(new Value.UnsetValue(), "arguments[0]"));
        Assert.Contains("self_id 3", closing.Error, System.StringComparison.Ordinal);
        Assert.Null(closing.Result);
        Assert.NotEmpty(unset.Error);
        Assert.Null(unset.Result);
    }

    private async Task<Connection> OpenAsync(ClientWire wire)
    {
        var service = await services.ServiceAsync("connect");
        return await Connection.OpenAsync(wire.To(service.Address), TestContext.Current.CancellationToken);
    }

    private static async Task<string> ParseAsync(Connection connection)
    {
        var parsed = await connection.CallAsync<Wire.ParseFileRequest, Wire.ParseFileResponse>(
            "ParseFile", new Wire.ParseFileRequest { Content = Model }, TestContext.Current.CancellationToken);
        Assert.True(parsed.Error.Length == 0, parsed.Error);
        Assert.DoesNotContain(parsed.Diagnostics, diagnostic => diagnostic.Severity == "error");
        return parsed.ModelHash;
    }
}
