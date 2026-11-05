using System.Linq;
using System.Net;
using System.Text;
using System.Threading.Tasks;
using Google.Protobuf;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The protobuf JSON mapping as <see cref="MessageEncoding.Json"/> writes and reads it, including
/// what it cannot carry from a newer service.
/// </summary>
[Trait(Category.Name, Category.Unit)]
public sealed class JsonBodiesTests
{
    [Fact]
    public void A64BitIntegerIsWrittenAsAStringAndReadFromEitherSpelling()
    {
        var written = Write(new Wire.Value { IntValue = long.MinValue });

        Assert.Equal("""{ "intValue": "-9223372036854775808" }""", written);
        Assert.Equal(long.MaxValue, Read<Wire.Value>("""{"intValue":"9223372036854775807"}""").IntValue);
        Assert.Equal(10, Read<Wire.Value>("""{"intValue":10}""").IntValue);
        Assert.Throws<InvalidProtocolBufferException>(() => Read<Wire.Value>("""{"intValue":"9223372036854775808"}"""));
        Assert.Equal("""{ "seed": "18446744073709551615" }""", Write(new Wire.ExecuteActionRequest { Seed = ulong.MaxValue }));
    }

    [Fact]
    public void BytesAreStandardBase64()
    {
        var request = new Wire.MigrateRequest { Content = ByteString.CopyFrom(0, 1, 2, 0xFB, 0xFF) };

        Assert.Equal("""{ "content": "AAEC+/8=" }""", Write(request));
        Assert.Equal(request, Read<Wire.MigrateRequest>("""{"content":"AAEC+/8="}"""));
    }

    [Fact]
    public void AOneofArmIsWrittenAtItsDefaultAndTwoArmsAreRefused()
    {
        Assert.Equal("""{ "intValue": "0" }""", Write(new Wire.Value { IntValue = 0 }));
        Assert.Equal("""{ "boolValue": false }""", Write(new Wire.Value { BoolValue = false }));
        Assert.Equal("""{ "stringValue": "" }""", Write(new Wire.Value { StringValue = string.Empty }));
        Assert.Equal(Wire.Value.KindOneofCase.IntValue, Read<Wire.Value>("""{"intValue":"0"}""").KindCase);
        Assert.Throws<InvalidProtocolBufferException>(() => Read<Wire.Value>("""{"intValue":"1","realValue":2}"""));
    }

    [Fact]
    public void AFieldAtItsDefaultIsLeftOutAndAPresentOptionalIsKept()
    {
        Assert.Equal("{ }", Write(new Wire.ServerInfoResponse { Version = string.Empty }));
        Assert.Equal("""{ "advance": 0 }""", Write(new Wire.ExecuteActionRequest { Advance = 0 }));
        Assert.True(Read<Wire.ExecuteActionRequest>("""{"advance":0}""").HasAdvance);
        Assert.False(Read<Wire.ExecuteActionRequest>("{}").HasAdvance);
    }

    [Fact]
    public void NamesAreLowerCamelCaseAndTheProtoNameIsReadToo()
    {
        Assert.Equal("""{ "modelHash": "m", "symbolId": "A" }""", Write(new Wire.GetSymbolRequest { ModelHash = "m", SymbolId = "A" }));
        Assert.Equal("m", Read<Wire.GetSymbolRequest>("""{"model_hash":"m"}""").ModelHash);
    }

    [Fact]
    public void AnEnumIsWrittenByNameAndAnUnknownNumberIsKept()
    {
        Assert.Equal("""{ "failureReason": "FAILURE_REASON_WRONG_KIND" }""", Write(new Wire.Verdict { FailureReason = Wire.FailureReason.WrongKind }));
        Assert.Equal(Wire.FailureReason.WrongKind, Read<Wire.Verdict>("""{"failureReason":"FAILURE_REASON_WRONG_KIND"}""").FailureReason);
        Assert.Equal(99, (int)Read<Wire.Verdict>("""{"failureReason":99}""").FailureReason);
        Assert.Equal("""{ "failureReason": 99 }""", Write(new Wire.Verdict { FailureReason = (Wire.FailureReason)99 }));
    }

    [Fact]
    public void AnUnknownFieldIsIgnoredAndAnUnknownEnumNameReadsAsZero()
    {
        // The limitation the encoding documents: a name this client has no number for cannot be kept.
        var verdict = Read<Wire.Verdict>("""{"holds":true,"failureReason":"FAILURE_REASON_FROM_A_NEWER_SERVICE","fromANewerService":{"a":[1]}}""");

        Assert.True(verdict.Holds);
        Assert.Equal(Wire.FailureReason.Unspecified, verdict.FailureReason);
        Assert.Equal(Wire.Value.KindOneofCase.None, Read<Wire.Value>("""{"aNewerArm":{}}""").KindCase);
    }

    [Fact]
    public void RealsKeepTheirSignAndTheirNonFiniteValues()
    {
        Assert.True(double.IsNegative(Read<Wire.Value>("""{"realValue":-0}""").RealValue));
        Assert.True(double.IsNegative(Read<Wire.Value>("""{"realValue":-0.0}""").RealValue));
        Assert.True(double.IsNaN(Read<Wire.Value>("""{"realValue":"NaN"}""").RealValue));
        Assert.Equal(double.PositiveInfinity, Read<Wire.Value>("""{"realValue":"Infinity"}""").RealValue);
        Assert.Equal(double.NegativeInfinity, Read<Wire.Value>("""{"realValue":"-Infinity"}""").RealValue);
        Assert.Equal("""{ "realValue": -0 }""", Write(new Wire.Value { RealValue = -0.0 }));
        Assert.Equal("""{ "realValue": "NaN" }""", Write(new Wire.Value { RealValue = double.NaN }));
    }

    [Theory]
    [InlineData("")]
    [InlineData("[]")]
    [InlineData("null")]
    [InlineData("{\"version\":\"v1\"} trailing")]
    [InlineData("{\"version\":\"v1\"}{}")]
    [InlineData("{\"version\":1}")]
    public void ABodyThatIsNotTheMessageIsRefused(string body)
    {
        var error = Assert.ThrowsAny<System.IO.IOException>(() => Read<Wire.ServerInfoResponse>(body));

        Assert.True(error is InvalidProtocolBufferException or InvalidJsonException, error.GetType().Name);
    }

    [Fact]
    public void ABodyThatIsNotUtf8IsRefusedRatherThanRepaired()
    {
        byte[] body = [.. "{\"version\":\"v"u8, 0xFF, .. "\"}"u8];

        Assert.Throws<DecoderFallbackException>(() => JsonBodies.Parse(body, Wire.ServerInfoResponse.Descriptor));
    }

    [Fact]
    public async Task AConnectionOverJsonReadsWhatItWasSent()
    {
        var answer = new Wire.SymbolResponse
        {
            Symbol = new Wire.SymbolInfo { Id = "Vehicle::mass", Name = "mass", Metadata = { ["multiplicity"] = "1" }, ChildIds = { "a", "b" } },
            Error = "\u00e9\u4e2d\U0001F600",
        };
        using var handler = new FakeServiceHandler(ClientWire.ConnectJson) { Respond = (_, _) => Task.FromResult(FakeServiceHandler.AnswerOn(ClientWire.ConnectJson, answer)) };
        await using var connection = await FakeConnection.OpenAsync(handler);

        var read = await connection.GetSymbolAsync();

        Assert.Equal(answer, read);
        Assert.Equal(MessageEncoding.Json, connection.Encoding);
        Assert.Equal("application/json", handler.RequestsFor("GetSymbol").Single().ContentType);
        Assert.Equal(HttpStatusCode.OK, ConnectResponses.Answer(answer, MessageEncoding.Json).StatusCode);
    }

    private static string Write(IMessage message) => Encoding.UTF8.GetString(JsonBodies.Serialize(message));

    private static T Read<T>(string json)
        where T : IMessage<T>, new() =>
        (T)JsonBodies.Parse(Encoding.UTF8.GetBytes(json), new T().Descriptor);
}
