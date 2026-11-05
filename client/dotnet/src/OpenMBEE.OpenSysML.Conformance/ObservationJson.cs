using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.Encodings.Web;
using System.Text.Json;

namespace OpenMBEE.OpenSysML.Conformance;

/// <summary>
/// Renders a normalized response as indented JSON with its keys sorted, as the reference runner's
/// <c>-v</c> prints it: the observation a scenario was judged against.
/// </summary>
internal static class ObservationJson
{
    private static readonly JsonWriterOptions Options = new()
    {
        Indented = true,
        IndentSize = 2,
        NewLine = "\n",
        Encoder = JavaScriptEncoder.UnsafeRelaxedJsonEscaping,
    };

    /// <summary>A normalized tree as JSON. A Real JSON cannot hold (NaN, an infinity) is written as its spelling, as text.</summary>
    /// <param name="tree">The tree.</param>
    /// <returns>The JSON, without a final newline.</returns>
    internal static string Render(IReadOnlyDictionary<string, object?> tree)
    {
        ArgumentNullException.ThrowIfNull(tree);
        using var buffer = new MemoryStream();
        using (var json = new Utf8JsonWriter(buffer, Options))
        {
            Write(json, tree);
        }

        return Encoding.UTF8.GetString(buffer.ToArray());
    }

    private static void Write(Utf8JsonWriter json, object? value)
    {
        switch (value)
        {
            case null:
                json.WriteNullValue();
                break;
            case string text:
                json.WriteStringValue(text);
                break;
            case bool flag:
                json.WriteBooleanValue(flag);
                break;
            case long integral:
                json.WriteNumberValue(integral);
                break;
            case ulong unsigned:
                json.WriteNumberValue(unsigned);
                break;
            case double real when double.IsFinite(real):
                json.WriteNumberValue(real);
                break;
            case double real:
                json.WriteStringValue(Spelling.Real(real));
                break;
            case IReadOnlyDictionary<string, object?> map:
                json.WriteStartObject();
                foreach (var key in map.Keys.Order(Spelling.CodePointOrder))
                {
                    json.WritePropertyName(key);
                    Write(json, map[key]);
                }

                json.WriteEndObject();
                break;
            case IReadOnlyList<object?> list:
                json.WriteStartArray();
                foreach (var item in list)
                {
                    Write(json, item);
                }

                json.WriteEndArray();
                break;
            default:
                throw new ArgumentException($"a normalized tree holds no {value.GetType()}", nameof(value));
        }
    }
}
