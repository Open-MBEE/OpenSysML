using System;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using System.Threading;
using Google.Protobuf;
using Google.Protobuf.Reflection;
using Grpc.Core;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// The service's methods, read from the descriptor of the generated protocol code rather than
/// listed by hand, so the registry cannot drift from the schema the client was built against.
/// Both transports dispatch through it.
/// </summary>
internal static class RpcMethods
{
    private static readonly FrozenDictionary<string, RpcMethod> ByName = Index(Descriptor.Methods);

    /// <summary>The service's descriptor: the one sysml.proto declares.</summary>
    internal static ServiceDescriptor Descriptor => Wire.SysMLService.Descriptor;

    /// <summary>The service's full name, sysml.SysMLService.</summary>
    internal static string ServiceName => Descriptor.FullName;

    /// <summary>Every method, in the order the schema declares them.</summary>
    internal static IReadOnlyList<RpcMethod> All { get; } = [.. Descriptor.Methods.Select(method => ByName[method.Name])];

    /// <summary>
    /// The method a name spells: bare (<c>ParseFile</c>) or qualified by the service
    /// (<c>sysml.SysMLService/ParseFile</c>), matched exactly.
    /// </summary>
    /// <param name="name">The method's name.</param>
    /// <returns>The method, or null when the service has none of that name.</returns>
    internal static RpcMethod? Find(string name)
    {
        ArgumentNullException.ThrowIfNull(name);
        return ByName.GetValueOrDefault(name);
    }

    /// <summary>
    /// The method a name spells, typed by the request and response messages the caller pairs
    /// with it. Nothing is sent: a pairing the schema does not declare is the caller's to refuse.
    /// </summary>
    /// <typeparam name="TRequest">The request message the caller sends.</typeparam>
    /// <typeparam name="TResponse">The response message the caller expects.</typeparam>
    /// <param name="name">The method's name, as <see cref="Find"/> reads it.</param>
    /// <param name="method">The typed method, when the name and the pairing are the schema's.</param>
    /// <param name="problem">Why the name or the pairing is not, otherwise.</param>
    /// <returns>Whether the method was resolved.</returns>
    internal static bool TryResolve<TRequest, TResponse>(
        string name,
        [NotNullWhen(true)] out RpcMethod<TRequest, TResponse>? method,
        [NotNullWhen(false)] out string? problem)
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        var untyped = Find(name);
        if (untyped is null)
        {
            method = null;
            problem = $"\"{name}\" is not a method of {ServiceName}: name one bare, as ParseFile, or qualified, as {ServiceName}/ParseFile.";
            return false;
        }

        method = untyped.As<TRequest, TResponse>();
        if (method is null)
        {
            problem = $"{untyped.QualifiedName} takes {untyped.RequestType.FullName} and returns {untyped.ResponseType.FullName}, "
                + $"not {typeof(TRequest).FullName} and {typeof(TResponse).FullName}.";
            return false;
        }

        problem = null;
        return true;
    }

    private static FrozenDictionary<string, RpcMethod> Index(IEnumerable<MethodDescriptor> methods)
    {
        var byName = new Dictionary<string, RpcMethod>(StringComparer.Ordinal);
        foreach (var descriptor in methods)
        {
            var method = new RpcMethod(descriptor);
            byName.Add(method.Name, method);
            byName.Add(method.QualifiedName, method);
        }

        return byName.ToFrozenDictionary(StringComparer.Ordinal);
    }
}

/// <summary>One unary method of the service, as its descriptor declares it.</summary>
internal sealed class RpcMethod
{
    private object? typed;

    /// <summary>Reads one method of the service descriptor.</summary>
    /// <param name="descriptor">The method's descriptor, from generated protocol code.</param>
    /// <exception cref="InvalidOperationException">The method streams, or its messages have no generated class.</exception>
    internal RpcMethod(MethodDescriptor descriptor)
    {
        ArgumentNullException.ThrowIfNull(descriptor);
        if (descriptor.IsClientStreaming || descriptor.IsServerStreaming)
        {
            throw new InvalidOperationException($"{descriptor.FullName} streams; the service's methods are unary.");
        }

        Descriptor = descriptor;
        QualifiedName = $"{descriptor.Service.FullName}/{descriptor.Name}";
        RequestType = GeneratedClass(descriptor.InputType, descriptor);
        ResponseType = GeneratedClass(descriptor.OutputType, descriptor);
        RequestParser = descriptor.InputType.Parser;
        ResponseParser = descriptor.OutputType.Parser;
    }

    /// <summary>The method's descriptor.</summary>
    internal MethodDescriptor Descriptor { get; }

    /// <summary>The method's bare name, as ParseFile.</summary>
    internal string Name => Descriptor.Name;

    /// <summary>
    /// The method's name qualified by the service, as sysml.SysMLService/ParseFile: the gRPC
    /// path and the Connect URL path, each after a leading slash.
    /// </summary>
    internal string QualifiedName { get; }

    /// <summary>The generated class of the request message.</summary>
    internal Type RequestType { get; }

    /// <summary>The generated class of the response message.</summary>
    internal Type ResponseType { get; }

    /// <summary>The request message's parser.</summary>
    internal MessageParser RequestParser { get; }

    /// <summary>The response message's parser.</summary>
    internal MessageParser ResponseParser { get; }

    /// <summary>
    /// This method typed by its messages, made once and shared, since a gRPC channel caches
    /// what it learns of a method by the method object.
    /// </summary>
    /// <returns>The typed method, or null when the types are not this method's messages.</returns>
    internal RpcMethod<TRequest, TResponse>? As<TRequest, TResponse>()
        where TRequest : class, IMessage<TRequest>
        where TResponse : class, IMessage<TResponse>
    {
        if (typeof(TRequest) != RequestType || typeof(TResponse) != ResponseType)
        {
            return null;
        }

        if (Volatile.Read(ref typed) is RpcMethod<TRequest, TResponse> shared)
        {
            return shared;
        }

        var made = new RpcMethod<TRequest, TResponse>(this);
        return (RpcMethod<TRequest, TResponse>)(Interlocked.CompareExchange(ref typed, made, null) ?? made);
    }

    private static Type GeneratedClass(MessageDescriptor message, MethodDescriptor method) =>
        message.ClrType ?? throw new InvalidOperationException(
            $"{method.FullName} uses {message.FullName}, for which no generated class was loaded.");
}

/// <summary>One unary method of the service, typed by its request and response messages.</summary>
/// <typeparam name="TRequest">The request message.</typeparam>
/// <typeparam name="TResponse">The response message.</typeparam>
internal sealed class RpcMethod<TRequest, TResponse>
    where TRequest : class, IMessage<TRequest>
    where TResponse : class, IMessage<TResponse>
{
    /// <summary>Types one method of the registry.</summary>
    /// <param name="method">The method, whose messages are <typeparamref name="TRequest"/> and <typeparamref name="TResponse"/>.</param>
    internal RpcMethod(RpcMethod method)
    {
        Untyped = method;
        RequestParser = (MessageParser<TRequest>)method.RequestParser;
        ResponseParser = (MessageParser<TResponse>)method.ResponseParser;
        Grpc = new Method<TRequest, TResponse>(
            MethodType.Unary,
            method.Descriptor.Service.FullName,
            method.Name,
            MarshallerOf(RequestParser),
            MarshallerOf(ResponseParser));
    }

    /// <summary>The method, untyped.</summary>
    internal RpcMethod Untyped { get; }

    /// <summary>The unary gRPC method a call invoker calls.</summary>
    internal Method<TRequest, TResponse> Grpc { get; }

    /// <summary>The request message's parser.</summary>
    internal MessageParser<TRequest> RequestParser { get; }

    /// <summary>The response message's parser.</summary>
    internal MessageParser<TResponse> ResponseParser { get; }

    private static Marshaller<T> MarshallerOf<T>(MessageParser<T> parser)
        where T : class, IMessage<T> =>
        Marshallers.Create<T>(
            (message, context) =>
            {
                context.SetPayloadLength(message.CalculateSize());
                message.WriteTo(context.GetBufferWriter());
                context.Complete();
            },
            context => parser.ParseFrom(context.PayloadAsReadOnlySequence()));
}
