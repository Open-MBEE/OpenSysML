namespace OpenMBEE.OpenSysML;

/// <summary>The protocol a connection speaks to the service. A connection never switches or falls back.</summary>
public enum Protocol
{
    /// <summary>gRPC over HTTP/2, with protobuf messages: the default.</summary>
    Grpc = 0,

    /// <summary>
    /// The Connect protocol's unary calls over HTTP/1.1: one POST per call, with a protobuf or JSON
    /// body as <see cref="ConnectionOptions.Encoding"/> says. It reaches a service through proxies
    /// that do not carry HTTP/2, and is answered by the service's default <c>connect</c> transport,
    /// not by its gRPC-only <c>grpc</c> transport.
    /// </summary>
    Connect = 1,
}
