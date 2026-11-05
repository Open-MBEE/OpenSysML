using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace OpenMBEE.OpenSysML.Tests;

/// <summary>
/// The .NET client's parity inventory, client/dotnet/test/testdata/parity.json: every RPC of the
/// service with the public method that delivers it and the capability gates its request is
/// subject to; the Value and EditOperation arms; the response fields a capability governs; and a
/// classification of every request and response field. <see cref="ParityChecker"/> holds it to
/// the schema the client was generated from and to the conformance scenarios.
/// </summary>
/// <param name="Service">The service's full name.</param>
/// <param name="Rpcs">One row per RPC, in the schema's order.</param>
/// <param name="Values">The arms of sysml.Value.</param>
/// <param name="Edits">The arms of sysml.EditOperation.</param>
/// <param name="Population">Response fields, by descriptor path, mapped to the capability that governs whether the service populates them.</param>
/// <param name="Fields">
/// Every field of every message an RPC reaches, by message and field name. The file writes each message's fields in
/// groups of one classification, <see cref="FieldGroups"/> expands them.
/// </param>
internal sealed record ParityInventory(
    string Service,
    IReadOnlyList<RpcRow> Rpcs,
    ValueArms Values,
    EditArms Edits,
    IReadOnlyDictionary<string, string> Population,
    IReadOnlyDictionary<string, IReadOnlyDictionary<string, FieldClass>> Fields)
{
    /// <summary>The committed inventory's path.</summary>
    internal static string CommittedPath => Repository.PathOf("client", "dotnet", "test", "testdata", "parity.json");

    private static readonly JsonSerializerOptions Options = new()
    {
        PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
        UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow,
        RespectNullableAnnotations = true,
        RespectRequiredConstructorParameters = true,
        AllowDuplicateProperties = false,
        Converters = { new FieldGroups() },
    };

    /// <summary>Reads the committed inventory.</summary>
    internal static ParityInventory LoadCommitted() => Parse(File.ReadAllText(CommittedPath));

    /// <summary>Reads an inventory, refusing an unknown or duplicated member and a missing required one.</summary>
    internal static ParityInventory Parse(string json) =>
        JsonSerializer.Deserialize<ParityInventory>(json, Options)
            ?? throw new JsonException("The parity inventory is null.");
}

/// <summary>One RPC: its wire types, the public surface that delivers it, and its request gates.</summary>
/// <param name="Rpc">The bare method name.</param>
/// <param name="Request">The request message's full name.</param>
/// <param name="Response">The response message's full name.</param>
/// <param name="Methods">The public methods that call it, as Owner.MethodAsync.</param>
/// <param name="Input">What those methods take.</param>
/// <param name="Result">What they return.</param>
/// <param name="Area">The area of the client that delivers it.</param>
/// <param name="Status">deferred, or implemented once a public method calls it.</param>
/// <param name="Gates">The capabilities the service requires of the request, in the order it checks them.</param>
/// <param name="ValueFields">The request fields, as dotted paths, whose Values are held to the Value arms' send gates.</param>
/// <param name="Scenarios">Every conformance scenario of the RPC, by ID.</param>
/// <param name="Tests">The tests that prove an implemented row, as Class.Method.</param>
internal sealed record RpcRow(
    string Rpc,
    string Request,
    string Response,
    IReadOnlyList<string> Methods,
    string Input,
    string Result,
    string Area,
    string Status,
    IReadOnlyList<Gate> Gates,
    IReadOnlyList<string> ValueFields,
    IReadOnlyList<string> Scenarios,
    IReadOnlyList<string>? Tests = null);

/// <summary>A capability the service requires, and the request content that makes it required.</summary>
/// <param name="Capability">The capability's name.</param>
/// <param name="When">The predicate, in words; always for an unconditional gate.</param>
/// <param name="Field">The field, as a dotted path from the gated message, the predicate reads.</param>
internal sealed record Gate(string Capability, string When, string? Field = null);

/// <summary>The arms of sysml.Value and how each crosses the wire.</summary>
/// <param name="Message">The message's full name.</param>
/// <param name="Oneof">The oneof holding the arms.</param>
/// <param name="Area">The area of the client that maps them.</param>
/// <param name="SendGateOrder">The send capabilities in the order the service checks them.</param>
/// <param name="Arms">Every arm, by field name.</param>
internal sealed record ValueArms(
    string Message,
    string Oneof,
    string Area,
    IReadOnlyList<string> SendGateOrder,
    IReadOnlyDictionary<string, ValueArm> Arms);

/// <summary>One arm of sysml.Value.</summary>
/// <param name="Kind">The client's name for the kind of value.</param>
/// <param name="Status">deferred, or implemented once it is encoded and decoded.</param>
/// <param name="Send">ungated, response-only, or the capability a request holding it needs.</param>
/// <param name="Receive">The capability under which the service sends it, if any.</param>
/// <param name="Note">Gates its nested content carries.</param>
/// <param name="Test">The test that proves an implemented arm, as Class.Method.</param>
internal sealed record ValueArm(string Kind, string Status, string Send, string? Receive = null, string? Note = null, string? Test = null);

/// <summary>The arms of sysml.EditOperation and the capabilities each needs.</summary>
/// <param name="Message">The message's full name.</param>
/// <param name="Oneof">The oneof holding the arms.</param>
/// <param name="Area">The area of the client that maps them.</param>
/// <param name="Checked">When the service checks the arms' gates.</param>
/// <param name="GateOrder">The arms' capabilities in the order the service checks them.</param>
/// <param name="Arms">Every arm, by field name.</param>
internal sealed record EditArms(
    string Message,
    string Oneof,
    string Area,
    string Checked,
    IReadOnlyList<string> GateOrder,
    IReadOnlyDictionary<string, EditArm> Arms);

/// <summary>One arm of sysml.EditOperation.</summary>
/// <param name="Edit">The client's name for the edit.</param>
/// <param name="Status">deferred, or implemented once it is encoded.</param>
/// <param name="Gates">The capabilities an operation of this arm needs; a field path is relative to the arm's message.</param>
/// <param name="Test">The test that proves an implemented arm, as Class.Method.</param>
internal sealed record EditArm(string Edit, string Status, IReadOnlyList<Gate> Gates, string? Test = null);

/// <summary>
/// How the client treats one field: deferred to the area that will map it, typed by a public API,
/// deliberately left to the raw call (raw-only), or not supported, with the reason a raw-only or
/// unsupported field is and the test that proves a field that is not deferred.
/// </summary>
/// <param name="Status">deferred, typed, raw-only or unsupported.</param>
/// <param name="Reason">Why the field is raw-only or unsupported.</param>
/// <param name="Test">The test that proves the classification, as Class.Method.</param>
internal sealed record FieldClass(string Status, string? Reason = null, string? Test = null);

/// <summary>
/// Reads the inventory's field classifications, written per message as groups of the fields that share one
/// classification: <c>{"status": …, "reason": …, "test": …, "fields": [names]}</c>, reason and test only where the
/// classification has them. A message, a group member or a field named twice, a group of no fields, and two groups of
/// one classification, which are one group, are refused, so each field has exactly one classification and the file
/// one way of writing it.
/// </summary>
internal sealed class FieldGroups : JsonConverter<IReadOnlyDictionary<string, IReadOnlyDictionary<string, FieldClass>>>
{
    public override IReadOnlyDictionary<string, IReadOnlyDictionary<string, FieldClass>> Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options)
    {
        Expect(ref reader, JsonTokenType.StartObject, "The field classifications are an object of messages");
        var messages = new Dictionary<string, IReadOnlyDictionary<string, FieldClass>>(StringComparer.Ordinal);
        while (reader.Read() && reader.TokenType != JsonTokenType.EndObject)
        {
            var message = reader.GetString()!;
            if (messages.ContainsKey(message))
            {
                throw new JsonException($"Message {message} is classified twice.");
            }

            reader.Read();
            messages[message] = Groups(ref reader, message);
        }

        return messages;
    }

    public override void Write(Utf8JsonWriter writer, IReadOnlyDictionary<string, IReadOnlyDictionary<string, FieldClass>> value, JsonSerializerOptions options) =>
        throw new NotSupportedException("The inventory is written by hand.");

    private static Dictionary<string, FieldClass> Groups(ref Utf8JsonReader reader, string message)
    {
        Expect(ref reader, JsonTokenType.StartArray, $"Message {message}'s fields are a list of groups");
        var fields = new Dictionary<string, FieldClass>(StringComparer.Ordinal);
        var classifications = new HashSet<FieldClass>();
        while (reader.Read() && reader.TokenType != JsonTokenType.EndArray)
        {
            var (classification, names) = Group(ref reader, message);
            if (!classifications.Add(classification))
            {
                throw new JsonException($"Message {message} has two groups classified {Describe(classification)}; they are one group.");
            }

            foreach (var name in names)
            {
                if (!fields.TryAdd(name, classification))
                {
                    throw new JsonException($"Field {message}.{name} is classified twice.");
                }
            }
        }

        return fields;
    }

    private static (FieldClass Classification, List<string> Fields) Group(ref Utf8JsonReader reader, string message)
    {
        Expect(ref reader, JsonTokenType.StartObject, $"A group of message {message} is an object");
        string? status = null, reason = null, test = null;
        List<string>? names = null;
        var seen = new HashSet<string>(StringComparer.Ordinal);
        while (reader.Read() && reader.TokenType != JsonTokenType.EndObject)
        {
            var member = reader.GetString()!;
            if (!seen.Add(member))
            {
                throw new JsonException($"A group of message {message} repeats its member {member}.");
            }

            reader.Read();
            switch (member)
            {
                case "status":
                    status = Text(ref reader, message, member);
                    break;
                case "reason":
                    reason = Text(ref reader, message, member);
                    break;
                case "test":
                    test = Text(ref reader, message, member);
                    break;
                case "fields":
                    names = Names(ref reader, message);
                    break;
                default:
                    throw new JsonException($"A group of message {message} has an unknown member {member}.");
            }
        }

        if (status is null || names is null)
        {
            throw new JsonException($"A group of message {message} has no {(status is null ? "status" : "fields")}.");
        }

        return names.Count == 0
            ? throw new JsonException($"A group of message {message} classifies no field.")
            : (new FieldClass(status, reason, test), names);
    }

    private static List<string> Names(ref Utf8JsonReader reader, string message)
    {
        Expect(ref reader, JsonTokenType.StartArray, $"A group's fields of message {message} are a list");
        var names = new List<string>();
        while (reader.Read() && reader.TokenType != JsonTokenType.EndArray)
        {
            names.Add(Text(ref reader, message, "fields"));
        }

        return names;
    }

    private static string Text(ref Utf8JsonReader reader, string message, string member) =>
        reader.TokenType == JsonTokenType.String
            ? reader.GetString()!
            : throw new JsonException($"A group's {member} of message {message} is text, not {reader.TokenType}.");

    private static void Expect(ref Utf8JsonReader reader, JsonTokenType token, string what)
    {
        if (reader.TokenType != token)
        {
            throw new JsonException($"{what}, not {reader.TokenType}.");
        }
    }

    private static string Describe(FieldClass classification) =>
        string.Join(", ", new[] { classification.Status, classification.Reason, classification.Test }.Where(part => part is not null));
}
