using System;

namespace OpenMBEE.OpenSysML;

/// <summary>
/// The service answered a value this client cannot read: one naming no kind, or of a kind added to
/// the service after this client, which a newer client may read. It is never read as a null, which
/// would stand for a value the model does not have. The response is not otherwise read.
/// </summary>
/// <remarks>
/// <see cref="ValueBytes"/> holds the value message as it arrived. Over protobuf that includes the
/// unknown arm's bytes; over Connect JSON an unknown field is not kept, so they hold none of it.
/// </remarks>
public class UnsupportedValueException : OpenSysMLException
{
    private readonly byte[] valueBytes;

    /// <summary>Creates the exception.</summary>
    /// <param name="message">What could not be read.</param>
    /// <param name="method">The method whose response held the value, as <c>sysml.SysMLService/Evaluate</c>.</param>
    /// <param name="path">Where in the response the value is, as <c>result.sequence[2]</c>.</param>
    /// <param name="valueBytes">The value message as it arrived, serialized; copied.</param>
    public UnsupportedValueException(string message, string method, string path, ReadOnlySpan<byte> valueBytes)
        : base(message)
    {
        ArgumentNullException.ThrowIfNull(method);
        ArgumentNullException.ThrowIfNull(path);
        Method = method;
        Path = path;
        this.valueBytes = valueBytes.ToArray();
    }

    /// <summary>The method whose response held the value, as <c>sysml.SysMLService/Evaluate</c>.</summary>
    public string Method { get; }

    /// <summary>Where in the response the value is, as <c>result.sequence[2]</c>.</summary>
    public string Path { get; }

    /// <summary>The value message as it arrived, serialized, unknown fields included.</summary>
    public ReadOnlyMemory<byte> ValueBytes => valueBytes;
}
