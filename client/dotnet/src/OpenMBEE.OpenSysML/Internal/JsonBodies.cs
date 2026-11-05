using System;
using System.Text;
using Google.Protobuf;
using Google.Protobuf.Reflection;

namespace OpenMBEE.OpenSysML.Internal;

/// <summary>
/// Message bodies in the protobuf JSON mapping, for <see cref="MessageEncoding.Json"/>. Requests are
/// written as the mapping specifies: lowerCamelCase names, 64-bit integers as strings, bytes as
/// base64, enums by name, fields at their default left out except a oneof arm, which is always
/// written. Responses are read leniently about what a newer service may add and strictly about
/// everything else.
/// </summary>
/// <remarks>
/// A response field this client does not know is ignored, so a service newer than the client can
/// still be read. An enum value given by a name this client does not know is ignored with it, and
/// reads as the enum's zero value: the JSON parser cannot keep a name it has no number for, where
/// binary protobuf keeps an unknown value's number. An unknown value given by number is kept.
/// </remarks>
internal static class JsonBodies
{
    /// <summary>The media type of a JSON body.</summary>
    internal const string MediaType = "application/json";

    private static readonly JsonFormatter Formatter = new(JsonFormatter.Settings.Default);
    private static readonly JsonParser Parser = new(JsonParser.Settings.Default.WithIgnoreUnknownFields(true));

    // Strict, so an answer that is not UTF-8 is refused instead of read with replacement characters.
    private static readonly UTF8Encoding StrictUtf8 = new(encoderShouldEmitUTF8Identifier: false, throwOnInvalidBytes: true);

    /// <summary>Writes a message as a JSON body, in UTF-8.</summary>
    /// <param name="message">The message.</param>
    /// <returns>The body.</returns>
    internal static byte[] Serialize(IMessage message)
    {
        ArgumentNullException.ThrowIfNull(message);

        // As binary protobuf does, an unpaired surrogate in a string is written as U+FFFD.
        return Encoding.UTF8.GetBytes(Formatter.Format(message));
    }

    /// <summary>Reads a JSON body as a message of a type.</summary>
    /// <param name="body">The body, in UTF-8.</param>
    /// <param name="descriptor">The message type to read.</param>
    /// <returns>The message.</returns>
    /// <exception cref="InvalidProtocolBufferException">The body is not that message in the JSON mapping.</exception>
    /// <exception cref="DecoderFallbackException">The body is not UTF-8.</exception>
    internal static IMessage Parse(ReadOnlySpan<byte> body, MessageDescriptor descriptor)
    {
        ArgumentNullException.ThrowIfNull(descriptor);
        return Parser.Parse(StrictUtf8.GetString(body), descriptor);
    }
}
