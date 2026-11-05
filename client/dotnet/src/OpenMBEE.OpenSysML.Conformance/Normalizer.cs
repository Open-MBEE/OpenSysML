using System;
using System.Collections;
using System.Collections.Frozen;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using Google.Protobuf;
using Google.Protobuf.Reflection;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Turns a response into the tree scenarios are compared against, with the values that are not the
/// same twice replaced; see "Normalized values" in conformance/README.md. A message becomes an
/// <see cref="IReadOnlyDictionary{TKey, TValue}"/> of its set fields by schema name, a map the
/// same by key, a repeated field an <see cref="IReadOnlyList{T}"/>, and a scalar a
/// <see cref="string"/>, <see cref="bool"/>, <see cref="double"/> (a Real), <see cref="long"/>
/// or <see cref="ulong"/> (integral, so never compared within a Real's tolerance). An enum is its
/// schema name and a bytes field its protobuf-JSON base64. A field without presence that holds
/// its default is absent, as it is on the wire.
/// </summary>
internal sealed class Normalizer
{
    /// <summary>What a service's build version becomes.</summary>
    internal const string VersionPlaceholder = "${version}";

    /// <summary>What an absolute path becomes.</summary>
    internal const string PathPlaceholder = "${path}";

    private const string VersionField = "sysml.ServerInfoResponse.version";

    /// <summary>The int64 fields carrying a runtime instance id, which a call assigns afresh.</summary>
    private static readonly FrozenSet<string> RuntimeIds = FrozenSet.Create(
        StringComparer.Ordinal,
        "sysml.Instance.id",
        "sysml.Value.instance_id",
        "sysml.Verdict.instance_id",
        "sysml.Function.self_id",
        "sysml.DocumentObject.instance_id");

    private readonly string modelHash;
    private readonly Dictionary<long, string> labels = [];

    /// <summary>A normalizer for one response.</summary>
    /// <param name="modelHash">The hash of the scenario's model, which becomes <c>${model_hash}</c>; empty when it has none.</param>
    internal Normalizer(string modelHash)
    {
        ArgumentNullException.ThrowIfNull(modelHash);
        this.modelHash = modelHash;
    }

    /// <summary>
    /// The normalized tree of a message. Runtime instance ids become <c>@1</c>, <c>@2</c>, … in order
    /// of first appearance, visiting fields by number, map entries by key and lists in order, so the
    /// labels are reproducible and two ids naming one object keep one label.
    /// </summary>
    /// <param name="message">The response.</param>
    /// <returns>Its tree.</returns>
    internal IReadOnlyDictionary<string, object?> Normalize(IMessage message)
    {
        ArgumentNullException.ThrowIfNull(message);
        var tree = new SortedDictionary<string, object?>(Spelling.CodePointOrder);
        foreach (var field in message.Descriptor.Fields.InFieldNumberOrder())
        {
            var value = field.Accessor.GetValue(message);
            if (!IsSet(field, message, value))
            {
                continue;
            }

            tree[field.Name] = field switch
            {
                { IsMap: true } => Entries(field, (IDictionary)value),
                { IsRepeated: true } => ((IList)value).Cast<object>().Select(item => Scalar(field, item)).ToList(),
                _ => Scalar(field, value),
            };
        }

        return tree;
    }

    /// <summary>Whether a field is set as the wire would carry it: a non-empty list or map, a present field, or a scalar off its default.</summary>
    /// <param name="field">The field.</param>
    /// <param name="message">The message holding it.</param>
    /// <param name="value">Its value in the message.</param>
    /// <returns>Whether it is set.</returns>
    internal static bool IsSet(FieldDescriptor field, IMessage message, object value)
    {
        if (field.IsMap || field.IsRepeated)
        {
            return ((ICollection)value).Count > 0;
        }

        if (field.HasPresence)
        {
            return field.Accessor.HasValue(message);
        }

        return value switch
        {
            string text => text.Length > 0,
            ByteString bytes => !bytes.IsEmpty,
            bool flag => flag,
            // As on the wire, a negative zero is not the default.
            double real => BitConverter.DoubleToInt64Bits(real) != 0,
            float real => BitConverter.SingleToInt32Bits(real) != 0,
            Enum number => Convert.ToInt64(number, CultureInfo.InvariantCulture) != 0,
            int number => number != 0,
            long number => number != 0,
            uint number => number != 0,
            ulong number => number != 0,
            _ => throw new InvalidOperationException($"{field.FullName} holds a {value.GetType()}, which no protobuf scalar is"),
        };
    }

    private SortedDictionary<string, object?> Entries(FieldDescriptor field, IDictionary map)
    {
        var keyField = field.MessageType.FindFieldByNumber(1);
        var valueField = field.MessageType.FindFieldByNumber(2);
        var entries = new List<(string Key, object Value)>(map.Count);
        var enumerator = map.GetEnumerator();
        while (enumerator.MoveNext())
        {
            entries.Add((MapKey(keyField, enumerator.Key), enumerator.Value!));
        }

        var byKey = new SortedDictionary<string, object?>(Spelling.CodePointOrder);
        foreach (var (key, value) in entries.OrderBy(entry => entry.Key, Spelling.CodePointOrder))
        {
            byKey[key] = Scalar(valueField, value);
        }

        return byKey;
    }

    private static string MapKey(FieldDescriptor field, object key) => key switch
    {
        string text => text,
        bool flag => flag ? "true" : "false",
        IFormattable number => number.ToString(null, CultureInfo.InvariantCulture),
        _ => throw new InvalidOperationException($"{field.FullName} has a {key.GetType()} key, which no protobuf map key is"),
    };

    private object Scalar(FieldDescriptor field, object value) => field.FieldType switch
    {
        FieldType.Message => Normalize((IMessage)value),
        FieldType.Enum => EnumName(field, value),
        FieldType.Bool => (bool)value,
        FieldType.String => Text(field.FullName, (string)value),
        FieldType.Bytes => ((ByteString)value).ToBase64(),
        FieldType.Double => (double)value,
        FieldType.Float => (double)(float)value,
        FieldType.Int64 or FieldType.SInt64 or FieldType.SFixed64 =>
            RuntimeIds.Contains(field.FullName) ? Label((long)value) : (long)value,
        FieldType.Int32 or FieldType.SInt32 or FieldType.SFixed32 => (long)(int)value,
        FieldType.UInt32 or FieldType.Fixed32 => (ulong)(uint)value,
        FieldType.UInt64 or FieldType.Fixed64 => (ulong)value,
        _ => throw new InvalidOperationException($"{field.FullName} has the field type {field.FieldType}, which this runner does not read"),
    };

    /// <summary>An enum as the schema names it; a number the schema does not name stays that number.</summary>
    private static object EnumName(FieldDescriptor field, object value)
    {
        var number = Convert.ToInt32(value, CultureInfo.InvariantCulture);
        return field.EnumType.FindValueByNumber(number)?.Name ?? (object)(long)number;
    }

    /// <summary>A string, unless it is the build version, the model's hash or an absolute path.</summary>
    private string Text(string field, string text)
    {
        if (field == VersionField)
        {
            return VersionPlaceholder;
        }

        if (modelHash.Length > 0 && text == modelHash)
        {
            return ScenarioRequests.ModelHashPlaceholder;
        }

        return Path.IsPathFullyQualified(text) ? PathPlaceholder : text;
    }

    private string Label(long id)
    {
        if (!labels.TryGetValue(id, out var label))
        {
            label = $"@{(labels.Count + 1).ToString(CultureInfo.InvariantCulture)}";
            labels.Add(id, label);
        }

        return label;
    }
}
