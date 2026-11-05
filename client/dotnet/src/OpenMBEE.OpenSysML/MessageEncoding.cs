namespace OpenMBEE.OpenSysML;

/// <summary>How a connection encodes the messages it sends and reads.</summary>
public enum MessageEncoding
{
    /// <summary>Binary protobuf: the default, and the only encoding <see cref="Protocol.Grpc"/> speaks.</summary>
    Protobuf = 0,

    /// <summary>
    /// The protobuf JSON mapping, over <see cref="Protocol.Connect"/> only. Larger and slower than
    /// protobuf; it suits a proxy or log that must read the traffic. A response field this client
    /// does not know is ignored, and so is an enum value given by a name this client does not know,
    /// which reads as the enum's zero value; binary protobuf keeps an unknown value's number.
    /// </summary>
    Json = 1,
}
