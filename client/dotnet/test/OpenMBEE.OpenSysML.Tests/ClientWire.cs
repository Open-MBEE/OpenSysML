using Xunit;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The three ways a connection can speak to the service. A test of behavior every protocol shares
/// is a theory over all three, so no protocol can pass it by being left out.
/// </summary>
public enum ClientWire
{
    /// <summary>gRPC with protobuf, the default.</summary>
    Grpc,

    /// <summary>Connect with protobuf bodies.</summary>
    ConnectProto,

    /// <summary>Connect with JSON bodies.</summary>
    ConnectJson,
}

/// <summary>What each <see cref="ClientWire"/> asks of a connection.</summary>
internal static class ClientWires
{
    /// <summary>Every wire, for a theory of behavior they share.</summary>
    public static TheoryData<ClientWire> All => [ClientWire.Grpc, ClientWire.ConnectProto, ClientWire.ConnectJson];

    /// <summary>The Connect wires, for a theory of behavior particular to the Connect protocol.</summary>
    public static TheoryData<ClientWire> Connect => [ClientWire.ConnectProto, ClientWire.ConnectJson];

    /// <summary>The protocol a wire speaks.</summary>
    internal static Protocol Protocol(this ClientWire wire) => wire == ClientWire.Grpc ? OpenSysML.Protocol.Grpc : OpenSysML.Protocol.Connect;

    /// <summary>The encoding a wire uses.</summary>
    internal static MessageEncoding Encoding(this ClientWire wire) => wire == ClientWire.ConnectJson ? MessageEncoding.Json : MessageEncoding.Protobuf;

    /// <summary>Options naming a service and speaking a wire.</summary>
    internal static ConnectionOptions To(this ClientWire wire, string address) =>
        new() { Address = address, Protocol = wire.Protocol(), Encoding = wire.Encoding() };
}
