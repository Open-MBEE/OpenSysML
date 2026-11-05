using System;
using System.Buffers;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Diagnostics.CodeAnalysis;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text.Json;
using System.Text.Unicode;

namespace OpenMBEE.OpenSysML.Metamodel;

/// <summary>
/// Reads JSON into a <see cref="MetamodelGraph"/> in two passes: the first creates an element for every
/// object of the document, the second converts every value the elements' metaclasses have a property for
/// and resolves every reference among them. Keys no property has, such as an exporter's own <c>sysx:</c>
/// annotations, are not converted; with <see cref="ReadOptions.KeepExtensions"/> they are copied, as written,
/// into JSON of their element's own. A document that is not an element graph fails the read; a value that does
/// not fit its property fails only reads of that property. Nothing of the source's JSON is kept.
/// </summary>
internal static class GraphReader
{
    private const string SysmlPrefix = "sysml:";
    private const string SysmlNamespace = "https://www.omg.org/spec/SysML#";
    private const string VersionedNamespace = "https://www.omg.org/spec/SysML/";
    private const int VersionDigits = 8;
    private const int DescribedLength = 64;
    private static readonly string[] OwnerKeys = ["owningRelationship", "owner", "owningNamespace"];
    private static readonly string[] OwnedKeys = ["ownedRelationship", "ownedRelatedElement"];

    /// <summary>Reads a document's root: an element, an array of elements or DataVersions, a DataVersion, or a Commit.</summary>
    internal static MetamodelGraph Read(JsonElement root, ReadOptions options)
    {
        Validate(root);
        switch (root.ValueKind)
        {
            case JsonValueKind.Object:
                var members = Members(root, "the document");
                return Build(
                    members.TryGetValue("change", out var change) && change.ValueKind == JsonValueKind.Array
                        ? Unwrap(change.EnumerateArray(), validate: false)
                        : Unwrap([root], validate: false),
                    options);
            case JsonValueKind.Array:
                return Build(Unwrap(root.EnumerateArray(), validate: false), options);
            default:
                throw new MalformedDocumentException($"a document is an element object, an array of them, a DataVersion or a Commit, not {Describe(root)}");
        }
    }

    /// <summary>Reads the items of a document's array: elements or DataVersions.</summary>
    internal static MetamodelGraph Read(IEnumerable<JsonElement> items, ReadOptions options) => Build(Unwrap(items, validate: true), options);

    private static List<Item> Unwrap(IEnumerable<JsonElement> items, bool validate)
    {
        var result = new List<Item>();
        var position = 0;
        foreach (var item in items)
        {
            position++;
            if (validate)
            {
                Validate(item);
            }

            if (item.ValueKind != JsonValueKind.Object)
            {
                throw new MalformedDocumentException($"item {position} of the document is {Describe(item)}, not an element object");
            }

            var members = Members(item, $"item {position} of the document");
            if (!members.ContainsKey("identity") || !members.TryGetValue("payload", out var payload))
            {
                result.Add(new Item(item, members));
            }
            else if (payload.ValueKind == JsonValueKind.Object)
            {
                result.Add(new Item(payload, Members(payload, $"the payload of item {position} of the document")));
            }
            else if (payload.ValueKind != JsonValueKind.Null)
            {
                throw new MalformedDocumentException($"the payload of DataVersion {position} is {Describe(payload)}, not an element object or null");
            }
        }

        return result;
    }

    private static MetamodelGraph Build(List<Item> items, ReadOptions options)
    {
        var supertypes = Supertypes(options);
        var metaclasses = new Dictionary<string, (MetaclassInfo Metaclass, bool Recognized)>(StringComparer.Ordinal);
        var elements = new MetamodelElement[items.Count];
        var byId = new Dictionary<string, MetamodelElement>(items.Count, StringComparer.Ordinal);
        var unrecognized = new HashSet<MetamodelElement>();
        for (var i = 0; i < items.Count; i++)
        {
            var members = items[i].Members;
            if (!members.TryGetValue("@id", out var id) || id.ValueKind != JsonValueKind.String)
            {
                throw new MalformedDocumentException($"element {i + 1} of the document has no string @id");
            }

            var elementId = IdentityString(id, $"element {i + 1}'s @id");
            if (!members.TryGetValue("@type", out var type) || type.ValueKind != JsonValueKind.String)
            {
                throw new MalformedDocumentException($"element \"{elementId}\" has no string @type");
            }

            if (byId.ContainsKey(elementId))
            {
                throw new MalformedDocumentException($"two elements have @id \"{elementId}\"");
            }

            var jsonType = IdentityString(type, $"element \"{elementId}\"'s @type");
            if (!metaclasses.TryGetValue(jsonType, out var resolved))
            {
                metaclasses[jsonType] = resolved = Resolve(jsonType, supertypes);
            }

            var element = resolved.Metaclass.CreateElement(elementId, jsonType);
            if (!resolved.Recognized)
            {
                unrecognized.Add(element);
            }

            elements[i] = element;
            byId.Add(elementId, element);
        }

        var referenced = new HashSet<string>(StringComparer.Ordinal);
        foreach (var item in items)
        {
            foreach (var key in OwnedKeys)
            {
                if (item.Members.TryGetValue(key, out var owned))
                {
                    CollectIds(owned, referenced);
                }
            }
        }

        var values = new ValueReader(byId, unrecognized, options.CheckRanges);
        var extensions = options.KeepExtensions ? new ExtensionCopier() : null;
        for (var i = 0; i < items.Count; i++)
        {
            foreach (var member in items[i].Json.EnumerateObject())
            {
                if (elements[i].MetaclassInfo.TryGetProperty(member.Name, out var property))
                {
                    values.Supply(elements[i], property, member.Value);
                }
                else if (member.Name is not "@id" and not "@type")
                {
                    extensions?.Add(member);
                }
            }

            if (extensions is not null)
            {
                elements[i].KeepExtensions(extensions.Copy());
            }
        }

        var roots = new List<INamespace>();
        for (var i = 0; i < items.Count; i++)
        {
            var members = items[i].Members;
            if (elements[i] is INamespace root
                && !referenced.Contains(root.Id)
                && !OwnerKeys.Any(key => members.TryGetValue(key, out var owner) && owner.ValueKind != JsonValueKind.Null))
            {
                roots.Add(root);
            }
        }

        return new MetamodelGraph(elements, Array.AsReadOnly(roots.ToArray()), options);
    }

    /// <summary>The metaclass an element of a written <c>@type</c> is read as, and whether the type names it.</summary>
    private static (MetaclassInfo Metaclass, bool Recognized) Resolve(string jsonType, Dictionary<string, string> supertypes)
    {
        var name = Normalize(jsonType);
        var seen = new HashSet<string>(StringComparer.Ordinal);
        for (var first = true; ; first = false)
        {
            if (Metaclasses.TryGet(name, out var metaclass))
            {
                return (metaclass, first);
            }

            if (!seen.Add(name))
            {
                throw new MalformedDocumentException($"the supertypes of @type \"{jsonType}\" form a cycle through \"{name}\"");
            }

            if (!supertypes.TryGetValue(name, out var parent))
            {
                return (Metaclasses.Element, false);
            }

            name = parent;
        }
    }

    private static Dictionary<string, string> Supertypes(ReadOptions options)
    {
        var result = new Dictionary<string, string>(StringComparer.Ordinal);
        foreach (var (key, value) in options.Supertypes ?? new Dictionary<string, string>())
        {
            if (key is null || value is null)
            {
                throw new ArgumentException("ReadOptions.Supertypes maps a type to null", nameof(options));
            }

            var type = Normalize(key);
            var supertype = Normalize(value);
            if (result.TryGetValue(type, out var other) && other != supertype)
            {
                throw new ArgumentException($"ReadOptions.Supertypes maps \"{type}\" to both \"{other}\" and \"{supertype}\"", nameof(options));
            }

            result[type] = supertype;
        }

        return result;
    }

    /// <summary>A written type without a <c>sysml:</c> prefix or a SysML namespace IRI, unversioned or versioned by an eight-digit release.</summary>
    private static string Normalize(string written)
    {
        if (written.StartsWith(SysmlPrefix, StringComparison.Ordinal))
        {
            return written[SysmlPrefix.Length..];
        }

        if (written.StartsWith(SysmlNamespace, StringComparison.Ordinal))
        {
            return written[SysmlNamespace.Length..];
        }

        var prefix = VersionedNamespace.Length + VersionDigits + 1;
        return written.StartsWith(VersionedNamespace, StringComparison.Ordinal)
            && written.Length >= prefix
            && !written.AsSpan(VersionedNamespace.Length, VersionDigits).ContainsAnyExceptInRange('0', '9')
            && written[prefix - 1] == '/'
                ? written[prefix..]
                : written;
    }

    /// <summary>Every <c>@id</c> a value refers to, however deeply nested.</summary>
    private static void CollectIds(JsonElement value, HashSet<string> ids)
    {
        if (value.ValueKind == JsonValueKind.Object)
        {
            var members = Members(value, "a reference");
            if (members.TryGetValue("@id", out var id) && id.ValueKind == JsonValueKind.String)
            {
                if (TryDecodeString(id, out var target, out _))
                {
                    ids.Add(target);
                }

                return;
            }

            foreach (var member in members.Values)
            {
                CollectIds(member, ids);
            }
        }
        else if (value.ValueKind == JsonValueKind.Array)
        {
            foreach (var item in value.EnumerateArray())
            {
                CollectIds(item, ids);
            }
        }
    }

    /// <summary>Validates every object's keys before conversion can skip a value or an envelope's metadata.</summary>
    private static void Validate(JsonElement root)
    {
        if (root.ValueKind != JsonValueKind.Undefined && !Utf8.IsValid(JsonMarshal.GetRawUtf8Value(root)))
        {
            throw new MalformedDocumentException("the document is not valid UTF-8");
        }

        var pending = new Stack<JsonElement>();
        pending.Push(root);
        while (pending.TryPop(out var value))
        {
            if (value.ValueKind == JsonValueKind.Object)
            {
                var names = new HashSet<string>(StringComparer.Ordinal);
                foreach (var member in value.EnumerateObject())
                {
                    string name;
                    try
                    {
                        name = member.Name;
                    }
                    catch (InvalidOperationException e) when (e is not ObjectDisposedException)
                    {
                        throw new MalformedDocumentException($"a property name is not a valid Unicode string: {e.Message}", e);
                    }

                    if (!names.Add(name))
                    {
                        throw new MalformedDocumentException($"an object of the document has the key \"{name}\" twice");
                    }

                    if (member.Value.ValueKind is JsonValueKind.Object or JsonValueKind.Array)
                    {
                        pending.Push(member.Value);
                    }
                }
            }
            else if (value.ValueKind == JsonValueKind.Array)
            {
                foreach (var item in value.EnumerateArray())
                {
                    if (item.ValueKind is JsonValueKind.Object or JsonValueKind.Array)
                    {
                        pending.Push(item);
                    }
                }
            }
        }
    }

    private static string IdentityString(JsonElement value, string where) =>
        TryDecodeString(value, out var text, out var failure)
            ? text
            : throw new MalformedDocumentException($"{where} is not a valid Unicode string: {failure.Message}", failure);

    private static bool TryDecodeString(
        JsonElement value,
        [NotNullWhen(true)] out string? text,
        [NotNullWhen(false)] out InvalidOperationException? failure)
    {
        if (value.ValueKind != JsonValueKind.String)
        {
            throw new ArgumentException("only JSON strings can be decoded", nameof(value));
        }

        try
        {
            text = value.GetString()!;
            failure = null;
            return true;
        }
        catch (InvalidOperationException e) when (e is not ObjectDisposedException)
        {
            text = null;
            failure = e;
            return false;
        }
    }

    /// <summary>An object's members by key; a key twice is malformed, whatever parsed the JSON.</summary>
    private static Dictionary<string, JsonElement> Members(JsonElement json, string where)
    {
        var members = new Dictionary<string, JsonElement>(StringComparer.Ordinal);
        foreach (var member in json.EnumerateObject())
        {
            if (!members.TryAdd(member.Name, member.Value))
            {
                throw new MalformedDocumentException($"{where} has the key \"{member.Name}\" twice");
            }
        }

        return members;
    }

    /// <summary>A JSON value for a diagnostic: its text when a scalar, shortened, else its kind.</summary>
    private static string Describe(JsonElement value) => value.ValueKind switch
    {
        JsonValueKind.Object => "an object",
        JsonValueKind.Array => "an array",
        JsonValueKind.Undefined => "no value",
        _ => value.GetRawText() is var text && text.Length > DescribedLength ? text[..(DescribedLength - 3)] + "..." : value.GetRawText(),
    };

    /// <summary>
    /// Copies an element's extension members into a JSON object of its own, which no source document owns and
    /// nothing disposes: one copy per element, however many members it holds.
    /// </summary>
    private sealed class ExtensionCopier
    {
        private static readonly IReadOnlyDictionary<string, JsonElement> None = ReadOnlyDictionary<string, JsonElement>.Empty;

        private readonly ArrayBufferWriter<byte> buffer = new();
        private readonly List<JsonProperty> members = [];

        internal void Add(JsonProperty member) => members.Add(member);

        /// <summary>The members added since the last copy, by key, as an immutable dictionary; then forgets them.</summary>
        internal IReadOnlyDictionary<string, JsonElement> Copy()
        {
            if (members.Count == 0)
            {
                return None;
            }

            buffer.ResetWrittenCount();
            using (var writer = new Utf8JsonWriter(buffer))
            {
                writer.WriteStartObject();
                foreach (var member in members)
                {
                    writer.WritePropertyName(member.Name);
                    WriteValue(writer, member.Value);
                }

                writer.WriteEndObject();
            }

            members.Clear();
            var reader = new Utf8JsonReader(buffer.WrittenSpan);
            var copy = JsonElement.ParseValue(ref reader);
            var byKey = new Dictionary<string, JsonElement>(StringComparer.Ordinal);
            foreach (var member in copy.EnumerateObject())
            {
                byKey.Add(member.Name, member.Value);
            }

            return new ReadOnlyDictionary<string, JsonElement>(byKey);
        }

        private static void WriteValue(Utf8JsonWriter writer, JsonElement value)
        {
            switch (value.ValueKind)
            {
                case JsonValueKind.Object:
                    writer.WriteStartObject();
                    foreach (var member in value.EnumerateObject())
                    {
                        writer.WritePropertyName(member.Name);
                        WriteValue(writer, member.Value);
                    }

                    writer.WriteEndObject();
                    break;
                case JsonValueKind.Array:
                    writer.WriteStartArray();
                    foreach (var item in value.EnumerateArray())
                    {
                        WriteValue(writer, item);
                    }

                    writer.WriteEndArray();
                    break;
                case JsonValueKind.String:
                    // Keep opaque string escapes without carrying comments or trailing commas from containers.
                    writer.WriteRawValue(JsonMarshal.GetRawUtf8Value(value), skipInputValidation: true);
                    break;
                default:
                    value.WriteTo(writer);
                    break;
            }
        }
    }

    /// <summary>One element object of the document, and its members by key.</summary>
    private sealed record Item(JsonElement Json, Dictionary<string, JsonElement> Members);

    /// <summary>
    /// Converts the values of the document's properties and supplies them to its elements. A value that does not
    /// fit its property, or a reference the document does not resolve, is supplied as that failure, which every
    /// read of the property reports: the first of a list's, in document order.
    /// </summary>
    private sealed class ValueReader(Dictionary<string, MetamodelElement> byId, HashSet<MetamodelElement> unrecognized, bool checkRanges)
    {
        internal void Supply(MetamodelElement element, MetaProperty property, JsonElement value)
        {
            if (value.ValueKind == JsonValueKind.Null)
            {
                element.Supply(property, null);
                return;
            }

            var converted = property.IsMany ? ConvertMany(element, property, value)
                : value.ValueKind == JsonValueKind.Array ? Converted.Malformed("expected a single value, got an array")
                : Convert(element, property, value);
            if (converted.MalformedReason is { } malformed)
            {
                element.SupplyMalformed(property, malformed);
            }
            else if (converted.IsUnresolved)
            {
                element.SupplyUnresolved(property, converted.UnresolvedId, converted.UnresolvedRef);
            }
            else if (converted.OutOfRange is { } reason)
            {
                element.SupplyOutOfRange(property, converted.Value!, reason);
            }
            else
            {
                element.Supply(property, converted.Value);
            }
        }

        private Converted ConvertMany(MetamodelElement element, MetaProperty property, JsonElement value)
        {
            if (value.ValueKind != JsonValueKind.Array)
            {
                return Converted.Malformed($"expected an array, got {Describe(value)}");
            }

            var items = new List<object>(value.GetArrayLength());
            string? outOfRange = null;
            foreach (var item in value.EnumerateArray())
            {
                var converted = item.ValueKind == JsonValueKind.Null ? Converted.Malformed("the array holds null") : Convert(element, property, item);
                if (converted.MalformedReason is not null || converted.IsUnresolved)
                {
                    return converted;
                }

                outOfRange ??= converted.OutOfRange;
                items.Add(converted.Value!);
            }

            return outOfRange is not null
                ? new Converted(Array.AsReadOnly(items.Cast<IMetamodelElement>().ToArray()), OutOfRange: outOfRange)
                : new Converted(TypedList.Of(property.ValueType, items));
        }

        private Converted Convert(MetamodelElement element, MetaProperty property, JsonElement value)
        {
            if (value.ValueKind == JsonValueKind.Object)
            {
                return property.RangeKind == PropertyRangeKind.Metaclass
                    ? Reference(element, property, value)
                    : Converted.Malformed($"expected {Expected(property)}, got a reference object");
            }

            string? text = null;
            if (value.ValueKind == JsonValueKind.String && !TryDecodeString(value, out text, out var failure))
            {
                return Converted.InvalidString(failure);
            }

            if (property.RangeKind == PropertyRangeKind.Enumeration && text is not null)
            {
                return property.RangeEnumeration!.TryParse(text, out var literal)
                    ? new Converted(literal)
                    : Converted.Malformed($"{Describe(value)} is not a {property.RangeEnumeration.Name} literal");
            }

            object? converted = property.RangeKind switch
            {
                PropertyRangeKind.Metaclass or PropertyRangeKind.Enumeration => null,
                _ when property.ValueType == typeof(bool) => value.ValueKind switch
                {
                    JsonValueKind.True => true,
                    JsonValueKind.False => false,
                    _ => null,
                },
                _ when property.ValueType == typeof(string) => text,
                _ when property.ValueType == typeof(int) => value.ValueKind == JsonValueKind.Number && value.TryGetInt32(out var integer) ? integer : null,
                _ => value.ValueKind == JsonValueKind.Number && value.TryGetDouble(out var real) && double.IsFinite(real) ? real : null,
            };
            return converted is not null
                ? new Converted(converted)
                : Converted.Malformed($"expected {Expected(property)}, got {Describe(value)}");
        }

        private Converted Reference(MetamodelElement element, MetaProperty property, JsonElement value)
        {
            var members = Members(value, $"a reference in \"{property.JsonKey}\" of element {element.Id}");
            if (members.TryGetValue("@ref", out var name))
            {
                if (name.ValueKind != JsonValueKind.String)
                {
                    return Converted.Malformed($"@ref must be a string, got {Describe(name)}");
                }

                return TryDecodeString(name, out var targetRef, out var failure)
                    ? new Converted(null, UnresolvedRef: targetRef)
                    : Converted.InvalidString(failure);
            }

            if (!members.TryGetValue("@id", out var id))
            {
                return Converted.Malformed("expected a reference object with an @id or @ref, got an object with neither");
            }

            if (id.ValueKind != JsonValueKind.String)
            {
                return Converted.Malformed($"@id must be a string, got {Describe(id)}");
            }

            if (!TryDecodeString(id, out var targetId, out var idFailure))
            {
                return Converted.InvalidString(idFailure);
            }

            if (!byId.TryGetValue(targetId, out var target))
            {
                return new Converted(null, UnresolvedId: targetId);
            }

            if (property.ValueType.IsInstanceOfType(target))
            {
                return new Converted(target);
            }

            var reason = $"reference target @id \"{targetId}\" has @type \"{target.JsonType}\", read as {target.MetaclassInfo.Name}; expected {property.RangeMetaclass!.Name}";
            if (unrecognized.Contains(target))
            {
                reason += "; ReadOptions.Supertypes maps an @type the metamodel lacks to a metaclass";
            }

            return checkRanges ? Converted.Malformed(reason) : new Converted(target, OutOfRange: reason);
        }

        private static string Expected(MetaProperty property) => property.RangeKind switch
        {
            PropertyRangeKind.Metaclass => $"a reference to {property.RangeMetaclass!.Name}",
            PropertyRangeKind.Enumeration => $"a {property.RangeEnumeration!.Name} literal",
            _ when property.ValueType == typeof(bool) => "a Boolean",
            _ when property.ValueType == typeof(string) => "a string",
            _ when property.ValueType == typeof(int) => "a 32-bit integer",
            _ => "a finite real number",
        };
    }

    /// <summary>
    /// A converted value: the value, a reference the document does not resolve, a value that does not fit the
    /// property, or references outside the property's range kept with range checks off.
    /// </summary>
    private readonly record struct Converted(
        object? Value,
        string? UnresolvedId = null,
        string? UnresolvedRef = null,
        string? OutOfRange = null,
        string? MalformedReason = null)
    {
        internal bool IsUnresolved => UnresolvedId is not null || UnresolvedRef is not null;

        internal static Converted Malformed(string reason) => new(null, MalformedReason: reason);

        internal static Converted InvalidString(InvalidOperationException failure) => Malformed($"invalid Unicode string: {failure.Message}");
    }
}
