using System;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Text.Json.Nodes;
using System.Text.RegularExpressions;
using Google.Protobuf;
using Google.Protobuf.Reflection;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Builds the request a scenario states as the RPC's wire message: the protobuf-JSON with its
/// placeholders resolved, decoded by the schema the client was generated from. A request that
/// does not fit its message, or names an RPC the schema lacks, is an error in the scenario.
/// </summary>
internal static partial class ScenarioRequests
{
    /// <summary>The placeholder a request writes for its model's hash.</summary>
    internal const string ModelHashPlaceholder = "${model_hash}";

    private static readonly JsonParser Parser = new(JsonParser.Settings.Default);

    /// <summary>The service every scenario addresses.</summary>
    internal static ServiceDescriptor Service => Wire.SysMLService.Descriptor;

    /// <summary>The method of the service a bare name spells.</summary>
    /// <param name="name">The method's bare name, as <see cref="Scenario.Method"/> gives it.</param>
    /// <returns>Its descriptor.</returns>
    /// <exception cref="InvalidDataException">The service has no such method.</exception>
    internal static MethodDescriptor Method(string name) =>
        Service.FindMethodByName(name)
            ?? throw new InvalidDataException($"{Service.FullName} has no RPC {Spelling.Quote(name)}");

    /// <summary>The scenario's request as its RPC's wire message, with every placeholder resolved.</summary>
    /// <param name="scenario">The scenario.</param>
    /// <param name="modelHash">The hash the service gave the scenario's model, or empty when it has none.</param>
    /// <param name="fixtures">The fixtures <c>${fixture:…}</c> and <c>${fixture_base64:…}</c> name.</param>
    /// <returns>The request message; the empty message when the scenario states no request.</returns>
    /// <exception cref="InvalidDataException">The RPC is unknown, a placeholder cannot be resolved, or the request does not fit.</exception>
    internal static IMessage Build(Scenario scenario, string modelHash, FixtureDirectory fixtures)
    {
        ArgumentNullException.ThrowIfNull(scenario);
        ArgumentNullException.ThrowIfNull(modelHash);
        ArgumentNullException.ThrowIfNull(fixtures);

        var input = Method(scenario.Method).InputType;
        if (scenario.Request is not { } request)
        {
            return input.Parser.ParseFrom(ByteString.Empty);
        }

        var resolved = Resolve(request, modelHash, fixtures);
        try
        {
            QuoteWideIntegers(resolved, input);
            return Parser.Parse(resolved?.ToJsonString() ?? "null", input);
        }
        catch (Exception e) when (e is InvalidProtocolBufferException or InvalidJsonException or InvalidDataException)
        {
            throw new InvalidDataException($"request does not fit {input.FullName}: {e.Message}", e);
        }
    }

    /// <summary>
    /// A request tree with <c>${model_hash}</c>, <c>${fixture:name}</c> and
    /// <c>${fixture_base64:name}</c> replaced wherever a whole string is one of them; the last is a
    /// fixture's bytes as protobuf-JSON carries a bytes field. Numbers keep their literals.
    /// </summary>
    /// <param name="element">The request, or a part of it.</param>
    /// <param name="modelHash">The model's hash, or empty when the scenario declares no model.</param>
    /// <param name="fixtures">The fixtures directory.</param>
    /// <returns>The resolved tree.</returns>
    /// <exception cref="InvalidDataException">A placeholder cannot be resolved, or an object states a member twice.</exception>
    internal static JsonNode? Resolve(JsonElement element, string modelHash, FixtureDirectory fixtures) =>
        element.ValueKind switch
        {
            JsonValueKind.Object => ResolveObject(element, modelHash, fixtures),
            JsonValueKind.Array => new JsonArray([.. element.EnumerateArray().Select(item => Resolve(item, modelHash, fixtures))]),
            JsonValueKind.String => JsonValue.Create(ResolveText(element.GetString()!, modelHash, fixtures)),
            _ => JsonNode.Parse(element.GetRawText()),
        };

    /// <summary>
    /// Writes exact canonical digits for 64-bit integer fields, also from decimal and exponent
    /// spellings, as text: the parser reads JSON numbers through a double. The schema imports no
    /// well-known type, whose JSON mappings this does not walk; a test holds it to that.
    /// </summary>
    /// <param name="node">A request, or a message within it, as resolved JSON.</param>
    /// <param name="message">The message it encodes.</param>
    internal static void QuoteWideIntegers(JsonNode? node, MessageDescriptor message)
    {
        if (node is not JsonObject members)
        {
            return;
        }

        foreach (var (name, value) in members.ToList())
        {
            var field = message.FindFieldByName(name)
                ?? message.Fields.InDeclarationOrder().FirstOrDefault(candidate => candidate.JsonName == name);
            if (field is null)
            {
                continue;
            }

            if (field.IsMap && value is JsonObject entries)
            {
                var valueField = field.MessageType.FindFieldByNumber(2);
                foreach (var (key, entry) in entries.ToList())
                {
                    if (Exact(entry, valueField) is { } exact)
                    {
                        entries[key] = exact;
                    }
                }
            }
            else if (field.IsRepeated && value is JsonArray items)
            {
                for (var index = 0; index < items.Count; index++)
                {
                    if (Exact(items[index], field) is { } exact)
                    {
                        items[index] = exact;
                    }
                }
            }
            else if (!field.IsMap && !field.IsRepeated && Exact(value, field) is { } exact)
            {
                members[name] = exact;
            }
        }
    }

    /// <summary>A field's value written exactly, or null when it is already.</summary>
    private static JsonValue? Exact(JsonNode? value, FieldDescriptor field)
    {
        switch (field.FieldType)
        {
            case FieldType.Int64 or FieldType.UInt64 or FieldType.SInt64 or FieldType.Fixed64 or FieldType.SFixed64
                when value is JsonValue number && number.GetValueKind() is JsonValueKind.Number or JsonValueKind.String:
                var literal = number.GetValueKind() == JsonValueKind.String ? number.GetValue<string>() : number.ToJsonString();
                var digits = JsonInteger.Normalize(literal);
                var unsigned = field.FieldType is FieldType.UInt64 or FieldType.Fixed64;
                if (digits is null || !(unsigned
                    ? ulong.TryParse(digits, NumberStyles.None, CultureInfo.InvariantCulture, out _)
                    : long.TryParse(digits, NumberStyles.AllowLeadingSign, CultureInfo.InvariantCulture, out _)))
                {
                    throw new InvalidDataException(
                        $"{field.FullName} needs an exact {(unsigned ? "uint64" : "int64")} value, got {Spelling.Quote(literal)}");
                }

                return JsonValue.Create(digits);
            case FieldType.Message:
                QuoteWideIntegers(value, field.MessageType);
                return null;
            default:
                return null;
        }
    }

    private static JsonObject ResolveObject(JsonElement element, string modelHash, FixtureDirectory fixtures)
    {
        var resolved = new JsonObject();
        foreach (var member in element.EnumerateObject())
        {
            if (!resolved.TryAdd(member.Name, Resolve(member.Value, modelHash, fixtures)))
            {
                throw new InvalidDataException($"the request states {Spelling.Quote(member.Name)} twice");
            }
        }

        return resolved;
    }

    private static string ResolveText(string text, string modelHash, FixtureDirectory fixtures)
    {
        if (text == ModelHashPlaceholder)
        {
            return modelHash.Length > 0
                ? modelHash
                : throw new InvalidDataException($"the request names {ModelHashPlaceholder} but the scenario declares no model");
        }

        if (FixtureReference().Match(text) is { Success: true } fixture)
        {
            return fixtures.Text(fixture.Groups[1].Value);
        }

        if (FixtureBase64Reference().Match(text) is { Success: true } binary)
        {
            return Convert.ToBase64String(fixtures.Bytes(binary.Groups[1].Value));
        }

        return text;
    }

    [GeneratedRegex(@"^\$\{fixture:([^}]+)\}\z", RegexOptions.CultureInvariant)]
    private static partial Regex FixtureReference();

    [GeneratedRegex(@"^\$\{fixture_base64:([^}]+)\}\z", RegexOptions.CultureInvariant)]
    private static partial Regex FixtureBase64Reference();
}
