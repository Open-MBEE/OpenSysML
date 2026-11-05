using System;
using System.Buffers;
using System.Linq;
using System.Reflection;
using System.Threading.Tasks;
using Google.Protobuf;
using Grpc.Core;
using OpenMBEE.OpenSysML.Internal;
using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

[Trait(Category.Name, Category.Unit)]
public sealed class RpcMethodsTests
{
    [Fact]
    public void EveryMethodOfTheDescriptorIsRegisteredInSchemaOrder()
    {
        var declared = Wire.SysMLService.Descriptor.Methods;

        Assert.Equal("sysml.SysMLService", RpcMethods.ServiceName);
        Assert.Equal(declared.Select(method => method.Name), RpcMethods.All.Select(method => method.Name));
        Assert.All(RpcMethods.All.Zip(declared), pair =>
        {
            var (method, descriptor) = pair;
            Assert.Same(descriptor, method.Descriptor);
            Assert.Equal($"sysml.SysMLService/{descriptor.Name}", method.QualifiedName);
            Assert.Equal(descriptor.InputType.ClrType, method.RequestType);
            Assert.Equal(descriptor.OutputType.ClrType, method.ResponseType);
            Assert.Same(descriptor.InputType.Parser, method.RequestParser);
            Assert.Same(descriptor.OutputType.Parser, method.ResponseParser);
        });
    }

    [Fact]
    public void TheRegistryAgreesWithTheGeneratedClientStub()
    {
        // The generated stub keeps its methods private; the registry must name the same calls.
        var stubMethods = typeof(Wire.SysMLService)
            .GetFields(BindingFlags.NonPublic | BindingFlags.Static)
            .Where(field => field.FieldType.IsGenericType && field.FieldType.GetGenericTypeDefinition() == typeof(Method<,>))
            .Select(field => (IMethod)field.GetValue(null)!)
            .ToDictionary(method => method.Name, StringComparer.Ordinal);

        Assert.Equal(RpcMethods.All.Select(method => method.Name).Order(StringComparer.Ordinal), stubMethods.Keys.Order(StringComparer.Ordinal));
        foreach (var method in RpcMethods.All)
        {
            var stub = stubMethods[method.Name];
            var arguments = stub.GetType().GetGenericArguments();
            Assert.Equal(stub.FullName, $"/{method.QualifiedName}");
            Assert.Equal(MethodType.Unary, stub.Type);
            Assert.Equal([method.RequestType, method.ResponseType], arguments);
        }
    }

    [Theory]
    [InlineData("ParseFile")]
    [InlineData("sysml.SysMLService/ParseFile")]
    public void BareAndQualifiedNamesFindTheMethod(string name)
    {
        var method = RpcMethods.Find(name);

        Assert.NotNull(method);
        Assert.Equal("ParseFile", method.Name);
        Assert.Same(RpcMethods.Find("ParseFile"), method);
    }

    [Theory]
    [InlineData("")]
    [InlineData("parsefile")]
    [InlineData("ParseFile ")]
    [InlineData("/sysml.SysMLService/ParseFile")]
    [InlineData("sysml.SysMLService.ParseFile")]
    [InlineData("SysMLService/ParseFile")]
    [InlineData("other.Service/ParseFile")]
    [InlineData("Forecast")]
    public void OtherSpellingsFindNothing(string name)
    {
        Assert.Null(RpcMethods.Find(name));
        Assert.False(RpcMethods.TryResolve<Wire.ParseFileRequest, Wire.ParseFileResponse>(name, out var method, out var problem));
        Assert.Null(method);
        Assert.Equal(
            $"\"{name}\" is not a method of sysml.SysMLService: name one bare, as ParseFile, or qualified, as sysml.SysMLService/ParseFile.",
            problem);
    }

    [Fact]
    public void ANullNameIsRefused()
    {
        Assert.Throws<ArgumentNullException>(() => RpcMethods.Find(null!));
        Assert.Throws<ArgumentNullException>(() => RpcMethods.TryResolve<Wire.ParseFileRequest, Wire.ParseFileResponse>(null!, out _, out _));
    }

    [Fact]
    public void TheDeclaredPairingResolvesToOneSharedUnaryMethod()
    {
        Assert.True(RpcMethods.TryResolve<Wire.EvaluateRequest, Wire.EvaluateResponse>("Evaluate", out var bare, out var problem));
        Assert.Null(problem);
        Assert.True(RpcMethods.TryResolve<Wire.EvaluateRequest, Wire.EvaluateResponse>("sysml.SysMLService/Evaluate", out var qualified, out _));

        Assert.Same(bare, qualified);
        Assert.Same(RpcMethods.Find("Evaluate"), bare.Untyped);
        Assert.Equal(MethodType.Unary, bare.Grpc.Type);
        Assert.Equal("/sysml.SysMLService/Evaluate", bare.Grpc.FullName);
        Assert.Equal("sysml.SysMLService", bare.Grpc.ServiceName);
        Assert.Equal("Evaluate", bare.Grpc.Name);
        Assert.Same(Wire.EvaluateRequest.Parser, bare.RequestParser);
        Assert.Same(Wire.EvaluateResponse.Parser, bare.ResponseParser);
    }

    [Fact]
    public void EveryMethodResolvesWithItsOwnMessages()
    {
        var resolve = typeof(RpcMethodsTests).GetMethod(nameof(Resolve), BindingFlags.NonPublic | BindingFlags.Static)!;
        foreach (var method in RpcMethods.All)
        {
            var typed = resolve.MakeGenericMethod(method.RequestType, method.ResponseType).Invoke(null, [method.QualifiedName]);

            Assert.NotNull(typed);
        }
    }

    [Fact]
    public void AMismatchedPairingIsRefusedWithBothPairings()
    {
        const string Expected = "sysml.SysMLService/Evaluate takes OpenMBEE.OpenSysML.Wire.EvaluateRequest and returns OpenMBEE.OpenSysML.Wire.EvaluateResponse, ";

        Assert.False(RpcMethods.TryResolve<Wire.ParseFileRequest, Wire.EvaluateResponse>("Evaluate", out var request, out var requestProblem));
        Assert.Null(request);
        Assert.Equal(Expected + "not OpenMBEE.OpenSysML.Wire.ParseFileRequest and OpenMBEE.OpenSysML.Wire.EvaluateResponse.", requestProblem);

        Assert.False(RpcMethods.TryResolve<Wire.EvaluateRequest, Wire.ParseFileResponse>("Evaluate", out _, out var responseProblem));
        Assert.Equal(Expected + "not OpenMBEE.OpenSysML.Wire.EvaluateRequest and OpenMBEE.OpenSysML.Wire.ParseFileResponse.", responseProblem);

        Assert.False(RpcMethods.TryResolve<Wire.EvaluateResponse, Wire.EvaluateRequest>("Evaluate", out _, out var swappedProblem));
        Assert.Equal(Expected + "not OpenMBEE.OpenSysML.Wire.EvaluateResponse and OpenMBEE.OpenSysML.Wire.EvaluateRequest.", swappedProblem);
    }

    [Fact]
    public void ConcurrentResolutionSharesOneTypedMethod()
    {
        var resolved = new RpcMethod<Wire.QueryRequest, Wire.QueryResponse>?[64];
        Parallel.For(0, resolved.Length, i =>
            RpcMethods.TryResolve<Wire.QueryRequest, Wire.QueryResponse>("Query", out resolved[i], out _));

        Assert.All(resolved, method => Assert.Same(resolved[0], method));
        Assert.NotNull(resolved[0]);
    }

    [Fact]
    public void TheMarshallersCarryTheProtobufEncoding()
    {
        Assert.True(RpcMethods.TryResolve<Wire.EvaluateRequest, Wire.EvaluateResponse>("Evaluate", out var method, out _));
        var request = new Wire.EvaluateRequest { ModelHash = "abc", Expression = "1 + 2", SubjectSymbolId = "Demo::s" };
        var response = new Wire.EvaluateResponse { Result = new Wire.Value { BigIntValue = "123456789012345678901234567890" } };

        var requestBytes = Serialize(method.Grpc.RequestMarshaller, request);
        var responseBytes = Serialize(method.Grpc.ResponseMarshaller, response);

        Assert.Equal(request.ToByteArray(), requestBytes);
        Assert.Equal(request, Deserialize(method.Grpc.RequestMarshaller, requestBytes));
        Assert.Equal(response.ToByteArray(), responseBytes);
        Assert.Equal(response, Deserialize(method.Grpc.ResponseMarshaller, responseBytes));
    }

    [Fact]
    public void UnknownFieldsSurviveTheMarshallers()
    {
        Assert.True(RpcMethods.TryResolve<Wire.EvaluateRequest, Wire.EvaluateResponse>("Evaluate", out var method, out _));
        // Field 99, varint 7: a field a newer service might add.
        byte[] newer = [.. new Wire.EvaluateResponse { Error = "e" }.ToByteArray(), 0x98, 0x06, 0x07];

        var read = Deserialize(method.Grpc.ResponseMarshaller, newer);

        Assert.Equal("e", read.Error);
        Assert.Equal(newer, Serialize(method.Grpc.ResponseMarshaller, read));
    }

    // The contextual path is the one a gRPC channel takes.
    private static byte[] Serialize<T>(Marshaller<T> marshaller, T message)
    {
        var context = new Serialization();
        marshaller.ContextualSerializer(message, context);
        Assert.True(context.Completed);
        Assert.Equal(context.Written.WrittenCount, context.DeclaredLength);
        return context.Written.WrittenSpan.ToArray();
    }

    private static T Deserialize<T>(Marshaller<T> marshaller, byte[] payload) =>
        marshaller.ContextualDeserializer(new Deserialization(payload));

    private static RpcMethod<TRequest, TResponse>? Resolve<TRequest, TResponse>(string name)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        Assert.True(RpcMethods.TryResolve<TRequest, TResponse>(name, out var method, out var problem), problem);
        Assert.Equal(typeof(TRequest), method.Untyped.RequestType);
        return method;
    }

    private sealed class Serialization : SerializationContext
    {
        internal ArrayBufferWriter<byte> Written { get; } = new();

        internal int? DeclaredLength { get; private set; }

        internal bool Completed { get; private set; }

        public override void SetPayloadLength(int payloadLength) => DeclaredLength = payloadLength;

        public override IBufferWriter<byte> GetBufferWriter() => Written;

        public override void Complete() => Completed = true;

        public override void Complete(byte[] payload) => throw new InvalidOperationException("The marshaller writes through the buffer writer.");
    }

    private sealed class Deserialization(byte[] payload) : DeserializationContext
    {
        public override int PayloadLength => payload.Length;

        public override byte[] PayloadAsNewBuffer() => throw new InvalidOperationException("The marshaller reads the payload as a sequence.");

        public override ReadOnlySequence<byte> PayloadAsReadOnlySequence() => new(payload);
    }
}
